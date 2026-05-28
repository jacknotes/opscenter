package model

import "time"

// LvsVSTag 是 LVS Virtual Server 的标签模型，用于标记 VS 所属分组。
type LvsVSTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VSIP      string    `gorm:"size:50;uniqueIndex;not null" json:"vs_ip"`
	Tag       string    `gorm:"size:100" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LvsVSTag) TableName() string {
	return "lvs_vs_tags"
}
