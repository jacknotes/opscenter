package service

import (
	"sync"
	"time"
)

// DefaultLockTimeout 是分布式锁的默认超时时间。
const DefaultLockTimeout = 10 * time.Minute

// LockInfo 存储锁的持有者信息和过期时间。
type LockInfo struct {
	Username  string
	LockedAt  time.Time
	ExpiresAt time.Time
}

// LockManager 是基于 sync.Map + CAS 的分布式锁管理器，按服务器 ID 加锁，防止并发操作。
type LockManager struct {
	locks    sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

// NewLockManager 创建锁管理器并启动后台清理协程。
func NewLockManager() *LockManager {
	lm := &LockManager{
		stop: make(chan struct{}),
	}
	go lm.cleanup()
	return lm
}

// TryLock 尝试获取指定服务器的锁。使用 CAS 操作保证原子性。
// 返回 (true, nil) 表示成功获取；返回 (false, lockInfo) 表示锁已被他人持有。
func (lm *LockManager) TryLock(serverID uint, username string, timeout time.Duration) (bool, *LockInfo) {
	now := time.Now()
	newLock := &LockInfo{
		Username:  username,
		LockedAt:  now,
		ExpiresAt: now.Add(timeout),
	}

	for {
		val, loaded := lm.locks.Load(serverID)
		if !loaded {
			// No lock exists, try to store
			_, loaded = lm.locks.LoadOrStore(serverID, newLock)
			if !loaded {
				return true, nil
			}
			// Someone else stored concurrently, retry
			continue
		}

		existing := val.(*LockInfo)
		if now.After(existing.ExpiresAt) {
			// Lock expired, try to delete and replace
			if lm.locks.CompareAndDelete(serverID, val) {
				_, loaded = lm.locks.LoadOrStore(serverID, newLock)
				if !loaded {
					return true, nil
				}
			}
			// CAS failed, retry
			continue
		}

		// Lock is held and not expired
		return false, existing
	}
}

// Unlock 释放指定服务器的锁，仅锁的持有者可以释放。
func (lm *LockManager) Unlock(serverID uint, username string) bool {
	val, ok := lm.locks.Load(serverID)
	if !ok {
		return false
	}
	info := val.(*LockInfo)
	if info.Username != username {
		return false
	}
	return lm.locks.CompareAndDelete(serverID, val)
}

// IsLocked 检查指定服务器是否被锁定，过期锁会被自动清理。
func (lm *LockManager) IsLocked(serverID uint) (bool, *LockInfo) {
	val, ok := lm.locks.Load(serverID)
	if !ok {
		return false, nil
	}
	info := val.(*LockInfo)
	if time.Now().After(info.ExpiresAt) {
		lm.locks.CompareAndDelete(serverID, val)
		return false, nil
	}
	return true, info
}

// Stop 停止后台清理协程。
func (lm *LockManager) Stop() {
	lm.stopOnce.Do(func() {
		close(lm.stop)
	})
}

func (lm *LockManager) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lm.stop:
			return
		case <-ticker.C:
			now := time.Now()
			lm.locks.Range(func(key, value interface{}) bool {
				info := value.(*LockInfo)
				if now.After(info.ExpiresAt) {
					lm.locks.Delete(key)
				}
				return true
			})
		}
	}
}
