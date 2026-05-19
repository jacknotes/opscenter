package model

import (
	"time"

	"gorm.io/gorm"
)

// User 是系统用户模型，支持 admin/user 两级角色，使用软删除。
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	Name      string         `gorm:"size:50;not null" json:"name"`
	Email     string         `gorm:"size:100;not null" json:"email"`
	Role      string         `gorm:"size:20;not null;default:user" json:"role"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定 GORM 使用的表名为 users。
func (User) TableName() string {
	return "users"
}
