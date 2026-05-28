package model

import "time"

// LvsPreprodBinding 是 LVS 与预生产环境的绑定关系模型，
// 记录 VS 标签 + RS 环境标签与预生产服务器的对应关系。
type LvsPreprodBinding struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	VSTag           string    `gorm:"size:100;not null;uniqueIndex:idx_vs_rs" json:"vs_tag"`
	RSEnvTag        string    `gorm:"size:100;not null;uniqueIndex:idx_vs_rs" json:"rs_env_tag"`
	PreprodServerID uint      `gorm:"not null" json:"preprod_server_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (LvsPreprodBinding) TableName() string {
	return "lvs_preprod_bindings"
}
