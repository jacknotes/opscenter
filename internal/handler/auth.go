package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/middleware"
	"opscenter/internal/model"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
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
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "用户名或密码错误", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !user.Enabled {
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "账户已被禁用", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账户已被禁用，请联系管理员"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		createAuditLog(h.db, c, "auth", "login", req.Username, "", "failed", "用户名或密码错误", 0, "")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

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
	if adminPwd == "" {
		adminPwd = "admin123"
		log.Println("警告: 未配置 admin_password，使用默认密码 admin123，请尽快修改")
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
	} else if adminPwd != "admin123" {
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
	c.JSON(http.StatusOK, users)
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

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

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

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
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

	action := "enable_user"
	if !newStatus {
		action = "disable_user"
	}
	createAuditLog(h.db, c, "auth", action,
		fmt.Sprintf("%s用户: %s (ID: %d)", map[bool]string{true: "启用", false: "禁用"}[newStatus], user.Username, id),
		"", "success", "", 0, "")
	c.JSON(http.StatusOK, gin.H{"enabled": newStatus})
}
