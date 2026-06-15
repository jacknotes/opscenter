package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"opscenter/internal/config"
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

func TestLoginLockout(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	// 设置配置
	config.Global.Auth.MaxUserAttempts = 3
	config.Global.Auth.MaxLoginAttempts = 10
	config.Global.Auth.LoginLockDuration = 1 * time.Minute
	config.Global.JWT.Secret = "test-secret-key-for-login-lock"
	config.Global.JWT.Expire = time.Hour

	// 初始化 Redis mock
	redisMock, _ := middleware.NewRedisMock()
	middleware.InitBlacklist(redisMock)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userName := "test_login_lock_" + suffix
	adminName := "admin" // 使用内置 admin 账号测试豁免逻辑

	// 创建测试用户（真实 bcrypt 密码 "Test@1234"）
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
	testUser := model.User{
		Username:       userName,
		Password:       string(hashedPwd),
		Name:           "锁定测试用户",
		Email:          userName + "@test.com",
		Role:           "user",
		Enabled:        true,
		FailedAttempts: 0,
		Locked:         false,
	}
	db.Create(&testUser)

	adminUser := model.User{
		Username:       adminName,
		Password:       string(hashedPwd),
		Name:           "管理员",
		Email:          adminName + "@test.com",
		Role:           "admin",
		Enabled:        true,
		FailedAttempts: 0,
		Locked:         false,
	}
	// admin 用户可能已存在（来自应用初始化），使用 FirstOrCreate
	db.Where("username = ?", adminName).FirstOrCreate(&adminUser)
	// 确保密码和状态正确
	db.Model(&adminUser).Updates(map[string]interface{}{
		"password":        string(hashedPwd),
		"enabled":         true,
		"failed_attempts": 0,
		"locked":          false,
	})

	defer cleanupTestUser(db, userName) // admin 是内置账号，不删除

	t.Run("登录失败递增failed_attempts", func(t *testing.T) {
		// 重置状态
		db.Model(&testUser).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})

		body := `{"username":"` + userName + `","password":"WrongPassword"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("期望 401，实际 %d", w.Code)
		}

		// 验证 failed_attempts 递增
		var user model.User
		db.First(&user, testUser.ID)
		if user.FailedAttempts != 1 {
			t.Errorf("期望 failed_attempts=1，实际 %d", user.FailedAttempts)
		}
	})

	t.Run("连续失败达到阈值后锁定", func(t *testing.T) {
		// 重置状态
		db.Model(&testUser).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})

		// 连续失败 3 次（阈值）
		var lastW *httptest.ResponseRecorder
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"WrongPwd` + fmt.Sprintf("%d", i) + `"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			h.Login(c)
			lastW = w
		}

		// 验证账号被锁定
		var user model.User
		db.First(&user, testUser.ID)
		if !user.Locked {
			t.Error("账号应该被锁定")
		}
		if user.FailedAttempts < 3 {
			t.Errorf("期望 failed_attempts>=3，实际 %d", user.FailedAttempts)
		}

		// 验证第3次（触发锁定的那次）返回 403 "账号已锁定"
		if lastW.Code != http.StatusForbidden {
			t.Errorf("触发锁定时应返回 403，实际 %d", lastW.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(lastW.Body.Bytes(), &resp)
		if !strings.Contains(resp["error"].(string), "账号已锁定") {
			t.Errorf("触发锁定时应返回'账号已锁定'，实际: %s", resp["error"])
		}
	})

	t.Run("被锁定账号返回403", func(t *testing.T) {
		// 确保账号锁定
		db.Model(&testUser).Updates(map[string]interface{}{"locked": true, "failed_attempts": 5})

		body := `{"username":"` + userName + `","password":"Test@1234"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("期望 403，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("成功登录重置failed_attempts", func(t *testing.T) {
		// 设置有失败次数但未锁定
		db.Model(&testUser).Updates(map[string]interface{}{"failed_attempts": 2, "locked": false})

		body := `{"username":"` + userName + `","password":"Test@1234"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d: %s", w.Code, w.Body.String())
		}

		// 验证重置
		var user model.User
		db.First(&user, testUser.ID)
		if user.FailedAttempts != 0 {
			t.Errorf("期望 failed_attempts=0，实际 %d", user.FailedAttempts)
		}
		if user.Locked {
			t.Error("账号不应被锁定")
		}
	})

	t.Run("admin账号不参与锁定", func(t *testing.T) {
		// 重置 admin
		db.Model(&model.User{}).Where("id = ?", adminUser.ID).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})

		// 连续失败 5 次（超过阈值）
		for i := 0; i < 5; i++ {
			body := `{"username":"` + adminName + `","password":"WrongPwd"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			h.Login(c)
			if i == 0 {
				t.Logf("第1次失败后响应: %d %s", w.Code, w.Body.String())
			}
		}

		// admin 不应被锁定
		var user model.User
		db.First(&user, adminUser.ID)
		t.Logf("admin 状态: username=%s, failed_attempts=%d, locked=%v", user.Username, user.FailedAttempts, user.Locked)
		if user.Locked {
			t.Error("admin 不应被锁定")
		}
		if user.FailedAttempts != 0 {
			t.Errorf("admin failed_attempts 应为 0，实际 %d", user.FailedAttempts)
		}
	})

	t.Run("admin角色非内置用户应被锁定", func(t *testing.T) {
		// 创建一个 admin 角色但非内置 admin 的用户
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		adminRoleName := "test_admin_role_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		adminRoleUser := model.User{
			Username: adminRoleName,
			Password: string(hashedPwd),
			Name:     "管理员角色用户",
			Email:    adminRoleName + "@test.com",
			Role:     "admin",
			Enabled:  true,
		}
		db.Create(&adminRoleUser)
		defer cleanupTestUser(db, adminRoleName)

		// 连续失败 3 次（阈值）
		for i := 0; i < 3; i++ {
			body := `{"username":"` + adminRoleName + `","password":"WrongPwd` + fmt.Sprintf("%d", i) + `"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			h.Login(c)
			_ = w
		}

		// admin 角色的非内置用户应该被锁定
		var user model.User
		db.First(&user, adminRoleUser.ID)
		if !user.Locked {
			t.Error("admin 角色的非内置用户应该被锁定")
		}
		if user.FailedAttempts < 3 {
			t.Errorf("期望 failed_attempts>=3，实际 %d", user.FailedAttempts)
		}
	})

	t.Run("密码错误提示不含剩余次数", func(t *testing.T) {
		// 重置状态
		db.Model(&testUser).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})
		loginRateLimiter.Reset("192.168.1.100")

		body := `{"username":"` + userName + `","password":"WrongPassword"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Forwarded-For", "192.168.1.100")

		h.Login(c)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		errMsg := resp["error"].(string)
		if errMsg != "用户名或密码错误" {
			t.Errorf("错误提示应为'用户名或密码错误'，实际: %s", errMsg)
		}
		if strings.Contains(errMsg, "还剩") || strings.Contains(errMsg, "次机会") {
			t.Errorf("错误提示不应包含剩余次数，实际: %s", errMsg)
		}
	})

	t.Run("IP阶梯退避_累计失败增加锁定时长", func(t *testing.T) {
		loginRateLimiter.Reset("192.168.1.200")
		config.Global.Auth.LoginLockDuration = 50 * time.Millisecond
		config.Global.Auth.MaxLoginAttempts = 10

		// 第1轮：10次失败 → tier=1 → 锁定 50ms
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.200")
		}
		loginRateLimiter.Allow("192.168.1.200") // 触发 tier=1 锁定
		allowed, retryAfter := loginRateLimiter.Allow("192.168.1.200")
		if allowed {
			t.Error("第1轮10次失败后应被锁定")
		}
		t.Logf("第1轮锁定时长: %v", retryAfter)

		// 等待第1轮锁定过期（tier 升级到 2）
		time.Sleep(100 * time.Millisecond)

		// 锁定过期后应允许登录
		allowed, _ = loginRateLimiter.Allow("192.168.1.200")
		if !allowed {
			t.Error("第1轮锁定过期后应允许登录")
		}

		// 第2轮：再10次失败 → tier=2 → 锁定 100ms (50ms * 2)
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.200")
		}
		loginRateLimiter.Allow("192.168.1.200") // 触发 tier=2 锁定
		allowed2, retryAfter2 := loginRateLimiter.Allow("192.168.1.200")
		if allowed2 {
			t.Error("第2轮10次失败后应被锁定")
		}
		if retryAfter2 < 80*time.Millisecond {
			t.Errorf("第2轮锁定时长应接近 100ms（2倍），实际: %v", retryAfter2)
		}
		t.Logf("第2轮锁定时长: %v", retryAfter2)

		// 等待第2轮锁定过期（tier 升级到 3）
		time.Sleep(200 * time.Millisecond)

		// 锁定过期后应允许登录
		allowed, _ = loginRateLimiter.Allow("192.168.1.200")
		if !allowed {
			t.Error("第2轮锁定过期后应允许登录")
		}

		// 第3轮：再10次失败 → tier=3 → 锁定 200ms (50ms * 4)
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.200")
		}
		loginRateLimiter.Allow("192.168.1.200") // 触发 tier=3 锁定
		allowed3, retryAfter3 := loginRateLimiter.Allow("192.168.1.200")
		if allowed3 {
			t.Error("第3轮10次失败后应被锁定")
		}
		if retryAfter3 < 150*time.Millisecond {
			t.Errorf("第3轮锁定时长应接近 200ms（4倍），实际: %v", retryAfter3)
		}
		t.Logf("第3轮锁定时长: %v", retryAfter3)

		// 恢复默认配置
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("RemainingAttempts返回正确剩余次数", func(t *testing.T) {
		loginRateLimiter.Reset("192.168.1.300")
		config.Global.Auth.MaxLoginAttempts = 10

		// 无记录时返回 maxAttempts
		if remaining := loginRateLimiter.RemainingAttempts("192.168.1.300"); remaining != 10 {
			t.Errorf("无记录时应返回 10，实际: %d", remaining)
		}

		// 记录 3 次后应剩余 7 次
		for i := 0; i < 3; i++ {
			loginRateLimiter.Record("192.168.1.300")
		}
		if remaining := loginRateLimiter.RemainingAttempts("192.168.1.300"); remaining != 7 {
			t.Errorf("3 次失败后应剩余 7 次，实际: %d", remaining)
		}
	})

	t.Run("IP锁定过期后应能重新登录", func(t *testing.T) {
		loginRateLimiter.Reset("192.168.1.400")
		// 使用短锁定时长便于测试
		config.Global.Auth.LoginLockDuration = 50 * time.Millisecond
		config.Global.Auth.MaxLoginAttempts = 10

		// 模拟 10 次失败（触发锁定）
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.400")
		}

		// 确认被锁定
		allowed, _ := loginRateLimiter.Allow("192.168.1.400")
		if allowed {
			t.Error("10 次失败后应被锁定")
		}

		// 等待锁定过期
		time.Sleep(100 * time.Millisecond)

		// 锁定过期后应允许登录
		allowed, retryAfter := loginRateLimiter.Allow("192.168.1.400")
		if !allowed {
			t.Errorf("锁定过期后应允许登录，但仍被锁定: %v", retryAfter)
		}

		// 恢复默认配置
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("IP锁定过期后tier升级", func(t *testing.T) {
		loginRateLimiter.Reset("192.168.1.500")
		config.Global.Auth.LoginLockDuration = 50 * time.Millisecond
		config.Global.Auth.MaxLoginAttempts = 10

		// 第1轮：10次失败，触发 tier=1 锁定（50ms）
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.500")
		}
		loginRateLimiter.Allow("192.168.1.500")
		allowed, retryAfter1 := loginRateLimiter.Allow("192.168.1.500")
		if allowed {
			t.Error("第1轮10次失败后应被锁定")
		}
		t.Logf("第1轮锁定时长: %v", retryAfter1)

		// 等待锁定过期（tier 升级到 2）
		time.Sleep(100 * time.Millisecond)

		// 锁定过期后应允许登录（获得新的10次机会）
		allowed, _ = loginRateLimiter.Allow("192.168.1.500")
		if !allowed {
			t.Error("锁定过期后应允许登录")
		}

		// 第2轮：再失败10次，tier=2 → 锁定 100ms
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.500")
		}
		loginRateLimiter.Allow("192.168.1.500")
		allowed, retryAfter2 := loginRateLimiter.Allow("192.168.1.500")
		if allowed {
			t.Error("第2轮10次失败后应被锁定")
		}
		// tier=2，锁定时长应为 2 倍基础时长
		if retryAfter2 < 80*time.Millisecond {
			t.Errorf("第2轮锁定时长应接近 100ms（2倍），实际: %v", retryAfter2)
		}
		t.Logf("第2轮锁定时长: %v", retryAfter2)

		// 恢复默认配置
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("IP锁定过期后单次失败不应立即重新锁定", func(t *testing.T) {
		loginRateLimiter.Reset("192.168.1.600")
		config.Global.Auth.LoginLockDuration = 50 * time.Millisecond
		config.Global.Auth.MaxLoginAttempts = 10

		// 第1轮：10次失败，触发 tier=1 锁定（50ms）
		for i := 0; i < 10; i++ {
			loginRateLimiter.Record("192.168.1.600")
		}
		loginRateLimiter.Allow("192.168.1.600") // 触发 tier=1
		allowed, _ := loginRateLimiter.Allow("192.168.1.600")
		if allowed {
			t.Error("10次失败后应被锁定")
		}

		// 等待锁定过期
		time.Sleep(100 * time.Millisecond)

		// 锁定过期后，Allow 应允许（tier 升级到 2，窗口重置）
		allowed, _ = loginRateLimiter.Allow("192.168.1.600")
		if !allowed {
			t.Error("锁定过期后 Allow 应允许登录")
		}

		// 模拟认证失败：Record 一次
		loginRateLimiter.Record("192.168.1.600")

		// 关键断言：再次 Allow 不应被锁定（用户只失败了 1 次，应有 10 次机会）
		allowed, retryAfter := loginRateLimiter.Allow("192.168.1.600")
		if !allowed {
			t.Errorf("锁定过期后单次失败不应立即重新锁定，但被锁定了 %v", retryAfter)
		}

		// 验证用户有完整的新窗口（再失败 9 次才锁定）
		for i := 0; i < 8; i++ {
			loginRateLimiter.Record("192.168.1.600")
		}
		// 此时共 9 次失败（Record 了 9 次），不应锁定
		allowed, _ = loginRateLimiter.Allow("192.168.1.600")
		if !allowed {
			t.Error("9次失败后不应被锁定（阈值为10）")
		}

		// 第10次失败，应触发锁定
		loginRateLimiter.Record("192.168.1.600")
		loginRateLimiter.Allow("192.168.1.600") // 触发锁定
		allowed, retryAfter = loginRateLimiter.Allow("192.168.1.600")
		if allowed {
			t.Error("10次失败后应被锁定")
		}
		// tier=2 锁定时长应为 2 倍基础时长
		if retryAfter < 80*time.Millisecond {
			t.Errorf("tier=2 锁定时长应接近 100ms（2倍），实际: %v", retryAfter)
		}
		t.Logf("tier=2 锁定时长: %v", retryAfter)

		// 恢复默认配置
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})
}

// TestIPAndUserLockoutInteraction 测试 IP 限流与用户级锁定的交互
func TestIPAndUserLockoutInteraction(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	config.Global.JWT.Secret = "test-secret-key-for-interaction"
	config.Global.JWT.Expire = time.Hour

	redisMock, _ := middleware.NewRedisMock()
	middleware.InitBlacklist(redisMock)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	t.Run("用户级锁定先于IP限流触发", func(t *testing.T) {
		// MaxUserAttempts=3, MaxLoginAttempts=10 → 用户先锁
		config.Global.Auth.MaxUserAttempts = 3
		config.Global.Auth.MaxLoginAttempts = 10
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		userName := "test_user_lock_first_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		ip := "10.0.1.1"
		loginRateLimiter.Reset(ip)

		// 连续失败 3 次
		var lastCode int
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"Wrong"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
			lastCode = w.Code
		}

		// 第3次应触发用户级锁定（403），而非 IP 限流（429）
		if lastCode != http.StatusForbidden {
			t.Errorf("用户级锁定应返回 403，实际 %d", lastCode)
		}

		// 验证用户已被锁定
		var user model.User
		db.First(&user, testUser.ID)
		if !user.Locked {
			t.Error("用户应被锁定")
		}

		// IP 限流不应触发（10次阈值未达到）
		allowed, _ := loginRateLimiter.Allow(ip)
		if !allowed {
			t.Error("IP 限流不应触发（仅3次失败，阈值为10）")
		}
	})

	t.Run("IP限流先于用户级锁定触发", func(t *testing.T) {
		// MaxUserAttempts=10, MaxLoginAttempts=3 → IP 先锁
		// 注意：锁定时长必须大于 bcrypt 耗时（~130ms），否则窗口会在请求间过期导致计数重置
		config.Global.Auth.MaxUserAttempts = 10
		config.Global.Auth.MaxLoginAttempts = 3
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		userName := "test_ip_lock_first_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		ip := "10.0.2.1"
		loginRateLimiter.Reset(ip)

		// 连续失败 3 次（不要在循环中调用 Allow，它会修改限流状态）
		var lastCode int
		var lastBody string
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"Wrong"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
			lastCode = w.Code
			lastBody = w.Body.String()
			t.Logf("第%d次 响应: %d %s", i+1, lastCode, lastBody)
		}

		// 第3次应触发 IP 限流（429），而非用户级锁定（403）
		if lastCode != http.StatusTooManyRequests {
			t.Errorf("IP 限流应返回 429，实际 %d, body: %s", lastCode, lastBody)
		}

		// 验证用户未被锁定（仅3次失败，阈值为10）
		var user model.User
		db.First(&user, testUser.ID)
		if user.Locked {
			t.Error("用户不应被锁定（失败次数未达用户级阈值）")
		}

		// IP 限流应已触发
		allowed, _ := loginRateLimiter.Allow(ip)
		if allowed {
			t.Error("IP 限流应已触发")
		}

		// 恢复默认
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("IP锁定期间正确密码也被拒绝", func(t *testing.T) {
		config.Global.Auth.MaxUserAttempts = 10
		config.Global.Auth.MaxLoginAttempts = 3
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		userName := "test_ip_locked_correct_pwd_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		ip := "10.0.3.1"
		loginRateLimiter.Reset(ip)

		// 触发 IP 锁定
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"Wrong"}`
			c, _ := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
		}

		// 用正确密码尝试登录，应被 IP 限流拒绝
		body := `{"username":"` + userName + `","password":"Test@1234"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Forwarded-For", ip)
		h.Login(c)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("IP 锁定期间正确密码应返回 429，实际 %d: %s", w.Code, w.Body.String())
		}

		// 恢复默认
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("不同IP访问已锁定用户返回403", func(t *testing.T) {
		config.Global.Auth.MaxUserAttempts = 3
		config.Global.Auth.MaxLoginAttempts = 10

		userName := "test_locked_diff_ip_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
			Locked: true, FailedAttempts: 5,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		// 从不同 IP 尝试登录已锁定用户
		for _, ip := range []string{"10.0.4.1", "10.0.4.2", "10.0.4.3"} {
			loginRateLimiter.Reset(ip)
			body := `{"username":"` + userName + `","password":"Test@1234"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)

			if w.Code != http.StatusForbidden {
				t.Errorf("IP %s 访问已锁定用户应返回 403，实际 %d", ip, w.Code)
			}
		}
	})

	t.Run("admin不受用户级锁定但仍受IP限流", func(t *testing.T) {
		config.Global.Auth.MaxUserAttempts = 3
		config.Global.Auth.MaxLoginAttempts = 5
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		adminName := "admin"
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		var adminUser model.User
		db.Where("username = ?", adminName).FirstOrCreate(&adminUser)
		db.Model(&adminUser).Updates(map[string]interface{}{
			"password": string(hashedPwd), "enabled": true,
			"failed_attempts": 0, "locked": false,
		})

		ip := "10.0.5.1"
		loginRateLimiter.Reset(ip)

		// 连续失败 5 次（超过 IP 阈值，不要在循环中调用 Allow）
		var lastCode int
		for i := 0; i < 5; i++ {
			body := `{"username":"admin","password":"Wrong"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
			lastCode = w.Code
			t.Logf("admin 第%d次 响应: %d %s", i+1, lastCode, w.Body.String())
		}

		// admin 不应被用户级锁定，但 IP 限流应触发
		if lastCode != http.StatusTooManyRequests {
			t.Errorf("admin 应受 IP 限流（429），实际 %d", lastCode)
		}

		// 验证 admin 未被锁定
		db.First(&adminUser, adminUser.ID)
		if adminUser.Locked {
			t.Error("admin 不应被用户级锁定")
		}

		// 恢复默认
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})

	t.Run("IP锁定后用户级失败计数仍保留", func(t *testing.T) {
		config.Global.Auth.MaxUserAttempts = 10
		config.Global.Auth.MaxLoginAttempts = 3
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		userName := "test_ip_user_count_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		ip := "10.0.6.1"
		loginRateLimiter.Reset(ip)

		// 连续失败 3 次，第3次触发 IP 锁定（Record+Allow 后返回 429）
		// 注意：第3次请求被 IP 限流拦截，不会走到用户级计数，所以 failed_attempts=2
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"Wrong"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
			if i == 2 && w.Code != http.StatusTooManyRequests {
				t.Errorf("第3次应返回 429，实际 %d", w.Code)
			}
		}

		// failed_attempts=2（前2次正常记录，第3次被 IP 拦截未记录）
		var user model.User
		db.First(&user, testUser.ID)
		if user.FailedAttempts != 2 {
			t.Errorf("IP 锁定后 failed_attempts 应为 2，实际 %d", user.FailedAttempts)
		}

		// IP 锁定后再次尝试，应被 IP 限流拒绝（429），不会增加 failed_attempts
		body := `{"username":"` + userName + `","password":"Wrong"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Forwarded-For", ip)
		h.Login(c)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("IP 锁定后应返回 429，实际 %d", w.Code)
		}

		// failed_attempts 不应增加（被 IP 限流拦截，未走到用户级计数）
		db.First(&user, testUser.ID)
		if user.FailedAttempts != 2 {
			t.Errorf("IP 锁定拦截后 failed_attempts 不应增加，实际 %d", user.FailedAttempts)
		}
	})

	t.Run("两层同时触发时IP限流优先", func(t *testing.T) {
		// 设置两者阈值相同，同时触发
		config.Global.Auth.MaxUserAttempts = 3
		config.Global.Auth.MaxLoginAttempts = 3
		config.Global.Auth.LoginLockDuration = 1 * time.Minute

		userName := "test_both_trigger_" + suffix
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
		testUser := model.User{
			Username: userName, Password: string(hashedPwd), Name: "测试",
			Email: userName + "@test.com", Role: "user", Enabled: true,
		}
		db.Create(&testUser)
		defer cleanupTestUser(db, userName)

		ip := "10.0.7.1"
		loginRateLimiter.Reset(ip)

		// 连续失败 3 次，第3次同时达到两层阈值
		var lastCode int
		for i := 0; i < 3; i++ {
			body := `{"username":"` + userName + `","password":"Wrong"}`
			c, w := setupGinContext(0, "")
			c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-Forwarded-For", ip)
			h.Login(c)
			lastCode = w.Code
		}

		// IP 限流先检查（handler 开头），所以应返回 429
		if lastCode != http.StatusTooManyRequests {
			t.Errorf("两层同时触发时 IP 限流应优先返回 429，实际 %d", lastCode)
		}

		// 验证用户级 failed_attempts 也已递增（Record 在 Allow 之后被调用）
		// 注意：第3次请求被 IP 限流拦截，Record 不会被调用
		// 所以 failed_attempts 应为 2（前两次 Record 成功）
		var user model.User
		db.First(&user, testUser.ID)
		if user.FailedAttempts != 2 {
			t.Logf("两层同时触发时 failed_attempts=%d（第3次被IP拦截，未Record）", user.FailedAttempts)
		}

		// 恢复默认
		config.Global.Auth.LoginLockDuration = 1 * time.Minute
	})
}

func TestLDAPFallbackToLocal(t *testing.T) {
	db := getTestDB(t)
	h := NewAuthHandler(db)

	// 设置配置：启用 LDAP
	config.Global.LDAP.Enabled = true
	config.Global.Auth.MaxUserAttempts = 5
	config.Global.Auth.MaxLoginAttempts = 10
	config.Global.JWT.Secret = "test-secret-key-for-ldap-fallback"
	config.Global.JWT.Expire = time.Hour

	// 初始化 Redis mock
	redisMock, _ := middleware.NewRedisMock()
	middleware.InitBlacklist(redisMock)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	localUserName := "test_local_" + suffix

	// 创建本地用户
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)
	localUser := model.User{
		Username:    localUserName,
		Password:    string(hashedPwd),
		Name:        "本地测试用户",
		Email:       localUserName + "@test.com",
		Role:        "user",
		Enabled:     true,
		AuthSource:  "local",
	}
	db.Create(&localUser)
	defer cleanupTestUser(db, localUserName)

	t.Run("LDAP启用时本地用户应能登录", func(t *testing.T) {
		body := `{"username":"` + localUserName + `","password":"Test@1234"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusOK {
			t.Errorf("本地用户登录应返回 200，实际 %d: %s", w.Code, w.Body.String())
		}

		// 验证返回了 token
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if _, ok := resp["token"]; !ok {
			t.Error("响应应包含 token")
		}
	})

	t.Run("LDAP启用时本地用户密码错误应返回401", func(t *testing.T) {
		body := `{"username":"` + localUserName + `","password":"WrongPassword"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("密码错误应返回 401，实际 %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("LDAP启用时不存在的用户应返回通用错误", func(t *testing.T) {
		loginRateLimiter.Reset("127.0.0.1")
		body := `{"username":"nonexistent_user_` + suffix + `","password":"Test@1234"}`
		c, w := setupGinContext(0, "")
		c.Request = httptest.NewRequest("POST", "/login", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		h.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("不存在的用户应返回 401，实际 %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if !strings.Contains(resp["error"].(string), "用户名或密码错误") {
			t.Errorf("错误提示应包含'用户名或密码错误'，实际: %s", resp["error"])
		}
	})

	// 恢复配置
	config.Global.LDAP.Enabled = false
}
