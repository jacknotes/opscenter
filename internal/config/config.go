package config

import (
	"fmt"
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
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
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
	return d.Username + ":" + d.Password + "@tcp(" + d.Host + ":" + itoa(d.Port) + ")/" + d.DBName + "?charset=" + d.Charset + "&parseTime=True&loc=Local"
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
