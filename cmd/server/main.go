package main

import (
	"flag"
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"opscenter/internal/config"
	"opscenter/internal/model"
	"opscenter/internal/router"
)

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
	r := router.Setup(db)

	// Start server
	addr := fmt.Sprintf("%s:%d", config.Global.Server.Host, config.Global.Server.Port)
	log.Printf("服务启动在 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
