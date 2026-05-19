package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

type LVSHandler struct {
	db            *gorm.DB
	sshManager    *service.SSHManager
	previewMgr    *service.PreviewManager
	lvsService    *service.LVSService
}

func NewLVSHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager) *LVSHandler {
	return &LVSHandler{
		db:         db,
		sshManager: sshManager,
		previewMgr: previewMgr,
		lvsService: service.NewLVSService(sshManager),
	}
}

type LVSOpRequest struct {
	ServerID uint   `json:"server_id" binding:"required"`
	VSIP     string `json:"vs_ip" binding:"required"`
	RSIP     string `json:"rs_ip" binding:"required"`
	State    string `json:"state" binding:"required"`
}

type LVSSwapRequest struct {
	ServerID uint   `json:"server_id" binding:"required"`
	VSIP     string `json:"vs_ip" binding:"required"`
	RSIP1    string `json:"rs_ip1" binding:"required"`
	RSIP2    string `json:"rs_ip2" binding:"required"`
}

type PreviewExecuteRequest struct {
	PreviewID string `json:"preview_id" binding:"required"`
}

func (h *LVSHandler) List(c *gin.Context) {
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

	servers := h.lvsService.ParseListOutput(output)
	c.JSON(http.StatusOK, servers)
}

func (h *LVSHandler) Status(c *gin.Context) {
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

	output, err := h.sshManager.Execute(&server, server.ScriptPath+" status")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"output": output})
}

func (h *LVSHandler) OpPreview(c *gin.Context) {
	var req LVSOpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if !service.ValidateIP(req.VSIP) || !service.ValidateIP(req.RSIP) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP格式错误"})
		return
	}

	if req.State != "on" && req.State != "off" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态必须是 on 或 off"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	// Get current status
	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	command, description := h.lvsService.GenerateOpPreview(server.ScriptPath, req.VSIP, req.RSIP, req.State)

	previewID := h.previewMgr.Create("lvs", "op", req.ServerID, map[string]interface{}{
		"vs_ip": req.VSIP,
		"rs_ip": req.RSIP,
		"state": req.State,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

func (h *LVSHandler) OpExecute(c *gin.Context) {
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

	if preview.Module != "lvs" || preview.Action != "op" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	vsIP, _ := params["vs_ip"].(string)
	rsIP, _ := params["rs_ip"].(string)
	state, _ := params["state"].(string)
	if vsIP == "" || rsIP == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}
	command, _ := h.lvsService.GenerateOpPreview(server.ScriptPath, vsIP, rsIP, state)

	output, err := h.sshManager.Execute(&server, command)

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	status := "success"
	if err != nil {
		status = "failed"
	}
	createAuditLog(h.db, c, "lvs", "op",
		fmt.Sprintf("VS:%s RS:%s State:%s", params["vs_ip"], params["rs_ip"], params["state"]),
		command, status, output, server.ID, server.Name)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err), "output": output})
		return
	}
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"output": output, "status": "success"})
}

func (h *LVSHandler) SwapPreview(c *gin.Context) {
	var req LVSSwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if !service.ValidateIP(req.VSIP) || !service.ValidateIP(req.RSIP1) || !service.ValidateIP(req.RSIP2) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP格式错误"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	currentOutput, _ := h.sshManager.Execute(&server, server.ScriptPath+" list")
	command, description := h.lvsService.GenerateSwapPreview(server.ScriptPath, req.VSIP, req.RSIP1, req.RSIP2)

	previewID := h.previewMgr.Create("lvs", "swap", req.ServerID, map[string]interface{}{
		"vs_ip":  req.VSIP,
		"rs_ip1": req.RSIP1,
		"rs_ip2": req.RSIP2,
	})

	c.JSON(http.StatusOK, gin.H{
		"preview_id":     previewID,
		"current_status": currentOutput,
		"command":        command,
		"description":    description,
	})
}

func (h *LVSHandler) SwapExecute(c *gin.Context) {
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

	if preview.Module != "lvs" || preview.Action != "swap" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览类型不匹配"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}

	params := preview.Params
	vsIP, _ := params["vs_ip"].(string)
	rsIP1, _ := params["rs_ip1"].(string)
	rsIP2, _ := params["rs_ip2"].(string)
	if vsIP == "" || rsIP1 == "" || rsIP2 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览参数不完整"})
		return
	}
	command, _ := h.lvsService.GenerateSwapPreview(server.ScriptPath, vsIP, rsIP1, rsIP2)

	output, err := h.sshManager.Execute(&server, command)

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	status := "success"
	if err != nil {
		status = "failed"
	}
	createAuditLog(h.db, c, "lvs", "swap",
		fmt.Sprintf("VS:%s RS1:%s RS2:%s", params["vs_ip"], params["rs_ip1"], params["rs_ip2"]),
		command, status, output, server.ID, server.Name)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err), "output": output})
		return
	}
	h.previewMgr.Delete(req.PreviewID)

	c.JSON(http.StatusOK, gin.H{"output": output, "status": "success"})
}
