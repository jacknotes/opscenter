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

// Rollouts godoc
//
//	@Summary		获取 K8s Rollout 列表
//	@Description	获取指定 K8s 服务器的 Argo Rollout 列表
//	@Tags			K8s
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{array}		service.Rollout
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/k8s/rollouts [get]
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

// OnlinePreview godoc
//
//	@Summary		K8s 上线预览
//	@Description	预览 K8s 项目的 canary 上线操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sBatchRequest	true	"项目列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/online/preview [post]
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

// OnlineExecute godoc
//
//	@Summary		执行 K8s 上线
//	@Description	根据预览 ID 执行 K8s canary 上线操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/online/execute [post]
func (h *K8sHandler) OnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "online")
}

// SyncPreview godoc
//
//	@Summary		K8s 同步预览
//	@Description	预览 K8s 项目的全量同步操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sBatchRequest	true	"项目列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/sync/preview [post]
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

// SyncExecute godoc
//
//	@Summary		执行 K8s 同步
//	@Description	根据预览 ID 执行 K8s 全量同步操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/sync/execute [post]
func (h *K8sHandler) SyncExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "sync")
}

// RollbackPreview godoc
//
//	@Summary		K8s 回滚预览
//	@Description	预览 K8s 项目的回滚操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sBatchRequest	true	"项目列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/rollback/preview [post]
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

// RollbackExecute godoc
//
//	@Summary		执行 K8s 回滚
//	@Description	根据预览 ID 执行 K8s 回滚操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/rollback/execute [post]
func (h *K8sHandler) RollbackExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "rollback")
}

// FullOnlinePreview godoc
//
//	@Summary		K8s 全量上线预览
//	@Description	预览 K8s 全量上线操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sFullRequest	true	"服务器 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/full_online/preview [post]
func (h *K8sHandler) FullOnlinePreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "online")
}

// FullOnlineExecute godoc
//
//	@Summary		执行 K8s 全量上线
//	@Description	根据预览 ID 执行 K8s 全量上线操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/full_online/execute [post]
func (h *K8sHandler) FullOnlineExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "full_online")
}

// FullSyncPreview godoc
//
//	@Summary		K8s 全量同步预览
//	@Description	预览 K8s 全量同步操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sFullRequest	true	"服务器 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/full_sync/preview [post]
func (h *K8sHandler) FullSyncPreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "sync")
}

// FullSyncExecute godoc
//
//	@Summary		执行 K8s 全量同步
//	@Description	根据预览 ID 执行 K8s 全量同步操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/full_sync/execute [post]
func (h *K8sHandler) FullSyncExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executeK8sAction(c, req.PreviewID, "full_sync")
}

// FullRollbackPreview godoc
//
//	@Summary		K8s 全量回滚预览
//	@Description	预览 K8s 全量回滚操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		K8sFullRequest	true	"服务器 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/k8s/full_rollback/preview [post]
func (h *K8sHandler) FullRollbackPreview(c *gin.Context) {
	var req K8sFullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.generateFullPreview(c, req.ServerID, "rollback")
}

// FullRollbackExecute godoc
//
//	@Summary		执行 K8s 全量回滚
//	@Description	根据预览 ID 执行 K8s 全量回滚操作
//	@Tags			K8s
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/k8s/full_rollback/execute [post]
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
	} else if cmdsI, ok := params["commands"].([]interface{}); ok {
		for _, c := range cmdsI {
			if s, ok := c.(string); ok {
				commands = append(commands, s)
			}
		}
	} else if cmd, ok := params["command"].(string); ok {
		commands = []string{cmd}
	}
	if len(commands) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览命令为空"})
		return
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

	status := "success"
	if lastErr != nil {
		status = "failed"
	}
	createAuditLog(h.db, c, "k8s", action,
		fmt.Sprintf("Commands: %v", commands),
		fmt.Sprintf("%v", commands), status, fmt.Sprintf("%v", outputs), server.ID, server.Name)

	if lastErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", lastErr), "output": outputs})
		return
	}
	h.previewMgr.Delete(previewID)

	c.JSON(http.StatusOK, gin.H{"output": outputs, "status": "success"})
}
