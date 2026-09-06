<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import type { ActionStat, ProjectStat, ProjectSummary, ProjectTrendPoint, ServerResponse } from '@/api/types'
import BaseChart from '@/components/BaseChart.vue'
import DateRangeSelector from '@/components/DateRangeSelector.vue'
import { useTheme } from '@/composables/useTheme'

const props = defineProps<{
    title: string
    /** 全屏滚动定位用的图表名（data-chart 属性值） */
    chartName: string
    /** 项目列表计数列名（发布次数 / 操作次数） */
    countLabel: string
    servers: ServerResponse[]
    serverName: string
    /** 卡片级日期范围（对齐 v1：每张统计卡独立选择时间段） */
    dateRange: string[] | null
    loading: boolean
    summary: ProjectSummary
    trend: ProjectTrendPoint[]
    byProject: ProjectStat[]
    byAction: ActionStat[]
    fullscreen: boolean
}>()

const emit = defineEmits<{
  'update:serverName': [value: string]
  'update:dateRange': [value: string[] | null]
  refresh: []
  toggleFullscreen: []
}>()

const { theme } = useTheme()

// ---- 主题感知配色 ----
const palette = computed(() => {
  void theme.value
  const dark = theme.value === 'dark'
  return {
    text: dark ? '#e2e8f0' : '#1e293b',
    subText: dark ? '#94a3b8' : '#475569',
    muted: dark ? '#64748b' : '#64748b',
    split: dark ? 'rgba(148,163,184,0.12)' : 'rgba(71,85,105,0.12)',
    border: dark ? 'rgba(148,163,184,0.16)' : 'rgba(71,85,105,0.16)',
    tooltipBg: dark ? '#1e293b' : '#ffffff',
  }
})

const SERIES_COLORS = ['#fbbf24', '#38bdf8', '#34d399', '#f87171', '#a78bfa', '#ec4899', '#2dd4bf', '#fb923c']

// ---- tooltip 配置 ----
function tooltipConf(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    backgroundColor: palette.value.tooltipBg,
    borderColor: palette.value.subText,
    textStyle: { color: palette.value.text, fontSize: 13 },
    confine: true,
    ...extra,
  }
}

// 过滤 tooltip 中值为 0 的项
function filterZeroAxisTooltip(params: unknown): string {
  const list = (Array.isArray(params) ? params : []).filter(
    (p) => typeof p === 'object' && p !== null && (p as { value?: number }).value !== 0,
  ) as { axisValue?: string; marker: string; seriesName: string; value: number }[]
  if (list.length === 0) return ''
  let html = (list[0].axisValue ?? '') + '<br/>'
  for (const p of list) html += p.marker + ' ' + p.seriesName + ': ' + p.value + '<br/>'
  return html
}

// ---- 动作名称中文化 ----
const ACTION_NAMES: Record<string, Record<string, string>> = {
  k8s: {
    online: '上线',
    sync: '同步',
    rollback: '回滚',
    full_online: '完全上线',
    full_sync: '完全同步',
    full_rollback: '完全回滚',
  },
  preprod: {
    scaledown: '缩容',
    scaleup: '扩容',
    batch_scaledown: '批量缩容',
    batch_scaleup: '批量扩容',
    full_scaledown: '全量缩容',
    full_scaleup: '全量扩容',
  },
}

const kind = computed(() => (props.title.includes('预生产') ? 'preprod' : 'k8s'))

// ---- 服务发布趋势（Top5 项目折线） ----
const trendOption = computed<EChartsCoreOption>(() => {
  if (props.trend.length === 0) return {}
  const periods = [...new Set(props.trend.map((d) => d.period))].sort()
  const projectCounts = new Map<string, number>()
  for (const d of props.trend) projectCounts.set(d.project, (projectCounts.get(d.project) ?? 0) + d.count)
  const projects = [...projectCounts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([name]) => name)
  const series = projects.map((proj) => {
    const map = new Map<string, number>()
    for (const d of props.trend) if (d.project === proj) map.set(d.period, d.count)
    return {
      name: proj,
      type: 'line' as const,
      smooth: true,
      symbol: 'circle',
      symbolSize: 6,
      data: periods.map((p) => map.get(p) ?? 0),
      lineStyle: { width: 2 },
      areaStyle: { opacity: 0.08 },
    }
  })
  return {
    tooltip: tooltipConf({ trigger: 'axis', formatter: filterZeroAxisTooltip }),
    grid: { top: 10, right: 16, bottom: periods.length > 15 ? 40 : 20, left: 50 },
    // 数据窗口缩放：周期较多时可拖动滑块查看区间（对齐 v1 dataZoom）
    ...(periods.length > 15
      ? {
          dataZoom: [
            {
              type: 'slider',
              height: 14,
              bottom: 6,
              startValue: periods[Math.max(0, periods.length - 15)],
              end: 100,
              borderColor: palette.value.border,
              textStyle: { color: palette.value.muted, fontSize: 10 },
            },
          ],
        }
      : {}),
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: palette.value.subText, fontSize: 11, rotate: periods.length > 15 ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: palette.value.split } },
      axisLabel: { color: palette.value.muted, fontSize: 11 },
    },
    color: SERIES_COLORS,
    series,
  }
})

// ---- 操作类型环形图 ----
const actionPieOption = computed<EChartsCoreOption>(() => {
  if (props.byAction.length === 0) return {}
  const names = ACTION_NAMES[kind.value]
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: palette.value.subText, fontSize: 11 } },
    color: SERIES_COLORS,
    series: [
      {
        type: 'pie',
        radius: ['40%', '65%'],
        center: ['50%', '45%'],
        label: { show: true, formatter: '{b}\n{d}%', color: palette.value.subText, fontSize: 11 },
        data: props.byAction.map((a) => ({ name: names?.[a.action] ?? a.action, value: a.count })),
      },
    ],
  }
})

// ---- 项目明细列表：排序 + 分页 ----
const page = ref(1)
const PAGE_SIZE = 15
const sortAsc = ref(false)

const sortedProjects = computed<ProjectStat[]>(() =>
  [...props.byProject].sort((a, b) => (sortAsc.value ? a.count - b.count : b.count - a.count)),
)

const pagedProjects = computed(() => {
  const start = (page.value - 1) * PAGE_SIZE
  return sortedProjects.value.slice(start, start + PAGE_SIZE)
})

const totalPages = computed(() => Math.ceil(sortedProjects.value.length / PAGE_SIZE))

watch(
  () => props.byProject,
  () => {
    page.value = 1
  },
)

function onServerChange(val: string | undefined): void {
  emit('update:serverName', val ?? '')
  emit('refresh')
}

function onDateRangeChange(val: string[] | null): void {
  emit('update:dateRange', val)
}
</script>

<template>
  <div
    class="card chart-card reveal"
    :data-chart="chartName"
    :class="{ 'fullscreen-card': props.fullscreen }"
    :style="props.fullscreen ? { flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' } : {}"
  >
    <div class="chart-head">
      <h3 class="chart-title">{{ props.title }}</h3>
      <div class="head-controls">
        <DateRangeSelector
          :model-value="props.dateRange"
          @update:model-value="onDateRangeChange"
        />
        <el-select
          :model-value="props.serverName"
          placeholder="全部服务器"
          clearable
          size="small"
          style="width: 150px"
          @change="onServerChange"
        >
          <el-option label="全部服务器" value="" />
          <el-option v-for="s in props.servers" :key="s.id" :label="s.name" :value="s.name" />
        </el-select>
        <el-button text type="primary" size="small" :loading="props.loading" @click="emit('refresh')">
          刷新
        </el-button>
        <el-button text type="primary" size="small" @click="emit('toggleFullscreen')">
          {{ props.fullscreen ? '退出全屏' : '全屏' }}
        </el-button>
      </div>
    </div>

    <!-- 汇总指标 -->
    <div class="metric-row">
      <div class="metric">
        <div class="metric-label">总{{ kind === 'preprod' ? '操作' : '发布' }}次数</div>
        <div class="metric-value" style="color: #f59e0b">{{ props.summary.total }}</div>
      </div>
      <div class="metric">
        <div class="metric-label">全量次数</div>
        <div class="metric-value" style="color: #a78bfa">{{ props.summary.full_ops ?? 0 }}</div>
      </div>
      <div class="metric">
        <div class="metric-label">成功次数</div>
        <div class="metric-value" style="color: #34d399">{{ props.summary.success }}</div>
      </div>
      <div class="metric">
        <div class="metric-label">失败次数</div>
        <div class="metric-value" style="color: #f87171">{{ props.summary.failed }}</div>
      </div>
    </div>

    <!-- 趋势 + 操作类型 -->
    <div class="charts-split">
      <div class="split-left">
        <div class="sub-label">服务{{ kind === 'preprod' ? '扩缩容' : '发布' }}趋势</div>
        <BaseChart
          v-if="Object.keys(trendOption).length > 0"
          :option="trendOption"
          height="220px"
          :loading="props.loading"
          class="split-chart"
          :class="{ 'fullscreen-chart': props.fullscreen }"
        />
        <div v-else class="chart-empty">暂无数据</div>
      </div>
      <div class="split-right">
        <div class="sub-label">操作类型</div>
        <BaseChart
          v-if="Object.keys(actionPieOption).length > 0"
          :option="actionPieOption"
          height="220px"
          class="split-chart"
          :class="{ 'fullscreen-chart': props.fullscreen }"
        />
        <div v-else class="chart-empty">暂无数据</div>
      </div>
    </div>

    <!-- 项目明细列表 -->
    <div v-if="sortedProjects.length > 0" class="project-list-section">
      <div class="sub-label">
        项目明细 <span class="project-total-count">共 {{ sortedProjects.length }} 个项目</span>
      </div>
      <div class="project-list-header">
        <span class="col-name">项目名称</span>
        <span class="col-count sortable" @click="sortAsc = !sortAsc">
          {{ props.countLabel }}
          <span class="sort-icon">{{ sortAsc ? '↑' : '↓' }}</span>
        </span>
      </div>
      <div v-for="item in pagedProjects" :key="item.project" class="project-list-item">
        <span class="col-name" :title="item.project">{{ item.project }}</span>
        <span class="col-count">{{ item.count }}</span>
      </div>
      <div v-if="totalPages > 1" class="project-list-pagination">
        <el-pagination
          v-model:current-page="page"
          :page-size="PAGE_SIZE"
          :total="sortedProjects.length"
          layout="prev, pager, next"
          small
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.chart-card {
  padding: var(--space-4) var(--space-5);
  min-width: 0;
}

.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}

.chart-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: var(--text-md);
  color: var(--text-primary);
  font-weight: 600;
}

.head-controls {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.metric-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

.metric {
  background: var(--bg-input);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-2);
  text-align: center;
}

.metric-label {
  font-size: var(--text-xs);
  color: var(--text-secondary);
  margin-bottom: 2px;
}

.metric-value {
  font-size: var(--text-xl);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.charts-split {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: var(--space-4);
}

@media (max-width: 900px) {
  .charts-split {
    grid-template-columns: 1fr;
  }
}

.split-left,
.split-right {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.split-chart {
  flex: 1;
  min-height: 0;
}

.chart-empty {
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.sub-label {
  font-size: var(--text-sm);
  font-weight: 600;
  margin-bottom: var(--space-2);
  color: var(--text-secondary);
}

.project-total-count {
  font-weight: 400;
  font-size: var(--text-xs);
  color: var(--text-muted);
  margin-left: var(--space-2);
}

.project-list-section {
  margin-top: var(--space-3);
}

.project-list-header {
  display: flex;
  align-items: center;
  padding: var(--space-1) 0;
  border-bottom: 1px solid var(--border-faint);
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--text-secondary);
}

.project-list-item {
  display: flex;
  align-items: center;
  padding: 5px 0;
  border-bottom: 1px solid var(--border-faint);
  font-size: var(--text-xs);
  color: var(--text-secondary);
}

.project-list-item:last-child {
  border-bottom: none;
}

.col-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: var(--space-3);
}

.col-count {
  width: 80px;
  text-align: right;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.col-count.sortable {
  cursor: pointer;
  user-select: none;
}

.sort-icon {
  font-size: var(--text-xs);
}

.project-list-pagination {
  display: flex;
  justify-content: center;
  margin-top: var(--space-2);
}

/* 全屏时图表区撑满剩余高度 */
.fullscreen-card .charts-split {
  flex: 1;
  min-height: 0;
}

.fullscreen-chart {
  height: auto !important;
}
</style>
