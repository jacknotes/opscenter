package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

const nginxCmd = "PATH=$PATH:/usr/local/nginx/sbin:/usr/sbin:/opt/nginx/sbin nginx"
const maxBackups = 10

type NginxHandler struct {
	db          *gorm.DB
	sshManager  *service.SSHManager
	previewMgr  *service.PreviewManager
	nginxService *service.NginxService
}

func NewNginxHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager) *NginxHandler {
	return &NginxHandler{
		db:           db,
		sshManager:   sshManager,
		previewMgr:   previewMgr,
		nginxService: service.NewNginxService(sshManager),
	}
}

type NginxUpstreamRequest struct {
	ServerID     uint     `json:"server_id" binding:"required"`
	ConfigFile   string   `json:"config_file" binding:"required"`
	UpstreamNames []string `json:"upstream_names" binding:"required"`
	BackendIP    string   `json:"backend_ip" binding:"required"`
}

type NginxConfigRequest struct {
	ServerID uint `json:"server_id" binding:"required"`
}

type NginxReloadRequest struct {
	ServerID uint `json:"server_id" binding:"required"`
}

type NginxRollbackRequest struct {
	ServerID   uint   `json:"server_id" binding:"required"`
	ConfigFile string `json:"config_file" binding:"required"`
	BackupFile string `json:"backup_file" binding:"required"`
}

type NginxSwapRequest struct {
	ServerID      uint     `json:"server_id" binding:"required"`
	ConfigFile    string   `json:"config_file" binding:"required"`
	UpstreamNames []string `json:"upstream_names" binding:"required"`
	OfflineIP     string   `json:"offline_ip" binding:"required"`
	OnlineIP      string   `json:"online_ip" binding:"required"`
}

type NginxBatchSwapItem struct {
	UpstreamName string `json:"upstream_name"`
	OfflineIP    string `json:"offline_ip"`
	OnlineIP     string `json:"online_ip"`
}

type NginxBatchRequest struct {
	ServerID   uint                 `json:"server_id" binding:"required"`
	ConfigFile string               `json:"config_file" binding:"required"`
	Swaps      []NginxBatchSwapItem `json:"swaps"`
	Offlines   []NginxBatchSwapItem `json:"offlines"`
}

func (h *NginxHandler) Configs(c *gin.Context) {
	serverID := c.Query("server_id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定服务器"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// 确保路径以 / 结尾
	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	// 如果没有配置模式，使用默认的 *.conf
	configPattern := server.ConfigPattern
	if configPattern == "" {
		configPattern = "*.conf"
	}

	// 分离正向模式和排除模式（! 前缀）
	var includePatterns, excludePatterns []string
	for _, p := range strings.Split(configPattern, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(p, "!"))
		} else {
			includePatterns = append(includePatterns, p)
		}
	}

	// 对每个正向模式执行 ls，合并结果
	fileNames := make([]string, 0)
	seen := make(map[string]bool)
	for _, pattern := range includePatterns {
		cmd := fmt.Sprintf("ls %s%s", configPath, pattern)
		output, err := h.sshManager.Execute(&server, cmd)
		if err != nil {
			continue // 该模式无匹配，跳过
		}
		for _, f := range splitLines(output) {
			if f == "" {
				continue
			}
			if idx := lastIndexOf(f, '/'); idx >= 0 {
				f = f[idx+1:]
			}
			if !seen[f] {
				seen[f] = true
				fileNames = append(fileNames, f)
			}
		}
	}

	// 过滤排除模式
	if len(excludePatterns) > 0 {
		filtered := make([]string, 0)
		for _, f := range fileNames {
			excluded := false
			for _, ep := range excludePatterns {
				if matched, _ := filepath.Match(ep, f); matched {
					excluded = true
					break
				}
			}
			if !excluded {
				filtered = append(filtered, f)
			}
		}
		fileNames = filtered
	}

	c.JSON(http.StatusOK, fileNames)
}

func (h *NginxHandler) Upstreams(c *gin.Context) {
	serverID := c.Query("server_id")
	configFile := c.Query("config_file")

	if serverID == "" || configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// 确保路径以 / 结尾
	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	cmd := fmt.Sprintf("cat %s%s", configPath, configFile)
	output, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v, 输出: %s", err, output)})
		return
	}

	upstreams := h.nginxService.ParseConfig(output)
	c.JSON(http.StatusOK, gin.H{
		"upstreams": upstreams,
		"raw":       output,
	})
}

func (h *NginxHandler) OnlinePreview(c *gin.Context) {
	var req NginxUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	// 支持多个 IP，用逗号分隔
	backendIPs := splitIPs(req.BackendIP)

	// 生成整个文件的 diff
	currentConfig := config
	for _, upstreamName := range req.UpstreamNames {
		for _, ip := range backendIPs {
			_, after := h.nginxService.GenerateDiff(currentConfig, upstreamName, ip, "online")
			currentConfig = after
		}
	}

	// 生成逐行 diff
	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "online", req.ServerID, map[string]interface{}{
		"config_file":    req.ConfigFile,
		"upstream_names": req.UpstreamNames,
		"backend_ip":     req.BackendIP,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": fmt.Sprintf("将 %s 在 %v 中上线（去掉注释）", req.BackendIP, req.UpstreamNames),
	})
}

func (h *NginxHandler) OnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeNginxAction(c, req.PreviewID, "online")
}

func (h *NginxHandler) OfflinePreview(c *gin.Context) {
	var req NginxUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	// 支持多个 IP，用逗号分隔
	backendIPs := splitIPs(req.BackendIP)

	// 校验：不允许整个 upstream 组的所有服务器都被下线
	upstreams := h.nginxService.ParseConfig(config)
	for _, upstreamName := range req.UpstreamNames {
		for _, u := range upstreams {
			if u.Name != upstreamName {
				continue
			}
			// 统计当前在线的服务器数量
			upCount := 0
			for _, s := range u.Servers {
				if s.Status == "up" {
					upCount++
				}
			}
			// 统计本次要下线的、属于该 upstream 且当前在线的服务器数量
			willDown := 0
			for _, s := range u.Servers {
				if s.Status == "up" {
					for _, ip := range backendIPs {
						serverAddr := s.IP
						if s.Port != "" && s.Port != "80" {
							serverAddr = s.IP + ":" + s.Port
						}
						if serverAddr == ip || s.IP == ip {
							willDown++
							break
						}
					}
				}
			}
			if upCount > 0 && willDown >= upCount {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("禁止操作：upstream [%s] 中所有在线服务器都将被下线，至少需要保留一台在线服务器", upstreamName),
				})
				return
			}
		}
	}

	// 生成整个文件的 diff
	currentConfig := config
	for _, upstreamName := range req.UpstreamNames {
		for _, ip := range backendIPs {
			_, after := h.nginxService.GenerateDiff(currentConfig, upstreamName, ip, "offline")
			currentConfig = after
		}
	}

	// 生成逐行 diff
	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "offline", req.ServerID, map[string]interface{}{
		"config_file":    req.ConfigFile,
		"upstream_names": req.UpstreamNames,
		"backend_ip":     req.BackendIP,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": fmt.Sprintf("将 %s 在 %v 中下线（添加注释）", req.BackendIP, req.UpstreamNames),
	})
}

func (h *NginxHandler) OfflineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeNginxAction(c, req.PreviewID, "offline")
}

func (h *NginxHandler) SwapPreview(c *gin.Context) {
	var req NginxSwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	// 规范化 IP：去掉默认端口 :80
	offlineIP := normalizeIP(req.OfflineIP)
	onlineIP := normalizeIP(req.OnlineIP)

	// 校验每个 upstream
	upstreams := h.nginxService.ParseConfig(config)
	for _, upstreamName := range req.UpstreamNames {
		var targetUpstream *service.NginxUpstream
		for _, u := range upstreams {
			if u.Name == upstreamName {
				targetUpstream = &u
				break
			}
		}
		if targetUpstream == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", upstreamName)})
			return
		}

		offlineFound, onlineFound := false, false
		for _, s := range targetUpstream.Servers {
			addr := normalizeIP(s.IP + ":" + s.Port)
			if addr == offlineIP || s.IP == offlineIP {
				if s.Status != "up" {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 当前不是在线状态，无法下线", upstreamName, offlineIP)})
					return
				}
				offlineFound = true
			}
			if addr == onlineIP || s.IP == onlineIP {
				if s.Status != "down" {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 当前不是离线状态，无法上线", upstreamName, onlineIP)})
					return
				}
				onlineFound = true
			}
		}
		if !offlineFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中未找到服务器 %s", upstreamName, offlineIP)})
			return
		}
		if !onlineFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中未找到服务器 %s", upstreamName, onlineIP)})
			return
		}
	}

	// 生成整个文件的 diff（依次对每个 upstream 执行切换）
	currentConfig := config
	for _, upstreamName := range req.UpstreamNames {
		_, currentConfig = h.nginxService.GenerateSwapDiff(currentConfig, upstreamName, offlineIP, onlineIP)
	}
	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "swap", req.ServerID, map[string]interface{}{
		"config_file":    req.ConfigFile,
		"upstream_names": req.UpstreamNames,
		"offline_ip":     offlineIP,
		"online_ip":      onlineIP,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": fmt.Sprintf("切换 %v: %s 下线 → %s 上线", req.UpstreamNames, offlineIP, onlineIP),
	})
}

func (h *NginxHandler) SwapExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	preview, ok := h.previewMgr.Get(req.PreviewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "nginx" || preview.Action != "swap" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	configFile := params["config_file"].(string)
	offlineIP := params["offline_ip"].(string)
	onlineIP := params["online_ip"].(string)

	// upstream_names 可能是 []string 或 []interface{}，统一转为 []string
	var upstreamNames []string
	switch v := params["upstream_names"].(type) {
	case []string:
		upstreamNames = v
	case []interface{}:
		for _, item := range v {
			upstreamNames = append(upstreamNames, item.(string))
		}
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	// 备份
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, server.BackupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)

	// 清理旧备份
	backupPath := server.BackupPath
	if backupPath != "" && backupPath[len(backupPath)-1] != '/' {
		backupPath += "/"
	}
	cleanupCmd := fmt.Sprintf("cd %s && ls -t %s.bak.* 2>/dev/null | tail -n +%d | xargs -r rm -f", backupPath, configFile, maxBackups+1)
	h.sshManager.Execute(&server, cleanupCmd)

	// 对每个 upstream 执行切换
	var allCommands []string
	for _, upstreamName := range upstreamNames {
		commands := h.nginxService.GenerateSwapModifyCommands(configPath, configFile, upstreamName, offlineIP, onlineIP)
		allCommands = append(allCommands, commands...)
		for _, cmd := range commands {
			_, err := h.sshManager.Execute(&server, cmd)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
				return
			}
		}
	}

	// 测试并重载
	testOutput, err := h.sshManager.Execute(&server, nginxCmd+" -t")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误，请检查: %s", testOutput)})
		return
	}

	h.sshManager.Execute(&server, "systemctl reload nginx")
	h.sshManager.CloseServer(server.ID)

	logOutput := fmt.Sprintf("切换成功: %v %s 下线 → %s 上线", upstreamNames, offlineIP, onlineIP)
	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "nginx",
		Action:     "swap",
		Target:     fmt.Sprintf("%s %v %s->%s", configFile, upstreamNames, offlineIP, onlineIP),
		Detail:     strings.Join(allCommands, "\n"),
		PreviewID:  req.PreviewID,
		Status:     "success",
		Output:     logOutput,
		ServerID:   server.ID,
		ServerName: server.Name,
	}
	h.db.Create(&logEntry)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "切换成功", "output": logOutput})
}

func (h *NginxHandler) BatchPreview(c *gin.Context) {
	var req NginxBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.Swaps) == 0 && len(req.Offlines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少选择一个操作"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	upstreams := h.nginxService.ParseConfig(config)
	upstreamMap := make(map[string]*service.NginxUpstream)
	for i := range upstreams {
		upstreamMap[upstreams[i].Name] = &upstreams[i]
	}

	// 校验并生成 diff
	currentConfig := config
	var descriptions []string

	for _, item := range req.Swaps {
		offlineIP := normalizeIP(item.OfflineIP)
		onlineIP := normalizeIP(item.OnlineIP)
		u, ok := upstreamMap[item.UpstreamName]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", item.UpstreamName)})
			return
		}
		if !validateServerStatus(u, offlineIP, "up") {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 不在线", item.UpstreamName, offlineIP)})
			return
		}
		if !validateServerStatus(u, onlineIP, "down") {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 不离线", item.UpstreamName, onlineIP)})
			return
		}
		_, currentConfig = h.nginxService.GenerateSwapDiff(currentConfig, item.UpstreamName, offlineIP, onlineIP)
		descriptions = append(descriptions, fmt.Sprintf("[%s] %s↔%s 切换", item.UpstreamName, offlineIP, onlineIP))
	}

	for _, item := range req.Offlines {
		offlineIP := normalizeIP(item.OfflineIP)
		u, ok := upstreamMap[item.UpstreamName]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", item.UpstreamName)})
			return
		}
		if !validateServerStatus(u, offlineIP, "up") {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 不在线", item.UpstreamName, offlineIP)})
			return
		}
		_, currentConfig = h.nginxService.GenerateDiff(currentConfig, item.UpstreamName, offlineIP, "offline")
		descriptions = append(descriptions, fmt.Sprintf("[%s] %s 下线", item.UpstreamName, offlineIP))
	}

	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "batch", req.ServerID, map[string]interface{}{
		"config_file": req.ConfigFile,
		"swaps":       req.Swaps,
		"offlines":    req.Offlines,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": "批量操作：" + strings.Join(descriptions, "；"),
	})
}

func (h *NginxHandler) BatchExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	preview, ok := h.previewMgr.Get(req.PreviewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "nginx" || preview.Action != "batch" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	configFile := params["config_file"].(string)

	// 解析 swaps
	var swaps []NginxBatchSwapItem
	for _, item := range params["swaps"].([]interface{}) {
		m := item.(map[string]interface{})
		swaps = append(swaps, NginxBatchSwapItem{
			UpstreamName: m["upstream_name"].(string),
			OfflineIP:    normalizeIP(m["offline_ip"].(string)),
			OnlineIP:     normalizeIP(m["online_ip"].(string)),
		})
	}

	// 解析 offlines
	var offlines []NginxBatchSwapItem
	if rawOfflines, ok := params["offlines"]; ok && rawOfflines != nil {
		for _, item := range rawOfflines.([]interface{}) {
			m := item.(map[string]interface{})
			offlines = append(offlines, NginxBatchSwapItem{
				UpstreamName: m["upstream_name"].(string),
				OfflineIP:    normalizeIP(m["offline_ip"].(string)),
			})
		}
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	// 备份
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, server.BackupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)

	backupPath := server.BackupPath
	if backupPath != "" && backupPath[len(backupPath)-1] != '/' {
		backupPath += "/"
	}
	cleanupCmd := fmt.Sprintf("cd %s && ls -t %s.bak.* 2>/dev/null | tail -n +%d | xargs -r rm -f", backupPath, configFile, maxBackups+1)
	h.sshManager.Execute(&server, cleanupCmd)

	// 执行切换操作
	var allCommands []string
	for _, item := range swaps {
		commands := h.nginxService.GenerateSwapModifyCommands(configPath, configFile, item.UpstreamName, item.OfflineIP, item.OnlineIP)
		allCommands = append(allCommands, commands...)
		for _, cmd := range commands {
			if _, err := h.sshManager.Execute(&server, cmd); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
				return
			}
		}
	}

	// 执行下线操作
	for _, item := range offlines {
		cmd := h.nginxService.GenerateModifyCommand(configPath, configFile, item.UpstreamName, []string{item.OfflineIP}, "offline")
		allCommands = append(allCommands, cmd)
		if _, err := h.sshManager.Execute(&server, cmd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
			return
		}
	}

	// 测试并重载
	testOutput, err := h.sshManager.Execute(&server, nginxCmd+" -t")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误，请检查: %s", testOutput)})
		return
	}

	h.sshManager.Execute(&server, "systemctl reload nginx")
	h.sshManager.CloseServer(server.ID)

	logOutput := fmt.Sprintf("批量操作成功：%d 个切换，%d 个下线", len(swaps), len(offlines))
	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "nginx",
		Action:     "batch",
		Target:     fmt.Sprintf("%s %d swaps, %d offlines", configFile, len(swaps), len(offlines)),
		Detail:     strings.Join(allCommands, "\n"),
		PreviewID:  req.PreviewID,
		Status:     "success",
		Output:     logOutput,
		ServerID:   server.ID,
		ServerName: server.Name,
	}
	h.db.Create(&logEntry)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "批量操作成功", "output": logOutput})
}

// validateServerStatus 校验指定 IP 的服务器状态
func validateServerStatus(upstream *service.NginxUpstream, ip, expectedStatus string) bool {
	for _, s := range upstream.Servers {
		addr := normalizeIP(s.IP + ":" + s.Port)
		if addr == ip || s.IP == ip {
			return s.Status == expectedStatus
		}
	}
	return false
}

func (h *NginxHandler) Reload(c *gin.Context) {
	var req NginxReloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// Test config first
	testCmd := nginxCmd + " -t"
	testOutput, err := h.sshManager.Execute(&server, testCmd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误: %s", testOutput)})
		return
	}

	// Reload
	reloadCmd := "systemctl reload nginx"
	_, err = h.sshManager.Execute(&server, reloadCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("reload失败: %v", err)})
		return
	}

	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "nginx",
		Action:     "reload",
		Target:     server.Name,
		Detail:     fmt.Sprintf("%s && %s", testCmd, reloadCmd),
		Status:     "success",
		Output:     fmt.Sprintf("%s\nnginx reload 成功", testOutput),
		ServerID:   server.ID,
		ServerName: server.Name,
	}
	h.db.Create(&logEntry)

	c.JSON(http.StatusOK, gin.H{"message": "reload成功"})
}

func (h *NginxHandler) RollbackPreview(c *gin.Context) {
	var req NginxRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// Read current config
	currentCmd := fmt.Sprintf("cat %s/%s", server.ConfigPath, req.ConfigFile)
	currentConfig, err := h.sshManager.Execute(&server, currentCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取当前配置失败: %v", err)})
		return
	}

	// Read backup config
	backupCmd := fmt.Sprintf("cat %s/%s", server.BackupPath, req.BackupFile)
	backupConfig, err := h.sshManager.Execute(&server, backupCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取备份配置失败: %v", err)})
		return
	}

	previewID := h.previewMgr.Create("nginx", "rollback", req.ServerID, map[string]interface{}{
		"config_file": req.ConfigFile,
		"backup_file": req.BackupFile,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      currentConfig,
		"after":       backupConfig,
		"description": fmt.Sprintf("回滚 %s 到备份 %s", req.ConfigFile, req.BackupFile),
	})
}

func (h *NginxHandler) RollbackExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	preview, ok := h.previewMgr.Get(req.PreviewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "nginx" || preview.Action != "rollback" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	configFile := params["config_file"].(string)
	backupFile := params["backup_file"].(string)

	// Copy backup to config
	copyCmd := fmt.Sprintf("cp %s/%s %s/%s", server.BackupPath, backupFile, server.ConfigPath, configFile)
	_, err := h.sshManager.Execute(&server, copyCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("回滚失败: %v", err)})
		return
	}

	// Test and reload
	testCmd := nginxCmd + " -t"
	testOutput, err := h.sshManager.Execute(&server, testCmd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误: %s", testOutput)})
		return
	}

	reloadCmd := "systemctl reload nginx"
	h.sshManager.Execute(&server, reloadCmd)

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	detail := fmt.Sprintf("%s\n%s && %s", copyCmd, testCmd, reloadCmd)
	logOutput := fmt.Sprintf("%s\n回滚成功: %s -> %s", testOutput, backupFile, configFile)

	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "nginx",
		Action:     "rollback",
		Target:     fmt.Sprintf("%s -> %s", configFile, backupFile),
		Detail:     detail,
		PreviewID:  req.PreviewID,
		Status:     "success",
		Output:     logOutput,
		ServerID:   server.ID,
		ServerName: server.Name,
	}
	h.db.Create(&logEntry)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "回滚成功"})
}

func (h *NginxHandler) Backups(c *gin.Context) {
	serverID := c.Query("server_id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定服务器"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	cmd := fmt.Sprintf("ls -t %s", server.BackupPath)
	output, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取备份列表失败: %v", err)})
		return
	}

	files := splitLines(output)
	c.JSON(http.StatusOK, files)
}

func (h *NginxHandler) executeNginxAction(c *gin.Context, previewID, action string) {
	preview, ok := h.previewMgr.Get(previewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "nginx" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	configFile := params["config_file"].(string)
	backendIP := params["backend_ip"].(string)

	// upstream_names 可能是 []string 或 []interface{}，统一转为 []string
	var upstreamNames []string
	switch v := params["upstream_names"].(type) {
	case []string:
		upstreamNames = v
	case []interface{}:
		for _, item := range v {
			upstreamNames = append(upstreamNames, item.(string))
		}
	}

	configPath := server.ConfigPath
	if configPath != "" && configPath[len(configPath)-1] != '/' {
		configPath += "/"
	}

	// Backup first
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, server.BackupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)

	// 清理旧备份，保留最近 maxBackups 个
	backupPath := server.BackupPath
	if backupPath != "" && backupPath[len(backupPath)-1] != '/' {
		backupPath += "/"
	}
	cleanupCmd := fmt.Sprintf("cd %s && ls -t %s.bak.* 2>/dev/null | tail -n +%d | xargs -r rm -f", backupPath, configFile, maxBackups+1)
	h.sshManager.Execute(&server, cleanupCmd)

	// 支持多个 IP，用逗号分隔
	backendIPs := splitIPs(backendIP)

	// Execute changes - 每个 upstream 用一条 sed 命令处理所有 IP
	var commands []string
	var lastErr error
	for _, upstreamName := range upstreamNames {
		cmd := h.nginxService.GenerateModifyCommand(configPath, configFile, upstreamName, backendIPs, action)
		commands = append(commands, cmd)
		_, err := h.sshManager.Execute(&server, cmd)
		if err != nil {
			lastErr = err
			break
		}
	}

	if lastErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", lastErr)})
		return
	}

	// Test and reload
	testOutput, err := h.sshManager.Execute(&server, nginxCmd + " -t")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误，请检查: %s", testOutput)})
		return
	}

	h.sshManager.Execute(&server, "systemctl reload nginx")

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	// sed -i 无输出，生成有意义的操作摘要
	actionDesc := "上线"
	if action == "offline" {
		actionDesc = "下线"
	}
	logOutput := fmt.Sprintf("成功将 %s 在 %v 中%s", backendIP, upstreamNames, actionDesc)

	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "nginx",
		Action:     action,
		Target:     fmt.Sprintf("%s %v %s", configFile, upstreamNames, backendIP),
		Detail:     strings.Join(commands, "\n"),
		PreviewID:  previewID,
		Status:     "success",
		Output:     logOutput,
		ServerID:   server.ID,
		ServerName: server.Name,
	}
	h.db.Create(&logEntry)
	h.previewMgr.Delete(previewID)

	c.JSON(http.StatusOK, gin.H{"message": action + "成功", "output": logOutput})
}

// splitIPs 分割逗号分隔的 IP 列表
func splitIPs(ips string) []string {
	var result []string
	for _, ip := range strings.Split(ips, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			result = append(result, ip)
		}
	}
	return result
}

// normalizeIP 规范化 IP 地址，去掉默认端口 :80
func normalizeIP(ip string) string {
	if strings.HasSuffix(ip, ":80") {
		return ip[:len(ip)-3]
	}
	return ip
}

func lastIndexOf(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
