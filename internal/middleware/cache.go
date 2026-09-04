package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// StaticCache 为前端静态资源设置缓存策略：
//   - /assets/ 下的构建产物（文件名带内容 hash）：强缓存一年（immutable），
//     浏览器后续访问不再发起协商请求，二次打开页面近乎零请求
//   - index.html 及其余路径：no-cache 协商缓存，保证发版后新资源立即可见
//     （index.html 引用的 assets 文件名变化后，旧 hash 文件自然失效）
func StaticCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	}
}
