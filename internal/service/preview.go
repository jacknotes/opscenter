package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

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

// PreviewManager 管理操作预览的内存存储，使用 UUID 作为键，5 分钟自动过期。
type PreviewManager struct {
	previews sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

// NewPreviewManager 创建预览管理器并启动后台清理协程。
func NewPreviewManager() *PreviewManager {
	pm := &PreviewManager{
		stop: make(chan struct{}),
	}
	go pm.cleanup()
	return pm
}

// Create 创建一条预览记录，返回 UUID 作为 preview_id。
func (pm *PreviewManager) Create(module, action string, serverID uint, params map[string]interface{}) string {
	id := uuid.New().String()
	now := time.Now()

	data := &PreviewData{
		ID:        id,
		Module:    module,
		Action:    action,
		ServerID:  serverID,
		Params:    params,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	pm.previews.Store(id, data)
	return id
}

// Get 根据 preview_id 获取预览记录，过期记录会被自动删除。
func (pm *PreviewManager) Get(id string) (*PreviewData, bool) {
	val, ok := pm.previews.Load(id)
	if !ok {
		return nil, false
	}

	data := val.(*PreviewData)
	if time.Now().After(data.ExpiresAt) {
		pm.previews.Delete(id)
		return nil, false
	}

	return data, true
}

// Delete 删除指定的预览记录。
func (pm *PreviewManager) Delete(id string) {
	pm.previews.Delete(id)
}

// Stop 停止后台清理协程。
func (pm *PreviewManager) Stop() {
	pm.stopOnce.Do(func() {
		close(pm.stop)
	})
}

func (pm *PreviewManager) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stop:
			return
		case <-ticker.C:
			now := time.Now()
			pm.previews.Range(func(key, value interface{}) bool {
				data := value.(*PreviewData)
				if now.After(data.ExpiresAt) {
					pm.previews.Delete(key)
				}
				return true
			})
		}
	}
}
