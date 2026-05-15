package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

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

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !user.Enabled {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账户已被禁用，请联系管理员"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: token, User: user})
}

func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) InitAdmin() {
	var count int64
	h.db.Model(&model.User{}).Count(&count)
	if count == 0 {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		h.db.Create(&model.User{
			Username: "admin",
			Password: string(hashedPwd),
			Name:     "管理员",
			Email:    "admin@example.com",
			Role:     "admin",
		})
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

func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.db.Order("id ASC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, users)
}

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

	c.JSON(http.StatusCreated, user)
}

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

	// 不能修改自己的角色
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(uint) == uint(id) && req.Role != user.Role {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的角色"})
		return
	}

	// 只有 admin 用户可以更改其他管理员的角色
	currentUsername, _ := c.Get("username")
	if user.Role == "admin" && currentUsername.(string) != "admin" && currentUserID.(uint) != uint(id) && req.Role != user.Role {
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
		if currentUserID.(uint) == uint(id) && !*req.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用自己的账户"})
			return
		}
		updates["enabled"] = *req.Enabled
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 不能删除自己
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(uint) == uint(id) {
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

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

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

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := h.db.Model(&user).Update("password", string(hashedPwd)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 只能改自己的密码
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(uint) != uint(id) {
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

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

func (h *AuthHandler) ToggleUserEnabled(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 不能禁用自己
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(uint) == uint(id) {
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

	c.JSON(http.StatusOK, gin.H{"enabled": newStatus})
}
