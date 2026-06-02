package model

import "time"

// LvsRSTag 是 LVS Real Server 的环境标签模型，用于标记 RS 所属环境和禁用状态。
// 以 (RSIP, VSIP) 为联合唯一键，同一 RS 在不同 VS 下可独立设置标签和禁用状态。
type LvsRSTag struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	RSIP           string    `gorm:"size:50;not null;uniqueIndex:idx_rs_vs" json:"rs_ip"`
	VSIP           string    `gorm:"size:50;not null;default:'';uniqueIndex:idx_rs_vs" json:"vs_ip"`
	Tag            string    `gorm:"size:100" json:"tag"`
	Disabled       bool      `gorm:"default:false" json:"disabled"`
	DisabledReason string    `gorm:"size:500" json:"disabled_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (LvsRSTag) TableName() string {
	return "lvs_rs_tags"
}
