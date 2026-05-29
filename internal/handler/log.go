package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

type LogHandler struct {
	db *gorm.DB
}

func NewLogHandler(db *gorm.DB) *LogHandler {
	return &LogHandler{db: db}
}

// List godoc
//
//	@Summary		获取操作日志列表
//	@Description	分页查询操作日志，支持按模块、服务器、用户、状态、时间、关键字等筛选
//	@Tags			操作日志
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int		false	"页码，默认 1"
//	@Param			size		query		int		false	"每页数量，默认 20，最大 100"
//	@Param			module		query		string	false	"模块 (lvs/k8s/nginx/preprod/server)"
//	@Param			server_id	query		string	false	"服务器 ID"
//	@Param			username	query		string	false	"用户名（模糊匹配）"
//	@Param			status		query		string	false	"状态 (success/failed)"
//	@Param			action		query		string	false	"操作类型"
//	@Param			keyword		query		string	false	"关键字搜索（模糊匹配用户名、动作、目标、服务器名、IP）"
//	@Param			start_time	query		string	false	"开始时间 (2006-01-02)"
//	@Param			end_time	query		string	false	"结束时间 (2006-01-02)"
//	@Success		200			{object}	object
//	@Failure		500			{object}	object
//	@Router			/logs [get]
func (h *LogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	module := c.Query("module")
	serverID := c.Query("server_id")
	username := c.Query("username")
	status := c.Query("status")
	action := c.Query("action")
	keyword := c.Query("keyword")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	var total int64
	var logs []model.OperationLog

	query := h.db.Model(&model.OperationLog{})

	// 非管理员只能查看 lvs/nginx/k8s/preprod 模块的日志
	role, _ := c.Get("role")
	if role != "admin" {
		query = query.Where("module NOT IN ?", []string{"auth", "server"})
	}

	if module != "" {
		query = query.Where("module = ?", module)
	}
	if serverID != "" {
		query = query.Where("server_id = ?", serverID)
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR action LIKE ? OR target LIKE ? OR server_name LIKE ? OR ip LIKE ?", like, like, like, like, like)
	}
	if startTime != "" {
		if t, err := time.Parse("2006-01-02", startTime); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endTime != "" {
		if t, err := time.Parse("2006-01-02", endTime); err == nil {
			query = query.Where("created_at < ?", t.Add(24*time.Hour))
		}
	}

	query.Session(&gorm.Session{}).Count(&total)

	if err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"size":  size,
		"data":  logs,
	})
}
