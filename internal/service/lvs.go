package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"opscenter/internal/model"
)

// StatusGroup 表示从 keepalived 配置中解析出的 VS 及其 RS 列表。
type StatusGroup struct {
	VSIP        string    `json:"vs_ip"`
	VSPort      string    `json:"vs_port"`
	RealServers []StatusRS `json:"real_servers"`
}

// StatusRS 表示 keepalived 配置中的 RS 条目及其状态。
type StatusRS struct {
	IP     string `json:"ip"`
	Port   string `json:"port"`
	Status string `json:"status"`
}

// VirtualServer 表示 LVS 虚拟服务器（VS）及其后端真实服务器列表。
type VirtualServer struct {
	IP          string       `json:"ip"`
	Port        string       `json:"port"`
	Protocol    string       `json:"protocol"`
	Scheduler   string       `json:"scheduler"`
	Flags       string       `json:"flags"`
	RealServers []RealServer `json:"real_servers"`
	Role        string       `json:"role,omitempty"` // "master" 或 "backup"
	Tag         string       `json:"tag,omitempty"`  // VS 标签
}

// RealServer 表示 LVS 后端真实服务器（RS）的状态信息。
type RealServer struct {
	IP             string `json:"ip"`
	Port           string `json:"port"`
	Forward        string `json:"forward"`
	Weight         int    `json:"weight"`
	ActiveConn     int    `json:"active_conn"`
	InActConn      int    `json:"inact_conn"`
	Status         string `json:"status"`
	Tag            string `json:"tag,omitempty"`
	Disabled       bool   `json:"disabled,omitempty"`
	DisabledReason string `json:"disabled_reason,omitempty"`
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
	vsPattern       = regexp.MustCompile(`^(\w+)\s+(\d+\.\d+\.\d+\.\d+):(\d+)\s+(\w+)\s*(.*)$`)
	rsPattern       = regexp.MustCompile(`^->\s+(\d+\.\d+\.\d+\.\d+):(\d+)\s+(\w+)\s+(\d+)\s+(\d+)\s+(\d+)$`)
	statusVsPattern = regexp.MustCompile(`vs_(\d+\.\d+\.\d+\.\d+)_(\d+)\.conf:`)
	statusRsPattern = regexp.MustCompile(`rs_(\d+\.\d+\.\d+\.\d+)_(\d+)\.conf`)
)

// ParseListOutput 解析 ipvsadm -ln 命令输出，提取虚拟服务器和真实服务器信息。
func (s *LVSService) ParseListOutput(output string) []VirtualServer {
	var servers []VirtualServer
	var currentVS *VirtualServer

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "IP Virtual") || strings.HasPrefix(line, "Prot") {
			continue
		}

		if matches := vsPattern.FindStringSubmatch(line); matches != nil {
			if currentVS != nil {
				servers = append(servers, *currentVS)
			}
			currentVS = &VirtualServer{
				Protocol:  matches[1],
				IP:        matches[2],
				Port:      matches[3],
				Scheduler: matches[4],
				Flags:     strings.TrimSpace(matches[5]),
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

// ParseStatusOutput 解析 keepalived 配置状态输出，返回结构化数据。
// 输出格式: /path/vs_IP_PORT.conf:    include ../real_server/rs_IP_PORT.conf
// 带 '!' 前缀的 include 表示该 RS 已下线。
func (s *LVSService) ParseStatusOutput(output string) []StatusGroup {
	groupMap := make(map[string]*StatusGroup)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 提取 VS 文件名中的 IP 和端口
		vsMatch := statusVsPattern.FindStringSubmatch(line)
		if vsMatch == nil {
			continue
		}
		vsKey := vsMatch[1] + ":" + vsMatch[2]

		// 提取 RS 文件名中的 IP 和端口
		rsMatch := statusRsPattern.FindStringSubmatch(line)
		if rsMatch == nil {
			continue
		}

		if _, ok := groupMap[vsKey]; !ok {
			groupMap[vsKey] = &StatusGroup{
				VSIP:   vsMatch[1],
				VSPort: vsMatch[2],
			}
		}

		// 判断是否被注释（下线）
		down := strings.Contains(line[:strings.Index(line, "include")], "!")
		status := "up"
		if down {
			status = "down"
		}

		groupMap[vsKey].RealServers = append(groupMap[vsKey].RealServers, StatusRS{
			IP:     rsMatch[1],
			Port:   rsMatch[2],
			Status: status,
		})
	}

	var result []StatusGroup
	for _, g := range groupMap {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].VSIP != result[j].VSIP {
			return result[i].VSIP < result[j].VSIP
		}
		return result[i].VSPort < result[j].VSPort
	})
	return result
}

// MergeOfflineRS 将 status 中的下线 RS 合并到 list 结果中。
// ipvsadm -ln 不显示下线的 RS，需要从 keepalived 配置中补充。
func (s *LVSService) MergeOfflineRS(listServers []VirtualServer, statusGroups []StatusGroup) []VirtualServer {
	// 建立 list 结果索引: "IP:Port" -> VirtualServer
	listIndex := make(map[string]int)
	for i, vs := range listServers {
		listIndex[vs.IP+":"+vs.Port] = i
	}

	// 建立 list 中每个 VS 的 RS 索引: "VS_IP:VS_Port:RS_IP" -> true
	rsIndex := make(map[string]bool)
	for _, vs := range listServers {
		for _, rs := range vs.RealServers {
			rsIndex[vs.IP+":"+vs.Port+":"+rs.IP] = true
		}
	}

	for _, sg := range statusGroups {
		vsKey := sg.VSIP + ":" + sg.VSPort
		idx, exists := listIndex[vsKey]
		if !exists {
			// VS 不在 list 中（异常情况），跳过
			continue
		}

		for _, srs := range sg.RealServers {
			rsKey := sg.VSIP + ":" + sg.VSPort + ":" + srs.IP
			if rsIndex[rsKey] {
				continue // 已存在，跳过
			}
			// 补充离线 RS
			listServers[idx].RealServers = append(listServers[idx].RealServers, RealServer{
				IP:     srs.IP,
				Port:   srs.Port,
				Status: srs.Status,
			})
			rsIndex[rsKey] = true
		}
	}

	return listServers
}

// DetectRoles 检测 VS IP 是否绑定在本机，判断主备角色。
// 返回 "master" 或 "backup"，检测失败返回空字符串。
func (s *LVSService) DetectRoles(vsIPs []string, server *model.Server) map[string]string {
	roles := make(map[string]string)
	if len(vsIPs) == 0 {
		return roles
	}

	escapedIPs := make([]string, len(vsIPs))
	for i, ip := range vsIPs {
		escapedIPs[i] = strings.ReplaceAll(ip, ".", "\\.")
	}
	grepPattern := strings.Join(escapedIPs, "|")
	checkCmd := fmt.Sprintf("ip -4 a show | grep -oE '%s' || true", grepPattern)
	checkOutput, checkErr := s.sshManager.Execute(context.Background(), server, checkCmd)
	if checkErr != nil {
		return roles
	}

	masterIPs := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(checkOutput), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			masterIPs[line] = true
		}
	}
	for _, ip := range vsIPs {
		if masterIPs[ip] {
			roles[ip] = "master"
		} else {
			roles[ip] = "backup"
		}
	}
	return roles
}

// GenerateOpPreview 生成 LVS 上线/下线操作的命令和描述。
// vsIP/rsIP 为完整 IP 地址，脚本接收末位数字，IP_PREFIX 通过环境变量传递。
func (s *LVSService) GenerateOpPreview(scriptPath, vsIP, rsIP, state string) (command, description string) {
	prefix := ipPrefix(vsIP)
	command = fmt.Sprintf("IP_PREFIX=%s %s op %s %s %s", prefix, scriptPath, lastOctet(vsIP), lastOctet(rsIP), state)
	if state == "on" {
		description = fmt.Sprintf("将 %s 从 %s 的后端上线", rsIP, vsIP)
	} else {
		description = fmt.Sprintf("将 %s 从 %s 的后端下线", rsIP, vsIP)
	}
	return
}

// GenerateSwapPreview 生成 LVS 切换操作的命令和描述。
// vsIP/rsIP1/rsIP2 为完整 IP 地址，脚本接收末位数字，IP_PREFIX 通过环境变量传递。
func (s *LVSService) GenerateSwapPreview(scriptPath, vsIP, rsIP1, rsIP2 string) (command, description string) {
	prefix := ipPrefix(vsIP)
	command = fmt.Sprintf("IP_PREFIX=%s %s swap %s %s %s", prefix, scriptPath, lastOctet(vsIP), lastOctet(rsIP1), lastOctet(rsIP2))
	description = fmt.Sprintf("切换 %s 和 %s 在 %s 的状态", rsIP1, rsIP2, vsIP)
	return
}

// lastOctet 从完整 IP 地址中提取末位数字。若已是纯数字则直接返回。
func lastOctet(ip string) string {
	if idx := strings.LastIndex(ip, "."); idx >= 0 {
		return ip[idx+1:]
	}
	return ip
}

// ipPrefix 从完整 IP 地址中提取子网前缀（含末尾点号）。若无法提取则返回空字符串。
func ipPrefix(ip string) string {
	if idx := strings.LastIndex(ip, "."); idx >= 0 {
		return ip[:idx+1]
	}
	return ""
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
