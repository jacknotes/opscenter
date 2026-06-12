package middleware

import (
	"encoding/json"
	"testing"

	"github.com/go-redis/redismock/v9"
)

func TestTokenBlacklist(t *testing.T) {
	db, mock := redismock.NewClientMock()
	InitBlacklist(db)

	jti := "test-jti-123"

	// 初始状态，jti 不在黑名单中
	mock.ExpectExists(blacklistKeyPrefix + jti).SetVal(0)
	if IsBlacklisted(jti) {
		t.Error("新 jti 不应在黑名单中")
	}

	// 加入黑名单
	mock.ExpectSet(blacklistKeyPrefix+jti, "1", 0).SetVal("OK")
	BlacklistToken(jti)

	// 加入后应返回 true
	mock.ExpectExists(blacklistKeyPrefix + jti).SetVal(1)
	if !IsBlacklisted(jti) {
		t.Error("加入黑名单后应返回 true")
	}

	// 另一个 jti 不受影响
	otherJti := "other-jti-456"
	mock.ExpectExists(blacklistKeyPrefix + otherJti).SetVal(0)
	if IsBlacklisted(otherJti) {
		t.Error("未加入的 jti 不应在黑名单中")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的期望: %v", err)
	}
}

func TestGetActiveUserInfo(t *testing.T) {
	db, mock := redismock.NewClientMock()
	InitBlacklist(db)

	username := "testuser"
	info := ActiveUserInfo{
		Role:        "user",
		LoginTime:   "2026-06-12T10:00:00Z",
		LoginMethod: "local",
		LastActive:  "2026-06-12T10:00:00Z",
		JTI:         "test-jti-789",
	}
	data, _ := json.Marshal(info)

	// 用户在线
	mock.ExpectGet(activeUserKeyPrefix + username).SetVal(string(data))
	result := GetActiveUserInfo(username)
	if result == nil {
		t.Fatal("在线用户应返回信息")
	}
	if result.JTI != "test-jti-789" {
		t.Errorf("JTI 期望 test-jti-789，实际 %s", result.JTI)
	}

	// 用户不在线
	mock.ExpectGet(activeUserKeyPrefix + "nobody").RedisNil()
	result = GetActiveUserInfo("nobody")
	if result != nil {
		t.Error("离线用户应返回 nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的期望: %v", err)
	}
}

func TestForceKickUser(t *testing.T) {
	db, mock := redismock.NewClientMock()
	InitBlacklist(db)

	username := "kickme"
	info := ActiveUserInfo{
		Role: "user",
		JTI:  "kick-jti-123",
	}
	data, _ := json.Marshal(info)

	// 用户在线，执行踢下线
	mock.ExpectGet(activeUserKeyPrefix + username).SetVal(string(data))
	mock.ExpectSet(blacklistKeyPrefix+"kick-jti-123", "1", 0).SetVal("OK")
	mock.ExpectDel(activeUserKeyPrefix + username).SetVal(1)
	kicked := ForceKickUser(username)
	if !kicked {
		t.Error("在线用户应被成功踢下线")
	}

	// 用户不在线
	mock.ExpectGet(activeUserKeyPrefix + "nobody").RedisNil()
	kicked = ForceKickUser("nobody")
	if kicked {
		t.Error("离线用户不应被踢下线")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的期望: %v", err)
	}
}
