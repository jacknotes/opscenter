<template>
  <el-card class="chart-card" shadow="hover">
    <template #header>
      <div class="card-header">
        <span class="chart-title">LVS 连接统计</span>
        <div class="card-header-controls">
          <el-select v-model="serverId" placeholder="选择 LVS 服务器" clearable size="small" style="width: 160px" @change="onServerChange">
            <el-option v-for="s in lvsServers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-select v-model="vsIp" placeholder="选择 VS" clearable size="small" style="width: 160px" :disabled="!serverId" @change="onVsChange">
            <el-option v-for="ip in vsList" :key="ip" :label="ip" :value="ip" />
          </el-select>
          <el-select v-model="rsIp" placeholder="选择 RS" clearable size="small" style="width: 160px" :disabled="!vsIp" @change="onRsChange">
            <el-option v-for="ip in rsList" :key="ip" :label="ip" :value="ip" />
          </el-select>
          <el-radio-group v-model="duration" size="small" @change="onDurationChange">
            <el-radio-button :value="5">5分钟</el-radio-button>
            <el-radio-button :value="15">15分钟</el-radio-button>
            <el-radio-button :value="30">30分钟</el-radio-button>
            <el-radio-button :value="60">60分钟</el-radio-button>
          </el-radio-group>
          <el-button text type="primary" size="small" :loading="loading" @click="loadData">
            <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
          </el-button>
        </div>
      </div>
    </template>
    <v-chart v-if="chartData.length > 0" class="conn-chart" :option="chartOption" autoresize />
    <div v-else class="empty-state">
      <el-icon class="empty-state-icon"><DataLine /></el-icon>
      <span class="empty-state-text">{{ emptyText }}</span>
    </div>
  </el-card>
</template>

<script setup>
import { ref, computed, onMounted, onActivated, onDeactivated, onUnmounted } from 'vue'
import { getLvsList } from '../../api/lvs'
import { getLvsConnStats } from '../../api/dashboard'
import { useServerSelector } from '../../composables/useServerSelector'
import { STORAGE_KEYS } from '../../utils/constants'
import { DataLine, Refresh } from '@element-plus/icons-vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { TitleComponent, TooltipComponent, LegendComponent, GridComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([LineChart, TitleComponent, TooltipComponent, LegendComponent, GridComponent, CanvasRenderer])

// ---- localStorage 读写 ----
function saveKey(key, val) {
  try {
    if (val === null || val === undefined || val === '') {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, String(val))
    }
  } catch {}
}

// ---- 主题感知 ----
function getCSSVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const themeColors = computed(() => ({
  text: getCSSVar('--text-primary') || '#E2E8F0',
  subText: getCSSVar('--text-regular') || '#94A3B8',
  muted: getCSSVar('--text-secondary') || '#64748B',
  border: getCSSVar('--border-default') || 'rgba(255,255,255,0.06)',
  tooltipBg: getCSSVar('--bg-elevated') || '#1A1D2E',
  tooltipBorder: getCSSVar('--border-default') || 'rgba(255,255,255,0.1)',
}))

// ---- 筛选状态 ----
const { servers: lvsServers, serverId, initServers, refreshServers } = useServerSelector('lvs', STORAGE_KEYS.LVS_CONN_SERVER)
const vsIp = ref(localStorage.getItem(STORAGE_KEYS.LVS_CONN_VS) || null)
const rsIp = ref(localStorage.getItem(STORAGE_KEYS.LVS_CONN_RS) || null)
const duration = ref(Number(localStorage.getItem(STORAGE_KEYS.LVS_CONN_DURATION)) || 15)
const vsList = ref([])
const rsList = ref([])

// 从 LVS list 数据中缓存的 VS/RS 映射
let vsRsMap = {} // { vsIp: [rsIp1, rsIp2, ...] }

// ---- 图表数据 ----
const loading = ref(false)
const chartData = ref([])

const emptyText = computed(() => {
  if (!serverId.value) return '请选择 LVS 服务器'
  if (!vsIp.value) return '请选择 Virtual Server'
  if (!rsIp.value) return '请选择 Real Server'
  return '暂无连接数据（等待首次采集）'
})

// ---- 服务器变更：加载 VS 列表 ----
async function onServerChange() {
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
function onVsChange() {
  saveKey(STORAGE_KEYS.LVS_CONN_VS, vsIp.value)
  rsIp.value = null
  rsList.value = vsRsMap[vsIp.value] || []
  chartData.value = []
  saveKey(STORAGE_KEYS.LVS_CONN_RS, null)
}

// ---- RS 变更 ----
function onRsChange() {
  saveKey(STORAGE_KEYS.LVS_CONN_RS, rsIp.value)
  loadData()
}

// ---- 时间范围变更 ----
function onDurationChange() {
  saveKey(STORAGE_KEYS.LVS_CONN_DURATION, duration.value)
  loadData()
}

// ---- 加载 VS/RS 列表（供恢复时调用） ----
async function loadVsRsList() {
  try {
    const data = await getLvsList(serverId.value)
    const vsSet = new Set()
    const map = {}
    if (Array.isArray(data)) {
      for (const vs of data) {
        vsSet.add(vs.ip)
        if (!map[vs.ip]) map[vs.ip] = new Set()
        for (const rs of vs.real_servers || []) {
          map[vs.ip].add(rs.ip)
        }
      }
    }
    vsList.value = [...vsSet].sort()
    vsRsMap = {}
    for (const [ip, rsSet] of Object.entries(map)) {
      vsRsMap[ip] = [...rsSet].sort()
    }
  } catch {}
}

// ---- 加载连接统计数据 ----
async function loadData() {
  chartData.value = []
  if (!serverId.value || !vsIp.value || !rsIp.value) return

  loading.value = true
  try {
    const res = await getLvsConnStats({
      server_id: serverId.value,
      vs_ip: vsIp.value,
      rs_ip: rsIp.value,
      duration: duration.value,
    })
    chartData.value = res?.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

// ---- 图表配置 ----
const chartOption = computed(() => {
  if (chartData.value.length === 0) return {}

  const times = chartData.value.map((d) => {
    const t = new Date(d.collected_at)
    return t.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  })
  const activeData = chartData.value.map((d) => d.active_conn)
  const inactData = chartData.value.map((d) => d.inact_conn)

  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: themeColors.value.tooltipBg,
      borderColor: themeColors.value.tooltipBorder,
      textStyle: { color: themeColors.value.text, fontSize: 13 },
    },
    legend: {
      data: ['ActiveConn', 'InActConn'],
      right: 0,
      top: 0,
      textStyle: { color: themeColors.value.subText, fontSize: 12 },
    },
    color: ['#06B6D4', '#F59E0B'],
    grid: { left: 60, right: 20, top: 40, bottom: 30 },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: false,
      axisLine: { lineStyle: { color: themeColors.value.border } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: themeColors.value.border, type: 'dashed' } },
      axisLabel: { color: themeColors.value.muted, fontSize: 11 },
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
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: '#06B6D4' },
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
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: '#F59E0B' },
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

// ---- 自动刷新 ----
let autoRefreshTimer = null

function startAutoRefresh() {
  stopAutoRefresh()
  autoRefreshTimer = setInterval(() => {
    if (serverId.value && vsIp.value && rsIp.value) {
      loadData()
    }
  }, 30000)
}

function stopAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

// ---- 初始化：恢复保存的选择并级联加载 ----
async function init() {
  // 保存 localStorage 中的值，以便 initServers 完成后恢复
  const savedVsIp = localStorage.getItem(STORAGE_KEYS.LVS_CONN_VS)
  const savedRsIp = localStorage.getItem(STORAGE_KEYS.LVS_CONN_RS)

  await initServers()

  // 如果保存了 serverId，恢复 VS/RS 列表并自动加载数据
  if (serverId.value) {
    await loadVsRsList()

    // 恢复 VS
    if (savedVsIp && vsList.value.includes(savedVsIp)) {
      vsIp.value = savedVsIp
      rsList.value = vsRsMap[savedVsIp] || []

      // 恢复 RS
      if (savedRsIp && rsList.value.includes(savedRsIp)) {
        rsIp.value = savedRsIp
        loadData()
      }
    }
  }
}

onMounted(async () => {
  await init()
  startAutoRefresh()
})

onActivated(async () => {
  await refreshServers()
  startAutoRefresh()
})

onDeactivated(() => {
  stopAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
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
.conn-chart {
  height: 300px;
}
.empty-state {
  height: 300px;
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
</style>
