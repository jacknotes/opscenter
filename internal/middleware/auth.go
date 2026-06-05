// Package middleware 提供 Gin 中间件，包括 JWT 认证、CORS、用户状态检查和管理员权限校验。
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"opscenter/internal/config"
	"opscenter/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const blacklistKeyPrefix = "opscenter:blacklist:jti:"

// Claims 是 JWT 的自定义声明，包含用户 ID、用户名和角色。
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// rdb 是 token 黑名单使用的 Redis 客户端。
var rdb *redis.Client

// InitBlacklist 初始化 token 黑名单的 Redis 客户端。
func InitBlacklist(client *redis.Client) {
	rdb = client
}

// BlacklistToken 将 token 的 jti 加入黑名单。
func BlacklistToken(jti string) {
	if rdb == nil {
		return
	}
	rdb.Set(context.Background(), blacklistKeyPrefix+jti, "1", config.Global.JWT.Expire)
}

// IsBlacklisted 检查 jti 是否在黑名单中。
func IsBlacklisted(jti string) bool {
	if rdb == nil {
		return false
	}
	val, err := rdb.Exists(context.Background(), blacklistKeyPrefix+jti).Result()
	if err != nil {
		return false
	}
	return val > 0
}

const activeUserKeyPrefix = "opscenter:active_user:"

// TrackActiveUser 标记用户为在线（登录时调用），key 在 JWT 过期后自动清除。
func TrackActiveUser(username string) {
	if rdb == nil {
		return
	}
	rdb.Set(context.Background(), activeUserKeyPrefix+username, "1", config.Global.JWT.Expire)
}

// UntrackActiveUser 标记用户为离线（登出时调用）。
func UntrackActiveUser(username string) {
	if rdb == nil {
		return
	}
	rdb.Del(context.Background(), activeUserKeyPrefix+username)
}

// GetActiveUserCount 获取当前在线用户数。
func GetActiveUserCount() int64 {
	if rdb == nil {
		return 0
	}
	var count int64
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(context.Background(), cursor, activeUserKeyPrefix+"*", 100).Result()
		if err != nil {
			break
		}
		count += int64(len(keys))
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count
}

// Auth 返回 JWT 认证中间件。支持从 Authorization Header 或 URL query 参数 token 中提取令牌。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
			c.Abort()
			return
		}

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Global.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
			c.Abort()
			return
		}

		// 检查 token 是否已被撤销
		if claims.ID != "" && IsBlacklisted(claims.ID) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌已被撤销"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// UserEnabledCheck 返回用户启用状态检查中间件。已禁用的用户将被拒绝访问。
func UserEnabledCheck(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			c.Abort()
			return
		}

		var user model.User
		if err := db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			c.Abort()
			return
		}

		if !user.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "账户已被禁用"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminRequired 返回管理员权限检查中间件。仅允许 role 为 admin 的用户通过。
func AdminRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}

		var user model.User
		if err := db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}

		if user.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}

		// 更新 context 中的 role 为最新值
		c.Set("role", user.Role)
		c.Next()
	}
}

// GenerateToken 生成 JWT 令牌，包含用户 ID、用户名、角色信息和唯一标识 jti，过期时间由配置决定。
func GenerateToken(userID uint, username, role string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.Global.JWT.Expire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.Global.JWT.Secret))
}
