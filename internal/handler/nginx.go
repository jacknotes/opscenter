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
	db           *gorm.DB
	sshManager   *service.SSHManager
	previewMgr   *service.PreviewManager
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
	ServerID      uint     `json:"server_id" binding:"required"`
	ConfigFile    string   `json:"config_file" binding:"required"`
	UpstreamNames []string `json:"upstream_names" binding:"required"`
	BackendIP     string   `json:"backend_ip" binding:"required"`
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

type NginxToggleRequest struct {
	ServerID      uint     `json:"server_id" binding:"required"`
	ConfigFile    string   `json:"config_file" binding:"required"`
	UpstreamNames []string `json:"upstream_names" binding:"required"`
}

type NginxBatchItem struct {
	UpstreamName string `json:"upstream_name"`
	Action       string `json:"action"`               // "online", "offline", "toggle"
	BackendIP    string `json:"backend_ip,omitempty"` // online/offline 时需要
}

type NginxBatchRequestV2 struct {
	ServerID   uint             `json:"server_id" binding:"required"`
	ConfigFile string           `json:"config_file" binding:"required"`
	Items      []NginxBatchItem `json:"items" binding:"required"`
}

func ensureTrailingSlash(path string) string {
	if path != "" && path[len(path)-1] != '/' {
		return path + "/"
	}
	return path
}

func extractStringSlice(params map[string]interface{}, key string) ([]string, error) {
	val, ok := params[key]
	if !ok {
		return nil, nil
	}
	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s 元素类型错误", key)
			}
			result = append(result, s)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s 类型异常: %T", key, val)
	}
}

// validateConfigFile 校验配置文件名，防止路径穿越和命令注入
func validateConfigFile(file string) error {
	if !service.ValidateFilePath(file) {
		return fmt.Errorf("非法的配置文件名: %s", file)
	}
	return nil
}

// Configs godoc
//
//	@Summary		获取 Nginx 配置文件列表
//	@Description	获取指定 Nginx 服务器的配置文件列表
//	@Tags			Nginx
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{array}		string
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Router			/nginx/configs [get]
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

	configPath := ensureTrailingSlash(server.ConfigPath)

	configPattern := server.ConfigPattern
	if configPattern == "" {
		configPattern = "*.conf"
	}

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
			if idx := strings.LastIndexByte(f, '/'); idx >= 0 {
				f = f[idx+1:]
			}
			if !seen[f] {
				seen[f] = true
				fileNames = append(fileNames, f)
			}
		}
	}

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

// Upstreams godoc
//
//	@Summary		获取 Nginx upstream 列表
//	@Description	获取指定配置文件中的 upstream 及其服务器列表
//	@Tags			Nginx
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Param			config_file	query		string	true	"配置文件名"
//	@Success		200			{object}	object
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/nginx/upstreams [get]
func (h *NginxHandler) Upstreams(c *gin.Context) {
	serverID := c.Query("server_id")
	configFile := c.Query("config_file")

	if serverID == "" || configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	if err := validateConfigFile(configFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// 确保路径以 / 结尾
	configPath := ensureTrailingSlash(server.ConfigPath)

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

// OnlinePreview godoc
//
//	@Summary		Nginx 上线预览
//	@Description	预览 Nginx upstream 后端服务器上线操作（去掉注释）
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxUpstreamRequest	true	"操作参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/online/preview [post]
func (h *NginxHandler) OnlinePreview(c *gin.Context) {
	var req NginxUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	backendIPs := splitIPs(req.BackendIP)

	currentConfig := config
	for _, upstreamName := range req.UpstreamNames {
		for _, ip := range backendIPs {
			_, after := h.nginxService.GenerateDiff(currentConfig, upstreamName, ip, "online")
			currentConfig = after
		}
	}

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

// OnlineExecute godoc
//
//	@Summary		执行 Nginx 上线
//	@Description	根据预览 ID 执行 Nginx upstream 后端服务器上线操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/online/execute [post]
func (h *NginxHandler) OnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeNginxAction(c, req.PreviewID, "online")
}

// OfflinePreview godoc
//
//	@Summary		Nginx 下线预览
//	@Description	预览 Nginx upstream 后端服务器下线操作（添加注释）
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxUpstreamRequest	true	"操作参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/offline/preview [post]
func (h *NginxHandler) OfflinePreview(c *gin.Context) {
	var req NginxUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	backendIPs := splitIPs(req.BackendIP)

	upstreams := h.nginxService.ParseConfig(config)
	for _, upstreamName := range req.UpstreamNames {
		for _, u := range upstreams {
			if u.Name != upstreamName {
				continue
			}
			upCount := 0
			for _, s := range u.Servers {
				if s.Status == "up" {
					upCount++
				}
			}
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

// OfflineExecute godoc
//
//	@Summary		执行 Nginx 下线
//	@Description	根据预览 ID 执行 Nginx upstream 后端服务器下线操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/offline/execute [post]
func (h *NginxHandler) OfflineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeNginxAction(c, req.PreviewID, "offline")
}

// SwapPreview godoc
//
//	@Summary		Nginx 切换预览
//	@Description	预览 Nginx upstream 后端服务器切换操作（一上一下）
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxSwapRequest	true	"切换参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/swap/preview [post]
func (h *NginxHandler) SwapPreview(c *gin.Context) {
	var req NginxSwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	offlineIP := normalizeIP(req.OfflineIP)
	onlineIP := normalizeIP(req.OnlineIP)

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

// SwapExecute godoc
//
//	@Summary		执行 Nginx 切换
//	@Description	根据预览 ID 执行 Nginx upstream 后端服务器切换操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/swap/execute [post]
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
	configFile, _ := params["config_file"].(string)
	offlineIP, _ := params["offline_ip"].(string)
	onlineIP, _ := params["online_ip"].(string)
	if configFile == "" || offlineIP == "" || onlineIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}

	upstreamNames, err := extractStringSlice(params, "upstream_names")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)
	backupPath := ensureTrailingSlash(server.BackupPath)
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, backupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)
	cleanupCmd := h.nginxService.GenerateCleanupCommand(backupPath, configFile, maxBackups)
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
	createAuditLog(h.db, c, "nginx", "swap",
		fmt.Sprintf("%s %v %s->%s", configFile, upstreamNames, offlineIP, onlineIP),
		strings.Join(allCommands, "\n"), "success", logOutput, server.ID, server.Name)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "切换成功", "output": logOutput})
}

// TogglePreview godoc
//
//	@Summary		Nginx 状态切换预览
//	@Description	预览 Nginx upstream 中所有服务器状态反转操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxToggleRequest	true	"操作参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/toggle/preview [post]
func (h *NginxHandler) TogglePreview(c *gin.Context) {
	var req NginxToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)

	cmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	config, err := h.sshManager.Execute(&server, cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}

	// 校验每个 upstream 同时存在 up 和 down 的 server
	upstreams := h.nginxService.ParseConfig(config)
	upstreamMap := make(map[string]*service.NginxUpstream)
	for i := range upstreams {
		upstreamMap[upstreams[i].Name] = &upstreams[i]
	}
	for _, name := range req.UpstreamNames {
		u, ok := upstreamMap[name]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", name)})
			return
		}
		hasUp, hasDown := false, false
		for _, s := range u.Servers {
			if s.Status == "up" {
				hasUp = true
			} else {
				hasDown = true
			}
		}
		if !hasUp || !hasDown {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中所有服务器状态相同，无需切换", name)})
			return
		}
	}

	// 依次对每个 upstream 生成 toggle diff
	currentConfig := config
	for _, name := range req.UpstreamNames {
		_, currentConfig = h.nginxService.GenerateToggleDiff(currentConfig, name)
	}
	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "toggle", req.ServerID, map[string]interface{}{
		"config_file":    req.ConfigFile,
		"upstream_names": req.UpstreamNames,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": fmt.Sprintf("切换 %v 中所有服务器状态", req.UpstreamNames),
	})
}

// ToggleExecute godoc
//
//	@Summary		执行 Nginx 状态切换
//	@Description	根据预览 ID 执行 Nginx upstream 服务器状态反转操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/toggle/execute [post]
func (h *NginxHandler) ToggleExecute(c *gin.Context) {
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

	if preview.Module != "nginx" || preview.Action != "toggle" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	configFile, _ := params["config_file"].(string)
	if configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}

	upstreamNames, err := extractStringSlice(params, "upstream_names")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)
	backupPath := ensureTrailingSlash(server.BackupPath)
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, backupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)
	cleanupCmd := h.nginxService.GenerateCleanupCommand(backupPath, configFile, maxBackups)
	h.sshManager.Execute(&server, cleanupCmd)

	// 读取配置并解析 upstream 的 server 列表
	catCmd := fmt.Sprintf("cat %s%s", configPath, configFile)
	configContent, err := h.sshManager.Execute(&server, catCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}
	parsed := h.nginxService.ParseConfig(configContent)
	upstreamMap := make(map[string]*service.NginxUpstream)
	for i := range parsed {
		upstreamMap[parsed[i].Name] = &parsed[i]
	}

	// 对每个 upstream 执行切换
	var allCommands []string
	for _, upstreamName := range upstreamNames {
		u, ok := upstreamMap[upstreamName]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", upstreamName)})
			return
		}
		commands := h.nginxService.GenerateToggleModifyCommands(configPath, configFile, upstreamName, u.Servers)
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

	logOutput := fmt.Sprintf("切换成功: %v 中所有服务器状态已反转", upstreamNames)
	createAuditLog(h.db, c, "nginx", "toggle",
		fmt.Sprintf("%s %v", configFile, upstreamNames),
		strings.Join(allCommands, "\n"), "success", logOutput, server.ID, server.Name)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "切换成功", "output": logOutput})
}

// BatchPreview godoc
//
//	@Summary		Nginx 批量操作预览
//	@Description	预览 Nginx upstream 的批量操作（上线/下线/切换混合）
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxBatchRequestV2	true	"批量操作参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/batch/preview [post]
func (h *NginxHandler) BatchPreview(c *gin.Context) {
	var req NginxBatchRequestV2
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少选择一个操作"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)

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

	// 统计每个 upstream 被下线的服务器数量，防止累积下线导致全部离线
	offlineCountMap := make(map[string]int)
	for _, item := range req.Items {
		if item.Action == "offline" {
			offlineCountMap[item.UpstreamName]++
		}
	}
	// 校验累积下线数量
	for upstreamName, offlineCount := range offlineCountMap {
		u, ok := upstreamMap[upstreamName]
		if !ok {
			continue
		}
		upCount := 0
		for _, s := range u.Servers {
			if s.Status == "up" {
				upCount++
			}
		}
		if offlineCount >= upCount {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中所有在线服务器都将被下线（%d 台下线 / %d 台在线），至少需要保留一台在线服务器", upstreamName, offlineCount, upCount)})
			return
		}
	}

	// 校验并生成 diff（按顺序处理每个 item，前一个的 after 是下一个的 before）
	currentConfig := config
	var descriptions []string

	for _, item := range req.Items {
		u, ok := upstreamMap[item.UpstreamName]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", item.UpstreamName)})
			return
		}

		switch item.Action {
		case "online":
			backendIP := normalizeIP(item.BackendIP)
			if !validateServerStatus(u, backendIP, "down") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 不在离线状态", item.UpstreamName, backendIP)})
				return
			}
			_, currentConfig = h.nginxService.GenerateDiff(currentConfig, item.UpstreamName, backendIP, "online")
			descriptions = append(descriptions, fmt.Sprintf("[%s] %s 上线", item.UpstreamName, backendIP))

		case "offline":
			backendIP := normalizeIP(item.BackendIP)
			if !validateServerStatus(u, backendIP, "up") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中服务器 %s 不在在线状态", item.UpstreamName, backendIP)})
				return
			}
			_, currentConfig = h.nginxService.GenerateDiff(currentConfig, item.UpstreamName, backendIP, "offline")
			descriptions = append(descriptions, fmt.Sprintf("[%s] %s 下线", item.UpstreamName, backendIP))

		case "toggle":
			hasUp, hasDown := false, false
			for _, s := range u.Servers {
				if s.Status == "up" {
					hasUp = true
				} else {
					hasDown = true
				}
			}
			if !hasUp || !hasDown {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 中所有服务器状态相同，无需切换", item.UpstreamName)})
				return
			}
			_, currentConfig = h.nginxService.GenerateToggleDiff(currentConfig, item.UpstreamName)
			descriptions = append(descriptions, fmt.Sprintf("[%s] 切换所有状态", item.UpstreamName))

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的操作类型: %s", item.Action)})
			return
		}
	}

	lineDiffs := h.nginxService.GenerateLineDiffs(config, currentConfig)

	previewID := h.previewMgr.Create("nginx", "batch", req.ServerID, map[string]interface{}{
		"config_file": req.ConfigFile,
		"items":       req.Items,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":  previewID,
		"before":      config,
		"after":       currentConfig,
		"line_diffs":  lineDiffs,
		"description": "批量操作：" + strings.Join(descriptions, "；"),
	})
}

// BatchExecute godoc
//
//	@Summary		执行 Nginx 批量操作
//	@Description	根据预览 ID 执行 Nginx upstream 的批量操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/upstream/batch/execute [post]
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
	configFile, _ := params["config_file"].(string)
	if configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}

	// 解析 items
	var items []NginxBatchItem
	rawItems, ok := params["items"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览数据中缺少 items"})
		return
	}
	switch v := rawItems.(type) {
	case []NginxBatchItem:
		items = v
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			upstreamName, _ := m["upstream_name"].(string)
			action, _ := m["action"].(string)
			bi := NginxBatchItem{
				UpstreamName: upstreamName,
				Action:       action,
			}
			if ip, ok := m["backend_ip"].(string); ok {
				bi.BackendIP = normalizeIP(ip)
			}
			items = append(items, bi)
		}
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("items 类型异常: %T", rawItems)})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)
	backupPath := ensureTrailingSlash(server.BackupPath)
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, backupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)
	cleanupCmd := h.nginxService.GenerateCleanupCommand(backupPath, configFile, maxBackups)
	h.sshManager.Execute(&server, cleanupCmd)

	// 读取配置用于 toggle 操作获取 server 列表
	catCmd := fmt.Sprintf("cat %s%s", configPath, configFile)
	configContent, err := h.sshManager.Execute(&server, catCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取配置失败: %v", err)})
		return
	}
	parsed := h.nginxService.ParseConfig(configContent)
	upstreamMap := make(map[string]*service.NginxUpstream)
	for i := range parsed {
		upstreamMap[parsed[i].Name] = &parsed[i]
	}

	// 按顺序执行每个操作
	var allCommands []string
	var actionCounts = map[string]int{"online": 0, "offline": 0, "toggle": 0}

	for _, item := range items {
		switch item.Action {
		case "online":
			cmd := h.nginxService.GenerateModifyCommand(configPath, configFile, item.UpstreamName, []string{item.BackendIP}, "online")
			allCommands = append(allCommands, cmd)
			if _, err := h.sshManager.Execute(&server, cmd); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("[%s] %s 上线失败: %v", item.UpstreamName, item.BackendIP, err)})
				return
			}
			actionCounts["online"]++

		case "offline":
			cmd := h.nginxService.GenerateModifyCommand(configPath, configFile, item.UpstreamName, []string{item.BackendIP}, "offline")
			allCommands = append(allCommands, cmd)
			if _, err := h.sshManager.Execute(&server, cmd); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("[%s] %s 下线失败: %v", item.UpstreamName, item.BackendIP, err)})
				return
			}
			actionCounts["offline"]++

		case "toggle":
			u, ok := upstreamMap[item.UpstreamName]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upstream [%s] 不存在", item.UpstreamName)})
				return
			}
			commands := h.nginxService.GenerateToggleModifyCommands(configPath, configFile, item.UpstreamName, u.Servers)
			allCommands = append(allCommands, commands...)
			for _, cmd := range commands {
				if _, err := h.sshManager.Execute(&server, cmd); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("[%s] 切换失败: %v (命令: %s)", item.UpstreamName, err, cmd)})
					return
				}
			}
			actionCounts["toggle"]++
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

	logOutput := fmt.Sprintf("批量操作成功：%d 个上线，%d 个下线，%d 个切换", actionCounts["online"], actionCounts["offline"], actionCounts["toggle"])
	createAuditLog(h.db, c, "nginx", "batch",
		fmt.Sprintf("%s %d items", configFile, len(items)),
		strings.Join(allCommands, "\n"), "success", logOutput, server.ID, server.Name)
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

// Reload godoc
//
//	@Summary		重载 Nginx 配置
//	@Description	测试并重载指定 Nginx 服务器的配置
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxReloadRequest	true	"服务器 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/reload [post]
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

	createAuditLog(h.db, c, "nginx", "reload",
		server.Name,
		fmt.Sprintf("%s && %s", testCmd, reloadCmd), "success",
		fmt.Sprintf("%s\nnginx reload 成功", testOutput), server.ID, server.Name)

	c.JSON(http.StatusOK, gin.H{"message": "reload成功"})
}

// RollbackPreview godoc
//
//	@Summary		Nginx 回滚预览
//	@Description	预览 Nginx 配置文件回滚操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		NginxRollbackRequest	true	"回滚参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/rollback/preview [post]
func (h *NginxHandler) RollbackPreview(c *gin.Context) {
	var req NginxRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateConfigFile(req.ConfigFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateConfigFile(req.BackupFile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的备份文件名"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)
	currentCmd := fmt.Sprintf("cat %s%s", configPath, req.ConfigFile)
	currentConfig, err := h.sshManager.Execute(&server, currentCmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取当前配置失败: %v", err)})
		return
	}

	backupPath := ensureTrailingSlash(server.BackupPath)
	backupCmd := fmt.Sprintf("cat %s%s", backupPath, req.BackupFile)
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

// RollbackExecute godoc
//
//	@Summary		执行 Nginx 回滚
//	@Description	根据预览 ID 执行 Nginx 配置文件回滚操作
//	@Tags			Nginx
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/nginx/rollback/execute [post]
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
	configFile, _ := params["config_file"].(string)
	if configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}
	backupFile, _ := params["backup_file"].(string)
	if backupFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}

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

	createAuditLog(h.db, c, "nginx", "rollback",
		fmt.Sprintf("%s -> %s", configFile, backupFile),
		detail, "success", logOutput, server.ID, server.Name)
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"message": "回滚成功"})
}

// Backups godoc
//
//	@Summary		获取 Nginx 备份列表
//	@Description	获取指定 Nginx 服务器的配置备份文件列表
//	@Tags			Nginx
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{array}		string
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/nginx/backups [get]
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

	cmd := fmt.Sprintf("ls -t %s 2>/dev/null", server.BackupPath)
	output, _ := h.sshManager.Execute(&server, cmd)

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
	configFile, _ := params["config_file"].(string)
	if configFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}
	backendIP, _ := params["backend_ip"].(string)

	upstreamNames, err := extractStringSlice(params, "upstream_names")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configPath := ensureTrailingSlash(server.ConfigPath)
	backupPath := ensureTrailingSlash(server.BackupPath)
	backupCmd := h.nginxService.GenerateBackupCommand(configPath, backupPath, configFile)
	h.sshManager.Execute(&server, backupCmd)
	cleanupCmd := h.nginxService.GenerateCleanupCommand(backupPath, configFile, maxBackups)
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

	testOutput, err := h.sshManager.Execute(&server, nginxCmd+" -t")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("配置语法错误，请检查: %s", testOutput)})
		return
	}

	h.sshManager.Execute(&server, "systemctl reload nginx")

	h.sshManager.CloseServer(server.ID)

	actionDesc := "上线"
	if action == "offline" {
		actionDesc = "下线"
	}
	logOutput := fmt.Sprintf("成功将 %s 在 %v 中%s", backendIP, upstreamNames, actionDesc)

	createAuditLog(h.db, c, "nginx", action,
		fmt.Sprintf("%s %v %s", configFile, upstreamNames, backendIP),
		strings.Join(commands, "\n"), "success", logOutput, server.ID, server.Name)
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

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
