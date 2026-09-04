<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import type { ServerResponse } from '@/api/types'
import { dashboardApi, lvsApi, serverApi } from '@/api'
import { STORAGE_KEYS } from '@/utils/constants'
import { useTheme } from '@/composables/useTheme'
import BaseChart from '@/components/BaseChart.vue'

const props = defineProps<{ fullscreen: boolean }>()

const emit = defineEmits<{ toggleFullscreen: [] }>()

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

// ---- localStorage 读写 ----
function saveKey(key: string, val: string | number | null | undefined): void {
  try {
    if (val === null || val === undefined || val === '') {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, String(val))
    }
  } catch {
    /* 忽略存储异常 */
  }
}

// ---- 筛选状态 ----
const lvsServers = ref<ServerResponse[]>([])
const serverId = ref<number | undefined>(
  Number(localStorage.getItem(STORAGE_KEYS.LVS_CONN_SERVER)) || undefined,
)
const vsIp = ref<string | null>(localStorage.getItem(STORAGE_KEYS.LVS_CONN_VS))
const rsIp = ref<string | null>(localStorage.getItem(STORAGE_KEYS.LVS_CONN_RS))
const duration = ref<number>(Number(localStorage.getItem(STORAGE_KEYS.LVS_CONN_DURATION)) || 15)
const vsList = ref<string[]>([])
const rsList = ref<string[]>([])

/** 从 LVS list 数据中缓存的 VS -> RS 映射 */
let vsRsMap: Record<string, string[]> = {}

// ---- 图表数据 ----
const loading = ref(false)
interface LvsConnPoint {
  collected_at: string
  active_conn: number
  inact_conn: number
}
const chartData = ref<LvsConnPoint[]>([])

const emptyText = computed(() => {
  if (!serverId.value) return '请选择 LVS 服务器'
  if (!vsIp.value) return '请选择 Virtual Server'
  if (!rsIp.value) return '请选择 Real Server'
  return '暂无连接数据（等待首次采集）'
})

// ---- 服务器变更：加载 VS 列表 ----
async function onServerChange(): Promise<void> {
  saveKey(STORAGE_KEYS.LVS_CONN_SERVER, serverId.value)
  vsIp.value = null
  rsIp.value = null
  vsList.value = []
  rsList.value = []
  vsRsMap = {}
  chartData.value = []
  saveKey(STORAGE_KEYS.LVS_CONN_VS, null)
  saveKey(STORAGE_KEYS.LVS_CONN_RS, null)

  if (!serverId.value) return
  await loadVsRsList()
}

// ---- VS 变更：加载 RS 列表 ----
function onVsChange(): void {
  saveKey(STORAGE_KEYS.LVS_CONN_VS, vsIp.value)
  rsIp.value = null
  rsList.value = vsIp.value ? (vsRsMap[vsIp.value] ?? []) : []
  chartData.value = []
  saveKey(STORAGE_KEYS.LVS_CONN_RS, null)
}

// ---- RS 变更 ----
function onRsChange(): void {
  saveKey(STORAGE_KEYS.LVS_CONN_RS, rsIp.value)
  void loadData()
}

// ---- 时间范围变更 ----
function onDurationChange(): void {
  saveKey(STORAGE_KEYS.LVS_CONN_DURATION, duration.value)
  void loadData()
}

// ---- 加载 VS/RS 列表 ----
async function loadVsRsList(): Promise<void> {
  if (!serverId.value) return
  try {
    const data = await lvsApi.list(serverId.value)
    const vsSet = new Set<string>()
    const map: Record<string, Set<string>> = {}
    for (const vs of data) {
      vsSet.add(vs.ip)
      if (!map[vs.ip]) map[vs.ip] = new Set()
      for (const rs of vs.real_servers ?? []) map[vs.ip].add(rs.ip)
    }
    vsList.value = [...vsSet].sort()
    vsRsMap = {}
    for (const [ip, rsSet] of Object.entries(map)) vsRsMap[ip] = [...rsSet].sort()
  } catch {
    /* 静默失败，界面展示空状态 */
  }
}

// ---- 加载连接统计数据 ----
async function loadData(): Promise<void> {
  chartData.value = []
  if (!serverId.value || !vsIp.value || !rsIp.value) return

  loading.value = true
  try {
    const res = await dashboardApi.lvsConnStats({
      server_id: serverId.value,
      vs_ip: vsIp.value,
      rs_ip: rsIp.value,
      duration: duration.value,
    })
    chartData.value = res?.data ?? []
  } catch {
    /* 静默失败，界面展示空状态 */
  } finally {
    loading.value = false
  }
}

// ---- 图表配置 ----
const chartOption = computed<EChartsCoreOption>(() => {
  if (chartData.value.length === 0) return {}

  const times = chartData.value.map((d) => {
    const t = new Date(d.collected_at)
    return t.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const activeData = chartData.value.map((d) => d.active_conn)
  const inactData = chartData.value.map((d) => d.inact_conn)
  const p = palette.value

  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: p.tooltipBg,
      borderColor: p.subText,
      textStyle: { color: p.text, fontSize: 13 },
      confine: true,
    },
    legend: {
      data: ['ActiveConn', 'InActConn'],
      right: 0,
      top: 0,
      textStyle: { color: p.subText, fontSize: 12 },
    },
    color: ['#38bdf8', '#fbbf24'],
    grid: { left: 60, right: 20, top: 40, bottom: 30 },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: { lineStyle: { color: p.border } },
      axisLabel: { color: p.muted, fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: p.split } },
      axisLabel: { color: p.muted, fontSize: 11 },
    },
    series: [
      {
        name: 'ActiveConn',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { width: 2 },
        areaStyle: {
          opacity: 0.15,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#38bdf8' },
              { offset: 1, color: 'transparent' },
            ],
          },
        },
        data: activeData,
      },
      {
        name: 'InActConn',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { width: 2 },
        areaStyle: {
          opacity: 0.15,
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#fbbf24' },
              { offset: 1, color: 'transparent' },
            ],
          },
        },
        data: inactData,
      },
    ],
    animationDuration: 600,
    animationEasing: 'cubicOut',
  }
})

// ---- 自动刷新（30s 轮询） ----
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

function startAutoRefresh(): void {
  stopAutoRefresh()
  autoRefreshTimer = setInterval(() => {
    if (serverId.value && vsIp.value && rsIp.value) void loadData()
  }, 30000)
}

function stopAutoRefresh(): void {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

// ---- 初始化：恢复保存的选择并级联加载 ----
async function init(): Promise<void> {
  try {
    lvsServers.value = await serverApi.list({ type: 'lvs' })
  } catch {
    lvsServers.value = []
  }
  // 当前选中服务器不在列表中时清空
  if (serverId.value !== undefined && !lvsServers.value.some((s) => s.id === serverId.value)) {
    serverId.value = undefined
    saveKey(STORAGE_KEYS.LVS_CONN_SERVER, null)
    return
  }

  const savedVsIp = localStorage.getItem(STORAGE_KEYS.LVS_CONN_VS)
  const savedRsIp = localStorage.getItem(STORAGE_KEYS.LVS_CONN_RS)

  if (!serverId.value) return
  await loadVsRsList()

  if (savedVsIp && vsList.value.includes(savedVsIp)) {
    vsIp.value = savedVsIp
    rsList.value = vsRsMap[savedVsIp] ?? []

    if (savedRsIp && rsList.value.includes(savedRsIp)) {
      rsIp.value = savedRsIp
      void loadData()
    }
  }
}

onMounted(async () => {
  await init()
  startAutoRefresh()
})

onBeforeUnmount(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div
    class="card chart-card reveal"
    :class="{ 'fullscreen-card': props.fullscreen }"
    :style="props.fullscreen ? { flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' } : {}"
  >
    <div class="chart-head">
      <h3 class="chart-title">LVS 连接统计</h3>
      <div class="head-controls">
        <el-select
          v-model="serverId"
          placeholder="选择 LVS 服务器"
          clearable
          filterable
          size="small"
          style="width: 160px"
          @change="onServerChange"
        >
          <el-option v-for="s in lvsServers" :key="s.id" :label="s.name" :value="s.id" />
        </el-select>
        <el-select
          v-model="vsIp"
          placeholder="选择 VS"
          clearable
          filterable
          size="small"
          style="width: 160px"
          :disabled="!serverId"
          @change="onVsChange"
        >
          <el-option v-for="ip in vsList" :key="ip" :label="ip" :value="ip" />
        </el-select>
        <el-select
          v-model="rsIp"
          placeholder="选择 RS"
          clearable
          filterable
          size="small"
          style="width: 160px"
          :disabled="!vsIp"
          @change="onRsChange"
        >
          <el-option v-for="ip in rsList" :key="ip" :label="ip" :value="ip" />
        </el-select>
        <el-radio-group v-model="duration" size="small" @change="onDurationChange">
          <el-radio-button :value="5">5分钟</el-radio-button>
          <el-radio-button :value="15">15分钟</el-radio-button>
          <el-radio-button :value="30">30分钟</el-radio-button>
          <el-radio-button :value="60">60分钟</el-radio-button>
        </el-radio-group>
        <el-button text type="primary" size="small" :loading="loading" @click="loadData">刷新</el-button>
        <el-button text type="primary" size="small" @click="emit('toggleFullscreen')">
          {{ props.fullscreen ? '退出全屏' : '全屏' }}
        </el-button>
      </div>
    </div>

    <BaseChart
      v-if="chartOption && Object.keys(chartOption).length > 0"
      :option="chartOption"
      height="240px"
      :loading="loading"
      class="conn-chart"
      :class="{ 'fullscreen-chart': props.fullscreen }"
    />
    <div v-else class="empty-state" v-loading="loading">
      <span class="empty-text">{{ emptyText }}</span>
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

.conn-chart {
  flex: 1;
  min-height: 0;
}

.fullscreen-chart {
  height: auto !important;
}

.empty-state {
  height: 240px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-text {
  color: var(--text-muted);
  font-size: var(--text-sm);
}
</style>
