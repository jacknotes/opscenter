<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span style="white-space: nowrap;">服务器：</span>
          <el-select v-model="serverId" placeholder="选择预生产服务器" style="width: 280px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-input v-model="search" placeholder="搜索类型/名称" clearable style="width: 220px;" />
          <el-radio-group v-model="statusFilter" @change="currentPage = 1">
            <el-radio-button value="all">全部</el-radio-button>
            <el-radio-button value="up">已扩容</el-radio-button>
            <el-radio-button value="down">已缩容</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <!-- 批量操作按钮 -->
      <div style="margin-bottom: 15px; display: flex; gap: 10px; align-items: center;">
        <el-button type="primary" @click="toggleSelectAll">{{ allSelected ? '取消选择' : '全选' }}</el-button>
        <el-button type="success" @click="handleRefresh">刷新</el-button>
        <el-button type="danger" :disabled="selectedIds.size === 0 || !canBatchScaleDown" @click="handleBatchScaleDown">
          批量缩容
        </el-button>
        <el-button type="success" :disabled="selectedIds.size === 0 || !canBatchScaleUp" @click="handleBatchScaleUp">
          批量扩容
        </el-button>
        <span v-if="selectedIds.size > 0" style="margin-left: 10px; font-size: 13px; color: #909399;">
          已选 {{ selectedIds.size }} 项
          <template v-if="batchSkipDown > 0">，{{ batchSkipDown }} 项已缩容将跳过</template>
          <template v-if="batchSkipUp > 0">，{{ batchSkipUp }} 项已扩容将跳过</template>
        </span>
      </div>

      <el-table ref="tableRef" :data="paginatedResources" :row-key="row => row.name" stripe border @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="45" />
        <el-table-column prop="category" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.category === 'rollout' ? '' : row.category === 'deployment' ? 'success' : 'warning'">{{ row.category }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="300" />
        <el-table-column prop="current" label="当前副本" width="100" align="center">
          <template #default="{ row }">
            <span>{{ row.current }}</span>
            <el-tag v-if="row.current === 0" type="info" size="small" style="margin-left: 4px;">已缩容</el-tag>
            <el-tag v-else-if="row.current > 0 && row.current === row.target_replicas" type="success" size="small" style="margin-left: 4px;">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_replicas" label="目标副本" width="90" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-warning': row.current > 0 && row.current !== row.target_replicas }">
              {{ row.target_replicas || '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="available" label="可用副本" width="90" align="center" />
        <el-table-column prop="age" label="年龄" width="100" />
      </el-table>

      <div style="margin-top: 15px; display: flex; justify-content: flex-end;">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="filteredResources.length"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="650px">
      <div v-if="previewData">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <p><strong>命令：</strong><code>{{ previewData.command }}</code></p>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Large Batch Confirm Dialog -->
    <el-dialog v-model="batchConfirmVisible" title="批量操作确认" width="500px">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
        <template #title>
          当前选中 <b>{{ batchConfirmNames.length }}</b> 个资源，请输入 <b>确认执行</b> 以继续
        </template>
      </el-alert>
      <div style="max-height: 200px; overflow-y: auto; background: #f5f7fa; padding: 10px; border-radius: 4px; margin-bottom: 16px;">
        <div v-for="name in batchConfirmNames" :key="name" style="font-size: 13px; line-height: 1.8;">{{ name }}</div>
      </div>
      <el-input v-model="batchConfirmText" placeholder='请输入"确认执行"' />
      <template #footer>
        <el-button @click="batchConfirmVisible = false">取消</el-button>
        <el-button type="primary" :disabled="batchConfirmText !== '确认执行'" @click="onBatchConfirm">确认</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>执行结果</template>
      <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 4px; max-height: 400px; overflow-y: auto;">{{ output }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { getServers, getPreprodStatus, preprodScaleDownPreview, preprodScaleDownExecute, preprodScaleUpPreview, preprodScaleUpExecute } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const BATCH_THRESHOLD = 10

const servers = ref([])
const serverId = ref(null)
const resources = ref([])
const selectedIds = ref(new Set())
const tableRef = ref(null)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')
const search = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const skipSelectionSync = ref(false)
const statusFilter = ref('all')

// Large batch confirm
const batchConfirmVisible = ref(false)
const batchConfirmText = ref('')
const batchConfirmNames = ref([])
const batchConfirmAction = ref('')

const filteredResources = computed(() => {
  let list = resources.value
  if (statusFilter.value === 'up') {
    list = list.filter(r => r.current > 0)
  } else if (statusFilter.value === 'down') {
    list = list.filter(r => r.current === 0)
  }
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(r =>
      r.category.toLowerCase().includes(q) ||
      r.name.toLowerCase().includes(q)
    )
  }
  return list
})

const paginatedResources = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredResources.value.slice(start, start + pageSize.value)
})

const allSelected = computed(() =>
  filteredResources.value.length > 0 && filteredResources.value.every(r => selectedIds.value.has(r.name))
)

const selectedResources = computed(() =>
  filteredResources.value.filter(r => selectedIds.value.has(r.name))
)

const canBatchScaleDown = computed(() =>
  selectedResources.value.some(r => r.current > 0)
)

const canBatchScaleUp = computed(() =>
  selectedResources.value.some(r => r.current === 0)
)

const batchSkipDown = computed(() => {
  if (selectedIds.value.size === 0) return 0
  return selectedResources.value.filter(r => r.current === 0).length
})

const batchSkipUp = computed(() => {
  if (selectedIds.value.size === 0) return 0
  return selectedResources.value.filter(r => r.current > 0).length
})

function handleSizeChange(size) {
  pageSize.value = size
  currentPage.value = 1
  restoreSelection()
}

function handleCurrentChange() {
  restoreSelection()
}

function restoreSelection() {
  skipSelectionSync.value = true
  setTimeout(() => {
    paginatedResources.value.forEach(row => {
      if (selectedIds.value.has(row.name)) {
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
    filteredResources.value.forEach(row => {
      selectedIds.value.add(row.name)
    })
    paginatedResources.value.forEach(row => {
      tableRef.value.toggleRowSelection(row, true)
    })
  }
}

function toggleSelectAll() {
  handleToggleSelect()
}

async function handleRefresh() {
  await loadData()
  ElMessage.success('刷新成功')
}

onMounted(async () => {
  try {
    servers.value = await getServers('preprod')
    if (servers.value.length > 0) {
      serverId.value = servers.value[0].id
      await loadData()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function loadData() {
  if (!serverId.value) return
  try {
    resources.value = await getPreprodStatus(serverId.value)
    selectedIds.value.clear()
    tableRef.value?.clearSelection()
  } catch (e) {
    ElMessage.error('加载数据失败')
  }
}

function handleSelectionChange(rows) {
  if (skipSelectionSync.value) return
  const pageKeys = paginatedResources.value.map(r => r.name)
  pageKeys.forEach(key => selectedIds.value.delete(key))
  rows.forEach(r => selectedIds.value.add(r.name))
}

async function handleBatchScaleDown() {
  const targets = selectedResources.value.filter(r => r.current > 0)
  const skipCount = selectedResources.value.filter(r => r.current === 0).length
  const names = targets.map(r => r.name)
  if (names.length === 0) return

  let msg = `确认缩容以下 ${names.length} 个资源至 0 副本？`
  if (skipCount > 0) {
    msg += `\n\n（已选 ${selectedIds.value.size} 项，其中 ${skipCount} 项已缩容将跳过）`
  }
  msg += `\n\n${names.join('\n')}`

  try {
    await ElMessageBox.confirm(msg, '批量缩容', { type: 'warning' })
  } catch (e) {
    if (e === 'cancel') return
  }

  if (names.length > BATCH_THRESHOLD) {
    batchConfirmNames.value = names
    batchConfirmAction.value = 'scaledown'
    batchConfirmText.value = ''
    batchConfirmVisible.value = true
    return
  }

  await doPreview('scaledown', names)
}

async function handleBatchScaleUp() {
  const targets = selectedResources.value.filter(r => r.current === 0)
  const skipCount = selectedResources.value.filter(r => r.current > 0).length
  const names = targets.map(r => r.name)
  if (names.length === 0) return

  let msg = `确认扩容以下 ${names.length} 个资源至目标副本数？`
  if (skipCount > 0) {
    msg += `\n\n（已选 ${selectedIds.value.size} 项，其中 ${skipCount} 项已扩容将跳过）`
  }
  msg += `\n\n${names.join('\n')}`

  try {
    await ElMessageBox.confirm(msg, '批量扩容', { type: 'warning' })
  } catch (e) {
    if (e === 'cancel') return
  }

  if (names.length > BATCH_THRESHOLD) {
    batchConfirmNames.value = names
    batchConfirmAction.value = 'scaleup'
    batchConfirmText.value = ''
    batchConfirmVisible.value = true
    return
  }

  await doPreview('scaleup', names)
}

async function onBatchConfirm() {
  batchConfirmVisible.value = false
  await doPreview(batchConfirmAction.value, batchConfirmNames.value)
}

async function doPreview(action, resourceNames) {
  const previewFn = action === 'scaledown' ? preprodScaleDownPreview : preprodScaleUpPreview
  try {
    const res = await previewFn({ server_id: serverId.value, resource_names: resourceNames })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  const executeFn = currentAction.value === 'scaledown' ? preprodScaleDownExecute : preprodScaleUpExecute

  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = res.output
    ElMessage.success('执行成功')
    previewVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = e.response?.data?.output || ''
  } finally {
    executing.value = false
  }
}
</script>

<style scoped>
.text-warning {
  color: #e6a23c;
  font-weight: bold;
}
</style>
