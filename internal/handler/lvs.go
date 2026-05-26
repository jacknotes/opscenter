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
	db         *gorm.DB
	sshManager *service.SSHManager
	previewMgr *service.PreviewManager
	lvsService *service.LVSService
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

// List godoc
//
//	@Summary		获取 LVS 虚拟服务器列表
//	@Description	获取指定 LVS 服务器的虚拟服务器和真实服务器列表
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{array}		service.VirtualServer
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/lvs/list [get]
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

	// 获取 status 输出，补充下线的 RS
	statusOutput, statusErr := h.sshManager.Execute(&server, server.ScriptPath+" status")
	if statusErr == nil && statusOutput != "" {
		statusGroups := h.lvsService.ParseStatusOutput(statusOutput)
		servers = h.lvsService.MergeOfflineRS(servers, statusGroups)
	}

	c.JSON(http.StatusOK, servers)
}

// Status godoc
//
//	@Summary		获取 LVS 状态
//	@Description	获取指定 LVS 服务器的详细状态信息
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		string	true	"服务器 ID"
//	@Success		200			{object}	object
//	@Failure		400			{object}	object
//	@Failure		404			{object}	object
//	@Failure		500			{object}	object
//	@Router			/lvs/status [get]
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

	statusGroups := h.lvsService.ParseStatusOutput(output)
	c.JSON(http.StatusOK, gin.H{"output": output, "groups": statusGroups})
}

// OpPreview godoc
//
//	@Summary		LVS 上下线操作预览
//	@Description	预览 LVS 真实服务器上下线操作的命令和影响
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		LVSOpRequest	true	"操作参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/lvs/op/preview [post]
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

// OpExecute godoc
//
//	@Summary		执行 LVS 上下线操作
//	@Description	根据预览 ID 执行 LVS 真实服务器上下线操作
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/lvs/op/execute [post]
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

// SwapPreview godoc
//
//	@Summary		LVS 切换操作预览
//	@Description	预览 LVS 真实服务器切换操作的命令和影响
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		LVSSwapRequest	true	"切换参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Router			/lvs/swap/preview [post]
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

// SwapExecute godoc
//
//	@Summary		执行 LVS 切换操作
//	@Description	根据预览 ID 执行 LVS 真实服务器切换操作
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		PreviewExecuteRequest	true	"预览 ID"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		404		{object}	object
//	@Failure		500		{object}	object
//	@Router			/lvs/swap/execute [post]
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
