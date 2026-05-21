package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

type PreprodScaleRequest struct {
	ServerID      uint     `json:"server_id" binding:"required"`
	ResourceNames []string `json:"resource_names"`
}

type PreprodHandler struct {
	db             *gorm.DB
	sshManager     *service.SSHManager
	previewMgr     *service.PreviewManager
	lockManager    *service.LockManager
	preprodService *service.PreprodService
}

func NewPreprodHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager, lockManager *service.LockManager) *PreprodHandler {
	return &PreprodHandler{
		db:             db,
		sshManager:     sshManager,
		previewMgr:     previewMgr,
		lockManager:    lockManager,
		preprodService: service.NewPreprodService(sshManager),
	}
}

// Status godoc
//
//	@Summary		获取预生产环境状态
//	@Description	获取指定服务器的预生产环境资源状态
//	@Tags			预生产
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{array}		service.PreprodResource
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/preprod/status [get]
func (h *PreprodHandler) Status(c *gin.Context) {
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

	targetOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list-targets")

	resources := h.preprodService.ParseListOutput(output)
	if resources == nil {
		resources = []service.PreprodResource{}
	}
	if targetOutput != "" {
		targets := h.preprodService.ParseTargetOutput(targetOutput)
		resources = h.preprodService.MergeTargets(resources, targets)
	}
	c.JSON(http.StatusOK, resources)
}

// ScaleDownPreview godoc
//
//	@Summary		预生产缩容预览
//	@Description	预览预生产环境缩容操作的命令和影响
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreprodScaleRequest	true	"缩容参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/preprod/scaledown/preview [post]
func (h *PreprodHandler) ScaleDownPreview(c *gin.Context) {
	var req PreprodScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateResourceNames(req.ResourceNames); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaledown", req.ResourceNames)

	previewID := h.previewMgr.Create("preprod", "scaledown", req.ServerID, map[string]interface{}{
		"command":        command,
		"resource_names": req.ResourceNames,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

// ScaleDownExecute godoc
//
//	@Summary		执行预生产缩容
//	@Description	根据预览 ID 执行预生产环境缩容操作
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/preprod/scaledown/execute [post]
func (h *PreprodHandler) ScaleDownExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executePreprodAction(c, req.PreviewID, "scaledown")
}

// ScaleUpPreview godoc
//
//	@Summary		预生产扩容预览
//	@Description	预览预生产环境扩容操作的命令和影响
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreprodScaleRequest	true	"扩容参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/preprod/scaleup/preview [post]
func (h *PreprodHandler) ScaleUpPreview(c *gin.Context) {
	var req PreprodScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := validateResourceNames(req.ResourceNames); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaleup", req.ResourceNames)

	previewID := h.previewMgr.Create("preprod", "scaleup", req.ServerID, map[string]interface{}{
		"command":        command,
		"resource_names": req.ResourceNames,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

// ScaleUpExecute godoc
//
//	@Summary		执行预生产扩容
//	@Description	根据预览 ID 执行预生产环境扩容操作
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/preprod/scaleup/execute [post]
func (h *PreprodHandler) ScaleUpExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executePreprodAction(c, req.PreviewID, "scaleup")
}

func (h *PreprodHandler) executePreprodAction(c *gin.Context, previewID, action string) {
	preview, ok := h.previewMgr.Get(previewID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期或不存在"})
		return
	}

	if preview.Module != "preprod" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// Acquire lock
	username := c.GetString("username")
	locked, holder := h.lockManager.TryLock(preview.ServerID, username, 10*time.Minute)
	if !locked {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("操作正在进行中，请等待 (当前操作人: %s)", holder.Username),
		})
		return
	}
	defer h.lockManager.Unlock(preview.ServerID, username)

	command, _ := preview.Params["command"].(string)
	if command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览命令为空"})
		return
	}
	output, err := h.sshManager.ExecuteWithPipe(&server, command, server.ScriptPassword)

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	status := "success"
	if err != nil {
		status = "failed"
	}
	createAuditLog(h.db, c, "preprod", action,
		command, command, status, output, server.ID, server.Name)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err), "output": output})
		return
	}
	h.previewMgr.Delete(previewID)

	c.JSON(http.StatusOK, gin.H{"output": output, "status": "success"})
}

// validateResourceNames 校验资源名称列表，防止命令注入
func validateResourceNames(names []string) error {
	for _, name := range names {
		if !service.ValidateProjectName(name) {
			return fmt.Errorf("资源名称 [%s] 包含非法字符", name)
		}
	}
	return nil
}
