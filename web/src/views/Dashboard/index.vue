<template>
  <div class="dashboard">
    <!-- 顶部标题栏 -->
    <div class="dash-toolbar">
      <div class="dash-toolbar-left">
        <h1 class="dash-title">运维总览</h1>
        <span class="dash-hint">远程状态每 30 分钟自动刷新</span>
      </div>
      <div class="dash-toolbar-right">
        <span v-if="lastUpdated" class="dash-last-updated"> 更新于 {{ formatTimeShort(lastUpdated) }} </span>
        <el-button text type="primary" size="small" :loading="remoteLoading" @click="refreshAll">
          <el-icon style="margin-right: 4px"><Refresh /></el-icon>
          刷新全部
        </el-button>
      </div>
    </div>

    <!-- 数字卡片 -->
    <StatCards :online-users="onlineUsers" :login-success="todayLoginSuccess" :login-failed="todayLoginFailed" />

    <!-- 模块实时状态（4 个饼图） -->
    <ModulePies
      :loading="remoteLoading"
      :error="remoteError"
      :lvs-stats="lvsStats"
      :nginx-stats="nginxStats"
      :k8s-stats="k8sStats"
      :preprod-stats="preprodStats"
      :theme-colors="themeColors"
      :colors="C"
      :card-bg="cardBg"
      @retry="loadRemoteStats"
    />

    <!-- 登录统计 + 服务器分布 + 用户分布 -->
    <div class="dash-row" :class="userStore.isAdmin ? 'misc-row-admin' : 'misc-row'">
      <el-card class="chart-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span class="chart-title">登录统计趋势</span>
            <el-radio-group
              :model-value="loginGranularity"
              size="small"
              @update:model-value="
                (v) => {
                  loginGranularity = v
                  loadLoginStats()
                }
              "
            >
              <el-radio-button value="day">按天</el-radio-button>
              <el-radio-button value="week">按周</el-radio-button>
              <el-radio-button value="month">按月</el-radio-button>
              <el-radio-button value="year">按年</el-radio-button>
            </el-radio-group>
          </div>
        </template>
        <v-chart v-if="loginChartData.length > 0" class="trend-chart" :option="loginBarOption" autoresize />
        <div v-else class="empty-state">
          <el-icon class="empty-state-icon"><DataLine /></el-icon>
          <span class="empty-state-text">暂无登录数据</span>
        </div>
      </el-card>
      <template v-if="userStore.isAdmin">
        <el-card class="chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="chart-title">服务器类型分布</span>
              <el-tag size="small" type="info" effect="plain">共 {{ serverStats?.total || 0 }} 台</el-tag>
            </div>
          </template>
          <v-chart v-if="serverStats?.by_type" class="chart chart--sm" :option="serverTypePie" autoresize />
          <div v-else-if="statsLoading" class="chart-loading"><el-skeleton :rows="3" animated /></div>
          <div v-else class="chart-empty">暂无数据</div>
        </el-card>
        <el-card class="chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="chart-title">用户角色分布</span>
              <el-tag size="small" type="info" effect="plain">共 {{ userStats?.total || 0 }} 人</el-tag>
            </div>
          </template>
          <v-chart v-if="userStats?.by_role" class="chart chart--sm" :option="userRolePie" autoresize />
          <div v-else-if="statsLoading" class="chart-loading"><el-skeleton :rows="3" animated /></div>
          <div v-else class="chart-empty">暂无数据</div>
        </el-card>
      </template>
    </div>

    <!-- LVS 连接统计 -->
    <div class="dash-row">
      <LvsConnChart />
    </div>

    <!-- 发布趋势 + 操作动作明细 -->
    <div class="dash-row trend-action-row">
      <el-card class="chart-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span class="chart-title">各模块发布次数趋势</span>
            <el-radio-group
              :model-value="deployGranularity"
              size="small"
              @update:model-value="
                (v) => {
                  deployGranularity = v
                  loadActivityStats()
                }
              "
            >
              <el-radio-button value="day">按天</el-radio-button>
              <el-radio-button value="week">按周</el-radio-button>
              <el-radio-button value="month">按月</el-radio-button>
              <el-radio-button value="year">按年</el-radio-button>
            </el-radio-group>
          </div>
        </template>
        <v-chart v-if="deployChartData.length > 0" class="trend-chart" :option="deployLineOption" autoresize />
        <div v-else class="empty-state">
          <el-icon class="empty-state-icon"><DataLine /></el-icon>
          <span class="empty-state-text">暂无发布数据</span>
        </div>
      </el-card>
      <el-card class="chart-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span class="chart-title">各模块操作动作明细</span>
            <el-radio-group
              :model-value="actionGranularity"
              size="small"
              @update:model-value="
                (v) => {
                  actionGranularity = v
                  loadActionStats()
                }
              "
            >
              <el-radio-button value="day">按天</el-radio-button>
              <el-radio-button value="week">按周</el-radio-button>
              <el-radio-button value="month">按月</el-radio-button>
              <el-radio-button value="year">按年</el-radio-button>
            </el-radio-group>
          </div>
        </template>
        <el-tabs v-model="activeActionTab" type="border-card" class="action-tabs">
          <el-tab-pane v-for="mod in actionModules" :key="mod.key" :label="mod.label" :name="mod.key">
            <v-chart v-if="Object.keys(mod.option).length > 0" class="action-chart" :option="mod.option" autoresize />
            <div v-else class="action-empty">暂无{{ mod.label }}操作记录</div>
          </el-tab-pane>
        </el-tabs>
      </el-card>
    </div>

    <!-- K8S 项目发布统计 -->
    <div class="dash-row">
      <el-card class="chart-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span class="chart-title">K8S 项目发布统计</span>
            <div class="card-header-controls">
              <el-select v-model="k8sServerFilter" placeholder="全部服务器" clearable size="small" style="width: 150px">
                <el-option label="全部服务器" value="" />
                <el-option v-for="s in k8sServers" :key="s.id" :label="s.name" :value="s.name" />
              </el-select>
              <el-radio-group
                :model-value="k8sProjectGranularity"
                size="small"
                @update:model-value="
                  (v) => {
                    k8sProjectGranularity = v
                    loadK8sProjectStats()
                  }
                "
              >
                <el-radio-button value="day">按天</el-radio-button>
                <el-radio-button value="week">按周</el-radio-button>
                <el-radio-button value="month">按月</el-radio-button>
                <el-radio-button value="year">按年</el-radio-button>
              </el-radio-group>
              <el-button text type="primary" size="small" @click="loadK8sProjectStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
            </div>
          </div>
        </template>
        <el-row :gutter="10" class="metric-row">
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">总发布</div>
              <div class="metric-value" style="color: #f59e0b">{{ k8sProjectSummary.total }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">全量</div>
              <div class="metric-value" style="color: #8b5cf6">{{ k8sProjectSummary.full_ops || 0 }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">成功</div>
              <div class="metric-value" style="color: #22c55e">{{ k8sProjectSummary.success }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">失败</div>
              <div class="metric-value" style="color: #ef4444">{{ k8sProjectSummary.failed }}</div>
            </div></el-col
          >
        </el-row>
        <el-row :gutter="12">
          <el-col :span="16">
            <div class="sub-label">服务发布趋势</div>
            <v-chart
              v-if="Object.keys(k8sTrendOption).length > 0"
              :option="k8sTrendOption"
              autoresize
              style="height: 220px"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
          <el-col :span="8">
            <div class="sub-label">操作类型</div>
            <v-chart
              v-if="Object.keys(k8sActionPieOption).length > 0"
              :option="k8sActionPieOption"
              autoresize
              style="height: 220px"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
        </el-row>
        <div v-if="k8sProjectRanking.length > 0" class="ranking-section">
          <div class="sub-label">发布排行 Top 5</div>
          <div v-for="item in k8sProjectRanking.slice(0, 5)" :key="item.project" class="ranking-item">
            <span class="ranking-name" :title="item.project">{{ item.project }}</span>
            <div class="ranking-bar-bg">
              <div class="ranking-bar" :style="{ width: k8sRankingBarWidth(item.count) }"></div>
            </div>
            <span class="ranking-count">{{ item.count }}</span>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 预生产扩缩容统计 -->
    <div class="dash-row">
      <el-card class="chart-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <span class="chart-title">预生产扩缩容统计</span>
            <div class="card-header-controls">
              <el-select
                v-model="preprodServerFilter"
                placeholder="全部服务器"
                clearable
                size="small"
                style="width: 150px"
              >
                <el-option label="全部服务器" value="" />
                <el-option v-for="s in preprodServers" :key="s.id" :label="s.name" :value="s.name" />
              </el-select>
              <el-radio-group
                :model-value="preprodProjectGranularity"
                size="small"
                @update:model-value="
                  (v) => {
                    preprodProjectGranularity = v
                    loadPreprodProjectStats()
                  }
                "
              >
                <el-radio-button value="day">按天</el-radio-button>
                <el-radio-button value="week">按周</el-radio-button>
                <el-radio-button value="month">按月</el-radio-button>
                <el-radio-button value="year">按年</el-radio-button>
              </el-radio-group>
              <el-button text type="primary" size="small" @click="loadPreprodProjectStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
            </div>
          </div>
        </template>
        <el-row :gutter="10" class="metric-row">
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">总操作</div>
              <div class="metric-value" style="color: #ef4444">{{ preprodProjectSummary.total }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">全量</div>
              <div class="metric-value" style="color: #8b5cf6">{{ preprodProjectSummary.full_ops || 0 }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">成功</div>
              <div class="metric-value" style="color: #22c55e">{{ preprodProjectSummary.success }}</div>
            </div></el-col
          >
          <el-col :span="6"
            ><div class="metric">
              <div class="metric-label">失败</div>
              <div class="metric-value" style="color: #ef4444">{{ preprodProjectSummary.failed }}</div>
            </div></el-col
          >
        </el-row>
        <el-row :gutter="12">
          <el-col :span="16">
            <div class="sub-label">服务扩缩容趋势</div>
            <v-chart
              v-if="Object.keys(preprodTrendOption).length > 0"
              :option="preprodTrendOption"
              autoresize
              style="height: 220px"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
          <el-col :span="8">
            <div class="sub-label">操作类型</div>
            <v-chart
              v-if="Object.keys(preprodActionPieOption).length > 0"
              :option="preprodActionPieOption"
              autoresize
              style="height: 220px"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
        </el-row>
        <div v-if="preprodProjectRanking.length > 0" class="ranking-section">
          <div class="sub-label">操作排行 Top 5</div>
          <div v-for="item in preprodProjectRanking.slice(0, 5)" :key="item.project" class="ranking-item">
            <span class="ranking-name" :title="item.project">{{ item.project }}</span>
            <div class="ranking-bar-bg">
              <div class="ranking-bar preprod" :style="{ width: preprodRankingBarWidth(item.count) }"></div>
            </div>
            <span class="ranking-count">{{ item.count }}</span>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, onMounted, onActivated, watch, nextTick } from 'vue'
import {
  getDashboardStats,
  getDashboardRemoteStats,
  getActivityStats,
  getK8sProjectStats,
  getPreprodProjectStats,
  getServers,
} from '../../api'
import { useUserStore } from '../../stores/user'
import { useAppStore } from '../../stores/app'
import { ElMessage } from 'element-plus'
import { DataLine, Refresh } from '@element-plus/icons-vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { PieChart, LineChart, BarChart } from 'echarts/charts'
import { TitleComponent, TooltipComponent, LegendComponent, GridComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { formatTimeShort } from '../../utils/format'
import StatCards from './StatCards.vue'
import ModulePies from './ModulePies.vue'
import LvsConnChart from './LvsConnChart.vue'

use([
  PieChart,
  LineChart,
  BarChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  DataZoomComponent,
  CanvasRenderer,
])

const userStore = useUserStore()
const appStore = useAppStore()

// ---- 主题感知（缓存 CSS 变量，仅主题切换时刷新） ----
let _cssVarCache = null
function getCSSVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}
function readThemeVars() {
  return {
    text: getCSSVar('--text-primary') || '#E2E8F0',
    subText: getCSSVar('--text-regular') || '#94A3B8',
    muted: getCSSVar('--text-secondary') || '#64748B',
    border: getCSSVar('--border-default') || 'rgba(255,255,255,0.06)',
    tooltipBg: getCSSVar('--bg-elevated') || '#1A1D2E',
    tooltipBorder: getCSSVar('--border-default') || 'rgba(255,255,255,0.1)',
    cardBg: getCSSVar('--card-bg') || '#141722',
    lvs: getCSSVar('--module-lvs') || '#06B6D4',
    nginx: getCSSVar('--module-nginx') || '#22C55E',
    k8s: getCSSVar('--module-k8s') || '#8B5CF6',
    preprod: getCSSVar('--module-preprod') || '#F59E0B',
  }
}
function ensureCssVarCache() {
  if (!_cssVarCache) _cssVarCache = readThemeVars()
  return _cssVarCache
}
watch(() => appStore.theme, () => { _cssVarCache = null })

const themeColors = computed(() => {
  const v = ensureCssVarCache()
  return { text: v.text, subText: v.subText, muted: v.muted, border: v.border, tooltipBg: v.tooltipBg, tooltipBorder: v.tooltipBorder }
})

const C = computed(() => {
  const v = ensureCssVarCache()
  return {
    lvs: v.lvs, nginx: v.nginx, k8s: v.k8s, preprod: v.preprod,
    success: '#22C55E', failed: '#EF4444',
    online: '#22C55E', offline: '#EF4444',
    pending: '#F59E0B', other: '#94A3B8',
    scaledDown: '#EF4444', expanded: '#F59E0B', normal: '#22C55E',
  }
})

const cardBg = computed(() => ensureCssVarCache().cardBg)

const MODULE_NAMES = { lvs: 'LVS', nginx: 'Nginx', k8s: 'K8S', preprod: '预生产' }

// ---- 数据 ----
const onlineUsers = ref(0)
const todayLoginSuccess = ref(0)
const todayLoginFailed = ref(0)
const statsLoading = ref(true)
const serverStats = ref(null)
const userStats = ref(null)
const remoteLoading = ref(true)
const lastUpdated = ref(null)
const remoteError = ref(null)
const lvsStats = ref(null)
const nginxStats = ref(null)
const k8sStats = ref(null)
const preprodStats = ref(null)
const deployGranularity = ref('day')
const loginGranularity = ref('day')
const deployChartData = shallowRef([])
const loginChartData = shallowRef([])
const actionStatsData = shallowRef([])
const activeActionTab = ref('lvs')
const actionGranularity = ref('day')

// ---- tooltip 配置 ----
function tooltipConf(extra = {}) {
  return {
    backgroundColor: themeColors.value.tooltipBg,
    borderColor: themeColors.value.tooltipBorder,
    textStyle: { color: themeColors.value.text, fontSize: 13 },
    ...extra,
  }
}

// ---- 饼图：服务器类型分布 ----
const serverTypePie = computed(() => {
  if (!serverStats.value?.by_type) return {}
  const data = Object.entries(serverStats.value.by_type).map(([k, v]) => ({
    name: k === 'kubernetes' ? 'K8S' : k.toUpperCase(),
    value: v,
  }))
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} 台 ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: themeColors.value.subText, fontSize: 12 } },
    color: [C.value.lvs, C.value.nginx, C.value.k8s, C.value.preprod],
    series: [
      {
        type: 'pie',
        radius: '60%',
        center: ['50%', '42%'],
        avoidLabelOverlap: true,
        itemStyle: { borderRadius: 8, borderColor: ensureCssVarCache().cardBg, borderWidth: 3 },
        label: { show: true, formatter: '{b}\n{c} 台', color: themeColors.value.subText, fontSize: 12 },
        emphasis: {
          label: { fontSize: 15, fontWeight: 'bold', color: themeColors.value.text },
          itemStyle: { shadowBlur: 20, shadowColor: 'rgba(0,0,0,0.2)' },
        },
        data,
      },
    ],
  }
})

// ---- 饼图：用户角色分布 ----
const userRolePie = computed(() => {
  if (!userStats.value?.by_role) return {}
  const data = Object.entries(userStats.value.by_role).map(([k, v]) => ({
    name: k === 'admin' ? '管理员' : '普通用户',
    value: v,
  }))
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} 人 ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: themeColors.value.subText, fontSize: 12 } },
    color: ['#EF4444', '#64748B'],
    series: [
      {
        type: 'pie',
        radius: '60%',
        center: ['50%', '42%'],
        avoidLabelOverlap: true,
        itemStyle: { borderRadius: 8, borderColor: ensureCssVarCache().cardBg, borderWidth: 3 },
        label: { show: true, formatter: '{b}\n{c} 人', color: themeColors.value.subText, fontSize: 12 },
        emphasis: {
          label: { fontSize: 15, fontWeight: 'bold', color: themeColors.value.text },
          itemStyle: { shadowBlur: 20, shadowColor: 'rgba(0,0,0,0.2)' },
        },
        data,
      },
    ],
  }
})

// ---- 操作动作统计 ----
const ACTION_NAMES = {
  lvs: { op: '上线/下线', swap: '切换' },
  nginx: { online: '上线', offline: '下线', swap: '切换', toggle: '单个切换', batch: '批量操作', rollback: '回滚' },
  k8s: {
    online: '上线',
    sync: '同步',
    rollback: '回滚',
    full_online: '完全上线',
    full_sync: '完全同步',
    full_rollback: '完全回滚',
  },
  preprod: { scaleup: '扩容', scaledown: '缩容' },
}

function makeActionBarOption(module) {
  const items = actionStatsData.value.filter((d) => d.module === module)
  if (items.length === 0) return {}
  items.sort((a, b) => b.count - a.count)
  const names = items.map((d) => ACTION_NAMES[module]?.[d.action] || d.action)
  const values = items.map((d) => d.count)
  const color = C.value[module]
  return {
    tooltip: tooltipConf({ trigger: 'axis', axisPointer: { type: 'shadow' } }),
    grid: { left: 20, right: 50, top: 20, bottom: 30, containLabel: true },
    xAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: themeColors.value.border, type: 'dashed' } },
      axisLabel: { color: themeColors.value.muted, fontSize: 12 },
    },
    yAxis: {
      type: 'category',
      data: names,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: themeColors.value.subText, fontSize: 13 },
    },
    series: [
      {
        type: 'bar',
        data: values,
        barWidth: '50%',
        itemStyle: {
          borderRadius: [0, 6, 6, 0],
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 1,
            y2: 0,
            colorStops: [
              { offset: 0, color: color + '33' },
              { offset: 1, color },
            ],
          },
        },
        emphasis: { itemStyle: { shadowBlur: 12, shadowColor: color + '40' } },
        label: { show: true, position: 'right', color: themeColors.value.text, fontSize: 14, fontWeight: 700 },
      },
    ],
  }
}

const actionModules = computed(() => [
  { key: 'lvs', label: 'LVS', option: makeActionBarOption('lvs') },
  { key: 'nginx', label: 'Nginx', option: makeActionBarOption('nginx') },
  { key: 'k8s', label: 'K8S', option: makeActionBarOption('k8s') },
  { key: 'preprod', label: '预生产', option: makeActionBarOption('preprod') },
])

// ---- 折线图：发布趋势 ----
const deployLineOption = computed(() => {
  if (deployChartData.value.length === 0) return {}
  const periods = [...new Set(deployChartData.value.map((d) => d.period))].sort()
  const modules = ['lvs', 'nginx', 'k8s', 'preprod']
  const series = modules.map((mod) => {
    const map = {}
    deployChartData.value
      .filter((d) => d.module === mod)
      .forEach((d) => {
        map[d.period] = d.count
      })
    return {
      name: MODULE_NAMES[mod],
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 8,
      lineStyle: { width: 3 },
      areaStyle: {
        opacity: 0.15,
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: C.value[mod] },
            { offset: 1, color: 'transparent' },
          ],
        },
      },
      data: periods.map((p) => map[p] || 0),
    }
  })
  const showZoom = periods.length > 15
  return {
    tooltip: tooltipConf({
      trigger: 'axis',
      axisPointer: { type: 'cross', lineStyle: { color: themeColors.value.muted, type: 'dashed' } },
    }),
    legend: {
      data: modules.map((m) => MODULE_NAMES[m]),
      right: 0,
      top: 'center',
      orient: 'vertical',
      textStyle: { color: themeColors.value.subText, fontSize: 12 },
    },
    color: [C.value.lvs, C.value.nginx, C.value.k8s, C.value.preprod],
    grid: { left: 60, right: 80, top: 20, bottom: showZoom ? 60 : 40 },
    xAxis: {
      type: 'category',
      data: periods,
      boundaryGap: false,
      axisLine: { lineStyle: { color: themeColors.value.border } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: themeColors.value.border, type: 'dashed' } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11 },
    },
    dataZoom: showZoom
      ? [{ type: 'inside' }, { type: 'slider', height: 20, bottom: 4, borderColor: 'transparent' }]
      : undefined,
    animationDuration: 800,
    animationEasing: 'cubicOut',
    series,
  }
})

// ---- 柱状图：登录统计 ----
const loginBarOption = computed(() => {
  if (loginChartData.value.length === 0) return {}
  const periods = [...new Set(loginChartData.value.map((d) => d.period))].sort()
  const successMap = {}
  const failedMap = {}
  loginChartData.value.forEach((d) => {
    if (d.status === 'success') successMap[d.period] = d.count
    else if (d.status === 'failed') failedMap[d.period] = d.count
  })
  const showZoom = periods.length > 15
  return {
    tooltip: tooltipConf({ trigger: 'axis', axisPointer: { type: 'shadow' } }),
    legend: {
      data: ['成功', '失败'],
      right: 0,
      top: 'center',
      orient: 'vertical',
      textStyle: { color: themeColors.value.subText, fontSize: 12 },
    },
    color: [C.value.success, C.value.failed],
    grid: { left: 60, right: 80, top: 20, bottom: showZoom ? 60 : 40 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLine: { lineStyle: { color: themeColors.value.border } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: themeColors.value.border, type: 'dashed' } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11 },
    },
    dataZoom: showZoom
      ? [{ type: 'inside' }, { type: 'slider', height: 20, bottom: 4, borderColor: 'transparent' }]
      : undefined,
    series: [
      {
        name: '成功',
        type: 'bar',
        stack: 'login',
        barWidth: '50%',
        data: periods.map((p) => successMap[p] || 0),
        itemStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#4ade80' },
              { offset: 1, color: '#22C55E' },
            ],
          },
        },
      },
      {
        name: '失败',
        type: 'bar',
        stack: 'login',
        data: periods.map((p) => failedMap[p] || 0),
        itemStyle: {
          borderRadius: [4, 4, 0, 0],
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#f87171' },
              { offset: 1, color: '#EF4444' },
            ],
          },
        },
      },
    ],
  }
})

// ---- K8S 项目发布统计 ----
const k8sProjectGranularity = ref('day')
const k8sServerFilter = ref('')
const k8sServers = shallowRef([])
const k8sProjectSummary = ref({ total: 0, success: 0, failed: 0, full_ops: 0 })
const k8sProjectTrend = shallowRef([])
const k8sProjectByProject = shallowRef([])
const k8sProjectByAction = shallowRef([])

const K8S_ACTION_NAMES = {
  online: '上线',
  sync: '同步',
  rollback: '回滚',
  full_online: '完全上线',
  full_sync: '完全同步',
  full_rollback: '完全回滚',
}

const k8sTrendOption = computed(() => {
  if (!k8sProjectTrend.value.length) return {}
  const periods = [...new Set(k8sProjectTrend.value.map((t) => t.period))].sort()
  const projects = [...new Set(k8sProjectTrend.value.map((t) => t.project))]
  const series = projects.map((proj) => {
    const map = {}
    k8sProjectTrend.value
      .filter((t) => t.project === proj)
      .forEach((t) => {
        map[t.period] = t.count
      })
    return {
      name: proj,
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 6,
      data: periods.map((p) => map[p] || 0),
      lineStyle: { width: 2 },
      areaStyle: { opacity: 0.08 },
    }
  })
  const showZoom = periods.length > 15
  return {
    tooltip: tooltipConf({ trigger: 'axis' }),
    legend: {
      type: 'scroll',
      bottom: showZoom ? 24 : 0,
      textStyle: { color: themeColors.value.subText, fontSize: 11 },
    },
    grid: { top: 10, right: 16, bottom: showZoom ? 56 : 40, left: 50 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: themeColors.value.subText, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: { type: 'value', minInterval: 1, axisLabel: { color: themeColors.value.subText, fontSize: 11 } },
    color: ['#F59E0B', '#06B6D4', '#22C55E', '#EF4444', '#8B5CF6', '#EC4899', '#14B8A6', '#F97316'],
    dataZoom: showZoom
      ? [{ type: 'inside' }, { type: 'slider', height: 16, bottom: 4, borderColor: 'transparent' }]
      : undefined,
    series,
  }
})

const k8sActionPieOption = computed(() => {
  if (!k8sProjectByAction.value.length) return {}
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: themeColors.value.subText, fontSize: 11 } },
    color: ['#22C55E', '#06B6D4', '#EF4444', '#F59E0B', '#8B5CF6', '#EC4899'],
    series: [
      {
        type: 'pie',
        radius: ['40%', '65%'],
        center: ['50%', '45%'],
        label: { show: true, formatter: '{b}\n{d}%', color: themeColors.value.subText, fontSize: 11 },
        data: k8sProjectByAction.value.map((a) => ({ name: K8S_ACTION_NAMES[a.action] || a.action, value: a.count })),
      },
    ],
  }
})

const k8sProjectRanking = computed(() => [...k8sProjectByProject.value].sort((a, b) => b.count - a.count).slice(0, 10))
function k8sRankingBarWidth(count) {
  return Math.round((count / (k8sProjectRanking.value[0]?.count || 1)) * 100) + '%'
}

// ---- 预生产扩缩容统计 ----
const preprodProjectGranularity = ref('day')
const preprodServerFilter = ref('')
const preprodServers = shallowRef([])
const preprodProjectSummary = ref({ total: 0, success: 0, failed: 0, full_ops: 0 })
const preprodProjectTrend = shallowRef([])
const preprodProjectByProject = shallowRef([])
const preprodProjectByAction = shallowRef([])

const PREPROD_ACTION_NAMES = {
  scaledown: '缩容',
  scaleup: '扩容',
  batch_scaledown: '批量缩容',
  batch_scaleup: '批量扩容',
  full_scaledown: '全量缩容',
  full_scaleup: '全量扩容',
}

const preprodTrendOption = computed(() => {
  if (!preprodProjectTrend.value.length) return {}
  const periods = [...new Set(preprodProjectTrend.value.map((t) => t.period))].sort()
  const projects = [...new Set(preprodProjectTrend.value.map((t) => t.project))]
  const series = projects.map((proj) => {
    const map = {}
    preprodProjectTrend.value
      .filter((t) => t.project === proj)
      .forEach((t) => {
        map[t.period] = t.count
      })
    return {
      name: proj,
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 6,
      data: periods.map((p) => map[p] || 0),
      lineStyle: { width: 2 },
      areaStyle: { opacity: 0.08 },
    }
  })
  const showZoom = periods.length > 15
  return {
    tooltip: tooltipConf({ trigger: 'axis' }),
    legend: {
      type: 'scroll',
      bottom: showZoom ? 24 : 0,
      textStyle: { color: themeColors.value.subText, fontSize: 11 },
    },
    grid: { top: 10, right: 16, bottom: showZoom ? 56 : 40, left: 50 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: themeColors.value.subText, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: { type: 'value', minInterval: 1, axisLabel: { color: themeColors.value.subText, fontSize: 11 } },
    color: ['#EF4444', '#06B6D4', '#22C55E', '#F59E0B', '#8B5CF6', '#EC4899', '#14B8A6', '#F97316'],
    dataZoom: showZoom
      ? [{ type: 'inside' }, { type: 'slider', height: 16, bottom: 4, borderColor: 'transparent' }]
      : undefined,
    series,
  }
})

const preprodActionPieOption = computed(() => {
  if (!preprodProjectByAction.value.length) return {}
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: themeColors.value.subText, fontSize: 11 } },
    color: ['#EF4444', '#22C55E', '#06B6D4', '#F59E0B', '#8B5CF6', '#EC4899'],
    series: [
      {
        type: 'pie',
        radius: ['40%', '65%'],
        center: ['50%', '45%'],
        label: { show: true, formatter: '{b}\n{d}%', color: themeColors.value.subText, fontSize: 11 },
        data: preprodProjectByAction.value.map((a) => ({
          name: PREPROD_ACTION_NAMES[a.action] || a.action,
          value: a.count,
        })),
      },
    ],
  }
})

const preprodProjectRanking = computed(() =>
  [...preprodProjectByProject.value].sort((a, b) => b.count - a.count).slice(0, 10)
)
function preprodRankingBarWidth(count) {
  return Math.round((count / (preprodProjectRanking.value[0]?.count || 1)) * 100) + '%'
}

// ---- 数据加载 ----
async function loadStats() {
  statsLoading.value = true
  try {
    const res = await getDashboardStats()
    serverStats.value = res.servers || null
    userStats.value = res.users || null
    onlineUsers.value = res.online_users || 0
  } catch {
    ElMessage.error('加载 MySQL 统计失败')
  } finally {
    statsLoading.value = false
  }
}

async function loadRemoteStats() {
  remoteLoading.value = true
  remoteError.value = null
  try {
    const res = await getDashboardRemoteStats()
    lvsStats.value = res.lvs || null
    nginxStats.value = res.nginx || null
    k8sStats.value = res.k8s || null
    preprodStats.value = res.preprod || null
  } catch {
    ElMessage.error('加载远程统计失败')
    remoteError.value = '远程数据加载失败'
  } finally {
    remoteLoading.value = false
  }
}

async function loadActivityStats() {
  try {
    const res = await getActivityStats({ granularity: deployGranularity.value })
    deployChartData.value = res.deploy_stats || []
  } catch {}
}

async function loadLoginStats() {
  try {
    const res = await getActivityStats({ granularity: loginGranularity.value })
    loginChartData.value = res.login_stats || []
    if (loginGranularity.value === 'day') {
      const today = new Date().toISOString().slice(0, 10)
      const todayLogins = (res.login_stats || []).filter((d) => d.period === today)
      todayLoginSuccess.value = todayLogins.find((d) => d.status === 'success')?.count || 0
      todayLoginFailed.value = todayLogins.find((d) => d.status === 'failed')?.count || 0
    }
  } catch {}
}

async function loadActionStats() {
  try {
    const res = await getActivityStats({
      granularity: deployGranularity.value,
      action_granularity: actionGranularity.value,
    })
    actionStatsData.value = res.action_stats || []
  } catch {}
}

async function loadK8sProjectStats() {
  try {
    const params = { granularity: k8sProjectGranularity.value }
    if (k8sServerFilter.value) params.server_name = k8sServerFilter.value
    const res = await getK8sProjectStats(params)
    k8sProjectSummary.value = res.summary || { total: 0, success: 0, failed: 0, full_ops: 0 }
    k8sProjectTrend.value = res.trend || []
    k8sProjectByProject.value = res.by_project || []
    k8sProjectByAction.value = res.by_action || []
  } catch {}
}

async function loadPreprodProjectStats() {
  try {
    const params = { granularity: preprodProjectGranularity.value }
    if (preprodServerFilter.value) params.server_name = preprodServerFilter.value
    const res = await getPreprodProjectStats(params)
    preprodProjectSummary.value = res.summary || { total: 0, success: 0, failed: 0, full_ops: 0 }
    preprodProjectTrend.value = res.trend || []
    preprodProjectByProject.value = res.by_project || []
    preprodProjectByAction.value = res.by_action || []
  } catch {}
}

async function loadK8sServers() {
  try {
    const res = await getServers('kubernetes')
    k8sServers.value = Array.isArray(res) ? res : []
  } catch {}
}
async function loadPreprodServers() {
  try {
    const res = await getServers('preprod')
    preprodServers.value = Array.isArray(res) ? res : []
  } catch {}
}

watch(k8sServerFilter, () => loadK8sProjectStats())
watch(preprodServerFilter, () => loadPreprodProjectStats())

function refreshAll() {
  loadStats()
  loadRemoteStats()
  loadActivityStats()
  loadLoginStats()
  loadActionStats()
  loadK8sProjectStats()
  loadPreprodProjectStats()
  lastUpdated.value = new Date()
}

onMounted(async () => {
  // 第一优先级：核心统计卡片和远程状态
  loadStats()
  loadRemoteStats()
  loadLoginStats()
  // 第二优先级：图表数据（延迟一帧，让首屏先渲染）
  await nextTick()
  loadActivityStats()
  loadActionStats()
  // 第三优先级：项目统计和服务器列表（更延迟）
  setTimeout(() => {
    loadK8sServers()
    loadPreprodServers()
    loadK8sProjectStats()
    loadPreprodProjectStats()
  }, 200)
  lastUpdated.value = new Date()
})

onActivated(() => {
  // 5 分钟内不重复刷新，避免频繁切换页面时的冗余请求
  if (!lastUpdated.value || Date.now() - lastUpdated.value.getTime() > 5 * 60 * 1000) {
    loadStats()
    loadRemoteStats()
    loadLoginStats()
    lastUpdated.value = new Date()
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}
.dash-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.dash-toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dash-last-updated {
  font-size: 12px;
  color: var(--text-secondary);
}
.dash-toolbar-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.dash-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}
.dash-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.dash-row {
  display: grid;
  gap: 16px;
  margin-bottom: 16px;
}
.misc-row {
  grid-template-columns: 1fr;
}
.misc-row-admin {
  grid-template-columns: 2fr 1fr 1fr;
}
.trend-action-row {
  grid-template-columns: 1fr 1fr;
}

.chart-card {
  min-width: 0;
}
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}
.card-header-controls {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.chart-title {
  font-weight: 600;
  font-size: 14px;
}
.chart {
  height: 220px;
}
.chart-wrap {
  position: relative;
}
.chart-loading,
.chart-error {
  height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.chart-empty {
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-placeholder);
  font-size: 13px;
}

.action-tabs {
  border-radius: 8px;
}
.action-chart {
  height: 280px;
}
.action-empty {
  height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-placeholder);
  font-size: 14px;
}
.trend-chart {
  height: 320px;
}

.empty-state {
  height: 260px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
.empty-state-icon {
  font-size: 48px;
  color: var(--el-text-color-placeholder);
}
.empty-state-text {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.metric-row {
  margin-bottom: 14px;
}
.metric {
  background: var(--el-fill-color-light);
  border-radius: 8px;
  padding: 10px 8px;
  text-align: center;
}
.metric-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 2px;
}
.metric-value {
  font-size: 22px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}
.sub-label {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--el-text-color-regular);
}
.ranking-section {
  margin-top: 14px;
}
.ranking-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}
.ranking-name {
  width: 120px;
  font-size: 12px;
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--el-text-color-regular);
}
.ranking-bar-bg {
  flex: 1;
  height: 14px;
  background: var(--el-fill-color-light);
  border-radius: 3px;
  overflow: hidden;
}
.ranking-bar {
  height: 100%;
  background: linear-gradient(90deg, #f59e0b, #fbbf24);
  border-radius: 3px;
  transition: width 0.4s ease;
}
.ranking-bar.preprod {
  background: linear-gradient(90deg, #ef4444, #f87171);
}
.ranking-count {
  width: 36px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
  font-variant-numeric: tabular-nums;
  text-align: right;
}

@media (max-width: 1200px) {
  .misc-row-admin {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 768px) {
  .misc-row-admin {
    grid-template-columns: 1fr;
  }
  .trend-action-row {
    grid-template-columns: 1fr;
  }
  .trend-chart {
    height: 260px;
  }
  .action-chart {
    height: 240px;
  }
  .ranking-name {
    width: 80px;
  }
}
</style>
