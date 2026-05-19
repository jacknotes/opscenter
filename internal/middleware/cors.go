package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opscenter/internal/config"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origins := config.Global.Server.AllowedOrigins
		origin := c.GetHeader("Origin")

		if len(origins) == 0 {
			// 无白名单配置，允许所有来源（开发环境兼容）
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			allowed := false
			for _, o := range origins {
				if origin == o {
					allowed = true
					break
				}
			}
			if allowed {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
