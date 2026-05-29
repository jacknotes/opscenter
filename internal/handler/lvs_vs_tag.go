package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

type LvsVSTagHandler struct {
	db *gorm.DB
}

func NewLvsVSTagHandler(db *gorm.DB) *LvsVSTagHandler {
	return &LvsVSTagHandler{db: db}
}

type LvsVSTagRequest struct {
	VSIP string `json:"vs_ip" binding:"required"`
	Tag  string `json:"tag"`
}

// List godoc
//
//	@Summary		获取 LVS VS 标签列表
//	@Description	获取所有 VS 标签，支持按 vs_ip 批量查询
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			vs_ips	query		string	false	"逗号分隔的 VS IP 列表"
//	@Success		200		{array}		model.LvsVSTag
//	@Router			/lvs/vs_tags [get]
func (h *LvsVSTagHandler) List(c *gin.Context) {
	var tags []model.LvsVSTag
	query := h.db

	if vsIPs := c.Query("vs_ips"); vsIPs != "" {
		ips := strings.Split(vsIPs, ",")
		for i := range ips {
			ips[i] = strings.TrimSpace(ips[i])
		}
		query = query.Where("vs_ip IN ?", ips)
	}

	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询VS标签失败"})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// CreateOrUpdate godoc
//
//	@Summary		创建或更新 LVS VS 标签
//	@Description	为指定 VS IP 设置标签（upsert）
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		LvsVSTagRequest	true	"标签参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Router			/lvs/vs_tags [put]
func (h *LvsVSTagHandler) CreateOrUpdate(c *gin.Context) {
	var req LvsVSTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if !service.ValidateIP(req.VSIP) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP地址格式无效"})
		return
	}

	updates := map[string]interface{}{
		"tag": req.Tag,
	}

	tag := model.LvsVSTag{VSIP: req.VSIP}
	if err := h.db.Where("vs_ip = ?", req.VSIP).Assign(updates).FirstOrCreate(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存VS标签失败"})
		return
	}

	createAuditLog(h.db, c, "lvs", "update_vs_tag",
		fmt.Sprintf("更新VS标签: %s -> %s", req.VSIP, req.Tag),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "VS标签已保存"})
}

// Delete godoc
//
//	@Summary		删除 LVS VS 标签
//	@Description	删除指定 VS IP 的标签
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			vs_ip	path		string	true	"VS IP"
//	@Success		200		{object}	object
//	@Failure		404		{object}	object
//	@Router			/lvs/vs_tags/{vs_ip} [delete]
func (h *LvsVSTagHandler) Delete(c *gin.Context) {
	vsIP := c.Param("vs_ip")
	if vsIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 VS IP"})
		return
	}

	result := h.db.Where("vs_ip = ?", vsIP).Delete(&model.LvsVSTag{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除VS标签失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "VS标签不存在"})
		return
	}

	createAuditLog(h.db, c, "lvs", "delete_vs_tag",
		fmt.Sprintf("删除VS标签: %s", vsIP),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "VS标签已删除"})
}
