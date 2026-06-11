package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

func (h *ServerHandler) auditLog(c *gin.Context, action, target, detail, status, output string) {
	createAuditLog(h.db, c, "server", action, target, detail, status, output, 0, "")
}

type ServerHandler struct {
	db *gorm.DB
}

func NewServerHandler(db *gorm.DB) *ServerHandler {
	return &ServerHandler{db: db}
}

// validateServerFields 校验服务器的路径和配置模式字段，防止注入
func validateServerFields(s *model.Server) error {
	if s.ConfigPath != "" && !service.ValidateDirectoryPath(s.ConfigPath) {
		return fmt.Errorf("配置路径 [%s] 包含非法字符", s.ConfigPath)
	}
	if s.BackupPath != "" && !service.ValidateDirectoryPath(s.BackupPath) {
		return fmt.Errorf("备份路径 [%s] 包含非法字符", s.BackupPath)
	}
	if s.ConfigPattern != "" && !service.ValidateConfigPattern(s.ConfigPattern) {
		return fmt.Errorf("配置模式 [%s] 包含非法字符", s.ConfigPattern)
	}
	return nil
}

// List godoc
//
//	@Summary		获取服务器列表
//	@Description	获取服务器列表，可按类型和启用状态筛选
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			type	query		string	false	"服务器类型 (lvs/k8s/nginx/preprod)"
//	@Param			all		query		string	false	"是否显示全部 (true/false)"
//	@Success		200		{array}		model.ServerResponse
//	@Failure		401		{object}	object
//	@Router			/servers [get]
func (h *ServerHandler) List(c *gin.Context) {
	serverType := c.Query("type")
	showAll := c.Query("all") == "true"

	var servers []model.Server
	query := h.db.Order("name ASC")
	if serverType != "" {
		query = query.Where("server_type = ?", serverType)
	}
	if !showAll {
		query = query.Where("enabled = ?", true)
	}

	if err := query.Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 转换为响应格式（不包含敏感信息）
	var responses []model.ServerResponse
	for i := range servers {
		responses = append(responses, servers[i].ToResponse())
	}

	c.JSON(http.StatusOK, responses)
}

// Get godoc
//
//	@Summary		获取服务器详情
//	@Description	根据 ID 获取服务器详情（脱敏）
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"服务器 ID"
//	@Success		200	{object}	model.ServerResponse
//	@Failure		400	{object}	object
//	@Failure		404	{object}	object
//	@Router			/servers/{id} [get]
func (h *ServerHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	c.JSON(http.StatusOK, server.ToResponse())
}

// GetForEdit godoc
//
//	@Summary		获取服务器编辑信息
//	@Description	获取服务器完整信息（密码用掩码显示），仅管理员可用
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"服务器 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		404	{object}	object
//	@Router			/servers/{id}/edit [get]
func (h *ServerHandler) GetForEdit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// 返回完整信息（密码用掩码显示）
	c.JSON(http.StatusOK, gin.H{
		"id":                  server.ID,
		"name":                server.Name,
		"host":                server.Host,
		"port":                server.Port,
		"username":            server.Username,
		"auth_type":           server.AuthType,
		"password":            "",
		"private_key":         "",
		"server_type":         server.ServerType,
		"env":                 server.Env,
		"script_path":         server.ScriptPath,
		"script_password":     "",
		"config_path":         server.ConfigPath,
		"config_pattern":      server.ConfigPattern,
		"backup_path":         server.BackupPath,
		"description":         server.Description,
		"enabled":             server.Enabled,
		"has_password":        server.Password != "",
		"has_private_key":     server.PrivateKey != "",
		"has_script_password": server.ScriptPassword != "",
	})
}

// Create godoc
//
//	@Summary		创建服务器
//	@Description	创建新服务器，仅管理员可用
//	@Tags			服务器
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server	body		model.Server	true	"服务器信息"
//	@Success		201		{object}	model.ServerResponse
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/servers [post]
func (h *ServerHandler) Create(c *gin.Context) {
	var server model.Server
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 校验路径和配置模式，防止注入
	if err := validateServerFields(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 使用 Select 明确指定要保存的字段，确保空字符串也能保存
	if err := h.db.Select(
		"name", "host", "port", "username", "auth_type",
		"password", "private_key", "server_type", "env",
		"script_path", "script_password", "config_path",
		"config_pattern", "backup_path", "description", "enabled",
	).Create(&server).Error; err != nil {
		log.Printf("创建服务器失败: %+v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	createAuditLog(h.db, c, "server", "create_server", fmt.Sprintf("创建服务器: %s (%s:%d)", server.Name, server.Host, server.Port), "", "success", "", server.ID, server.Name)
	c.JSON(http.StatusCreated, server.ToResponse())
}

// Update godoc
//
//	@Summary		更新服务器
//	@Description	更新服务器信息，仅管理员可用。密码字段传 __keep__ 表示保留原值
//	@Tags			服务器
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"服务器 ID"
//	@Param			server	body		model.Server	true	"服务器信息"
//	@Success		200		{object}	model.ServerResponse
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/servers/{id} [put]
func (h *ServerHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var existing model.Server
	if err := h.db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	var input model.Server
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 校验路径和配置模式，防止注入
	if err := validateServerFields(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 前端发送 __keep__ 表示保留原密码，不覆盖
	keepPassword := input.Password == "__keep__"
	keepPrivateKey := input.PrivateKey == "__keep__"
	keepScriptPwd := input.ScriptPassword == "__keep__"

	input.ID = uint(id)
	input.CreatedAt = existing.CreatedAt
	// 使用 Select 明确指定要保存的字段，确保空字符串也能保存
	fields := []string{
		"name", "host", "port", "username", "auth_type",
		"server_type", "env", "script_path",
		"config_path", "config_pattern", "backup_path", "description", "enabled",
	}
	if !keepPassword {
		fields = append(fields, "password")
	}
	if !keepPrivateKey {
		fields = append(fields, "private_key")
	}
	if !keepScriptPwd {
		fields = append(fields, "script_password")
	}
	if err := h.db.Select(fields).Save(&input).Error; err != nil {
		log.Printf("更新服务器失败: %+v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	createAuditLog(h.db, c, "server", "update_server", fmt.Sprintf("更新服务器: %s (ID: %d)", input.Name, id), "", "success", "", uint(id), input.Name)
	c.JSON(http.StatusOK, input.ToResponse())
}

// Delete godoc
//
//	@Summary		删除服务器
//	@Description	删除指定服务器（软删除），仅管理员可用
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"服务器 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		500	{object}	object
//	@Router			/servers/{id} [delete]
func (h *ServerHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var serverName string
	var server model.Server
	if err := h.db.First(&server, id).Error; err == nil {
		serverName = server.Name
	}

	if err := h.db.Delete(&model.Server{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	createAuditLog(h.db, c, "server", "delete_server", fmt.Sprintf("删除服务器: %s (ID: %d)", serverName, id), "", "success", "", uint(id), serverName)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ToggleEnabled godoc
//
//	@Summary		切换服务器启用状态
//	@Description	启用或禁用服务器，仅管理员可用
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"服务器 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		404	{object}	object
//	@Failure		500	{object}	object
//	@Router			/servers/{id}/toggle [put]
func (h *ServerHandler) ToggleEnabled(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	newState := !server.Enabled
	if err := h.db.Model(&server).Update("enabled", newState).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	action := "enable_server"
	if !newState {
		action = "disable_server"
	}
	createAuditLog(h.db, c, "server", action, fmt.Sprintf("%s服务器: %s (ID: %d)", map[bool]string{true: "启用", false: "禁用"}[newState], server.Name, id), "", "success", "", uint(id), server.Name)

	if newState {
		c.JSON(http.StatusOK, gin.H{"message": "已启用", "enabled": true})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "已禁用", "enabled": false})
	}
}

// TestConnection godoc
//
//	@Summary		测试服务器连接
//	@Description	测试 SSH 连接到指定服务器，仅管理员可用
//	@Tags			服务器
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"服务器 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		404	{object}	object
//	@Router			/servers/{id}/test [post]
func (h *ServerHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	var auth ssh.AuthMethod
	switch server.AuthType {
	case "password":
		auth = ssh.Password(server.Password)
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("解析私钥失败: %v", err), "success": false})
			return
		}
		auth = ssh.PublicKeys(signer)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的认证类型", "success": false})
		return
	}

	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: service.GetHostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("连接失败: %v", err),
		})
		return
	}
	defer client.Close()

	// 执行一个简单命令验证连接
	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("创建会话失败: %v", err),
		})
		return
	}
	defer session.Close()

	output, err := session.CombinedOutput("echo ok")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("执行命令失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("连接成功 (%s@%s:%d)", server.Username, server.Host, server.Port),
		"output":  string(output),
	})
}

type BatchDeleteServersRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchDeleteServers godoc
//
//	@Summary		批量删除服务器
//	@Description	批量删除服务器（软删除），仅管理员可用
//	@Tags			服务器
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchDeleteServersRequest	true	"服务器 ID 列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/servers/batch-delete [post]
func (h *ServerHandler) BatchDeleteServers(c *gin.Context) {
	var req BatchDeleteServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要删除的服务器"})
		return
	}

	deleted := 0
	failed := 0
	var deletedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		var server model.Server
		if err := h.db.First(&server, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		if err := h.db.Delete(&model.Server{}, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, server.Name)
			continue
		}

		deleted++
		deletedNames = append(deletedNames, server.Name)
	}

	createAuditLog(h.db, c, "server", "batch_delete_servers",
		fmt.Sprintf("批量删除服务器: 成功 %d, 失败 %d", deleted, failed),
		"", "success", fmt.Sprintf("删除的服务器: %v", deletedNames), 0, "")

	message := fmt.Sprintf("批量删除完成: 成功 %d, 失败 %d", deleted, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"deleted": deleted,
		"failed":  failed,
	})
}

type BatchToggleServersRequest struct {
	IDs     []uint `json:"ids" binding:"required"`
	Enabled bool   `json:"enabled"`
}

// BatchToggleServers godoc
//
//	@Summary		批量启用/禁用服务器
//	@Description	批量切换服务器启用状态，仅管理员可用
//	@Tags			服务器
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchToggleServersRequest	true	"服务器 ID 列表和目标状态"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/servers/batch-toggle [post]
func (h *ServerHandler) BatchToggleServers(c *gin.Context) {
	var req BatchToggleServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要操作的服务器"})
		return
	}

	updated := 0
	failed := 0
	var updatedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		var server model.Server
		if err := h.db.First(&server, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		if err := h.db.Model(&server).Update("enabled", req.Enabled).Error; err != nil {
			failed++
			failedNames = append(failedNames, server.Name)
			continue
		}

		updated++
		updatedNames = append(updatedNames, server.Name)
	}

	action := "启用"
	if !req.Enabled {
		action = "禁用"
	}

	createAuditLog(h.db, c, "server", "batch_toggle_servers",
		fmt.Sprintf("批量%s服务器: 成功 %d, 失败 %d", action, updated, failed),
		"", "success", fmt.Sprintf("%s的服务器: %v", action, updatedNames), 0, "")

	message := fmt.Sprintf("批量%s完成: 成功 %d, 失败 %d", action, updated, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"updated": updated,
		"failed":  failed,
	})
}

type BatchTestServersRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchTestServers godoc
//
//	@Summary		批量测试服务器连接
//	@Description	批量测试 SSH 连接到指定服务器，仅管理员可用
//	@Tags			服务器
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchTestServersRequest	true	"服务器 ID 列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Router			/servers/batch-test [post]
func (h *ServerHandler) BatchTestServers(c *gin.Context) {
	var req BatchTestServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要测试的服务器"})
		return
	}

	type TestResult struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	var results []TestResult
	success := 0
	failed := 0

	for _, id := range req.IDs {
		var server model.Server
		if err := h.db.First(&server, id).Error; err != nil {
			results = append(results, TestResult{ID: id, Success: false, Error: "服务器不存在"})
			failed++
			continue
		}

		var auth ssh.AuthMethod
		switch server.AuthType {
		case "password":
			auth = ssh.Password(server.Password)
		case "key":
			signer, err := ssh.ParsePrivateKey([]byte(server.PrivateKey))
			if err != nil {
				results = append(results, TestResult{ID: id, Name: server.Name, Success: false, Error: fmt.Sprintf("解析私钥失败: %v", err)})
				failed++
				continue
			}
			auth = ssh.PublicKeys(signer)
		default:
			results = append(results, TestResult{ID: id, Name: server.Name, Success: false, Error: "不支持的认证类型"})
			failed++
			continue
		}

		config := &ssh.ClientConfig{
			User:            server.Username,
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: service.GetHostKeyCallback(),
			Timeout:         10 * time.Second,
		}

		addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			results = append(results, TestResult{ID: id, Name: server.Name, Success: false, Error: fmt.Sprintf("连接失败: %v", err)})
			failed++
			continue
		}

		session, err := client.NewSession()
		if err != nil {
			client.Close()
			results = append(results, TestResult{ID: id, Name: server.Name, Success: false, Error: fmt.Sprintf("创建会话失败: %v", err)})
			failed++
			continue
		}

		_, err = session.CombinedOutput("echo ok")
		session.Close()
		client.Close()

		if err != nil {
			results = append(results, TestResult{ID: id, Name: server.Name, Success: false, Error: fmt.Sprintf("执行命令失败: %v", err)})
			failed++
			continue
		}

		results = append(results, TestResult{ID: id, Name: server.Name, Success: true})
		success++
	}

	createAuditLog(h.db, c, "server", "batch_test_servers",
		fmt.Sprintf("批量测试服务器连接: 成功 %d, 失败 %d", success, failed),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("批量测试完成: 成功 %d, 失败 %d", success, failed),
		"success": success,
		"failed":  failed,
		"results": results,
	})
}
