package handler

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

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

// auditJSON 是审计日志 JSON 输出的结构。
type auditJSON struct {
	Timestamp  string `json:"timestamp"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Module     string `json:"module"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Detail     string `json:"detail,omitempty"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	IP         string `json:"ip"`
	ServerID   uint   `json:"server_id,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	PreviewID  string `json:"preview_id,omitempty"`
}

// AuditWriter 是审计日志 JSON 输出的 writer，由 main.go 根据配置初始化。
// 默认输出到 stdout，可通过 InitAuditWriter 切换为文件输出或禁用。
var (
	AuditWriter  io.Writer = os.Stdout
	auditEncoder *json.Encoder
	auditMu      sync.Mutex
	auditEnabled bool
	auditMaxLen  int = 4096
)

// InitAuditWriter 初始化审计日志 JSON 输出。
// enabled: 是否启用；output: "stdout" 或 "file"；filePath: 文件路径；maxOutput: Output 最大字符数。
func InitAuditWriter(enabled bool, output, filePath string, maxOutput int) {
	auditEnabled = enabled
	if maxOutput > 0 {
		auditMaxLen = maxOutput
	}
	if !enabled {
		return
	}

	switch output {
	case "file":
		if filePath == "" {
			log.Printf("[WARN] 审计日志输出为 file 但未配置 file_path，回退到 stdout")
			AuditWriter = os.Stdout
		} else {
			// 自动创建目录
			dir := filePath[:strings.LastIndex(filePath, "/")]
			if dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					log.Printf("[WARN] 审计日志目录创建失败: %v，回退到 stdout", err)
					AuditWriter = os.Stdout
					break
				}
			}
			f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Printf("[WARN] 审计日志文件打开失败: %v，回退到 stdout", err)
				AuditWriter = os.Stdout
			} else {
				AuditWriter = f
				log.Printf("[INFO] 审计日志 JSON 输出到文件: %s", filePath)
			}
		}
	default:
		AuditWriter = os.Stdout
		log.Printf("[INFO] 审计日志 JSON 输出到 stdout")
	}

	auditEncoder = json.NewEncoder(AuditWriter)
}

// outputAuditJSON 将审计日志以 JSON 格式写入 AuditWriter。
// 使用 mutex 保证并发安全，写入失败仅打印警告不影响主流程。
func outputAuditJSON(entry *model.OperationLog) {
	if !auditEnabled || auditEncoder == nil {
		return
	}

	detail := entry.Detail
	if len(detail) > auditMaxLen {
		detail = detail[:auditMaxLen] + "...(truncated)"
	}
	output := entry.Output
	if len(output) > auditMaxLen {
		output = output[:auditMaxLen] + "...(truncated)"
	}

	j := auditJSON{
		Timestamp:  entry.CreatedAt.Format(time.RFC3339Nano),
		UserID:     entry.UserID,
		Username:   entry.Username,
		Module:     entry.Module,
		Action:     entry.Action,
		Target:     entry.Target,
		Detail:     detail,
		Status:     entry.Status,
		Output:     output,
		IP:         entry.IP,
		ServerID:   entry.ServerID,
		ServerName: entry.ServerName,
		PreviewID:  entry.PreviewID,
	}

	auditMu.Lock()
	defer auditMu.Unlock()
	if err := auditEncoder.Encode(&j); err != nil {
		log.Printf("[WARN] 审计日志 JSON 输出失败: %v", err)
	}
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

	// JSON 输出到 stdout/文件
	outputAuditJSON(&logEntry)
}
