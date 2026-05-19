package handler

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

// getClientIP 从请求头中提取客户端 IP 链。
// 优先返回 X-Forwarded-For 完整链（保留所有代理节点，便于排查）；
// 其次从 X-Real-IP 取；兜底使用 Gin 的 ClientIP。
func getClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// 去除每个 IP 的前后空格后原样返回完整链
		parts := strings.Split(xff, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return strings.Join(parts, ", ")
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return c.ClientIP()
}

// createAuditLog 将操作审计日志写入数据库。写入失败时仅打印警告，不影响主流程。
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
		IP:         getClientIP(c),
		ServerID:   serverID,
		ServerName: serverName,
	}
	if err := db.Create(&logEntry).Error; err != nil {
		log.Printf("[WARN] 审计日志写入失败: %v (module=%s, action=%s, target=%s)", err, module, action, target)
	}
}
