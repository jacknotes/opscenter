package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

type PreprodHandler struct {
	db            *gorm.DB
	sshManager    *service.SSHManager
	previewMgr    *service.PreviewManager
	preprodService *service.PreprodService
}

func NewPreprodHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager) *PreprodHandler {
	return &PreprodHandler{
		db:             db,
		sshManager:     sshManager,
		previewMgr:     previewMgr,
		preprodService: service.NewPreprodService(sshManager),
	}
}

type PreprodRequest struct {
	ServerID uint `json:"server_id" binding:"required"`
}

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

	resources := h.preprodService.ParseListOutput(output)
	c.JSON(http.StatusOK, resources)
}

func (h *PreprodHandler) ScaleDownPreview(c *gin.Context) {
	var req PreprodRequest
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
	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaledown")

	previewID := h.previewMgr.Create("preprod", "scaledown", req.ServerID, map[string]interface{}{
		"command": command,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

func (h *PreprodHandler) ScaleDownExecute(c *gin.Context) {
	var req PreviewExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	h.executePreprodAction(c, req.PreviewID, "scaledown")
}

func (h *PreprodHandler) ScaleUpPreview(c *gin.Context) {
	var req PreprodRequest
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
	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaleup")

	previewID := h.previewMgr.Create("preprod", "scaleup", req.ServerID, map[string]interface{}{
		"command": command,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

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

	command := preview.Params["command"].(string)
	output, err := h.sshManager.ExecuteWithPipe(&server, command, server.ScriptPassword)

	logEntry := model.OperationLog{
		Username:  c.GetString("username"),
		Module:    "preprod",
		Action:    action,
		Target:    command,
		Detail:    command,
		PreviewID: previewID,
	}

	if err != nil {
		logEntry.Status = "failed"
		logEntry.Output = output
		h.db.Create(&logEntry)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err), "output": output})
		return
	}

	logEntry.Status = "success"
	logEntry.Output = output
	h.db.Create(&logEntry)
	h.previewMgr.Delete(previewID)

	c.JSON(http.StatusOK, gin.H{"output": output, "status": "success"})
}
