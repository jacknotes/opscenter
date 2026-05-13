package handler

import (
	"fmt"
	"net/http"
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

	cmd := fmt.Sprintf("ls %s%s", configPath, configPattern)
	output, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取配置列表失败: %v", err)})
		return
	}

	// 只返回文件名，不包含路径前缀
	files := splitLines(output)
	var fileNames []string
	for _, f := range files {
		if idx := lastIndexOf(f, '/'); idx >= 0 {
			fileNames = append(fileNames, f[idx+1:])
		} else {
			fileNames = append(fileNames, f)
		}
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
		Username: c.GetString("username"),
		Module:   "nginx",
		Action:   "reload",
		Target:   server.Name,
		Detail:   fmt.Sprintf("%s && %s", testCmd, reloadCmd),
		Status:   "success",
		Output:   fmt.Sprintf("%s\nnginx reload 成功", testOutput),
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

	detail := fmt.Sprintf("%s\n%s && %s", copyCmd, testCmd, reloadCmd)
	logOutput := fmt.Sprintf("%s\n回滚成功: %s -> %s", testOutput, backupFile, configFile)

	logEntry := model.OperationLog{
		Username:  c.GetString("username"),
		Module:    "nginx",
		Action:    "rollback",
		Target:    fmt.Sprintf("%s -> %s", configFile, backupFile),
		Detail:    detail,
		PreviewID: req.PreviewID,
		Status:    "success",
		Output:    logOutput,
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

	// sed -i 无输出，生成有意义的操作摘要
	actionDesc := "上线"
	if action == "offline" {
		actionDesc = "下线"
	}
	logOutput := fmt.Sprintf("成功将 %s 在 %v 中%s", backendIP, upstreamNames, actionDesc)

	logEntry := model.OperationLog{
		Username:  c.GetString("username"),
		Module:    "nginx",
		Action:    action,
		Target:    fmt.Sprintf("%s %v %s", configFile, upstreamNames, backendIP),
		Detail:    strings.Join(commands, "\n"),
		PreviewID: previewID,
		Status:    "success",
		Output:    logOutput,
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
