package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/model"
	"opscenter/internal/service"
)

type PreprodScaleRequest struct {
	ServerID      uint     `json:"server_id" binding:"required"`
	ResourceNames []string `json:"resource_names"`
}

type CheckLvsForScaleDownRequest struct {
	PreprodServerID uint `json:"preprod_server_id" binding:"required"`
}

type CheckLvsOnlineRequest struct {
	VSIP string `json:"vs_ip" binding:"required"`
	RSIP string `json:"rs_ip" binding:"required"`
}

type PreprodHandler struct {
	db             *gorm.DB
	sshManager     *service.SSHManager
	previewMgr     *service.PreviewManager
	lockManager    *service.LockManager
	preprodService *service.PreprodService
	lvsService     *service.LVSService
}

func NewPreprodHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager, lockManager *service.LockManager) *PreprodHandler {
	return &PreprodHandler{
		db:             db,
		sshManager:     sshManager,
		previewMgr:     previewMgr,
		lockManager:    lockManager,
		preprodService: service.NewPreprodService(sshManager),
		lvsService:     service.NewLVSService(sshManager),
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
	ctx := c.Request.Context()
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

	output, err := h.sshManager.Execute(ctx, &server, server.ScriptPath+" list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("执行失败: %v", err)})
		return
	}

	targetOutput, err := h.sshManager.Execute(ctx, &server, server.ScriptPath+" list-targets")
	if err != nil {
		log.Printf("获取 target 状态失败: %v", err)
	}

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
	ctx := c.Request.Context()
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

	currentOutput, err := h.sshManager.Execute(ctx, &server, server.ScriptPath+" list")
	if err != nil {
		log.Printf("获取当前状态失败: %v", err)
	}

	// 如果未指定具体资源，从 list 输出中解析所有资源名称
	resourceNames := req.ResourceNames
	if len(resourceNames) == 0 && currentOutput != "" {
		resources := h.preprodService.ParseListOutput(currentOutput)
		for _, r := range resources {
			if r.Name != "" {
				resourceNames = append(resourceNames, r.Name)
			}
		}
	}

	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaledown", req.ResourceNames)

	previewID := h.previewMgr.Create("preprod", "scaledown", req.ServerID, map[string]interface{}{
		"command":        command,
		"resource_names": resourceNames,
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
	ctx := c.Request.Context()
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

	currentOutput, err := h.sshManager.Execute(ctx, &server, server.ScriptPath+" list")
	if err != nil {
		log.Printf("获取当前状态失败: %v", err)
	}

	// 如果未指定具体资源，从 list 输出中解析所有资源名称
	resourceNames := req.ResourceNames
	if len(resourceNames) == 0 && currentOutput != "" {
		resources := h.preprodService.ParseListOutput(currentOutput)
		for _, r := range resources {
			if r.Name != "" {
				resourceNames = append(resourceNames, r.Name)
			}
		}
	}

	command, description := h.preprodService.GeneratePreview(server.ScriptPath, "scaleup", req.ResourceNames)

	previewID := h.previewMgr.Create("preprod", "scaleup", req.ServerID, map[string]interface{}{
		"command":        command,
		"resource_names": resourceNames,
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

// CheckLvsForScaleDown 检查缩容前 LVS RS 状态。
//
//	@Summary		检查缩容前LVS状态
//	@Description	检查预生产服务器缩容前对应LVS的在线状态
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CheckLvsForScaleDownRequest	true	"检查请求"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		500		{object}	object
//	@Router			/lvs/check/scaledown [post]
func (h *PreprodHandler) CheckLvsForScaleDown(c *gin.Context) {
	ctx := c.Request.Context()
	var req CheckLvsForScaleDownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 查询该 preprod 服务器的所有绑定
	var bindings []model.LvsPreprodBinding
	if err := h.db.Where("preprod_server_id = ?", req.PreprodServerID).Find(&bindings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询绑定关系失败"})
		return
	}
	if len(bindings) == 0 {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 查询所有 LVS 服务器
	var lvsServers []model.Server
	if err := h.db.Where("server_type = ? AND enabled = ?", "lvs", true).Find(&lvsServers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询LVS服务器失败"})
		return
	}
	if len(lvsServers) == 0 {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 构建绑定索引: vs_tag -> []binding
	bindingMap := make(map[string][]model.LvsPreprodBinding)
	for _, b := range bindings {
		bindingMap[b.VSTag] = append(bindingMap[b.VSTag], b)
	}

	// 并发查询所有 LVS 服务器
	type warningResult struct {
		VSTag     string `json:"vs_tag"`
		RSEnvTag  string `json:"rs_env_tag"`
		RSIP      string `json:"rs_ip"`
		Status    string `json:"status"`
		LVSServer string `json:"lvs_server"`
	}

	var (
		warnings []warningResult
		mu       sync.Mutex
		wg       sync.WaitGroup
		done     bool
	)

	for _, lvsServer := range lvsServers {
		wg.Add(1)
		go func(srv model.Server) {
			defer wg.Done()

			// 检查是否已有结果
			mu.Lock()
			if done {
				mu.Unlock()
				return
			}
			mu.Unlock()

			// 获取 LVS 数据
			output, err := h.sshManager.Execute(ctx, &srv, srv.ScriptPath+" list")
			if err != nil {
				return
			}
			vsList := h.lvsService.ParseListOutput(output)
			if len(vsList) == 0 {
				return
			}

			// 补充下线 RS
			statusOutput, statusErr := h.sshManager.Execute(ctx, &srv, srv.ScriptPath+" status")
			if statusErr == nil && statusOutput != "" {
				statusGroups := h.lvsService.ParseStatusOutput(statusOutput)
				vsList = h.lvsService.MergeOfflineRS(vsList, statusGroups)
			}

			// 收集 VS IP 并检测角色
			vsIPSet := make(map[string]bool)
			for _, vs := range vsList {
				if vs.IP != "0.0.0.0" {
					vsIPSet[vs.IP] = true
				}
			}
			vsIPs := make([]string, 0, len(vsIPSet))
			for ip := range vsIPSet {
				vsIPs = append(vsIPs, ip)
			}
			roles := h.lvsService.DetectRoles(vsIPs, &srv)

			// 查询 VS 标签
			var vsTags []model.LvsVSTag
			if len(vsIPs) > 0 {
				h.db.Where("vs_ip IN ?", vsIPs).Find(&vsTags)
			}
			vsTagMap := make(map[string]string)
			for _, t := range vsTags {
				vsTagMap[t.VSIP] = t.Tag
			}

			// 查询 RS 标签（按 VS+RS 复合键）
			rsIPSet := make(map[string]bool)
			for _, vs := range vsList {
				for _, rs := range vs.RealServers {
					rsIPSet[rs.IP] = true
				}
			}
			rsIPList := make([]string, 0, len(rsIPSet))
			for ip := range rsIPSet {
				rsIPList = append(rsIPList, ip)
			}
			var rsTags []model.LvsRSTag
			if len(rsIPList) > 0 {
				h.db.Where("rs_ip IN ?", rsIPList).Find(&rsTags)
			}
			rsTagMap := make(map[string]string) // key: "vs_ip:rs_ip"
			for _, t := range rsTags {
				rsTagMap[t.VSIP+":"+t.RSIP] = t.Tag
			}

			// 匹配绑定：找 master VS，检查 RS 状态
			for _, vs := range vsList {
				if roles[vs.IP] != "master" {
					continue
				}
				vsTag := vsTagMap[vs.IP]
				bindingsForVS, ok := bindingMap[vsTag]
				if !ok {
					continue
				}
				for _, binding := range bindingsForVS {
					for _, rs := range vs.RealServers {
						if rsTagMap[vs.IP+":"+rs.IP] == binding.RSEnvTag && rs.Status == "up" {
							mu.Lock()
							if !done {
								warnings = append(warnings, warningResult{
									VSTag:     binding.VSTag,
									RSEnvTag:  binding.RSEnvTag,
									RSIP:      rs.IP,
									Status:    rs.Status,
									LVSServer: srv.Name,
								})
								done = true
							}
							mu.Unlock()
							return
						}
					}
				}
			}
		}(lvsServer)
	}
	wg.Wait()

	if len(warnings) > 0 {
		c.JSON(http.StatusOK, gin.H{"need_warning": true, "warnings": warnings})
	} else {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
	}
}

// CheckLvsOnline 检查 LVS 上线前预生产资源状态。
//
//	@Summary		检查LVS在线状态
//	@Description	检查指定服务器的LVS在线状态
//	@Tags			预生产
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CheckLvsOnlineRequest	true	"检查请求"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Router			/preprod/check/lvs_online [post]
func (h *PreprodHandler) CheckLvsOnline(c *gin.Context) {
	ctx := c.Request.Context()
	var req CheckLvsOnlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 查询 VS 标签
	var vsTag model.LvsVSTag
	if err := h.db.Where("vs_ip = ?", req.VSIP).First(&vsTag).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 查询 RS 标签
	var rsTag model.LvsRSTag
	if err := h.db.Where("rs_ip = ?", req.RSIP).First(&rsTag).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 查询绑定
	var binding model.LvsPreprodBinding
	if err := h.db.Where("vs_tag = ? AND rs_env_tag = ?", vsTag.Tag, rsTag.Tag).First(&binding).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 查询 preprod 服务器
	var preprodServer model.Server
	if err := h.db.First(&preprodServer, binding.PreprodServerID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}

	// 获取资源状态
	output, err := h.sshManager.Execute(ctx, &preprodServer, preprodServer.ScriptPath+" list")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
		return
	}
	targetOutput, err := h.sshManager.Execute(ctx, &preprodServer, preprodServer.ScriptPath+" list-targets")
	if err != nil {
		log.Printf("获取 target 状态失败: %v", err)
	}

	resources := h.preprodService.ParseListOutput(output)
	if resources == nil {
		resources = []service.PreprodResource{}
	}
	if targetOutput != "" {
		targets := h.preprodService.ParseTargetOutput(targetOutput)
		resources = h.preprodService.MergeTargets(resources, targets)
	}

	// 检查哪些资源副本不正常
	type abnormalResource struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Current  int    `json:"current"`
		Target   int    `json:"target"`
	}
	var abnormal []abnormalResource
	for _, r := range resources {
		target := r.TargetReplicas
		if target <= 0 {
			target = r.Desired
		}
		if target > 0 && r.Current < target {
			abnormal = append(abnormal, abnormalResource{
				Name:     r.Name,
				Category: r.Category,
				Current:  r.Current,
				Target:   target,
			})
		}
	}

	if len(abnormal) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"need_warning": true,
			"warnings":     abnormal,
			"vs_tag":       vsTag.Tag,
			"rs_env_tag":   rsTag.Tag,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"need_warning": false})
	}
}

func (h *PreprodHandler) executePreprodAction(c *gin.Context, previewID, action string) {
	ctx := c.Request.Context()
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
	locked, holder := h.lockManager.TryLock(preview.ServerID, username, config.Global.Timeouts.Lock)
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
	output, err := h.sshManager.ExecuteWithPipe(ctx, &server, command, server.ScriptPassword)

	// 执行完成后关闭SSH连接，强制下次请求重新连接
	h.sshManager.CloseServer(server.ID)

	status := "success"
	if err != nil {
		status = "failed"
	}

	// 提取资源名称列表，区分批量/全量操作
	var projectNames string
	var projectCount int
	var auditAction string
	if names, ok := preview.Params["resource_names"].([]interface{}); ok {
		var strNames []string
		for _, n := range names {
			if s, ok := n.(string); ok && s != "" {
				strNames = append(strNames, s)
			}
		}
		if len(strNames) > 0 {
			projectNames = strings.Join(strNames, ",")
			projectCount = len(strNames)
			auditAction = "batch_" + action
		} else {
			projectNames = "*"
			projectCount = 0
			auditAction = "full_" + action
		}
	} else {
		projectNames = "*"
		projectCount = 0
		auditAction = "full_" + action
	}

	createAuditLogWithProjects(h.db, c, "preprod", auditAction,
		command, command, status, output, server.ID, server.Name,
		projectNames, projectCount)

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
