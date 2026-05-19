package service

import (
	"sync"
	"time"
)

const DefaultLockTimeout = 10 * time.Minute

type LockInfo struct {
	Username  string
	LockedAt  time.Time
	ExpiresAt time.Time
}

type LockManager struct {
	locks    sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

func NewLockManager() *LockManager {
	lm := &LockManager{
		stop: make(chan struct{}),
	}
	go lm.cleanup()
	return lm
}

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
