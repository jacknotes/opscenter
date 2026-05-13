<template>
  <div class="nginx-page">
    <el-card class="main-card">
      <template #header>
        <div class="header-row">
          <div class="header-field">
            <span class="field-label">服务器</span>
            <el-select v-model="serverId" placeholder="选择Nginx服务器" style="width: 250px" size="large" @change="loadConfigs">
              <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
            </el-select>
          </div>
          <div class="header-field">
            <span class="field-label">配置文件</span>
            <el-select v-model="configFile" placeholder="选择配置文件" style="width: 250px" size="large" @change="loadUpstreams">
              <el-option v-for="f in configFiles" :key="f" :label="f" :value="f" />
            </el-select>
          </div>
          <el-button size="large" @click="handleViewConfig">查看配置</el-button>
          <el-button size="large" type="warning" @click="handleReload">重载配置</el-button>
        </div>
      </template>

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
      </div>

      <!-- Search + Toolbar -->
      <div class="toolbar">
        <el-input v-model="filterKeyword" placeholder="过滤 upstream、IP 或端口" clearable class="search-input" size="large">
          <template #prefix><span style="opacity: 0.5;">&#128269;</span></template>
        </el-input>
        <el-button size="large" @click="toggleExpandAll">{{ allExpanded ? '全部折叠' : '全部展开' }}</el-button>
        <span class="selected-count">已选择 {{ selectedServers.length }} 个后端</span>
        <el-button size="large" :type="isAllSelected ? 'warning' : 'primary'" @click="toggleSelectAll">{{ isAllSelected ? '取消全选' : '全选' }}</el-button>
        <el-button size="large" type="success" :disabled="selectedServers.length === 0" @click="handleBatchOnline">批量上线</el-button>
        <el-button size="large" type="danger" :disabled="selectedServers.length === 0" @click="handleBatchOffline">批量下线</el-button>
        <el-button size="large" type="info" @click="backupDialogVisible = true">备份列表</el-button>
        <el-button size="large" :disabled="!output" @click="outputDialogVisible = true">执行结果</el-button>
        <el-button size="large" type="primary" @click="handleRefresh">刷新</el-button>
      </div>

      <!-- Upstream Groups -->
      <div v-loading="loadingUpstreams">
        <el-collapse v-model="expandedUpstreams" class="upstream-collapse">
          <el-collapse-item v-for="upstream in filteredUpstreams" :key="upstream.name" :name="upstream.name" class="upstream-item">
            <template #title>
              <div class="upstream-header">
                <span class="upstream-name">{{ upstream.name }}</span>
                <div class="upstream-badges">
                  <span class="badge badge-info">{{ upstream.servers.length }} 台</span>
                  <span class="badge badge-success">{{ upstream.servers.filter(s => s.status === 'up').length }} up</span>
                  <span class="badge badge-danger">{{ upstream.servers.filter(s => s.status === 'down').length }} down</span>
                </div>
              </div>
            </template>
            <el-table :data="upstream.servers" size="small" :row-class-name="({ row }) => isServerSelected(upstream.name, row) ? 'selected-row' : ''" class="server-table">
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
      <el-table :data="backupList" size="small" max-height="400" v-loading="loadingBackups" class="backup-table">
        <el-table-column label="文件名" min-width="200">
          <template #default="{ row }">{{ row.name }}</template>
        </el-table-column>
        <el-table-column label="备份时间" width="180">
          <template #default="{ row }">{{ row.time }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="warning" size="small" @click="handleRollbackFromDialog(row.name)">回滚</el-button>
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

    <!-- Output Dialog -->
    <el-dialog v-model="outputDialogVisible" title="执行结果" width="600px" class="cool-dialog">
      <pre class="terminal-pre">{{ output }}</pre>
    </el-dialog>

    <!-- Config Viewer Dialog -->
    <el-dialog v-model="configDialogVisible" :title="'配置文件 - ' + configFile" width="80%" top="5vh" class="cool-dialog">
      <pre class="terminal-pre terminal-lg" v-html="highlightedConfig"></pre>
    </el-dialog>

    <!-- Operation Result Dialog -->
    <el-dialog v-model="resultDialogVisible" title="操作完成" width="500px" class="cool-dialog">
      <div v-if="resultData" class="result-body">
        <div class="result-item">
          <span class="result-label">操作</span>
          <el-tag :type="resultData.action === 'online' ? 'success' : 'danger'" size="large">
            {{ resultData.action === 'online' ? '批量上线' : '批量下线' }}
          </el-tag>
        </div>
        <div class="result-item">
          <span class="result-label">Upstream 组</span>
          <span class="result-value">{{ resultData.upstreamNames.length }} 个</span>
        </div>
        <div class="result-item">
          <span class="result-label">涉及 Upstream</span>
          <div class="result-tags">
            <el-tag v-for="name in resultData.upstreamNames" :key="name" size="small" type="info">{{ name }}</el-tag>
          </div>
        </div>
        <div class="result-item">
          <span class="result-label">后端 IP</span>
          <span class="result-value">{{ resultData.ipCount }} 个</span>
        </div>
        <div class="result-item">
          <span class="result-label">状态</span>
          <el-tag type="success" size="large">执行成功</el-tag>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  getServers, getNginxConfigs, getNginxUpstreams,
  nginxOnlinePreview, nginxOnlineExecute,
  nginxOfflinePreview, nginxOfflineExecute,
  nginxReload, nginxRollbackPreview, nginxRollbackExecute,
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
const outputDialogVisible = ref(false)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')
const expandedUpstreams = ref([])
const filterKeyword = ref('')
const rawConfig = ref('')
const configDialogVisible = ref(false)
const loadingUpstreams = ref(false)
const loadingBackups = ref(false)
const resultDialogVisible = ref(false)
const resultData = ref(null)

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
  return rawConfig.value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
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
  if (!kw) return upstreams.value
  return upstreams.value.filter(u => {
    if (u.name.toLowerCase().includes(kw)) return true
    return u.servers.some(s => s.ip.toLowerCase().includes(kw) || (s.port && s.port.includes(kw)))
  })
})

const totalUpCount = computed(() => {
  return filteredUpstreams.value.reduce((sum, u) => sum + u.servers.filter(s => s.status === 'up').length, 0)
})

const totalDownCount = computed(() => {
  return filteredUpstreams.value.reduce((sum, u) => sum + u.servers.filter(s => s.status === 'down').length, 0)
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
    servers.value = await getServers('nginx')
    if (servers.value.length > 0) {
      serverId.value = servers.value[0].id
      await loadConfigs()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function loadConfigs() {
  if (!serverId.value) return
  try {
    configFiles.value = await getNginxConfigs(serverId.value)
    if (configFiles.value.length > 0) {
      configFile.value = configFiles.value[0]
      await loadUpstreams()
    }
  } catch (e) {
    ElMessage.error('加载配置列表失败: ' + (e.response?.data?.error || e.message))
  }
  loadingBackups.value = true
  try {
    backups.value = await getNginxBackups(serverId.value)
  } catch (e) {
    console.error('加载备份列表失败:', e)
  } finally {
    loadingBackups.value = false
  }
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

async function handleReload() {
  try {
    await ElMessageBox.confirm('确定要重载 Nginx 配置吗？', '确认')
    await nginxReload({ server_id: serverId.value })
    ElMessage.success('重载配置成功')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '重载配置失败')
    }
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

async function executePreview() {
  executing.value = true
  const executeFn = {
    online: nginxOnlineExecute,
    offline: nginxOfflineExecute,
    rollback: nginxRollbackExecute
  }[currentAction.value]

  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = res.output || res.message
    previewVisible.value = false

    if (currentBatchInfo.value && currentAction.value !== 'rollback') {
      resultData.value = currentBatchInfo.value
      resultDialogVisible.value = true
    } else {
      ElMessage.success('执行成功')
    }

    await loadUpstreams()
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = e.response?.data?.output || ''
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
  overflow: hidden;
}

:deep(.el-card__header) {
  border-bottom: none;
  padding-bottom: 0;
  background: linear-gradient(135deg, #f5f7fa 0%, #e8ecf1 100%);
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
  margin-bottom: 16px;
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
  margin-bottom: 16px;
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

/* ===== Result Dialog ===== */
.result-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.result-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.result-label {
  min-width: 80px;
  font-size: 13px;
  color: #909399;
  font-weight: 500;
  line-height: 32px;
}

.result-value {
  font-size: 14px;
  color: #303133;
  font-weight: 600;
  line-height: 32px;
}

.result-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

:deep(.el-loading-mask) {
  border-radius: 8px;
}
</style>
