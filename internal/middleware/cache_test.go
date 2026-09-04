package middleware

import (
	stdgzip "compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// 验证缓存策略：assets 强缓存，其余路径 no-cache
func TestStaticCacheHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(StaticCache())
	r.GET("/assets/app-abc123.js", func(c *gin.Context) { c.String(http.StatusOK, "js") })
	r.GET("/index.html", func(c *gin.Context) { c.String(http.StatusOK, "html") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app-abc123.js", nil))
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("assets 缓存头不符合预期: %q", got)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index.html 缓存头不符合预期: %q", got)
	}
}

// 验证 gzip 中间件：静态资源压缩，/api 与 /swagger 排除（与 router.go 配置保持一致）
func TestGzipCompressionWithExclusions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gzip.Gzip(gzip.DefaultCompression,
		gzip.WithExcludedPathsRegexs([]string{`^/api/`, `^/swagger/`})))
	body := strings.Repeat("hello opscenter ", 200)
	r.GET("/assets/app.js", func(c *gin.Context) { c.String(http.StatusOK, body) })
	r.GET("/api/health", func(c *gin.Context) { c.String(http.StatusOK, body) })

	// 静态资源：应返回 gzip 编码
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(w, req)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("静态资源未被 gzip 压缩")
	}
	if w.Body.Len() >= len(body) {
		t.Errorf("压缩后体积未减小: %d >= %d", w.Body.Len(), len(body))
	}
	zr, err := stdgzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil || string(plain) != body {
		t.Errorf("gzip 内容解压后与原文不一致: err=%v len=%d", err, len(plain))
	}

	// /api 路径：排除压缩
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(w, req)
	if enc := w.Header().Get("Content-Encoding"); enc == "gzip" {
		t.Errorf("/api 路径不应被 gzip 压缩")
	}
}
