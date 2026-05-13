package model

import (
	"time"

	"gorm.io/gorm"
)

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
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
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
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

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
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

func (Server) TableName() string {
	return "servers"
}
