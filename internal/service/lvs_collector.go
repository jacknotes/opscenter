package service

import (
	"context"
	"log"
	"sync"
	"time"

	"opscenter/internal/model"

	"gorm.io/gorm"
)

// LvsCollector 是 LVS 连接数后台采集器，定时通过 SSH 执行 ipvsadm -ln，
// 解析每个 RS 的 ActiveConn/InActConn 并写入数据库，供 Dashboard 图表展示。
type LvsCollector struct {
	db         *gorm.DB
	sshManager *SSHManager
	lvsService *LVSService
	interval   time.Duration
	stopCh     chan struct{}
}

// NewLvsCollector 创建 LVS 连接数采集器。
// interval 为采集间隔，建议 30 秒。
func NewLvsCollector(db *gorm.DB, sshManager *SSHManager, interval time.Duration) *LvsCollector {
	return &LvsCollector{
		db:         db,
		sshManager: sshManager,
		lvsService: NewLVSService(sshManager),
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动后台采集循环，通过 ctx 控制生命周期。
// 首次采集在启动后立即执行一次。
func (c *LvsCollector) Start(ctx context.Context) {
	log.Printf("[LvsCollector] 启动，采集间隔: %v", c.interval)

	// 首次采集
	c.collect(ctx)

	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("[LvsCollector] 收到 context 取消信号，停止采集")
				return
			case <-c.stopCh:
				log.Println("[LvsCollector] 收到停止信号，停止采集")
				return
			case <-ticker.C:
				c.collect(ctx)
			}
		}
	}()
}

// Stop 停止采集器。
func (c *LvsCollector) Stop() {
	close(c.stopCh)
}

// collect 执行一次数据采集：查询所有 LVS 服务器，并发获取连接数据并批量写入。
func (c *LvsCollector) collect(ctx context.Context) {
	var servers []model.Server
	if err := c.db.WithContext(ctx).Where("server_type = ? AND enabled = ?", "lvs", true).Find(&servers).Error; err != nil {
		log.Printf("[LvsCollector] 查询 LVS 服务器失败: %v", err)
		return
	}
	if len(servers) == 0 {
		return
	}

	now := time.Now()
	var allStats []model.LvsConnStat
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, srv := range servers {
		wg.Add(1)
		go func(server model.Server) {
			defer wg.Done()
			stats := c.collectOne(ctx, &server, now)
			if len(stats) > 0 {
				mu.Lock()
				allStats = append(allStats, stats...)
				mu.Unlock()
			}
		}(srv)
	}

	wg.Wait()

	if len(allStats) == 0 {
		return
	}

	// 批量写入
	if err := c.db.CreateInBatches(allStats, 200).Error; err != nil {
		log.Printf("[LvsCollector] 批量写入失败 (%d 条): %v", len(allStats), err)
	} else {
		log.Printf("[LvsCollector] 采集完成，写入 %d 条记录", len(allStats))
	}
}

// collectOne 从单台 LVS 服务器采集连接数据。
func (c *LvsCollector) collectOne(ctx context.Context, server *model.Server, now time.Time) []model.LvsConnStat {
	// 设置单台服务器的采集超时为 20 秒
	sshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	output, err := c.sshManager.Execute(sshCtx, server, server.ScriptPath+" list")
	if err != nil {
		log.Printf("[LvsCollector] %s 执行 list 失败: %v", server.Name, err)
		return nil
	}

	vsList := c.lvsService.ParseListOutput(output)
	if len(vsList) == 0 {
		return nil
	}

	// 获取 status 补充离线 RS
	statusOutput, statusErr := c.sshManager.Execute(sshCtx, server, server.ScriptPath+" status")
	if statusErr == nil && statusOutput != "" {
		statusGroups := c.lvsService.ParseStatusOutput(statusOutput)
		vsList = c.lvsService.MergeOfflineRS(vsList, statusGroups)
	}

	var stats []model.LvsConnStat
	for _, vs := range vsList {
		for _, rs := range vs.RealServers {
			stats = append(stats, model.LvsConnStat{
				ServerID:    server.ID,
				ServerName:  server.Name,
				VSIP:        vs.IP,
				VSPort:      vs.Port,
				RSIP:        rs.IP,
				RSPort:      rs.Port,
				ActiveConn:  rs.ActiveConn,
				InActConn:   rs.InActConn,
				CollectedAt: now,
			})
		}
	}
	return stats
}
