# Dashboard 日期范围选择器改造设计

**日期**: 2026-06-13
**状态**: 待审核
**范围**: Dashboard 页面中 6 个时间相关图表的交互改造

## 背景

当前 Dashboard 有 6 个涉及时间的图表，使用了 3 种不同的时间选择模式：

1. **纯粒度按钮**（登录统计、发布趋势、操作明细）：只有「按天/按周/按月/按年」radio 按钮，无日期范围选择，后端自行计算时间窗口
2. **粒度 + 日期范围 + 服务器筛选**（K8S 项目、预生产）：有日期范围选择器和粒度按钮并存
3. **时间窗口按钮**（LVS 连接）：5/15/30/60 分钟的实时监控模式

用户要求：移除「按天/按周/按月/按年」独立按钮，将快捷方式集成到日期选择框中，所有图表根据用户选定的日期范围显示数据。

## 分组架构

Dashboard 中涉及时间的 6 个图表分为 4 组：

| 组 | 图表 | 时间控制方式 | 变更 |
|---|---|---|---|
| **A. 活动统计** | 登录统计趋势、各模块发布次数趋势、各模块操作动作明细 | 共享一个日期范围选择器 | 移除粒度按钮，新增按钮组+日期框 |
| **B. K8S 项目** | K8S 项目发布统计 | 独立日期范围选择器 + 服务器筛选 | 更新快捷按钮，移除粒度按钮，限制最大 30 天 |
| **C. 预生产** | 预生产扩缩容统计 | 独立日期范围选择器 + 服务器筛选 | 同上 |
| **LVS 连接** | LVS 连接统计 | 保持现状（5/15/30/60 分钟按钮） | 无变更 |

### 权限说明

活动统计组的所有图表，管理员和普通用户看到的内容完全一致。后端代码中虽有对非管理员额外过滤 `auth`/`server` 模块的逻辑，但查询本身已限定为 `lvs/nginx/k8s/preprod` 四个模块，该过滤为冗余代码，不影响实际结果。

## 日期范围选择器设计

### 组件结构

每组的日期选择器由两部分组成：

```
┌──────────────────────────────────────────────────────────┐
│  [今天] [7天] [14天] [30天]  [📅 2026-06-07 至 2026-06-13]  │
└──────────────────────────────────────────────────────────┘
```

- 左侧：`el-radio-group` 快捷按钮（今天、7天、14天、30天）
- 右侧：`el-date-picker` 日期范围框（`type="daterange"`）

### 按钮组行为

| 按钮 | 计算逻辑 | 跨度 |
|---|---|---|
| 今天 | 当天 ~ 当天 | 1 天 |
| 7天 | 当前日期往前推 6 天 ~ 当前日期 | 7 天 |
| 14天 | 往前推 13 天 ~ 当前日期 | 14 天 |
| 30天 | 往前推 29 天 ~ 当前日期 | 30 天 |

> 注：前端传入的 `end_date` 为日期字符串（如 `2026-06-13`），后端自动扩展到当天 23:59:59。

- 点击按钮后自动计算起止日期并填充到日期框
- 当前选中的按钮高亮（主题色），其他按钮恢复默认

### 日期框行为

- 使用 Element Plus 的 `el-date-picker`，`type="daterange"`
- 通过 `disabledDate` 回调限制最大选择范围 30 天
- 日期格式 `YYYY-MM-DD`
- 用户手动选择日期范围时，所有快捷按钮取消高亮

### 状态同步

- 按钮组和日期框双向联动：点击按钮更新日期框，手动选日期取消按钮高亮
- 每组的日期范围状态独立管理

### 布局位置

- **活动统计组**：选择器放在该组卡片标题栏右侧
- **K8S 项目组**：选择器放在标题栏右侧，服务器筛选下拉框在选择器左侧
- **预生产组**：同 K8S

## 后端 API 改造

### 参数变更

| 端点 | 当前参数 | 改为 |
|---|---|---|
| `/dashboard/activity-stats` | `granularity` (必填)，`action_granularity` (可选) | `start_date` + `end_date` (必填) |
| `/dashboard/k8s-project-stats` | `granularity` + `start_date` + `end_date` + `server_name` | `start_date` + `end_date` + `server_name` |
| `/dashboard/preprod-project-stats` | 同上 | 同上 |

### activity-stats 端点改造

**移除：**
- `granularity` 参数及相关的 `switch` 分支（计算 `dateFormat`、`defaultDuration`）
- `action_granularity` 参数及相关的 `actionStartTime` 计算逻辑

**新增：**
- 接收 `start_date`、`end_date` 查询参数（格式 `YYYY-MM-DD`）
- `end_date` 扩展到当天 23:59:59
- `DATE_FORMAT` 固定使用 `%Y-%m-%d`（按天分组，因为最大 30 天范围内无需按周/月/年）

**查询逻辑：**
```go
// 发布统计
WHERE module IN ('lvs','nginx','k8s','preprod')
  AND created_at >= startDate
  AND created_at <= endDate  // 23:59:59
GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d'), module

// 登录统计
WHERE module = 'auth' AND action = 'login'
  AND created_at >= startDate AND created_at <= endDate
GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d'), status

// 操作动作统计 — 使用同一时间范围（移除独立的 actionStartTime）
WHERE module IN ('lvs','nginx','k8s','preprod')
  AND created_at >= startDate AND created_at <= endDate
GROUP BY module, action
```

### k8s-project-stats / preprod-project-stats 端点改造

**移除：**
- `granularity` 参数及相关的两层时间窗口逻辑（trend window / summary window）
- `defaultDuration` 和 `summaryStartTime` 的 switch 分支

**保留：**
- `server_name` 可选筛选参数

**改动：**
- 统一使用 `start_date`/`end_date` 作为查询范围
- Summary 数据改为查询整个选定范围（不再只取当前自然周期）
- Trend 数据和 Summary 数据使用同一个时间窗口
- `DATE_FORMAT` 固定使用 `%Y-%m-%d`

### 响应格式

三个端点的响应结构保持不变：

```json
// activity-stats
{
  "deploy_stats": [{"period": "2026-06-07", "module": "k8s", "count": 15}],
  "login_stats": [{"period": "2026-06-07", "status": "success", "count": 50}],
  "action_stats": [{"module": "k8s", "action": "online", "count": 20}]
}

// k8s-project-stats / preprod-project-stats
{
  "summary": {"total": 150, "success": 140, "failed": 10, "full_ops": 5},
  "trend": [{"period": "2026-06-07", "project": "service-a", "count": 5}],
  "by_project": [{"project": "service-a", "count": 20, "success": 18, "failed": 2}],
  "by_action": [{"action": "online", "count": 80}]
}
```

唯一变化：`period` 字段统一为 `YYYY-MM-DD` 格式。

## 前端改造

### 新建可复用组件

**`web/src/components/DateRangeSelector.vue`**

```vue
<template>
  <div class="date-range-selector">
    <el-radio-group v-model="activePreset" @change="onPresetChange" size="small">
      <el-radio-button label="today">今天</el-radio-button>
      <el-radio-button label="7d">7天</el-radio-button>
      <el-radio-button label="14d">14天</el-radio-button>
      <el-radio-button label="30d">30天</el-radio-button>
    </el-radio-group>
    <el-date-picker
      v-model="dateRange"
      type="daterange"
      format="YYYY-MM-DD"
      :disabled-date="disabledDate"
      @change="onDateChange"
    />
  </div>
</template>
```

**Props：**
- `modelValue` — 当前日期范围 `[startDate, endDate]`，支持 `v-model`
- `defaultPreset` — 默认选中的快捷按钮，可选值 `today`/`7d`/`14d`/`30d`，默认 `7d`

**Events：**
- `update:modelValue` — 日期范围变化时触发

**内部逻辑：**
- 点击按钮 → 计算日期 → 更新 `dateRange` → 触发 `emit`
- 手动选日期 → 清除按钮高亮（`activePreset = null`）→ 触发 `emit`
- `disabledDate`：当用户已选起始日时，结束日距起始日超过 30 天的日期禁用；未选起始日时不额外限制

### index.vue 改造要点

**活动统计组：**
- 引入 `DateRangeSelector`，放在该组卡片标题栏右侧
- 移除 `loginGranularity`、`deployGranularity`、`actionGranularity` 三个 ref 和对应的 radio group
- 移除 `action_granularity` 相关逻辑
- 新增 `activityDateRange` ref，默认值为 7 天前 ~ 今天
- watch `activityDateRange`，变化时调用 `loadActivityStats()`

**K8S 项目组：**
- 将现有的 `el-date-picker` 替换为 `DateRangeSelector`
- 保留服务器筛选 `el-select`，放在选择器左侧
- 移除现有的 `k8sGranularity` radio group
- 新增 `k8sDateRange` ref
- watch `k8sDateRange`，变化时调用 `loadK8sProjectStats()` 并重置分页

**预生产组：**
- 同 K8S 项目组的改造方式
- 移除 `preprodGranularity` radio group

**LVS 连接组：**
- 无变更

### API 调用改造

```js
// web/src/api/dashboard.js

// 之前
getActivityStats({ granularity: 'day' })
getK8sProjectStats({ granularity: 'week', start_date: '...', end_date: '...', server_name: '...' })

// 之后
getActivityStats({ start_date: '2026-06-07', end_date: '2026-06-13' })
getK8sProjectStats({ start_date: '2026-06-07', end_date: '2026-06-13', server_name: '...' })
```

API 函数签名不变，调用时传入的参数变化。

## 默认行为与边界处理

**默认值：**
- 三组日期选择器均默认选中「7天」（当前日期往前推 6 天 ~ 当前日期）
- LVS 连接组保持默认「15分钟」

**边界处理：**
- `disabledDate` 回调禁止选择超过 30 天的范围
- 无数据时段：图表正常显示空状态（折线图无数据点，柱状图无柱子），不弹错误提示
- 按钮与日期框联动：手动选日期后按钮不高亮；点击按钮后日期框自动填充

**不受影响的部分：**
- StatCards（在线用户、今日登录成功/失败）：保持不变，固定显示今日数据
- ModulePies（4 个环形图）：保持不变，显示实时 SSH 采集数据
- 服务器类型饼图、用户角色饼图：保持不变
- LVS 连接统计：保持不变

## 涉及文件

| 文件 | 变更类型 |
|---|---|
| `web/src/components/DateRangeSelector.vue` | 新建 |
| `web/src/views/Dashboard/index.vue` | 修改 |
| `web/src/api/dashboard.js` | 修改（参数调整） |
| `internal/handler/dashboard.go` | 修改 |
