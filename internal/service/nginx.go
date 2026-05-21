package service

import (
	"fmt"
	"regexp"
	"strings"
)

// NginxUpstream 表示 Nginx upstream 块，包含名称和后端服务器列表。
type NginxUpstream struct {
	Name    string        `json:"name"`
	Servers []NginxServer `json:"servers"`
	Config  string        `json:"config"`
}

// NginxServer 表示 upstream 中的一个后端服务器。
type NginxServer struct {
	IP     string `json:"ip"`
	Port   string `json:"port"`
	Status string `json:"status"`
	Weight string `json:"weight"`
}

// NginxService 提供 Nginx upstream 管理的业务逻辑，包括配置解析、diff 生成和 sed 命令构建。
type NginxService struct {
	sshManager *SSHManager
}

// NewNginxService 创建 Nginx 服务实例。
func NewNginxService(sshManager *SSHManager) *NginxService {
	return &NginxService{sshManager: sshManager}
}

var (
	upstreamPattern        = regexp.MustCompile(`(?s)upstream\s+([\w-]+)\s*\{([^}]*)\}`)
	serverPattern          = regexp.MustCompile(`server\s+([\d.]+)(?::(\d+))?(?:\s+weight=(\d+))?\s*;`)
	commentedServerPattern = regexp.MustCompile(`#+server\s+([\d.]+)(?::(\d+))?(?:\s+weight=(\d+))?\s*;`)
)

// ParseConfig 解析 Nginx 配置文件内容，提取所有 upstream 块及其后端服务器。
// 注释的 server 行（#server）状态为 down，未注释的为 up。
func (s *NginxService) ParseConfig(configContent string) []NginxUpstream {
	var upstreams []NginxUpstream

	matches := upstreamPattern.FindAllStringSubmatch(configContent, -1)
	for _, match := range matches {
		name := match[1]
		body := match[2]

		u := NginxUpstream{
			Name:   name,
			Config: match[0],
		}

		lines := strings.Split(body, "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}

			// 必须先检查注释行，否则 #server 会被 serverPattern 匹配
			if sm := commentedServerPattern.FindStringSubmatch(trimmedLine); sm != nil {
				port := sm[2]
				if port == "" {
					port = "80"
				}
				u.Servers = append(u.Servers, NginxServer{
					IP:     sm[1],
					Port:   port,
					Weight: sm[3],
					Status: "down",
				})
				continue
			}

			// 检查是否是未注释的 server 行
			if sm := serverPattern.FindStringSubmatch(trimmedLine); sm != nil {
				port := sm[2]
				if port == "" {
					port = "80"
				}
				u.Servers = append(u.Servers, NginxServer{
					IP:     sm[1],
					Port:   port,
					Weight: sm[3],
					Status: "up",
				})
			}
		}

		upstreams = append(upstreams, u)
	}

	return upstreams
}

func matchLine(line, backendIP string) bool {
	if strings.Contains(line, backendIP) {
		return true
	}
	// 后端 IP 带端口时，尝试只匹配 IP 部分（处理 upstream 中 server 未写端口的情况）
	if idx := strings.LastIndex(backendIP, ":"); idx > 0 {
		ipOnly := backendIP[:idx]
		port := backendIP[idx+1:]
		if port == "80" && strings.Contains(line, ipOnly) {
			return true
		}
	}
	return false
}

// GenerateDiff 生成单个 server 上线/下线操作的配置 diff。
// online 操作取消注释，offline 操作添加注释。
func (s *NginxService) GenerateDiff(config string, upstreamName, backendIP, action string) (before, after string) {
	before = config

	lines := strings.Split(config, "\n")
	var newLines []string
	inTargetUpstream := false

	for _, line := range lines {
		if strings.Contains(line, "upstream") {
			inTargetUpstream = strings.Contains(line, upstreamName)
		}

		if inTargetUpstream && strings.TrimSpace(line) == "}" {
			inTargetUpstream = false
		} else if inTargetUpstream && matchLine(line, backendIP) {
			trimmedLine := strings.TrimSpace(line)
			switch action {
			case "online":
				trimmed := strings.TrimLeft(trimmedLine, "#")
				if strings.HasPrefix(trimmed, "server") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + trimmed
				}
			case "offline":
				if strings.HasPrefix(trimmedLine, "server") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + "#" + trimmedLine
				}
			}
		}

		newLines = append(newLines, line)
	}

	after = strings.Join(newLines, "\n")
	return
}

// GenerateBackupCommand 生成配置文件备份命令，备份文件名带时间戳。自动创建备份目录。
func (s *NginxService) GenerateBackupCommand(configPath, backupPath, configFile string) string {
	return fmt.Sprintf("mkdir -p %s && cp %s/%s %s/%s.bak.$(date +%%Y%%m%%d%%H%%M%%S)", backupPath, configPath, configFile, backupPath, configFile)
}

// GenerateCleanupCommand 生成清理旧备份命令，保留最近 maxBackups 个备份文件。
func (s *NginxService) GenerateCleanupCommand(backupPath, configFile string, maxBackups int) string {
	return fmt.Sprintf("cd %s && ls -t %s.bak.* 2>/dev/null | tail -n +%d | xargs -r rm -f", backupPath, configFile, maxBackups+1)
}

// GenerateModifyCommand 生成 sed 命令用于批量上线/下线多个后端 IP。
// 支持 :80 端口的自动省略匹配。
func (s *NginxService) GenerateModifyCommand(configPath, configFile, upstreamName string, backendIPs []string, action string) string {
	var ipParts []string
	for _, ip := range backendIPs {
		if idx := strings.LastIndex(ip, ":"); idx > 0 {
			port := ip[idx+1:]
			if port == "80" {
				ipParts = append(ipParts, ip[:idx])
				continue
			}
		}
		ipParts = append(ipParts, ip)
	}
	ipPattern := strings.Join(ipParts, "\\|")

	var sedPattern string
	switch action {
	case "online":
		sedPattern = fmt.Sprintf("sed -i '/%s/,/}/{s/#\\+server\\(.*\\(%s\\)\\)/server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile)
	case "offline":
		sedPattern = fmt.Sprintf("sed -i '/%s/,/}/{s/server\\(.*\\(%s\\)\\)/#server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile)
	}
	return sedPattern
}

// GenerateSwapDiff 生成切换操作的 diff（同时下线一个 server 并上线另一个 server）
func (s *NginxService) GenerateSwapDiff(config, upstreamName, offlineIP, onlineIP string) (before, after string) {
	before = config

	lines := strings.Split(config, "\n")
	var newLines []string
	inTargetUpstream := false

	for _, line := range lines {
		if strings.Contains(line, "upstream") {
			inTargetUpstream = strings.Contains(line, upstreamName)
		}

		if inTargetUpstream && strings.TrimSpace(line) == "}" {
			inTargetUpstream = false
		} else if inTargetUpstream && matchLine(line, offlineIP) {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "server") {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				line = indent + "#" + trimmedLine
			}
		} else if inTargetUpstream && matchLine(line, onlineIP) {
			trimmedLine := strings.TrimSpace(line)
			trimmed := strings.TrimLeft(trimmedLine, "#")
			if strings.HasPrefix(trimmed, "server") {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				line = indent + trimmed
			}
		}

		newLines = append(newLines, line)
	}

	after = strings.Join(newLines, "\n")
	return
}

// GenerateSwapModifyCommands 生成切换操作的 sed 命令列表
func (s *NginxService) GenerateSwapModifyCommands(configPath, configFile, upstreamName, offlineIP, onlineIP string) []string {
	offlinePattern := offlineIP
	if idx := strings.LastIndex(offlineIP, ":"); idx > 0 {
		if offlineIP[idx+1:] == "80" {
			offlinePattern = offlineIP[:idx]
		}
	}
	onlinePattern := onlineIP
	if idx := strings.LastIndex(onlineIP, ":"); idx > 0 {
		if onlineIP[idx+1:] == "80" {
			onlinePattern = onlineIP[:idx]
		}
	}

	offlineCmd := fmt.Sprintf("sed -i '/%s/,/}/{s/server\\(.*\\(%s\\)\\)/#server\\1/}' %s/%s", upstreamName, offlinePattern, configPath, configFile)
	onlineCmd := fmt.Sprintf("sed -i '/%s/,/}/{s/#\\+server\\(.*\\(%s\\)\\)/server\\1/}' %s/%s", upstreamName, onlinePattern, configPath, configFile)

	return []string{offlineCmd, onlineCmd}
}

// GenerateToggleDiff 生成组切换操作的 diff（反转整个 upstream 组内所有 server 的状态）
func (s *NginxService) GenerateToggleDiff(config, upstreamName string) (before, after string) {
	before = config

	lines := strings.Split(config, "\n")
	var newLines []string
	inTargetUpstream := false

	for _, line := range lines {
		if strings.Contains(line, "upstream") {
			inTargetUpstream = strings.Contains(line, upstreamName)
		}

		if inTargetUpstream && strings.TrimSpace(line) == "}" {
			inTargetUpstream = false
		} else if inTargetUpstream {
			trimmedLine := strings.TrimSpace(line)
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			if commentedServerPattern.FindStringSubmatch(trimmedLine) != nil {
				// 注释的 server → 取消注释（上线）
				line = indent + strings.TrimLeft(trimmedLine, "#")
			} else if serverPattern.FindStringSubmatch(trimmedLine) != nil {
				// 未注释的 server → 加注释（下线）
				line = indent + "#" + trimmedLine
			}
		}

		newLines = append(newLines, line)
	}

	after = strings.Join(newLines, "\n")
	return
}

// GenerateToggleModifyCommands 生成组切换操作的 sed 命令列表（按 IP 精确匹配，避免相互干扰）
func (s *NginxService) GenerateToggleModifyCommands(configPath, configFile, upstreamName string, servers []NginxServer) []string {
	var commands []string
	for _, srv := range servers {
		ipPattern := srv.IP
		if srv.Port != "" && srv.Port != "80" {
			ipPattern = srv.IP + ":" + srv.Port
		}
		if srv.Status == "up" {
			commands = append(commands, fmt.Sprintf("sed -i '/%s/,/}/{s/server\\(.*\\(%s\\)\\)/#server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile))
		} else {
			commands = append(commands, fmt.Sprintf("sed -i '/%s/,/}/{s/#\\+server\\(.*\\(%s\\)\\)/server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile))
		}
	}
	return commands
}

// LineDiff 表示一行的 diff
type LineDiff struct {
	LineNum int    `json:"line_num"`
	Type    string `json:"type"` // "same", "added", "removed"
	Content string `json:"content"`
}

// GenerateLineDiffs 生成逐行 diff
func (s *NginxService) GenerateLineDiffs(before, after string) []LineDiff {
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	var diffs []LineDiff

	beforeIdx := 0
	afterIdx := 0

	for beforeIdx < len(beforeLines) || afterIdx < len(afterLines) {
		if beforeIdx < len(beforeLines) && afterIdx < len(afterLines) {
			bLine := beforeLines[beforeIdx]
			aLine := afterLines[afterIdx]
			bTrimmed := strings.TrimSpace(bLine)
			aTrimmed := strings.TrimSpace(aLine)

			if bTrimmed == aTrimmed {
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "same",
					Content: afterLines[afterIdx],
				})
				beforeIdx++
				afterIdx++
			} else if strings.HasPrefix(aTrimmed, "#") && strings.TrimLeft(aTrimmed, "#") == strings.TrimLeft(bTrimmed, "#") && strings.HasPrefix(strings.TrimLeft(aTrimmed, "#"), "server") {
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "removed",
					Content: bLine,
				})
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "added",
					Content: aLine,
				})
				beforeIdx++
				afterIdx++
			} else if strings.HasPrefix(bTrimmed, "#") && strings.TrimLeft(bTrimmed, "#") == strings.TrimLeft(aTrimmed, "#") && strings.HasPrefix(strings.TrimLeft(bTrimmed, "#"), "server") {
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "removed",
					Content: bLine,
				})
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "added",
					Content: aLine,
				})
				beforeIdx++
				afterIdx++
			} else {
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "same",
					Content: afterLines[afterIdx],
				})
				beforeIdx++
				afterIdx++
			}
		} else if beforeIdx < len(beforeLines) {
			diffs = append(diffs, LineDiff{
				LineNum: len(diffs) + 1,
				Type:    "removed",
				Content: beforeLines[beforeIdx],
			})
			beforeIdx++
		} else {
			diffs = append(diffs, LineDiff{
				LineNum: afterIdx + 1,
				Type:    "added",
				Content: afterLines[afterIdx],
			})
			afterIdx++
		}
	}

	return diffs
}
