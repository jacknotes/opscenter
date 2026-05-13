package model

import "time"

type OperationLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	Username   string    `gorm:"size:50" json:"username"`
	Module     string    `gorm:"size:20;not null;index" json:"module"`
	Action     string    `gorm:"size:30;not null" json:"action"`
	Target     string    `gorm:"size:500" json:"target"`
	Detail     string    `gorm:"type:text" json:"detail"`
	Status     string    `gorm:"size:20;not null" json:"status"`
	Output     string    `gorm:"type:text" json:"output"`
	PreviewID  string    `gorm:"size:64" json:"preview_id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (OperationLog) TableName() string {
	return "operation_logs"
}
