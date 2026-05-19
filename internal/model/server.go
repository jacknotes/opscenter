package model

import (
	"log"
	"time"

	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/pkg/crypto"
)

// Server 是远程服务器配置模型，包含 SSH 连接信息和脚本配置。
// 敏感字段（password、private_key、script_password）通过 GORM 钩子自动加解密。
type Server struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"size:100;not null" json:"name"`
	Host           string         `gorm:"size:50;not null" json:"host"`
	Port           int            `gorm:"default:22" json:"port"`
	Username       string         `gorm:"size:50;not null" json:"username"`
	AuthType       string         `gorm:"size:20;not null" json:"auth_type"`
	Password       string         `gorm:"size:255" json:"password,omitempty"`
	PrivateKey     string         `gorm:"type:text" json:"private_key,omitempty"`
	ServerType     string         `gorm:"size:30;not null;index" json:"server_type"`
	Env            string         `gorm:"size:20;index" json:"env"`
	ScriptPath     string         `gorm:"size:255" json:"script_path"`
	ScriptPassword string         `gorm:"size:255" json:"script_password,omitempty"`
	ConfigPath     string         `gorm:"size:255" json:"config_path"`
	ConfigPattern  string         `gorm:"size:100" json:"config_pattern"`
	BackupPath     string         `gorm:"size:255" json:"backup_path"`
	Description    string         `gorm:"size:500" json:"description"`
	Enabled        bool           `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// 标记敏感字段是否已解密（防止 BeforeSave 重复加密）
	decrypted bool `gorm:"-"`
}

// ServerResponse 用于返回给前端（不包含敏感信息）
type ServerResponse struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	Username      string    `json:"username"`
	AuthType      string    `json:"auth_type"`
	HasPassword   bool      `json:"has_password"`
	HasPrivateKey bool      `json:"has_private_key"`
	ServerType    string    `json:"server_type"`
	Env           string    `json:"env"`
	ScriptPath    string    `json:"script_path"`
	HasScriptPwd  bool      `json:"has_script_password"`
	ConfigPath    string    `json:"config_path"`
	ConfigPattern string    `json:"config_pattern"`
	BackupPath    string    `json:"backup_path"`
	Description   string    `json:"description"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToResponse 将 Server 转换为脱敏的 ServerResponse，敏感字段仅返回是否存在（布尔值）。
func (s *Server) ToResponse() ServerResponse {
	return ServerResponse{
		ID:            s.ID,
		Name:          s.Name,
		Host:          s.Host,
		Port:          s.Port,
		Username:      s.Username,
		AuthType:      s.AuthType,
		HasPassword:   s.Password != "",
		HasPrivateKey: s.PrivateKey != "",
		ServerType:    s.ServerType,
		Env:           s.Env,
		ScriptPath:    s.ScriptPath,
		HasScriptPwd:  s.ScriptPassword != "",
		ConfigPath:    s.ConfigPath,
		ConfigPattern: s.ConfigPattern,
		BackupPath:    s.BackupPath,
		Description:   s.Description,
		Enabled:       s.Enabled,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

// TableName 指定 GORM 使用的表名为 servers。
func (Server) TableName() string {
	return "servers"
}

// BeforeSave 是 GORM 钩子，在保存前对敏感字段进行 AES-256-GCM 加密。
// 若字段已解密则重新加密；若未经过解密流程（如新建记录），则尝试检测是否已加密。
func (s *Server) BeforeSave(tx *gorm.DB) error {
	key := config.Global.Crypto.Key
	if key == "" {
		return nil
	}

	// 如果字段已由 AfterFind 解密，直接加密
	// 如果字段未解密（如直接 GORM Create），尝试检测是否已加密
	encryptField := func(value string) (string, error) {
		if value == "" {
			return value, nil
		}
		if s.decrypted {
			// 已解密，需要重新加密
			return crypto.Encrypt(value, key)
		}
		// 未经过 AfterFind（如 Create），尝试解密判断是否已加密
		if _, err := crypto.Decrypt(value, key); err == nil {
			// 能解密，说明已加密，保持原值
			return value, nil
		}
		// 不能解密，说明是明文，需要加密
		return crypto.Encrypt(value, key)
	}

	var err error
	if s.Password, err = encryptField(s.Password); err != nil {
		return err
	}
	if s.PrivateKey, err = encryptField(s.PrivateKey); err != nil {
		return err
	}
	if s.ScriptPassword, err = encryptField(s.ScriptPassword); err != nil {
		return err
	}
	return nil
}

// AfterFind 是 GORM 钩子，在查询后对敏感字段进行 AES-256-GCM 解密。
// 解密失败时降级为明文处理并打印警告日志。
func (s *Server) AfterFind(tx *gorm.DB) error {
	key := config.Global.Crypto.Key
	if key == "" {
		return nil
	}
	if s.Password != "" {
		decrypted, err := crypto.Decrypt(s.Password, key)
		if err != nil {
			log.Printf("[WARN] failed to decrypt password for server %d, treating as plaintext: %v", s.ID, err)
		} else {
			s.Password = decrypted
		}
	}
	if s.PrivateKey != "" {
		decrypted, err := crypto.Decrypt(s.PrivateKey, key)
		if err != nil {
			log.Printf("[WARN] failed to decrypt private_key for server %d, treating as plaintext: %v", s.ID, err)
		} else {
			s.PrivateKey = decrypted
		}
	}
	if s.ScriptPassword != "" {
		decrypted, err := crypto.Decrypt(s.ScriptPassword, key)
		if err != nil {
			log.Printf("[WARN] failed to decrypt script_password for server %d, treating as plaintext: %v", s.ID, err)
		} else {
			s.ScriptPassword = decrypted
		}
	}
	s.decrypted = true
	return nil
}
