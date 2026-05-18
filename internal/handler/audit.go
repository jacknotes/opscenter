package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

func createAuditLog(db *gorm.DB, c *gin.Context, module, action, target, detail, status, output string, serverID uint, serverName string) {
	var userID uint
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(uint); ok {
			userID = id
		}
	}

	logEntry := model.OperationLog{
		UserID:     userID,
		Username:   c.GetString("username"),
		Module:     module,
		Action:     action,
		Target:     target,
		Detail:     detail,
		Status:     status,
		Output:     output,
		IP:         c.ClientIP(),
		ServerID:   serverID,
		ServerName: serverName,
	}
	db.Create(&logEntry)
}
