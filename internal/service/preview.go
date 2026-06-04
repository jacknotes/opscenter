package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"opscenter/internal/config"
)

const previewKeyPrefix = "opscenter:preview:"

// PreviewData 存储操作预览数据，用于预览 → 执行两步流程。
type PreviewData struct {
	ID        string
	Module    string
	Action    string
	ServerID  uint
	Params    map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

type previewDataJSON struct {
	ID        string                 `json:"id"`
	Module    string                 `json:"module"`
	Action    string                 `json:"action"`
	ServerID  uint                   `json:"server_id"`
	Params    map[string]interface{} `json:"params"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// PreviewManager 管理操作预览的 Redis 存储，使用 UUID 作为键，5 分钟自动过期。
type PreviewManager struct {
	rdb *redis.Client
}

// NewPreviewManager 创建预览管理器。
func NewPreviewManager(rdb *redis.Client) *PreviewManager {
	return &PreviewManager{rdb: rdb}
}

// Create 创建一条预览记录，返回 UUID 作为 preview_id。
func (pm *PreviewManager) Create(module, action string, serverID uint, params map[string]interface{}) string {
	id := uuid.New().String()
	now := time.Now()

	data := previewDataJSON{
		ID:        id,
		Module:    module,
		Action:    action,
		ServerID:  serverID,
		Params:    params,
		CreatedAt: now,
		ExpiresAt: now.Add(config.Global.Timeouts.Preview),
	}

	bytes, _ := json.Marshal(data)
	pm.rdb.Set(context.Background(), previewKeyPrefix+id, bytes, config.Global.Timeouts.Preview)
	return id
}

// Get 根据 preview_id 获取预览记录。
func (pm *PreviewManager) Get(id string) (*PreviewData, bool) {
	val, err := pm.rdb.Get(context.Background(), previewKeyPrefix+id).Bytes()
	if err != nil {
		return nil, false
	}

	var data previewDataJSON
	if err := json.Unmarshal(val, &data); err != nil {
		return nil, false
	}

	return &PreviewData{
		ID:        data.ID,
		Module:    data.Module,
		Action:    data.Action,
		ServerID:  data.ServerID,
		Params:    data.Params,
		CreatedAt: data.CreatedAt,
		ExpiresAt: data.ExpiresAt,
	}, true
}

// Delete 删除指定的预览记录。
func (pm *PreviewManager) Delete(id string) {
	pm.rdb.Del(context.Background(), previewKeyPrefix+id)
}

// ClearAll 清除所有预览数据。服务启动时调用，清除上次运行遗留的预览。
func (pm *PreviewManager) ClearAll() {
	ctx := context.Background()
	iter := pm.rdb.Scan(ctx, 0, previewKeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		pm.rdb.Del(ctx, iter.Val())
	}
}

// Stop 无操作，Redis 版本由 TTL 自动过期，无需手动清理。
func (pm *PreviewManager) Stop() {}
