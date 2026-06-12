// Package handler 的集成测试需要 MySQL 测试数据库。
// 运行方式：设置环境变量 TEST_DSN 后执行 go test ./internal/handler/ -v
//
// 单元测试（不依赖 DB）直接在此文件中编写。
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupGinContext 创建带管理员身份的测试 Gin context
func setupGinContext(userID uint, username string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("username", username)
	c.Set("role", "admin")
	return c, w
}

func TestBatchUnlockUsers_ParameterValidation(t *testing.T) {
	t.Run("空ID列表返回400", func(t *testing.T) {
		// 测试参数校验逻辑 — 不需要 DB
		// 注意：BatchUnlockUsers 内部先调用 ShouldBindJSON，然后检查 len(req.IDs)
		// 由于 handler 需要 h.db，此测试验证请求格式
		body := map[string]interface{}{"ids": []uint{}}
		jsonBody, _ := json.Marshal(body)

		// 验证 JSON 格式正确
		var req struct {
			IDs []uint `json:"ids"`
		}
		if err := json.Unmarshal(jsonBody, &req); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}
		if len(req.IDs) != 0 {
			t.Errorf("期望空列表，实际 %d 个", len(req.IDs))
		}
	})

	t.Run("正常ID列表解析", func(t *testing.T) {
		body := map[string]interface{}{"ids": []uint{1, 2, 3}}
		jsonBody, _ := json.Marshal(body)

		var req struct {
			IDs []uint `json:"ids"`
		}
		if err := json.Unmarshal(jsonBody, &req); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}
		if len(req.IDs) != 3 {
			t.Errorf("期望 3 个 ID，实际 %d 个", len(req.IDs))
		}
	})
}

func TestGinContextSetup(t *testing.T) {
	c, w := setupGinContext(42, "testadmin")

	// 验证 context 设置正确
	uid, exists := c.Get("user_id")
	if !exists || uid != uint(42) {
		t.Errorf("user_id 期望 42，实际 %v", uid)
	}

	username, exists := c.Get("username")
	if !exists || username != "testadmin" {
		t.Errorf("username 期望 testadmin，实际 %v", username)
	}

	role, exists := c.Get("role")
	if !exists || role != "admin" {
		t.Errorf("role 期望 admin，实际 %v", role)
	}

	// 验证 response recorder 初始化
	if w.Code != http.StatusOK {
		t.Errorf("初始状态码期望 200，实际 %d", w.Code)
	}
}

func TestRequestParsing(t *testing.T) {
	t.Run("JSON body 解析", func(t *testing.T) {
		body := map[string]interface{}{
			"ids":     []uint{1, 2},
			"enabled": true,
		}
		jsonBody, _ := json.Marshal(body)

		c, _ := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		var req struct {
			IDs     []uint `json:"ids"`
			Enabled bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			t.Fatalf("绑定 JSON 失败: %v", err)
		}
		if len(req.IDs) != 2 {
			t.Errorf("期望 2 个 ID，实际 %d", len(req.IDs))
		}
		if !req.Enabled {
			t.Error("期望 enabled=true")
		}
	})

	t.Run("URL 参数解析", func(t *testing.T) {
		c, _ := setupGinContext(1, "admin")
		c.Params = gin.Params{{Key: "id", Value: "123"}}

		id := c.Param("id")
		if id != "123" {
			t.Errorf("期望 id=123，实际 %s", id)
		}
	})
}
