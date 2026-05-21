package handler

import (
	"log"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/model"
)

// 敏感字段匹配模式（键值类型，保留关键字替换值）
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)--password[= ]\S+`),
	regexp.MustCompile(`(?i)\-p\S+`), // MySQL -pPassword 格式
	regexp.MustCompile(`(?i)(private_key|privkey)\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)(secret|token)\s*[=:]\s*\S+`),
}

// 全文替换模式（匹配的整体都是敏感的，直接替换为 ***）
var fullMatchPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)echo\s+'[A-Za-z0-9+/=]+'\s*\|\s*base64\s`),
	regexp.MustCompile(`(?i)echo\s+"[^"]*"\s*\|\s*sudo\s+-S`),
}

// sanitizeCommand 对命令中的敏感信息进行脱敏处理
func sanitizeCommand(cmd string) string {
	result := cmd
	// 先处理全文替换模式
	for _, pattern := range fullMatchPatterns {
		result = pattern.ReplaceAllString(result, "*** ")
	}
	// 再处理键值替换模式（保留关键字，替换值）
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			for _, sep := range []string{"=", ":", " "} {
				if idx := strings.LastIndex(match, sep); idx > 0 {
					return match[:idx+len(sep)] + "***"
				}
			}
			return "***"
		})
	}
	return result
}

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
		Detail:     sanitizeCommand(detail),
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
