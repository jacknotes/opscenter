package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"opscenter/internal/model"
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

func (h *ServerHandler) List(c *gin.Context) {
	serverType := c.Query("type")
	showAll := c.Query("all") == "true"

	var servers []model.Server
	query := h.db.Order("id ASC")
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
	for _, s := range servers {
		responses = append(responses, s.ToResponse())
	}

	c.JSON(http.StatusOK, responses)
}

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
		"id":              server.ID,
		"name":            server.Name,
		"host":            server.Host,
		"port":            server.Port,
		"username":        server.Username,
		"auth_type":       server.AuthType,
		"password":        "",
		"private_key":     "",
		"server_type":     server.ServerType,
		"env":             server.Env,
		"script_path":     server.ScriptPath,
		"script_password": "",
		"config_path":     server.ConfigPath,
		"config_pattern":  server.ConfigPattern,
		"backup_path":     server.BackupPath,
		"description":     server.Description,
		"enabled":         server.Enabled,
		"has_password":    server.Password != "",
		"has_private_key": server.PrivateKey != "",
		"has_script_password":  server.ScriptPassword != "",
	})
}

func (h *ServerHandler) Create(c *gin.Context) {
	var server model.Server
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
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

	h.auditLog(c, "create_server", fmt.Sprintf("创建服务器: %s (%s:%d)", server.Name, server.Host, server.Port), "", "success", "")
	c.JSON(http.StatusCreated, server.ToResponse())
}

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

	h.auditLog(c, "update_server", fmt.Sprintf("更新服务器: %s (ID: %d)", input.Name, id), "", "success", "")
	c.JSON(http.StatusOK, input.ToResponse())
}

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

	h.auditLog(c, "delete_server", fmt.Sprintf("删除服务器: %s (ID: %d)", serverName, id), "", "success", "")
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

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
	h.auditLog(c, action, fmt.Sprintf("%s服务器: %s (ID: %d)", map[bool]string{true: "启用", false: "禁用"}[newState], server.Name, id), "", "success", "")

	if newState {
		c.JSON(http.StatusOK, gin.H{"message": "已启用", "enabled": true})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "已禁用", "enabled": false})
	}
}

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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
