package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"opscenter/internal/config"
	"opscenter/internal/model"
	"opscenter/internal/router"
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

// main 是程序入口，负责加载配置、连接数据库、自动迁移模型、启动 HTTP 服务器，
// 并在收到 SIGINT/SIGTERM 信号时优雅停机，清理 SSH 连接和后台协程。
func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// Load config
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// Connect database
	db, err := gorm.Open(mysql.Open(config.Global.Database.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&model.User{}, &model.Server{}, &model.OperationLog{}); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// Setup router
	app := router.Setup(db)

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
	app.SSHManager.Close()
	app.LockManager.Stop()
	app.PreviewMgr.Stop()

	log.Println("服务已停止")
}
