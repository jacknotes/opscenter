package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"opscenter/internal/middleware"
	"opscenter/internal/model"
)

// getTestDB 连接测试数据库。通过环境变量 TEST_DSN 或使用默认本地 MySQL。
func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		dsn = "opscenter:your-db-password@tcp(uatmysql.hs.com:3306)/opscenter?charset=utf8mb4&parseTime=True&loc=Local&tls=false"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	// 自动迁移确保表结构正确
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}
	return db
}

// cleanupTestUser 清理测试用户（硬删除）
func cleanupTestUser(db *gorm.DB, usernames ...string) {
	for _, u := range usernames {
		db.Unscoped().Where("username = ?", u).Delete(&model.User{})
	}
}

func setupGinContext(userID uint, username string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", userID)
	c.Set("username", username)
	c.Set("role", "admin")
	return c, w
}

func TestUnlockUser(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	lockedName := "test_locked_" + suffix
	unlockedName := "test_unlocked_" + suffix
	adminName := "test_admin_unlock_" + suffix

	// 创建测试用户
	lockedUser := model.User{
		Username:       lockedName,
		Password:       "$2a$10$dummy",
		Name:           "锁定用户",
		Email:          lockedName + "@test.com",
		Role:           "user",
		Enabled:        true,
		FailedAttempts: 5,
		Locked:         true,
	}
	db.Create(&lockedUser)

	unlockedUser := model.User{
		Username:       unlockedName,
		Password:       "$2a$10$dummy",
		Name:           "正常用户",
		Email:          unlockedName + "@test.com",
		Role:           "user",
		Enabled:        true,
		FailedAttempts: 0,
		Locked:         false,
	}
	db.Create(&unlockedUser)

	adminUser := model.User{
		Username:       adminName,
		Password:       "$2a$10$dummy",
		Name:           "管理员",
		Email:          adminName + "@test.com",
		Role:           "admin",
		Enabled:        true,
		FailedAttempts: 0,
		Locked:         false,
	}
	db.Create(&adminUser)

	defer cleanupTestUser(db, lockedName, unlockedName, adminName)

	t.Run("解锁已锁定用户", func(t *testing.T) {
		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("PUT", fmt.Sprintf("/users/%d/unlock", lockedUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", lockedUser.ID)}}

		h.UnlockUser(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d: %s", w.Code, w.Body.String())
		}

		// 验证数据库
		var user model.User
		db.First(&user, lockedUser.ID)
		if user.Locked {
			t.Error("用户应该已解锁")
		}
		if user.FailedAttempts != 0 {
			t.Errorf("失败次数应为 0，实际 %d", user.FailedAttempts)
		}
	})

	t.Run("未锁定用户返回400", func(t *testing.T) {
		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("PUT", fmt.Sprintf("/users/%d/unlock", unlockedUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", unlockedUser.ID)}}

		h.UnlockUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin用户无需解锁", func(t *testing.T) {
		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("PUT", fmt.Sprintf("/users/%d/unlock", adminUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", adminUser.ID)}}

		h.UnlockUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("不存在的用户返回404", func(t *testing.T) {
		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("PUT", "/users/999999/unlock", nil)
		c.Params = gin.Params{{Key: "id", Value: "999999"}}

		h.UnlockUser(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("期望 404，实际 %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestBatchUnlockUsers(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	u1Name := "test_batch_lock1_" + suffix
	u2Name := "test_batch_lock2_" + suffix
	u3Name := "test_batch_unlock_" + suffix

	u1 := model.User{Username: u1Name, Password: "$2a$10$dummy", Name: "锁定1", Email: u1Name + "@test.com", Role: "user", Enabled: true, FailedAttempts: 5, Locked: true}
	u2 := model.User{Username: u2Name, Password: "$2a$10$dummy", Name: "锁定2", Email: u2Name + "@test.com", Role: "user", Enabled: true, FailedAttempts: 3, Locked: true}
	u3 := model.User{Username: u3Name, Password: "$2a$10$dummy", Name: "正常", Email: u3Name + "@test.com", Role: "user", Enabled: true, FailedAttempts: 0, Locked: false}
	db.Create(&u1)
	db.Create(&u2)
	db.Create(&u3)

	defer cleanupTestUser(db, u1Name, u2Name, u3Name)

	t.Run("批量解锁两个锁定用户", func(t *testing.T) {
		body := map[string]interface{}{"ids": []uint{u1.ID, u2.ID}}
		jsonBody, _ := json.Marshal(body)

		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", "/users/batch-unlock", bytes.NewBuffer(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		h.BatchUnlockUsers(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d: %s", w.Code, w.Body.String())
		}

		// 验证
		var r1, r2 model.User
		db.First(&r1, u1.ID)
		db.First(&r2, u2.ID)
		if r1.Locked || r2.Locked {
			t.Error("两个用户都应该已解锁")
		}
		if r1.FailedAttempts != 0 || r2.FailedAttempts != 0 {
			t.Error("失败次数应为 0")
		}
	})

	t.Run("跳过未锁定用户", func(t *testing.T) {
		// 重新锁定 u1
		db.Model(&model.User{}).Where("id = ?", u1.ID).Updates(map[string]interface{}{"locked": true, "failed_attempts": 5})

		body := map[string]interface{}{"ids": []uint{u1.ID, u3.ID}}
		jsonBody, _ := json.Marshal(body)

		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", "/users/batch-unlock", bytes.NewBuffer(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		h.BatchUnlockUsers(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["unlocked"].(float64) != 1 {
			t.Errorf("期望解锁 1 个，实际 %v", resp["unlocked"])
		}
		if resp["failed"].(float64) != 1 {
			t.Errorf("期望失败 1 个，实际 %v", resp["failed"])
		}
	})

	t.Run("空列表返回400", func(t *testing.T) {
		body := map[string]interface{}{"ids": []uint{}}
		jsonBody, _ := json.Marshal(body)

		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", "/users/batch-unlock", bytes.NewBuffer(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		h.BatchUnlockUsers(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestKickUser(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	// 初始化 Redis mock
	redisMock, mock := middleware.NewRedisMock()
	middleware.InitBlacklist(redisMock)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	onlineName := "test_kick_online_" + suffix
	offlineName := "test_kick_offline_" + suffix
	adminName := "test_kick_admin_" + suffix

	onlineUser := model.User{Username: onlineName, Password: "$2a$10$dummy", Name: "在线", Email: onlineName + "@test.com", Role: "user", Enabled: true}
	offlineUser := model.User{Username: offlineName, Password: "$2a$10$dummy", Name: "离线", Email: offlineName + "@test.com", Role: "user", Enabled: true}
	adminUser := model.User{Username: adminName, Password: "$2a$10$dummy", Name: "管理员", Email: adminName + "@test.com", Role: "admin", Enabled: true}
	db.Create(&onlineUser)
	db.Create(&offlineUser)
	db.Create(&adminUser)

	defer cleanupTestUser(db, onlineName, offlineName, adminName)

	t.Run("踢在线用户下线", func(t *testing.T) {
		// Mock: 用户在线
		info := middleware.ActiveUserInfo{Role: "user", JTI: "kick-test-jti"}
		data, _ := json.Marshal(info)
		mock.ExpectGet("opscenter:active_user:" + onlineName).SetVal(string(data))
		mock.ExpectSet("opscenter:blacklist:jti:kick-test-jti", "1", 0).SetVal("OK")
		mock.ExpectDel("opscenter:active_user:" + onlineName).SetVal(1)

		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", fmt.Sprintf("/users/%d/kick", onlineUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", onlineUser.ID)}}

		h.KickUser(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("不能踢admin用户", func(t *testing.T) {
		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", fmt.Sprintf("/users/%d/kick", adminUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", adminUser.ID)}}

		h.KickUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("不能踢自己", func(t *testing.T) {
		// user_id == target id
		c, w := setupGinContext(onlineUser.ID, "admin")
		c.Request = httptest.NewRequest("POST", fmt.Sprintf("/users/%d/kick", onlineUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", onlineUser.ID)}}

		h.KickUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("用户不在线", func(t *testing.T) {
		mock.ExpectGet("opscenter:active_user:" + offlineName).RedisNil()

		c, w := setupGinContext(1, "admin")
		c.Request = httptest.NewRequest("POST", fmt.Sprintf("/users/%d/kick", offlineUser.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", offlineUser.ID)}}

		h.KickUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d: %s", w.Code, w.Body.String())
		}
	})
}
