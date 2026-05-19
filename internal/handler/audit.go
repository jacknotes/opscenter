package handler

import (
	"log"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

func getClientIP(c *gin.Context) string {
	// 优先从 X-Forwarded-For 取第一个非私网 IP
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		for _, ip := range strings.Split(xff, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" && !isPrivateIP(ip) {
				return ip
			}
		}
	}
	// 其次从 X-Real-IP 取
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		xri = strings.TrimSpace(xri)
		if !isPrivateIP(xri) {
			return xri
		}
	}
	// 兜底用 Gin 的 ClientIP
	return c.ClientIP()
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

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
