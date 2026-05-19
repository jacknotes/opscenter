package service

import (
	"fmt"
	"regexp"
	"strings"
)

// VirtualServer 表示 LVS 虚拟服务器（VS）及其后端真实服务器列表。
type VirtualServer struct {
	IP          string       `json:"ip"`
	Port        string       `json:"port"`
	Protocol    string       `json:"protocol"`
	RealServers []RealServer `json:"real_servers"`
}

// RealServer 表示 LVS 后端真实服务器（RS）的状态信息。
type RealServer struct {
	IP         string `json:"ip"`
	Port       string `json:"port"`
	Forward    string `json:"forward"`
	Weight     int    `json:"weight"`
	ActiveConn int    `json:"active_conn"`
	InActConn  int    `json:"inact_conn"`
	Status     string `json:"status"`
}

// LVSService 提供 LVS 管理的业务逻辑，包括输出解析和预览生成。
type LVSService struct {
	sshManager *SSHManager
}

// NewLVSService 创建 LVS 服务实例。
func NewLVSService(sshManager *SSHManager) *LVSService {
	return &LVSService{sshManager: sshManager}
}

var (
	vsPattern = regexp.MustCompile(`^(\w+)\s+(\d+\.\d+\.\d+\.\d+):(\d+)\s+\w+.*$`)
	rsPattern = regexp.MustCompile(`^\s+->\s+(\d+\.\d+\.\d+\.\d+):(\d+)\s+(\w+)\s+(\d+)\s+(\d+)\s+(\d+)$`)
)

// ParseListOutput 解析 ipvsadm -ln 命令输出，提取虚拟服务器和真实服务器信息。
func (s *LVSService) ParseListOutput(output string) []VirtualServer {
	var servers []VirtualServer
	var currentVS *VirtualServer

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "IP Virtual") || strings.HasPrefix(line, "Prot") {
			continue
		}

		if matches := vsPattern.FindStringSubmatch(line); matches != nil {
			if currentVS != nil {
				servers = append(servers, *currentVS)
			}
			currentVS = &VirtualServer{
				Protocol: matches[1],
				IP:       matches[2],
				Port:     matches[3],
			}
			continue
		}

		if currentVS != nil {
			if matches := rsPattern.FindStringSubmatch(line); matches != nil {
				status := "up"
				if strings.Contains(line, "down") {
					status = "down"
				}
				rs := RealServer{
					IP:         matches[1],
					Port:       matches[2],
					Forward:    matches[3],
					Weight:     parseInt(matches[4]),
					ActiveConn: parseInt(matches[5]),
					InActConn:  parseInt(matches[6]),
					Status:     status,
				}
				currentVS.RealServers = append(currentVS.RealServers, rs)
			}
		}
	}

	if currentVS != nil {
		servers = append(servers, *currentVS)
	}

	return servers
}

// ParseStatusOutput 解析 LVS 状态输出，当前直接返回原文。
func (s *LVSService) ParseStatusOutput(output string) string {
	return output
}

// GenerateOpPreview 生成 LVS 上线/下线操作的命令和描述。
func (s *LVSService) GenerateOpPreview(scriptPath, vsIP, rsIP, state string) (command, description string) {
	command = fmt.Sprintf("%s op %s %s %s", scriptPath, vsIP, rsIP, state)
	if state == "on" {
		description = fmt.Sprintf("将 192.168.13.%s 从 192.168.13.%s 的后端上线", rsIP, vsIP)
	} else {
		description = fmt.Sprintf("将 192.168.13.%s 从 192.168.13.%s 的后端下线", rsIP, vsIP)
	}
	return
}

// GenerateSwapPreview 生成 LVS 切换操作的命令和描述。
func (s *LVSService) GenerateSwapPreview(scriptPath, vsIP, rsIP1, rsIP2 string) (command, description string) {
	command = fmt.Sprintf("%s swap %s %s %s", scriptPath, vsIP, rsIP1, rsIP2)
	description = fmt.Sprintf("切换 192.168.13.%s 和 192.168.13.%s 在 192.168.13.%s 的状态", rsIP1, rsIP2, vsIP)
	return
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
