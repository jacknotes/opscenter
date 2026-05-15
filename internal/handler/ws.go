package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"opscenter/internal/model"
	"opscenter/internal/service"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
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

func (h *WSHandler) Handle(c *gin.Context) {
	username, _ := c.Get("username")
	log.Printf("[WS] Connection from user: %v", username)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v", err)
		return
	}
	log.Printf("[WS] Upgrade success for user: %v", username)

	sc := &safeConn{conn: conn}

	// 防止 panic 导致连接无响应
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS] Panic recovered: %v", r)
			sc.WriteJSON(WSMessage{Type: "error", Message: fmt.Sprintf("服务内部错误: %v", r)})
			sc.Close()
		}
	}()

	connID := uuid.New().String()
	usernameStr, _ := username.(string)
	h.clients.Store(connID, &ConnInfo{
		Username: usernameStr,
		Conn:     conn,
	})
	defer func() {
		h.clients.Delete(connID)
		sc.Close()
	}()

	// Ping/pong heartbeat
	sc.SetReadDeadline(time.Now().Add(60 * time.Second))
	sc.SetPongHandler(func(string) error {
		sc.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
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

	// Read start message
	_, message, err := sc.ReadMessage()
	if err != nil {
		log.Printf("[WS] Read start message failed: %v", err)
		sc.WriteJSON(WSMessage{Type: "error", Message: "读取请求超时或连接已断开"})
		return
	}
	log.Printf("[WS] Received message: %s", string(message))

	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil || msg.Type != "start" || msg.PreviewID == "" {
		log.Printf("[WS] Invalid message: type=%s, previewID=%s, err=%v", msg.Type, msg.PreviewID, err)
		sc.WriteJSON(WSMessage{Type: "error", Message: "无效的请求"})
		return
	}
	log.Printf("[WS] Start execution, previewID: %s", msg.PreviewID)

	// Get preview
	preview, ok := h.previewMgr.Get(msg.PreviewID)
	if !ok {
		log.Printf("[WS] Preview not found or expired: %s", msg.PreviewID)
		sc.WriteJSON(WSMessage{Type: "error", Message: "预览已过期或不存在，请重新预览"})
		return
	}
	log.Printf("[WS] Preview found, module: %s, action: %s, serverID: %d", preview.Module, preview.Action, preview.ServerID)

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
	log.Printf("[WS] Trying lock for server %d, user %s", preview.ServerID, usernameStr)
	locked, holder := h.lockManager.TryLock(preview.ServerID, usernameStr, 10*time.Minute)
	if !locked {
		log.Printf("[WS] Lock denied, held by: %s", holder.Username)
		sc.WriteJSON(WSMessage{
			Type:    "lock_error",
			Message: fmt.Sprintf("操作正在进行中，请等待 (当前操作人: %s)", holder.Username),
			Holder:  holder.Username,
		})
		return
	}
	log.Printf("[WS] Lock acquired for server %d", preview.ServerID)

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

	logEntry := model.OperationLog{
		Username:   usernameStr,
		Module:     "preprod",
		Action:     preview.Action,
		Target:     command,
		Detail:     command,
		PreviewID:  msg.PreviewID,
		ServerID:   server.ID,
		ServerName: server.Name,
	}

	// Stream execution
	log.Printf("[WS] Starting stream execution: %s", command)
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
				for range outputCh {
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
		<-errCh
	}

	// Send completion status
	if execErr != nil {
		log.Printf("[WS] Execution failed: %v, output lines: %d", execErr, len(allOutput))
		sc.WriteJSON(WSMessage{Type: "error", Message: execErr.Error()})
		logEntry.Status = "failed"
	} else {
		log.Printf("[WS] Execution success, output lines: %d", len(allOutput))
		sc.WriteJSON(WSMessage{Type: "done", Status: "success"})
		logEntry.Status = "success"
	}

	// Write log and cleanup
	outputStr := ""
	for _, line := range allOutput {
		outputStr += line + "\n"
	}
	logEntry.Output = outputStr
	h.db.Create(&logEntry)
	h.previewMgr.Delete(msg.PreviewID)
	h.lockManager.Unlock(preview.ServerID, usernameStr)
}
