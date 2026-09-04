<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { extractErrorMessage } from '@/api/client'
import { dashboardApi, serverApi } from '@/api'
import type {
  ActivityStats,
  DashboardStats,
  ProjectStatsResponse,
  RemoteStats,
  ServerResponse,
} from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { useDashboardFullscreen } from '@/composables/useDashboardFullscreen'
import { i18n } from '@/i18n'
import { MODULE_LABELS } from '@/utils/constants'
import StatCard from '@/components/StatCard.vue'
import BaseChart from '@/components/BaseChart.vue'
import DateRangeSelector from '@/components/DateRangeSelector.vue'
import ModulePies from './ModulePies.vue'
import ProjectStatsCard from './ProjectStatsCard.vue'
import LvsConnChart from './LvsConnChart.vue'
import OnlineUsersDialog from './OnlineUsersDialog.vue'
import type { EChartsCoreOption } from 'echarts/core'

const t = i18n.global.t
const auth = useAuthStore()
const { theme } = useTheme()
const { fullscreenChart, toggleFullscreen, getFullscreenCardStyle } = useDashboardFullscreen()

const loading = ref(false)
const k8sLoading = ref(false)
const preprodLoading = ref(false)
const stats = ref<DashboardStats | null>(null)
const remote = ref<RemoteStats | null>(null)
const activity = ref<ActivityStats | null>(null)
const k8sProjects = ref<ProjectStatsResponse | null>(null)
const preprodProjects = ref<ProjectStatsResponse | null>(null)

// ---- 共享日期范围：驱动全部统计图表（activity / k8s / preprod） ----
const dateRange = ref<string[] | null>(null)

const k8sServerName = ref('')
const preprodServerName = ref('')
const k8sServers = ref<ServerResponse[]>([])
const preprodServers = ref<ServerResponse[]>([])

const MODULE_COLORS: Record<string, string> = {
  lvs: '#818cf8',
  nginx: '#a78bfa',
  k8s: '#38bdf8',
  preprod: '#34d399',
}

const MODULE_LABEL: Record<string, string> = {
  lvs: MODULE_LABELS.lvs,
  nginx: MODULE_LABELS.nginx,
  k8s: MODULE_LABELS.k8s,
  preprod: MODULE_LABELS.preprod,
}

// ---------- 数据加载 ----------
async function loadStats(): Promise<void> {
  try {
    stats.value = await dashboardApi.stats()
  } catch (err) {
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  }
}

async function loadRemote(): Promise<void> {
  try {
    remote.value = await dashboardApi.remoteStats()
  } catch (err) {
    remote.value = null
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  }
}

async function loadActivity(): Promise<void> {
  if (!dateRange.value || dateRange.value.length !== 2) return
  loading.value = true
  try {
    activity.value = await dashboardApi.activityStats({
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
    })
  } catch (err) {
    activity.value = null
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  } finally {
    loading.value = false
  }
}

async function loadK8sProjects(): Promise<void> {
  if (!dateRange.value || dateRange.value.length !== 2) return
  k8sLoading.value = true
  try {
    k8sProjects.value = await dashboardApi.k8sProjectStats({
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      ...(k8sServerName.value ? { server_name: k8sServerName.value } : {}),
    })
  } catch (err) {
    k8sProjects.value = null
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  } finally {
    k8sLoading.value = false
  }
}

async function loadPreprodProjects(): Promise<void> {
  if (!dateRange.value || dateRange.value.length !== 2) return
  preprodLoading.value = true
  try {
    preprodProjects.value = await dashboardApi.preprodProjectStats({
      start_date: dateRange.value[0],
      end_date: dateRange.value[1],
      ...(preprodServerName.value ? { server_name: preprodServerName.value } : {}),
    })
  } catch (err) {
    preprodProjects.value = null
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  } finally {
    preprodLoading.value = false
  }
}

async function loadServerOptions(): Promise<void> {
  for (const [type, target] of [
    ['kubernetes', k8sServers],
    ['preprod', preprodServers],
  ] as const) {
    try {
      target.value = await serverApi.list({ type })
    } catch {
      target.value = []
    }
  }
}

function refreshAll(): void {
  void loadStats()
  void loadRemote()
  void loadActivity()
  void loadK8sProjects()
  void loadPreprodProjects()
}

// DateRangeSelector onMounted 会 emit 初始范围，触发本 watch 完成首次加载
watch(dateRange, () => {
  void loadActivity()
  void loadK8sProjects()
  void loadPreprodProjects()
})

watch(k8sServerName, () => void loadK8sProjects())
watch(preprodServerName, () => void loadPreprodProjects())

onMounted(async () => {
  // 第一优先级：统计卡片与远程状态
  void loadStats()
  void loadRemote()
  // 第二优先级：图表数据（延迟一帧，让首屏先渲染）
  await nextTick()
  void loadServerOptions()
})

// ---------- 在线用户对话框 ----------
const onlineUsersVisible = ref(false)
function showOnlineUsers(): void {
  onlineUsersVisible.value = true
}

// ---------- 统计卡片数值 ----------
const nf = new Intl.NumberFormat('zh-CN')
const fmt = (v: number | undefined | null): string => (v === undefined || v === null ? '-' : nf.format(v))

const srv = computed(() => stats.value?.servers)
const usr = computed(() => stats.value?.users)
const lvsR = computed(() => remote.value?.lvs)
const ngxR = computed(() => remote.value?.nginx)
const k8sR = computed(() => remote.value?.k8s)
const preR = computed(() => remote.value?.preprod)
const remoteFailed = computed(
  () => remote.value !== null && Object.values(remote.value).some((v) => v === null),
)

// ---------- 图表通用配色（跟随主题令牌取值，主题切换时重算 option） ----------
const chartPalette = computed(() => {
  void theme.value
  const dark = theme.value === 'dark'
  return {
    text: dark ? '#94a3b8' : '#475569',
    muted: dark ? '#64748b' : '#64748b',
    split: dark ? 'rgba(148,163,184,0.12)' : 'rgba(71,85,105,0.12)',
    border: dark ? 'rgba(148,163,184,0.16)' : 'rgba(71,85,105,0.16)',
    tooltipBg: dark ? '#1e293b' : '#ffffff',
  }
})

// ---- tooltip 配置 ----
function tooltipConf(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    backgroundColor: chartPalette.value.tooltipBg,
    borderColor: chartPalette.value.text,
    textStyle: { color: chartPalette.value.text, fontSize: 13 },
    confine: true,
    ...extra,
  }
}

// 过滤 tooltip 中值为 0 的项，保留折线但不显示多余的 0 值信息
interface TooltipParam {
  axisValue?: string
  marker: string
  seriesName: string
  value: number
}

function filterZeroAxisTooltip(params: unknown): string {
  const list = (Array.isArray(params) ? params : []).filter(
    (p) => typeof p === 'object' && p !== null && (p as TooltipParam).value !== 0,
  ) as TooltipParam[]
  if (list.length === 0) return ''
  let html = (list[0].axisValue ?? '') + '<br/>'
  for (const p of list) html += p.marker + ' ' + p.seriesName + ': ' + p.value + '<br/>'
  return html
}

// ---------- 发布趋势（折线 + 面积，v1 样式） ----------
const deployTrendOption = computed<EChartsCoreOption>(() => {
  const deploys = activity.value?.deploy_stats ?? []
  if (deploys.length === 0) return {}
  const periods = [...new Set(deploys.map((d) => d.period))].sort()
  const modules = ['lvs', 'nginx', 'k8s', 'preprod']
  const series = modules.map((mod) => {
    const map = new Map<string, number>()
    for (const d of deploys) if (d.module === mod) map.set(d.period, d.count)
    const color = MODULE_COLORS[mod]
    return {
      name: MODULE_LABEL[mod] ?? mod,
      type: 'line' as const,
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
            { offset: 0, color },
            { offset: 1, color: 'transparent' },
          ],
        },
      },
      data: periods.map((p) => map.get(p) ?? 0),
    }
  })
  return {
    tooltip: tooltipConf({
      trigger: 'axis',
      axisPointer: { type: 'cross', lineStyle: { color: chartPalette.value.muted } },
      formatter: filterZeroAxisTooltip,
    }),
    legend: {
      data: modules.map((m) => MODULE_LABEL[m] ?? m),
      right: 0,
      top: 'center',
      orient: 'vertical',
      textStyle: { color: chartPalette.value.text, fontSize: 12 },
    },
    color: modules.map((m) => MODULE_COLORS[m]),
    grid: { left: 60, right: 80, top: 20, bottom: 40 },
    xAxis: {
      type: 'category',
      data: periods,
      boundaryGap: false,
      axisLine: { lineStyle: { color: chartPalette.value.border } },
      axisLabel: { color: chartPalette.value.muted, fontSize: 11, rotate: periods.length > 15 ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: chartPalette.value.split } },
      axisLabel: { color: chartPalette.value.muted, fontSize: 11 },
    },
    animationDuration: 800,
    animationEasing: 'cubicOut',
    series,
  }
})

// ---------- 登录统计（堆叠柱状，仅 admin） ----------
const loginBarOption = computed<EChartsCoreOption>(() => {
  const logins = activity.value?.login_stats ?? []
  if (logins.length === 0) return {}
  const periods = [...new Set(logins.map((d) => d.period))].sort()
  const sum = (status: string) =>
    periods.map((p) => logins.filter((d) => d.period === p && d.status === status).reduce((s, d) => s + d.count, 0))
  const p = chartPalette.value
  return {
    tooltip: tooltipConf({ trigger: 'axis', axisPointer: { type: 'shadow' }, formatter: filterZeroAxisTooltip }),
    legend: {
      data: [t('common.success'), t('common.failed')],
      right: 0,
      top: 'center',
      orient: 'vertical',
      textStyle: { color: p.text, fontSize: 12 },
    },
    color: ['#34d399', '#f87171'],
    grid: { left: 60, right: 80, top: 20, bottom: 40 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLine: { lineStyle: { color: p.border } },
      axisLabel: { color: p.muted, fontSize: 11, rotate: periods.length > 15 ? 30 : 0 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: p.split } },
      axisLabel: { color: p.muted, fontSize: 11 },
    },
    series: [
      {
        name: t('common.success'),
        type: 'bar' as const,
        stack: 'login',
        barWidth: '50%',
        data: sum('success'),
        itemStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#4ade80' },
              { offset: 1, color: '#34d399' },
            ],
          },
        },
      },
      {
        name: t('common.failed'),
        type: 'bar' as const,
        stack: 'login',
        data: sum('failed'),
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
              { offset: 1, color: '#ef4444' },
            ],
          },
        },
      },
    ],
  }
})

// ---------- 各模块操作动作明细（tabs + 横向条形，v1 样式） ----------
const ACTION_NAMES: Record<string, Record<string, string>> = {
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

const activeActionTab = ref('lvs')

function makeActionBarOption(module: string): EChartsCoreOption {
  const items = (activity.value?.action_stats ?? []).filter((d) => d.module === module)
  if (items.length === 0) return {}
  items.sort((a, b) => b.count - a.count)
  const names = items.map((d) => ACTION_NAMES[module]?.[d.action] ?? d.action)
  const values = items.map((d) => d.count)
  const color = MODULE_COLORS[module] ?? '#818cf8'
  const p = chartPalette.value
  return {
    tooltip: tooltipConf({ trigger: 'axis', axisPointer: { type: 'shadow' } }),
    grid: { left: 20, right: 50, top: 20, bottom: 30, containLabel: true },
    xAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: p.split } },
      axisLabel: { color: p.muted, fontSize: 12 },
    },
    yAxis: {
      type: 'category',
      data: names,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: p.text, fontSize: 13 },
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
        label: { show: true, position: 'right', color: p.text, fontSize: 14, fontWeight: 700 },
      },
    ],
  }
}

const actionModules = computed(() => [
  { key: 'lvs', label: MODULE_LABELS.lvs, option: makeActionBarOption('lvs') },
  { key: 'nginx', label: MODULE_LABELS.nginx, option: makeActionBarOption('nginx') },
  { key: 'k8s', label: MODULE_LABELS.k8s, option: makeActionBarOption('k8s') },
  { key: 'preprod', label: MODULE_LABELS.preprod, option: makeActionBarOption('preprod') },
])

// ---------- 项目统计默认值 ----------
const EMPTY_SUMMARY = { total: 0, success: 0, failed: 0, full_ops: 0 }
</script>

<template>
  <div class="page" :class="{ 'has-fullscreen': fullscreenChart }">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('dashboard.title') }}</h1>
        <p class="page-subtitle">{{ t('dashboard.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <DateRangeSelector v-model="dateRange" />
        <el-button size="small" :loading="loading || k8sLoading || preprodLoading" @click="refreshAll">
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="remoteFailed"
      type="warning"
      :title="t('dashboard.remoteError')"
      :closable="true"
      class="remote-alert"
    />

    <!-- 统计卡片 -->
    <div v-show="!fullscreenChart" class="stat-grid">
      <StatCard :label="t('dashboard.servers')" :value="fmt(srv?.total)" :hint="`${t('common.enabled')} ${fmt(srv?.enabled)} · ${t('common.disabled')} ${fmt(srv?.disabled)}`" :delay="0" color="var(--indigo-400)" />
      <StatCard v-if="auth.isAdmin" :label="t('dashboard.users')" :value="fmt(usr?.total)" :hint="`${t('common.enabled')} ${fmt(usr?.enabled)}`" :delay="1" color="var(--violet-400)" />
      <div v-if="auth.isAdmin" class="online-card" title="点击查看在线用户列表" @click="showOnlineUsers">
        <StatCard :label="t('dashboard.onlineUsers')" :value="fmt(stats?.online_users)" :delay="2" color="var(--emerald-400)" live />
      </div>
      <StatCard :label="t('dashboard.lvsVs')" :value="fmt(lvsR?.vs_count)" :hint="`${t('dashboard.lvsRsOnline')} ${fmt(lvsR?.rs_online)} / ${t('dashboard.lvsRsOffline')} ${fmt(lvsR?.rs_offline)}`" :delay="3" color="var(--sky-400)" />
      <StatCard :label="t('dashboard.nginxUpstreams')" :value="fmt(ngxR?.upstream_count)" :hint="`${t('dashboard.nginxOnline')} ${fmt(ngxR?.server_online)} / ${t('dashboard.nginxOffline')} ${fmt(ngxR?.server_offline)}`" :delay="4" color="var(--amber-400)" />
      <StatCard :label="t('dashboard.k8sRollouts')" :value="fmt(k8sR?.total_rollouts)" :hint="`${t('dashboard.k8sPending')} ${fmt(k8sR?.pending)} / ${t('dashboard.k8sOnline')} ${fmt(k8sR?.online)}`" :delay="5" color="var(--indigo-600)" />
      <StatCard :label="t('dashboard.preprodResources')" :value="fmt(preR?.total_resources)" :hint="`${t('dashboard.preprodScaled')} ${fmt(preR?.scaled_down)} / ${t('dashboard.preprodNormal')} ${fmt(preR?.normal)}`" :delay="6" color="var(--emerald-400)" />
    </div>

    <!-- 模块实时状态（4 个环形图） -->
    <ModulePies
      v-show="!fullscreenChart"
      :loading="false"
      :error="remoteFailed"
      :lvs="lvsR ?? null"
      :nginx="ngxR ?? null"
      :k8s="k8sR ?? null"
      :preprod="preR ?? null"
      @retry="loadRemote"
    />

    <!-- 发布趋势 + 登录统计 + 操作动作明细 -->
    <div class="charts-row-3">
      <div
        v-show="!fullscreenChart || fullscreenChart === 'deployTrend'"
        data-chart="deployTrend"
        class="card chart-card reveal d-1"
        :class="{ 'fullscreen-card': fullscreenChart === 'deployTrend' }"
        :style="getFullscreenCardStyle('deployTrend')"
      >
        <div class="chart-head">
          <h3 class="chart-title">各模块发布次数趋势</h3>
          <div class="head-controls">
            <DateRangeSelector v-model="dateRange" />
            <el-button text type="primary" size="small" :loading="loading" @click="loadActivity">刷新</el-button>
            <el-button text type="primary" size="small" @click="toggleFullscreen('deployTrend')">
              {{ fullscreenChart === 'deployTrend' ? '退出全屏' : '全屏' }}
            </el-button>
          </div>
        </div>
        <BaseChart
          v-if="Object.keys(deployTrendOption).length > 0"
          :option="deployTrendOption"
          height="300px"
          :loading="loading"
          class="fill-chart"
          :class="{ 'fullscreen-chart': fullscreenChart === 'deployTrend' }"
        />
        <div v-else class="chart-empty">暂无发布数据</div>
      </div>

      <div
        v-show="!fullscreenChart || fullscreenChart === 'loginStats'"
        data-chart="loginStats"
        class="card chart-card reveal d-2"
        :class="{ 'fullscreen-card': fullscreenChart === 'loginStats' }"
        :style="getFullscreenCardStyle('loginStats')"
      >
        <div class="chart-head">
          <h3 class="chart-title">{{ t('dashboard.loginStats') }}</h3>
          <div class="head-controls">
            <DateRangeSelector v-model="dateRange" />
            <el-button text type="primary" size="small" :loading="loading" @click="loadActivity">刷新</el-button>
            <el-button text type="primary" size="small" @click="toggleFullscreen('loginStats')">
              {{ fullscreenChart === 'loginStats' ? '退出全屏' : '全屏' }}
            </el-button>
          </div>
        </div>
        <BaseChart
          v-if="Object.keys(loginBarOption).length > 0"
          :option="loginBarOption"
          height="300px"
          :loading="loading"
          class="fill-chart"
          :class="{ 'fullscreen-chart': fullscreenChart === 'loginStats' }"
        />
        <div v-else class="chart-empty">暂无登录数据</div>
      </div>

      <div
        v-show="!fullscreenChart || fullscreenChart === 'actionDetail'"
        data-chart="actionDetail"
        class="card chart-card reveal d-3"
        :class="{ 'fullscreen-card': fullscreenChart === 'actionDetail' }"
        :style="getFullscreenCardStyle('actionDetail')"
      >
        <div class="chart-head">
          <h3 class="chart-title">各模块操作动作明细</h3>
          <div class="head-controls">
            <DateRangeSelector v-model="dateRange" />
            <el-button text type="primary" size="small" :loading="loading" @click="loadActivity">刷新</el-button>
            <el-button text type="primary" size="small" @click="toggleFullscreen('actionDetail')">
              {{ fullscreenChart === 'actionDetail' ? '退出全屏' : '全屏' }}
            </el-button>
          </div>
        </div>
        <el-tabs v-model="activeActionTab" type="border-card" class="action-tabs">
          <el-tab-pane v-for="mod in actionModules" :key="mod.key" :label="mod.label" :name="mod.key">
            <BaseChart
              v-if="Object.keys(mod.option).length > 0"
              :option="mod.option"
              height="260px"
              class="action-chart"
              :class="{ 'fullscreen-chart': fullscreenChart === 'actionDetail' }"
            />
            <div v-else class="action-empty">暂无{{ mod.label }}操作记录</div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <!-- LVS 连接统计 -->
    <div
      v-show="!fullscreenChart || fullscreenChart === 'lvsConn'"
      data-chart="lvsConn"
      class="wide-row"
      :class="{ 'fullscreen-active-row': fullscreenChart === 'lvsConn' }"
    >
      <LvsConnChart
        :fullscreen="fullscreenChart === 'lvsConn'"
        @toggle-fullscreen="toggleFullscreen('lvsConn')"
      />
    </div>

    <!-- K8S 项目发布统计 -->
    <div v-show="!fullscreenChart || fullscreenChart === 'k8sProject'" class="wide-row">
      <ProjectStatsCard
        chart-name="k8sProject"
        title="K8S 项目发布统计"
        count-label="发布次数"
        :servers="k8sServers"
        v-model:server-name="k8sServerName"
        :loading="k8sLoading"
        :summary="k8sProjects?.summary ?? EMPTY_SUMMARY"
        :trend="k8sProjects?.trend ?? []"
        :by-project="k8sProjects?.by_project ?? []"
        :by-action="k8sProjects?.by_action ?? []"
        :fullscreen="fullscreenChart === 'k8sProject'"
        @refresh="loadK8sProjects"
        @toggle-fullscreen="toggleFullscreen('k8sProject')"
      />
    </div>

    <!-- 预生产扩缩容统计 -->
    <div v-show="!fullscreenChart || fullscreenChart === 'preprodProject'" class="wide-row">
      <ProjectStatsCard
        chart-name="preprodProject"
        title="预生产扩缩容统计"
        count-label="操作次数"
        :servers="preprodServers"
        v-model:server-name="preprodServerName"
        :loading="preprodLoading"
        :summary="preprodProjects?.summary ?? EMPTY_SUMMARY"
        :trend="preprodProjects?.trend ?? []"
        :by-project="preprodProjects?.by_project ?? []"
        :by-action="preprodProjects?.by_action ?? []"
        :fullscreen="fullscreenChart === 'preprodProject'"
        @refresh="loadPreprodProjects"
        @toggle-fullscreen="toggleFullscreen('preprodProject')"
      />
    </div>

    <!-- 在线用户列表对话框 -->
    <OnlineUsersDialog v-model:visible="onlineUsersVisible" />
  </div>
</template>

<style scoped>
.remote-alert {
  margin-bottom: var(--space-5);
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 900px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 560px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }
}

.online-card {
  cursor: pointer;
}

.charts-row-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

@media (max-width: 1200px) {
  .charts-row-3 {
    grid-template-columns: 1fr;
  }
}

.wide-row {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

.wide-row.fullscreen-active-row {
  flex: 1;
  min-height: 0;
}

.chart-card {
  padding: var(--space-4) var(--space-5);
  min-width: 0;
  display: flex;
  flex-direction: column;
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

.fill-chart {
  flex: 1;
  min-height: 0;
}

.chart-empty {
  height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.action-tabs {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-sm);
}

.action-chart {
  flex: 1;
  min-height: 0;
}

.action-empty {
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

/* 全屏布局 */
.page.has-fullscreen {
  min-height: calc(100vh - var(--topbar-h) - var(--page-pad) * 2);
}

.fullscreen-card :deep(.action-tabs) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.fullscreen-card :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: visible;
}

.fullscreen-card :deep(.el-tab-pane) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.fullscreen-chart {
  height: auto !important;
}
</style>
