// Package model 定义 GORM 数据模型，包括用户、服务器和操作日志。
// Server 模型通过 GORM 钩子自动对敏感字段进行 AES-256-GCM 加解密。
package model

import "time"

// OperationLog 记录所有操作的审计日志，包括操作模块、动作、目标、状态和输出。
type OperationLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:50" json:"username"`
	Module     string    `gorm:"size:20;not null;index" json:"module"`
	Action     string    `gorm:"size:30;not null" json:"action"`
	Target     string    `gorm:"type:text" json:"target"`
	Detail     string    `gorm:"type:text" json:"detail"`
	Status     string    `gorm:"size:20;not null" json:"status"`
	Output     string    `gorm:"type:text" json:"output"`
	PreviewID  string    `gorm:"size:64" json:"preview_id"`
	ServerID   uint      `gorm:"index" json:"server_id"`
	ServerName string    `gorm:"size:100" json:"server_name"`
	IP           string    `gorm:"size:255" json:"ip"`
	ProjectNames string    `gorm:"type:text" json:"project_names"` // 逗号分隔的服务名列表（K8s/Preprod）
	ProjectCount int       `gorm:"default:0" json:"project_count"` // 涉及的服务数量
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定 GORM 使用的表名为 operation_logs。
func (OperationLog) TableName() string {
	return "operation_logs"
}
