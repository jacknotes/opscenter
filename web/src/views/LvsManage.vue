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
        <!-- 操作栏 -->
        <div class="toolbar">
        <el-button type="info" class="el-button--cyan" @click="toggleExpandAll">{{ allExpanded ? '折叠' : '展开' }}</el-button>
        <el-button type="info" class="el-button--cyan" @click="toggleAllFiltered">{{ isAllFilteredSelected ? '取消' : '全选' }}</el-button>
        <el-button type="primary" :disabled="!canBatchOnline" @click="handleBatchOnline">上线</el-button>
        <el-button type="danger" :disabled="!canBatchOffline" @click="handleBatchOffline">下线</el-button>
        <el-button type="primary" :disabled="!canSwap" @click="handleSwap">切换</el-button>
        <el-button type="success" @click="loadStatus" :loading="statusLoading">查看配置</el-button>
        <el-button type="info" class="el-button--cyan" @click="handleRefresh" :loading="loading">刷新</el-button>
        <span style="margin-left: auto;"></span>
        <span class="stat-chip stat-chip-success">在线 <b>{{ totalUpCount }}</b></span>
        <span class="stat-chip stat-chip-danger">离线 <b>{{ totalDownCount }}</b></span>
        <span class="stat-chip stat-chip-primary">已选 <b>{{ batchSelectedIPs.length }}</b></span>
      </div>
      </template>

      <!-- 主表格：按 VIP 分组，每端口一行，展开后 RS 表格插入在组下方 -->
      <el-table :data="flattenedMainData" :span-method="mainSpanMethod" :row-class-name="({ rowIndex }) => 'vip-group-' + (flattenedMainData[rowIndex]?.groupIdx % 2)" border v-force-reflow max-height="calc(100vh - 240px)" row-key="uid">
        <el-table-column label="" width="45" align="center">
          <template #default="{ row }">
            <template v-if="row.isDetail">
              <el-table :data="getRSView(row.group).data" :span-method="getRSView(row.group).spanMethod" :row-class-name="({ row: rs }) => rs.disabled ? 'rs-disabled-row' : ''" stripe size="small" style="width: 100%;">
                <el-table-column label="" width="45" align="center">
                  <template #header>
                    <el-checkbox :model-value="isAllSelected(row.group)" :indeterminate="isIndeterminate(row.group)" @change="(val) => toggleSelectAll(row.group, val)" />
                  </template>
                  <template #default="{ row: rs }">
                    <el-checkbox :model-value="isBatchSelected(row.group.ip, rs.ip)" :disabled="rs.disabled" @change="(val) => toggleBatch(row.group.ip, rs.ip, val)" />
                  </template>
                </el-table-column>
                <el-table-column label="Real Server" min-width="100">
                  <template #default="{ row: rs }">
                    <span :style="rs.disabled ? 'color: #475569; text-decoration: line-through;' : ''">{{ rs.ip }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="标签" min-width="100" align="center">
                  <template #default="{ row: rs }">
                    <template v-if="rs.disabled">
                      <el-tooltip :content="rs.disabledReason" placement="top" :disabled="!rs.disabledReason">
                        <el-tag type="info" size="small" style="cursor: pointer;" @click="openRSTagDialog(rs.vipIp, rs)">已禁用</el-tag>
                      </el-tooltip>
                    </template>
                    <el-tooltip v-else-if="rs.tag" :content="rs.tag" placement="top" :show-after="300">
                      <el-tag
                        :type="rs.tag.includes('生产') && !rs.tag.includes('预生产') ? 'danger' : 'warning'"
                        size="small"
                        class="tag-truncate"
                        style="cursor: pointer; max-width: 140px;"
                        @click="openRSTagDialog(rs.vipIp, rs)"
                      >{{ rs.tag }}</el-tag>
                    </el-tooltip>
                    <el-button
                      v-else
                      type="info"
                      link
                      size="small"
                      @click="openRSTagDialog(rs.vipIp, rs)"
                    >设置标签</el-button>
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
            <span v-else-if="row.isFirst" style="cursor: pointer; display: inline-flex; align-items: center;" @click="toggleVIP(row.ip)">
              <el-icon :size="14"><ArrowDown v-if="expandedVIPs.has(row.ip)" /><ArrowRight v-else /></el-icon>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Virtual Server" min-width="100">
          <template #default="{ row }">
            <div v-if="!row.isDetail" style="font-weight: bold;">{{ row.ip }}</div>
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
                  style="cursor: pointer; max-width: 140px;"
                  @click="openVSTagDialog(row.ip, row.tag)"
                >{{ row.tag }}</el-tag>
              </el-tooltip>
              <el-button
                v-else
                type="info"
                link
                size="small"
                @click="openVSTagDialog(row.ip, '')"
              >设置标签</el-button>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="端口" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column label="调度算法" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.scheduler }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Flags" min-width="100">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.flags }}</span>
          </template>
        </el-table-column>
        <el-table-column label="协议" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.protocol }}</span>
          </template>
        </el-table-column>
        <el-table-column label="RS 数量" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail">{{ row.rsCount }}</span>
          </template>
        </el-table-column>
        <el-table-column label="在线" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail" style="color: #22C55E; font-weight: bold;">{{ row.upCount }}</span>
          </template>
        </el-table-column>
        <el-table-column label="离线" min-width="100" align="center">
          <template #default="{ row }">
            <span v-if="!row.isDetail" style="color: #EF4444; font-weight: bold;">{{ row.downCount }}</span>
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
        <pre style="background: #1A1D2E; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ previewData.current_status }}</pre>
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
          <div style="font-weight: bold; font-size: 14px; margin-bottom: 8px; padding-bottom: 4px; border-bottom: 2px solid #06B6D4; color: var(--text-primary);">
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
        <pre style="background: #1A1D2E; padding: 10px; border-radius: 4px; max-height: 500px; overflow-y: auto; font-size: 13px;">{{ statusRaw }}</pre>
      </div>
    </el-dialog>

    <!-- Tag Edit Dialog -->
    <el-dialog v-model="tagDialogVisible" title="设置 RS 标签" width="min(400px, 90vw)" align-center>
      <el-form label-width="80px">
        <el-form-item label="RS IP">
          <el-input :model-value="tagForm.rs_ip" disabled />
        </el-form-item>
        <el-form-item label="标签">
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
        <el-form-item label="禁用操作">
          <el-switch v-model="tagForm.disabled" />
        </el-form-item>
        <el-form-item v-if="tagForm.disabled" label="禁用原因" required>
          <el-input v-model="tagForm.disabled_reason" type="textarea" :rows="2" placeholder="请输入禁用原因（必填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button v-if="tagForm.tag || tagForm.disabled" type="danger" @click="handleDeleteTag" :loading="tagSaving">删除配置</el-button>
        <span style="flex: 1;"></span>
        <el-button @click="tagDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveTag" :loading="tagSaving">保存</el-button>
      </template>
    </el-dialog>

    <!-- VS Tag Edit Dialog -->
    <el-dialog v-model="vsTagDialogVisible" title="设置 VS 标签" width="min(400px, 90vw)" align-center>
      <el-form label-width="80px">
        <el-form-item label="VS IP">
          <el-input :model-value="vsTagForm.vs_ip" disabled />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="vsTagForm.tag" placeholder="请输入标签，如：1号lvs" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button v-if="vsTagForm.tag" type="danger" @click="handleDeleteVSTag" :loading="vsTagSaving">删除标签</el-button>
        <span style="flex: 1;"></span>
        <el-button @click="vsTagDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveVSTag" :loading="vsTagSaving">保存</el-button>
      </template>
    </el-dialog>

    <!-- LVS Online Check Warning Dialog -->
    <el-dialog v-model="lvsOnlineCheckVisible" title="上线前检查" width="min(600px, 90vw)" align-center @close="handleLvsOnlineCheckCancel">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
        <template #title>
          <span v-if="lvsOnlineCheckData">{{ lvsOnlineCheckData.vs_tag }} 的 RS {{ lvsOnlineCheckData.rs_env_tag }} 上线前，预生产环境以下资源副本异常：</span>
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
      <el-alert type="info" :closable="false" style="margin-top: 12px;">
        请输入"确认执行"以继续上线操作
      </el-alert>
      <el-input v-model="lvsOnlineCheckConfirmText" placeholder="请输入 确认执行" style="margin-top: 8px;" />
      <template #footer>
        <el-button @click="lvsOnlineCheckVisible = false">取消</el-button>
        <el-button type="primary" :disabled="lvsOnlineCheckConfirmText !== '确认执行'" @click="handleLvsOnlineCheckConfirm">确认执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { getServers, getLvsList, getLvsStatus, lvsOpPreview, lvsOpExecute, lvsSwapPreview, lvsSwapExecute, updateLvsTag, deleteLvsTag, updateLvsVSTag, deleteLvsVSTag, checkLvsOnlineForPreprod } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, ArrowDown } from '@element-plus/icons-vue'
import { STORAGE_KEYS, AUTO_REFRESH_INTERVAL_MS } from '../constants'

const servers = ref([])
const serverId = ref(null)
const lvsData = ref([])
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
const outputCache = new Map()
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
  if (!data || !Array.isArray(data)) return []
  const map = new Map()
  for (const vs of data) {
    if (!map.has(vs.ip)) {
      map.set(vs.ip, { ip: vs.ip, entries: [], realServersMap: new Map(), role: vs.role || '', tag: vs.tag || '' })
    }
    const group = map.get(vs.ip)
    if (vs.role && !group.role) group.role = vs.role
    if (vs.tag && !group.tag) group.tag = vs.tag
    group.entries.push({ port: vs.port, protocol: vs.protocol, scheduler: vs.scheduler, flags: vs.flags })

    for (const rs of vs.real_servers) {
      if (!group.realServersMap.has(rs.ip)) {
        group.realServersMap.set(rs.ip, { ip: rs.ip, vipIp: vs.ip, statuses: [], tag: rs.tag || '', disabled: !!rs.disabled, disabledReason: rs.disabled_reason || '' })
      } else {
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

  return Array.from(map.values()).map(g => ({
    ip: g.ip,
    entries: g.entries,
    realServers: Array.from(g.realServersMap.values()),
    role: g.role,
    tag: g.tag,
  }))
}

// 虚拟服务器选项
const vsOptions = computed(() => groupedData.value.map(g => g.ip))

// 全局在线/离线统计
const totalUpCount = computed(() => filteredGroups.value.reduce((sum, g) => sum + countUp(g), 0))
const totalDownCount = computed(() => filteredGroups.value.reduce((sum, g) => sum + countDown(g), 0))

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
  const allKeys = filteredGroups.value.flatMap(g => g.realServers.filter(rs => !rs.disabled).map(rs => g.ip + ':' + rs.ip))
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
  let groupIdx = 0
  for (const group of filteredGroups.value) {
    const rsCount = group.realServers.length
    const upCount = countUp(group)
    const downCount = countDown(group)
    group.entries.forEach((entry, i) => {
      rows.push({ uid: group.ip + ':' + entry.port, ...entry, ip: group.ip, rsCount, upCount, downCount, isFirst: i === 0, isLast: i === group.entries.length - 1, group, role: group.role, tag: group.tag, groupIdx })
    })
    // 展开时在组末尾插入 RS 详情行
    if (expandedVIPs.value.has(group.ip)) {
      rows.push({ uid: group.ip + ':detail', ip: group.ip, isDetail: true, group, groupIdx })
    }
    groupIdx++
  }
  return rows
})

// 主表格合并单元格
function mainSpanMethod({ rowIndex, columnIndex }) {
  const row = flattenedMainData.value[rowIndex]
  // 详情行：合并所有列为一个单元格
  if (row.isDetail) {
    return columnIndex === 0 ? [1, 11] : [0, 0]
  }
  // 端口行：合并 expand按钮(0)、IP(1)、角色(2)、标签(3)、RS数量(8)、在线(9)、离线(10)
  if (columnIndex === 0 || columnIndex === 1 || columnIndex === 2 || columnIndex === 3 || columnIndex === 8 || columnIndex === 9 || columnIndex === 10) {
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

// 切换服务器或虚拟服务器时，缓存/恢复执行结果
watch([serverId, vsFilter], ([newServer, newVs], [oldServer, oldVs]) => {
  if (oldServer != null) {
    const oldKey = `${oldServer}:${oldVs}`
    outputCache.set(oldKey, output.value)
  }
  const newKey = `${newServer}:${newVs}`
  const cached = outputCache.get(newKey)
  output.value = cached || ''
})

onMounted(async () => {
  try {
    servers.value = (await getServers('lvs')) || []
    if (servers.value.length > 0) {
      const saved = localStorage.getItem(STORAGE_KEYS.LVS_SERVER)
      if (saved && servers.value.some(s => s.id === Number(saved))) {
        serverId.value = Number(saved)
      } else {
        serverId.value = servers.value[0].id
      }
      await loadData()
    }
  } catch (e) {
    ElMessage.error('加载服务器列表失败')
  }
  // 页面加载后自动启动 300 秒定时刷新
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
  } catch (e) {
    // loadData 已经处理了错误提示
  }
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
    // 默认展开所有 VIP
    if (allExpanded.value) {
      expandedVIPs.value = new Set(groupedData.value.map(g => g.ip))
    }
    if (vsFilter.value && !groupedData.value.some(g => g.ip === vsFilter.value)) {
      vsFilter.value = ''
    }
  } catch (e) {
    if (e.code === 'ECONNABORTED' || e.message?.includes('timeout')) {
      ElMessage.error('连接超时，目标服务器可能不可达，请检查服务器状态')
    } else if (e.response?.data?.error) {
      ElMessage.error(e.response.data.error)
    } else if (!e.response) {
      ElMessage.error('网络异常，无法连接到后端服务')
    } else {
      ElMessage.error('加载数据失败：' + (e.message || '未知错误'))
    }
  } finally {
    loading.value = false
  }
}

// 静默刷新：不显示 loading，保留展开/选择状态
async function silentRefresh() {
  if (!serverId.value) return
  try {
    lvsData.value = await getLvsList(serverId.value)
    groupedData.value = groupByVIP(lvsData.value)
    invalidateRSCache()
    // 清理已不存在的 RS 选中项
    const validKeys = new Set(
      filteredGroups.value.flatMap(g => g.realServers.map(rs => g.ip + ':' + rs.ip))
    )
    batchSelected.value = new Set(Array.from(batchSelected.value).filter(k => validKeys.has(k)))
  } catch {
    // 静默失败，不弹错误提示
  }
}

// 自动刷新定时器管理
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
      rows.push({ ip: rs.ip, vipIp: rs.vipIp, port: s.port, status: s.status, forward: s.forward, weight: s.weight, activeConn: s.activeConn, inactConn: s.inactConn, tag: rs.tag || '', disabled: !!rs.disabled, disabledReason: rs.disabledReason || '' })
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

// 获取 VIP 下所有唯一的可操作 RS IP 列表（排除禁用）
function getRSKeys(group) {
  return group.realServers.filter(rs => !rs.disabled).map(rs => group.ip + ':' + rs.ip)
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

// 全选/反选（仅操作未禁用的 RS）
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
  const allKeys = filteredGroups.value.flatMap(g => g.realServers.filter(rs => !rs.disabled).map(rs => g.ip + ':' + rs.ip))
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

  // 检查 LVS 上线前的预生产依赖
  for (const { vip, rsIp } of targets) {
    try {
      const checkRes = await checkLvsOnlineForPreprod({ vs_ip: vip, rs_ip: rsIp })
      if (checkRes.need_warning) {
        lvsOnlineCheckData.value = checkRes
        lvsOnlineCheckVisible.value = true
        lvsOnlineCheckConfirmText.value = ''
        // 等待用户确认
        const confirmed = await new Promise(resolve => {
          lvsOnlineCheckCallback.value = resolve
        })
        if (!confirmed) return
      }
    } catch {
      // 检查失败不阻塞操作
    }
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

function openTagDialog(vsIp, rsIp, currentTag, disabled, disabledReason) {
  tagForm.value = { vs_ip: vsIp, rs_ip: rsIp, tag: currentTag || '', disabled: !!disabled, disabled_reason: disabledReason || '' }
  tagDialogVisible.value = true
}

// 包装函数：从内层表格模板调用时，外层 row.group.ip 通过 vsIp 参数传入
function openRSTagDialog(vsIp, rs) {
  openTagDialog(vsIp, rs.ip, rs.tag, rs.disabled, rs.disabledReason)
}

async function handleSaveTag() {
  if (tagForm.value.disabled && !tagForm.value.disabled_reason.trim()) {
    ElMessage.warning('禁用时必须填写禁用原因')
    return
  }
  tagSaving.value = true
  try {
    await updateLvsTag({
      vs_ip: tagForm.value.vs_ip,
      rs_ip: tagForm.value.rs_ip,
      tag: tagForm.value.tag,
      disabled: tagForm.value.disabled,
      disabled_reason: tagForm.value.disabled_reason,
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

async function handleDeleteTag() {
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
    await deleteLvsTag(tagForm.value.vs_ip, tagForm.value.rs_ip)
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

async function handleSaveVSTag() {
  vsTagSaving.value = true
  try {
    await updateLvsVSTag({
      vs_ip: vsTagForm.value.vs_ip,
      tag: vsTagForm.value.tag,
    })
    ElMessage.success('VS标签已保存')
    vsTagDialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    vsTagSaving.value = false
  }
}

async function handleDeleteVSTag() {
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
    await deleteLvsVSTag(vsTagForm.value.vs_ip)
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
/* ===== Stat Chips ===== */
.stat-chip {
  font-size: 13px;
  color: #94A3B8;
  background: var(--bg-elevated);
  padding: 4px 10px;
  border-radius: 6px;
  white-space: nowrap;
}

.stat-chip b {
  margin-left: 4px;
  font-size: 14px;
  color: var(--text-primary);
}

.stat-chip-success b { color: #22C55E; }
.stat-chip-danger b { color: #EF4444; }
.stat-chip-primary b { color: #06B6D4; }

.tag-truncate :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.rs-disabled-row) {
  background-color: rgba(255, 255, 255, 0.02) !important;
  opacity: 0.6;
}

:deep(.rs-disabled-row:hover > td) {
  background-color: rgba(255, 255, 255, 0.02) !important;
}

/* ===== Card Header ===== */
:deep(.el-card__header) {
  border-bottom: none;
  padding-bottom: 0;
}

/* ===== Toolbar ===== */
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 3px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border: 1px solid var(--border-default);
  flex-wrap: wrap;
}

.toolbar :deep(.el-dropdown) {
  display: inline-flex;
}

/* ===== VIP 行统一背景色 ===== */
:deep(.vip-group-0 td),
:deep(.vip-group-1 td) {
  background-color: var(--card-bg, #141722);
}

html:not(.dark) :deep(.vip-group-0 td),
html:not(.dark) :deep(.vip-group-1 td) {
  background-color: #ffffff;
}

:deep(.vip-group-0:hover > td),
:deep(.vip-group-1:hover > td) {
  background-color: rgba(6, 182, 212, 0.04) !important;
}
</style>
