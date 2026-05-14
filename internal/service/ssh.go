package service

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"opscenter/internal/model"
)

type SSHManager struct {
	clients map[uint]*ssh.Client
	mu      sync.RWMutex
}

func NewSSHManager() *SSHManager {
	return &SSHManager{
		clients: make(map[uint]*ssh.Client),
	}
}

func (m *SSHManager) GetClient(server *model.Server) (*ssh.Client, error) {
	m.mu.RLock()
	if client, ok := m.clients[server.ID]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	return m.connect(server)
}

func (m *SSHManager) connect(server *model.Server) (*ssh.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if client, ok := m.clients[server.ID]; ok {
		return client, nil
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

	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %v", err)
	}

	m.clients[server.ID] = client
	return client, nil
}

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

func (m *SSHManager) ExecuteWithPipe(server *model.Server, command, password string) (string, error) {
	fullCommand := fmt.Sprintf("echo '%s' | %s", password, command)
	return m.Execute(server, fullCommand)
}

func (m *SSHManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[uint]*ssh.Client)
}

// Command whitelist patterns
var (
	lvsCommandPattern   = regexp.MustCompile(`^/[\w/./-]+\.sh\s+(list|status|op\s+\d{1,3}\s+\d{1,3}\s+(on|off)|swap\s+\d{1,3}\s+\d{1,3}\s+\d{1,3})$`)
	k8sCommandPattern   = regexp.MustCompile(`^/[\w/./-]+\.sh\s+(list|single_(online|sync|rollback)\s+[\w.-]+\s+[\w-]+|full_(online|sync|rollback)|(scale(down|up)))$`)
	nginxCommandPattern = regexp.MustCompile(`^(cat|cp|sed\s+-i|nginx\s+(-t|-s\s+reload)|ls)\s+[\w/.%*-]+$`)
)

func ValidateCommand(serverType, command string) bool {
	switch serverType {
	case "lvs":
		return lvsCommandPattern.MatchString(command)
	case "kubernetes":
		return k8sCommandPattern.MatchString(command)
	case "nginx":
		return nginxCommandPattern.MatchString(command)
	default:
		return false
	}
}

// ValidateIP validates IP last octet (1-254)
func ValidateIP(ip string) bool {
	if len(ip) == 0 || len(ip) > 3 {
		return false
	}
	for _, c := range ip {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ValidateProjectName validates K8s project name
func ValidateProjectName(name string) bool {
	pattern := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	return pattern.MatchString(name)
}

// ValidateNamespace validates K8s namespace
func ValidateNamespace(ns string) bool {
	pattern := regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	return pattern.MatchString(ns)
}
