package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/config"
	"opscenter/internal/middleware"
	"opscenter/internal/model"
	"opscenter/internal/service"
)

type DashboardHandler struct {
	db             *gorm.DB
	sshManager     *service.SSHManager
	lvsService     *service.LVSService
	nginxService   *service.NginxService
	k8sService     *service.K8sService
	preprodService *service.PreprodService
}

type lvsData struct {
	VSCount       int
	RSOnline      int
	RSOffline     int
	TotalActive   int
	TotalInactive int
}

type nginxData struct {
	UpstreamCount int
	ServerOnline  int
	ServerOffline int
}

func NewDashboardHandler(db *gorm.DB, sshManager *service.SSHManager) *DashboardHandler {
	return &DashboardHandler{
		db:             db,
		sshManager:     sshManager,
		lvsService:     service.NewLVSService(sshManager),
		nginxService:   service.NewNginxService(sshManager),
		k8sService:     service.NewK8sService(sshManager),
		preprodService: service.NewPreprodService(sshManager),
	}
}

// Stats godoc
//
//	@Summary		获取仪表盘 MySQL 统计数据
//	@Description	获取服务器和用户的聚合统计信息（无需 SSH 调用）
//	@Tags			仪表盘
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/stats [get]
func (h *DashboardHandler) Stats(c *gin.Context) {
	ctx := c.Request.Context()
	result := gin.H{}

	// 服务器统计
	var total int64
	if err := h.db.WithContext(ctx).Model(&model.Server{}).Count(&total).Error; err != nil {
		log.Printf("查询服务器总数失败: %v", err)
	}

	var enabled, disabled int64
	if err := h.db.WithContext(ctx).Model(&model.Server{}).Where("enabled = ?", true).Count(&enabled).Error; err != nil {
		log.Printf("查询已启用服务器数失败: %v", err)
	}
	disabled = total - enabled

	// 按类型分组
	type typeCount struct {
		ServerType string
		Count      int64
	}
	var byType []typeCount
	if err := h.db.WithContext(ctx).Model(&model.Server{}).Where("enabled = ?", true).Select("server_type, count(*) as count").Group("server_type").Scan(&byType).Error; err != nil {
		log.Printf("查询服务器类型统计失败: %v", err)
	}
	typeMap := make(map[string]int64)
	for _, tc := range byType {
		typeMap[tc.ServerType] = tc.Count
	}

	// 按环境分组
	type envCount struct {
		Env   string
		Count int64
	}
	var byEnv []envCount
	if err := h.db.WithContext(ctx).Model(&model.Server{}).Select("env, count(*) as count").Where("env != ? AND enabled = ?", "", true).Group("env").Scan(&byEnv).Error; err != nil {
		log.Printf("查询服务器环境统计失败: %v", err)
	}
	envMap := make(map[string]int64)
	for _, ec := range byEnv {
		envMap[ec.Env] = ec.Count
	}

	result["servers"] = gin.H{
		"total":    total,
		"enabled":  enabled,
		"disabled": disabled,
		"by_type":  typeMap,
		"by_env":   envMap,
	}

	// 用户统计（仅 admin 可见）
	role, _ := c.Get("role")
	if role == "admin" {
		var userTotal int64
		if err := h.db.WithContext(ctx).Model(&model.User{}).Count(&userTotal).Error; err != nil {
			log.Printf("查询用户总数失败: %v", err)
		}

		var userEnabled, userDisabled int64
		if err := h.db.WithContext(ctx).Model(&model.User{}).Where("enabled = ?", true).Count(&userEnabled).Error; err != nil {
			log.Printf("查询已启用用户数失败: %v", err)
		}
		userDisabled = userTotal - userEnabled

		type roleCount struct {
			Role  string
			Count int64
		}
		var byRole []roleCount
		if err := h.db.WithContext(ctx).Model(&model.User{}).Select("role, count(*) as count").Group("role").Scan(&byRole).Error; err != nil {
			log.Printf("查询用户角色统计失败: %v", err)
		}
		roleMap := make(map[string]int64)
		for _, rc := range byRole {
			roleMap[rc.Role] = rc.Count
		}

		result["users"] = gin.H{
			"total":    userTotal,
			"enabled":  userEnabled,
			"disabled": userDisabled,
			"by_role":  roleMap,
		}

		// 在线用户数：通过 Redis 活跃用户集合获取
		result["online_users"] = middleware.GetActiveUserCount()
	}

	c.JSON(http.StatusOK, result)
}

// ActivityStats godoc
//
//	@Summary		获取仪表盘活动统计数据
//	@Description	按日/周/月/年统计各模块发布次数和登录成功/失败次数
//	@Tags			仪表盘
//	@Produce		json
//	@Param			granularity	query	string	true	"时间粒度: day/week/month/year"
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/activity-stats [get]
func (h *DashboardHandler) ActivityStats(c *gin.Context) {
	ctx := c.Request.Context()
	granularity := c.DefaultQuery("granularity", "day")
	actionGranularity := c.DefaultQuery("action_granularity", "")

	// 根据粒度确定时间范围和分组表达式
	var dateFormat string
	var defaultDuration time.Duration
	switch granularity {
	case "week":
		dateFormat = "%x-W%v"
		defaultDuration = 12 * 7 * 24 * time.Hour
	case "month":
		dateFormat = "%Y-%m"
		defaultDuration = 365 * 24 * time.Hour
	case "year":
		dateFormat = "%Y"
		defaultDuration = 5 * 365 * 24 * time.Hour
	default: // day
		dateFormat = "%Y-%m-%d"
		defaultDuration = 30 * 24 * time.Hour
	}

	startTime := time.Now().Add(-defaultDuration)

	// 操作动作统计使用独立的时间粒度，按自然周期起始点计算
	actionGran := granularity
	if actionGranularity != "" {
		actionGran = actionGranularity
	}
	now := time.Now()
	var actionStartTime time.Time
	switch actionGran {
	case "week":
		// 本周一 00:00
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		actionStartTime = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	case "month":
		// 本月 1 日 00:00
		actionStartTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		// 本年 1 月 1 日 00:00
		actionStartTime = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default: // day
		// 今天 00:00
		actionStartTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	type moduleStat struct {
		Period string `json:"period"`
		Module string `json:"module"`
		Count  int64  `json:"count"`
	}
	type loginStat struct {
		Period string `json:"period"`
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	// 发布统计：LVS/Nginx/K8S/Preprod
	var deployStats []moduleStat
	deployQuery := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, module, count(*) as count", dateFormat).
		Where("module IN ? AND created_at > ?", []string{"lvs", "nginx", "k8s", "preprod"}, startTime).
		Group("period, module").
		Order("period")

	// 非管理员过滤掉 auth/server 模块（虽然这里只查了 lvs/nginx/k8s/preprod，但保持一致）
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		deployQuery = deployQuery.Where("module NOT IN ?", []string{"auth", "server"})
	}

	if err := deployQuery.Scan(&deployStats).Error; err != nil {
		log.Printf("查询发布统计失败: %v", err)
	}

	// 登录统计
	var loginStats []loginStat
	loginQuery := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, status, count(*) as count", dateFormat).
		Where("module = ? AND action = ? AND created_at > ?", "auth", "login", startTime).
		Group("period, status").
		Order("period")

	// 非管理员不可见登录统计
	if userRole != "admin" {
		loginStats = []loginStat{}
	} else {
		if err := loginQuery.Scan(&loginStats).Error; err != nil {
			log.Printf("查询登录统计失败: %v", err)
		}
	}

	// 操作动作统计：按 module + action 分组，全局累计
	type actionStat struct {
		Module string `json:"module"`
		Action string `json:"action"`
		Count  int64  `json:"count"`
	}
	var actionStats []actionStat
	actionQuery := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("module, action, count(*) as count").
		Where("module IN ? AND created_at > ?", []string{"lvs", "nginx", "k8s", "preprod"}, actionStartTime).
		Group("module, action").
		Order("module, count DESC")
	if err := actionQuery.Scan(&actionStats).Error; err != nil {
		log.Printf("查询操作动作统计失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"deploy_stats":  deployStats,
		"login_stats":   loginStats,
		"action_stats":  actionStats,
	})
}

// RemoteStats godoc
//
//	@Summary		获取仪表盘远程模块统计数据
//	@Description	通过 SSH 并行查询 LVS/Nginx/K8s/Preprod 的聚合统计信息
//	@Tags			仪表盘
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/remote-stats [get]
func (h *DashboardHandler) RemoteStats(c *gin.Context) {
	ctx := c.Request.Context()
	type moduleResult struct {
		LVS     interface{} `json:"lvs"`
		Nginx   interface{} `json:"nginx"`
		K8s     interface{} `json:"k8s"`
		Preprod interface{} `json:"preprod"`
	}

	var result moduleResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// LVS 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := h.fetchLVSStats(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			log.Printf("[Dashboard] LVS 统计失败: %v", err)
			result.LVS = nil
		} else {
			result.LVS = stats
		}
	}()

	// Nginx 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := h.fetchNginxStats(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			log.Printf("[Dashboard] Nginx 统计失败: %v", err)
			result.Nginx = nil
		} else {
			result.Nginx = stats
		}
	}()

	// K8s 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := h.fetchK8sStats(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			log.Printf("[Dashboard] K8s 统计失败: %v", err)
			result.K8s = nil
		} else {
			result.K8s = stats
		}
	}()

	// Preprod 统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := h.fetchPreprodStats(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			log.Printf("[Dashboard] Preprod 统计失败: %v", err)
			result.Preprod = nil
		} else {
			result.Preprod = stats
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// all completed normally
	case <-time.After(60 * time.Second):
		log.Printf("[Dashboard] 远程统计获取超时")
	}

	c.JSON(http.StatusOK, result)
}

// fetchLVSStats 并行查询所有 LVS 服务器，聚合 VS/RS 统计。
func (h *DashboardHandler) fetchLVSStats(ctx context.Context) (gin.H, error) {
	var servers []model.Server
	if err := h.db.WithContext(ctx).Where("server_type = ? AND enabled = ?", "lvs", true).Find(&servers).Error; err != nil {
		return nil, fmt.Errorf("查询 LVS 服务器失败: %w", err)
	}
	if len(servers) == 0 {
		return gin.H{"vs_count": 0, "rs_online": 0, "rs_offline": 0, "total_active_conn": 0, "total_inact_conn": 0}, nil
	}

	results := make([]lvsData, len(servers))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, server model.Server) {
			defer wg.Done()
			d, err := h.fetchSingleLVS(ctx, &server)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", server.Name, err))
				return
			}
			results[idx] = d
		}(i, srv)
	}

	wg.Wait()

	var total lvsData
	for _, r := range results {
		total.VSCount += r.VSCount
		total.RSOnline += r.RSOnline
		total.RSOffline += r.RSOffline
		total.TotalActive += r.TotalActive
		total.TotalInactive += r.TotalInactive
	}

	if len(errs) > 0 {
		log.Printf("[Dashboard] LVS 部分服务器查询失败: %s", strings.Join(errs, "; "))
	}

	return gin.H{
		"vs_count":         total.VSCount,
		"rs_online":        total.RSOnline,
		"rs_offline":       total.RSOffline,
		"total_active_conn": total.TotalActive,
		"total_inact_conn":  total.TotalInactive,
	}, nil
}

func (h *DashboardHandler) fetchSingleLVS(ctx context.Context, server *model.Server) (lvsData, error) {
	var d lvsData

	output, err := h.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" list", config.Global.Timeouts.DashboardSSH)
	if err != nil {
		return d, fmt.Errorf("执行 list 失败: %w", err)
	}

	vsList := h.lvsService.ParseListOutput(output)

	// 获取 status 补充离线 RS
	statusOutput, statusErr := h.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" status", config.Global.Timeouts.DashboardSSH)
	if statusErr == nil && statusOutput != "" {
		statusGroups := h.lvsService.ParseStatusOutput(statusOutput)
		vsList = h.lvsService.MergeOfflineRS(vsList, statusGroups)
	}

	d.VSCount = len(vsList)
	for _, vs := range vsList {
		for _, rs := range vs.RealServers {
			if rs.Status == "up" {
				d.RSOnline++
			} else {
				d.RSOffline++
			}
			d.TotalActive += rs.ActiveConn
			d.TotalInactive += rs.InActConn
		}
	}
	return d, nil
}

// fetchNginxStats 并行查询所有 Nginx 服务器，聚合 upstream 统计。
func (h *DashboardHandler) fetchNginxStats(ctx context.Context) (gin.H, error) {
	var servers []model.Server
	if err := h.db.WithContext(ctx).Where("server_type = ? AND enabled = ?", "nginx", true).Find(&servers).Error; err != nil {
		return nil, fmt.Errorf("查询 Nginx 服务器失败: %w", err)
	}
	if len(servers) == 0 {
		return gin.H{"upstream_count": 0, "server_online": 0, "server_offline": 0}, nil
	}

	results := make([]nginxData, len(servers))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, server model.Server) {
			defer wg.Done()
			d, err := h.fetchSingleNginx(ctx, &server)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", server.Name, err))
				return
			}
			results[idx] = d
		}(i, srv)
	}

	wg.Wait()

	var total nginxData
	for _, r := range results {
		total.UpstreamCount += r.UpstreamCount
		total.ServerOnline += r.ServerOnline
		total.ServerOffline += r.ServerOffline
	}

	if len(errs) > 0 {
		log.Printf("[Dashboard] Nginx 部分服务器查询失败: %s", strings.Join(errs, "; "))
	}

	return gin.H{
		"upstream_count": total.UpstreamCount,
		"server_online":  total.ServerOnline,
		"server_offline": total.ServerOffline,
	}, nil
}

func (h *DashboardHandler) fetchSingleNginx(ctx context.Context, server *model.Server) (nginxData, error) {
	var d nginxData

	configPath := ensureTrailingSlash(server.ConfigPath)
	configPattern := server.ConfigPattern
	if configPattern == "" {
		configPattern = "*.conf"
	}

	// 解析 include/exclude 模式
	var includePatterns, excludePatterns []string
	for _, p := range strings.Split(configPattern, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "!") {
			excludePatterns = append(excludePatterns, strings.TrimPrefix(p, "!"))
		} else {
			includePatterns = append(includePatterns, p)
		}
	}

	// 列出配置文件
	fileNames := make([]string, 0)
	seen := make(map[string]bool)
	for _, pattern := range includePatterns {
		cmd := fmt.Sprintf("ls %s%s", configPath, pattern)
		output, err := h.sshManager.ExecuteWithTimeout(ctx, server, cmd, config.Global.Timeouts.DashboardSSH)
		if err != nil {
			continue
		}
		for _, f := range splitLines(output) {
			if f == "" {
				continue
			}
			if idx := strings.LastIndexByte(f, '/'); idx >= 0 {
				f = f[idx+1:]
			}
			if !seen[f] {
				seen[f] = true
				fileNames = append(fileNames, f)
			}
		}
	}

	// 排除文件
	if len(excludePatterns) > 0 {
		filtered := make([]string, 0)
		for _, f := range fileNames {
			excluded := false
			for _, ep := range excludePatterns {
				if matched, _ := filepath.Match(ep, f); matched {
					excluded = true
					break
				}
			}
			if !excluded {
				filtered = append(filtered, f)
			}
		}
		fileNames = filtered
	}

	// 读取每个配置文件并解析 upstream
	for _, fileName := range fileNames {
		cmd := fmt.Sprintf("cat %s%s", configPath, fileName)
		output, err := h.sshManager.ExecuteWithTimeout(ctx, server, cmd, config.Global.Timeouts.DashboardSSH)
		if err != nil {
			continue
		}
		upstreams := h.nginxService.ParseConfig(output)
		d.UpstreamCount += len(upstreams)
		for _, u := range upstreams {
			for _, s := range u.Servers {
				if s.Status == "up" {
					d.ServerOnline++
				} else {
					d.ServerOffline++
				}
			}
		}
	}

	return d, nil
}

// fetchK8sStats 并行查询所有 K8s 服务器，聚合 Rollout 统计。
func (h *DashboardHandler) fetchK8sStats(ctx context.Context) (gin.H, error) {
	var servers []model.Server
	if err := h.db.WithContext(ctx).Where("server_type = ? AND enabled = ?", "kubernetes", true).Find(&servers).Error; err != nil {
		return nil, fmt.Errorf("查询 K8s 服务器失败: %w", err)
	}
	if len(servers) == 0 {
		return gin.H{"total_rollouts": 0, "by_namespace": map[string]int{}, "pending": 0, "online": 0}, nil
	}

	var allRollouts []service.Rollout
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []string

	for _, srv := range servers {
		wg.Add(1)
		go func(server model.Server) {
			defer wg.Done()
			output, err := h.sshManager.ExecuteWithTimeout(ctx, &server, server.ScriptPath+" list", config.Global.Timeouts.DashboardSSH)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", server.Name, err))
				mu.Unlock()
				return
			}
			rollouts := h.k8sService.ParseListOutput(output)
			mu.Lock()
			allRollouts = append(allRollouts, rollouts...)
			mu.Unlock()
		}(srv)
	}

	wg.Wait()

	if len(errs) > 0 {
		log.Printf("[Dashboard] K8s 部分服务器查询失败: %s", strings.Join(errs, "; "))
	}

	nsMap := make(map[string]int)
	pending := 0
	online := 0
	for _, r := range allRollouts {
		nsMap[r.Namespace]++
		if r.Step == "1/5" {
			pending++
		} else if r.Step == "3/5" {
			online++
		}
	}

	return gin.H{
		"total_rollouts": len(allRollouts),
		"by_namespace":   nsMap,
		"pending":        pending,
		"online":         online,
	}, nil
}

// fetchPreprodStats 并行查询所有 Preprod 服务器，聚合资源统计。
func (h *DashboardHandler) fetchPreprodStats(ctx context.Context) (gin.H, error) {
	var servers []model.Server
	if err := h.db.WithContext(ctx).Where("server_type = ? AND enabled = ?", "preprod", true).Find(&servers).Error; err != nil {
		return nil, fmt.Errorf("查询 Preprod 服务器失败: %w", err)
	}
	if len(servers) == 0 {
		return gin.H{"total_resources": 0, "scaled_down": 0, "expanded": 0, "normal": 0}, nil
	}

	type prepodResult struct {
		Resources []service.PreprodResource
	}

	results := make([]prepodResult, len(servers))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, server model.Server) {
			defer wg.Done()
			d, err := h.fetchSinglePreprod(ctx, &server)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", server.Name, err))
				return
			}
			results[idx] = prepodResult{Resources: d}
		}(i, srv)
	}

	wg.Wait()

	if len(errs) > 0 {
		log.Printf("[Dashboard] Preprod 部分服务器查询失败: %s", strings.Join(errs, "; "))
	}

	var allResources []service.PreprodResource
	for _, r := range results {
		allResources = append(allResources, r.Resources...)
	}

	scaledDown := 0
	expanded := 0
	normal := 0
	for _, r := range allResources {
		if r.Current == 0 {
			scaledDown++
		} else if r.Current < r.TargetReplicas && r.TargetReplicas > 0 {
			expanded++
		} else {
			normal++
		}
	}

	return gin.H{
		"total_resources": len(allResources),
		"scaled_down":     scaledDown,
		"expanded":        expanded,
		"normal":          normal,
	}, nil
}

func (h *DashboardHandler) fetchSinglePreprod(ctx context.Context, server *model.Server) ([]service.PreprodResource, error) {
	listOutput, err := h.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" list", config.Global.Timeouts.DashboardSSH)
	if err != nil {
		return nil, fmt.Errorf("执行 list 失败: %w", err)
	}

	resources := h.preprodService.ParseListOutput(listOutput)

	// 获取 target 信息
	targetOutput, targetErr := h.sshManager.ExecuteWithTimeout(ctx, server, server.ScriptPath+" list-targets", config.Global.Timeouts.DashboardSSH)
	if targetErr == nil && targetOutput != "" {
		targets := h.preprodService.ParseTargetOutput(targetOutput)
		resources = h.preprodService.MergeTargets(resources, targets)
	}

	return resources, nil
}
