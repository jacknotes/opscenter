<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择LVS服务器" style="width: 150px" @change="onServerChange">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span class="filter-label">虚拟服务器:</span>
          <el-select v-model="vsFilter" placeholder="全部" clearable filterable style="width: 150px">
            <el-option v-for="ip in vsOptions" :key="ip" :label="ip" :value="ip" />
          </el-select>
        </div>
        <!-- 操作栏 -->
        <div class="toolbar">
          <el-button type="info" class="el-button--cyan" @click="toggleExpandAll">{{
            allExpanded ? '折叠' : '展开'
          }}</el-button>
          <el-button type="info" class="el-button--cyan" @click="toggleAllFiltered">{{
            isAllFilteredSelected ? '取消' : '全选'
          }}</el-button>
          <el-button type="primary" :disabled="!canBatchOnline" @click="handleBatchOnline">上线</el-button>
          <el-button type="danger" :disabled="!canBatchOffline" @click="handleBatchOffline">下线</el-button>
          <el-button type="primary" :disabled="!canSwap" @click="handleSwap">切换</el-button>
          <el-button type="success" :loading="statusLoading" @click="loadStatus">查看配置</el-button>
          <el-button type="info" class="el-button--cyan" :loading="loading" @click="handleRefresh">刷新</el-button>
          <span style="margin-left: auto"></span>
          <span class="stat-chip stat-chip-success"
            >在线 <b>{{ totalUpCount }}</b></span
          >
          <span class="stat-chip stat-chip-danger"
            >离线 <b>{{ totalDownCount }}</b></span
          >
          <span class="stat-chip stat-chip-primary"
            >已选 <b>{{ batchSelectedIPs.length }}</b></span
          >
        </div>
      </template>

      <!-- 主表格：按 VIP 分组，每端口一行，展开后 RS 表格插入在组下方 -->
      <el-table
        v-force-reflow
        v-loading="loading"
        :data="flattenedMainData"
        :span-method="mainSpanMethod"
        :row-class-name="({ rowIndex }) => 'vip-group-' + (flattenedMainData[rowIndex]?.groupIdx % 2)"
        border
        max-height="calc(100vh - 240px)"
        row-key="uid"
      >
        <el-table-column label="" width="45" align="center">
          <template #default="{ row }">
            <template v-if="row.isDetail">
              <el-table
                :data="getRSView(row.group).data"
                :span-method="getRSView(row.group).spanMethod"
                :row-class-name="({ row: rs }) => (rs.disabled ? 'rs-disabled-row' : '')"
                stripe
                size="small"
                style="width: 100%"
              >
                <el-table-column label="" width="45" align="center">
                  <template #header>
                    <el-checkbox
                      :model-value="isAllSelected(row.group)"
                      :indeterminate="isIndeterminate(row.group)"
                      @change="(val) => toggleSelectAll(row.group, val)"
                    />
                  </template>
                  <template #default="{ row: rs }">
                    <el-checkbox
                      :model-value="isBatchSelected(row.group.ip, rs.ip)"
                      :disabled="rs.disabled"
                      @change="(val) => toggleBatch(row.group.ip, rs.ip, val)"
                    />
                  </template>
                </el-table-column>
                <el-table-column label="Real Server" min-width="100">
                  <template #default="{ row: rs }">
                    <span :style="rs.disabled ? 'color: #475569; text-decoration: line-through;' : ''">{{
                      rs.ip
                    }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="标签" min-width="100" align="center">
                  <template #default="{ row: rs }">
                    <template v-if="rs.disabled">
                      <el-tooltip :content="rs.disabledReason" placement="top" :disabled="!rs.disabledReason">
                        <el-tag type="info" size="small" style="cursor: pointer" @click="openRSTagDialog(rs.vipIp, rs)"
                          >已禁用</el-tag
                        >
                      </el-tooltip>
                    </template>
                    <el-tooltip v-else-if="rs.tag" :content="rs.tag" placement="top" :show-after="300">
                      <el-tag
                        :type="rs.tag.includes('生产') && !rs.tag.includes('预生产') ? 'danger' : 'warning'"
                        size="small"
                        class="tag-truncate"
                        style="cursor: pointer; max-width: 140px"
                        @click="openRSTagDialog(rs.vipIp, rs)"
                        >{{ rs.tag }}</el-tag
                      >
                    </el-tooltip>
                    <el-button v-else type="info" link size="small" @click="openRSTagDialog(rs.vipIp, rs)"
                      >设置标签</el-button
                    >
                  </template>
                </el-table-column>
                <el-table-column label="端口" min-width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.port }}</template>
                </el-table-column>
                <el-table-column label="状态" min-width="100" align="center">
                  <template #default="{ row: rs }">
                    <el-tag :type="rs.status === 'up' ? 'success' : 'danger'" size="small">{{ rs.status }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="转发" min-width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.forward }}</template>
                </el-table-column>
                <el-table-column label="Weight" min-width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.weight }}</template>
                </el-table-column>
                <el-table-column label="ActiveConn" min-width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.activeConn }}</template>
                </el-table-column>
                <el-table-column label="InActConn" min-width="100" align="center">
                  <template #default="{ row: rs }">{{ rs.inactConn }}</template>
                </el-table-column>
              </el-table>
            </template>
            <span
              v-else-if="row.isFirst"
              style="cursor: pointer; display: inline-flex; align-items: center"
              @click="toggleVIP(row.ip)"
            >
              <el-icon :size="14"><ArrowDown v-if="expandedVIPs.has(row.ip)" /><ArrowRight v-else /></el-icon>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Virtual Server" min-width="100">
          <template #default="{ row }">
            <div v-if="!row.isDetail" style="font-weight: bold">{{ row.ip }}</div>
          </template>
        </el-table-column>
        <el-table-column label="角色" min-width="100" align="center">
          <template #default="{ row }">
            <template v-if="!row.isDetail">
              <el-tag v-if="row.role === 'master'" type="success" size="small">主</el-tag>
              <el-tag v-else-if="row.role === 'backup'" type="info" size="small">备</el-tag>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="标签" min-width="100" align="center">
          <template #default="{ row }">
            <template v-if="row.isFirst && !row.isDetail">
              <el-tooltip v-if="row.tag" :content="row.tag" placement="top" :show-after="300">
                <el-tag
                  type="warning"
                  size="small"
                  class="tag-truncate"
                  style="cursor: pointer; max-width: 140px"
                  @click="openVSTagDialog(row.ip, row.tag)"
                  >{{ row.tag }}</el-tag
                >
              </el-tooltip>
              <el-button v-else type="info" link size="small" @click="openVSTagDialog(row.ip, '')">设置标签</el-button>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="端口" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail">{{ row.port }}</span></template
          >
        </el-table-column>
        <el-table-column label="调度算法" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail">{{ row.scheduler }}</span></template
          >
        </el-table-column>
        <el-table-column label="Flags" min-width="100">
          <template #default="{ row }"
            ><span v-if="!row.isDetail">{{ row.flags }}</span></template
          >
        </el-table-column>
        <el-table-column label="协议" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail">{{ row.protocol }}</span></template
          >
        </el-table-column>
        <el-table-column label="RS 数量" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail">{{ row.rsCount }}</span></template
          >
        </el-table-column>
        <el-table-column label="在线" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail" style="color: var(--color-success); font-weight: bold">{{
              row.upCount
            }}</span></template
          >
        </el-table-column>
        <el-table-column label="离线" min-width="100" align="center">
          <template #default="{ row }"
            ><span v-if="!row.isDetail" style="color: var(--color-danger); font-weight: bold">{{
              row.downCount
            }}</span></template
          >
        </el-table-column>
      </el-table>

      <div v-if="!loading && flattenedMainData.length === 0" class="empty-state">
        <el-icon class="empty-state-icon"><Connection /></el-icon>
        <span class="empty-state-text">暂无 LVS 数据</span>
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" :width="batchPreviews.length > 1 ? 'min(800px, 90vw)' : 'min(600px, 90vw)'" align-center>
      <div v-if="previewData">
        <div v-if="batchPreviews.length > 1">
          <p style="color: #e6a23c; margin-bottom: 12px">共 {{ batchPreviews.length }} 个操作将依次执行</p>
          <el-table :data="batchPreviews" stripe size="small" border max-height="300">
            <el-table-column type="index" label="#" width="50" />
            <el-table-column label="操作" min-width="300"
              ><template #default="{ row }">{{ row.description }}</template></el-table-column
            >
            <el-table-column label="命令" min-width="250"
              ><template #default="{ row }"
                ><code style="font-size: 12px">{{ row.command }}</code></template
              ></el-table-column
            >
          </el-table>
        </div>
        <div v-else>
          <p><strong>操作：</strong>{{ previewData.description }}</p>
          <p>
            <strong>命令：</strong><code>{{ previewData.command }}</code>
          </p>
        </div>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre class="preview-pre">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px">
      <template #header>执行结果</template>
      <pre class="terminal-pre">{{ output }}</pre>
    </el-card>

    <!-- 子组件对话框 -->
    <StatusDialog v-model="statusVisible" :groups="statusGroups" :raw="statusRaw" />
    <TagDialog
      v-model="tagDialogVisible"
      :form="tagForm"
      :saving="tagSaving"
      @save="handleSaveTag"
      @delete="handleDeleteTag"
    />
    <VSTagDialog
      v-model="vsTagDialogVisible"
      :form="vsTagForm"
      :saving="vsTagSaving"
      @save="handleSaveVSTag"
      @delete="handleDeleteVSTag"
    />

    <!-- LVS Online Check Warning Dialog -->
    <el-dialog
      v-model="lvsOnlineCheckVisible"
      title="上线前检查"
      width="min(600px, 90vw)"
      align-center
      @close="handleLvsOnlineCheckCancel"
    >
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        <template #title>
          <span v-if="lvsOnlineCheckData"
            >{{ lvsOnlineCheckData.vs_tag }} 的 RS
            {{ lvsOnlineCheckData.rs_env_tag }} 上线前，预生产环境以下资源副本异常：</span
          >
        </template>
      </el-alert>
      <div v-if="lvsOnlineCheckData && lvsOnlineCheckData.warnings">
        <el-table :data="lvsOnlineCheckData.warnings" stripe size="small" border max-height="300">
          <el-table-column prop="name" label="资源名称" min-width="150" />
          <el-table-column prop="category" label="类型" width="100" />
          <el-table-column prop="current" label="当前副本" width="100" align="center" />
          <el-table-column prop="target" label="目标副本" width="100" align="center" />
        </el-table>
      </div>
      <el-alert type="info" :closable="false" style="margin-top: 12px">请输入"确认执行"以继续上线操作</el-alert>
      <el-input v-model="lvsOnlineCheckConfirmText" placeholder="请输入 确认执行" style="margin-top: 8px" />
      <template #footer>
        <el-button @click="lvsOnlineCheckVisible = false">取消</el-button>
        <el-button
          type="primary"
          :disabled="lvsOnlineCheckConfirmText !== '确认执行'"
          @click="handleLvsOnlineCheckConfirm"
          >确认执行</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, onMounted, onUnmounted, onActivated, onDeactivated, watch } from 'vue'
import {
  getLvsList,
  getLvsStatus,
  lvsOpPreview,
  lvsOpExecute,
  lvsSwapPreview,
  lvsSwapExecute,
  updateLvsTag,
  deleteLvsTag,
  updateLvsVSTag,
  deleteLvsVSTag,
  checkLvsOnlineForPreprod,
} from '../../api'
import { useServerSelector } from '../../composables/useServerSelector'
import { useOutputCache } from '../../composables/useOutputCache'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, ArrowDown, Connection } from '@element-plus/icons-vue'
import { STORAGE_KEYS, AUTO_REFRESH_INTERVAL_MS } from '../../utils/constants'
import TagDialog from './TagDialog.vue'
import VSTagDialog from './VSTagDialog.vue'
import StatusDialog from './StatusDialog.vue'

// --- 组合式函数 ---
const { servers, serverId, initServers, refreshServers, saveSelection } = useServerSelector('lvs', STORAGE_KEYS.LVS_SERVER)
const lvsData = shallowRef([])
const groupedData = ref([])
const vsFilter = ref(localStorage.getItem(STORAGE_KEYS.LVS_VS_FILTER) || '')
watch(vsFilter, (val) => {
  if (val) {
    localStorage.setItem(STORAGE_KEYS.LVS_VS_FILTER, val)
  } else {
    localStorage.removeItem(STORAGE_KEYS.LVS_VS_FILTER)
  }
})
const allExpanded = ref(true)
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
let autoRefreshTimer = null
const tagDialogVisible = ref(false)
const tagForm = ref({ vs_ip: '', rs_ip: '', tag: '', disabled: false, disabled_reason: '' })
const tagSaving = ref(false)
const tagOptions = [
  { label: '生产环境', value: '生产环境' },
  { label: '预生产环境', value: '预生产环境' },
]
const vsTagDialogVisible = ref(false)
const vsTagForm = ref({ vs_ip: '', tag: '' })
const vsTagSaving = ref(false)
const lvsOnlineCheckVisible = ref(false)
const lvsOnlineCheckData = ref(null)
const lvsOnlineCheckCallback = ref(null)
const lvsOnlineCheckConfirmText = ref('')

const batchSelectedIPs = computed(() =>
  Array.from(batchSelected.value).map((key) => {
    const idx = key.indexOf(':')
    return { vip: key.substring(0, idx), rsIp: key.substring(idx + 1) }
  })
)

const batchSelectedDetails = computed(() => {
  const details = []
  for (const { vip, rsIp } of batchSelectedIPs.value) {
    const group = groupedData.value.find((g) => g.ip === vip)
    if (!group) continue
    const rs = group.realServers.find((r) => r.ip === rsIp)
    if (!rs) continue
    details.push({
      vip,
      rsIp,
      isFullyUp: rs.statuses.every((s) => s.status === 'up'),
      isFullyDown: rs.statuses.every((s) => s.status === 'down'),
    })
  }
  return details
})

const canBatchOnline = computed(() => batchSelectedDetails.value.some((d) => d.isFullyDown))
const canBatchOffline = computed(() => batchSelectedDetails.value.some((d) => d.isFullyUp))
const canSwap = computed(() => {
  const d = batchSelectedDetails.value
  return d.length === 2 && d[0].vip === d[1].vip && d.some((x) => x.isFullyUp) && d.some((x) => x.isFullyDown)
})

function groupByVIP(data) {
  if (!data || !Array.isArray(data)) return []
  const map = new Map()
  for (const vs of data) {
    if (!map.has(vs.ip))
      map.set(vs.ip, { ip: vs.ip, entries: [], realServersMap: new Map(), role: vs.role || '', tag: vs.tag || '' })
    const group = map.get(vs.ip)
    if (vs.role && !group.role) group.role = vs.role
    if (vs.tag && !group.tag) group.tag = vs.tag
    group.entries.push({ port: vs.port, protocol: vs.protocol, scheduler: vs.scheduler, flags: vs.flags })
    for (const rs of vs.real_servers) {
      if (!group.realServersMap.has(rs.ip))
        group.realServersMap.set(rs.ip, {
          ip: rs.ip,
          vipIp: vs.ip,
          statuses: [],
          tag: rs.tag || '',
          disabled: !!rs.disabled,
          disabledReason: rs.disabled_reason || '',
        })
      else {
        if (rs.tag) group.realServersMap.get(rs.ip).tag = rs.tag
        if (rs.disabled) {
          group.realServersMap.get(rs.ip).disabled = true
          group.realServersMap.get(rs.ip).disabledReason = rs.disabled_reason || ''
        }
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
  return Array.from(map.values()).map((g) => ({
    ip: g.ip,
    entries: g.entries,
    realServers: Array.from(g.realServersMap.values()),
    role: g.role,
    tag: g.tag,
  }))
}

const vsOptions = computed(() => groupedData.value.map((g) => g.ip))
const totalUpCount = computed(() => filteredGroups.value.reduce((sum, g) => sum + countUp(g), 0))
const totalDownCount = computed(() => filteredGroups.value.reduce((sum, g) => sum + countDown(g), 0))
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
  expandedVIPs.value = val ? new Set(filteredGroups.value.map((g) => g.ip)) : new Set()
})

const isAllFilteredSelected = computed(() => {
  const allKeys = filteredGroups.value.flatMap((g) =>
    g.realServers.filter((rs) => !rs.disabled).map((rs) => g.ip + ':' + rs.ip)
  )
  return allKeys.length > 0 && allKeys.every((k) => batchSelected.value.has(k))
})
const filteredGroups = computed(() =>
  vsFilter.value ? groupedData.value.filter((g) => g.ip === vsFilter.value) : groupedData.value
)

function countUp(row) {
  return row.realServers.filter((rs) => rs.statuses.some((s) => s.status === 'up')).length
}
function countDown(row) {
  return row.realServers.filter((rs) => rs.statuses.every((s) => s.status === 'down')).length
}

const flattenedMainData = computed(() => {
  const rows = []
  let groupIdx = 0
  for (const group of filteredGroups.value) {
    const rsCount = group.realServers.length
    const upCount = countUp(group)
    const downCount = countDown(group)
    group.entries.forEach((entry, i) => {
      rows.push({
        uid: group.ip + ':' + entry.port,
        ...entry,
        ip: group.ip,
        rsCount,
        upCount,
        downCount,
        isFirst: i === 0,
        isLast: i === group.entries.length - 1,
        group,
        role: group.role,
        tag: group.tag,
        groupIdx,
      })
    })
    if (expandedVIPs.value.has(group.ip))
      rows.push({ uid: group.ip + ':detail', ip: group.ip, isDetail: true, group, groupIdx })
    groupIdx++
  }
  return rows
})

function mainSpanMethod({ rowIndex, columnIndex }) {
  const row = flattenedMainData.value[rowIndex]
  if (row.isDetail) return columnIndex === 0 ? [1, 11] : [0, 0]
  if ([0, 1, 2, 3, 8, 9, 10].includes(columnIndex)) {
    if (!row.isFirst) return [0, 0]
    let count = 0
    let i = rowIndex
    while (
      i < flattenedMainData.value.length &&
      flattenedMainData.value[i].ip === row.ip &&
      !flattenedMainData.value[i].isDetail
    ) {
      count++
      i++
    }
    return [count, 1]
  }
}

useOutputCache([() => serverId.value, () => vsFilter.value], output)

onMounted(async () => {
  await initServers()
  await loadData()
  startAutoRefresh()
})

async function onServerChange() {
  localStorage.removeItem(STORAGE_KEYS.LVS_VS_FILTER)
  vsFilter.value = ''
  await loadData()
}
async function handleRefresh() {
  try {
    await loadData()
    ElMessage.success('刷新成功')
  } catch {}
}

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem(STORAGE_KEYS.LVS_SERVER, serverId.value)
  loading.value = true
  try {
    lvsData.value = await getLvsList(serverId.value)
    groupedData.value = groupByVIP(lvsData.value)
    invalidateRSCache()
    batchSelected.value = new Set()
    if (allExpanded.value) expandedVIPs.value = new Set(groupedData.value.map((g) => g.ip))
    if (vsFilter.value && !groupedData.value.some((g) => g.ip === vsFilter.value)) vsFilter.value = ''
  } catch (e) {
    if (e.code === 'ECONNABORTED' || e.message?.includes('timeout')) ElMessage.error('连接超时，目标服务器可能不可达')
    else if (e.response?.data?.error) ElMessage.error(e.response.data.error)
    else if (!e.response) ElMessage.error('网络异常')
    else ElMessage.error('加载数据失败：' + (e.message || '未知错误'))
  } finally {
    loading.value = false
  }
}

async function silentRefresh() {
  if (!serverId.value) return
  try {
    lvsData.value = await getLvsList(serverId.value)
    groupedData.value = groupByVIP(lvsData.value)
    invalidateRSCache()
    const validKeys = new Set(filteredGroups.value.flatMap((g) => g.realServers.map((rs) => g.ip + ':' + rs.ip)))
    batchSelected.value = new Set(Array.from(batchSelected.value).filter((k) => validKeys.has(k)))
  } catch {}
}

function startAutoRefresh() {
  stopAutoRefresh()
  autoRefreshTimer = setInterval(silentRefresh, AUTO_REFRESH_INTERVAL_MS)
}
function stopAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}
onUnmounted(() => {
  stopAutoRefresh()
})

// keep-alive: 页面激活时重启定时器，离开时暂停
onActivated(async () => {
  await refreshServers()
  if (serverId.value) {
    loadData()
    startAutoRefresh()
  }
})
onDeactivated(() => {
  stopAutoRefresh()
})

async function loadStatus() {
  if (!serverId.value) return
  statusLoading.value = true
  try {
    const res = await getLvsStatus(serverId.value)
    statusGroups.value = vsFilter.value
      ? (res.groups || []).filter((g) => g.vs_ip === vsFilter.value)
      : res.groups || []
    statusRaw.value = res.output || '无数据'
    statusVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '获取状态失败')
  } finally {
    statusLoading.value = false
  }
}

function flattenRS(group) {
  const rows = []
  for (const rs of group.realServers) {
    for (const s of rs.statuses) {
      rows.push({
        ip: rs.ip,
        vipIp: rs.vipIp,
        port: s.port,
        status: s.status,
        forward: s.forward,
        weight: s.weight,
        activeConn: s.activeConn,
        inactConn: s.inactConn,
        tag: rs.tag || '',
        disabled: !!rs.disabled,
        disabledReason: rs.disabledReason || '',
      })
    }
  }
  return rows
}

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
  if (checked) newSet.add(key)
  else newSet.delete(key)
  batchSelected.value = newSet
}
function getRSKeys(group) {
  return group.realServers.filter((rs) => !rs.disabled).map((rs) => group.ip + ':' + rs.ip)
}
function isAllSelected(group) {
  const keys = getRSKeys(group)
  return keys.length > 0 && keys.every((k) => batchSelected.value.has(k))
}
function isIndeterminate(group) {
  const keys = getRSKeys(group)
  const selected = keys.filter((k) => batchSelected.value.has(k))
  return selected.length > 0 && selected.length < keys.length
}
function toggleSelectAll(group, checked) {
  const keys = getRSKeys(group)
  const newSet = new Set(batchSelected.value)
  for (const k of keys) {
    if (checked) newSet.add(k)
    else newSet.delete(k)
  }
  batchSelected.value = newSet
}
function toggleExpandAll() {
  allExpanded.value = !allExpanded.value
}
function toggleVIP(vip) {
  const newSet = new Set(expandedVIPs.value)
  if (newSet.has(vip)) newSet.delete(vip)
  else newSet.add(vip)
  expandedVIPs.value = newSet
}
function toggleAllFiltered() {
  const allKeys = filteredGroups.value.flatMap((g) =>
    g.realServers.filter((rs) => !rs.disabled).map((rs) => g.ip + ':' + rs.ip)
  )
  const allSel = allKeys.length > 0 && allKeys.every((k) => batchSelected.value.has(k))
  const newSet = new Set(batchSelected.value)
  if (allSel) {
    for (const k of allKeys) newSet.delete(k)
  } else {
    for (const k of allKeys) newSet.add(k)
  }
  batchSelected.value = newSet
}

async function handleBatchOnline() {
  const allDetails = batchSelectedDetails.value
  const targets = allDetails.filter((d) => d.isFullyDown)
  if (targets.length === 0) {
    ElMessage.warning('所选服务器均已在线，无需上线')
    return
  }
  for (const { vip, rsIp } of targets) {
    try {
      const checkRes = await checkLvsOnlineForPreprod({ vs_ip: vip, rs_ip: rsIp })
      if (checkRes.need_warning) {
        lvsOnlineCheckData.value = checkRes
        lvsOnlineCheckVisible.value = true
        lvsOnlineCheckConfirmText.value = ''
        const confirmed = await new Promise((resolve) => {
          lvsOnlineCheckCallback.value = resolve
        })
        if (!confirmed) return
      }
    } catch (e) {
      ElMessage.error(`预生产安全检查失败: ${e.response?.data?.error || e.message || '未知错误'}，操作中止`)
      return
    }
  }
  const skipped = allDetails.length - targets.length
  if (skipped > 0)
    ElMessage.warning(`${skipped} 台已在线将跳过，将上线 ${targets.length} 台离线服务器`)
  const byVip = {}
  for (const { vip, rsIp } of targets) {
    if (!byVip[vip]) byVip[vip] = []
    if (!byVip[vip].includes(rsIp)) byVip[vip].push(rsIp)
  }
  try {
    const previews = []
    for (const [vip, rsIps] of Object.entries(byVip)) {
      for (const rsIp of rsIps) {
        previews.push(await lvsOpPreview({ server_id: serverId.value, vs_ip: vip, rs_ip: rsIp, state: 'on' }))
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

function handleLvsOnlineCheckConfirm() {
  lvsOnlineCheckVisible.value = false
  if (lvsOnlineCheckCallback.value) {
    lvsOnlineCheckCallback.value(true)
    lvsOnlineCheckCallback.value = null
  }
}
function handleLvsOnlineCheckCancel() {
  if (lvsOnlineCheckCallback.value) {
    lvsOnlineCheckCallback.value(false)
    lvsOnlineCheckCallback.value = null
  }
}

async function handleBatchOffline() {
  const allDetails = batchSelectedDetails.value
  const targets = allDetails.filter((d) => d.isFullyUp)
  if (targets.length === 0) {
    ElMessage.warning('所选服务器均已离线，无需下线')
    return
  }
  const byVip = {}
  for (const { vip, rsIp } of targets) {
    if (!byVip[vip]) byVip[vip] = []
    if (!byVip[vip].includes(rsIp)) byVip[vip].push(rsIp)
  }
  for (const [vip, rsIps] of Object.entries(byVip)) {
    const group = groupedData.value.find((g) => g.ip === vip)
    if (!group) continue
    const upCount = group.realServers.filter((r) => r.statuses.every((s) => s.status === 'up')).length
    if (upCount - rsIps.length < 1) {
      ElMessage.warning(`VIP ${vip} 下线后将无在线服务器，至少需要保留一台`)
      return
    }
  }
  const skipped = allDetails.length - targets.length
  if (skipped > 0) ElMessage.warning(`${skipped} 台服务器已离线，将自动跳过`)
  try {
    const previews = []
    for (const [vip, rsIps] of Object.entries(byVip)) {
      for (const rsIp of rsIps) {
        previews.push(await lvsOpPreview({ server_id: serverId.value, vs_ip: vip, rs_ip: rsIp, state: 'off' }))
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

async function handleSwap() {
  const details = batchSelectedDetails.value
  if (details.length !== 2) return
  const vip = details[0].vip
  const upRs = details.find((d) => d.isFullyUp)
  const downRs = details.find((d) => d.isFullyDown)
  try {
    const checkRes = await checkLvsOnlineForPreprod({ vs_ip: vip, rs_ip: downRs.rsIp })
    if (checkRes.need_warning) {
      lvsOnlineCheckData.value = checkRes
      lvsOnlineCheckVisible.value = true
      lvsOnlineCheckConfirmText.value = ''
      const confirmed = await new Promise((resolve) => {
        lvsOnlineCheckCallback.value = resolve
      })
      if (!confirmed) return
    }
  } catch (e) {
    ElMessage.error(`预生产安全检查失败: ${e.response?.data?.error || e.message || '未知错误'}，操作中止`)
    return
  }
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
      let hasError = false
      for (const p of batchPreviews.value) {
        try {
          const res = await lvsOpExecute({ preview_id: p.preview_id })
          allOutput += (res.output || '') + '\n'
        } catch (e) {
          allOutput += `[错误] ${e.response?.data?.error || e.message || '执行失败'}\n`
          hasError = true
        }
      }
      output.value = allOutput.trim()
      if (hasError) {
        ElMessage.warning('批量执行部分失败，请查看输出详情')
      } else {
        ElMessage.success('批量执行成功')
      }
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

function openTagDialog(vsIp, rsIp, currentTag, disabled, disabledReason) {
  tagForm.value = {
    vs_ip: vsIp,
    rs_ip: rsIp,
    tag: currentTag || '',
    disabled: !!disabled,
    disabled_reason: disabledReason || '',
  }
  tagDialogVisible.value = true
}
function openRSTagDialog(vsIp, rs) {
  openTagDialog(vsIp, rs.ip, rs.tag, rs.disabled, rs.disabledReason)
}

async function handleSaveTag(formData) {
  if (formData.disabled && !formData.disabled_reason.trim()) {
    ElMessage.warning('禁用时必须填写禁用原因')
    return
  }
  tagSaving.value = true
  try {
    await updateLvsTag({
      vs_ip: formData.vs_ip,
      rs_ip: formData.rs_ip,
      tag: formData.tag,
      disabled: formData.disabled,
      disabled_reason: formData.disabled_reason,
    })
    ElMessage.success('保存成功')
    tagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    tagSaving.value = false
  }
}

async function handleDeleteTag(formData) {
  try {
    await ElMessageBox.confirm('确定要删除该标签吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  tagSaving.value = true
  try {
    await deleteLvsTag(formData.vs_ip, formData.rs_ip)
    ElMessage.success('标签已删除')
    tagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  } finally {
    tagSaving.value = false
  }
}

function openVSTagDialog(vsIp, currentTag) {
  vsTagForm.value = { vs_ip: vsIp, tag: currentTag || '' }
  vsTagDialogVisible.value = true
}

async function handleSaveVSTag(formData) {
  vsTagSaving.value = true
  try {
    await updateLvsVSTag({ vs_ip: formData.vs_ip, tag: formData.tag })
    ElMessage.success('VS标签已保存')
    vsTagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    vsTagSaving.value = false
  }
}

async function handleDeleteVSTag(formData) {
  try {
    await ElMessageBox.confirm('确定要删除该标签吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  vsTagSaving.value = true
  try {
    await deleteLvsVSTag(formData.vs_ip)
    ElMessage.success('VS标签已删除')
    vsTagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  } finally {
    vsTagSaving.value = false
  }
}
</script>

<style scoped>
.tag-truncate :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.rs-disabled-row) {
  background-color: var(--bg-elevated) !important;
  opacity: 0.6;
}
:deep(.rs-disabled-row:hover > td) {
  background-color: var(--bg-elevated) !important;
}
:deep(.vip-group-0 td),
:deep(.vip-group-1 td) {
  background-color: var(--card-bg);
}
:deep(.vip-group-0:hover > td),
:deep(.vip-group-1:hover > td) {
  background-color: rgba(6, 182, 212, 0.04) !important;
}
@media (max-width: 768px) {
  :deep(.el-table) {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }
  :deep(.el-table .el-table__inner-wrapper) {
    min-width: 800px;
  }
}
</style>
