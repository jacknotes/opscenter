package middleware

import (
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
