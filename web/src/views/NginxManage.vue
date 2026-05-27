<template>
  <div class="nginx-page">
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择Nginx服务器" style="width: 250px" @change="loadConfigs">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span class="filter-label">文件:</span>
          <el-select v-model="configFile" placeholder="选择配置文件" style="width: 250px" @change="onConfigChange">
            <el-option v-for="f in configFiles" :key="f" :label="f" :value="f" />
          </el-select>
          <el-input v-model="filterKeyword" placeholder="搜索upstream/ip/port" clearable style="width: 250px;" />
        </div>

        <!-- Stats Bar -->
        <div class="stats-bar">
          <div class="stat-item">
            <span class="stat-value">{{ filteredUpstreams.length }}</span>
            <span class="stat-label">Upstream 组</span>
          </div>
          <div class="stat-item">
            <span class="stat-value stat-success">{{ totalUpCount }}</span>
            <span class="stat-label">在线</span>
          </div>
          <div class="stat-item">
            <span class="stat-value stat-danger">{{ totalDownCount }}</span>
            <span class="stat-label">离线</span>
          </div>
          <div class="stat-item">
            <span class="stat-value stat-primary">{{ selectedServers.length }}</span>
            <span class="stat-label">已选择后端</span>
          </div>
        </div>

        <!-- Toolbar -->
        <div class="toolbar">
          <el-button type="info" class="el-button--cyan" @click="toggleExpandAll">{{ allExpanded ? '折叠' : '展开' }}</el-button>
          <el-button type="info" class="el-button--cyan" @click="toggleSelectAll">{{ isAllSelected ? '取消全选' : '全选' }}</el-button>
          <el-button type="primary" :disabled="selectedServers.length === 0" @click="handleBatchOnline">批量上线</el-button>
          <el-button type="danger" :disabled="selectedServers.length === 0" @click="handleBatchOffline">批量下线</el-button>
          <el-button type="primary" @click="openBatchDialog">批量操作</el-button>
          <el-button type="warning" @click="openBackupDialog">备份列表</el-button>
          <el-button type="success" @click="handleViewConfig">查看配置</el-button>
          <el-button type="info" class="el-button--cyan" @click="handleRefresh">刷新</el-button>
        </div>
      </template>

      <!-- Upstream Groups -->
      <div v-loading="loadingUpstreams">
        <el-collapse v-model="expandedUpstreams" class="upstream-collapse">
          <el-collapse-item v-for="upstream in filteredUpstreams" :key="upstream.name" :name="upstream.name" class="upstream-item">
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
                >切换</el-button>
              </div>
            </template>
            <el-table :data="upstream.servers" size="small" :row-class-name="({ row }) => isServerSelected(upstream.name, row) ? 'selected-row' : ''" class="server-table" v-force-reflow>
              <el-table-column width="50">
                <template #header>
                  <el-checkbox :model-value="isUpstreamAllSelected(upstream)" @change="val => toggleUpstreamAll(upstream, val)" />
                </template>
                <template #default="{ row }">
                  <el-checkbox :model-value="isServerSelected(upstream.name, row)" @change="() => toggleServer(upstream.name, row)" />
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
                    v-if="upstream.servers.some(s => s.status !== row.status)"
                    type="primary"
                    size="small"
                    link
                    @click="handleSwap(upstream, row)"
                  >切换</el-button>
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

    <!-- Backup Dialog -->
    <el-dialog v-model="backupDialogVisible" title="备份列表" width="700px" class="cool-dialog">
      <el-table :data="backupList" size="small" max-height="400" v-loading="loadingBackups" class="backup-table" v-force-reflow>
        <el-table-column label="文件名" min-width="200">
          <template #default="{ row }">{{ row.name }}</template>
        </el-table-column>
        <el-table-column label="备份时间" width="180">
          <template #default="{ row }">{{ row.time }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="handleRollbackFromDialog(row.name)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="90%" top="5vh" class="cool-dialog">
      <div v-if="previewData">
        <div class="preview-desc">{{ previewData.description }}</div>
        <div class="diff-container">
          <div class="diff-header">
            <span class="diff-filename">{{ configFile }}</span>
          </div>
          <div class="diff-body">
            <div
              v-for="(line, index) in previewData.line_diffs"
              :key="index"
              :class="['diff-line', `diff-${line.type}`]"
            >
              <span class="diff-line-num">{{ line.line_num }}</span>
              <span class="diff-line-prefix">{{ getLinePrefix(line.type) }}</span>
              <span class="diff-line-content">{{ line.content }}</span>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Config Viewer Dialog -->
    <el-dialog v-model="configDialogVisible" :title="'配置文件 - ' + configFile" width="80%" top="5vh" class="cool-dialog">
      <pre class="terminal-pre terminal-lg" v-html="highlightedConfig"></pre>
    </el-dialog>

    <!-- Swap Target Dialog -->
    <el-dialog v-model="swapDialogVisible" title="切换服务器" width="600px" class="cool-dialog">
      <div v-if="swapOfflineIP" class="swap-dialog-body">
        <div class="swap-ip-pair">
          <el-tag type="danger" size="large">{{ swapOfflineIP }} (下线)</el-tag>
          <span class="swap-arrow">⇅</span>
          <el-tag type="success" size="large">{{ swapOnlineIP }} (上线)</el-tag>
        </div>
        <div class="swap-upstream-list">
          <div class="swap-label">选择要执行切换的 Upstream 组：</div>
          <div v-for="item in swapAffectedUpstreams" :key="item.name" class="swap-upstream-item">
            <el-checkbox v-model="item.checked" />
            <span class="upstream-name">{{ item.name }}</span>
            <span class="badge badge-info">{{ item.totalCount }} 台</span>
            <span class="badge badge-success">{{ item.upCount }} up</span>
            <span class="badge badge-danger">{{ item.downCount }} down</span>
          </div>
          <div v-if="swapAffectedUpstreams.length === 0" style="color: #909399; padding: 20px 0; text-align: center;">
            未找到同时包含这两台服务器的 Upstream 组
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="swapDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="swapAffectedUpstreams.filter(i => i.checked).length === 0" @click="confirmSwap">确认切换</el-button>
      </template>
    </el-dialog>

    <!-- Batch Operations Dialog -->
    <el-dialog v-model="batchDialogVisible" title="批量操作" width="800px" class="cool-dialog">
      <div class="batch-dialog-body">
        <div class="batch-hint">
          <span>为每个 Upstream 组选择操作类型，支持上线、下线、切换（反转全部状态）混合操作</span>
          <el-button size="small" @click="toggleBatchSelectAll" style="margin-left: 12px;">
            {{ isBatchAllSelected ? '取消全选' : '全选' }}
          </el-button>
        </div>
        <el-table :data="batchItems" size="small" max-height="500" class="batch-table" v-force-reflow>
          <el-table-column label="启用" width="60" align="center">
            <template #default="{ row }">
              <el-checkbox v-model="row.enabled" :disabled="!row.hasBoth && !row.hasMultipleUp" />
            </template>
          </el-table-column>
          <el-table-column label="Upstream 组" prop="upstreamName" min-width="150" />
          <el-table-column label="状态" width="130">
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
                <el-option
                  v-for="a in getAvailableActions(row)"
                  :key="a.value"
                  :label="a.label"
                  :value="a.value"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="目标服务器" min-width="200">
            <template #default="{ row }">
              <template v-if="row.action === 'online' || row.action === 'offline'">
                <div style="display: flex; align-items: center; gap: 6px;">
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
                        @click.stop="toggleBatchIPSelectAll(row)"
                        style="padding: 0 4px; font-size: 12px;"
                      >{{ isBatchIPAllSelected(row) ? '取消全选' : '全选' }}</el-button>
                    </template>
                    <el-option
                      v-for="s in getSelectableServers(row)"
                      :key="serverKey(s)"
                      :label="serverKey(s)"
                      :value="normalizeIPKey(s)"
                    />
                  </el-select>
                </div>
                <div v-if="row.action === 'offline'" class="batch-offline-hint">
                  至少保留 1 台在线服务器
                </div>
              </template>
              <span v-else-if="row.action === 'toggle'" style="color: #909399; font-size: 12px;">全部反转</span>
            </template>
          </el-table-column>
        </el-table>
        <div v-if="batchItems.length === 0" class="batch-empty">
          暂无可执行的批量操作
        </div>
      </div>
      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="executing"
          :disabled="batchValidCount === 0"
          @click="executeBatch"
        >预览并执行（{{ batchValidCount }} 项）</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-weight: 700;">执行结果</span>
          <el-button size="small" text @click="output = ''">关闭</el-button>
        </div>
      </template>
      <div class="output-body">
        <div class="output-meta">
          <div class="output-meta-item">
            <span class="output-meta-label">操作类型</span>
            <el-tag :type="outputMeta.actionType" size="small">{{ outputMeta.actionLabel }}</el-tag>
          </div>
          <div class="output-meta-item" v-if="outputMeta.upstreamNames.length > 0">
            <span class="output-meta-label">Upstream 组</span>
            <div class="output-meta-tags">
              <el-tag v-for="name in outputMeta.upstreamNames" :key="name" size="small" type="info">{{ name }}</el-tag>
            </div>
          </div>
          <div class="output-meta-item" v-if="outputMeta.ipCount > 0">
            <span class="output-meta-label">涉及服务器</span>
            <span class="output-meta-value">{{ outputMeta.ipCount }} 台</span>
          </div>
          <div class="output-meta-item">
            <span class="output-meta-label">执行状态</span>
            <el-tag :type="outputMeta.success ? 'success' : 'danger'" size="small">
              {{ outputMeta.success ? '执行成功' : '执行失败' }}
            </el-tag>
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
import { ref, computed, onMounted } from 'vue'
import {
  getServers, getNginxConfigs, getNginxUpstreams,
  nginxOnlinePreview, nginxOnlineExecute,
  nginxOfflinePreview, nginxOfflineExecute,
  nginxSwapPreview, nginxSwapExecute,
  nginxTogglePreview, nginxToggleExecute,
  nginxBatchPreview, nginxBatchExecute,
  nginxRollbackPreview, nginxRollbackExecute,
  getNginxBackups
} from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const configFiles = ref([])
const configFile = ref('')
const upstreams = ref([])
const backups = ref([])
const backupDialogVisible = ref(false)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const outputMeta = ref({ actionType: 'info', actionLabel: '', upstreamNames: [], ipCount: 0, success: true, time: '' })
const currentAction = ref('')
const expandedUpstreams = ref([])
const filterKeyword = ref('')
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

const selectedMap = ref({})

// ===== Backup list with parsed timestamps =====
function parseBackupTime(filename) {
  const match = filename.match(/\.bak\.(\d{14})$/)
  if (!match) return ''
  const t = match[1]
  return `${t.slice(0,4)}-${t.slice(4,6)}-${t.slice(6,8)} ${t.slice(8,10)}:${t.slice(10,12)}:${t.slice(12,14)}`
}

const backupList = computed(() => {
  return backups.value.map(name => ({ name, time: parseBackupTime(name) }))
})

// ===== Nginx config syntax highlighting =====
const highlightedConfig = computed(() => {
  if (!rawConfig.value) return ''
  const escapeHtml = s => s.replace(/[&<>]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]))
  return escapeHtml(rawConfig.value)
    .replace(/(#.*)$/gm, '<span class="hl-comment">$1</span>')
    .replace(/^(\s*)([\w_]+(?:\s+[\w_]+)*)(?=\s)/gm, (match, indent, directive) => {
      if (directive.startsWith('#') || directive.startsWith('server')) return match
      return `${indent}<span class="hl-directive">${directive}</span>`
    })
    .replace(/\{/g, '<span class="hl-brace">{</span>')
    .replace(/\}/g, '<span class="hl-brace">}</span>')
    .replace(/(\d+\.\d+\.\d+\.\d+(?::\d+)?)/g, '<span class="hl-ip">$1</span>')
    .replace(/(;)/g, '<span class="hl-semicolon">$1</span>')
})

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
    newMap[upstream.name] = new Set(upstream.servers.map(s => serverKey(s)))
  } else {
    delete newMap[upstream.name]
  }
  selectedMap.value = newMap
}

const selectedServers = computed(() => {
  const result = []
  for (const [upstreamName, keys] of Object.entries(selectedMap.value)) {
    const upstream = upstreams.value.find(u => u.name === upstreamName)
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
  const list = kw
    ? upstreams.value.filter(u => {
        if (u.name.toLowerCase().includes(kw)) return true
        return u.servers.some(s => s.ip.toLowerCase().includes(kw) || (s.port && s.port.includes(kw)))
      })
    : upstreams.value
  return list.map(u => {
    const upCount = u.servers.filter(s => s.status === 'up').length
    const downCount = u.servers.length - upCount
    return { ...u, upCount, downCount, hasBoth: upCount > 0 && downCount > 0 }
  })
})

const totalUpCount = computed(() => {
  return filteredUpstreams.value.reduce((sum, u) => sum + u.upCount, 0)
})

const totalDownCount = computed(() => {
  return filteredUpstreams.value.reduce((sum, u) => sum + u.downCount, 0)
})

const isAllSelected = computed(() => {
  const filtered = filteredUpstreams.value
  if (filtered.length === 0) return false
  return filtered.every(u => {
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
      newMap[u.name] = new Set(u.servers.map(s => serverKey(s)))
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
    expandedUpstreams.value = filteredUpstreams.value.map(u => u.name)
  }
}

function getLinePrefix(type) {
  switch (type) {
    case 'added': return '+'
    case 'removed': return '-'
    default: return ' '
  }
}

onMounted(async () => {
  try {
    servers.value = (await getServers('nginx')) || []
    if (servers.value.length > 0) {
      const saved = localStorage.getItem('nginx_server')
      if (saved && servers.value.some(s => s.id === Number(saved))) {
        serverId.value = Number(saved)
      } else {
        serverId.value = servers.value[0].id
      }
      await loadConfigs()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function loadConfigs() {
  if (!serverId.value) return
  localStorage.setItem('nginx_server', serverId.value)
  try {
    configFiles.value = await getNginxConfigs(serverId.value)
    if (configFiles.value.length > 0) {
      const saved = localStorage.getItem(`nginx_config_${serverId.value}`)
      if (saved && configFiles.value.includes(saved)) {
        configFile.value = saved
      } else {
        configFile.value = configFiles.value[0]
      }
      await loadUpstreams()
    }
  } catch (e) {
    ElMessage.error('加载配置列表失败: ' + (e.response?.data?.error || e.message))
  }
}

function onConfigChange() {
  if (serverId.value && configFile.value) {
    localStorage.setItem(`nginx_config_${serverId.value}`, configFile.value)
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
    const timeoutId = setTimeout(() => controller.abort(), 10000)
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
  await loadUpstreams()
  ElMessage.success('刷新成功')
}

async function handleBatchOnline() {
  const onlineServers = selectedServers.value.filter(s => s.status !== 'up')
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
  const upstreamNames = Object.keys(grouped)
  const allIps = Object.values(grouped).flat()
  await handleBatchAction(upstreamNames, allIps, 'online')
}

async function handleBatchOffline() {
  const offlineServers = selectedServers.value.filter(s => s.status !== 'down')
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
    const upstream = upstreams.value.find(u => u.name === upstreamName)
    if (!upstream) continue
    const totalUp = upstream.servers.filter(s => s.status === 'up').length
    if (ips.length >= totalUp) {
      ElMessage.error(`禁止操作：upstream [${upstreamName}] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`)
      return
    }
  }
  const upstreamNames = Object.keys(grouped)
  const allIps = Object.values(grouped).flat()
  await handleBatchAction(upstreamNames, allIps, 'offline')
}

function normalizeIPKey(s) {
  const key = serverKey(s)
  return key.endsWith(':80') ? key.slice(0, -3) : key
}

function handleSwap(upstream, server) {
  // 找出同 upstream 中状态相反的 server
  const opposite = upstream.servers.find(s => s.status !== server.status)
  if (!opposite) return

  // 确定 offlineIP 和 onlineIP
  const offlineIP = server.status === 'up' ? normalizeIPKey(server) : normalizeIPKey(opposite)
  const onlineIP = server.status === 'up' ? normalizeIPKey(opposite) : normalizeIPKey(server)

  swapOfflineIP.value = offlineIP
  swapOnlineIP.value = onlineIP

  // 扫描所有 upstream，找到同时包含这两个 IP 且状态正确的组
  const affected = []
  for (const u of upstreams.value) {
    let hasOffline = false
    let hasOnline = false
    for (const s of u.servers) {
      const key = normalizeIPKey(s)
      if (key === offlineIP && s.status === 'up') hasOffline = true
      if (key === onlineIP && s.status === 'down') hasOnline = true
    }
    if (hasOffline && hasOnline) {
      affected.push({
        name: u.name,
        totalCount: u.servers.length,
        upCount: u.servers.filter(s => s.status === 'up').length,
        downCount: u.servers.filter(s => s.status === 'down').length,
        checked: true
      })
    }
  }

  swapAffectedUpstreams.value = affected
  swapDialogVisible.value = true
}

async function confirmSwap() {
  const selectedUpstreams = swapAffectedUpstreams.value.filter(i => i.checked).map(i => i.name)
  if (selectedUpstreams.length === 0) return
  swapDialogVisible.value = false

  try {
    const res = await nginxSwapPreview({
      server_id: serverId.value,
      config_file: configFile.value,
      upstream_names: selectedUpstreams,
      offline_ip: swapOfflineIP.value,
      online_ip: swapOnlineIP.value
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
  const upCount = upstream.servers.filter(s => s.status === 'up').length
  const downCount = upstream.servers.filter(s => s.status === 'down').length
  try {
    await ElMessageBox.confirm(
      `将反转 ${upstream.name} 中所有服务器状态：${upCount} 台 up → down，${downCount} 台 down → up。确认执行？`,
      '切换确认',
      { confirmButtonText: '确认', cancelButtonText: '取消', type: 'warning' }
    )
  } catch { return }

  try {
    const res = await nginxTogglePreview({
      server_id: serverId.value,
      config_file: configFile.value,
      upstream_names: [upstream.name]
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

function openBatchDialog() {
  // 列出所有 upstream 组，让用户选择操作类型
  const items = upstreams.value.map(u => {
    const upServers = u.servers.filter(s => s.status === 'up')
    const downServers = u.servers.filter(s => s.status === 'down')
    const hasBoth = upServers.length > 0 && downServers.length > 0
    const hasMultipleUp = upServers.length >= 2

    // 默认操作：有 up 和 down 的默认 toggle，只有 up 的默认不操作
    let defaultAction = ''
    if (hasBoth) defaultAction = 'toggle'

    return {
      upstreamName: u.name,
      enabled: false,
      action: defaultAction,
      backendIPs: [],
      servers: u.servers,
      upCount: upServers.length,
      downCount: downServers.length,
      hasBoth,
      hasMultipleUp
    }
  })
  batchItems.value = items
  batchDialogVisible.value = true
}

const isBatchAllSelected = computed(() => {
  const eligible = batchItems.value.filter(i => i.hasBoth || i.hasMultipleUp)
  return eligible.length > 0 && eligible.every(i => i.enabled)
})

function toggleBatchSelectAll() {
  const newState = !isBatchAllSelected.value
  batchItems.value.forEach(i => {
    if (i.hasBoth || i.hasMultipleUp) {
      i.enabled = newState
    }
  })
}

const batchValidCount = computed(() => {
  return batchItems.value.filter(i => {
    if (!i.enabled || !i.action) return false
    if (i.action === 'toggle') return true
    return i.backendIPs && i.backendIPs.length > 0
  }).reduce((sum, i) => {
    if (i.action === 'toggle') return sum + 1
    return sum + i.backendIPs.length
  }, 0)
})

function getAvailableActions(item) {
  const actions = []
  if (item.hasBoth) {
    actions.push({ label: '切换（反转全部）', value: 'toggle' })
  }
  if (item.hasMultipleUp) {
    actions.push({ label: '下线', value: 'offline' })
  }
  if (item.downCount > 0) {
    actions.push({ label: '上线', value: 'online' })
  }
  return actions
}

function getSelectableServers(item) {
  if (item.action === 'online') {
    return item.servers.filter(s => s.status === 'down')
  } else if (item.action === 'offline') {
    return item.servers.filter(s => s.status === 'up')
  }
  return []
}

// 获取下线操作时允许全选的服务器（排除第一台，确保至少保留一台在线）
function getOfflineSafeServers(item) {
  const upServers = item.servers.filter(s => s.status === 'up')
  return upServers.slice(1)
}

function isBatchIPAllSelected(item) {
  const selectable = getSelectableServers(item)
  if (selectable.length === 0) return false
  if (item.action === 'offline') {
    const safe = getOfflineSafeServers(item)
    return safe.length > 0 && safe.every(s => item.backendIPs.includes(normalizeIPKey(s)))
  }
  return selectable.every(s => item.backendIPs.includes(normalizeIPKey(s)))
}

function toggleBatchIPSelectAll(item) {
  if (isBatchIPAllSelected(item)) {
    item.backendIPs = []
  } else {
    if (item.action === 'offline') {
      // 下线全选：排除第一台在线服务器
      item.backendIPs = getOfflineSafeServers(item).map(s => normalizeIPKey(s))
    } else {
      item.backendIPs = getSelectableServers(item).map(s => normalizeIPKey(s))
    }
  }
}

function onBatchActionChange(row) {
  row.backendIPs = []
}

async function executeBatch() {
  // 校验：下线操作不能将所有在线服务器下线
  for (const i of batchItems.value) {
    if (!i.enabled || i.action !== 'offline') continue
    const upServers = i.servers.filter(s => s.status === 'up')
    if (i.backendIPs.length >= upServers.length) {
      ElMessage.error(`upstream [${i.upstreamName}] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`)
      return
    }
  }

  // 展开多选 IP：每个 IP 生成一个独立的 item
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
    const res = await nginxBatchPreview({
      server_id: serverId.value,
      config_file: configFile.value,
      items
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'batch'
    currentBatchInfo.value = {
      upstreamNames: items.map(i => i.upstream_name),
      ipCount: items.length,
      action: 'batch'
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
      backend_ip: ips.join(',')
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

const currentBatchInfo = ref(null)

async function handleRollbackFromDialog(backupFile) {
  try {
    await ElMessageBox.confirm(
      `确定要回滚到备份文件吗？\n\n文件：${backupFile}\n时间：${parseBackupTime(backupFile)}`,
      '回滚确认',
      { confirmButtonText: '确认回滚', cancelButtonText: '取消', type: 'warning' }
    )
    backupDialogVisible.value = false
    await handleRollback(backupFile)
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '预览失败')
    }
  }
}

async function handleRollback(backupFile) {
  try {
    const res = await nginxRollbackPreview({
      server_id: serverId.value,
      config_file: configFile.value,
      backup_file: backupFile
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'rollback'
    currentBatchInfo.value = null
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

const ACTION_META = {
  online:  { type: 'success', label: '批量上线' },
  offline: { type: 'danger',  label: '批量下线' },
  swap:    { type: 'warning', label: '切换' },
  toggle:  { type: 'warning', label: '组切换' },
  batch:   { type: 'warning', label: '批量操作' },
  rollback:{ type: 'info',    label: '回滚' }
}

function getNowStr() {
  const d = new Date()
  const pad = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
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
    rollback: nginxRollbackExecute
  }[action]

  const meta = ACTION_META[action] || { type: 'info', label: action }

  try {
    const res = await executeFn({ preview_id: previewId.value })
    const msg = res.output || res.message || '执行成功'
    output.value = msg
    outputMeta.value = {
      actionType: meta.type,
      actionLabel: meta.label,
      upstreamNames: currentBatchInfo.value?.upstreamNames || [],
      ipCount: currentBatchInfo.value?.ipCount || 0,
      success: true,
      time: getNowStr()
    }
    previewVisible.value = false
    ElMessage.success('执行成功')

    await loadUpstreams()
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
      time: getNowStr()
    }
  } finally {
    executing.value = false
  }
}
</script>

<style scoped>
/* ===== Page ===== */
.nginx-page {
  padding: 2px;
}

/* ===== Main Card ===== */
.main-card {
  border: none;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  overflow-y: auto;
  max-height: calc(100vh - 100px);
}

:deep(.el-card__header) {
  border-bottom: none;
  padding-bottom: 0;
}

/* ===== Header Row ===== */
.header-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-field {
  display: flex;
  align-items: center;
  gap: 8px;
}

.field-label {
  font-size: 14px;
  font-weight: 600;
  color: #606266;
  white-space: nowrap;
}

/* ===== Stats Bar ===== */
.stats-bar {
  display: flex;
  gap: 0;
  margin-top: 10px;
  margin-bottom: 0;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
}

.stat-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border-right: 1px solid #e4e7ed;
}

.stat-item:last-child {
  border-right: none;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  line-height: 1.2;
}

.stat-value.stat-success { color: #67c23a; }
.stat-value.stat-danger { color: #f56c6c; }
.stat-value.stat-primary { color: #409eff; }

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

/* ===== Toolbar ===== */
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 0;
  padding: 10px 14px;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  flex-wrap: wrap;
}

.search-input {
  flex: 1;
  min-width: 200px;
}

:deep(.search-input .el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px #dcdfe6 inset;
  transition: box-shadow 0.2s;
}

:deep(.search-input .el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #c0c4cc inset;
}

:deep(.search-input .el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.selected-count {
  font-size: 13px;
  color: #909399;
  white-space: nowrap;
  padding: 0 4px;
}

/* ===== Upstream Collapse ===== */
.upstream-collapse {
  border: none;
}

:deep(.upstream-collapse .el-collapse-item) {
  margin-bottom: 10px;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid #e4e7ed;
  background: #fff;
  transition: box-shadow 0.2s, border-color 0.2s;
}

:deep(.upstream-collapse .el-collapse-item:hover) {
  border-color: #c0c4cc;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

:deep(.upstream-collapse .el-collapse-item__header) {
  background: #fafbfc;
  border-bottom: 1px solid transparent;
  padding: 0 16px;
  height: 48px;
  line-height: 48px;
  font-size: 14px;
  transition: background 0.2s;
}

:deep(.upstream-collapse .el-collapse-item.is-active .el-collapse-item__header) {
  border-bottom-color: #e4e7ed;
}

:deep(.upstream-collapse .el-collapse-item__wrap) {
  border-bottom: none;
}

:deep(.upstream-collapse .el-collapse-item__content) {
  padding: 0;
}

.upstream-header {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.upstream-name {
  font-weight: 700;
  color: #303133;
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
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.badge-info {
  background: #ecf5ff;
  color: #409eff;
}

.badge-success {
  background: #f0f9eb;
  color: #67c23a;
}

.badge-danger {
  background: #fef0f0;
  color: #f56c6c;
}

/* ===== Server Table ===== */
.server-table {
  border-radius: 0;
}

@media (max-width: 768px) {
  .server-table {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }

  :deep(.server-table .el-table__inner-wrapper) {
    min-width: 460px;
  }

  .batch-table {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }

  :deep(.batch-table .el-table__inner-wrapper) {
    min-width: 600px;
  }
}

:deep(.server-table .el-table__header th) {
  background: #f5f7fa !important;
  color: #606266;
  font-weight: 600;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

:deep(.server-table .el-table__row) {
  transition: background-color 0.15s;
}

:deep(.server-table .el-table__row:hover > td) {
  background-color: #f5f7fa !important;
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
  background: #67c23a;
  box-shadow: 0 0 6px rgba(103, 194, 58, 0.5);
}

.status-down {
  background: #f56c6c;
  box-shadow: 0 0 6px rgba(245, 108, 108, 0.4);
}

:deep(.selected-row) {
  background-color: #ecf5ff !important;
}

:deep(.selected-row:hover > td) {
  background-color: #d9ecff !important;
}

/* ===== Empty State ===== */
.empty-state {
  text-align: center;
  padding: 40px 0;
  color: #909399;
  font-size: 14px;
}

/* ===== Dialogs ===== */
:deep(.cool-dialog .el-dialog) {
  border-radius: 12px;
  overflow: hidden;
}

:deep(.cool-dialog .el-dialog__header) {
  background: linear-gradient(135deg, #f5f7fa 0%, #e8ecf1 100%);
  padding: 16px 20px;
  margin: 0;
}

:deep(.cool-dialog .el-dialog__title) {
  font-weight: 700;
  font-size: 15px;
}

:deep(.cool-dialog .el-dialog__body) {
  padding: 20px;
}

.preview-desc {
  font-size: 14px;
  color: #303133;
  margin-bottom: 16px;
  padding: 10px 14px;
  background: #f5f7fa;
  border-radius: 8px;
  border-left: 3px solid #409eff;
}

/* ===== Diff Viewer ===== */
.diff-container {
  border: 1px solid #383838;
  border-radius: 8px;
  overflow: hidden;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: 13px;
}

.diff-header {
  background: #2d2d2d;
  padding: 10px 16px;
  border-bottom: 1px solid #383838;
}

.diff-filename {
  color: #e6e6e6;
  font-weight: 600;
  font-size: 13px;
}

.diff-body {
  max-height: 500px;
  overflow-y: auto;
  background: #1e1e1e;
}

.diff-line {
  display: flex;
  padding: 1px 0;
  line-height: 1.6;
}

.diff-line:hover {
  background: rgba(255, 255, 255, 0.04);
}

.diff-line-num {
  width: 50px;
  text-align: right;
  padding-right: 12px;
  color: #636d83;
  user-select: none;
  flex-shrink: 0;
  font-size: 12px;
}

.diff-line-prefix {
  width: 24px;
  text-align: center;
  user-select: none;
  flex-shrink: 0;
  font-weight: 700;
}

.diff-line-content {
  flex: 1;
  white-space: pre;
  overflow-x: auto;
  padding-right: 16px;
}

.diff-same {
  background: transparent;
}

.diff-same .diff-line-content {
  color: #abb2bf;
}

.diff-added {
  background: rgba(72, 199, 111, 0.1);
}

.diff-added .diff-line-prefix {
  color: #48c76f;
}

.diff-added .diff-line-content {
  color: #a8dab5;
}

.diff-removed {
  background: rgba(245, 108, 108, 0.1);
}

.diff-removed .diff-line-prefix {
  color: #f56c6c;
}

.diff-removed .diff-line-content {
  color: #e8a0a0;
}

/* ===== Terminal Pre ===== */
.terminal-pre {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 8px;
  max-height: 400px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}

.terminal-lg {
  max-height: 600px;
}

/* ===== Nginx Syntax Highlighting ===== */
:deep(.hl-comment) {
  color: #6a9955;
  font-style: italic;
}

:deep(.hl-directive) {
  color: #569cd6;
  font-weight: 600;
}

:deep(.hl-brace) {
  color: #ffd700;
  font-weight: 700;
}

:deep(.hl-ip) {
  color: #ce9178;
}

:deep(.hl-semicolon) {
  color: #808080;
}

/* ===== Backup Table ===== */
:deep(.backup-table .el-table__header th) {
  background: #f5f7fa !important;
  font-weight: 600;
}

/* ===== Output Area ===== */
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
  background: #f5f7fa;
  border-radius: 8px;
  border: 1px solid #e4e7ed;
}

.output-meta-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.output-meta-label {
  font-size: 13px;
  color: #909399;
  white-space: nowrap;
}

.output-meta-value {
  font-size: 13px;
  color: #303133;
  font-weight: 600;
}

.output-meta-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

:deep(.el-loading-mask) {
  border-radius: 8px;
}

/* ===== Swap Dialog ===== */
.swap-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.swap-ip-pair {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.swap-label {
  font-size: 13px;
  color: #606266;
  font-weight: 500;
  margin-bottom: 10px;
}

.swap-arrow {
  font-size: 28px;
  color: #409eff;
  font-weight: 700;
  line-height: 1;
}

.swap-upstream-list {
  max-height: 400px;
  overflow-y: auto;
}

.swap-upstream-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  margin-bottom: 8px;
  transition: background 0.15s;
}

.swap-upstream-item:hover {
  background: #f5f7fa;
}

.swap-upstream-item .upstream-name {
  font-weight: 600;
  color: #303133;
  font-size: 14px;
}

/* ===== Batch Dialog ===== */
.batch-dialog-body {
  max-height: 550px;
  overflow: auto;
  -webkit-overflow-scrolling: touch;
}

.batch-hint {
  display: flex;
  align-items: center;
  font-size: 13px;
  color: #909399;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
}

.batch-table {
  width: 100%;
}

.batch-empty {
  text-align: center;
  color: #909399;
  padding: 30px 0;
  font-size: 14px;
}

.batch-offline-hint {
  font-size: 11px;
  color: #e6a23c;
  margin-top: 2px;
  line-height: 1.2;
}

/* ===== Upstream Toggle Button ===== */
.upstream-toggle-btn {
  margin-left: auto;
  margin-right: 8px;
}
</style>
