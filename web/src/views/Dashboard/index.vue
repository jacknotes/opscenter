<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { extractErrorMessage } from '@/api/client'
import { dashboardApi } from '@/api'
import type {
  ActivityStats,
  DashboardStats,
  Granularity,
  ProjectStat,
  ProjectStatsResponse,
  RemoteStats,
} from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { i18n } from '@/i18n'
import StatCard from '@/components/StatCard.vue'
import BaseChart from '@/components/BaseChart.vue'
import type { EChartsCoreOption } from 'echarts/core'

const t = i18n.global.t
const auth = useAuthStore()
const { theme } = useTheme()

const loading = ref(false)
const stats = ref<DashboardStats | null>(null)
const remote = ref<RemoteStats | null>(null)
const activity = ref<ActivityStats | null>(null)
const k8sProjects = ref<ProjectStatsResponse | null>(null)
const granularity = ref<Granularity>('day')

const MODULE_COLORS: Record<string, string> = {
  lvs: '#818cf8',
  nginx: '#a78bfa',
  k8s: '#38bdf8',
  preprod: '#34d399',
}

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const tasks = [
      dashboardApi.stats().then((d) => (stats.value = d)),
      dashboardApi.remoteStats().then((d) => (remote.value = d)),
      dashboardApi.activityStats({ granularity: granularity.value }).then((d) => (activity.value = d)),
      dashboardApi.k8sProjectStats({ granularity: granularity.value }).then((d) => (k8sProjects.value = d)),
    ]
    await Promise.allSettled(tasks)
  } catch (err) {
    // 各接口已独立兜底，这里仅防御
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
watch(granularity, () => {
  void loadAll()
})

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
  return {
    text: theme.value === 'dark' ? '#94a3b8' : '#475569',
    split: theme.value === 'dark' ? 'rgba(148,163,184,0.12)' : 'rgba(71,85,105,0.12)',
  }
})

const MODULE_LABEL: Record<string, string> = {
  lvs: 'LVS',
  nginx: 'Nginx',
  k8s: 'K8S',
  preprod: t('nav.preprod'),
}

// 操作趋势：按周期堆叠柱状图
const trendOption = computed<EChartsCoreOption>(() => {
  const deploys = activity.value?.deploy_stats ?? []
  const periods = [...new Set(deploys.map((d) => d.period))].sort()
  const modules = [...new Set(deploys.map((d) => d.module))]
  return {
    color: modules.map((m) => MODULE_COLORS[m] ?? '#818cf8'),
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { data: modules.map((m) => MODULE_LABEL[m] ?? m), textStyle: { color: chartPalette.value.text } },
    grid: { left: 48, right: 16, top: 40, bottom: 28 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: chartPalette.value.text },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: chartPalette.value.text },
      splitLine: { lineStyle: { color: chartPalette.value.split } },
      minInterval: 1,
    },
    series: modules.map((m) => ({
      name: MODULE_LABEL[m] ?? m,
      type: 'bar',
      stack: 'total',
      data: periods.map((p) => deploys.filter((d) => d.period === p && d.module === m).reduce((s, d) => s + d.count, 0)),
      barMaxWidth: 26,
    })),
  }
})

// 登录统计（仅 admin）：成功/失败双线
const loginOption = computed<EChartsCoreOption>(() => {
  const logins = activity.value?.login_stats ?? []
  const periods = [...new Set(logins.map((d) => d.period))].sort()
  const sum = (status: string) =>
    periods.map((p) => logins.filter((d) => d.period === p && d.status === status).reduce((s, d) => s + d.count, 0))
  return {
    color: ['#34d399', '#f87171'],
    tooltip: { trigger: 'axis' },
    legend: {
      data: [t('common.success'), t('common.failed')],
      textStyle: { color: chartPalette.value.text },
    },
    grid: { left: 48, right: 16, top: 40, bottom: 28 },
    xAxis: { type: 'category', data: periods, axisLabel: { color: chartPalette.value.text } },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: chartPalette.value.text },
      splitLine: { lineStyle: { color: chartPalette.value.split } },
    },
    series: [
      { name: t('common.success'), type: 'line', smooth: true, data: sum('success') },
      { name: t('common.failed'), type: 'line', smooth: true, data: sum('failed') },
    ],
  }
})

// 模块动作分布：环形图
const actionPieOption = computed<EChartsCoreOption>(() => {
  const actions = activity.value?.action_stats ?? []
  const agg = new Map<string, number>()
  for (const a of actions) agg.set(a.module, (agg.get(a.module) ?? 0) + a.count)
  const data = [...agg.entries()].map(([k, v]) => ({ name: MODULE_LABEL[k] ?? k, value: v }))
  return {
    color: Object.values(MODULE_COLORS),
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: chartPalette.value.text } },
    series: [
      {
        type: 'pie',
        radius: ['46%', '72%'],
        center: ['50%', '44%'],
        label: { color: chartPalette.value.text },
        data,
        emphasis: { itemStyle: { shadowBlur: 18, shadowColor: 'rgba(99,102,241,0.4)' } },
      },
    ],
  }
})

// K8S 项目 Top10：横向条形
const topProjects = computed<ProjectStat[]>(() =>
  [...(k8sProjects.value?.by_project ?? [])].sort((a, b) => b.count - a.count).slice(0, 10).reverse(),
)
const projectBarOption = computed<EChartsCoreOption>(() => {
  const rows = topProjects.value
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 130, right: 32, top: 12, bottom: 28 },
    xAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: chartPalette.value.text },
      splitLine: { lineStyle: { color: chartPalette.value.split } },
    },
    yAxis: {
      type: 'category',
      data: rows.map((r) => r.project),
      axisLabel: { color: chartPalette.value.text, width: 120, overflow: 'truncate' },
    },
    series: [
      {
        name: t('common.success'),
        type: 'bar',
        stack: 's',
        itemStyle: { color: '#34d399' },
        data: rows.map((r) => r.success),
        barMaxWidth: 18,
      },
      {
        name: t('common.failed'),
        type: 'bar',
        stack: 's',
        itemStyle: { color: '#f87171' },
        data: rows.map((r) => r.failed),
        barMaxWidth: 18,
      },
    ],
  }
})

const granularities: { value: Granularity; label: string }[] = [
  { value: 'day', label: t('dashboard.granularity.day') },
  { value: 'week', label: t('dashboard.granularity.week') },
  { value: 'month', label: t('dashboard.granularity.month') },
  { value: 'year', label: t('dashboard.granularity.year') },
]
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('dashboard.title') }}</h1>
        <p class="page-subtitle">{{ t('dashboard.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-radio-group v-model="granularity" size="small">
          <el-radio-button v-for="g in granularities" :key="g.value" :value="g.value">
            {{ g.label }}
          </el-radio-button>
        </el-radio-group>
        <el-button size="small" :loading="loading" @click="loadAll">{{ t('common.refresh') }}</el-button>
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
    <div class="stat-grid">
      <StatCard :label="t('dashboard.servers')" :value="fmt(srv?.total)" :hint="`${t('common.enabled')} ${fmt(srv?.enabled)} · ${t('common.disabled')} ${fmt(srv?.disabled)}`" :delay="0" color="var(--indigo-400)" />
      <StatCard v-if="auth.isAdmin" :label="t('dashboard.users')" :value="fmt(usr?.total)" :hint="`${t('common.enabled')} ${fmt(usr?.enabled)}`" :delay="1" color="var(--violet-400)" />
      <StatCard v-if="auth.isAdmin" :label="t('dashboard.onlineUsers')" :value="fmt(stats?.online_users)" :delay="2" color="var(--emerald-400)" live />
      <StatCard :label="t('dashboard.lvsVs')" :value="fmt(lvsR?.vs_count)" :hint="`${t('dashboard.lvsRsOnline')} ${fmt(lvsR?.rs_online)} / ${t('dashboard.lvsRsOffline')} ${fmt(lvsR?.rs_offline)}`" :delay="3" color="var(--sky-400)" />
      <StatCard :label="t('dashboard.nginxUpstreams')" :value="fmt(ngxR?.upstream_count)" :hint="`${t('dashboard.nginxOnline')} ${fmt(ngxR?.server_online)} / ${t('dashboard.nginxOffline')} ${fmt(ngxR?.server_offline)}`" :delay="4" color="var(--amber-400)" />
      <StatCard :label="t('dashboard.k8sRollouts')" :value="fmt(k8sR?.total_rollouts)" :hint="`${t('dashboard.k8sPending')} ${fmt(k8sR?.pending)} / ${t('dashboard.k8sOnline')} ${fmt(k8sR?.online)}`" :delay="5" color="var(--indigo-600)" />
      <StatCard :label="t('dashboard.preprodResources')" :value="fmt(preR?.total_resources)" :hint="`${t('dashboard.preprodScaled')} ${fmt(preR?.scaled_down)} / ${t('dashboard.preprodNormal')} ${fmt(preR?.normal)}`" :delay="6" color="var(--emerald-400)" />
    </div>

    <!-- 图表区 -->
    <div class="charts-row">
      <div class="card chart-card reveal d-1">
        <h3 class="chart-title">{{ t('dashboard.deployTrend') }}</h3>
        <BaseChart :option="trendOption" height="300px" :loading="loading" />
      </div>
      <div v-if="auth.isAdmin" class="card chart-card reveal d-2">
        <h3 class="chart-title">{{ t('dashboard.loginStats') }}</h3>
        <BaseChart :option="loginOption" height="300px" :loading="loading" />
      </div>
      <div class="card chart-card reveal d-3">
        <h3 class="chart-title">{{ t('dashboard.moduleDist') }}</h3>
        <BaseChart :option="actionPieOption" height="300px" :loading="loading" />
      </div>
    </div>

    <div class="charts-row">
      <div class="card chart-card wide reveal d-4">
        <h3 class="chart-title">{{ t('dashboard.k8sProject') }}</h3>
        <BaseChart :option="projectBarOption" height="320px" :loading="loading" />
      </div>
    </div>
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
  margin-bottom: var(--space-5);
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

.charts-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

@media (max-width: 1200px) {
  .charts-row {
    grid-template-columns: 1fr;
  }
}

.chart-card {
  padding: var(--space-4) var(--space-5);
  min-width: 0;
}

.chart-card.wide {
  grid-column: 1 / -1;
}

.chart-title {
  margin: 0 0 var(--space-3);
  font-family: var(--font-display);
  font-size: var(--text-md);
  color: var(--text-primary);
  font-weight: 600;
}
</style>
