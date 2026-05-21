package middleware

import "testing"

func TestTokenBlacklist(t *testing.T) {
	// 初始状态，jti 不在黑名单中
	jti := "test-jti-123"
	if IsBlacklisted(jti) {
		t.Error("新 jti 不应在黑名单中")
	}

	// 加入黑名单
	BlacklistToken(jti)
	if !IsBlacklisted(jti) {
		t.Error("加入黑名单后应返回 true")
	}

	// 另一个 jti 不受影响
	otherJti := "other-jti-456"
	if IsBlacklisted(otherJti) {
		t.Error("未加入的 jti 不应在黑名单中")
	}
}

func TestTokenBlacklist_Concurrent(t *testing.T) {
	// 并发写入测试
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			BlacklistToken("concurrent-jti")
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if !IsBlacklisted("concurrent-jti") {
		t.Error("并发写入后 jti 应在黑名单中")
	}
}
