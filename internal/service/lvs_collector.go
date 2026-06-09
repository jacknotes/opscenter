package service

import (
	"context"
	"log"
	"sync"
	"time"

	"opscenter/internal/config"
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
	stopOnce   sync.Once
	doneCh     chan struct{} // 采集 goroutine 退出后关闭，用于优雅停机
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
		doneCh:     make(chan struct{}),
	}
}

// Start 启动后台采集循环，通过 ctx 控制生命周期。
// 首次采集在 goroutine 中异步执行，不阻塞调用方。
func (c *LvsCollector) Start(ctx context.Context) {
	log.Printf("[LvsCollector] 启动，采集间隔: %v", c.interval)

	go func() {
		defer close(c.doneCh)

		// 首次采集
		c.collect(ctx)

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

// Stop 停止采集器。可安全重复调用。
func (c *LvsCollector) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}

// Wait 等待采集 goroutine 完全退出。用于优雅停机，确保 SSH 连接关闭前采集已结束。
func (c *LvsCollector) Wait() {
	<-c.doneCh
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
	timeout := config.Global.Timeouts.DashboardSSH

	output, err := c.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" list", timeout)
	if err != nil {
		log.Printf("[LvsCollector] %s 执行 list 失败: %v", server.Name, err)
		return nil
	}

	vsList := c.lvsService.ParseListOutput(output)
	if len(vsList) == 0 {
		return nil
	}

	// 获取 status 补充离线 RS
	statusOutput, statusErr := c.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" status", timeout)
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
