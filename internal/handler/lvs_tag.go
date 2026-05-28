package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

type LVSTagHandler struct {
	db *gorm.DB
}

func NewLVSTagHandler(db *gorm.DB) *LVSTagHandler {
	return &LVSTagHandler{db: db}
}

type LvsTagRequest struct {
	RSIP           string `json:"rs_ip" binding:"required"`
	Tag            string `json:"tag"`
	Disabled       bool   `json:"disabled"`
	DisabledReason string `json:"disabled_reason"`
}

// List godoc
//
//	@Summary		获取 LVS RS 标签列表
//	@Description	获取所有 RS 标签，支持按 rs_ip 批量查询
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			rs_ips	query		string	false	"逗号分隔的 RS IP 列表"
//	@Success		200		{array}		model.LvsRSTag
//	@Router			/lvs/tags [get]
func (h *LVSTagHandler) List(c *gin.Context) {
	var tags []model.LvsRSTag
	query := h.db

	if rsIPs := c.Query("rs_ips"); rsIPs != "" {
		ips := strings.Split(rsIPs, ",")
		for i := range ips {
			ips[i] = strings.TrimSpace(ips[i])
		}
		query = query.Where("rs_ip IN ?", ips)
	}

	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询标签失败"})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// CreateOrUpdate godoc
//
//	@Summary		创建或更新 LVS RS 标签
//	@Description	为指定 RS IP 设置环境标签（upsert）
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		LvsTagRequest	true	"标签参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Router			/lvs/tags [put]
func (h *LVSTagHandler) CreateOrUpdate(c *gin.Context) {
	var req LvsTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.Disabled && strings.TrimSpace(req.DisabledReason) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "禁用时必须填写禁用原因"})
		return
	}

	updates := map[string]interface{}{
		"tag":             req.Tag,
		"disabled":        req.Disabled,
		"disabled_reason": req.DisabledReason,
	}

	tag := model.LvsRSTag{RSIP: req.RSIP}
	if err := h.db.Where("rs_ip = ?", req.RSIP).Assign(updates).FirstOrCreate(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存标签失败"})
		return
	}

	action := "update_tag"
	detail := fmt.Sprintf("更新RS标签: %s -> %s", req.RSIP, req.Tag)
	if req.Disabled {
		action = "disable_rs"
		detail = fmt.Sprintf("禁用RS: %s, 原因: %s", req.RSIP, req.DisabledReason)
	}
	createAuditLog(h.db, c, "lvs", action, detail, "", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "标签已保存"})
}

// Delete godoc
//
//	@Summary		删除 LVS RS 标签
//	@Description	删除指定 RS IP 的环境标签
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			rs_ip	path		string	true	"RS IP"
//	@Success		200		{object}	object
//	@Failure		404		{object}	object
//	@Router			/lvs/tags/{rs_ip} [delete]
func (h *LVSTagHandler) Delete(c *gin.Context) {
	rsIP := c.Param("rs_ip")
	if rsIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 RS IP"})
		return
	}

	result := h.db.Where("rs_ip = ?", rsIP).Delete(&model.LvsRSTag{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除标签失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	createAuditLog(h.db, c, "lvs", "delete_tag",
		fmt.Sprintf("删除RS标签: %s", rsIP),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "标签已删除"})
}
