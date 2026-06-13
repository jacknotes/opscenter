<template>
  <div class="dashboard" :class="{ 'has-fullscreen': fullscreenChart }">
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
    <StatCards v-show="!fullscreenChart" :online-users="onlineUsers" :login-success="todayLoginSuccess" :login-failed="todayLoginFailed" :is-admin="userStore.isAdmin" style="margin-top: 20px" @show-online-users="showOnlineUsersDialog" />

    <!-- 模块实时状态（4 个饼图） -->
    <ModulePies
      v-show="!fullscreenChart"
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
    <div v-show="!fullscreenChart || fullscreenChart === 'loginStats'" class="dash-row" :class="[userStore.isAdmin ? 'misc-row-admin' : 'misc-row', { 'fullscreen-active-row': fullscreenChart === 'loginStats' }]">
      <el-card class="chart-card" :class="{ 'fullscreen-card': fullscreenChart === 'loginStats' }" shadow="hover" :style="getFullscreenCardStyle('loginStats')">
        <template #header>
          <div class="card-header">
            <span class="chart-title">登录统计趋势</span>
            <div class="card-header-controls">
              <DateRangeSelector v-model="activityDateRange" />
              <el-button text type="primary" size="small" @click="loadAllActivityStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
              <el-button text type="primary" size="small" @click="toggleFullscreen('loginStats')">
                <el-icon><component :is="fullscreenChart === 'loginStats' ? ScaleToOriginal : FullScreen" /></el-icon>
              </el-button>
            </div>
          </div>
        </template>
        <v-chart v-if="loginChartData.length > 0" class="trend-chart" :class="{ 'fullscreen-chart': fullscreenChart === 'loginStats' }" :option="loginBarOption" autoresize />
        <div v-else class="empty-state">
          <el-icon class="empty-state-icon"><DataLine /></el-icon>
          <span class="empty-state-text">暂无登录数据</span>
        </div>
      </el-card>
      <template v-if="userStore.isAdmin && !fullscreenChart">
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
    <div v-show="!fullscreenChart || fullscreenChart === 'lvsConn'" class="dash-row" :class="{ 'fullscreen-active-row': fullscreenChart === 'lvsConn' }">
      <LvsConnChart :is-fullscreen="fullscreenChart === 'lvsConn'" :fullscreen="fullscreenChart === 'lvsConn'" @toggle-fullscreen="toggleFullscreen('lvsConn')" />
    </div>

    <!-- 发布趋势 + 操作动作明细 -->
    <div v-show="!fullscreenChart || fullscreenChart === 'deployTrend' || fullscreenChart === 'actionDetail'" class="dash-row trend-action-row" :class="{ 'fullscreen-active-row': fullscreenChart === 'deployTrend' || fullscreenChart === 'actionDetail' }">
      <el-card
        v-show="!fullscreenChart || fullscreenChart === 'deployTrend'"
        class="chart-card"
        :class="{ 'fullscreen-card': fullscreenChart === 'deployTrend' }"
        shadow="hover"
        :style="getFullscreenCardStyle('deployTrend')"
      >
        <template #header>
          <div class="card-header">
            <span class="chart-title">各模块发布次数趋势</span>
            <div class="card-header-controls">
              <DateRangeSelector v-model="activityDateRange" />
              <el-button text type="primary" size="small" @click="loadAllActivityStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
              <el-button text type="primary" size="small" @click="toggleFullscreen('deployTrend')">
                <el-icon><component :is="fullscreenChart === 'deployTrend' ? ScaleToOriginal : FullScreen" /></el-icon>
              </el-button>
            </div>
          </div>
        </template>
        <v-chart
          v-if="deployChartData.length > 0"
          class="trend-chart"
          :class="{ 'fullscreen-chart': fullscreenChart === 'deployTrend' }"
          :option="deployLineOption"
          autoresize
        />
        <div v-else class="empty-state">
          <el-icon class="empty-state-icon"><DataLine /></el-icon>
          <span class="empty-state-text">暂无发布数据</span>
        </div>
      </el-card>
      <el-card
        v-show="!fullscreenChart || fullscreenChart === 'actionDetail'"
        class="chart-card"
        :class="{ 'fullscreen-card': fullscreenChart === 'actionDetail' }"
        shadow="hover"
        :style="getFullscreenCardStyle('actionDetail')"
      >
        <template #header>
          <div class="card-header">
            <span class="chart-title">各模块操作动作明细</span>
            <div class="card-header-controls">
              <DateRangeSelector v-model="activityDateRange" />
              <el-button text type="primary" size="small" @click="loadAllActivityStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
              <el-button text type="primary" size="small" @click="toggleFullscreen('actionDetail')">
                <el-icon><component :is="fullscreenChart === 'actionDetail' ? ScaleToOriginal : FullScreen" /></el-icon>
              </el-button>
            </div>
          </div>
        </template>
        <el-tabs v-model="activeActionTab" type="border-card" class="action-tabs">
          <el-tab-pane v-for="mod in actionModules" :key="mod.key" :label="mod.label" :name="mod.key">
            <v-chart v-if="Object.keys(mod.option).length > 0" class="action-chart" :class="{ 'fullscreen-chart': fullscreenChart === 'actionDetail' }" :option="mod.option" autoresize />
            <div v-else class="action-empty">暂无{{ mod.label }}操作记录</div>
          </el-tab-pane>
        </el-tabs>
      </el-card>
    </div>

    <!-- K8S 项目发布统计 -->
    <div v-show="!fullscreenChart || fullscreenChart === 'k8sProject'" class="dash-row" :class="{ 'fullscreen-active-row': fullscreenChart === 'k8sProject' }">
      <el-card class="chart-card" :class="{ 'fullscreen-card': fullscreenChart === 'k8sProject' }" shadow="hover" :style="getFullscreenCardStyle('k8sProject')">
        <template #header>
          <div class="card-header">
            <span class="chart-title">K8S 项目发布统计</span>
            <div class="card-header-controls">
              <el-select v-model="k8sServerFilter" placeholder="全部服务器" clearable size="small" style="width: 150px">
                <el-option label="全部服务器" value="" />
                <el-option v-for="s in k8sServers" :key="s.id" :label="s.name" :value="s.name" />
              </el-select>
              <DateRangeSelector v-model="k8sDateRange" />
              <el-button text type="primary" size="small" @click="loadK8sProjectStats">
                <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
              </el-button>
              <el-button text type="primary" size="small" @click="toggleFullscreen('k8sProject')">
                <el-icon><component :is="fullscreenChart === 'k8sProject' ? ScaleToOriginal : FullScreen" /></el-icon>
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
              class="project-trend-chart"
              :class="{ 'fullscreen-chart': fullscreenChart === 'k8sProject' }"
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
              :class="{ 'fullscreen-chart': fullscreenChart === 'k8sProject' }"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
        </el-row>
        <div v-if="k8sSortedProjects.length > 0" class="project-list-section">
          <div class="sub-label">项目明细</div>
          <div class="project-list-header">
            <span class="project-list-col-name">项目名称</span>
            <span class="project-list-col-count sortable" @click="k8sProjectSortAsc = !k8sProjectSortAsc">
              发布次数
              <el-icon class="sort-icon"><component :is="k8sProjectSortAsc ? SortUp : SortDown" /></el-icon>
            </span>
          </div>
          <div v-for="item in k8sPagedProjects" :key="item.project" class="project-list-item">
            <span class="project-list-col-name" :title="item.project">{{ item.project }}</span>
            <span class="project-list-col-count">{{ item.count }}</span>
          </div>
          <div v-if="k8sProjectTotalPages > 1" class="project-list-pagination">
            <el-pagination
              v-model:current-page="k8sProjectPage"
              :page-size="k8sProjectPageSize"
              :total="k8sSortedProjects.length"
              layout="prev, pager, next"
              small
            />
          </div>
        </div>
      </el-card>
    </div>

    <!-- 预生产扩缩容统计 -->
    <div v-show="!fullscreenChart || fullscreenChart === 'preprodProject'" class="dash-row" :class="{ 'fullscreen-active-row': fullscreenChart === 'preprodProject' }">
      <el-card class="chart-card" :class="{ 'fullscreen-card': fullscreenChart === 'preprodProject' }" shadow="hover" :style="getFullscreenCardStyle('preprodProject')">
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
              <el-date-picker
                v-model="preprodDateRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                size="small"
                value-format="YYYY-MM-DD"
                :shortcuts="dateShortcuts"
                style="width: 260px"
              />
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
              <el-button text type="primary" size="small" @click="toggleFullscreen('preprodProject')">
                <el-icon><component :is="fullscreenChart === 'preprodProject' ? ScaleToOriginal : FullScreen" /></el-icon>
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
              class="project-trend-chart"
              :class="{ 'fullscreen-chart': fullscreenChart === 'preprodProject' }"
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
              :class="{ 'fullscreen-chart': fullscreenChart === 'preprodProject' }"
            />
            <el-empty v-else description="暂无数据" :image-size="48" />
          </el-col>
        </el-row>
        <div v-if="preprodSortedProjects.length > 0" class="project-list-section">
          <div class="sub-label">项目明细</div>
          <div class="project-list-header">
            <span class="project-list-col-name">项目名称</span>
            <span class="project-list-col-count sortable" @click="preprodProjectSortAsc = !preprodProjectSortAsc">
              操作次数
              <el-icon class="sort-icon"><component :is="preprodProjectSortAsc ? SortUp : SortDown" /></el-icon>
            </span>
          </div>
          <div v-for="item in preprodPagedProjects" :key="item.project" class="project-list-item">
            <span class="project-list-col-name" :title="item.project">{{ item.project }}</span>
            <span class="project-list-col-count">{{ item.count }}</span>
          </div>
          <div v-if="preprodProjectTotalPages > 1" class="project-list-pagination">
            <el-pagination
              v-model:current-page="preprodProjectPage"
              :page-size="preprodProjectPageSize"
              :total="preprodSortedProjects.length"
              layout="prev, pager, next"
              small
            />
          </div>
        </div>
      </el-card>
    </div>

    <!-- 在线用户列表对话框 -->
    <el-dialog
      v-model="onlineUsersDialogVisible"
      title="在线用户列表"
      width="600px"
      :close-on-click-modal="true"
    >
      <el-table
        :data="onlineUsersList"
        v-loading="onlineUsersLoading"
        stripe
        style="width: 100%"
      >
        <el-table-column prop="username" label="用户名" min-width="100" />
        <el-table-column prop="role" label="角色" width="80">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
              {{ row.role === 'admin' ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="login_method" label="登录方式" width="100">
          <template #default="{ row }">
            <el-tag :type="row.login_method === 'ldap' ? 'warning' : 'success'" size="small" effect="plain">
              {{ row.login_method === 'ldap' ? 'LDAP' : '本地' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="login_time" label="登录时间" min-width="160">
          <template #default="{ row }">
            {{ row.login_time ? formatTime(new Date(row.login_time)) : '-' }}
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="loadOnlineUsers" :loading="onlineUsersLoading">
          <el-icon style="margin-right: 4px"><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button @click="onlineUsersDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
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
  getOnlineUsers,
} from '../../api'
import { useUserStore } from '../../stores/user'
import { useAppStore } from '../../stores/app'
import { ElMessage } from 'element-plus'
import { DataLine, Refresh, FullScreen, ScaleToOriginal, SortUp, SortDown } from '@element-plus/icons-vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { PieChart, LineChart, BarChart } from 'echarts/charts'
import { TitleComponent, TooltipComponent, LegendComponent, GridComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { formatTimeShort, formatTime } from '../../utils/format'
import StatCards from './StatCards.vue'
import ModulePies from './ModulePies.vue'
import LvsConnChart from './LvsConnChart.vue'
import DateRangeSelector from '../../components/DateRangeSelector.vue'

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
const _cssVarCache = ref(null)
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
  if (!_cssVarCache.value) _cssVarCache.value = readThemeVars()
  return _cssVarCache.value
}
watch(() => appStore.theme, () => { _cssVarCache.value = null })

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

// ---- 全屏状态管理 ----
const fullscreenChart = ref(null) // 记录当前全屏的图表名称

function toggleFullscreen(chartName) {
  if (fullscreenChart.value === chartName) {
    fullscreenChart.value = null
  } else {
    fullscreenChart.value = chartName
  }
}

function getFullscreenCardStyle(chartName) {
  if (fullscreenChart.value === chartName) {
    return { flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' }
  }
  return {}
}

const MODULE_NAMES = { lvs: 'LVS', nginx: 'Nginx', k8s: 'K8S', preprod: '预生产' }

// ---- 日期工具 ----
function formatDate(date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

const dateShortcuts = [
  { text: '最近 7 天', value: () => { const e = new Date(); const s = new Date(); s.setDate(s.getDate() - 7); return [s, e] } },
  { text: '最近 30 天', value: () => { const e = new Date(); const s = new Date(); s.setDate(s.getDate() - 30); return [s, e] } },
  { text: '最近 90 天', value: () => { const e = new Date(); const s = new Date(); s.setDate(s.getDate() - 90); return [s, e] } },
]

// ---- 数据 ----
// 在线用户列表
const onlineUsersDialogVisible = ref(false)
const onlineUsersLoading = ref(false)
const onlineUsersList = ref([])

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
const activityDateRange = ref(null)
const deployChartData = shallowRef([])
const loginChartData = shallowRef([])
const actionStatsData = shallowRef([])
const activeActionTab = ref('lvs')

// ---- tooltip 配置 ----
function tooltipConf(extra = {}) {
  return {
    backgroundColor: themeColors.value.tooltipBg,
    borderColor: themeColors.value.tooltipBorder,
    textStyle: { color: themeColors.value.text, fontSize: 13 },
    confine: true,
    ...extra,
  }
}

// 过滤 tooltip 中值为 0 的项，保留折线但不显示多余的 0 值信息
function filterZeroAxisTooltip(params) {
  const list = Array.isArray(params) ? params.filter((p) => p.value !== 0) : []
  if (list.length === 0) return ''
  let html = list[0].axisValue + '<br/>'
  list.forEach((p) => {
    html += p.marker + ' ' + p.seriesName + ': ' + p.value + '<br/>'
  })
  return html
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
      splitLine: { lineStyle: { color: themeColors.value.border } },
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
      axisPointer: { type: 'cross', lineStyle: { color: themeColors.value.muted } },
      formatter: filterZeroAxisTooltip,
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
      splitLine: { lineStyle: { color: themeColors.value.border } },
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
    tooltip: tooltipConf({ trigger: 'axis', axisPointer: { type: 'shadow' }, formatter: filterZeroAxisTooltip }),
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
      splitLine: { lineStyle: { color: themeColors.value.border } },
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
const k8sServerFilter = ref('')
const k8sDateRange = ref(null)
const k8sServers = shallowRef([])
const k8sProjectSummary = ref({ total: 0, success: 0, failed: 0, full_ops: 0 })
const k8sProjectTrend = shallowRef([])
const k8sProjectByProject = shallowRef([])
const k8sProjectByAction = shallowRef([])

const k8sProjectPage = ref(1)
const k8sProjectPageSize = 15
const k8sProjectSortAsc = ref(false)

const k8sSortedProjects = computed(() =>
  [...k8sProjectByProject.value].sort((a, b) => k8sProjectSortAsc.value ? a.count - b.count : b.count - a.count)
)

const k8sPagedProjects = computed(() => {
  const start = (k8sProjectPage.value - 1) * k8sProjectPageSize
  return k8sSortedProjects.value.slice(start, start + k8sProjectPageSize)
})

const k8sProjectTotalPages = computed(() =>
  Math.ceil(k8sSortedProjects.value.length / k8sProjectPageSize)
)

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
  // 计算每个项目的总次数，取 top 5
  const projectCounts = {}
  k8sProjectTrend.value.forEach((t) => {
    projectCounts[t.project] = (projectCounts[t.project] || 0) + t.count
  })
  const projects = Object.entries(projectCounts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([name]) => name)
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
    tooltip: tooltipConf({ trigger: 'axis', formatter: filterZeroAxisTooltip }),
    grid: { top: 10, right: 16, bottom: showZoom ? 40 : 20, left: 50 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: themeColors.value.subText, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: themeColors.value.border } }, axisLabel: { color: themeColors.value.muted, fontSize: 11 } },
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

// ---- 预生产扩缩容统计 ----
const preprodProjectGranularity = ref('day')
const preprodServerFilter = ref('')
const preprodDateRange = ref(null)
const preprodServers = shallowRef([])
const preprodProjectSummary = ref({ total: 0, success: 0, failed: 0, full_ops: 0 })
const preprodProjectTrend = shallowRef([])
const preprodProjectByProject = shallowRef([])
const preprodProjectByAction = shallowRef([])

const preprodProjectPage = ref(1)
const preprodProjectPageSize = 15
const preprodProjectSortAsc = ref(false)

const preprodSortedProjects = computed(() =>
  [...preprodProjectByProject.value].sort((a, b) => preprodProjectSortAsc.value ? a.count - b.count : b.count - a.count)
)

const preprodPagedProjects = computed(() => {
  const start = (preprodProjectPage.value - 1) * preprodProjectPageSize
  return preprodSortedProjects.value.slice(start, start + preprodProjectPageSize)
})

const preprodProjectTotalPages = computed(() =>
  Math.ceil(preprodSortedProjects.value.length / preprodProjectPageSize)
)

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
  // 计算每个项目的总次数，取 top 5
  const projectCounts = {}
  preprodProjectTrend.value.forEach((t) => {
    projectCounts[t.project] = (projectCounts[t.project] || 0) + t.count
  })
  const projects = Object.entries(projectCounts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([name]) => name)
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
    tooltip: tooltipConf({ trigger: 'axis', formatter: filterZeroAxisTooltip }),
    grid: { top: 10, right: 16, bottom: showZoom ? 40 : 20, left: 50 },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: { color: themeColors.value.subText, fontSize: 11, rotate: showZoom ? 30 : 0 },
    },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: themeColors.value.border } }, axisLabel: { color: themeColors.value.muted, fontSize: 11 } },
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

async function loadOnlineUsers() {
  onlineUsersLoading.value = true
  try {
    const res = await getOnlineUsers()
    onlineUsersList.value = res.users || []
  } catch {
    ElMessage.error('加载在线用户列表失败')
  } finally {
    onlineUsersLoading.value = false
  }
}

function showOnlineUsersDialog() {
  onlineUsersDialogVisible.value = true
  loadOnlineUsers()
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

async function loadPreprodProjectStats() {
  try {
    const params = { granularity: preprodProjectGranularity.value }
    if (preprodServerFilter.value) params.server_name = preprodServerFilter.value
    if (preprodDateRange.value && preprodDateRange.value.length === 2) {
      params.start_date = preprodDateRange.value[0]
      params.end_date = preprodDateRange.value[1]
    }
    const res = await getPreprodProjectStats(params)
    preprodProjectSummary.value = res.summary || { total: 0, success: 0, failed: 0, full_ops: 0 }
    preprodProjectTrend.value = res.trend || []
    preprodProjectByProject.value = res.by_project || []
    preprodProjectByAction.value = res.by_action || []
  } catch (err) {
    console.error('加载预生产项目统计失败:', err)
  }
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

watch(activityDateRange, () => { loadAllActivityStats() })
watch(k8sServerFilter, () => loadK8sProjectStats())
watch(preprodServerFilter, () => loadPreprodProjectStats())
watch(k8sDateRange, () => { k8sProjectPage.value = 1; loadK8sProjectStats() })
watch(preprodDateRange, () => { preprodProjectPage.value = 1; loadPreprodProjectStats() })

function refreshAll() {
  loadStats()
  loadRemoteStats()
  loadAllActivityStats()
  loadK8sProjectStats()
  loadPreprodProjectStats()
  lastUpdated.value = new Date()
}

onMounted(async () => {
  // 第一优先级：核心统计卡片和远程状态
  loadStats()
  loadRemoteStats()
  // 第二优先级：图表数据（延迟一帧，让首屏先渲染）
  await nextTick()
  loadAllActivityStats()
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
    loadAllActivityStats()
    lastUpdated.value = new Date()
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
  display: flex;
  flex-direction: column;
}
.dash-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: sticky;
  top: -20px;
  z-index: 50;
  background: var(--content-bg);
  padding: 12px 0;
  margin-bottom: 20px;
  margin-left: -20px;
  margin-right: -20px;
  padding-left: 20px;
  padding-right: 20px;
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
  display: flex;
  gap: 16px;
}
.trend-action-row > .chart-card {
  flex: 1;
  min-width: 0;
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
  height: 220px;
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
  height: 220px;
}
.project-trend-chart {
  height: 220px;
  overflow: hidden;
}

.empty-state {
  height: 220px;
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

/* 全屏模式 */
.fullscreen-active-row {
  flex: 1 !important;
  min-height: 0;
}
/* 登录统计行：全屏卡片跨所有 grid 列 */
.misc-row-admin .fullscreen-card,
.misc-row .fullscreen-card {
  grid-column: 1 / -1;
  height: 100%;
}
.fullscreen-card :deep(.el-card__body) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.fullscreen-chart {
  flex: 1 !important;
  min-height: 0 !important;
  height: auto !important;
}
/* K8S/预生产：图表行撑满（不含 metric-row） */
.fullscreen-card :deep(.el-row:has(.sub-label)) {
  flex: 1;
  min-height: 0;
}
.fullscreen-card :deep(.el-row:has(.sub-label) .el-col) {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
/* 操作明细 tabs 全屏撑满 */
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
.ranking-count {
  width: 36px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.project-list-section {
  margin-top: 14px;
}
.project-list-header {
  display: flex;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
}
.project-list-item {
  display: flex;
  align-items: center;
  padding: 5px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-size: 12px;
  color: var(--el-text-color-regular);
}
.project-list-item:last-child {
  border-bottom: none;
}
.project-list-col-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: 12px;
}
.project-list-col-count {
  width: 80px;
  text-align: right;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.project-list-col-count.sortable {
  cursor: pointer;
  user-select: none;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
}
.project-list-col-count.sortable:hover {
  color: var(--el-color-primary);
}
.sort-icon {
  font-size: 12px;
}
.project-list-pagination {
  display: flex;
  justify-content: center;
  margin-top: 8px;
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
    height: 220px;
  }
  .action-chart {
    height: 220px;
  }
  .ranking-name {
    width: 80px;
  }
}
</style>
