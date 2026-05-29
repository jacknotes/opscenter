// Package config 负责加载和管理应用配置。
// 配置从 YAML 文件读取，支持通过环境变量覆盖关键字段。
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是应用的顶层配置结构，包含服务器、数据库、Redis、JWT 和加密相关配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	Crypto   CryptoConfig   `yaml:"crypto"`
	Timeouts TimeoutConfig  `yaml:"timeouts"`
	Nginx    NginxConfig    `yaml:"nginx"`
	Auth     AuthConfig     `yaml:"auth"`
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

// TimeoutConfig 是运维超时配置，所有字段均有默认值。
type TimeoutConfig struct {
	SSHConnect   time.Duration `yaml:"ssh_connect"`   // SSH 连接超时，默认 10s
	SSHIdle      time.Duration `yaml:"ssh_idle"`      // SSH 连接空闲超时，默认 10m
	SSHLifetime  time.Duration `yaml:"ssh_lifetime"`  // SSH 连接最大生命周期，默认 1h
	SSHCleanup   time.Duration `yaml:"ssh_cleanup"`   // SSH 连接清理间隔，默认 5m
	WSRead       time.Duration `yaml:"ws_read"`       // WebSocket 读超时，默认 60s
	WSPing       time.Duration `yaml:"ws_ping"`       // WebSocket ping 间隔，默认 30s
	Lock         time.Duration `yaml:"lock"`          // 分布式锁超时，默认 10m
	Preview      time.Duration `yaml:"preview"`       // 预览数据过期时间，默认 5m
	DashboardSSH time.Duration `yaml:"dashboard_ssh"` // Dashboard SSH 命令超时，默认 20s
}

// NginxConfig 是 Nginx 相关配置。
type NginxConfig struct {
	MaxBackups int `yaml:"max_backups"` // 最大备份数量，默认 10
}

// AuthConfig 是认证相关配置。
type AuthConfig struct {
	MaxLoginAttempts  int           `yaml:"max_login_attempts"`  // 最大失败尝试次数，默认 10
	LoginLockDuration time.Duration `yaml:"login_lock_duration"` // 登录锁定时长，默认 1m
}

// RedisConfig 是 Redis 连接配置，支持单节点和哨兵两种模式。
type RedisConfig struct {
	Mode          string   `yaml:"mode"`
	Password      string   `yaml:"password"`
	DB            int      `yaml:"db"`
	Host          string   `yaml:"host"`
	Port          int      `yaml:"port"`
	MasterName    string   `yaml:"master_name"`
	SentinelAddrs []string `yaml:"sentinel_addrs"`
	// 连接池配置
	PoolSize     int `yaml:"pool_size"`
	MinIdleConns int `yaml:"min_idle_conns"`
}

// Global 是全局配置单例，程序启动时由 Load 初始化。
var Global Config

// Load 从指定的 YAML 文件加载配置到 Global 单例。
// 支持通过环境变量覆盖：DB_HOST、DB_PORT、DB_PASSWORD、JWT_SECRET、CRYPTO_KEY、ADMIN_PASSWORD、
// REDIS_MODE、REDIS_HOST、REDIS_PORT、REDIS_PASSWORD、REDIS_DB、REDIS_MASTER_NAME、REDIS_SENTINEL_ADDRS。
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
	if v := os.Getenv("REDIS_MODE"); v != "" {
		Global.Redis.Mode = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		Global.Redis.Password = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		Global.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			Global.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			Global.Redis.DB = db
		}
	}
	if v := os.Getenv("REDIS_MASTER_NAME"); v != "" {
		Global.Redis.MasterName = v
	}
	if v := os.Getenv("REDIS_SENTINEL_ADDRS"); v != "" {
		Global.Redis.SentinelAddrs = strings.Split(v, ",")
	}

	// Redis 默认值
	if Global.Redis.Mode == "" {
		Global.Redis.Mode = "standalone"
	}
	if Global.Redis.Port == 0 {
		Global.Redis.Port = 6379
	}
	if Global.Redis.PoolSize == 0 {
		Global.Redis.PoolSize = 10
	}
	if Global.Redis.MinIdleConns == 0 {
		Global.Redis.MinIdleConns = 2
	}

	// 超时默认值
	if Global.Timeouts.SSHConnect == 0 {
		Global.Timeouts.SSHConnect = 10 * time.Second
	}
	if Global.Timeouts.SSHIdle == 0 {
		Global.Timeouts.SSHIdle = 10 * time.Minute
	}
	if Global.Timeouts.SSHLifetime == 0 {
		Global.Timeouts.SSHLifetime = 1 * time.Hour
	}
	if Global.Timeouts.SSHCleanup == 0 {
		Global.Timeouts.SSHCleanup = 5 * time.Minute
	}
	if Global.Timeouts.WSRead == 0 {
		Global.Timeouts.WSRead = 60 * time.Second
	}
	if Global.Timeouts.WSPing == 0 {
		Global.Timeouts.WSPing = 30 * time.Second
	}
	if Global.Timeouts.Lock == 0 {
		Global.Timeouts.Lock = 10 * time.Minute
	}
	if Global.Timeouts.Preview == 0 {
		Global.Timeouts.Preview = 5 * time.Minute
	}
	if Global.Timeouts.DashboardSSH == 0 {
		Global.Timeouts.DashboardSSH = 20 * time.Second
	}
	// Nginx 默认值
	if Global.Nginx.MaxBackups == 0 {
		Global.Nginx.MaxBackups = 10
	}
	// Auth 默认值
	if Global.Auth.MaxLoginAttempts == 0 {
		Global.Auth.MaxLoginAttempts = 10
	}
	if Global.Auth.LoginLockDuration == 0 {
		Global.Auth.LoginLockDuration = 1 * time.Minute
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
