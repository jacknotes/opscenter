package service

import (
	"fmt"
	"regexp"
	"strings"
)

type NginxUpstream struct {
	Name    string       `json:"name"`
	Servers []NginxServer `json:"servers"`
	Config  string       `json:"config"`
}

type NginxServer struct {
	IP     string `json:"ip"`
	Port   string `json:"port"`
	Status string `json:"status"`
	Weight string `json:"weight"`
}

type NginxService struct {
	sshManager *SSHManager
}

func NewNginxService(sshManager *SSHManager) *NginxService {
	return &NginxService{sshManager: sshManager}
}

var (
	// 匹配 upstream 块，支持名称中的下划线和连字符
	upstreamPattern = regexp.MustCompile(`(?s)upstream\s+([\w-]+)\s*\{([^}]*)\}`)
	// 匹配未注释的 server 行
	serverPattern = regexp.MustCompile(`server\s+([\d.]+)(?::(\d+))?(?:\s+weight=(\d+))?\s*;`)
	// 匹配被注释的 server 行（支持多个 #，如 #server、##server、###server）
	commentedServerPattern = regexp.MustCompile(`#+server\s+([\d.]+)(?::(\d+))?(?:\s+weight=(\d+))?\s*;`)
)

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

		// 解析每一行
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}

			// 检查是否是被注释的 server 行（必须在未注释之前检查）
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

func (s *NginxService) GenerateDiff(config string, upstreamName, backendIP, action string) (before, after string) {
	before = config

	lines := strings.Split(config, "\n")
	var newLines []string
	inTargetUpstream := false

	for _, line := range lines {
		// 检测进入目标 upstream 块
		if strings.Contains(line, "upstream") && strings.Contains(line, upstreamName) {
			inTargetUpstream = true
		}

		if inTargetUpstream && matchLine(line, backendIP) {
			trimmedLine := strings.TrimSpace(line)
			switch action {
			case "online":
				// 去掉 server 前面所有 #，支持 #server、##server、###server 等
				trimmed := strings.TrimLeft(trimmedLine, "#")
				if strings.HasPrefix(trimmed, "server") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + trimmed
				}
			case "offline":
				// 在 server 前面加 #，变成 #server
				if strings.HasPrefix(trimmedLine, "server") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					line = indent + "#" + trimmedLine
				}
			}
			inTargetUpstream = false
		}

		newLines = append(newLines, line)
	}

	after = strings.Join(newLines, "\n")
	return
}

func (s *NginxService) GenerateBackupCommand(configPath, backupPath, configFile string) string {
	return fmt.Sprintf("cp %s/%s %s/%s.bak.$(date +%%Y%%m%%d%%H%%M%%S)", configPath, configFile, backupPath, configFile)
}

func (s *NginxService) GenerateModifyCommand(configPath, configFile, upstreamName string, backendIPs []string, action string) string {
	// 构建多个 IP 的 OR 匹配模式: IP1\|IP2\|IP3
	// 对于带 :80 端口的 IP，只用 IP 部分匹配（兼容 upstream 中 server 未写端口的情况）
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
		// 去掉 server 前面所有 #（支持 #server、##server、###server 等）
		sedPattern = fmt.Sprintf("sed -i '/%s/,/}/{s/#\\+server\\(.*\\(%s\\)\\)/server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile)
	case "offline":
		// 在 server 前面加 #，变成 #server
		sedPattern = fmt.Sprintf("sed -i '/%s/,/}/{s/server\\(.*\\(%s\\)\\)/#server\\1/}' %s/%s", upstreamName, ipPattern, configPath, configFile)
	}
	return sedPattern
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
				// 相同行
				diffs = append(diffs, LineDiff{
					LineNum: afterIdx + 1,
					Type:    "same",
					Content: afterLines[afterIdx],
				})
				beforeIdx++
				afterIdx++
			} else if strings.HasPrefix(aTrimmed, "#") && strings.TrimLeft(aTrimmed, "#") == strings.TrimLeft(bTrimmed, "#") && strings.HasPrefix(strings.TrimLeft(aTrimmed, "#"), "server") {
				// 注释变更（上线/下线）：##server -> #server 或 server -> #server 等
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
				// 注释变更（上线/下线）：#server -> server 或 ##server -> server 等
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
				// 不同行（非注释变更）
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
