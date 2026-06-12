package handler

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/middleware"
	"opscenter/internal/model"
	"opscenter/internal/service"
)

// validatePassword 校验密码强度：至少8位，包含大写、小写、数字、特殊符号
func validatePassword(pwd string) string {
	if len(pwd) < 8 {
		return "密码长度不能少于8位"
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range pwd {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?~`", ch):
			hasSpecial = true
		}
	}
	var missing []string
	if !hasUpper {
		missing = append(missing, "大写字母")
	}
	if !hasLower {
		missing = append(missing, "小写字母")
	}
	if !hasDigit {
		missing = append(missing, "数字")
	}
	if !hasSpecial {
		missing = append(missing, "特殊符号(!@#$%^&*等)")
	}
	if len(missing) > 0 {
		return "密码必须包含" + strings.Join(missing, "、")
	}
	return ""
}

// emailRegex 邮箱格式正则
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// validateEmail 校验邮箱格式
func validateEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// loginAttempt 用于记录登录尝试信息
type loginAttempt struct {
	mu        sync.Mutex
	Count     int
	FirstTime time.Time
}

// LoginRateLimiter 基于 IP 的登录速率限制器
type LoginRateLimiter struct {
	attempts sync.Map // key: IP string, value: *loginAttempt
}

var loginRateLimiter = &LoginRateLimiter{}

// Allow 检查指定 IP 是否允许登录，同时清理过期条目
func (rl *LoginRateLimiter) Allow(ip string) bool {
	now := time.Now()
	lockDuration := config.Global.Auth.LoginLockDuration

	// 清理过期条目
	rl.attempts.Range(func(key, value interface{}) bool {
		if entry, ok := value.(*loginAttempt); ok {
			entry.mu.Lock()
			expired := now.Sub(entry.FirstTime) > lockDuration
			entry.mu.Unlock()
			if expired {
				rl.attempts.Delete(key)
			}
		}
		return true
	})

	val, loaded := rl.attempts.Load(ip)
	if !loaded {
		return true
	}

	entry := val.(*loginAttempt)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if now.Sub(entry.FirstTime) > lockDuration {
		// 窗口已过期，重置（defer 会解锁，之后由 Record 创建新条目）
		rl.attempts.Delete(ip)
		return true
	}

	return entry.Count < config.Global.Auth.MaxLoginAttempts
}

// Record 记录一次登录尝试
func (rl *LoginRateLimiter) Record(ip string) {
	now := time.Now()
	lockDuration := config.Global.Auth.LoginLockDuration
	val, loaded := rl.attempts.Load(ip)
	if !loaded {
		rl.attempts.Store(ip, &loginAttempt{Count: 1, FirstTime: now})
		return
	}

	entry := val.(*loginAttempt)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	if now.Sub(entry.FirstTime) > lockDuration {
		// 窗口已过期，重置
		rl.attempts.Store(ip, &loginAttempt{Count: 1, FirstTime: now})
		return
	}

	entry.Count++
}

// Reset 重置指定 IP 的登录计数
func (rl *LoginRateLimiter) Reset(ip string) {
	rl.attempts.Delete(ip)
}

type AuthHandler struct {
	db         *gorm.DB
	ldapSvc    *service.LDAPService
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		db:      db,
		ldapSvc: service.NewLDAPService(&config.Global.LDAP),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// Login godoc
//
//	@Summary		用户登录
//	@Description	用户名密码登录，返回 JWT token
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest	true	"登录参数"
//	@Success		200		{object}	LoginResponse
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Router			/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	clientIP := c.ClientIP()

	// 检查登录速率限制
	if !loginRateLimiter.Allow(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User

	// 优先尝试 LDAP 认证（非 admin 用户）
	if config.Global.LDAP.Enabled && req.Username != "admin" {
		// 先检查用户是否已导入到系统
		if err := h.db.Where("username = ? AND auth_source = ?", req.Username, "ldap").First(&user).Error; err == nil {
			// 用户已导入，进行 LDAP 认证
			ldapInfo, ldapErr := h.ldapSvc.Authenticate(req.Username, req.Password)
			if ldapErr != nil {
				// LDAP 认证失败
				loginRateLimiter.Record(clientIP)
				createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "用户名或密码错误", 0, "")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
				return
			}

			// 检查用户是否被禁用
			if !user.Enabled {
				loginRateLimiter.Record(clientIP)
				createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "账户已被禁用", 0, "")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "账户已被禁用，请联系管理员"})
				return
			}

			// 检查用户是否被锁定
			if user.Locked {
				loginRateLimiter.Record(clientIP)
				createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "账号已锁定", 0, "")
				c.JSON(http.StatusForbidden, gin.H{"error": "账号已锁定，请联系管理员解锁"})
				return
			}

			// 更新 LDAP 信息（如果变化）
			if user.LDAPDN != ldapInfo.DN {
				h.db.Model(&user).Update("ldap_dn", ldapInfo.DN)
			}

			token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
				return
			}

			// 重置失败计数
			if user.FailedAttempts > 0 {
				h.db.Model(&user).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})
			}

			loginRateLimiter.Reset(clientIP)
			middleware.TrackActiveUser(req.Username, middleware.ActiveUserInfo{
				Role:        user.Role,
				LoginTime:   time.Now().Format(time.RFC3339),
				LoginMethod: "ldap",
				LastActive:  time.Now().Format(time.RFC3339),
				JTI:         middleware.ExtractJTI(token),
			})
			createAuditLog(h.db, c, "auth", "login", req.Username, "", "success", "LDAP 登录成功", 0, "")
			c.JSON(http.StatusOK, LoginResponse{Token: token, User: user})
			return
		}

		// 用户未导入，提示联系管理员
		loginRateLimiter.Record(clientIP)
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "LDAP 用户未授权", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未授权，请联系管理员导入 LDAP 账户"})
		return
	}

	// 本地密码认证
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		loginRateLimiter.Record(clientIP)
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "用户名或密码错误", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !user.Enabled {
		loginRateLimiter.Record(clientIP)
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "账户已被禁用", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账户已被禁用，请联系管理员"})
		return
	}

	// LDAP 用户不能使用本地密码登录
	if user.AuthSource == "ldap" {
		loginRateLimiter.Record(clientIP)
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "LDAP 用户请使用域账号登录", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "LDAP 用户请使用域账号登录"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		loginRateLimiter.Record(clientIP)
		// 用户级失败计数（admin 不参与锁定）
		if user.Role != "admin" {
			h.db.Model(&user).Update("failed_attempts", gorm.Expr("failed_attempts + 1"))
			h.db.Model(&user).Where("failed_attempts >= ?", config.Global.Auth.MaxUserAttempts).Update("locked", true)
			// 重新加载用户以获取最新的 failed_attempts
			h.db.First(&user, user.ID)
		}
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "用户名或密码错误", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 检查用户是否被锁定
	if user.Locked {
		loginRateLimiter.Record(clientIP)
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "账号已锁定", 0, "")
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已锁定，请联系管理员解锁"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// 重置失败计数
	if user.FailedAttempts > 0 {
		h.db.Model(&user).Updates(map[string]interface{}{"failed_attempts": 0, "locked": false})
	}

	loginRateLimiter.Reset(clientIP)
	middleware.TrackActiveUser(req.Username, middleware.ActiveUserInfo{
		Role:        user.Role,
		LoginTime:   time.Now().Format(time.RFC3339),
		LoginMethod: "local",
		LastActive:  time.Now().Format(time.RFC3339),
		JTI:         middleware.ExtractJTI(token),
	})
	createAuditLog(h.db, c, "auth", "login", req.Username, "", "success", "登录成功", 0, "")
	c.JSON(http.StatusOK, LoginResponse{Token: token, User: user})
}

// Logout godoc
//
//	@Summary		用户登出
//	@Description	登出当前用户，记录审计日志
//	@Tags			认证
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 将当前 token 加入黑名单
	if jti, exists := c.Get("jti"); exists {
		if jtiStr, ok := jti.(string); ok {
			middleware.BlacklistToken(jtiStr)
		}
	}
	username, _ := c.Get("username")
	middleware.UntrackActiveUser(fmt.Sprintf("%v", username))
	createAuditLog(h.db, c, "auth", "logout", fmt.Sprintf("%v", username), "", "success", "登出成功", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// GetUserInfo godoc
//
//	@Summary		获取当前用户信息
//	@Description	获取当前登录用户的详细信息
//	@Tags			用户
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	model.User
//	@Failure		401	{object}	object
//	@Failure		404	{object}	object
//	@Router			/user/info [get]
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) InitAdmin() {
	adminPwd := config.Global.Server.AdminPassword
	isDefault := false
	if adminPwd == "" {
		adminPwd = "Admin@123"
		isDefault = true
		log.Println("警告: 未配置 admin_password，使用默认密码 Admin@123，请尽快修改")
	}

	// 校验密码强度（默认密码跳过校验）
	if !isDefault {
		if errMsg := validatePassword(adminPwd); errMsg != "" {
			log.Fatalf("admin_password 密码强度不足: %s，请修改 config.yaml 后重启", errMsg)
		}
	}

	var admin model.User
	if err := h.db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		// admin 不存在，创建
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(adminPwd), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("初始化管理员密码失败: %v", err)
			return
		}
		h.db.Create(&model.User{
			Username: "admin",
			Password: string(hashedPwd),
			Name:     "管理员",
			Email:    "admin@example.com",
			Role:     "admin",
		})
		log.Println("管理员账户已初始化")
	} else if !isDefault {
		// admin 已存在且配置了非默认密码，检查是否需要同步
		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(adminPwd)); err != nil {
			// 密码不一致，同步更新
			hashedPwd, err := bcrypt.GenerateFromPassword([]byte(adminPwd), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("同步管理员密码失败: %v", err)
				return
			}
			h.db.Model(&admin).Update("password", string(hashedPwd))
			log.Println("警告: 管理员密码已从配置文件同步，请确保这是预期操作")
		}
	}
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

type UpdateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Enabled  *bool  `json:"enabled"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ListUsers godoc
//
//	@Summary		获取用户列表
//	@Description	获取所有用户列表（管理员）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		model.User
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Router			/users [get]
func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.db.Order("id ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 构造带在线状态的响应
	result := make([]gin.H, len(users))
	for i, user := range users {
		online := middleware.GetActiveUserInfo(user.Username) != nil
		result[i] = gin.H{
			"id":              user.ID,
			"username":        user.Username,
			"name":            user.Name,
			"email":           user.Email,
			"role":            user.Role,
			"enabled":         user.Enabled,
			"auth_source":     user.AuthSource,
			"failed_attempts": user.FailedAttempts,
			"locked":          user.Locked,
			"online":          online,
			"created_at":      user.CreatedAt,
			"updated_at":      user.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, result)
}

// CreateUser godoc
//
//	@Summary		创建用户
//	@Description	创建新用户（管理员）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateUserRequest	true	"用户信息"
//	@Success		201		{object}	model.User
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users [post]
func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色只能是 admin 或 user"})
		return
	}

	if !validateEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式无效"})
		return
	}

	if errMsg := validatePassword(req.Password); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 清理同名的软删除用户，避免唯一索引冲突
	h.db.Unscoped().Where("username = ? AND deleted_at IS NOT NULL", req.Username).Delete(&model.User{})

	user := model.User{
		Username: req.Username,
		Password: string(hashedPwd),
		Name:     req.Name,
		Email:    req.Email,
		Role:     req.Role,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败，用户名可能已存在"})
		return
	}

	createAuditLog(h.db, c, "auth", "create_user",
		fmt.Sprintf("创建用户: %s (角色: %s)", req.Username, req.Role),
		"", "success", "", 0, "")
	c.JSON(http.StatusCreated, user)
}

// getCurrentUserID 安全获取当前用户 ID
func getCurrentUserID(c *gin.Context) (uint, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := uid.(uint)
	return id, ok
}

// UpdateUser godoc
//
//	@Summary		更新用户
//	@Description	更新用户信息（管理员）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"用户 ID"
//	@Param			body	body		UpdateUserRequest	true	"用户信息"
//	@Success		200		{object}	model.User
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Failure		404		{object}	object
//	@Router			/users/{id} [put]
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色只能是 admin 或 user"})
		return
	}

	if !validateEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式无效"})
		return
	}

	// admin 用户固定为管理员，不可更改角色
	if user.Username == "admin" && req.Role != "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin 用户固定为管理员角色，不可更改"})
		return
	}

	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// 不能修改自己的角色
	if currentUserID == uint(id) && req.Role != user.Role {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的角色"})
		return
	}

	// 只有 admin 用户可以更改其他管理员的角色
	currentUsername, _ := c.Get("username")
	currentUsernameStr, _ := currentUsername.(string)
	if user.Role == "admin" && currentUsernameStr != "admin" && currentUserID != uint(id) && req.Role != user.Role {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有 admin 用户可以更改其他管理员的角色"})
		return
	}

	updates := map[string]interface{}{
		"username": req.Username,
		"name":     req.Name,
		"email":    req.Email,
		"role":     req.Role,
	}
	if req.Enabled != nil {
		// 不能禁用 admin 用户
		if user.Username == "admin" && !*req.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用 admin 用户"})
			return
		}
		// 不能禁用自己
		if currentUserID == uint(id) && !*req.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用自己的账户"})
			return
		}
		updates["enabled"] = *req.Enabled
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	createAuditLog(h.db, c, "auth", "update_user",
		fmt.Sprintf("更新用户: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
//
//	@Summary		删除用户
//	@Description	删除用户（管理员，不能删除自己和admin）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"用户 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Failure		404	{object}	object
//	@Router			/users/{id} [delete]
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 不能删除自己
	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}

	// 不能删除 admin 用户
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if user.Username == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除 admin 用户"})
		return
	}

	if err := h.db.Delete(&model.User{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	createAuditLog(h.db, c, "auth", "delete_user",
		fmt.Sprintf("删除用户: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

type BatchDeleteUsersRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchDeleteUsers godoc
//
//	@Summary		批量删除用户
//	@Description	批量删除用户（管理员，不能删除自己和admin）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchDeleteUsersRequest	true	"用户 ID 列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/batch-delete [post]
func (h *AuthHandler) BatchDeleteUsers(c *gin.Context) {
	var req BatchDeleteUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要删除的用户"})
		return
	}

	currentUserID, _ := getCurrentUserID(c)

	deleted := 0
	failed := 0
	var deletedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		// 不能删除自己
		if currentUserID == id {
			failed++
			failedNames = append(failedNames, "自己")
			continue
		}

		var user model.User
		if err := h.db.First(&user, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		// 不能删除 admin
		if user.Username == "admin" {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if err := h.db.Delete(&model.User{}, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		deleted++
		deletedNames = append(deletedNames, user.Username)
	}

	createAuditLog(h.db, c, "auth", "batch_delete_users",
		fmt.Sprintf("批量删除用户: 成功 %d, 失败 %d", deleted, failed),
		"", "success", fmt.Sprintf("删除的用户: %v", deletedNames), 0, "")

	message := fmt.Sprintf("批量删除完成: 成功 %d, 失败 %d", deleted, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"deleted": deleted,
		"failed":  failed,
	})
}

type BatchToggleUsersRequest struct {
	IDs     []uint `json:"ids" binding:"required"`
	Enabled bool   `json:"enabled"`
}

// BatchToggleUsers godoc
//
//	@Summary		批量启用/禁用用户
//	@Description	批量切换用户启用状态（管理员，不能操作自己和admin）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchToggleUsersRequest	true	"用户 ID 列表和目标状态"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/batch-toggle [post]
func (h *AuthHandler) BatchToggleUsers(c *gin.Context) {
	var req BatchToggleUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要操作的用户"})
		return
	}

	currentUserID, _ := getCurrentUserID(c)

	updated := 0
	failed := 0
	var updatedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		// 不能操作自己
		if currentUserID == id {
			failed++
			failedNames = append(failedNames, "自己")
			continue
		}

		var user model.User
		if err := h.db.First(&user, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		// 不能操作 admin
		if user.Username == "admin" {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if err := h.db.Model(&user).Update("enabled", req.Enabled).Error; err != nil {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		// 禁用用户时自动强制下线
		if !req.Enabled {
			middleware.ForceKickUser(user.Username)
		}

		updated++
		updatedNames = append(updatedNames, user.Username)
	}

	action := "启用"
	if !req.Enabled {
		action = "禁用"
	}

	createAuditLog(h.db, c, "auth", "batch_toggle_users",
		fmt.Sprintf("批量%s用户: 成功 %d, 失败 %d", action, updated, failed),
		"", "success", fmt.Sprintf("%s的用户: %v", action, updatedNames), 0, "")

	message := fmt.Sprintf("批量%s完成: 成功 %d, 失败 %d", action, updated, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"updated": updated,
		"failed":  failed,
	})
}

// ResetPassword godoc
//
//	@Summary		重置用户密码
//	@Description	管理员重置指定用户的密码
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int						true	"用户 ID"
//	@Param			body	body		ResetPasswordRequest	true	"新密码"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Failure		404		{object}	object
//	@Router			/users/{id}/reset-password [put]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.Username == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许重置 admin 用户密码"})
		return
	}

	if user.AuthSource == "ldap" {
		c.JSON(http.StatusForbidden, gin.H{"error": "LDAP 用户密码由域控制器管理，不能在此重置"})
		return
	}

	if errMsg := validatePassword(req.Password); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := h.db.Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	createAuditLog(h.db, c, "auth", "reset_password",
		fmt.Sprintf("重置密码: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

// ChangePassword godoc
//
//	@Summary		修改密码
//	@Description	用户修改自己的密码
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int						true	"用户 ID"
//	@Param			body	body		ChangePasswordRequest	true	"密码信息"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/{id}/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 只能改自己的密码
	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if currentUserID != uint(id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的密码"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.AuthSource == "ldap" {
		c.JSON(http.StatusForbidden, gin.H{"error": "LDAP 用户密码由域控制器管理，不能在此修改"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}

	if errMsg := validatePassword(req.NewPassword); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := h.db.Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		return
	}

	createAuditLog(h.db, c, "auth", "change_password",
		fmt.Sprintf("修改密码: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// ToggleUserEnabled godoc
//
//	@Summary		启用/禁用用户
//	@Description	切换用户启用状态（管理员）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"用户 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Failure		404	{object}	object
//	@Router			/users/{id}/toggle [put]

// ListLDAPUsers godoc
//
//	@Summary		获取 LDAP 用户列表
//	@Description	从 LDAP 获取 OU 下的用户列表（管理员）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		service.LDAPUserInfo
//	@Failure		400	{object}	object
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Failure		500	{object}	object
//	@Router			/users/ldap [get]
func (h *AuthHandler) ListLDAPUsers(c *gin.Context) {
	if !config.Global.LDAP.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 未启用"})
		return
	}

	users, err := h.ldapSvc.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取 LDAP 用户失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, users)
}

// ImportLDAPUsers godoc
//
//	@Summary		导入 LDAP 用户
//	@Description	批量导入 LDAP 用户到系统（管理员）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		ImportLDAPUsersRequest	true	"导入参数"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/ldap/import [post]
func (h *AuthHandler) ImportLDAPUsers(c *gin.Context) {
	if !config.Global.LDAP.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LDAP 未启用"})
		return
	}

	var req ImportLDAPUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.Users) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要导入的用户"})
		return
	}

	imported := 0
	skipped := 0
	failed := 0

	for _, u := range req.Users {
		// 检查用户是否已存在（包括未软删除的）
		var existingUser model.User
		if err := h.db.Where("username = ?", u.Username).First(&existingUser).Error; err == nil {
			// 用户已存在，跳过
			skipped++
			continue
		}

		// 清理同名的软删除用户，避免唯一索引冲突
		h.db.Unscoped().Where("username = ? AND deleted_at IS NOT NULL", u.Username).Delete(&model.User{})

		// 创建用户
		user := model.User{
			Username:   u.Username,
			Name:       u.Name,
			Email:      u.Email,
			Role:       "user",
			Enabled:    true,
			AuthSource: "ldap",
			LDAPDN:     u.DN,
		}
		if user.Name == "" {
			user.Name = u.Username
		}

		if err := h.db.Create(&user).Error; err != nil {
			log.Printf("[LDAP] 导入用户 %s 失败: %v", u.Username, err)
			failed++
			continue
		}
		imported++
	}

	createAuditLog(h.db, c, "auth", "import_ldap_users",
		fmt.Sprintf("导入 LDAP 用户: 成功 %d, 跳过 %d, 失败 %d", imported, skipped, failed),
		"", "success", "", 0, "")

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("导入完成: 成功 %d, 跳过 %d, 失败 %d", imported, skipped, failed),
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
	})
}

type ImportLDAPUsersRequest struct {
	Users []ImportLDAPUser `json:"users" binding:"required"`
}

type ImportLDAPUser struct {
	Username string `json:"username" binding:"required"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	DN       string `json:"dn" binding:"required"`
}

// UnlockUser godoc
//
//	@Summary		解锁用户
//	@Description	管理员解锁被锁定的用户（重置 failed_attempts 和 locked）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"用户 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Failure		404	{object}	object
//	@Router			/users/{id}/unlock [put]
func (h *AuthHandler) UnlockUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.Username == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin 用户无需解锁"})
		return
	}

	if !user.Locked && user.FailedAttempts == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户未被锁定"})
		return
	}

	if err := h.db.Model(&user).Updates(map[string]interface{}{
		"locked":          false,
		"failed_attempts": 0,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解锁失败"})
		return
	}

	createAuditLog(h.db, c, "auth", "unlock_user",
		fmt.Sprintf("解锁用户: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "解锁成功"})
}

// KickUser godoc
//
//	@Summary		强制下线用户
//	@Description	管理员强制将在线用户踢下线（作废 token + 清除在线状态）
//	@Tags			用户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"用户 ID"
//	@Success		200	{object}	object
//	@Failure		400	{object}	object
//	@Failure		401	{object}	object
//	@Failure		403	{object}	object
//	@Failure		404	{object}	object
//	@Router			/users/{id}/kick [post]
func (h *AuthHandler) KickUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.Username == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能强制下线 admin 用户"})
		return
	}

	// 不能踢自己下线
	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能强制下线自己"})
		return
	}

	kicked := middleware.ForceKickUser(user.Username)
	if !kicked {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户当前不在线"})
		return
	}

	createAuditLog(h.db, c, "auth", "kick_user",
		fmt.Sprintf("强制下线用户: %s (ID: %d)", user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"message": "已强制下线"})
}

type BatchUnlockUsersRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchUnlockUsers godoc
//
//	@Summary		批量解锁用户
//	@Description	批量解锁被锁定的用户（管理员）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchUnlockUsersRequest	true	"用户 ID 列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/batch-unlock [post]
func (h *AuthHandler) BatchUnlockUsers(c *gin.Context) {
	var req BatchUnlockUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要解锁的用户"})
		return
	}

	unlocked := 0
	failed := 0
	var unlockedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		var user model.User
		if err := h.db.First(&user, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		if user.Username == "admin" {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if !user.Locked && user.FailedAttempts == 0 {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if err := h.db.Model(&user).Updates(map[string]interface{}{
			"locked":          false,
			"failed_attempts": 0,
		}).Error; err != nil {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		unlocked++
		unlockedNames = append(unlockedNames, user.Username)
	}

	createAuditLog(h.db, c, "auth", "batch_unlock_users",
		fmt.Sprintf("批量解锁用户: 成功 %d, 失败 %d", unlocked, failed),
		"", "success", fmt.Sprintf("解锁的用户: %v", unlockedNames), 0, "")

	message := fmt.Sprintf("批量解锁完成: 成功 %d, 失败 %d", unlocked, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  message,
		"unlocked": unlocked,
		"failed":   failed,
	})
}

// BatchKickUsers godoc
//
//	@Summary		批量强制下线用户
//	@Description	管理员批量强制将在线用户踢下线（作废 token + 清除在线状态）
//	@Tags			用户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		BatchUnlockUsersRequest	true	"用户 ID 列表"
//	@Success		200		{object}	object
//	@Failure		400		{object}	object
//	@Failure		401		{object}	object
//	@Failure		403		{object}	object
//	@Router			/users/batch-kick [post]
func (h *AuthHandler) BatchKickUsers(c *gin.Context) {
	var req BatchUnlockUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要下线的用户"})
		return
	}

	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	kicked := 0
	failed := 0
	var kickedNames []string
	var failedNames []string

	for _, id := range req.IDs {
		var user model.User
		if err := h.db.First(&user, id).Error; err != nil {
			failed++
			failedNames = append(failedNames, fmt.Sprintf("ID:%d", id))
			continue
		}

		if user.Username == "admin" {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if currentUserID == uint(id) {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		if !middleware.ForceKickUser(user.Username) {
			failed++
			failedNames = append(failedNames, user.Username)
			continue
		}

		kicked++
		kickedNames = append(kickedNames, user.Username)
	}

	createAuditLog(h.db, c, "auth", "batch_kick_users",
		fmt.Sprintf("批量强制下线用户: 成功 %d, 失败 %d", kicked, failed),
		"", "success", fmt.Sprintf("下线的用户: %v", kickedNames), 0, "")

	message := fmt.Sprintf("批量下线完成: 成功 %d, 失败 %d", kicked, failed)
	if len(failedNames) > 0 {
		message += fmt.Sprintf("\n失败: %s", strings.Join(failedNames, ", "))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"updated": kicked,
		"failed":  failed,
	})
}

func (h *AuthHandler) ToggleUserEnabled(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 不能禁用自己
	currentUserID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用自己的账户"})
		return
	}

	// 不能禁用 admin 用户
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if user.Username == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用 admin 用户"})
		return
	}

	newStatus := !user.Enabled
	if err := h.db.Model(&user).Update("enabled", newStatus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	// 禁用用户时自动强制下线
	if !newStatus {
		middleware.ForceKickUser(user.Username)
	}

	action := "enable_user"
	if !newStatus {
		action = "disable_user"
	}
	createAuditLog(h.db, c, "auth", action,
		fmt.Sprintf("%s用户: %s (ID: %d)", map[bool]string{true: "启用", false: "禁用"}[newStatus], user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"enabled": newStatus})
}
