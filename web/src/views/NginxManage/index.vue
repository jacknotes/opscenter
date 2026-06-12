<template>
  <div class="nginx-page">
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择Nginx服务器" style="width: 150px" @change="loadConfigs">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span class="filter-label">文件:</span>
          <el-select v-model="configFile" placeholder="选择配置文件" style="width: 150px" @change="onConfigChange">
            <el-option v-for="f in configFiles" :key="f" :label="f" :value="f" />
          </el-select>
          <span style="margin-left: auto"></span>
          <el-input v-model="filterKeyword" placeholder="搜索upstream/ip/port" clearable style="width: 250px" />
        </div>

        <!-- Toolbar -->
        <div class="toolbar">
          <el-button type="info" class="el-button--cyan" @click="toggleExpandAll">{{
            allExpanded ? '折叠' : '展开'
          }}</el-button>
          <el-button type="info" class="el-button--cyan" @click="toggleSelectAll">{{
            isAllSelected ? '取消' : '全选'
          }}</el-button>
          <el-button type="primary" :disabled="selectedServers.length === 0" @click="handleBatchOnline">上线</el-button>
          <el-button type="danger" :disabled="selectedServers.length === 0" @click="handleBatchOffline">下线</el-button>
          <el-button type="primary" @click="openBatchDialog">批量</el-button>
          <el-dropdown trigger="click" style="margin-left: 12px">
            <el-button type="info" class="el-button--cyan"
              >更多<el-icon style="margin-left: 4px"><ArrowDown /></el-icon
            ></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="openBackupDialog">备份列表</el-dropdown-item>
                <el-dropdown-item @click="handleViewConfig">查看配置</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="info" class="el-button--cyan" @click="handleRefresh">刷新</el-button>
          <span style="margin-left: auto"></span>
          <span class="stat-chip stat-chip-warning"
            >Upstream <b>{{ filteredUpstreams.length }}</b></span
          >
          <span class="stat-chip stat-chip-success"
            >在线 <b>{{ totalUpCount }}</b></span
          >
          <span
            class="stat-chip stat-chip-danger"
            :class="{ 'stat-chip-active': statusFilter === 'down' }"
            @click="toggleStatusFilter('down')"
            >离线 <b>{{ totalDownCount }}</b></span
          >
          <span class="stat-chip stat-chip-primary"
            >已选 <b>{{ selectedServers.length }}</b></span
          >
        </div>
      </template>

      <!-- Upstream Groups -->
      <div v-loading="loadingUpstreams">
        <el-collapse v-model="expandedUpstreams" class="upstream-collapse">
          <el-collapse-item
            v-for="upstream in filteredUpstreams"
            :key="upstream.name"
            :name="upstream.name"
            :class="[
              'upstream-item',
              upstream.downCount === 0
                ? 'health-healthy'
                : upstream.upCount === 0
                  ? 'health-critical'
                  : 'health-degraded',
            ]"
          >
            <template #title>
              <div class="upstream-header">
                <span class="upstream-name">{{ upstream.name }}</span>
                <div class="upstream-badges">
                  <span class="badge badge-info">{{ upstream.servers.length }} 台</span>
                  <span class="badge badge-success">{{ upstream.upCount }} up</span>
                  <span class="badge badge-danger">{{ upstream.downCount }} down</span>
                </div>
                <el-button
                  v-if="upstream.hasBoth"
                  type="warning"
                  size="small"
                  class="upstream-toggle-btn"
                  @click.stop="handleToggleAll(upstream)"
                  >切换</el-button
                >
              </div>
            </template>
            <el-table
              v-force-reflow
              :data="upstream.servers"
              size="small"
              :row-class-name="({ row }) => (isServerSelected(upstream.name, row) ? 'selected-row' : '')"
              class="server-table"
            >
              <el-table-column width="50">
                <template #header>
                  <el-checkbox
                    :model-value="isUpstreamAllSelected(upstream)"
                    @change="(val) => toggleUpstreamAll(upstream, val)"
                  />
                </template>
                <template #default="{ row }">
                  <el-checkbox
                    :model-value="isServerSelected(upstream.name, row)"
                    @change="() => toggleServer(upstream.name, row)"
                  />
                </template>
              </el-table-column>
              <el-table-column prop="ip" label="IP" width="150" />
              <el-table-column prop="port" label="端口" width="80" />
              <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                  <span :class="['status-dot', row.status === 'up' ? 'status-up' : 'status-down']"></span>
                  {{ row.status }}
                </template>
              </el-table-column>
              <el-table-column prop="weight" label="权重" width="80" />
              <el-table-column label="操作" width="80">
                <template #default="{ row }">
                  <el-button
                    v-if="upstream.servers.some((s) => s.status !== row.status)"
                    type="primary"
                    size="small"
                    link
                    @click="handleSwap(upstream, row)"
                    >切换</el-button
                  >
                </template>
              </el-table-column>
            </el-table>
          </el-collapse-item>
        </el-collapse>

        <div v-if="!loadingUpstreams && filteredUpstreams.length === 0" class="empty-state">
          <p>暂无数据</p>
        </div>
      </div>
    </el-card>

    <!-- 子组件对话框 -->
    <BackupDialog
      v-model="backupDialogVisible"
      :backup-list="backupList"
      :loading="loadingBackups"
      @rollback="handleRollbackFromDialog"
    />
    <PreviewDialog
      v-model="previewVisible"
      :data="previewData"
      :config-file="configFile"
      :executing="executing"
      @execute="executePreview"
    />
    <ConfigViewer v-model="configDialogVisible" :raw-config="rawConfig" :config-file="configFile" />
    <SwapDialog
      v-model="swapDialogVisible"
      :offline-ip="swapOfflineIP"
      :online-ip="swapOnlineIP"
      v-model:affected-upstreams="swapAffectedUpstreams"
      @confirm="confirmSwap"
    />

    <!-- Batch Operations Dialog (内联，逻辑耦合度高) -->
    <el-dialog v-model="batchDialogVisible" width="min(800px, 90vw)" class="cool-dialog" align-center>
      <template #header>
        <div class="batch-dialog-header">
          <span class="el-dialog__title">批量操作</span>
          <span class="batch-hint-text"
            >为每个 Upstream 组选择操作类型，支持上线、下线、切换（反转全部状态）混合操作</span
          >
        </div>
      </template>
      <div class="batch-dialog-body">
        <div class="batch-hint">
          <el-button size="small" @click="toggleBatchSelectAll">{{ isBatchAllSelected ? '取消' : '全选' }}</el-button>
          <el-button size="small" @click="toggleBatchExpandAll">{{ batchAllExpanded ? '折叠' : '展开' }}</el-button>
          <div style="display: flex; align-items: center; gap: 8px; margin-left: 12px">
            <el-input
              v-model="batchSearch"
              placeholder="搜索 IP / 端口 或 {端口}{操作|索引}"
              size="small"
              clearable
              style="width: 240px"
            />
            <span v-if="syntaxHint" class="batch-syntax-hint" :class="{ 'hint-error': syntaxHint === '语法格式错误' }">
              {{ syntaxHint }}
            </span>
          </div>
        </div>
        <el-table
          ref="batchTableRef"
          v-force-reflow
          :data="filteredBatchItems"
          size="small"
          max-height="500"
          class="batch-table"
          row-key="upstreamName"
        >
          <el-table-column type="expand" width="1">
            <template #default="{ row }">
              <div class="batch-expand-servers">
                <div v-for="s in row.servers" :key="serverKey(s)" class="batch-server-item">
                  <span class="status-dot" :class="s.status === 'up' ? 'status-up' : 'status-down'"></span>
                  <span class="batch-server-ip">{{ s.ip }}</span>
                  <span class="batch-server-port">:{{ s.port }}</span>
                  <span v-if="s.weight" class="batch-server-weight">w={{ s.weight }}</span>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="60" align="center">
            <template #default="{ row }">
              <el-checkbox v-model="row.enabled" :disabled="!row.hasBoth && !row.hasMultipleUp" />
            </template>
          </el-table-column>
          <el-table-column label="Upstream 组" min-width="150" sortable :sort-method="(a, b) => a.upstreamName.localeCompare(b.upstreamName)">
            <template #default="{ row }">
              <span class="batch-upstream-name" @mousedown.prevent @click="toggleBatchExpand(row)">{{
                row.upstreamName
              }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="130" sortable :sort-method="(a, b) => a.upCount - b.upCount">
            <template #default="{ row }">
              <span class="badge badge-success">{{ row.upCount }} up</span>
              <span class="badge badge-danger">{{ row.downCount }} down</span>
            </template>
          </el-table-column>
          <el-table-column label="操作类型" width="180">
            <template #default="{ row }">
              <el-select
                v-model="row.action"
                placeholder="选择操作"
                size="small"
                :disabled="!row.enabled"
                style="width: 100%"
                @change="onBatchActionChange(row)"
              >
                <el-option v-for="a in getAvailableActions(row)" :key="a.value" :label="a.label" :value="a.value" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="目标服务器" min-width="200">
            <template #default="{ row }">
              <template v-if="row.action === 'online' || row.action === 'offline'">
                <div style="display: flex; align-items: center; gap: 6px">
                  <el-select
                    v-model="row.backendIPs"
                    placeholder="选择服务器（可多选）"
                    size="small"
                    multiple
                    collapse-tags
                    collapse-tags-tooltip
                    style="flex: 1"
                  >
                    <template #header>
                      <el-button
                        size="small"
                        type="primary"
                        text
                        style="padding: 0 4px; font-size: 12px"
                        @click.stop="toggleBatchIPSelectAll(row)"
                        >{{ isBatchIPAllSelected(row) ? '取消全选' : '全选' }}</el-button
                      >
                    </template>
                    <el-option
                      v-for="s in getSelectableServers(row)"
                      :key="serverKey(s)"
                      :label="serverKey(s)"
                      :value="normalizeIPKey(s)"
                    />
                  </el-select>
                </div>
                <div v-if="row.action === 'offline'" class="batch-offline-hint">至少保留 1 台在线服务器</div>
              </template>
              <span v-else-if="row.action === 'toggle'" style="color: var(--text-secondary); font-size: 12px"
                >全部反转</span
              >
            </template>
          </el-table-column>
        </el-table>
        <div v-if="filteredBatchItems.length === 0" class="batch-empty">暂无可执行的批量操作</div>
      </div>
      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" :disabled="batchValidCount === 0" @click="executeBatch"
          >预览并执行（{{ batchValidCount }} 项）</el-button
        >
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="font-weight: 700">执行结果</span>
          <el-button size="small" text @click="output = ''">关闭</el-button>
        </div>
      </template>
      <div class="output-body">
        <div class="output-meta">
          <div class="output-meta-item">
            <span class="output-meta-label">操作类型</span>
            <el-tag :type="outputMeta.actionType" size="small">{{ outputMeta.actionLabel }}</el-tag>
          </div>
          <div v-if="outputMeta.upstreamNames.length > 0" class="output-meta-item">
            <span class="output-meta-label">Upstream 组</span>
            <div class="output-meta-tags">
              <el-tag v-for="name in outputMeta.upstreamNames" :key="name" size="small" type="info">{{ name }}</el-tag>
            </div>
          </div>
          <div v-if="outputMeta.ipCount > 0" class="output-meta-item">
            <span class="output-meta-label">涉及服务器</span>
            <span class="output-meta-value">{{ outputMeta.ipCount }} 台</span>
          </div>
          <div class="output-meta-item">
            <span class="output-meta-label">执行状态</span>
            <el-tag :type="outputMeta.success ? 'success' : 'danger'" size="small">{{
              outputMeta.success ? '执行成功' : '执行失败'
            }}</el-tag>
          </div>
          <div class="output-meta-item">
            <span class="output-meta-label">执行时间</span>
            <span class="output-meta-value">{{ outputMeta.time }}</span>
          </div>
        </div>
        <pre class="terminal-pre terminal-lg">{{ output }}</pre>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, onMounted, onActivated } from 'vue'
import {
  getNginxConfigs,
  getNginxUpstreams,
  nginxOnlinePreview,
  nginxOnlineExecute,
  nginxOfflinePreview,
  nginxOfflineExecute,
  nginxSwapPreview,
  nginxSwapExecute,
  nginxTogglePreview,
  nginxToggleExecute,
  nginxBatchPreview,
  nginxBatchExecute,
  nginxRollbackPreview,
  nginxRollbackExecute,
  getNginxBackups,
} from '../../api'
import { useServerSelector } from '../../composables/useServerSelector'
import { useOutputCache } from '../../composables/useOutputCache'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { STORAGE_KEYS, BACKUP_FETCH_TIMEOUT_MS } from '../../utils/constants'
import BackupDialog from './BackupDialog.vue'
import PreviewDialog from './PreviewDialog.vue'
import ConfigViewer from './ConfigViewer.vue'
import SwapDialog from './SwapDialog.vue'

// --- 组合式函数 ---
const { servers, serverId, initServers, refreshServers, saveSelection } = useServerSelector(
  'nginx',
  STORAGE_KEYS.NGINX_SERVER,
  loadConfigs
)
const configFiles = ref([])
const configFile = ref('')
const upstreams = shallowRef([])
const backups = ref([])
const backupDialogVisible = ref(false)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const outputMeta = ref({ actionType: 'info', actionLabel: '', upstreamNames: [], ipCount: 0, success: true, time: '' })
const currentAction = ref('')
const currentBatchInfo = ref(null)
const expandedUpstreams = ref([])
const filterKeyword = ref('')
const statusFilter = ref('all')
const rawConfig = ref('')
const configDialogVisible = ref(false)
const loadingUpstreams = ref(false)
const loadingBackups = ref(false)
const swapDialogVisible = ref(false)
const swapOfflineIP = ref('')
const swapOnlineIP = ref('')
const swapAffectedUpstreams = ref([])
const batchDialogVisible = ref(false)
const batchItems = ref([])
const batchSearch = ref('')
const syntaxHint = ref('')
const isSyntaxMode = ref(false)
const batchTableRef = ref(null)
const batchAllExpanded = ref(false)
const selectedMap = ref({})

watch(batchSearch, (val) => {
  const trimmed = val.trim()
  if (!trimmed) {
    syntaxHint.value = ''
    isSyntaxMode.value = false
    return
  }
  const parsed = parseBatchSearchSyntax(trimmed)
  if (parsed) {
    isSyntaxMode.value = true
    applyBatchSearchSyntax(parsed)
    syntaxHint.value = getSyntaxHint(trimmed)
  } else if (trimmed.startsWith('{')) {
    // 以 { 开头但格式不对 → 语法错误状态
    isSyntaxMode.value = false
    syntaxHint.value = '语法格式错误'
  } else {
    isSyntaxMode.value = false
    syntaxHint.value = ''
  }
})

// ===== Backup list =====
function parseBackupTime(filename) {
  const match = filename.match(/\.bak\.(\d{14})$/)
  if (!match) return ''
  const t = match[1]
  return `${t.slice(0, 4)}-${t.slice(4, 6)}-${t.slice(6, 8)} ${t.slice(8, 10)}:${t.slice(10, 12)}:${t.slice(12, 14)}`
}

const backupList = computed(() => backups.value.map((name) => ({ name, time: parseBackupTime(name) })))

function serverKey(s) {
  return s.port ? `${s.ip}:${s.port}` : s.ip
}

function isServerSelected(upstreamName, server) {
  const set = selectedMap.value[upstreamName]
  return set ? set.has(serverKey(server)) : false
}

function toggleServer(upstreamName, server) {
  const key = serverKey(server)
  const newMap = { ...selectedMap.value }
  const set = new Set(newMap[upstreamName] || [])
  if (set.has(key)) {
    set.delete(key)
  } else {
    set.add(key)
  }
  newMap[upstreamName] = set
  selectedMap.value = newMap
}

function isUpstreamAllSelected(upstream) {
  const set = selectedMap.value[upstream.name]
  return set ? set.size === upstream.servers.length : false
}

function toggleUpstreamAll(upstream, checked) {
  const newMap = { ...selectedMap.value }
  if (checked) {
    newMap[upstream.name] = new Set(upstream.servers.map((s) => serverKey(s)))
  } else {
    delete newMap[upstream.name]
  }
  selectedMap.value = newMap
}

const selectedServers = computed(() => {
  const result = []
  for (const [upstreamName, keys] of Object.entries(selectedMap.value)) {
    const upstream = upstreams.value.find((u) => u.name === upstreamName)
    if (!upstream) continue
    for (const server of upstream.servers) {
      if (keys.has(serverKey(server))) {
        result.push({ upstreamName, ip: serverKey(server), status: server.status })
      }
    }
  }
  return result
})

const filteredUpstreams = computed(() => {
  const kw = filterKeyword.value.trim().toLowerCase()
  let list = kw
    ? upstreams.value.filter((u) => {
        if (u.name.toLowerCase().includes(kw)) return true
        return u.servers.some((s) => s.ip.toLowerCase().includes(kw) || (s.port && s.port.includes(kw)))
      })
    : upstreams.value
  if (statusFilter.value === 'up') {
    list = list.filter((u) => u.servers.some((s) => s.status === 'up'))
  } else if (statusFilter.value === 'down') {
    list = list.filter((u) => u.servers.some((s) => s.status === 'down'))
  }
  return list.map((u) => {
    const upCount = u.servers.filter((s) => s.status === 'up').length
    const downCount = u.servers.length - upCount
    return { ...u, upCount, downCount, hasBoth: upCount > 0 && downCount > 0 }
  })
})

const totalUpCount = computed(() => filteredUpstreams.value.reduce((sum, u) => sum + u.upCount, 0))
const totalDownCount = computed(() => filteredUpstreams.value.reduce((sum, u) => sum + u.downCount, 0))

const isAllSelected = computed(() => {
  const filtered = filteredUpstreams.value
  if (filtered.length === 0) return false
  return filtered.every((u) => {
    const set = selectedMap.value[u.name]
    return set && set.size === u.servers.length
  })
})

function toggleSelectAll() {
  if (isAllSelected.value) {
    const newMap = { ...selectedMap.value }
    for (const u of filteredUpstreams.value) {
      delete newMap[u.name]
    }
    selectedMap.value = newMap
  } else {
    const newMap = { ...selectedMap.value }
    for (const u of filteredUpstreams.value) {
      newMap[u.name] = new Set(u.servers.map((s) => serverKey(s)))
    }
    selectedMap.value = newMap
  }
}

const allExpanded = computed(() => {
  const filtered = filteredUpstreams.value
  return filtered.length > 0 && expandedUpstreams.value.length === filtered.length
})

function toggleExpandAll() {
  if (allExpanded.value) {
    expandedUpstreams.value = []
  } else {
    expandedUpstreams.value = filteredUpstreams.value.map((u) => u.name)
  }
}

function toggleStatusFilter(type) {
  statusFilter.value = statusFilter.value === type ? 'all' : type
}

useOutputCache([() => serverId.value, () => configFile.value], output, {
  getExtra: () => outputMeta.value,
  setExtra: (extra) => {
    outputMeta.value = extra || {
      actionType: 'info',
      actionLabel: '',
      upstreamNames: [],
      ipCount: 0,
      success: true,
      time: '',
    }
  },
})

onMounted(initServers)

onActivated(async () => {
  await refreshServers()
  if (serverId.value) loadConfigs()
})

async function loadConfigs() {
  if (!serverId.value) return
  saveSelection()
  try {
    configFiles.value = await getNginxConfigs(serverId.value)
    if (configFiles.value.length > 0) {
      const saved = localStorage.getItem(STORAGE_KEYS.nginxConfig(serverId.value))
      configFile.value = saved && configFiles.value.includes(saved) ? saved : configFiles.value[0]
      await loadUpstreams()
    }
  } catch (e) {
    ElMessage.error('加载配置列表失败: ' + (e.response?.data?.error || e.message))
  }
}

function onConfigChange() {
  if (serverId.value && configFile.value) {
    localStorage.setItem(STORAGE_KEYS.nginxConfig(serverId.value), configFile.value)
  }
  loadUpstreams()
}

async function loadUpstreams() {
  if (!serverId.value || !configFile.value) return
  loadingUpstreams.value = true
  try {
    const res = await getNginxUpstreams(serverId.value, configFile.value)
    upstreams.value = res.upstreams || []
    rawConfig.value = res.raw || ''
    selectedMap.value = {}
    expandedUpstreams.value = upstreams.value.map((u) => u.name)
    if (upstreams.value.length === 0 && res.raw) {
      ElMessage.warning('未解析到upstream配置，请检查配置文件格式')
    }
  } catch (e) {
    ElMessage.error('加载upstream失败: ' + (e.response?.data?.error || e.message))
  } finally {
    loadingUpstreams.value = false
  }
}

async function openBackupDialog() {
  if (!serverId.value) {
    ElMessage.warning('请先选择服务器')
    return
  }
  backupDialogVisible.value = true
  loadingBackups.value = true
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), BACKUP_FETCH_TIMEOUT_MS)
    const res = await getNginxBackups(serverId.value, { signal: controller.signal })
    clearTimeout(timeoutId)
    backups.value = res || []
  } catch (e) {
    if (e.name === 'AbortError') {
      ElMessage.error('加载备份列表超时，请检查服务器连接')
    } else {
      ElMessage.error('加载备份列表失败: ' + (e.response?.data?.error || e.message))
    }
    backups.value = []
  } finally {
    loadingBackups.value = false
  }
}

function handleViewConfig() {
  if (!configFile.value) {
    ElMessage.warning('请先选择配置文件')
    return
  }
  configDialogVisible.value = true
}

async function handleRefresh() {
  if (!serverId.value || !configFile.value) {
    ElMessage.warning('请先选择服务器和配置文件')
    return
  }
  statusFilter.value = 'all'
  await loadUpstreams()
  ElMessage.success('刷新成功')
}

async function handleBatchOnline() {
  const onlineServers = selectedServers.value.filter((s) => s.status !== 'up')
  if (onlineServers.length === 0) {
    ElMessage.warning('选中的后端服务均已上线，无需重复操作')
    return
  }
  if (onlineServers.length < selectedServers.value.length) {
    ElMessage.warning('部分选中的后端服务已上线，将只对未上线的服务执行操作')
  }
  const grouped = {}
  for (const { upstreamName, ip } of onlineServers) {
    if (!grouped[upstreamName]) grouped[upstreamName] = []
    grouped[upstreamName].push(ip)
  }
  await handleBatchAction(Object.keys(grouped), Object.values(grouped).flat(), 'online')
}

async function handleBatchOffline() {
  const offlineServers = selectedServers.value.filter((s) => s.status !== 'down')
  if (offlineServers.length === 0) {
    ElMessage.warning('选中的后端服务均已下线，无需重复操作')
    return
  }
  const grouped = {}
  for (const { upstreamName, ip } of offlineServers) {
    if (!grouped[upstreamName]) grouped[upstreamName] = []
    grouped[upstreamName].push(ip)
  }
  for (const [upstreamName, ips] of Object.entries(grouped)) {
    const upstream = upstreams.value.find((u) => u.name === upstreamName)
    if (!upstream) continue
    const totalUp = upstream.servers.filter((s) => s.status === 'up').length
    if (ips.length >= totalUp) {
      ElMessage.warning(`禁止操作：upstream [${upstreamName}] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`)
      return
    }
  }
  if (offlineServers.length < selectedServers.value.length) {
    ElMessage.warning('部分选中的后端服务已下线，将只对未下线的服务执行操作')
  }
  await handleBatchAction(Object.keys(grouped), Object.values(grouped).flat(), 'offline')
}

function normalizeIPKey(s) {
  const key = serverKey(s)
  return key.endsWith(':80') ? key.slice(0, -3) : key
}

function handleSwap(upstream, server) {
  const opposite = upstream.servers.find((s) => s.status !== server.status)
  if (!opposite) return
  swapOfflineIP.value = server.status === 'up' ? normalizeIPKey(server) : normalizeIPKey(opposite)
  swapOnlineIP.value = server.status === 'up' ? normalizeIPKey(opposite) : normalizeIPKey(server)
  const affected = []
  for (const u of upstreams.value) {
    let hasOffline = false
    let hasOnline = false
    for (const s of u.servers) {
      const key = normalizeIPKey(s)
      if (key === swapOfflineIP.value && s.status === 'up') hasOffline = true
      if (key === swapOnlineIP.value && s.status === 'down') hasOnline = true
    }
    if (hasOffline && hasOnline) {
      affected.push({
        name: u.name,
        totalCount: u.servers.length,
        upCount: u.servers.filter((s) => s.status === 'up').length,
        downCount: u.servers.filter((s) => s.status === 'down').length,
        checked: true,
      })
    }
  }
  swapAffectedUpstreams.value = affected
  swapDialogVisible.value = true
}

async function confirmSwap() {
  const selectedUpstreams = swapAffectedUpstreams.value.filter((i) => i.checked).map((i) => i.name)
  if (selectedUpstreams.length === 0) return
  swapDialogVisible.value = false
  try {
    const res = await nginxSwapPreview({
      server_id: serverId.value,
      config_file: configFile.value,
      upstream_names: selectedUpstreams,
      offline_ip: swapOfflineIP.value,
      online_ip: swapOnlineIP.value,
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'swap'
    currentBatchInfo.value = { upstreamNames: selectedUpstreams, ipCount: 2, action: 'swap' }
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleToggleAll(upstream) {
  const upCount = upstream.servers.filter((s) => s.status === 'up').length
  const downCount = upstream.servers.filter((s) => s.status === 'down').length
  try {
    await ElMessageBox.confirm(
      `将反转 ${upstream.name} 中所有服务器状态：${upCount} 台 up → down，${downCount} 台 down → up。确认执行？`,
      '切换确认',
      { confirmButtonText: '确认', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    const res = await nginxTogglePreview({
      server_id: serverId.value,
      config_file: configFile.value,
      upstream_names: [upstream.name],
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'toggle'
    currentBatchInfo.value = { upstreamNames: [upstream.name], ipCount: upstream.servers.length, action: 'toggle' }
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

function parseBatchSearchSyntax(input) {
  const trimmed = input.trim()
  const match = trimmed.match(/^\{([^}]+)\}\{([^}]+)\}$/)
  if (!match) return null

  const ports = match[1].split(/\s*\|\s*/).map((p) => p.trim()).filter(Boolean)
  if (ports.length === 0) return null

  const parts = match[2].split(/\s*\|\s*/)
  const actionMap = { 上线: 'online', 下线: 'offline', 切换: 'toggle' }
  const action = actionMap[parts[0]?.trim()]
  if (!action) return null

  let index = 1
  if (action !== 'toggle') {
    const rawIndex = parts[1]?.trim()
    if (rawIndex !== undefined && rawIndex !== '') {
      const parsed = parseInt(rawIndex, 10)
      if (isNaN(parsed) || parsed === 0) return null
      index = parsed
    }
  }

  return { ports, action, index }
}

function applyBatchSearchSyntax(parsed) {
  let matchedCount = 0
  for (const item of batchItems.value) {
    const portMatch = item.servers.some((s) => parsed.ports.includes(s.port))
    if (!portMatch) {
      item.enabled = false
      continue
    }
    item.action = parsed.action
    if (parsed.action === 'toggle') {
      if (!item.hasBoth) {
        item.enabled = false
        item.action = ''
        continue
      }
      item.enabled = true
      item.backendIPs = []
    } else {
      const selectable = getSelectableServers(item)
      if (selectable.length === 0) {
        item.enabled = false
        item.action = ''
        continue
      }
      let idx = parsed.index
      if (idx === -1) idx = selectable.length
      else if (idx > selectable.length) {
        item.enabled = false
        item.action = ''
        continue
      }
      const target = selectable[idx - 1]
      if (!target) {
        item.enabled = false
        item.action = ''
        continue
      }
      item.enabled = true
      item.backendIPs = [normalizeIPKey(target)]
    }
    matchedCount++
  }
  return matchedCount
}

function getSyntaxHint(input) {
  const parsed = parseBatchSearchSyntax(input)
  if (!parsed) return ''
  if (parsed.action === 'toggle') {
    const count = batchItems.value.filter(
      (item) => item.servers.some((s) => parsed.ports.includes(s.port)) && item.hasBoth
    ).length
    return count > 0 ? `已选中 ${count} 个upstream组，切换全部` : ''
  }
  const actionLabel = parsed.action === 'online' ? '上线' : '下线'
  const indexLabel = parsed.index === -1 ? '最后 1' : `第 ${parsed.index}`
  const count = batchItems.value.filter((item) => {
    if (!item.servers.some((s) => parsed.ports.includes(s.port))) return false
    return getSelectableServers(item).length > 0
  }).length
  return count > 0 ? `已选中 ${count} 个upstream组，${actionLabel} ${indexLabel} 个服务器` : ''
}

function openBatchDialog() {
  batchItems.value = upstreams.value.map((u) => {
    const upServers = u.servers.filter((s) => s.status === 'up')
    const downServers = u.servers.filter((s) => s.status === 'down')
    const hasBoth = upServers.length > 0 && downServers.length > 0
    return {
      upstreamName: u.name,
      enabled: false,
      action: hasBoth ? 'toggle' : '',
      backendIPs: [],
      servers: u.servers,
      upCount: upServers.length,
      downCount: downServers.length,
      hasBoth,
      hasMultipleUp: upServers.length >= 2,
    }
  })
  batchSearch.value = ''
  batchDialogVisible.value = true
}

const filteredBatchItems = computed(() => {
  const q = batchSearch.value.trim().toLowerCase()
  if (!q) return batchItems.value
  if (isSyntaxMode.value) return batchItems.value
  return batchItems.value.filter((item) => {
    if (item.upstreamName.toLowerCase().includes(q)) return true
    return item.servers.some((s) => s.ip.includes(q) || s.port.includes(q))
  })
})

const isBatchAllSelected = computed(() => {
  const eligible = filteredBatchItems.value.filter((i) => i.hasBoth || i.hasMultipleUp)
  return eligible.length > 0 && eligible.every((i) => i.enabled)
})

function toggleBatchSelectAll() {
  const newState = !isBatchAllSelected.value
  filteredBatchItems.value.forEach((i) => {
    if (i.hasBoth || i.hasMultipleUp) {
      i.enabled = newState
    }
  })
}

function toggleBatchExpand(row) {
  batchTableRef.value?.toggleRowExpansion(row)
}

function toggleBatchExpandAll() {
  const newState = !batchAllExpanded.value
  filteredBatchItems.value.forEach((row) => {
    batchTableRef.value?.toggleRowExpansion(row, newState)
  })
  batchAllExpanded.value = newState
}

const batchValidCount = computed(() => {
  return batchItems.value
    .filter((i) => {
      if (!i.enabled || !i.action) return false
      if (i.action === 'toggle') return true
      return i.backendIPs && i.backendIPs.length > 0
    })
    .reduce((sum, i) => {
      if (i.action === 'toggle') return sum + 1
      return sum + i.backendIPs.length
    }, 0)
})

function getAvailableActions(item) {
  const actions = []
  if (item.hasBoth) actions.push({ label: '切换（反转全部）', value: 'toggle' })
  if (item.hasMultipleUp) actions.push({ label: '下线', value: 'offline' })
  if (item.downCount > 0) actions.push({ label: '上线', value: 'online' })
  return actions
}

function getSelectableServers(item) {
  if (item.action === 'online') return item.servers.filter((s) => s.status === 'down')
  if (item.action === 'offline') return item.servers.filter((s) => s.status === 'up')
  return []
}

function getOfflineSafeServers(item) {
  return item.servers.filter((s) => s.status === 'up').slice(1)
}

function isBatchIPAllSelected(item) {
  const selectable = getSelectableServers(item)
  if (selectable.length === 0) return false
  if (item.action === 'offline') {
    const safe = getOfflineSafeServers(item)
    return safe.length > 0 && safe.every((s) => item.backendIPs.includes(normalizeIPKey(s)))
  }
  return selectable.every((s) => item.backendIPs.includes(normalizeIPKey(s)))
}

function toggleBatchIPSelectAll(item) {
  if (isBatchIPAllSelected(item)) {
    item.backendIPs = []
  } else {
    item.backendIPs = (item.action === 'offline' ? getOfflineSafeServers(item) : getSelectableServers(item)).map((s) =>
      normalizeIPKey(s)
    )
  }
}

function onBatchActionChange(row) {
  row.backendIPs = []
}

async function executeBatch() {
  for (const i of batchItems.value) {
    if (!i.enabled || i.action !== 'offline') continue
    const upServers = i.servers.filter((s) => s.status === 'up')
    if (i.backendIPs.length >= upServers.length) {
      ElMessage.warning(`upstream [${i.upstreamName}] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`)
      return
    }
  }
  const items = []
  for (const i of batchItems.value) {
    if (!i.enabled || !i.action) continue
    if (i.action === 'toggle') {
      items.push({ upstream_name: i.upstreamName, action: 'toggle', backend_ip: '' })
    } else if (i.backendIPs && i.backendIPs.length > 0) {
      for (const ip of i.backendIPs) {
        items.push({ upstream_name: i.upstreamName, action: i.action, backend_ip: ip })
      }
    }
  }
  if (items.length === 0) {
    ElMessage.warning('请至少配置一个操作')
    return
  }
  try {
    const res = await nginxBatchPreview({ server_id: serverId.value, config_file: configFile.value, items })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'batch'
    currentBatchInfo.value = {
      upstreamNames: items.map((i) => i.upstream_name),
      ipCount: items.length,
      action: 'batch',
    }
    batchDialogVisible.value = false
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleBatchAction(upstreamNames, ips, action) {
  const previewFn = action === 'online' ? nginxOnlinePreview : nginxOfflinePreview
  try {
    const res = await previewFn({
      server_id: serverId.value,
      config_file: configFile.value,
      upstream_names: upstreamNames,
      backend_ip: ips.join(','),
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    currentBatchInfo.value = { upstreamNames, ipCount: ips.length, action }
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleRollbackFromDialog(backupFile) {
  try {
    await ElMessageBox.confirm(
      `确定要回滚到备份文件吗？\n\n文件：${backupFile}\n时间：${parseBackupTime(backupFile)}`,
      '回滚确认',
      { confirmButtonText: '确认回滚', cancelButtonText: '取消', type: 'warning' }
    )
    backupDialogVisible.value = false
    const res = await nginxRollbackPreview({
      server_id: serverId.value,
      config_file: configFile.value,
      backup_file: backupFile,
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'rollback'
    currentBatchInfo.value = null
    previewVisible.value = true
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '预览失败')
    }
  }
}

const ACTION_META = {
  online: { type: 'success', label: '批量上线' },
  offline: { type: 'danger', label: '批量下线' },
  swap: { type: 'warning', label: '切换' },
  toggle: { type: 'warning', label: '组切换' },
  batch: { type: 'warning', label: '批量操作' },
  rollback: { type: 'info', label: '回滚' },
}

function getNowStr() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

async function executePreview() {
  executing.value = true
  const action = currentAction.value
  const executeFn = {
    online: nginxOnlineExecute,
    offline: nginxOfflineExecute,
    swap: nginxSwapExecute,
    toggle: nginxToggleExecute,
    batch: nginxBatchExecute,
    rollback: nginxRollbackExecute,
  }[action]
  const meta = ACTION_META[action] || { type: 'info', label: action }
  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = res.output || res.message || '执行成功'
    outputMeta.value = {
      actionType: meta.type,
      actionLabel: meta.label,
      upstreamNames: currentBatchInfo.value?.upstreamNames || [],
      ipCount: currentBatchInfo.value?.ipCount || 0,
      success: true,
      time: getNowStr(),
    }
    previewVisible.value = false
    ElMessage.success('执行成功')
    await loadConfigs()
  } catch (e) {
    const msg = e.response?.data?.error || e.message || '执行失败'
    const detail = e.response?.data?.output || ''
    ElMessage.error(msg)
    output.value = detail ? `${msg}\n\n${detail}` : msg
    outputMeta.value = {
      actionType: 'danger',
      actionLabel: meta.label + '（失败）',
      upstreamNames: currentBatchInfo.value?.upstreamNames || [],
      ipCount: currentBatchInfo.value?.ipCount || 0,
      success: false,
      time: getNowStr(),
    }
  } finally {
    executing.value = false
  }
}
</script>

<style scoped>
.nginx-page {
  padding: 2px;
}

.stat-chip-danger {
  cursor: pointer;
  transition: all 0.2s;
}
.stat-chip-danger:hover {
  background: rgba(239, 68, 68, 0.15);
}
.stat-chip-danger.stat-chip-active {
  background: rgba(239, 68, 68, 0.2);
  color: #ef4444;
}
.stat-chip-danger.stat-chip-active b {
  color: #ef4444;
}

.upstream-collapse {
  border: none;
}
:deep(.upstream-collapse .el-collapse-item) {
  margin-bottom: 10px;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--border-default);
  background: var(--card-bg);
  transition: border-color 0.2s;
}
:deep(.upstream-collapse .el-collapse-item:hover) {
  border-color: var(--border-strong);
}
:deep(.upstream-collapse .health-healthy) {
  border-left: 3px solid #22c55e;
}
:deep(.upstream-collapse .health-degraded) {
  border-left: 3px solid #f59e0b;
}
:deep(.upstream-collapse .health-critical) {
  border-left: 3px solid #ef4444;
}
:deep(.upstream-collapse .el-collapse-item__header) {
  background: var(--bg-elevated);
  border-bottom: 1px solid transparent;
  padding: 0 16px;
  height: 48px;
  line-height: 48px;
  font-size: 14px;
  transition: background 0.2s;
  color: var(--text-primary);
}
:deep(.upstream-collapse .el-collapse-item.is-active .el-collapse-item__header) {
  border-bottom-color: var(--border-default);
}
:deep(.upstream-collapse .el-collapse-item__wrap) {
  border-bottom: none;
  background: transparent;
}
:deep(.upstream-collapse .el-collapse-item__content) {
  padding: 0;
  color: var(--text-regular);
}

.upstream-header {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.upstream-name {
  font-weight: 700;
  color: var(--text-primary);
  font-size: 14px;
  letter-spacing: 0.3px;
}
.upstream-badges {
  display: flex;
  gap: 6px;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
}
.badge-info {
  background: rgba(6, 182, 212, 0.12);
  color: #06b6d4;
}
.badge-success {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}
.badge-danger {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.server-table {
  border-radius: 0;
}
:deep(.server-table .el-table__header th) {
  background: var(--bg-elevated) !important;
  color: var(--text-regular);
  font-weight: 600;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
:deep(.server-table .el-table__row) {
  transition: background-color 0.15s;
}
:deep(.server-table .el-table__row:hover > td) {
  background-color: rgba(6, 182, 212, 0.04) !important;
}

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}
.status-up {
  background: #22c55e;
  box-shadow: 0 0 6px rgba(34, 197, 94, 0.5);
  animation: nginx-pulse-up 2.5s ease-in-out infinite;
}
.status-down {
  background: #ef4444;
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.4);
  animation: nginx-pulse-down 1.5s ease-in-out infinite;
}
@keyframes nginx-pulse-up {
  0%,
  100% {
    box-shadow: 0 0 4px rgba(34, 197, 94, 0.4);
  }
  50% {
    box-shadow: 0 0 8px rgba(34, 197, 94, 0.7);
  }
}
@keyframes nginx-pulse-down {
  0%,
  100% {
    box-shadow: 0 0 4px rgba(239, 68, 68, 0.3);
  }
  50% {
    box-shadow: 0 0 10px rgba(239, 68, 68, 0.6);
  }
}

:deep(.selected-row) {
  background-color: rgba(6, 182, 212, 0.08) !important;
}
:deep(.selected-row:hover > td) {
  background-color: rgba(6, 182, 212, 0.12) !important;
}

.empty-state {
  text-align: center;
  padding: 40px 0;
  color: var(--text-secondary);
  font-size: 14px;
}

.output-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.output-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 16px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border: 1px solid var(--border-default);
}
.output-meta-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.output-meta-label {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}
.output-meta-value {
  font-size: 13px;
  color: var(--text-primary);
  font-weight: 600;
}
.output-meta-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.terminal-pre {
  background: var(--terminal-bg);
  color: var(--terminal-text);
  padding: 16px;
  border-radius: 8px;
  max-height: 400px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: var(--font-base);
  line-height: 1.6;
  margin: 0;
  border: 1px solid var(--border-default);
}
.terminal-pre::selection,
.terminal-pre *::selection {
  background: rgba(34, 211, 238, 0.5) !important;
  color: #fff !important;
}
.terminal-lg {
  max-height: 600px;
}

.batch-dialog-body {
  max-height: 550px;
  overflow: auto;
  -webkit-overflow-scrolling: touch;
}
.batch-dialog-header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.batch-hint-text {
  font-size: 13px;
  color: var(--text-secondary);
}
.batch-hint {
  display: flex;
  align-items: center;
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 12px;
  padding: 8px 12px;
  background: var(--bg-elevated);
  border-radius: 6px;
}
.batch-table {
  width: 100%;
}
.batch-empty {
  text-align: center;
  color: var(--text-secondary);
  padding: 30px 0;
  font-size: 14px;
}
.batch-offline-hint {
  font-size: 11px;
  color: #f59e0b;
  margin-top: 2px;
  line-height: 1.2;
}
.batch-syntax-hint {
  font-size: 12px;
  color: var(--el-color-success);
  white-space: nowrap;
}
.batch-syntax-hint.hint-error {
  color: var(--el-color-danger);
}
.batch-upstream-name {
  color: #06b6d4;
  cursor: pointer;
  font-weight: 500;
}
.batch-upstream-name:hover {
  text-decoration: underline;
}
.batch-expand-servers {
  padding: 8px 12px 8px 76px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
}
.batch-server-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  line-height: 1;
}
.batch-server-ip {
  font-family: monospace;
  color: var(--text-primary);
}
.batch-server-port {
  color: var(--text-secondary);
  font-family: monospace;
}
.batch-server-weight {
  color: var(--text-secondary);
  font-size: 12px;
  margin-left: 2px;
}

:deep(.batch-table .el-table__expand-column .el-table__expand-icon) {
  display: none;
}
:deep(.batch-table td:focus),
:deep(.batch-table th:focus),
:deep(.batch-table *:focus),
:deep(.batch-table *:focus-visible) {
  outline: none !important;
}

.upstream-toggle-btn {
  margin-left: auto;
  margin-right: 8px;
}

:deep(.el-loading-mask) {
  border-radius: 8px;
}

@media (max-width: 768px) {
  .server-table {
    overflow-x: auto !important;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }
  :deep(.server-table .el-table__inner-wrapper) {
    overflow: visible !important;
  }
  :deep(.server-table .el-table__header-wrapper > table),
  :deep(.server-table .el-table__body-wrapper > table) {
    width: max-content !important;
    min-width: 100%;
  }
  :deep(.server-table .el-table__body-wrapper) {
    overflow-y: auto !important;
    overflow-x: hidden !important;
  }
  .batch-table {
    overflow-x: auto !important;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }
  :deep(.batch-table .el-table__inner-wrapper) {
    overflow: visible !important;
  }
  :deep(.batch-table .el-table__header-wrapper > table),
  :deep(.batch-table .el-table__body-wrapper > table) {
    width: max-content !important;
    min-width: 100%;
  }
  :deep(.batch-table .el-table__body-wrapper) {
    overflow-y: auto !important;
    overflow-x: hidden !important;
  }
}
</style>
