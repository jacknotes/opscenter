package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"opscenter/internal/config"
	"opscenter/internal/handler"
	"opscenter/internal/model"
	"opscenter/internal/router"
	"opscenter/internal/service"
)

//	@title			OpsCenter API
//	@version		1.0
//	@description	运维发布管理系统 API 文档
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.email	support@opscenter.local

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@host		localhost:18080
//	@BasePath	/api

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				"Bearer {token}"

// main 是程序入口，负责加载配置、连接数据库和 Redis、自动迁移模型、启动 HTTP 服务器，
// 并在收到 SIGINT/SIGTERM 信号时优雅停机，清理 SSH 连接和 Redis 客户端。
func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// Load config
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// Init audit log JSON writer
	auditCfg := config.Global.AuditLog
	handler.InitAuditWriter(auditCfg.Enabled, auditCfg.Output, auditCfg.FilePath, auditCfg.MaxOutput)

	// 当审计日志 JSON 输出到 stdout 时，禁用 Gin/GORM 日志避免污染 NDJSON 流。
	// 输出到 file 时不需要禁用，应用日志正常输出到 stdout。
	if auditCfg.Enabled && auditCfg.Output != "file" {
		gin.SetMode(gin.ReleaseMode)
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		log.SetOutput(io.Discard)
	}

	// Connect database
	gormLogLevel := logger.Info
	if auditCfg.Enabled && auditCfg.Output != "file" {
		gormLogLevel = logger.Silent
	}
	db, err := gorm.Open(mysql.Open(config.Global.Database.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&model.User{}, &model.Server{}, &model.OperationLog{}, &model.LvsRSTag{}, &model.LvsVSTag{}, &model.LvsPreprodBinding{}, &model.LvsConnStat{}); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// RS 标签索引迁移：旧版以 rs_ip 为唯一键，新版以 (rs_ip, vs_ip) 为联合唯一键
	// GORM AutoMigrate 已添加 vs_ip 列并创建新索引，此处清理可能残留的旧索引
	var indexes []string
	db.Raw("SHOW INDEX FROM lvs_rs_tags WHERE Key_name != 'PRIMARY' AND Key_name != 'idx_rs_vs'").Pluck("Key_name", &indexes)
	for _, idxName := range indexes {
		db.Exec(fmt.Sprintf("ALTER TABLE lvs_rs_tags DROP INDEX `%s`", idxName))
	}

	// Connect Redis
	var rdb *redis.Client
	rcfg := config.Global.Redis
	if rcfg.Mode == "sentinel" {
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    rcfg.MasterName,
			SentinelAddrs: rcfg.SentinelAddrs,
			Password:      rcfg.Password,
			DB:            rcfg.DB,
			PoolSize:      rcfg.PoolSize,
			MinIdleConns:  rcfg.MinIdleConns,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:         rcfg.Host + ":" + strconv.Itoa(rcfg.Port),
			Password:     rcfg.Password,
			DB:           rcfg.DB,
			PoolSize:     rcfg.PoolSize,
			MinIdleConns: rcfg.MinIdleConns,
		})
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}
	log.Println("Redis 连接成功")

	// Setup router
	app := router.Setup(db, rdb)

	// 清除上次运行遗留的分布式锁和预览数据，防止重启后操作被阻塞
	app.LockManager.ClearAll()
	app.PreviewMgr.ClearAll()

	// 启动 LVS 连接数后台采集器
	collectorCtx, collectorCancel := context.WithCancel(context.Background())
	lvsCollector := service.NewLvsCollector(db, app.SSHManager, config.Global.Timeouts.LvsCollectInterval)
	lvsCollector.Start(collectorCtx)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", config.Global.Server.Host, config.Global.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: app.Engine,
	}

	// Start server in goroutine
	go func() {
		log.Printf("服务启动在 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务失败: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("收到停机信号，正在优雅关闭...")

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP 服务关闭失败: %v", err)
	}

	// Cleanup resources
	collectorCancel()
	lvsCollector.Stop()
	lvsCollector.Wait() // 等待采集 goroutine 退出后再关闭 SSH 连接
	app.SSHManager.Close()
	app.LockManager.Stop()
	app.PreviewMgr.Stop()
	if err := rdb.Close(); err != nil {
		log.Printf("Redis 关闭失败: %v", err)
	}

	log.Println("服务已停止")
}
