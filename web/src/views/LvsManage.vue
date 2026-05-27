<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap;">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择LVS服务器" style="width: 150px" @change="onServerChange">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span class="filter-label">虚拟服务器:</span>
          <el-select v-model="vsFilter" placeholder="全部" clearable filterable style="width: 150px">
            <el-option v-for="ip in vsOptions" :key="ip" :label="ip" :value="ip" />
          </el-select>
        </div>
      </template>

      <!-- 操作栏 -->
      <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap;">
        <el-button type="info" class="el-button--cyan" @click="toggleExpandAll">{{ allExpanded ? '折叠' : '展开' }}</el-button>
        <el-button type="info" class="el-button--cyan" @click="toggleAllFiltered">{{ isAllFilteredSelected ? '取消' : '全选' }}</el-button>
        <el-button type="primary" :disabled="!canBatchOnline" @click="handleBatchOnline">上线</el-button>
        <el-button type="danger" :disabled="!canBatchOffline" @click="handleBatchOffline">下线</el-button>
        <el-button type="primary" :disabled="!canSwap" @click="handleSwap">切换</el-button>
        <el-button type="success" @click="loadStatus" :loading="statusLoading">查看状态</el-button>
        <el-button type="info" class="el-button--cyan" @click="loadData" :loading="loading">刷新</el-button>
        <span style="margin-left: auto;"></span>
        <span class="stat-chip stat-chip-primary">已选 <b>{{ batchSelectedIPs.length }}</b></span>
      </div>

      <!-- 主表格：按 VIP 分组，每端口一行，展开后 RS 表格插入在组下方 -->
      <el-table :data="flattenedMainData" :span-method="mainSpanMethod" stripe border v-force-reflow max-height="calc(100vh - 240px)" row-key="uid">
        <el-table-column label="" width="45" align="center">
          <template #default="{ row }">
            <template v-if="row.isDetail">
              <el-table :data="getRSView(row.group).data" :span-method="getRSView(row.group).spanMethod" stripe size="small" style="width: 100%;">
                <el-table-column label="" width="45" align="center">
                  <template #header>
                    <el-checkbox :model-value="isAllSelected(row.group)" :indeterminate="isIndeterminate(row.group)" @change="(val) => toggleSelectAll(row.group, val)" />
                  </template>
                  <template #default="{ row: rs }">
                    <el-checkbox :model-value="isBatchSelected(row.group.ip, rs.ip)" @change="(val) => toggleBatch(row.group.ip, rs.ip, val)" />
                  </template>
                </el-table-column>
                <el-table-column label="Real Server" min-width="130">
                  <template #default="{ row: rs }">{{ rs.ip }}</template>
                </el-table-column>
                <el-table-column label="环境" width="120" align="center">
                  <template #default="{ row: rs }">
                    <el-tag
                      v-if="rs.tag"
                      :type="rs.tag.includes('生产') && !rs.tag.includes('预生产') ? 'danger' : 'warning'"
                      size="small"
                      style="cursor: pointer;"
                      @click="openTagDialog(rs.ip, rs.tag)"
                    >{{ rs.tag }}</el-tag>
                    <el-button
                      v-else
                      type="info"
                      link
                      size="small"
                      @click="openTagDialog(rs.ip, '')"
                    >设置标签</el-button>
                  </template>
                </el-table-column>
                <el-table-column label="端口" width="80" align="center">
                  <template #default="{ row: rs }">{{ rs.port }}</template>
                </el-table-column>
                <el-table-column label="状态" width="80" align="center">
                  <template #default="{ row: rs }">
                    <el-tag :type="rs.status === 'up' ? 'success' : 'danger'" size="small">{{ rs.status }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="转发" width="80" align="center">
                  <template #default="{ row: rs }">{{ rs.forward }}</template>
                </el-table-column>
                <el-table-column label="Weight" width="80" align="center">
                  <template #default="{ row: rs }">{{ rs.weight }}</template>
                </el-table-column>
                <el-table-column label="ActiveConn" width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.activeConn }}</template>
                </el-table-column>
                <el-table-column label="InActConn" width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.inactConn }}</template>
                </el-table-column>
              </el-table>
            </template>
            <span v-else-if="row.isFirst" style="cursor: pointer; font-size: 14px;" @click="toggleVIP(row.ip)">
              {{ expandedVIPs.has(row.ip) ? '−' : '+' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Virtual Server" width="140">
          <template #default="{ row }">
            <div v-if="!row.isDetail" style="font-weight: bold;">{{ row.ip }}</div>
          </template>
        </el-table-column>
        <el-table-column label="端口" width="80" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column label="调度算法" width="90" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.scheduler }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Flags" min-width="150">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.flags }}</span>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="70" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.protocol }}</span>
          </template>
        </el-table-column>
        <el-table-column label="RS 数量" width="80" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.rsCount }}</span>
          </template>
        </el-table-column>
        <el-table-column label="在线" width="60" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail" style="color: #67c23a; font-weight: bold;">{{ row.upCount }}</span>
          </template>
        </el-table-column>
        <el-table-column label="离线" width="60" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail" style="color: #f56c6c; font-weight: bold;">{{ row.downCount }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" :width="batchPreviews.length > 1 ? '800px' : '600px'">
      <div v-if="previewData">
        <!-- 批量操作：汇总表格 -->
        <div v-if="batchPreviews.length > 1">
          <p style="color: #e6a23c; margin-bottom: 12px;">
            共 {{ batchPreviews.length }} 个操作将依次执行
          </p>
          <el-table :data="batchPreviews" stripe size="small" border max-height="300">
            <el-table-column type="index" label="#" width="50" />
            <el-table-column label="操作" min-width="300">
              <template #default="{ row }">{{ row.description }}</template>
            </el-table-column>
            <el-table-column label="命令" min-width="250">
              <template #default="{ row }"><code style="font-size: 12px;">{{ row.command }}</code></template>
            </el-table-column>
          </el-table>
        </div>
        <!-- 单个操作 -->
        <div v-else>
          <p><strong>操作：</strong>{{ previewData.description }}</p>
          <p><strong>命令：</strong><code>{{ previewData.command }}</code></p>
        </div>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>执行结果</template>
      <pre class="terminal-pre">{{ output }}</pre>
    </el-card>

    <!-- Status Dialog -->
    <el-dialog v-model="statusVisible" title="Keepalived 配置状态" width="900px">
      <div v-if="statusGroups.length > 0" style="max-height: 600px; overflow-y: auto; display: flex; flex-wrap: wrap; gap: 16px;">
        <div v-for="group in statusGroups" :key="group.vs_ip + ':' + group.vs_port" style="width: calc(50% - 8px); box-sizing: border-box;">
          <div style="font-weight: bold; font-size: 14px; margin-bottom: 8px; padding-bottom: 4px; border-bottom: 2px solid #409eff; color: #303133;">
            {{ group.vs_ip }}:{{ group.vs_port }}
          </div>
          <el-table :data="group.real_servers" stripe size="small" border>
            <el-table-column prop="ip" label="Real Server IP" min-width="120" />
            <el-table-column prop="port" label="端口" width="65" align="center" />
            <el-table-column label="状态" width="70" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'up' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
      <div v-else>
        <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px; max-height: 500px; overflow-y: auto; font-size: 13px;">{{ statusRaw }}</pre>
      </div>
    </el-dialog>

    <!-- Tag Edit Dialog -->
    <el-dialog v-model="tagDialogVisible" title="设置环境标签" width="400px">
      <el-form label-width="80px">
        <el-form-item label="RS IP">
          <el-input :model-value="tagForm.rs_ip" disabled />
        </el-form-item>
        <el-form-item label="环境标签">
          <el-select
            v-model="tagForm.tag"
            filterable
            allow-create
            clearable
            placeholder="选择或输入标签"
            style="width: 100%"
          >
            <el-option v-for="opt in tagOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button v-if="tagForm.tag" type="danger" @click="handleDeleteTag" :loading="tagSaving">删除标签</el-button>
        <span style="flex: 1;"></span>
        <el-button @click="tagDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveTag" :loading="tagSaving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { getServers, getLvsList, getLvsStatus, lvsOpPreview, lvsOpExecute, lvsSwapPreview, lvsSwapExecute, updateLvsTag, deleteLvsTag } from '../api'
import { ElMessage } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const lvsData = ref([])
const groupedData = ref([])
const vsFilter = ref(localStorage.getItem('lvs_vs_filter') || '')
watch(vsFilter, (val) => {
  if (val) {
    localStorage.setItem('lvs_vs_filter', val)
  } else {
    localStorage.removeItem('lvs_vs_filter')
  }
})
const allExpanded = ref(false)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')
const batchPreviews = ref([])
const statusVisible = ref(false)
const statusGroups = ref([])
const statusRaw = ref('')
const statusLoading = ref(false)
const loading = ref(false)
const batchSelected = ref(new Set())
const tagDialogVisible = ref(false)
const tagForm = ref({ rs_ip: '', tag: '' })
const tagSaving = ref(false)
const tagOptions = [
  { label: '生产环境', value: '生产环境' },
  { label: '预生产环境', value: '预生产环境' },
]

const batchSelectedIPs = computed(() => {
  return Array.from(batchSelected.value).map(key => {
    const idx = key.indexOf(':')
    return { vip: key.substring(0, idx), rsIp: key.substring(idx + 1) }
  })
})

// 获取选中 RS 的状态详情
const batchSelectedDetails = computed(() => {
  const details = []
  for (const { vip, rsIp } of batchSelectedIPs.value) {
    const group = groupedData.value.find(g => g.ip === vip)
    if (!group) continue
    const rs = group.realServers.find(r => r.ip === rsIp)
    if (!rs) continue
    const isFullyUp = rs.statuses.every(s => s.status === 'up')
    const isFullyDown = rs.statuses.every(s => s.status === 'down')
    details.push({ vip, rsIp, isFullyUp, isFullyDown })
  }
  return details
})

// 是否有 down 状态的 RS 被选中（用于启用批量上线）
const canBatchOnline = computed(() => batchSelectedDetails.value.some(d => d.isFullyDown))
// 是否有 up 状态的 RS 被选中（用于启用批量下线）
const canBatchOffline = computed(() => batchSelectedDetails.value.some(d => d.isFullyUp))
// 切换：精确选中 2 台 RS，同一 VIP，一台全 up 一台全 down
const canSwap = computed(() => {
  const details = batchSelectedDetails.value
  if (details.length !== 2) return false
  if (details[0].vip !== details[1].vip) return false
  const hasUp = details.some(d => d.isFullyUp) && details.some(d => d.isFullyDown)
  return hasUp
})

// 按 VIP 聚合数据
function groupByVIP(data) {
  const map = new Map()
  for (const vs of data) {
    if (!map.has(vs.ip)) {
      map.set(vs.ip, { ip: vs.ip, entries: [], realServersMap: new Map() })
    }
    const group = map.get(vs.ip)
    group.entries.push({ port: vs.port, protocol: vs.protocol, scheduler: vs.scheduler, flags: vs.flags })

    for (const rs of vs.real_servers) {
      if (!group.realServersMap.has(rs.ip)) {
        group.realServersMap.set(rs.ip, { ip: rs.ip, statuses: [], tag: rs.tag || '' })
      } else if (rs.tag) {
        group.realServersMap.get(rs.ip).tag = rs.tag
      }
      group.realServersMap.get(rs.ip).statuses.push({
        port: rs.port,
        status: rs.status,
        forward: rs.forward,
        weight: rs.weight,
        activeConn: rs.active_conn,
        inactConn: rs.inact_conn,
      })
    }
  }

  return Array.from(map.values()).map(g => ({
    ip: g.ip,
    entries: g.entries,
    realServers: Array.from(g.realServersMap.values()),
  }))
}

// 虚拟服务器选项
const vsOptions = computed(() => groupedData.value.map(g => g.ip))

// 自定义展开状态（VIP 级别）
const expandedVIPs = ref(new Set())

watch(vsFilter, (val) => {
  if (val) {
    expandedVIPs.value = new Set([val])
    allExpanded.value = true
  } else if (!allExpanded.value) {
    expandedVIPs.value = new Set()
  }
})

watch(allExpanded, (val) => {
  expandedVIPs.value = val ? new Set(filteredGroups.value.map(g => g.ip)) : new Set()
})

// 当前显示的所有 RS 是否全部选中
const isAllFilteredSelected = computed(() => {
  const allKeys = filteredGroups.value.flatMap(g => g.realServers.map(rs => g.ip + ':' + rs.ip))
  return allKeys.length > 0 && allKeys.every(k => batchSelected.value.has(k))
})

// 按虚拟服务器筛选
const filteredGroups = computed(() => {
  if (vsFilter.value) {
    return groupedData.value.filter(g => g.ip === vsFilter.value)
  }
  return groupedData.value
})

function countUp(row) {
  return row.realServers.filter(rs => rs.statuses.some(s => s.status === 'up')).length
}

function countDown(row) {
  return row.realServers.filter(rs => rs.statuses.every(s => s.status === 'down')).length
}

// 主表格：将 VIP 分组展平为每端口一行，展开时插入 RS 详情行
const flattenedMainData = computed(() => {
  const rows = []
  for (const group of filteredGroups.value) {
    const rsCount = group.realServers.length
    const upCount = countUp(group)
    const downCount = countDown(group)
    group.entries.forEach((entry, i) => {
      rows.push({ uid: group.ip + ':' + entry.port, ...entry, ip: group.ip, rsCount, upCount, downCount, isFirst: i === 0, isLast: i === group.entries.length - 1, group })
    })
    // 展开时在组末尾插入 RS 详情行
    if (expandedVIPs.value.has(group.ip)) {
      rows.push({ uid: group.ip + ':detail', ip: group.ip, isDetail: true, group })
    }
  }
  return rows
})

// 主表格合并单元格
function mainSpanMethod({ rowIndex, columnIndex }) {
  const row = flattenedMainData.value[rowIndex]
  // 详情行：合并所有列为一个单元格
  if (row.isDetail) {
    return columnIndex === 0 ? [1, 9] : [0, 0]
  }
  // 端口行：合并 expand按钮(0)、IP(1)、RS数量(6)、在线(7)、离线(8)
  if (columnIndex === 0 || columnIndex === 1 || columnIndex === 6 || columnIndex === 7 || columnIndex === 8) {
    if (!row.isFirst) return [0, 0]
    let count = 0
    let i = rowIndex
    while (i < flattenedMainData.value.length && flattenedMainData.value[i].ip === row.ip && !flattenedMainData.value[i].isDetail) {
      count++
      i++
    }
    return [count, 1]
  }
}

onMounted(async () => {
  try {
    servers.value = (await getServers('lvs')) || []
    if (servers.value.length > 0) {
      const saved = localStorage.getItem('lvs_server')
      if (saved && servers.value.some(s => s.id === Number(saved))) {
        serverId.value = Number(saved)
      } else {
        serverId.value = servers.value[0].id
      }
      await loadData()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function onServerChange() {
  localStorage.removeItem('lvs_vs_filter')
  vsFilter.value = ''
  await loadData()
}

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem('lvs_server', serverId.value)
  loading.value = true
  try {
    lvsData.value = await getLvsList(serverId.value)
    groupedData.value = groupByVIP(lvsData.value)
    invalidateRSCache()
    batchSelected.value = new Set()
    if (vsFilter.value && !groupedData.value.some(g => g.ip === vsFilter.value)) {
      vsFilter.value = ''
    }
  } catch (e) {
    ElMessage.error('加载数据失败')
  } finally {
    loading.value = false
  }
}

async function loadStatus() {
  if (!serverId.value) return
  statusLoading.value = true
  try {
    const res = await getLvsStatus(serverId.value)
    let groups = res.groups || []
    if (vsFilter.value) {
      groups = groups.filter(g => g.vs_ip === vsFilter.value)
    }
    statusGroups.value = groups
    statusRaw.value = res.output || '无数据'
    statusVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '获取状态失败')
  } finally {
    statusLoading.value = false
  }
}

// 将 RS 按端口展平为逐行数据
function flattenRS(group) {
  const rows = []
  for (const rs of group.realServers) {
    for (const s of rs.statuses) {
      rows.push({ ip: rs.ip, port: s.port, status: s.status, forward: s.forward, weight: s.weight, activeConn: s.activeConn, inactConn: s.inactConn, tag: rs.tag || '' })
    }
  }
  return rows
}

// 合并单元格：同 IP 的行只合并 checkbox 列，Real Server IP 不合并（保证表头与数据一一对应）
function spanRSMethod(flattened) {
  return ({ rowIndex, columnIndex }) => {
    if (columnIndex === 0) {
      const ip = flattened[rowIndex].ip
      if (rowIndex > 0 && flattened[rowIndex - 1].ip === ip) return [0, 0]
      let count = 1
      while (rowIndex + count < flattened.length && flattened[rowIndex + count].ip === ip) count++
      return [count, 1]
    }
    return [1, 1]
  }
}

// 缓存展平数据和对应的 span-method，避免模板中重复计算
const rsDataCache = new Map()
function invalidateRSCache() {
  rsDataCache.clear()
}
function getRSView(group) {
  const cached = rsDataCache.get(group.ip)
  if (cached) return cached
  const flat = flattenRS(group)
  const view = { data: flat, spanMethod: spanRSMethod(flat) }
  rsDataCache.set(group.ip, view)
  return view
}

function isBatchSelected(vip, rsIp) {
  return batchSelected.value.has(vip + ':' + rsIp)
}

function toggleBatch(vip, rsIp, checked) {
  const key = vip + ':' + rsIp
  const newSet = new Set(batchSelected.value)
  if (checked) {
    newSet.add(key)
  } else {
    newSet.delete(key)
  }
  batchSelected.value = newSet
}

// 获取 VIP 下所有唯一的 RS IP 列表
function getRSKeys(group) {
  return group.realServers.map(rs => group.ip + ':' + rs.ip)
}

// 全选状态
function isAllSelected(group) {
  const keys = getRSKeys(group)
  return keys.length > 0 && keys.every(k => batchSelected.value.has(k))
}

// 半选状态
function isIndeterminate(group) {
  const keys = getRSKeys(group)
  const selected = keys.filter(k => batchSelected.value.has(k))
  return selected.length > 0 && selected.length < keys.length
}

// 全选/反选
function toggleSelectAll(group, checked) {
  const keys = getRSKeys(group)
  const newSet = new Set(batchSelected.value)
  for (const k of keys) {
    if (checked) {
      newSet.add(k)
    } else {
      newSet.delete(k)
    }
  }
  batchSelected.value = newSet
}

function toggleExpandAll() {
  allExpanded.value = !allExpanded.value
}

function toggleVIP(vip) {
  const newSet = new Set(expandedVIPs.value)
  if (newSet.has(vip)) {
    newSet.delete(vip)
  } else {
    newSet.add(vip)
  }
  expandedVIPs.value = newSet
}

function toggleAllFiltered() {
  const allKeys = filteredGroups.value.flatMap(g => g.realServers.map(rs => g.ip + ':' + rs.ip))
  const allSelected = allKeys.length > 0 && allKeys.every(k => batchSelected.value.has(k))
  const newSet = new Set(batchSelected.value)
  if (allSelected) {
    for (const k of allKeys) newSet.delete(k)
  } else {
    for (const k of allKeys) newSet.add(k)
  }
  batchSelected.value = newSet
}

// 批量上线
async function handleBatchOnline() {
  const items = batchSelectedIPs.value
  if (items.length === 0) return

  // 只处理全部端口 down 的 RS，跳过已是 up 的
  const allDetails = batchSelectedDetails.value
  const targets = allDetails.filter(d => d.isFullyDown)
  if (targets.length === 0) {
    ElMessage.warning('所选服务器均已在线，无需上线')
    return
  }
  const skipped = allDetails.length - targets.length
  if (skipped > 0) {
    ElMessage.warning(`${skipped} 台服务器已在线，将自动跳过，仅对 ${targets.length} 台离线服务器执行上线`)
  }

  const byVip = {}
  for (const { vip, rsIp } of targets) {
    if (!byVip[vip]) byVip[vip] = []
    if (!byVip[vip].includes(rsIp)) byVip[vip].push(rsIp)
  }

  try {
    const previews = []
    for (const [vip, rsIps] of Object.entries(byVip)) {
      for (const rsIp of rsIps) {
        const res = await lvsOpPreview({ server_id: serverId.value, vs_ip: vip, rs_ip: rsIp, state: 'on' })
        previews.push(res)
      }
    }
    batchPreviews.value = previews
    currentAction.value = 'batch-op'
    previewData.value = previews[0]
    previewId.value = previews[0].preview_id
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

// 批量下线
async function handleBatchOffline() {
  const items = batchSelectedIPs.value
  if (items.length === 0) return

  // 只处理全部端口 up 的 RS，跳过已是 down 的
  const allDetails = batchSelectedDetails.value
  const targets = allDetails.filter(d => d.isFullyUp)
  if (targets.length === 0) {
    ElMessage.warning('所选服务器均已离线，无需下线')
    return
  }
  const byVip = {}
  for (const { vip, rsIp } of targets) {
    if (!byVip[vip]) byVip[vip] = []
    if (!byVip[vip].includes(rsIp)) byVip[vip].push(rsIp)
  }

  // 检查每个 VIP 下线后是否至少保留一台在线
  for (const [vip, rsIps] of Object.entries(byVip)) {
    const group = groupedData.value.find(g => g.ip === vip)
    if (!group) continue
    const upCount = group.realServers.filter(r => r.statuses.every(s => s.status === 'up')).length
    if (upCount - rsIps.length < 1) {
      ElMessage.error(`VIP ${vip} 下线后将无在线服务器，至少需要保留一台`)
      return
    }
  }

  const skipped = allDetails.length - targets.length
  if (skipped > 0) {
    ElMessage.warning(`${skipped} 台服务器已离线，将自动跳过，仅对 ${targets.length} 台在线服务器执行下线`)
  }

  try {
    const previews = []
    for (const [vip, rsIps] of Object.entries(byVip)) {
      for (const rsIp of rsIps) {
        const res = await lvsOpPreview({ server_id: serverId.value, vs_ip: vip, rs_ip: rsIp, state: 'off' })
        previews.push(res)
      }
    }
    batchPreviews.value = previews
    currentAction.value = 'batch-op'
    previewData.value = previews[0]
    previewId.value = previews[0].preview_id
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

// 切换：选中 2 台 RS（一 up 一 down），直接切换
async function handleSwap() {
  const details = batchSelectedDetails.value
  if (details.length !== 2) return

  const vip = details[0].vip
  const upRs = details.find(d => d.isFullyUp)
  const downRs = details.find(d => d.isFullyDown)

  try {
    const res = await lvsSwapPreview({ server_id: serverId.value, vs_ip: vip, rs_ip1: upRs.rsIp, rs_ip2: downRs.rsIp })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'swap'
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  try {
    if (currentAction.value === 'batch-op') {
      let allOutput = ''
      for (const preview of batchPreviews.value) {
        const res = await lvsOpExecute({ preview_id: preview.preview_id })
        allOutput += (res.output || '') + '\n'
      }
      output.value = allOutput.trim()
      ElMessage.success('批量执行成功')
    } else {
      const executeFn = currentAction.value === 'swap' ? lvsSwapExecute : lvsOpExecute
      const res = await executeFn({ preview_id: previewId.value })
      output.value = res.output
      ElMessage.success('执行成功')
    }
    previewVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = e.response?.data?.output || ''
  } finally {
    executing.value = false
  }
}

function openTagDialog(rsIp, currentTag) {
  tagForm.value = { rs_ip: rsIp, tag: currentTag }
  tagDialogVisible.value = true
}

async function handleSaveTag() {
  if (!tagForm.value.tag) {
    ElMessage.warning('请输入标签')
    return
  }
  tagSaving.value = true
  try {
    await updateLvsTag({ rs_ip: tagForm.value.rs_ip, tag: tagForm.value.tag })
    ElMessage.success('标签已保存')
    tagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    tagSaving.value = false
  }
}

async function handleDeleteTag() {
  tagSaving.value = true
  try {
    await deleteLvsTag(tagForm.value.rs_ip)
    ElMessage.success('标签已删除')
    tagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  } finally {
    tagSaving.value = false
  }
}
</script>

<style scoped>
/* ===== Stat Chips ===== */
.stat-chip {
  font-size: 13px;
  color: #606266;
  background: #f4f4f5;
  padding: 4px 10px;
  border-radius: 4px;
  white-space: nowrap;
}

.stat-chip b {
  margin-left: 4px;
  font-size: 14px;
  color: #303133;
}

.stat-chip-primary b { color: #409eff; }
</style>
