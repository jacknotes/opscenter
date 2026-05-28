package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

type LvsPreprodBindingHandler struct {
	db *gorm.DB
}

func NewLvsPreprodBindingHandler(db *gorm.DB) *LvsPreprodBindingHandler {
	return &LvsPreprodBindingHandler{db: db}
}

type BindingRequest struct {
	VSTag           string `json:"vs_tag" binding:"required"`
	RSEnvTag        string `json:"rs_env_tag" binding:"required"`
	PreprodServerID uint   `json:"preprod_server_id" binding:"required"`
}

// List godoc
//
//	@Summary		获取 LVS-Preprod 绑定列表
//	@Description	获取所有绑定关系，支持按 preprod_server_id 过滤
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			preprod_server_id	query		int	false	"预生产服务器 ID"
//	@Success		200					{array}		model.LvsPreprodBinding
//	@Router			/lvs/bindings [get]
func (h *LvsPreprodBindingHandler) List(c *gin.Context) {
	var bindings []model.LvsPreprodBinding
	query := h.db

	if serverIDStr := c.Query("preprod_server_id"); serverIDStr != "" {
		serverID, err := strconv.ParseUint(serverIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数 preprod_server_id 格式错误"})
			return
		}
		query = query.Where("preprod_server_id = ?", serverID)
	}

	if err := query.Find(&bindings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询绑定关系失败"})
		return
	}

	c.JSON(http.StatusOK, bindings)
}

// CreateOrUpdate godoc
//
//	@Summary		创建或更新 LVS-Preprod 绑定
//	@Description	创建绑定关系，若 (vs_tag, rs_env_tag) 已存在则更新
//	@Tags			LVS
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		BindingRequest	true	"绑定参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Router			/lvs/bindings [put]
func (h *LvsPreprodBindingHandler) CreateOrUpdate(c *gin.Context) {
	var req BindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	updates := map[string]interface{}{
		"preprod_server_id": req.PreprodServerID,
	}

	binding := model.LvsPreprodBinding{VSTag: req.VSTag, RSEnvTag: req.RSEnvTag}
	if err := h.db.Where("vs_tag = ? AND rs_env_tag = ?", req.VSTag, req.RSEnvTag).Assign(updates).FirstOrCreate(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存绑定关系失败"})
		return
	}

	createAuditLog(h.db, c, "lvs", "update_binding",
		fmt.Sprintf("更新绑定: VS[%s] RS[%s] -> Preprod[%d]", req.VSTag, req.RSEnvTag, req.PreprodServerID),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "绑定关系已保存"})
}

// Delete godoc
//
//	@Summary		删除 LVS-Preprod 绑定
//	@Description	删除指定 ID 的绑定关系
//	@Tags			LVS
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"绑定 ID"
//	@Success		200	{object}	object
//	@Failure		404	{object}	object
//	@Router			/lvs/bindings/{id} [delete]
func (h *LvsPreprodBindingHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数 id 格式错误"})
		return
	}

	result := h.db.Delete(&model.LvsPreprodBinding{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除绑定关系失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "绑定关系不存在"})
		return
	}

	createAuditLog(h.db, c, "lvs", "delete_binding",
		fmt.Sprintf("删除绑定: ID=%d", id),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{"message": "绑定关系已删除"})
}
