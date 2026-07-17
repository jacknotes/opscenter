<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择K8s服务器" style="width: 150px" @change="handleServerChange">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span style="margin-left: auto"></span>
          <el-input v-model="search" placeholder="搜索命名空间/名称/策略/步骤" clearable style="width: 250px" />
        </div>
        <div class="toolbar">
          <el-dropdown trigger="click" style="margin-right: 12px" @command="onStatusFilter">
            <el-button type="info" class="el-button--cyan"
              >{{ statusFilterLabel
              }}<el-icon style="margin-left: 4px"
                ><svg viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">
                  <path
                    fill="currentColor"
                    d="M831.872 340.864 512 652.672 192.128 340.864a30.592 30.592 0 0 0-42.752 0 29.12 29.12 0 0 0 0 41.6L489.664 714.24a32 32 0 0 0 44.672 0l340.288-331.712a29.12 29.12 0 0 0 0-41.728 30.592 30.592 0 0 0-42.752 0z"
                  /></svg></el-icon
            ></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="all">全部</el-dropdown-item>
                <el-dropdown-item command="pending">待发布</el-dropdown-item>
                <el-dropdown-item command="online">已上线</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="info" class="el-button--cyan" @click="toggleSelectAll()">{{
            allSelected ? '取消' : '全选'
          }}</el-button>
          <el-button type="primary" @click="handleAction('online')">{{
            selectedIds.size > 0 ? '批量上线' : '全量上线'
          }}</el-button>
          <el-button type="warning" @click="handleAction('sync')">{{
            selectedIds.size > 0 ? '批量同步' : '全量同步'
          }}</el-button>
          <el-button type="danger" @click="handleAction('rollback')">{{
            selectedIds.size > 0 ? '批量回滚' : '全量回滚'
          }}</el-button>
          <el-button type="info" class="el-button--cyan" @click="handleRefresh">刷新</el-button>
        </div>
      </template>

      <el-table
        ref="tableRef"
        v-force-reflow
        v-loading="loading"
        :data="paginatedRollouts"
        :row-key="(row) => row.namespace + '/' + row.name"
        stripe
        border
        max-height="calc(100vh - 280px)"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="namespace" label="命名空间" width="150" sortable />
        <el-table-column prop="name" label="名称" min-width="250" sortable />
        <el-table-column prop="strategy" label="策略" width="100" sortable />
        <el-table-column prop="status" label="状态" width="100" sortable>
          <template #default="{ row }">
            <el-tag :type="row.status === 'Paused' ? 'warning' : 'success'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="step" label="步骤" width="80" sortable :sort-method="sortStep" />
        <el-table-column prop="ready" label="就绪" width="80" sortable :sort-method="sortReady" />
      </el-table>

      <div v-if="!loading && paginatedRollouts.length === 0" class="empty-state">
        <el-icon class="empty-state-icon"><Box /></el-icon>
        <span class="empty-state-text">{{
          search || statusFilter !== 'all' ? '没有匹配的 Rollout' : '暂无 Paused 状态的 Rollout'
        }}</span>
      </div>

      <div class="pagination-wrapper">
        <div class="pagination-left">
          <span v-if="selectedIds.size > 0" class="selection-count">已选 {{ selectedIds.size }} 项</span>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="filteredRollouts.length"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="min(700px, 90vw)" top="5vh" align-center>
      <div v-if="previewData">
        <div class="preview-desc">{{ previewData.description }}</div>
        <p style="color: var(--text-secondary); margin-bottom: 8px"><strong>命令：</strong></p>
        <ul style="margin: 0 0 16px 20px; color: var(--text-regular)">
          <li v-for="cmd in previewData.commands" :key="cmd">
            <code style="color: var(--color-primary)">{{ cmd }}</code>
          </li>
        </ul>
        <el-divider />
        <p style="color: var(--text-secondary); margin-bottom: 8px"><strong>当前状态：</strong></p>
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
      <pre class="terminal-pre" style="max-height: 50vh; white-space: pre-wrap; word-break: break-all">{{
        output
      }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onActivated, onDeactivated } from 'vue'
import {
  getK8sRollouts,
  k8sOnlinePreview,
  k8sOnlineExecute,
  k8sSyncPreview,
  k8sSyncExecute,
  k8sRollbackPreview,
  k8sRollbackExecute,
  k8sFullOnlinePreview,
  k8sFullOnlineExecute,
  k8sFullSyncPreview,
  k8sFullSyncExecute,
  k8sFullRollbackPreview,
  k8sFullRollbackExecute,
} from '../api'
import { useServerSelector } from '../composables/useServerSelector'
import { useSelection } from '../composables/useSelection'
import { useOutputCache } from '../composables/useOutputCache'
import { usePreviewExecute } from '../composables/usePreviewExecute'
import { ElMessage } from 'element-plus'
import { showLoadError } from '../utils/message'
import { Box } from '@element-plus/icons-vue'
import { STORAGE_KEYS, DEFAULT_PAGE_SIZE } from '../utils/constants'

const rollouts = ref([])
const search = ref('')
const statusFilter = ref('all')
const loading = ref(false)
const statusFilterLabels = { all: '全部', pending: '待发布', online: '已上线' }
const statusFilterLabel = computed(() => statusFilterLabels[statusFilter.value] || '全部')
const currentPage = ref(1)
const pageSize = ref(DEFAULT_PAGE_SIZE)

// --- 必须在 composable 调用之前定义，避免暂时性死区 ---
const filteredRollouts = computed(() => {
  let list = rollouts.value.filter((r) => r.status === 'Paused')
  if (statusFilter.value === 'pending') {
    list = list.filter((r) => r.step.startsWith('1/'))
  } else if (statusFilter.value === 'online') {
    list = list.filter((r) => !r.step.startsWith('1/'))
  }
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(
      (r) =>
        r.namespace.toLowerCase().includes(q) ||
        r.name.toLowerCase().includes(q) ||
        r.strategy.toLowerCase().includes(q) ||
        r.step.toLowerCase().includes(q)
    )
  }
  return list
})

const paginatedRollouts = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredRollouts.value.slice(start, start + pageSize.value)
})

function parseStep(s) {
  if (!s) return -1
  const match = s.match(/^(\d+)\//)
  return match ? parseInt(match[1], 10) : -1
}

function sortStep(a, b) {
  return parseStep(a.step) - parseStep(b.step)
}

function sortReady(a, b) {
  return (parseInt(a.ready, 10) || 0) - (parseInt(b.ready, 10) || 0)
}

// --- 组合式函数 ---
const keyFn = (row) => row.namespace + '/' + row.name

const {
  servers,
  serverId,
  initServers,
  refreshServers,
  saveSelection,
  handleServerChange: onServerChange,
} = useServerSelector('kubernetes', STORAGE_KEYS.K8S_SERVER, loadData)
const {
  selectedIds,
  allSelected,
  tableRef,
  handleSelectionChange,
  handleSizeChange,
  handleCurrentChange,
  toggleSelectAll,
} = useSelection(keyFn, paginatedRollouts, { search, currentPage })
const output = ref('')
const { outputCache } = useOutputCache([() => serverId.value], output)
const {
  previewVisible,
  previewData,
  previewId,
  executing,
  currentAction,
  openPreview,
  executePreview: doExecutePreview,
} = usePreviewExecute(
  {
    online: k8sOnlineExecute,
    sync: k8sSyncExecute,
    rollback: k8sRollbackExecute,
    full_online: k8sFullOnlineExecute,
    full_sync: k8sFullSyncExecute,
    full_rollback: k8sFullRollbackExecute,
  },
  loadData,
  {
    onOutput: (res) => {
      const errOutput = res?.output
      output.value = Array.isArray(errOutput) ? errOutput.join('\n') : String(errOutput ?? '')
    },
  }
)

// --- 服务器切换时保存选择 ---
function handleServerChange() {
  saveSelection()
  loadData()
}

function onStatusFilter(cmd) {
  statusFilter.value = cmd
  currentPage.value = 1
}

async function handleRefresh() {
  try {
    await loadData()
    ElMessage.success('刷新成功')
  } catch (e) {
    // loadData 已经处理了错误提示
  }
}

onMounted(async () => {
  await initServers()
})

// keep-alive: 页面激活时挂载事件监听，离开时移除（首次挂载和后续激活都走这里）
onActivated(async () => {
  window.addEventListener('page-refresh', handleRefresh)
  await refreshServers()
  if (serverId.value) {
    loadData()
  } else {
    // 服务器全部禁用时清空旧数据，避免显示过期信息
    rollouts.value = []
  }
})
onDeactivated(() => {
  window.removeEventListener('page-refresh', handleRefresh)
})

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem(STORAGE_KEYS.K8S_SERVER, serverId.value)
  loading.value = true
  try {
    const data = await getK8sRollouts(serverId.value)
    rollouts.value = Array.isArray(data) ? data : []
    selectedIds.value.clear()
    tableRef.value?.clearSelection()
  } catch (e) {
    showLoadError(e, '加载数据失败')
    throw e
  } finally {
    loading.value = false
  }
}

async function handleAction(action) {
  if (action === 'online') {
    const pendingList = filteredRollouts.value.filter((r) => r.step.startsWith('1/'))
    if (selectedIds.value.size > 0) {
      const selectedPending = pendingList.filter((r) => selectedIds.value.has(keyFn(r)))
      if (selectedPending.length === 0) {
        ElMessage.warning('所选项目中没有待发布的项目')
        return
      }
    } else {
      if (pendingList.length === 0) {
        ElMessage.warning('当前没有待发布的项目')
        return
      }
    }
  }
  if (action === 'sync' || action === 'rollback') {
    const onlineList = filteredRollouts.value.filter((r) => !r.step.startsWith('1/'))
    if (selectedIds.value.size > 0) {
      const selectedOnline = onlineList.filter((r) => selectedIds.value.has(keyFn(r)))
      if (selectedOnline.length === 0) {
        ElMessage.warning('所选项目尚未上线，请先执行上线操作')
        return
      }
    } else {
      if (onlineList.length === 0) {
        ElMessage.warning('当前没有已上线的项目，请先执行上线操作')
        return
      }
    }
  }
  if (selectedIds.value.size > 0) {
    await handleBatch(action)
  } else {
    await handleFull(action)
  }
}

async function handleBatch(action) {
  const previewFn = {
    online: k8sOnlinePreview,
    sync: k8sSyncPreview,
    rollback: k8sRollbackPreview,
  }[action]

  const allSelectedItems = filteredRollouts.value.filter((r) => selectedIds.value.has(keyFn(r)))

  try {
    const res = await previewFn({
      server_id: serverId.value,
      projects: allSelectedItems.map((r) => ({ name: r.name, namespace: r.namespace })),
    })
    openPreview(res, action)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleFull(action) {
  const previewFn = {
    online: k8sFullOnlinePreview,
    sync: k8sFullSyncPreview,
    rollback: k8sFullRollbackPreview,
  }[action]

  try {
    const res = await previewFn({ server_id: serverId.value })
    openPreview(res, 'full_' + action)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  try {
    await doExecutePreview()
  } catch {
    // 错误已在 composable 中处理
  }
}
</script>

<style scoped>
.preview-desc {
  font-size: 14px;
  color: var(--text-primary);
  margin-bottom: 16px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border-left: 3px solid #06b6d4;
}
</style>
