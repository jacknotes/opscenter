<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span style="white-space: nowrap;">服务器：</span>
          <el-select v-model="serverId" placeholder="选择预生产服务器" style="width: 280px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </div>
      </template>

      <!-- 批量操作按钮 -->
      <div style="margin-bottom: 15px; display: flex; gap: 10px;">
        <el-button type="primary" @click="toggleSelectAll">{{ isAllSelected ? '取消选择' : '全选' }}</el-button>
        <el-button type="success" @click="loadData">刷新</el-button>
        <el-button type="danger" :disabled="selectedRows.length === 0 || !canBatchScaleDown" @click="handleBatchScaleDown">
          批量缩容 ({{ selectedRows.filter(r => r.current > 0).length }})
        </el-button>
        <el-button type="success" :disabled="selectedRows.length === 0 || !canBatchScaleUp" @click="handleBatchScaleUp">
          批量扩容 ({{ selectedRows.filter(r => r.current === 0).length }})
        </el-button>
      </div>

      <el-table ref="tableRef" :data="resources" stripe border @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="45" :selectable="() => true" />
        <el-table-column prop="category" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.category === 'rollout' ? '' : row.category === 'deployment' ? 'success' : 'warning'">{{ row.category }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="300" />
        <el-table-column prop="current" label="当前副本" width="90" align="center" />
        <el-table-column prop="target_replicas" label="目标副本" width="90" align="center">
          <template #default="{ row }">
            <span :class="{ 'text-warning': row.current > 0 && row.current !== row.target_replicas }">
              {{ row.target_replicas || '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="available" label="可用副本" width="90" align="center" />
        <el-table-column prop="age" label="年龄" width="100" />
        <el-table-column label="操作" width="120" align="center" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.current > 0" type="danger" size="small" link @click="handleSingleScaleDown(row)">
              缩容
            </el-button>
            <el-button v-else type="success" size="small" link @click="handleSingleScaleUp(row)">
              扩容
            </el-button>
          </template>
        </el-table-column>
      </el-table>
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

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>执行结果</template>
      <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 4px; max-height: 400px; overflow-y: auto;">{{ output }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getServers, getPreprodStatus, preprodScaleDownPreview, preprodScaleDownExecute, preprodScaleUpPreview, preprodScaleUpExecute } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const resources = ref([])
const selectedRows = ref([])
const tableRef = ref(null)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')

const canBatchScaleDown = computed(() => selectedRows.value.some(r => r.current > 0))
const canBatchScaleUp = computed(() => selectedRows.value.some(r => r.current === 0))
const isAllSelected = computed(() => resources.value.length > 0 && selectedRows.value.length === resources.value.length)

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
    clearSelection()
  } catch (e) {
    ElMessage.error('加载数据失败')
  }
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
}

function clearSelection() {
  tableRef.value?.clearSelection()
  selectedRows.value = []
}

function toggleSelectAll() {
  if (isAllSelected.value) {
    clearSelection()
  } else {
    resources.value.forEach(row => tableRef.value?.toggleRowSelection(row, true))
  }
}

async function handleSingleScaleDown(row) {
  try {
    await ElMessageBox.confirm(`确认缩容 ${row.name} 至 0 副本？`, '确认缩容', { type: 'warning' })
    await doPreview('scaledown', [row.name])
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.response?.data?.error || '操作失败')
  }
}

async function handleSingleScaleUp(row) {
  try {
    await ElMessageBox.confirm(`确认扩容 ${row.name} 至 ${row.target_replicas} 副本？`, '确认扩容', { type: 'warning' })
    await doPreview('scaleup', [row.name])
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.response?.data?.error || '操作失败')
  }
}

async function handleBatchScaleDown() {
  const names = selectedRows.value.filter(r => r.current > 0).map(r => r.name)
  if (names.length === 0) return
  try {
    await ElMessageBox.confirm(`确认缩容以下 ${names.length} 个资源至 0 副本？\n\n${names.join('\n')}`, '批量缩容', { type: 'warning' })
    await doPreview('scaledown', names)
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.response?.data?.error || '操作失败')
  }
}

async function handleBatchScaleUp() {
  const names = selectedRows.value.filter(r => r.current === 0).map(r => r.name)
  if (names.length === 0) return
  try {
    await ElMessageBox.confirm(`确认扩容以下 ${names.length} 个资源至目标副本数？\n\n${names.join('\n')}`, '批量扩容', { type: 'warning' })
    await doPreview('scaleup', names)
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.response?.data?.error || '操作失败')
  }
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
