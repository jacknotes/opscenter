package model

import "time"

// LvsRSTag 是 LVS Real Server 的环境标签模型，用于标记 RS 所属环境（如生产环境、预生产环境）。
type LvsRSTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RSIP      string    `gorm:"size:50;uniqueIndex;not null" json:"rs_ip"`
	Tag       string    `gorm:"size:100;not null" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LvsRSTag) TableName() string {
	return "lvs_rs_tags"
}
