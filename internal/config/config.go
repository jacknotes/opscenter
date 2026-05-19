package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Crypto   CryptoConfig   `yaml:"crypto"`
}

type ServerConfig struct {
	Port            int      `yaml:"port"`
	Host            string   `yaml:"host"`
	AdminPassword   string   `yaml:"admin_password"`
	AllowedOrigins  []string `yaml:"allowed_origins"`
	KnownHostsPath  string   `yaml:"known_hosts_path"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	Charset  string `yaml:"charset"`
}

type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Expire time.Duration `yaml:"expire"`
}

type CryptoConfig struct {
	Key string `yaml:"key"`
}

var Global Config

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
	return nil
}

func (d DatabaseConfig) DSN() string {
	return d.Username + ":" + url.QueryEscape(d.Password) + "@tcp(" + d.Host + ":" + strconv.Itoa(d.Port) + ")/" + d.DBName + "?charset=" + d.Charset + "&parseTime=True&loc=Local"
}
