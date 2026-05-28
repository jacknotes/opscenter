// Package service 提供业务逻辑层，包括 SSH 连接管理、操作预览、分布式锁、
// 以及 LVS/K8s/Nginx/预生产等业务的服务实现。
package service

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"opscenter/internal/config"
	"opscenter/internal/model"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshClient 包装 SSH 客户端，记录创建时间和最后使用时间
type sshClient struct {
	client    *ssh.Client
	createdAt time.Time
	lastUsed  time.Time
}

// SSHManager 管理 SSH 连接池，按服务器 ID 复用连接，支持单次执行和流式输出。
type SSHManager struct {
	clients     map[uint]*sshClient
	mu          sync.RWMutex
	maxIdle     time.Duration // 空闲超时
	maxLifetime time.Duration // 最大生命周期
}

// NewSSHManager 创建一个新的 SSH 连接管理器，并启动定期清理。
func NewSSHManager() *SSHManager {
	m := &SSHManager{
		clients:     make(map[uint]*sshClient),
		maxIdle:     10 * time.Minute,
		maxLifetime: 1 * time.Hour,
	}
	go m.cleanupStale()
	return m
}

// GetClient 获取指定服务器的 SSH 客户端，优先从连接池中复用，不存在则新建连接。
// 复用前检查连接是否过期（TTL 和空闲超时）。
func (m *SSHManager) GetClient(server *model.Server) (*ssh.Client, error) {
	m.mu.RLock()
	sc, ok := m.clients[server.ID]
	m.mu.RUnlock()

	if ok {
		now := time.Now()
		if now.Sub(sc.createdAt) > m.maxLifetime || now.Sub(sc.lastUsed) > m.maxIdle {
			m.CloseServer(server.ID)
		} else {
			sc.lastUsed = now
			return sc.client, nil
		}
	}

	return m.connect(server)
}

// hostKeyCallback 根据配置返回 SSH 主机密钥验证回调。
// 若配置了 known_hosts_path 则使用文件验证，否则跳过验证（不推荐生产环境）。
func hostKeyCallback() ssh.HostKeyCallback {
	knownHostsPath := config.Global.Server.KnownHostsPath
	if knownHostsPath == "" {
		log.Println("警告: 未配置 known_hosts_path，跳过 SSH 主机密钥验证（不推荐生产环境）")
		return ssh.InsecureIgnoreHostKey()
	}

	// 展开 ~ 路径
	if strings.HasPrefix(knownHostsPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("警告: 获取用户主目录失败: %v，跳过主机密钥验证", err)
			return ssh.InsecureIgnoreHostKey()
		}
		knownHostsPath = filepath.Join(home, knownHostsPath[2:])
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		log.Printf("警告: 加载 known_hosts 文件失败 (%s): %v，跳过主机密钥验证", knownHostsPath, err)
		return ssh.InsecureIgnoreHostKey()
	}
	return callback
}

// connect 建立 SSH 连接并加入连接池。使用 double-check 模式避免并发重复连接。
func (m *SSHManager) connect(server *model.Server) (*ssh.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if sc, ok := m.clients[server.ID]; ok {
		now := time.Now()
		if now.Sub(sc.createdAt) <= m.maxLifetime && now.Sub(sc.lastUsed) <= m.maxIdle {
			return sc.client, nil
		}
		// 过期，关闭旧连接
		sc.client.Close()
		delete(m.clients, server.ID)
	}

	var auth ssh.AuthMethod
	switch server.AuthType {
	case "password":
		auth = ssh.Password(server.Password)
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		auth = ssh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", server.AuthType)
	}

	sshCfg := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %v", err)
	}

	now := time.Now()
	m.clients[server.ID] = &sshClient{
		client:    client,
		createdAt: now,
		lastUsed:  now,
	}
	return client, nil
}

// Execute 在远程服务器上执行命令并返回合并输出（stdout + stderr）。
// 若会话创建失败会自动重连一次。
func (m *SSHManager) Execute(server *model.Server, command string) (string, error) {
	client, err := m.GetClient(server)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		// Try reconnect
		m.mu.Lock()
		delete(m.clients, server.ID)
		m.mu.Unlock()

		client, err = m.GetClient(server)
		if err != nil {
			return "", err
		}
		session, err = client.NewSession()
		if err != nil {
			return "", fmt.Errorf("创建会话失败: %v", err)
		}
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("执行命令失败: %v, 输出: %s", err, string(output))
	}

	return string(output), nil
}

// ExecuteWithTimeout 带超时的命令执行。超时后返回错误。
func (m *SSHManager) ExecuteWithTimeout(server *model.Server, command string, timeout time.Duration) (string, error) {
	type result struct {
		output string
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		output, err := m.Execute(server, command)
		ch <- result{output, err}
	}()

	select {
	case r := <-ch:
		return r.output, r.err
	case <-time.After(timeout):
		return "", fmt.Errorf("执行超时 (%v)", timeout)
	}
}

// ExecuteWithPipe 通过 stdin 管道将密码传递给远程命令。
// 密码使用 base64 编码避免 shell 元字符导致的注入问题。
func (m *SSHManager) ExecuteWithPipe(server *model.Server, command, password string) (string, error) {
	// 使用 base64 编码密码，避免单引号和 shell 元字符导致的命令注入
	encoded := base64.StdEncoding.EncodeToString([]byte(password))
	fullCommand := fmt.Sprintf("echo '%s' | base64 -d | %s", encoded, command)
	return m.Execute(server, fullCommand)
}

// Close 关闭所有 SSH 连接并清空连接池。
func (m *SSHManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sc := range m.clients {
		sc.client.Close()
	}
	m.clients = make(map[uint]*sshClient)
}

// CloseServer 关闭指定服务器的SSH连接，强制下次请求重新连接
func (m *SSHManager) CloseServer(serverID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sc, ok := m.clients[serverID]; ok {
		sc.client.Close()
		delete(m.clients, serverID)
	}
}

// cleanupStale 定期清理过期的 SSH 连接
func (m *SSHManager) cleanupStale() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, sc := range m.clients {
			if now.Sub(sc.createdAt) > m.maxLifetime || now.Sub(sc.lastUsed) > m.maxIdle {
				sc.client.Close()
				delete(m.clients, id)
			}
		}
		m.mu.Unlock()
	}
}

// StreamChunk 表示流式命令输出的一个片段，Err 标记是否来自 stderr。
type StreamChunk struct {
	Line string
	Err  bool
}

// ExecuteStream 在远程服务器上流式执行命令，通过 channel 返回实时输出。
// 密码通过 stdin 管道传递。返回两个 channel：输出片段和最终错误。
func (m *SSHManager) ExecuteStream(server *model.Server, command string, password string) (<-chan StreamChunk, <-chan error) {
	ch := make(chan StreamChunk, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		client, err := m.GetClient(server)
		if err != nil {
			errCh <- err
			return
		}

		session, err := client.NewSession()
		if err != nil {
			// Try reconnect
			m.mu.Lock()
			delete(m.clients, server.ID)
			m.mu.Unlock()

			client, err = m.GetClient(server)
			if err != nil {
				errCh <- err
				return
			}
			session, err = client.NewSession()
			if err != nil {
				errCh <- fmt.Errorf("创建会话失败: %v", err)
				return
			}
		}
		defer session.Close()

		stdin, err := session.StdinPipe()
		if err != nil {
			errCh <- fmt.Errorf("创建stdin管道失败: %v", err)
			return
		}

		stdout, err := session.StdoutPipe()
		if err != nil {
			errCh <- fmt.Errorf("创建stdout管道失败: %v", err)
			return
		}

		stderr, err := session.StderrPipe()
		if err != nil {
			errCh <- fmt.Errorf("创建stderr管道失败: %v", err)
			return
		}

		if err := session.Start(command); err != nil {
			errCh <- fmt.Errorf("启动命令失败: %v", err)
			return
		}

		// Send password via stdin pipe
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, password+"\n")
		}()

		// Read stdout and stderr concurrently
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			for scanner.Scan() {
				ch <- StreamChunk{Line: scanner.Text(), Err: false}
			}
		}()

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			for scanner.Scan() {
				ch <- StreamChunk{Line: scanner.Text(), Err: true}
			}
		}()

		// Wait for scanners to finish, then wait for session
		wg.Wait()
		errCh <- session.Wait()
	}()

	return ch, errCh
}

// Command whitelist patterns
var (
	lvsCommandPattern   = regexp.MustCompile(`^/[\w/./-]+\.sh\s+(list|status|op\s+[\d.]+\s+[\d.]+\s+(on|off)|swap\s+[\d.]+\s+[\d.]+\s+[\d.]+)$`)
	k8sCommandPattern   = regexp.MustCompile(`^/[\w/./-]+\.sh\s+(list|single_(online|sync|rollback)\s+[\w.-]+\s+[\w-]+|full_(online|sync|rollback)|scale(down|up)(\s+[\w.-]+)*)$`)
	nginxCommandPattern = regexp.MustCompile(`^(cat|cp|sed\s+-i|nginx\s+(-t|-s\s+reload)|ls)\s+[\w/.%*-]+$`)

	validateProjectNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	validateNamespacePattern   = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	validateUpstreamNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	validateConfigPatternPattern = regexp.MustCompile(`^[a-zA-Z0-9._*?,!-]+$`)

	// 文件路径中不允许的字符（防注入）
	unsafePathChars = regexp.MustCompile(`[;&|$\x60\n\r]`)
)

func ValidateCommand(serverType, command string) bool {
	switch serverType {
	case "lvs":
		return lvsCommandPattern.MatchString(command)
	case "kubernetes", "preprod":
		return k8sCommandPattern.MatchString(command)
	case "nginx":
		return nginxCommandPattern.MatchString(command)
	default:
		return false
	}
}

// ValidateIP validates a full IPv4 address or a last octet (1-254)
func ValidateIP(ip string) bool {
	if len(ip) == 0 {
		return false
	}
	// Full IPv4 address
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		for _, part := range parts {
			if len(part) == 0 || len(part) > 3 {
				return false
			}
			for _, c := range part {
				if c < '0' || c > '9' {
					return false
				}
			}
			if part[0] == '0' && len(part) > 1 {
				return false
			}
			n := 0
			for _, c := range part {
				n = n*10 + int(c-'0')
			}
			if n > 255 {
				return false
			}
		}
		return true
	}
	// Last octet only (1-254)
	if len(ip) > 3 {
		return false
	}
	for _, c := range ip {
		if c < '0' || c > '9' {
			return false
		}
	}
	if ip[0] == '0' && len(ip) > 1 {
		return false
	}
	n := 0
	for _, c := range ip {
		n = n*10 + int(c-'0')
	}
	return n >= 1 && n <= 254
}

// ValidateProjectName validates K8s project name
func ValidateProjectName(name string) bool {
	return validateProjectNamePattern.MatchString(name)
}

// ValidateNamespace validates K8s namespace
func ValidateNamespace(ns string) bool {
	return validateNamespacePattern.MatchString(ns)
}

// ValidateFilePath 校验文件路径不含 shell 元字符和路径穿越
func ValidateFilePath(path string) bool {
	if path == "" {
		return false
	}
	// 禁止路径穿越
	if strings.Contains(path, "..") {
		return false
	}
	// 禁止 shell 元字符
	if unsafePathChars.MatchString(path) {
		return false
	}
	return true
}

// ValidateUpstreamName validates Nginx upstream name (alphanumeric, underscore, hyphen)
func ValidateUpstreamName(name string) bool {
	return validateUpstreamNamePattern.MatchString(name)
}

// ValidateConfigPattern validates config file pattern (alphanumeric, dots, underscores, wildcards, hyphens)
func ValidateConfigPattern(pattern string) bool {
	return validateConfigPatternPattern.MatchString(pattern)
}

// ValidateDirectoryPath validates directory path (alphanumeric, slashes, dots, underscores, hyphens)
func ValidateDirectoryPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.Contains(path, "..") {
		return false
	}
	validateDirPattern := regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`)
	return validateDirPattern.MatchString(path)
}

// getHostKeyCallback 供 handler/server.go 的 TestConnection 使用
func GetHostKeyCallback() ssh.HostKeyCallback {
	return hostKeyCallback()
}
