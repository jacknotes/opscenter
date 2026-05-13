package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
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

type WSHandler struct {
	db         *gorm.DB
	sshManager *service.SSHManager
	clients    sync.Map
}

func NewWSHandler(db *gorm.DB, sshManager *service.SSHManager) *WSHandler {
	return &WSHandler{
		db:         db,
		sshManager: sshManager,
	}
}

type ExecRequest struct {
	ServerID uint   `json:"server_id"`
	Command  string `json:"command"`
}

func (h *WSHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Read message failed: %v", err)
		return
	}

	var req ExecRequest
	if err := json.Unmarshal(message, &req); err != nil {
		conn.WriteJSON(gin.H{"error": "Invalid request"})
		return
	}

	var server model.Server
	if err := h.db.First(&server, req.ServerID).Error; err != nil {
		conn.WriteJSON(gin.H{"error": "Server not found"})
		return
	}

	output, err := h.sshManager.Execute(&server, req.Command)
	if err != nil {
		conn.WriteJSON(gin.H{"error": err.Error(), "output": output})
		return
	}

	conn.WriteJSON(gin.H{"output": output, "status": "success"})
}
