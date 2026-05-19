<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择K8s服务器" style="width: 250px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-input v-model="search" placeholder="搜索命名空间/名称/策略/步骤" clearable style="width: 250px;" />
        </div>
      </template>

      <div style="margin-bottom: 15px; display: flex; gap: 10px;">
        <el-button type="primary" @click="handleToggleSelect">{{ allSelected ? '取消全选' : '全选' }}</el-button>
        <el-button type="primary" @click="handleAction('online')">{{ selectedIds.size > 0 ? '批量上线' : '全量上线' }}</el-button>
        <el-button type="warning" @click="handleAction('sync')">{{ selectedIds.size > 0 ? '批量同步' : '全量同步' }}</el-button>
        <el-button type="danger" @click="handleAction('rollback')">{{ selectedIds.size > 0 ? '批量回滚' : '全量回滚' }}</el-button>
        <el-button type="success" @click="handleRefresh">刷新</el-button>
        <span v-if="selectedIds.size > 0" style="margin-left: 10px; font-size: 13px; color: #909399;">
          已选 {{ selectedIds.size }} 项
        </span>
      </div>

      <el-table ref="tableRef" :data="paginatedRollouts" :row-key="row => row.namespace + '/' + row.name" stripe border @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="55" />
        <el-table-column prop="namespace" label="命名空间" width="150" />
        <el-table-column prop="name" label="名称" min-width="250" />
        <el-table-column prop="strategy" label="策略" width="100" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'Paused' ? 'warning' : 'success'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="step" label="步骤" width="80" />
        <el-table-column prop="ready" label="就绪" width="80" />
      </el-table>

      <div style="margin-top: 15px; display: flex; justify-content: flex-end;">
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
    <el-dialog v-model="previewVisible" title="变更预览" width="600px" top="5vh">
      <div v-if="previewData" style="max-height: 65vh; overflow-y: auto;">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <p><strong>命令：</strong></p>
        <ul>
          <li v-for="cmd in previewData.commands" :key="cmd"><code>{{ cmd }}</code></li>
        </ul>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px; white-space: pre-wrap; word-break: break-all;">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>执行结果</template>
      <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 4px; max-height: 50vh; overflow-y: auto; white-space: pre-wrap; word-break: break-all;">{{ output }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import {
  getServers, getK8sRollouts,
  k8sOnlinePreview, k8sOnlineExecute,
  k8sSyncPreview, k8sSyncExecute,
  k8sRollbackPreview, k8sRollbackExecute,
  k8sFullOnlinePreview, k8sFullOnlineExecute,
  k8sFullSyncPreview, k8sFullSyncExecute,
  k8sFullRollbackPreview, k8sFullRollbackExecute
} from '../api'
import { ElMessage } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const rollouts = ref([])
const selected = ref([])
const selectedIds = ref(new Set())
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')
const search = ref('')
const tableRef = ref(null)
const currentPage = ref(1)
const pageSize = ref(20)
const skipSelectionSync = ref(false)

const filteredRollouts = computed(() => {
  let list = rollouts.value.filter(r => r.status === 'Paused')
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(r =>
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

const allSelected = computed(() =>
  filteredRollouts.value.length > 0 && filteredRollouts.value.every(r => selectedIds.value.has(r.namespace + '/' + r.name))
)

function handleSizeChange(size) {
  pageSize.value = size
  currentPage.value = 1
  restoreSelection()
}

function handleCurrentChange() {
  restoreSelection()
}

// 换页/搜索后根据 selectedIds 恢复当前页的勾选状态
function restoreSelection() {
  skipSelectionSync.value = true
  setTimeout(() => {
    paginatedRollouts.value.forEach(row => {
      if (selectedIds.value.has(row.namespace + '/' + row.name)) {
        tableRef.value.toggleRowSelection(row, true)
      }
    })
    skipSelectionSync.value = false
  }, 0)
}

watch(search, () => { currentPage.value = 1; restoreSelection() })

function handleToggleSelect() {
  if (allSelected.value) {
    selectedIds.value.clear()
    tableRef.value.clearSelection()
  } else {
    filteredRollouts.value.forEach(row => {
      selectedIds.value.add(row.namespace + '/' + row.name)
    })
    // 只勾选当前页的行
    paginatedRollouts.value.forEach(row => {
      tableRef.value.toggleRowSelection(row, true)
    })
  }
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
  try {
    servers.value = await getServers('kubernetes')
    if (servers.value.length > 0) {
      const saved = localStorage.getItem('k8s_server')
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

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem('k8s_server', serverId.value)

  try {
    const data = await getK8sRollouts(serverId.value)
    // 处理后端可能返回null的情况
    rollouts.value = Array.isArray(data) ? data : []
    selectedIds.value.clear()
    tableRef.value?.clearSelection()
  } catch (e) {
    ElMessage.error('加载数据失败')
    throw e
  }
}

function handleSelectionChange(val) {
  selected.value = val
  // 数据变化导致的选中清空，跳过同步，由 restoreSelection 恢复
  if (skipSelectionSync.value) return
  // 同步当前页选中状态到 selectedIds
  const pageKeys = paginatedRollouts.value.map(r => r.namespace + '/' + r.name)
  pageKeys.forEach(key => selectedIds.value.delete(key))
  val.forEach(r => selectedIds.value.add(r.namespace + '/' + r.name))
}

async function handleAction(action) {
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
    rollback: k8sRollbackPreview
  }[action]

  const allSelectedItems = filteredRollouts.value.filter(r => selectedIds.value.has(r.namespace + '/' + r.name))

  try {
    const res = await previewFn({
      server_id: serverId.value,
      projects: allSelectedItems.map(r => ({ name: r.name, namespace: r.namespace }))
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleFull(action) {
  const previewFn = {
    online: k8sFullOnlinePreview,
    sync: k8sFullSyncPreview,
    rollback: k8sFullRollbackPreview
  }[action]

  try {
    const res = await previewFn({ server_id: serverId.value })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'full_' + action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  const executeFn = {
    online: k8sOnlineExecute,
    sync: k8sSyncExecute,
    rollback: k8sRollbackExecute,
    full_online: k8sFullOnlineExecute,
    full_sync: k8sFullSyncExecute,
    full_rollback: k8sFullRollbackExecute
  }[currentAction.value]

  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = Array.isArray(res.output) ? res.output.join('\n') : String(res.output ?? '')
    ElMessage.success('执行成功')
    previewVisible.value = false
    // 刷新数据，如果失败不影响执行成功的提示
    try {
      await loadData()
    } catch (e) {
      // loadData已经显示了错误提示
    }
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    const errOutput = e.response?.data?.output
    output.value = Array.isArray(errOutput) ? errOutput.join('\n') : String(errOutput ?? '')
  } finally {
    executing.value = false
  }
}
</script>
