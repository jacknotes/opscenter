package middleware

import (
	"net/http"

	"opscenter/internal/config"

	"github.com/gin-gonic/gin"
)

// CORS 返回跨域资源共享中间件。
// 若配置了 allowed_origins 白名单则按名单精确校验并启用 credentials；否则不设置 Allow-Origin（安全默认）。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origins := config.Global.Server.AllowedOrigins
		origin := c.GetHeader("Origin")

		if len(origins) == 0 {
			// 无白名单配置，不设置 Allow-Origin，仅允许同源请求
			// 如需跨域，请在 config.yaml 中配置 allowed_origins
		} else {
			for _, o := range origins {
				if origin == o {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Credentials", "true")
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Expose-Headers", "X-Warning")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
