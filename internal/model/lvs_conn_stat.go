package model

import "time"

// LvsConnStat 是 LVS 连接数时序统计数据模型，由后台采集器每 30 秒写入一次。
// 记录每个 VS 下每个 RS 的 ActiveConn 和 InActConn 快照，用于 Dashboard 图表展示。
type LvsConnStat struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ServerID    uint      `gorm:"not null;index:idx_lcs_query" json:"server_id"`
	ServerName  string    `gorm:"size:100" json:"server_name"`
	VSIP        string    `gorm:"size:50;not null;index:idx_lcs_query" json:"vs_ip"`
	VSPort      string    `gorm:"size:10;not null" json:"vs_port"`
	RSIP        string    `gorm:"size:50;not null;index:idx_lcs_query" json:"rs_ip"`
	RSPort      string    `gorm:"size:10;not null" json:"rs_port"`
	ActiveConn  int       `json:"active_conn"`
	InActConn   int       `json:"inact_conn"`
	CollectedAt time.Time `gorm:"not null;index:idx_lcs_query" json:"collected_at"`
}

func (LvsConnStat) TableName() string {
	return "lvs_conn_stats"
}
