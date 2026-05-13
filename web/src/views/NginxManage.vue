<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span>Nginx管理</span>
          <div style="display: flex; gap: 10px;">
            <el-select v-model="serverId" placeholder="选择Nginx服务器" style="width: 200px" @change="loadConfigs">
              <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
            </el-select>
            <el-select v-model="configFile" placeholder="选择配置文件" style="width: 250px" @change="loadUpstreams">
              <el-option v-for="f in configFiles" :key="f" :label="f" :value="f" />
            </el-select>
            <el-button type="warning" @click="handleReload">Reload</el-button>
          </div>
        </div>
      </template>

      <!-- 搜索框 -->
      <el-input v-model="filterKeyword" placeholder="过滤 upstream、IP 或端口" clearable style="margin-bottom: 10px;" />

      <!-- 操作栏 -->
      <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 15px; padding: 8px 12px; background: #f5f7fa; border-radius: 4px;">
        <span style="margin-right: 8px;">已选择: {{ selectedServers.length }} 个后端</span>
        <el-button size="small" @click="toggleExpandAll">{{ allExpanded ? '全部折叠' : '全部展开' }}</el-button>
        <el-button size="small" :type="isAllSelected ? 'warning' : 'primary'" @click="toggleSelectAll">{{ isAllSelected ? '取消全选' : '全选后端' }}</el-button>
        <el-button type="success" size="small" :disabled="selectedServers.length === 0" @click="handleBatchOnline">批量上线</el-button>
        <el-button type="danger" size="small" :disabled="selectedServers.length === 0" @click="handleBatchOffline">批量下线</el-button>
        <el-button size="small" type="info" @click="backupDialogVisible = true">备份列表</el-button>
        <el-button size="small" :disabled="!output" @click="outputDialogVisible = true">执行结果</el-button>
        <el-button size="small" @click="clearSelection">清除选择</el-button>
      </div>

      <el-collapse v-model="expandedUpstreams">
        <el-collapse-item v-for="upstream in filteredUpstreams" :key="upstream.name" :name="upstream.name">
          <template #title>
            <span style="font-weight: bold; margin-right: 12px;">{{ upstream.name }}</span>
            <el-tag size="small" type="info">{{ upstream.servers.length }} 台</el-tag>
            <el-tag size="small" type="success" style="margin-left: 6px;">{{ upstream.servers.filter(s => s.status === 'up').length }} up</el-tag>
            <el-tag size="small" type="danger" style="margin-left: 6px;">{{ upstream.servers.filter(s => s.status === 'down').length }} down</el-tag>
          </template>
          <el-table :data="upstream.servers" stripe size="small" :row-class-name="({ row }) => isServerSelected(upstream.name, row) ? 'selected-row' : ''">
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
                <el-tag :type="row.status === 'up' ? 'success' : 'danger'">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="weight" label="权重" width="80" />
          </el-table>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- Backup Dialog -->
    <el-dialog v-model="backupDialogVisible" title="备份列表" width="600px">
      <el-table :data="backups" stripe size="small" max-height="400">
        <el-table-column label="文件名">
          <template #default="{ row }">{{ row }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="warning" size="small" @click="handleRollbackFromDialog(row)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="90%" top="5vh">
      <div v-if="previewData">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <el-divider />

        <!-- 统一 diff 视图 -->
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
    <el-dialog v-model="outputDialogVisible" title="执行结果" width="600px">
      <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 4px; max-height: 400px; overflow-y: auto;">{{ output }}</pre>
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

// 选中的服务器 { upstreamName: Set("ip:port", ...) }
const selectedMap = ref({})

// 辅助：获取 server 的唯一 key
function serverKey(s) {
  return s.port ? `${s.ip}:${s.port}` : s.ip
}

// 判断某个 server 是否被选中
function isServerSelected(upstreamName, server) {
  const set = selectedMap.value[upstreamName]
  return set ? set.has(serverKey(server)) : false
}

// 单个 server 切换选中
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

// 判断某个 upstream 是否全选
function isUpstreamAllSelected(upstream) {
  const set = selectedMap.value[upstream.name]
  return set ? set.size === upstream.servers.length : false
}

// 整个 upstream 全选/取消
function toggleUpstreamAll(upstream, checked) {
  const newMap = { ...selectedMap.value }
  if (checked) {
    newMap[upstream.name] = new Set(upstream.servers.map(s => serverKey(s)))
  } else {
    delete newMap[upstream.name]
  }
  selectedMap.value = newMap
}

// 计算所有选中的服务器列表（供批量操作使用）
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

function clearSelection() {
  selectedMap.value = {}
}

// 过滤后的 upstream 列表（按名称或后端 IP 过滤）
const filteredUpstreams = computed(() => {
  const kw = filterKeyword.value.trim().toLowerCase()
  if (!kw) return upstreams.value
  return upstreams.value.filter(u => {
    if (u.name.toLowerCase().includes(kw)) return true
    return u.servers.some(s => s.ip.toLowerCase().includes(kw) || (s.port && s.port.includes(kw)))
  })
})

// 是否所有过滤后的 upstream 中的后端都已选中
const isAllSelected = computed(() => {
  const filtered = filteredUpstreams.value
  if (filtered.length === 0) return false
  return filtered.every(u => {
    const set = selectedMap.value[u.name]
    return set && set.size === u.servers.length
  })
})

// 全选 / 取消全选
function toggleSelectAll() {
  if (isAllSelected.value) {
    // 取消全选：只清除过滤后涉及的 upstream
    const newMap = { ...selectedMap.value }
    for (const u of filteredUpstreams.value) {
      delete newMap[u.name]
    }
    selectedMap.value = newMap
  } else {
    // 全选：将过滤后所有 upstream 的服务器加入
    const newMap = { ...selectedMap.value }
    for (const u of filteredUpstreams.value) {
      newMap[u.name] = new Set(u.servers.map(s => serverKey(s)))
    }
    selectedMap.value = newMap
  }
}

// 全部展开/折叠切换
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
  try {
    backups.value = await getNginxBackups(serverId.value)
  } catch (e) {
    console.error('加载备份列表失败:', e)
  }
}

async function loadUpstreams() {
  if (!serverId.value || !configFile.value) return
  try {
    const res = await getNginxUpstreams(serverId.value, configFile.value)
    upstreams.value = res.upstreams || []
    selectedMap.value = {}
    if (upstreams.value.length === 0 && res.raw) {
      ElMessage.warning('未解析到upstream配置，请检查配置文件格式')
    }
  } catch (e) {
    ElMessage.error('加载upstream失败: ' + (e.response?.data?.error || e.message))
  }
}

// 批量上线（选中的服务器）
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

// 批量下线（选中的服务器）
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
  // 校验：不能将某个 upstream 中所有在线服务器全部下线
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

// 批量操作（upstream 数组 + IP 数组）
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
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleReload() {
  try {
    await ElMessageBox.confirm('确定要 reload Nginx 吗？', '确认')
    await nginxReload({ server_id: serverId.value })
    ElMessage.success('reload 成功')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || 'reload 失败')
    }
  }
}

async function handleRollbackFromDialog(backupFile) {
  backupDialogVisible.value = false
  await handleRollback(backupFile)
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
    ElMessage.success('执行成功')
    previewVisible.value = false
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
.diff-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
}

.diff-header {
  background: #f5f7fa;
  padding: 8px 12px;
  border-bottom: 1px solid #dcdfe6;
  font-weight: bold;
}

.diff-body {
  max-height: 500px;
  overflow-y: auto;
}

.diff-line {
  display: flex;
  padding: 2px 0;
  line-height: 1.5;
}

.diff-line-num {
  width: 50px;
  text-align: right;
  padding-right: 10px;
  color: #909399;
  user-select: none;
  flex-shrink: 0;
}

.diff-line-prefix {
  width: 20px;
  text-align: center;
  user-select: none;
  flex-shrink: 0;
}

.diff-line-content {
  flex: 1;
  white-space: pre;
  overflow-x: auto;
}

.diff-same {
  background: #ffffff;
}

/* 新增行（上线操作 - 取消注释） */
.diff-added {
  background: #e6ffec;
}

.diff-added .diff-line-prefix {
  color: #22863a;
  font-weight: bold;
}

.diff-added .diff-line-content {
  color: #22863a;
}

/* 删除行（下线操作 - 添加注释） */
.diff-removed {
  background: #ffeef0;
}

.diff-removed .diff-line-prefix {
  color: #cb2431;
  font-weight: bold;
}

.diff-removed .diff-line-content {
  color: #cb2431;
}

:deep(.selected-row) {
  background-color: #ecf5ff !important;
}
:deep(.selected-row:hover > td) {
  background-color: #d9ecff !important;
}
</style>
