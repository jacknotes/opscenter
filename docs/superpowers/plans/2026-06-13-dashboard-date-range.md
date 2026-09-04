# Dashboard 日期范围选择器改造 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Dashboard 中「按天/按周/按月/按年」独立按钮移除，改为按钮组+日期框的统一日期范围选择器，所有图表根据用户选定的日期范围显示数据。

**Architecture:** 后端三个端点（activity-stats、k8s-project-stats、preprod-project-stats）统一改为接收 `start_date`/`end_date` 参数，移除 `granularity`。前端抽取 `DateRangeSelector.vue` 可复用组件，三组图表共用。

**Tech Stack:** Go + Gin + GORM (后端)、Vue 3 + Element Plus + ECharts (前端)

---

## File Structure

| 文件 | 变更类型 | 职责 |
|---|---|---|
| `internal/handler/dashboard.go` | 修改 | 三个端点改为 `start_date`/`end_date` 参数 |
| `web/src/components/DateRangeSelector.vue` | 新建 | 可复用日期范围选择器组件 |
| `web/src/views/Dashboard/index.vue` | 修改 | 替换粒度按钮为 DateRangeSelector，更新加载逻辑 |
| `web/src/api/dashboard.js` | 无需修改 | API 函数签名不变，调用方传参变化 |

---

### Task 1: 后端 — 改造 ActivityStats 端点

**Files:**
- Modify: `internal/handler/dashboard.go:190-303`

- [ ] **Step 1: 重构 ActivityStats 函数签名和参数解析**

将 `granularity`/`action_granularity` 参数替换为 `start_date`/`end_date`，移除粒度相关的 switch 分支，固定使用 `%Y-%m-%d` 格式。

在 `internal/handler/dashboard.go` 中，将 `ActivityStats` 函数的前 50 行（190-239）替换为：

```go
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
```

- [ ] **Step 2: 更新三个查询使用统一的 startTime/endTime**

替换 252-296 行的三个查询，使用 `sd` 和 `endTime` 替代原来的 `startTime` 和 `actionStartTime`：

```go
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
```

- [ ] **Step 3: 验证后端编译通过**

Run: `cd /home/jack/claudecode/opscenter && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add internal/handler/dashboard.go
git commit -m "refactor(dashboard): activity-stats 端点改为 start_date/end_date 参数"
```

---

### Task 2: 后端 — 改造 K8sProjectStats 端点

**Files:**
- Modify: `internal/handler/dashboard.go:318-532`

- [ ] **Step 1: 重构 K8sProjectStats 参数解析**

移除 `granularity` 参数和两层时间窗口逻辑，统一使用 `start_date`/`end_date`。

将 318-398 行替换为：

```go
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
```

- [ ] **Step 2: 更新聚合逻辑，移除 summaryStartTime 分支**

将 401-491 行的聚合循环中，移除 `summaryStartTime` 的条件判断，所有数据统一参与汇总。

将原来的聚合代码（从 `// 在 Go 中拆分 project_names 并聚合` 到循环结束）替换为：

```go
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

	trendMap := make(map[string]*projectTrend)     // "period|project" -> trend
	summaryMap := make(map[string]*projectSummary) // "project" -> summary
	actionMap := make(map[string]int64)            // "action" -> count
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
```

- [ ] **Step 3: 验证后端编译通过**

Run: `cd /home/jack/claudecode/opscenter && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add internal/handler/dashboard.go
git commit -m "refactor(dashboard): k8s-project-stats 端点移除 granularity，统一使用 start_date/end_date"
```

---

### Task 3: 后端 — 改造 PreprodProjectStats 端点

**Files:**
- Modify: `internal/handler/dashboard.go:547-757`

- [ ] **Step 1: 重构 PreprodProjectStats 参数解析**

与 K8sProjectStats 完全对称的改动。将 547-627 行替换为：

```go
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
```

- [ ] **Step 2: 更新聚合逻辑，移除 summaryStartTime 分支**

将 629-717 行的聚合代码替换为与 Task 1 Step 2 相同的结构（将 `"k8s"` 改为已删除的模块判断，因为这里只查 `"preprod"`）：

```go
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
```

- [ ] **Step 3: 验证后端编译通过**

Run: `cd /home/jack/claudecode/opscenter && go build ./...`
Expected: 无错误输出

- [ ] **Step 4: 提交**

```bash
git add internal/handler/dashboard.go
git commit -m "refactor(dashboard): preprod-project-stats 端点移除 granularity，统一使用 start_date/end_date"
```

---

### Task 4: 前端 — 创建 DateRangeSelector 组件

**Files:**
- Create: `web/src/components/DateRangeSelector.vue`

- [ ] **Step 1: 创建 DateRangeSelector.vue 组件**

```vue
<template>
  <div class="date-range-selector">
    <el-radio-group :model-value="activePreset" size="small" @update:model-value="onPresetChange">
      <el-radio-button value="today">今天</el-radio-button>
      <el-radio-button value="7d">7天</el-radio-button>
      <el-radio-button value="14d">14天</el-radio-button>
      <el-radio-button value="30d">30天</el-radio-button>
    </el-radio-group>
    <el-date-picker
      :model-value="modelValue"
      type="daterange"
      range-separator="至"
      start-placeholder="开始日期"
      end-placeholder="结束日期"
      format="YYYY-MM-DD"
      value-format="YYYY-MM-DD"
      size="small"
      :disabled-date="disabledDate"
      style="width: 260px"
      @update:model-value="onDateChange"
    />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

const props = defineProps({
  modelValue: {
    type: Array,
    default: null,
  },
  defaultPreset: {
    type: String,
    default: '7d',
    validator: (v) => ['today', '7d', '14d', '30d'].includes(v),
  },
})

const emit = defineEmits(['update:modelValue'])

const activePreset = ref(props.defaultPreset)

// 快捷按钮计算逻辑
const presetMap = {
  today: () => {
    const d = new Date()
    const ds = formatDate(d)
    return [ds, ds]
  },
  '7d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 6)
    return [formatDate(s), formatDate(e)]
  },
  '14d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 13)
    return [formatDate(s), formatDate(e)]
  },
  '30d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 29)
    return [formatDate(s), formatDate(e)]
  },
}

function formatDate(d) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function onPresetChange(preset) {
  activePreset.value = preset
  const range = presetMap[preset]()
  emit('update:modelValue', range)
}

function onDateChange(val) {
  activePreset.value = null
  emit('update:modelValue', val)
}

function disabledDate(date) {
  // 如果已有起始日，限制结束日距起始日不超过 30 天
  if (props.modelValue && props.modelValue.length === 2 && props.modelValue[0]) {
    const start = new Date(props.modelValue[0])
    const diff = Math.abs(date.getTime() - start.getTime())
    const days = diff / (1000 * 60 * 60 * 24)
    return days > 30
  }
  return false
}

// 初始化时根据 defaultPreset 设置日期范围
onMounted(() => {
  if (!props.modelValue && props.defaultPreset) {
    const range = presetMap[props.defaultPreset]()
    activePreset.value = props.defaultPreset
    emit('update:modelValue', range)
  }
})
</script>

<style scoped>
.date-range-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
```

- [ ] **Step 2: 验证前端构建通过**

Run: `cd /home/jack/claudecode/opscenter && npm run build --prefix web`
Expected: 构建成功，无错误

- [ ] **Step 3: 提交**

```bash
git add web/src/components/DateRangeSelector.vue
git commit -m "feat(ui): 新建 DateRangeSelector 可复用组件"
```

---

### Task 5: 前端 — 改造活动统计组（登录/发布/操作明细）

**Files:**
- Modify: `web/src/views/Dashboard/index.vue`

- [ ] **Step 1: 在 imports 中引入 DateRangeSelector**

在 `index.vue` 的 import 区域（约 515 行），添加：

```js
import DateRangeSelector from '../../components/DateRangeSelector.vue'
```

- [ ] **Step 2: 添加活动统计组的 dateRange ref**

在 `index.vue` 的 ref 声明区域（约 629 行），将：

```js
const loginGranularity = ref('day')
const deployGranularity = ref('day')
```

替换为：

```js
const activityDateRange = ref(null)
```

在 635 行，删除：

```js
const actionGranularity = ref('day')
```

- [ ] **Step 3: 替换登录统计趋势模板中的粒度按钮**

找到登录统计趋势区域的 `el-radio-group`（约 43-57 行），整块删除。

找到该区域的标题行（包含「登录统计趋势」文字），在标题右侧添加 `DateRangeSelector`。具体来说，找到类似如下的结构：

```vue
<div class="chart-header">
  <span class="chart-title">登录统计趋势</span>
  <!-- 原来的 el-radio-group 在这里 -->
</div>
```

替换为：

```vue
<div class="chart-header">
  <span class="chart-title">登录统计趋势</span>
  <DateRangeSelector v-model="activityDateRange" />
</div>
```

- [ ] **Step 4: 替换发布趋势模板中的粒度按钮**

找到「各模块发布次数趋势」区域的 `el-radio-group`（约 117-131 行），整块删除。

在该区域的标题处添加 `DateRangeSelector`（与登录统计共享同一个 `activityDateRange`）：

```vue
<div class="chart-header">
  <span class="chart-title">各模块发布次数趋势</span>
  <DateRangeSelector v-model="activityDateRange" />
</div>
```

- [ ] **Step 5: 替换操作明细模板中的粒度按钮**

找到「各模块操作动作明细」区域的 `el-radio-group`（约 164-178 行），整块删除。

在该区域的标题处添加 `DateRangeSelector`（共享同一个 `activityDateRange`）：

```vue
<div class="chart-header">
  <span class="chart-title">各模块操作动作明细</span>
  <DateRangeSelector v-model="activityDateRange" />
</div>
```

- [ ] **Step 6: 合并三个加载函数为一个**

将 `loadLoginStats`（1207-1218 行）、`loadActivityStats`（1200-1205 行）、`loadActionStats`（1220-1228 行）三个函数合并为一个 `loadAllActivityStats` 函数：

```js
async function loadAllActivityStats() {
  if (!activityDateRange.value || activityDateRange.value.length !== 2) return
  try {
    const params = {
      start_date: activityDateRange.value[0],
      end_date: activityDateRange.value[1],
    }
    const res = await getActivityStats(params)
    deployChartData.value = res.deploy_stats || []
    loginChartData.value = res.login_stats || []
    actionStatsData.value = res.action_stats || []

    // 更新今日登录统计卡片
    const today = new Date().toISOString().slice(0, 10)
    const todayLogins = (res.login_stats || []).filter((d) => d.period === today)
    todayLoginSuccess.value = todayLogins.find((d) => d.status === 'success')?.count || 0
    todayLoginFailed.value = todayLogins.find((d) => d.status === 'failed')?.count || 0
  } catch {}
}
```

- [ ] **Step 7: 添加 activityDateRange 的 watcher**

在 watch 区域（约 1279 行附近），添加：

```js
watch(activityDateRange, () => { loadAllActivityStats() })
```

- [ ] **Step 8: 更新 onMounted/onActivated 中的调用**

找到 `onMounted` 和 `onActivated` 中原来调用 `loadLoginStats()`、`loadActivityStats()`、`loadActionStats()` 的地方，统一替换为 `loadAllActivityStats()`。

- [ ] **Step 9: 验证前端构建通过**

Run: `cd /home/jack/claudecode/opscenter && npm run build --prefix web`
Expected: 构建成功，无错误

- [ ] **Step 10: 提交**

```bash
git add web/src/views/Dashboard/index.vue
git commit -m "refactor(ui): 活动统计组改用 DateRangeSelector，移除粒度按钮"
```

---

### Task 6: 前端 — 改造 K8S 项目统计组

**Files:**
- Modify: `web/src/views/Dashboard/index.vue`

- [ ] **Step 1: 移除 k8sProjectGranularity ref**

在 ref 声明区域（约 945 行），删除：

```js
const k8sProjectGranularity = ref('day')
```

将 `k8sDateRange` 的默认值从 `null` 改为由 DateRangeSelector 自动初始化（保持 `ref(null)`，组件 mounted 时会 emit 初始值）。

- [ ] **Step 2: 替换 K8S 模板中的粒度按钮和日期选择器**

找到 K8S 区域的 `el-date-picker`（约 208-218 行）和 `el-radio-group`（约 219-233 行），两块都删除。

替换为一个 `DateRangeSelector`，放在服务器筛选下拉框右侧：

```vue
<el-select v-model="k8sServerFilter" placeholder="全部服务器" clearable size="small" style="width: 150px">
  <el-option label="全部服务器" value="" />
  <el-option v-for="s in k8sServers" :key="s.id" :label="s.name" :value="s.name" />
</el-select>
<DateRangeSelector v-model="k8sDateRange" />
```

- [ ] **Step 3: 更新 loadK8sProjectStats 函数**

将 `loadK8sProjectStats`（约 1230-1246 行）替换为：

```js
async function loadK8sProjectStats() {
  if (!k8sDateRange.value || k8sDateRange.value.length !== 2) return
  try {
    const params = {
      start_date: k8sDateRange.value[0],
      end_date: k8sDateRange.value[1],
    }
    if (k8sServerFilter.value) params.server_name = k8sServerFilter.value
    const res = await getK8sProjectStats(params)
    k8sProjectSummary.value = res.summary || { total: 0, success: 0, failed: 0, full_ops: 0 }
    k8sProjectTrend.value = res.trend || []
    k8sProjectByProject.value = res.by_project || []
    k8sProjectByAction.value = res.by_action || []
  } catch (err) {
    console.error('加载 K8S 项目统计失败:', err)
  }
}
```

- [ ] **Step 4: 验证前端构建通过**

Run: `cd /home/jack/claudecode/opscenter && npm run build --prefix web`
Expected: 构建成功，无错误

- [ ] **Step 5: 提交**

```bash
git add web/src/views/Dashboard/index.vue
git commit -m "refactor(ui): K8S 项目统计组改用 DateRangeSelector，移除粒度按钮"
```

---

### Task 7: 前端 — 改造预生产扩缩容统计组

**Files:**
- Modify: `web/src/views/Dashboard/index.vue`

- [ ] **Step 1: 移除 preprodProjectGranularity ref**

在 ref 声明区域（约 1047 行），删除：

```js
const preprodProjectGranularity = ref('day')
```

- [ ] **Step 2: 替换预生产模板中的粒度按钮和日期选择器**

找到预生产区域的 `el-date-picker`（约 336-346 行）和 `el-radio-group`（约 347-361 行），两块都删除。

替换为：

```vue
<el-select v-model="preprodServerFilter" placeholder="全部服务器" clearable size="small" style="width: 150px">
  <el-option label="全部服务器" value="" />
  <el-option v-for="s in preprodServers" :key="s.id" :label="s.name" :value="s.name" />
</el-select>
<DateRangeSelector v-model="preprodDateRange" />
```

- [ ] **Step 3: 更新 loadPreprodProjectStats 函数**

将 `loadPreprodProjectStats`（约 1248-1264 行）替换为：

```js
async function loadPreprodProjectStats() {
  if (!preprodDateRange.value || preprodDateRange.value.length !== 2) return
  try {
    const params = {
      start_date: preprodDateRange.value[0],
      end_date: preprodDateRange.value[1],
    }
    if (preprodServerFilter.value) params.server_name = preprodServerFilter.value
    const res = await getPreprodProjectStats(params)
    preprodProjectSummary.value = res.summary || { total: 0, success: 0, failed: 0, full_ops: 0 }
    preprodProjectTrend.value = res.trend || []
    preprodProjectByProject.value = res.by_project || []
    preprodProjectByAction.value = res.by_action || []
  } catch (err) {
    console.error('加载预生产项目统计失败:', err)
  }
}
```

- [ ] **Step 4: 验证前端构建通过**

Run: `cd /home/jack/claudecode/opscenter && npm run build --prefix web`
Expected: 构建成功，无错误

- [ ] **Step 5: 提交**

```bash
git add web/src/views/Dashboard/index.vue
git commit -m "refactor(ui): 预生产统计组改用 DateRangeSelector，移除粒度按钮"
```

---

### Task 8: 清理与验证

**Files:**
- Modify: `web/src/views/Dashboard/index.vue`

- [ ] **Step 1: 删除不再使用的 dateShortcuts**

删除 `dateShortcuts` 定义（约 604-608 行），因为 K8S/预生产不再使用 `el-date-picker` 的 `shortcuts` 属性。

- [ ] **Step 2: 清理不再使用的 ref 和函数**

确认以下 ref 和函数已无引用，删除残留：
- `loginGranularity`、`deployGranularity`、`actionGranularity`
- `k8sProjectGranularity`、`preprodProjectGranularity`
- `loadLoginStats`、`loadActivityStats`、`loadActionStats`（已被 `loadAllActivityStats` 替代）

- [ ] **Step 3: 完整构建验证**

Run: `cd /home/jack/claudecode/opscenter && make build`
Expected: 前后端构建成功

- [ ] **Step 4: 提交**

```bash
git add web/src/views/Dashboard/index.vue
git commit -m "chore(dashboard): 清理不再使用的粒度相关代码"
```
