package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
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
	}

	// 在线用户数：所有用户可见
	result["online_users"] = middleware.GetActiveUserCount()

	c.JSON(http.StatusOK, result)
}

// OnlineUsers godoc
//
//	@Summary		获取在线用户列表
//	@Description	获取当前在线用户的详细信息列表（仅管理员）
//	@Tags			仪表盘
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Failure		403	{object}	object
//	@Router			/dashboard/online-users [get]
func (h *DashboardHandler) OnlineUsers(c *gin.Context) {
	users := middleware.GetOnlineUsers()
	if users == nil {
		users = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// ActivityStats godoc
//
//	@Summary		获取仪表盘活动统计数据
//	@Description	按指定日期范围统计各模块发布次数和登录成功/失败次数
//	@Tags			仪表盘
//	@Produce		json
//	@Param			start_date	query	string	true	"开始日期，格式 YYYY-MM-DD"
//	@Param			end_date	query	string	true	"结束日期，格式 YYYY-MM-DD"
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/activity-stats [get]
func (h *DashboardHandler) ActivityStats(c *gin.Context) {
	ctx := c.Request.Context()
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 和 end_date 为必填参数"})
		return
	}

	now := time.Now()
	sd, err := time.ParseInLocation("2006-01-02", startDate, now.Location())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 格式错误，应为 YYYY-MM-DD"})
		return
	}
	ed, err := time.ParseInLocation("2006-01-02", endDate, now.Location())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date 格式错误，应为 YYYY-MM-DD"})
		return
	}
	endTime := ed.Add(24*time.Hour - time.Second) // end_date 23:59:59

	dateFormat := "%Y-%m-%d"

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
	if err := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, module, count(*) as count", dateFormat).
		Where("module IN ? AND created_at >= ? AND created_at <= ?", []string{"lvs", "nginx", "k8s", "preprod"}, sd, endTime).
		Group("period, module").
		Order("period").
		Scan(&deployStats).Error; err != nil {
		log.Printf("查询发布统计失败: %v", err)
	}

	// 登录统计（所有用户可见）
	var loginStats []loginStat
	if err := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, status, count(*) as count", dateFormat).
		Where("module = ? AND action = ? AND created_at >= ? AND created_at <= ?", "auth", "login", sd, endTime).
		Group("period, status").
		Order("period").
		Scan(&loginStats).Error; err != nil {
		log.Printf("查询登录统计失败: %v", err)
	}

	// 操作动作统计：按 module + action 分组，使用同一时间范围
	type actionStat struct {
		Module string `json:"module"`
		Action string `json:"action"`
		Count  int64  `json:"count"`
	}
	var actionStats []actionStat
	if err := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("module, action, count(*) as count").
		Where("module IN ? AND created_at >= ? AND created_at <= ?", []string{"lvs", "nginx", "k8s", "preprod"}, sd, endTime).
		Group("module, action").
		Order("module, count DESC").
		Scan(&actionStats).Error; err != nil {
		log.Printf("查询操作动作统计失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"deploy_stats":  deployStats,
		"login_stats":   loginStats,
		"action_stats":  actionStats,
	})
}

// K8sProjectStats godoc
//
//	@Summary		获取 K8s 项目发布统计数据
//	@Description	按指定日期范围统计 K8s 各服务的发布次数、成功率和趋势
//	@Tags			仪表盘
//	@Produce		json
//	@Param			start_date	query	string	true	"开始日期，格式 YYYY-MM-DD"
//	@Param			end_date	query	string	true	"结束日期，格式 YYYY-MM-DD"
//	@Param			server_name	query	string	false	"服务器名称筛选"
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/k8s-project-stats [get]
func (h *DashboardHandler) K8sProjectStats(c *gin.Context) {
	ctx := c.Request.Context()
	serverName := c.Query("server_name")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 和 end_date 为必填参数"})
		return
	}

	now := time.Now()
	sd, err1 := time.ParseInLocation("2006-01-02", startDate, now.Location())
	ed, err2 := time.ParseInLocation("2006-01-02", endDate, now.Location())
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误，应为 YYYY-MM-DD"})
		return
	}
	endTime := ed.Add(24*time.Hour - time.Second)

	dateFormat := "%Y-%m-%d"

	// 查询 K8s 操作日志（只取需要的字段）
	type logRow struct {
		Period       string
		ProjectNames string
		Status       string
		Action       string
		CreatedAt    time.Time
	}
	var rows []logRow
	query := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, project_names, status, action, created_at", dateFormat).
		Where("module = ? AND created_at >= ? AND created_at <= ?", "k8s", sd, endTime)
	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}
	if err := query.Order("period").Scan(&rows).Error; err != nil {
		log.Printf("查询 K8s 项目统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	log.Printf("K8s 项目统计: 查询到 %d 条记录, 范围=%s 至 %s", len(rows), startDate, endDate)

	// 在 Go 中拆分 project_names 并聚合
	type projectTrend struct {
		Period  string `json:"period"`
		Project string `json:"project"`
		Count   int64  `json:"count"`
	}
	type projectSummary struct {
		Project string `json:"project"`
		Count   int64  `json:"count"`
		Success int64  `json:"success"`
		Failed  int64  `json:"failed"`
	}
	type actionSummary struct {
		Action string `json:"action"`
		Count  int64  `json:"count"`
	}

	trendMap := make(map[string]*projectTrend)
	summaryMap := make(map[string]*projectSummary)
	actionMap := make(map[string]int64)
	var totalCount, successCount, failedCount, fullOpsCount int64

	for _, row := range rows {
		isFullOp := row.ProjectNames == "*" || row.ProjectNames == ""

		// 趋势数据
		if !isFullOp {
			projects := strings.Split(row.ProjectNames, ",")
			for _, proj := range projects {
				proj = strings.TrimSpace(proj)
				if proj == "" {
					continue
				}
				key := row.Period + "|" + proj
				if t, ok := trendMap[key]; ok {
					t.Count++
				} else {
					trendMap[key] = &projectTrend{Period: row.Period, Project: proj, Count: 1}
				}
			}
		}

		// 汇总数据（整个选定范围）
		if isFullOp {
			fullOpsCount++
			totalCount++
			if row.Status == "success" {
				successCount++
			} else {
				failedCount++
			}
			actionMap[row.Action]++
		} else {
			projects := strings.Split(row.ProjectNames, ",")
			for _, proj := range projects {
				proj = strings.TrimSpace(proj)
				if proj == "" {
					continue
				}
				if s, ok := summaryMap[proj]; ok {
					s.Count++
					if row.Status == "success" {
						s.Success++
					} else {
						s.Failed++
					}
				} else {
					s := &projectSummary{Project: proj, Count: 1}
					if row.Status == "success" {
						s.Success = 1
					} else {
						s.Failed = 1
					}
					summaryMap[proj] = s
				}
			}
			actionMap[row.Action]++
			totalCount++
			if row.Status == "success" {
				successCount++
			} else {
				failedCount++
			}
		}
	}

	// 转换为切片并排序
	trend := make([]projectTrend, 0, len(trendMap))
	for _, t := range trendMap {
		trend = append(trend, *t)
	}
	sort.Slice(trend, func(i, j int) bool {
		if trend[i].Period != trend[j].Period {
			return trend[i].Period < trend[j].Period
		}
		return trend[i].Project < trend[j].Project
	})

	byProject := make([]projectSummary, 0, len(summaryMap))
	for _, s := range summaryMap {
		byProject = append(byProject, *s)
	}
	sort.Slice(byProject, func(i, j int) bool {
		return byProject[i].Count > byProject[j].Count
	})

	byAction := make([]actionSummary, 0, len(actionMap))
	for a, cnt := range actionMap {
		byAction = append(byAction, actionSummary{Action: a, Count: cnt})
	}
	sort.Slice(byAction, func(i, j int) bool {
		return byAction[i].Count > byAction[j].Count
	})

	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"total":    totalCount,
			"success":  successCount,
			"failed":   failedCount,
			"full_ops": fullOpsCount,
		},
		"trend":      trend,
		"by_project": byProject,
		"by_action":  byAction,
	})
}

// PreprodProjectStats godoc
//
//	@Summary		获取预生产扩缩容项目统计数据
//	@Description	按指定日期范围统计预生产各服务的扩缩容次数、成功率和趋势
//	@Tags			仪表盘
//	@Produce		json
//	@Param			start_date	query	string	true	"开始日期，格式 YYYY-MM-DD"
//	@Param			end_date	query	string	true	"结束日期，格式 YYYY-MM-DD"
//	@Param			server_name	query	string	false	"服务器名称筛选"
//	@Security		BearerAuth
//	@Success		200	{object}	object
//	@Router			/dashboard/preprod-project-stats [get]
func (h *DashboardHandler) PreprodProjectStats(c *gin.Context) {
	ctx := c.Request.Context()
	serverName := c.Query("server_name")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 和 end_date 为必填参数"})
		return
	}

	now := time.Now()
	sd, err1 := time.ParseInLocation("2006-01-02", startDate, now.Location())
	ed, err2 := time.ParseInLocation("2006-01-02", endDate, now.Location())
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误，应为 YYYY-MM-DD"})
		return
	}
	endTime := ed.Add(24*time.Hour - time.Second)

	dateFormat := "%Y-%m-%d"

	type logRow struct {
		Period       string
		ProjectNames string
		Status       string
		Action       string
		CreatedAt    time.Time
	}
	var rows []logRow
	query := h.db.WithContext(ctx).Model(&model.OperationLog{}).
		Select("DATE_FORMAT(created_at, ?) as period, project_names, status, action, created_at", dateFormat).
		Where("module = ? AND created_at >= ? AND created_at <= ?", "preprod", sd, endTime)
	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}
	if err := query.Order("period").Scan(&rows).Error; err != nil {
		log.Printf("查询预生产项目统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	log.Printf("预生产项目统计: 查询到 %d 条记录, 范围=%s 至 %s", len(rows), startDate, endDate)

	type projectTrend struct {
		Period  string `json:"period"`
		Project string `json:"project"`
		Count   int64  `json:"count"`
	}
	type projectSummary struct {
		Project string `json:"project"`
		Count   int64  `json:"count"`
		Success int64  `json:"success"`
		Failed  int64  `json:"failed"`
	}
	type actionSummary struct {
		Action string `json:"action"`
		Count  int64  `json:"count"`
	}

	trendMap := make(map[string]*projectTrend)
	summaryMap := make(map[string]*projectSummary)
	actionMap := make(map[string]int64)
	var totalCount, successCount, failedCount, fullOpsCount int64

	for _, row := range rows {
		isFullOp := row.ProjectNames == "*" || row.ProjectNames == ""

		// 趋势数据
		if !isFullOp {
			projects := strings.Split(row.ProjectNames, ",")
			for _, proj := range projects {
				proj = strings.TrimSpace(proj)
				if proj == "" {
					continue
				}
				key := row.Period + "|" + proj
				if t, ok := trendMap[key]; ok {
					t.Count++
				} else {
					trendMap[key] = &projectTrend{Period: row.Period, Project: proj, Count: 1}
				}
			}
		}

		// 汇总数据（整个选定范围）
		if isFullOp {
			fullOpsCount++
			totalCount++
			if row.Status == "success" {
				successCount++
			} else {
				failedCount++
			}
			actionMap[row.Action]++
		} else {
			projects := strings.Split(row.ProjectNames, ",")
			for _, proj := range projects {
				proj = strings.TrimSpace(proj)
				if proj == "" {
					continue
				}
				if s, ok := summaryMap[proj]; ok {
					s.Count++
					if row.Status == "success" {
						s.Success++
					} else {
						s.Failed++
					}
				} else {
					s := &projectSummary{Project: proj, Count: 1}
					if row.Status == "success" {
						s.Success = 1
					} else {
						s.Failed = 1
					}
					summaryMap[proj] = s
				}
			}
			actionMap[row.Action]++
			totalCount++
			if row.Status == "success" {
				successCount++
			} else {
				failedCount++
			}
		}
	}

	trend := make([]projectTrend, 0, len(trendMap))
	for _, t := range trendMap {
		trend = append(trend, *t)
	}
	sort.Slice(trend, func(i, j int) bool {
		if trend[i].Period != trend[j].Period {
			return trend[i].Period < trend[j].Period
		}
		return trend[i].Project < trend[j].Project
	})

	byProject := make([]projectSummary, 0, len(summaryMap))
	for _, s := range summaryMap {
		byProject = append(byProject, *s)
	}
	sort.Slice(byProject, func(i, j int) bool {
		return byProject[i].Count > byProject[j].Count
	})

	byAction := make([]actionSummary, 0, len(actionMap))
	for a, cnt := range actionMap {
		byAction = append(byAction, actionSummary{Action: a, Count: cnt})
	}
	sort.Slice(byAction, func(i, j int) bool {
		return byAction[i].Count > byAction[j].Count
	})

	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"total":    totalCount,
			"success":  successCount,
			"failed":   failedCount,
			"full_ops": fullOpsCount,
		},
		"trend":      trend,
		"by_project": byProject,
		"by_action":  byAction,
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

// LvsConnStats godoc
//
//	@Summary		获取 LVS 连接数时序统计
//	@Description	查询指定 VS/RS 的 ActiveConn 和 InActConn 时序数据，用于折线图展示
//	@Tags			仪表盘
//	@Produce		json
//	@Security		BearerAuth
//	@Param			server_id	query		int		true	"LVS 服务器 ID"
//	@Param			vs_ip		query		string	true	"Virtual Server IP"
//	@Param			rs_ip		query		string	true	"Real Server IP"
//	@Param			duration	query		int		false	"时间窗口（分钟）: 5/15/30/60，默认 15"
//	@Success		200			{object}	object
//	@Failure		400			{object}	object
//	@Router			/dashboard/lvs-conn-stats [get]
func (h *DashboardHandler) LvsConnStats(c *gin.Context) {
	ctx := c.Request.Context()

	serverID := c.Query("server_id")
	vsIP := c.Query("vs_ip")
	rsIP := c.Query("rs_ip")
	if serverID == "" || vsIP == "" || rsIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 server_id、vs_ip 和 rs_ip"})
		return
	}

	duration := 15 // 默认 15 分钟
	if d := c.Query("duration"); d != "" {
		switch d {
		case "5", "15", "30", "60":
			duration = parseInt(d)
		}
	}

	startTime := time.Now().Add(-time.Duration(duration) * time.Minute)

	type connStat struct {
		CollectedAt time.Time `json:"collected_at"`
		ActiveConn  int       `json:"active_conn"`
		InActConn   int       `json:"inact_conn"`
	}

	var data []connStat
	if err := h.db.WithContext(ctx).
		Model(&model.LvsConnStat{}).
		Select("collected_at, SUM(active_conn) as active_conn, SUM(in_act_conn) as in_act_conn").
		Where("server_id = ? AND vs_ip = ? AND rs_ip = ? AND collected_at >= ?", serverID, vsIP, rsIP, startTime).
		Group("collected_at").
		Order("collected_at ASC").
		Scan(&data).Error; err != nil {
		log.Printf("[Dashboard] 查询 LVS 连接统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
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
