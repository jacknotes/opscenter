package service

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const lockKeyPrefix = "opscenter:lock:"

// DefaultLockTimeout 是分布式锁的默认超时时间。
const DefaultLockTimeout = 10 * time.Minute

// LockInfo 存储锁的持有者信息和过期时间。
type LockInfo struct {
	Username  string
	LockedAt  time.Time
	ExpiresAt time.Time
}

type lockInfoJSON struct {
	Username    string `json:"username"`
	LockedAt    string `json:"locked_at"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

// TryLock Lua 脚本：原子性检查并获取锁
var tryLockScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false then
    redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
    return 1
end
local data = cjson.decode(current)
if data.expires_at_ms < tonumber(ARGV[3]) then
    redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
    return 1
end
return 0
`)

// Unlock Lua 脚本：仅锁持有者可释放
var unlockScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false then return 0 end
local data = cjson.decode(current)
if data.username == ARGV[1] then
    redis.call('DEL', KEYS[1])
    return 1
end
return 0
`)

// LockManager 是基于 Redis 的分布式锁管理器，按服务器 ID 加锁，防止并发操作。
type LockManager struct {
	rdb *redis.Client
}

// NewLockManager 创建锁管理器。
func NewLockManager(rdb *redis.Client) *LockManager {
	return &LockManager{rdb: rdb}
}

// TryLock 尝试获取指定服务器的锁。
// 返回 (true, nil) 表示成功获取；返回 (false, lockInfo) 表示锁已被他人持有。
func (lm *LockManager) TryLock(serverID uint, username string, timeout time.Duration) (bool, *LockInfo) {
	now := time.Now()
	lockJSON := lockInfoJSON{
		Username:    username,
		LockedAt:    now.Format(time.RFC3339Nano),
		ExpiresAtMS: now.Add(timeout).UnixMilli(),
	}
	data, _ := json.Marshal(lockJSON)

	key := lockKeyPrefix + strconv.FormatUint(uint64(serverID), 10)
	result, err := tryLockScript.Run(context.Background(), lm.rdb,
		[]string{key}, data, timeout.Milliseconds(), now.UnixMilli()).Int()

	if err != nil {
		return false, nil
	}

	if result == 1 {
		return true, nil
	}

	// 锁被他人持有，获取当前锁信息
	val, err := lm.rdb.Get(context.Background(), key).Bytes()
	if err != nil {
		return false, nil
	}
	var existing lockInfoJSON
	if err := json.Unmarshal(val, &existing); err != nil {
		return false, nil
	}
	lockedAt, _ := time.Parse(time.RFC3339Nano, existing.LockedAt)
	expiresAt := time.UnixMilli(existing.ExpiresAtMS)
	return false, &LockInfo{
		Username:  existing.Username,
		LockedAt:  lockedAt,
		ExpiresAt: expiresAt,
	}
}

// Unlock 释放指定服务器的锁，仅锁的持有者可以释放。
func (lm *LockManager) Unlock(serverID uint, username string) bool {
	key := lockKeyPrefix + strconv.FormatUint(uint64(serverID), 10)
	result, err := unlockScript.Run(context.Background(), lm.rdb,
		[]string{key}, username).Int()
	if err != nil {
		return false
	}
	return result == 1
}

// IsLocked 检查指定服务器是否被锁定。
func (lm *LockManager) IsLocked(serverID uint) (bool, *LockInfo) {
	key := lockKeyPrefix + strconv.FormatUint(uint64(serverID), 10)
	val, err := lm.rdb.Get(context.Background(), key).Bytes()
	if err != nil {
		return false, nil
	}
	var info lockInfoJSON
	if err := json.Unmarshal(val, &info); err != nil {
		return false, nil
	}
	lockedAt, _ := time.Parse(time.RFC3339Nano, info.LockedAt)
	expiresAt := time.UnixMilli(info.ExpiresAtMS)
	lockInfo := &LockInfo{
		Username:  info.Username,
		LockedAt:  lockedAt,
		ExpiresAt: expiresAt,
	}
	if time.Now().After(lockInfo.ExpiresAt) {
		lm.rdb.Del(context.Background(), key)
		return false, nil
	}
	return true, lockInfo
}

// Stop 无操作，Redis 版本由 TTL 自动过期，无需手动清理。
func (lm *LockManager) Stop() {}
