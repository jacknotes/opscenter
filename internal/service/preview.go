package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type PreviewData struct {
	ID        string
	Module    string
	Action    string
	ServerID  uint
	Params    map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PreviewManager struct {
	previews sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

func NewPreviewManager() *PreviewManager {
	pm := &PreviewManager{
		stop: make(chan struct{}),
	}
	go pm.cleanup()
	return pm
}

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

func (pm *PreviewManager) Delete(id string) {
	pm.previews.Delete(id)
}

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
