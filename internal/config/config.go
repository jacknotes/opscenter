// Package config 负责加载和管理应用配置。
// 配置从 YAML 文件读取，支持通过环境变量覆盖关键字段。
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是应用的顶层配置结构，包含服务器、数据库、JWT 和加密相关配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Crypto   CryptoConfig   `yaml:"crypto"`
}

// ServerConfig 是 HTTP 服务器配置。
type ServerConfig struct {
	Port           int      `yaml:"port"`
	Host           string   `yaml:"host"`
	AdminPassword  string   `yaml:"admin_password"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	KnownHostsPath string   `yaml:"known_hosts_path"`
}

// DatabaseConfig 是 MySQL 数据库连接配置。
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	Charset  string `yaml:"charset"`
}

// JWTConfig 是 JWT 认证配置，包含签名密钥和过期时间。
type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Expire time.Duration `yaml:"expire"`
}

// CryptoConfig 是 AES-256-GCM 加密配置，密钥长度必须为 16、24 或 32 字节。
type CryptoConfig struct {
	Key string `yaml:"key"`
}

// Global 是全局配置单例，程序启动时由 Load 初始化。
var Global Config

// Load 从指定的 YAML 文件加载配置到 Global 单例。
// 支持通过环境变量 DB_HOST、DB_PORT、DB_PASSWORD、JWT_SECRET、CRYPTO_KEY、ADMIN_PASSWORD 覆盖对应配置项。
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &Global); err != nil {
		return err
	}

	// 从环境变量覆盖配置
	if v := os.Getenv("DB_HOST"); v != "" {
		Global.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			Global.Database.Port = port
		}
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		Global.Database.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		Global.JWT.Secret = v
	}
	if v := os.Getenv("CRYPTO_KEY"); v != "" {
		Global.Crypto.Key = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		Global.Server.AdminPassword = v
	}

	if Global.JWT.Expire == 0 {
		Global.JWT.Expire = 24 * time.Hour
	}
	if Global.Crypto.Key != "" {
		keyLen := len(Global.Crypto.Key)
		if keyLen != 16 && keyLen != 24 && keyLen != 32 {
			return fmt.Errorf("crypto.key 长度必须为 16、24 或 32 字节，当前为 %d 字节", keyLen)
		}
	}
	if Global.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空")
	}
	if len(Global.JWT.Secret) < 16 {
		return fmt.Errorf("jwt.secret 长度至少 16 字节，当前为 %d 字节", len(Global.JWT.Secret))
	}
	return nil
}

// DSN 生成 MySQL 连接字符串（Data Source Name），密码部分会进行 URL 编码。
func (d DatabaseConfig) DSN() string {
	return d.Username + ":" + url.QueryEscape(d.Password) + "@tcp(" + d.Host + ":" + strconv.Itoa(d.Port) + ")/" + d.DBName + "?charset=" + d.Charset + "&parseTime=True&loc=Local"
}
