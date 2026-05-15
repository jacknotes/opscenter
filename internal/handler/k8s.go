package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

type K8sHandler struct {
	db         *gorm.DB
	sshManager *service.SSHManager
	previewMgr *service.PreviewManager
	k8sService *service.K8sService
}

func NewK8sHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager) *K8sHandler {
	return &K8sHandler{
		db:         db,
		sshManager: sshManager,
		previewMgr: previewMgr,
		k8sService: service.NewK8sService(sshManager),
	}
}

type K8sSingleRequest struct {
	ServerID  uint   `json:"server_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Namespace string `json:"namespace" binding:"required"`
}

type K8sBatchRequest struct {
	ServerID uint `json:"server_id" binding:"required"`
	Projects []struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"projects" binding:"required"`
}

type K8sFullRequest struct {
	ServerID uint `json:"server_id" binding:"required"`
}

func (h *K8sHandler) Rollouts(c *gin.Context) {
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

	output, err := h.sshManager.Execute(&server, server.ScriptPath+" list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
		return
	}

	rollouts := h.k8sService.ParseListOutput(output)
	if rollouts == nil {
		rollouts = []service.Rollout{}
	}
	c.JSON(http.StatusOK, rollouts)
}

func (h *K8sHandler) OnlinePreview(c *gin.Context) {
	var req K8sBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	commands := h.k8sService.GenerateBatchPreview(server.ScriptPath, "online", req.Projects)

	previewID := h.previewMgr.Create("k8s", "online", req.ServerID, map[string]interface{}{
		"projects": req.Projects,
		"commands": commands,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"commands":       commands,
		"description":    fmt.Sprintf("上线 %d 个项目的 canary 版本", len(req.Projects)),
	})
}

func (h *K8sHandler) OnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "online")
}

func (h *K8sHandler) SyncPreview(c *gin.Context) {
	var req K8sBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	commands := h.k8sService.GenerateBatchPreview(server.ScriptPath, "sync", req.Projects)

	previewID := h.previewMgr.Create("k8s", "sync", req.ServerID, map[string]interface{}{
		"projects": req.Projects,
		"commands": commands,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"commands":       commands,
		"description":    fmt.Sprintf("同步 %d 个项目的全量版本", len(req.Projects)),
	})
}

func (h *K8sHandler) SyncExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "sync")
}

func (h *K8sHandler) RollbackPreview(c *gin.Context) {
	var req K8sBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	commands := h.k8sService.GenerateBatchPreview(server.ScriptPath, "rollback", req.Projects)

	previewID := h.previewMgr.Create("k8s", "rollback", req.ServerID, map[string]interface{}{
		"projects": req.Projects,
		"commands": commands,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"commands":       commands,
		"description":    fmt.Sprintf("回滚 %d 个项目", len(req.Projects)),
	})
}

func (h *K8sHandler) RollbackExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "rollback")
}

func (h *K8sHandler) FullOnlinePreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "online")
}

func (h *K8sHandler) FullOnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "full_online")
}

func (h *K8sHandler) FullSyncPreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "sync")
}

func (h *K8sHandler) FullSyncExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "full_sync")
}

func (h *K8sHandler) FullRollbackPreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "rollback")
}

func (h *K8sHandler) FullRollbackExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "full_rollback")
}

func (h *K8sHandler) generateFullPreview(c *gin.Context, serverID uint, action string) {
	var server model.Server
	if err := h.db.First(&server, serverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	command, description := h.k8sService.GenerateFullPreview(server.ScriptPath, action)

	previewID := h.previewMgr.Create("k8s", "full_"+action, serverID, map[string]interface{}{
		"command": command,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"commands":       []string{command},
		"description":    description,
	})
}

func (h *K8sHandler) executeK8sAction(c *gin.Context, previewID, action string) {
	preview, ok := h.previewMgr.Get(previewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "k8s" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	var commands []string
	if cmds, ok := params["commands"].([]string); ok {
		commands = cmds
	} else if cmd, ok := params["command"].(string); ok {
		commands = []string{cmd}
	}

	var outputs []string
	var lastErr error
	for _, cmd := range commands {
		output, err := h.sshManager.ExecuteWithPipe(&server, cmd, server.ScriptPassword)
		outputs = append(outputs, output)
		if err != nil {
			lastErr = err
			break
		}
	}

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	logEntry := model.OperationLog{
		Username:   c.GetString("username"),
		Module:     "k8s",
		Action:     action,
		Target:     fmt.Sprintf("Commands: %v", commands),
		Detail:     fmt.Sprintf("%v", commands),
		PreviewID:  previewID,
		ServerID:   server.ID,
		ServerName: server.Name,
	}

	if lastErr != nil {
		logEntry.Status = "failed"
		logEntry.Output = fmt.Sprintf("%v", outputs)
		h.db.Create(&logEntry)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", lastErr), "output": outputs})
		return
	}

	logEntry.Status = "success"
	logEntry.Output = fmt.Sprintf("%v", outputs)
	h.db.Create(&logEntry)
	h.previewMgr.Delete(previewID)

	c.JSON(http.StatusOK, gin.H{"output": outputs, "status": "success"})
}
