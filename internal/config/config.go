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
	LDAP     LDAPConfig     `yaml:"ldap"`
	AuditLog AuditLogConfig `yaml:"audit_log"`
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

// AuditLogConfig 是审计日志 JSON 输出配置。
type AuditLogConfig struct {
	Enabled  bool   `yaml:"enabled"`   // 是否输出 JSON 审计日志，默认 false
	Output   string `yaml:"output"`    // 输出目标: "stdout"（默认）或 "file"
	FilePath string `yaml:"file_path"` // output=file 时的日志文件路径
	MaxOutput int   `yaml:"max_output"` // Output 字段最大字符数，默认 4096
}

// LDAPConfig 是 LDAP 认证配置。
type LDAPConfig struct {
	Enabled      bool           `yaml:"enabled"`       // 是否启用 LDAP 认证
	Host         string         `yaml:"host"`          // LDAP 服务器地址
	Port         int            `yaml:"port"`          // LDAP 服务器端口，默认 389
	BaseDN       string         `yaml:"base_dn"`       // 搜索基础 DN
	BindDN       string         `yaml:"bind_dn"`       // 绑定用户 DN
	BindPassword string         `yaml:"bind_password"` // 绑定用户密码
	Attributes   LDAPAttributes `yaml:"attributes"`    // 用户属性映射
	UserFilter   string         `yaml:"user_filter"`   // 用户搜索过滤器（可选）
	StartTLS     bool           `yaml:"start_tls"`     // 是否启用 StartTLS
}

// LDAPAttributes 是 LDAP 用户属性映射配置。
type LDAPAttributes struct {
	Username string `yaml:"username"` // 用户名属性，默认 sAMAccountName
	Name     string `yaml:"name"`     // 姓名属性，默认 displayName
	Email    string `yaml:"email"`    // 邮箱属性，默认 mail
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
	// 审计日志环境变量覆盖
	if v := os.Getenv("AUDIT_LOG_ENABLED"); v != "" {
		Global.AuditLog.Enabled = v == "true" || v == "1"
	}

	// LDAP 环境变量覆盖
	if v := os.Getenv("LDAP_ENABLED"); v != "" {
		Global.LDAP.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("LDAP_HOST"); v != "" {
		Global.LDAP.Host = v
	}
	if v := os.Getenv("LDAP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			Global.LDAP.Port = port
		}
	}
	if v := os.Getenv("LDAP_BIND_PASSWORD"); v != "" {
		Global.LDAP.BindPassword = v
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
	// LDAP 默认值
	// 审计日志默认值
	if Global.AuditLog.Output == "" {
		Global.AuditLog.Output = "stdout"
	}
	if Global.AuditLog.MaxOutput == 0 {
		Global.AuditLog.MaxOutput = 4096
	}

	if Global.LDAP.Port == 0 {
		Global.LDAP.Port = 389
	}
	if Global.LDAP.Attributes.Username == "" {
		Global.LDAP.Attributes.Username = "sAMAccountName"
	}
	if Global.LDAP.Attributes.Name == "" {
		Global.LDAP.Attributes.Name = "displayName"
	}
	if Global.LDAP.Attributes.Email == "" {
		Global.LDAP.Attributes.Email = "mail"
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
