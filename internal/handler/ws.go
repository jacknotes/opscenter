package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/middleware"
	"opscenter/internal/model"
	"opscenter/internal/service"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origins := config.Global.Server.AllowedOrigins
		if len(origins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, allowed := range origins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

type ConnInfo struct {
	Username string
	ServerID uint
	Conn     *websocket.Conn
}

type WSHandler struct {
	db          *gorm.DB
	sshManager  *service.SSHManager
	previewMgr  *service.PreviewManager
	lockManager *service.LockManager
	clients     sync.Map
}

func NewWSHandler(db *gorm.DB, sshManager *service.SSHManager, previewMgr *service.PreviewManager, lockManager *service.LockManager) *WSHandler {
	return &WSHandler{
		db:          db,
		sshManager:  sshManager,
		previewMgr:  previewMgr,
		lockManager: lockManager,
	}
}

type WSMessage struct {
	Type      string `json:"type"`
	Token     string `json:"token,omitempty"`
	PreviewID string `json:"preview_id,omitempty"`
	Data      string `json:"data,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	Holder    string `json:"holder,omitempty"`
}

type safeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (sc *safeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteJSON(v)
}

func (sc *safeConn) WriteMessage(messageType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn.WriteMessage(messageType, data)
}

func (sc *safeConn) ReadMessage() (messageType int, p []byte, err error) {
	return sc.conn.ReadMessage()
}

func (sc *safeConn) SetReadDeadline(t time.Time) error {
	return sc.conn.SetReadDeadline(t)
}

func (sc *safeConn) SetPongHandler(h func(string) error) {
	sc.conn.SetPongHandler(h)
}

func (sc *safeConn) Close() error {
	return sc.conn.Close()
}

// verifyWSToken 从消息中的 token 或 URL query 中验证 JWT
func verifyWSToken(c *gin.Context, msgToken string, db *gorm.DB) (*middleware.Claims, error) {
	tokenString := msgToken
	if tokenString == "" {
		// 兼容：从 URL query 中取 token
		tokenString = c.Query("token")
	}
	if tokenString == "" {
		return nil, fmt.Errorf("未提供认证令牌")
	}

	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Global.JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("无效的认证令牌")
	}

	// 检查 token 是否已被撤销
	if claims.ID != "" && middleware.IsBlacklisted(claims.ID) {
		return nil, fmt.Errorf("认证令牌已被撤销")
	}

	// 检查用户是否存在且启用
	var user model.User
	if err := db.First(&user, claims.UserID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	if !user.Enabled {
		return nil, fmt.Errorf("账户已被禁用")
	}

	return claims, nil
}

// Handle WebSocket命令执行
//
//	@Summary		WebSocket命令执行
//	@Description	通过WebSocket流式执行命令
//	@Tags			WebSocket
//	@Param			token		query		string	true	"JWT Token"
//	@Param			preview_id	query		string	true	"预览ID"
//	@Router			/ws/exec [get]
func (h *WSHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v", err)
		return
	}

	sc := &safeConn{conn: conn}

	// 防止 panic 导致连接无响应
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] Panic recovered: %v", r)
			sc.WriteJSON(WSMessage{Type: "error", Message: fmt.Sprintf("服务内部错误: %v", r)})
			sc.Close()
		}
	}()

	// Ping/pong heartbeat
	sc.SetReadDeadline(time.Now().Add(config.Global.Timeouts.WSRead))
	sc.SetPongHandler(func(string) error {
		sc.SetReadDeadline(time.Now().Add(config.Global.Timeouts.WSRead))
		return nil
	})

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(config.Global.Timeouts.WSPing)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := sc.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Read first message: must contain type="start", token, and preview_id
	_, message, err := sc.ReadMessage()
	if err != nil {
		log.Printf("[WS] Read start message failed: %v", err)
		sc.WriteJSON(WSMessage{Type: "error", Message: "读取请求超时或连接已断开"})
		return
	}

	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil || msg.Type != "start" || msg.PreviewID == "" {
		sc.WriteJSON(WSMessage{Type: "error", Message: "无效的请求"})
		return
	}

	// 验证 JWT token（从消息中或 URL query 中）
	claims, err := verifyWSToken(c, msg.Token, h.db)
	if err != nil {
		sc.WriteJSON(WSMessage{Type: "error", Message: err.Error()})
		return
	}

	usernameStr := claims.Username
	log.Printf("[WS] Authenticated user: %s", usernameStr)

	connID := uuid.New().String()
	h.clients.Store(connID, &ConnInfo{
		Username: usernameStr,
		Conn:     conn,
	})
	defer func() {
		h.clients.Delete(connID)
		sc.Close()
	}()

	// Get preview
	preview, ok := h.previewMgr.Get(msg.PreviewID)
	if !ok {
		sc.WriteJSON(WSMessage{Type: "error", Message: "预览已过期或不存在，请重新预览"})
		return
	}

	if preview.Module != "preprod" {
		sc.WriteJSON(WSMessage{Type: "error", Message: "预览类型不匹配"})
		return
	}

	// Load server
	var server model.Server
	if err := h.db.First(&server, preview.ServerID).Error; err != nil {
		sc.WriteJSON(WSMessage{Type: "error", Message: "服务器不存在"})
		return
	}

	// Acquire lock
	locked, holder := h.lockManager.TryLock(preview.ServerID, usernameStr, config.Global.Timeouts.Lock)
	if !locked {
		sc.WriteJSON(WSMessage{
			Type:    "lock_error",
			Message: fmt.Sprintf("操作正在进行中，请等待 (当前操作人: %s)", holder.Username),
			Holder:  holder.Username,
		})
		return
	}
	defer h.lockManager.Unlock(preview.ServerID, usernameStr)
	defer h.previewMgr.Delete(msg.PreviewID)

	h.clients.Store(connID, &ConnInfo{
		Username: usernameStr,
		ServerID: preview.ServerID,
		Conn:     conn,
	})

	command, _ := preview.Params["command"].(string)
	if command == "" {
		sc.WriteJSON(WSMessage{Type: "error", Message: "预览命令为空"})
		return
	}

	// 提取资源名称列表，区分批量/全量操作（与 HTTP handler 保持一致）
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
			auditAction = "batch_" + preview.Action
		} else {
			projectNames = "*"
			projectCount = 0
			auditAction = "full_" + preview.Action
		}
	} else {
		projectNames = "*"
		projectCount = 0
		auditAction = "full_" + preview.Action
	}

	logEntry := model.OperationLog{
		UserID:       claims.UserID,
		Username:     usernameStr,
		Module:       "preprod",
		Action:       auditAction,
		Target:       command,
		Detail:       command,
		PreviewID:    msg.PreviewID,
		ServerID:     server.ID,
		ServerName:   server.Name,
		IP:           getClientIP(c),
		ProjectNames: projectNames,
		ProjectCount: projectCount,
	}

	// Stream execution
	outputCh, errCh := h.sshManager.ExecuteStream(&server, command, server.ScriptPassword)

	var allOutput []string
	var execErr error

	for chunk := range outputCh {
		allOutput = append(allOutput, chunk.Line)
		streamType := "stdout"
		if chunk.Err {
			streamType = "stderr"
		}
		if err := sc.WriteJSON(WSMessage{
			Type:   "output",
			Data:   chunk.Line,
			Stream: streamType,
		}); err != nil {
			go func() {
				// 带超时的 drain，防止 SSH 流挂起导致 goroutine 永久阻塞
				timer := time.NewTimer(30 * time.Second)
				defer timer.Stop()
				for {
					select {
					case _, ok := <-outputCh:
						if !ok {
							return
						}
					case <-timer.C:
						return
					}
				}
			}()
			execErr = fmt.Errorf("客户端断开连接")
			break
		}
	}

	// Get session error
	if execErr == nil {
		if err := <-errCh; err != nil {
			execErr = err
		}
	} else {
		// 客户端已断开，设置超时防止阻塞（SSH 命令可能长时间运行）
		select {
		case <-errCh:
		case <-time.After(30 * time.Second):
			log.Printf("[WS] Drain timeout, SSH session may still be running")
		}
	}

	// Send completion status
	if execErr != nil {
		sc.WriteJSON(WSMessage{Type: "error", Message: execErr.Error()})
		logEntry.Status = "failed"
	} else {
		sc.WriteJSON(WSMessage{Type: "done", Status: "success"})
		logEntry.Status = "success"
	}

	// Write log and cleanup
	outputStr := strings.Join(allOutput, "\n")
	logEntry.Output = outputStr
	if err := h.db.Create(&logEntry).Error; err != nil {
		log.Printf("[WARN] 审计日志写入失败: %v (module=preprod, action=%s)", err, preview.Action)
	}
	outputAuditJSON(&logEntry)
}
