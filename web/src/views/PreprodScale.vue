<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择预生产服务器" style="width: 150px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span style="margin-left: auto;"></span>
          <el-input v-model="search" placeholder="搜索类型/名称" clearable style="width: 250px;" />
        </div>
        <!-- 批量操作按钮 -->
        <div class="toolbar">
        <el-dropdown trigger="click" @command="onStatusFilter" style="margin-right: 12px;">
          <el-button type="info" class="el-button--cyan">{{ statusFilterLabel }}<el-icon style="margin-left: 4px;"><svg viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg"><path fill="currentColor" d="M831.872 340.864 512 652.672 192.128 340.864a30.592 30.592 0 0 0-42.752 0 29.12 29.12 0 0 0 0 41.6L489.664 714.24a32 32 0 0 0 44.672 0l340.288-331.712a29.12 29.12 0 0 0 0-41.728 30.592 30.592 0 0 0-42.752 0z"></path></svg></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="all">全部</el-dropdown-item>
              <el-dropdown-item command="up">已扩容</el-dropdown-item>
              <el-dropdown-item command="down">已缩容</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="info" class="el-button--cyan" @click="toggleSelectAll">{{ allSelected ? '取消' : '全选' }}</el-button>
        <el-button type="danger" :disabled="selectedIds.size > 0 ? !canBatchScaleDown : !canFullScaleDown" @click="handleBatchScaleDown">
          {{ selectedIds.size > 0 ? '批量缩容' : '全量缩容' }}
        </el-button>
        <el-button type="success" :disabled="selectedIds.size > 0 ? !canBatchScaleUp : !canFullScaleUp" @click="handleBatchScaleUp">
          {{ selectedIds.size > 0 ? '批量扩容' : '全量扩容' }}
        </el-button>
        <el-button type="info" class="el-button--cyan" @click="openBindingDialog">依赖配置</el-button>
        <el-button type="info" class="el-button--cyan" @click="handleRefresh">刷新</el-button>
        <span v-if="selectedIds.size > 0" style="margin-left: 10px; font-size: 13px; color: #909399;">
          已选 {{ selectedIds.size }} 项
        </span>
      </div>
      </template>

      <el-table ref="tableRef" :data="paginatedResources" :row-key="row => row.name" stripe border @selection-change="handleSelectionChange" v-force-reflow max-height="calc(100vh - 240px)">
        <el-table-column type="selection" width="45" />
        <el-table-column prop="category" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.category === 'rollout' ? 'primary' : row.category === 'deployment' ? 'success' : 'warning'">{{ row.category }}</el-tag>
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

    <!-- Batch Confirm Dialog (unified for normal and large batches) -->
    <el-dialog v-model="batchConfirmVisible" :title="batchConfirmTitle" width="580px" top="8vh">
      <div style="margin-bottom: 16px;">
        <el-alert v-if="batchConfirmNames.length > BATCH_THRESHOLD" type="warning" :closable="false" show-icon style="margin-bottom: 12px;">
          <template #title>
            当前 <b>{{ batchConfirmNames.length }}</b> 个资源，请输入 <b>确认执行</b> 以继续
          </template>
        </el-alert>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
          <span style="font-size: 14px; color: #303133;">{{ batchConfirmAction === 'scaledown' ? '以下资源将缩容至 0 副本:' : '以下资源将扩容至目标副本数:' }}</span>
          <el-tag size="small" type="info">共 {{ batchConfirmNames.length }} 项</el-tag>
        </div>
        <el-scrollbar max-height="320px">
          <div style="background: #f5f7fa; padding: 8px 12px; border-radius: 6px; border: 1px solid #e4e7ed;">
            <div v-for="(name, idx) in batchConfirmNames" :key="name"
              style="font-size: 13px; line-height: 2; padding: 0 4px; display: flex; align-items: center; border-bottom: 1px dashed #ebeef5;">
              <span style="color: #909399; font-size: 12px; margin-right: 8px; min-width: 28px;">{{ idx + 1 }}.</span>
              <span>{{ name }}</span>
            </div>
          </div>
        </el-scrollbar>
      </div>
      <div v-if="batchConfirmSkipCount > 0" style="margin-bottom: 12px;">
        <el-text type="info" size="small">
          {{ batchConfirmIsFull ? '共' : '已选' }} {{ batchConfirmTotalCount }} 项，其中 {{ batchConfirmSkipCount }} 项{{ batchConfirmAction === 'scaledown' ? '已缩容' : '已扩容' }}将跳过
        </el-text>
      </div>
      <el-input v-if="batchConfirmNames.length > BATCH_THRESHOLD" v-model="batchConfirmText" placeholder='请输入"确认执行"' />
      <template #footer>
        <el-button @click="batchConfirmVisible = false">取消</el-button>
        <el-button type="primary" :disabled="batchConfirmNames.length > BATCH_THRESHOLD && batchConfirmText !== '确认执行'" @click="onBatchConfirm">
          确认{{ batchConfirmAction === 'scaledown' ? '缩容' : '扩容' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Dependency Warning Dialog -->
    <el-dialog v-model="depWarningVisible" title="操作警告" width="520px" top="15vh" :close-on-click-modal="false">
      <div style="margin-bottom: 16px;">
        <div style="display: flex; align-items: flex-start; gap: 10px; margin-bottom: 16px;">
          <span style="color: #f56c6c; font-size: 20px; line-height: 1;">⚠</span>
          <div>
            <div style="color: #f56c6c; font-weight: bold; font-size: 14px; line-height: 1.6; margin-bottom: 8px;">
              {{ depWarningText }}
            </div>
            <div style="color: #606266; font-size: 13px; line-height: 1.6;">
              涉及资源：
            </div>
          </div>
        </div>
        <el-scrollbar max-height="200px">
          <div style="background: #fef0f0; padding: 8px 12px; border-radius: 6px; border: 1px solid #fbc4c4;">
            <div v-for="(name, idx) in depWarningAffected" :key="name"
              style="font-size: 13px; line-height: 2; padding: 0 4px; display: flex; align-items: center; border-bottom: 1px dashed #f9d7d7;">
              <span style="color: #909399; font-size: 12px; margin-right: 8px; min-width: 28px;">{{ idx + 1 }}.</span>
              <span>{{ name }}</span>
            </div>
          </div>
        </el-scrollbar>
        <div style="margin-top: 12px; color: #909399; font-size: 12px;">
          如果确认执行，请在下方输入框中输入 <b>确认执行</b>
        </div>
        <el-input v-model="depWarningConfirmText" placeholder='请输入"确认执行"' style="margin-top: 8px;" />
      </div>
      <template #footer>
        <el-button @click="depWarningVisible = false">取消</el-button>
        <el-button type="danger" :disabled="depWarningConfirmText !== '确认执行'" @click="onDepWarningConfirm">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Binding Config Dialog -->
    <el-dialog v-model="bindingDialogVisible" title="LVS-Preprod 依赖配置" width="min(650px, 90vw)" align-center>
      <div style="margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center;">
        <span style="font-size: 14px; color: #606266;">配置 VS 标签和 RS 环境标签的绑定关系</span>
        <el-button type="primary" size="small" @click="showAddBinding">新增绑定</el-button>
      </div>
      <el-table :data="bindings" stripe size="small" border max-height="400">
        <el-table-column prop="vs_tag" label="VS 标签" min-width="120" />
        <el-table-column prop="rs_env_tag" label="RS 环境标签" min-width="150" />
        <el-table-column label="操作" width="80" align="center">
          <template #default="{ row }">
            <el-button type="danger" link size="small" @click="handleDeleteBinding(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- Add binding sub-form -->
      <div v-if="addBindingVisible" style="margin-top: 16px; padding: 12px; background: #f5f7fa; border-radius: 6px; border: 1px solid #e4e7ed;">
        <el-form label-width="100px" size="small">
          <el-form-item label="VS 标签">
            <el-select v-model="newBinding.vs_tag" filterable allow-create clearable placeholder="选择或输入 VS 标签" style="width: 100%">
              <el-option v-for="opt in vsTagOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
          <el-form-item label="RS 环境标签">
            <el-select v-model="newBinding.rs_env_tag" filterable allow-create clearable placeholder="选择或输入 RS 环境标签" style="width: 100%">
              <el-option v-for="opt in rsTagOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
        </el-form>
        <div style="text-align: right; margin-top: 8px;">
          <el-button size="small" @click="addBindingVisible = false">取消</el-button>
          <el-button type="primary" size="small" :disabled="!newBinding.vs_tag || !newBinding.rs_env_tag" @click="handleAddBinding">保存</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- LVS Scale Down Check Warning Dialog -->
    <el-dialog v-model="lvsCheckVisible" title="缩容前检查" width="min(600px, 90vw)" align-center @close="handleLvsCheckCancel">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
        <template #title>
          以下 LVS RS 仍处于上线状态，缩容前请确认已下线：
        </template>
      </el-alert>
      <div v-if="lvsCheckWarnings">
        <el-table :data="lvsCheckWarnings" stripe size="small" border max-height="300">
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 'Up' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="vs_tag" label="VS 标签" width="120" />
          <el-table-column prop="rs_env_tag" label="RS标签" width="150" />
          <el-table-column prop="rs_ip" label="RS IP" width="140" />
          <el-table-column prop="lvs_server" label="LVS 服务器" min-width="120" />
        </el-table>
      </div>
      <el-alert type="info" :closable="false" style="margin-top: 12px;">
        请输入"确认执行"以继续缩容操作
      </el-alert>
      <el-input v-model="lvsCheckConfirmText" placeholder="请输入 确认执行" style="margin-top: 8px;" />
      <template #footer>
        <el-button @click="lvsCheckVisible = false">取消</el-button>
        <el-button type="primary" :disabled="lvsCheckConfirmText !== '确认执行'" @click="handleLvsCheckConfirm">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Streaming Output Area -->
    <StreamOutput
      v-if="streamStatus !== 'idle'"
      :lines="outputLines"
      :status="streamStatus"
      :showCancel="false"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { getServers, getPreprodStatus, preprodScaleDownPreview, preprodScaleUpPreview, getWebSocketUrl, getLvsBindings, updateLvsBinding, deleteLvsBinding, getLvsVSTags, getLvsTags, checkLvsForScaleDown } from '../api'
import { useWebSocket } from '../composables/useWebSocket'
import { useUserStore } from '../stores/user'
import StreamOutput from '../components/StreamOutput.vue'
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
const currentAction = ref('')

// Streaming state
const userStore = useUserStore()
const { outputLines, status: streamStatus, connect: wsConnect, disconnect: wsDisconnect } = useWebSocket()
const outputCache = new Map()
const search = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const skipSelectionSync = ref(false)
const statusFilter = ref('all')
const statusFilterLabels = { all: '全部', up: '已扩容', down: '已缩容' }
const statusFilterLabel = computed(() => statusFilterLabels[statusFilter.value] || '全部')

// Batch confirm
const batchConfirmVisible = ref(false)
const batchConfirmText = ref('')
const batchConfirmNames = ref([])
const batchConfirmAction = ref('')
const batchConfirmSkipCount = ref(0)
const batchConfirmTotalCount = ref(0)
const batchConfirmIsFull = ref(false)

const batchConfirmTitle = computed(() => {
  const action = batchConfirmAction.value === 'scaledown' ? '缩容' : '扩容'
  const mode = batchConfirmIsFull.value ? '全量' : '批量'
  return `${mode}${action}确认`
})

// Dependency warning
const depWarningVisible = ref(false)
const depWarningText = ref('')
const depWarningAffected = ref([])
const depWarningConfirmText = ref('')
const depWarningCallback = ref(null)

// Binding config
const bindingDialogVisible = ref(false)
const bindings = ref([])
const addBindingVisible = ref(false)
const newBinding = ref({ vs_tag: '', rs_env_tag: '' })
const vsTagOptions = ref([])
const rsTagOptions = ref([])

// LVS scale down check
const lvsCheckVisible = ref(false)
const lvsCheckWarnings = ref(null)
const lvsCheckConfirmText = ref('')
const lvsCheckCallback = ref(null)

const requireSet = computed(() => new Set(resources.value.filter(r => r.category === 'require').map(r => r.name)))

const filteredResources = computed(() => {
  let list = resources.value
  if (statusFilter.value === 'up') {
    list = list.filter(r => r.current >= r.target_replicas)
  } else if (statusFilter.value === 'down') {
    list = list.filter(r => r.current < r.target_replicas)
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
  selectedResources.value.some(r => r.current < r.target_replicas)
)

// 全量操作可用性
const canFullScaleDown = computed(() =>
  resources.value.some(r => r.current > 0)
)

const canFullScaleUp = computed(() =>
  resources.value.some(r => r.current < r.target_replicas)
)


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

function onStatusFilter(cmd) {
  statusFilter.value = cmd
  currentPage.value = 1
}

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
  try {
    await loadData()
    ElMessage.success('刷新成功')
  } catch (e) {
    // loadData 已处理错误提示
  }
}

// 切换服务器时，缓存/恢复执行结果
watch(serverId, (newVal, oldVal) => {
  if (oldVal != null) {
    outputCache.set(oldVal, [...outputLines.value])
  }
  const cached = outputCache.get(newVal)
  outputLines.value = cached ? [...cached] : []
})

onMounted(async () => {
  try {
    servers.value = (await getServers('preprod')) || []
    if (servers.value.length > 0) {
      const saved = localStorage.getItem('preprod_server')
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
})

onBeforeUnmount(() => {
  wsDisconnect()
})

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem('preprod_server', serverId.value)
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

function showDepWarning(text, affected, callback) {
  depWarningText.value = text
  depWarningAffected.value = affected
  depWarningConfirmText.value = ''
  depWarningCallback.value = callback
  depWarningVisible.value = true
}

function onDepWarningConfirm() {
  depWarningVisible.value = false
  depWarningCallback.value?.()
}

async function openBindingDialog() {
  if (!serverId.value) {
    ElMessage.warning('请先选择服务器')
    return
  }
  addBindingVisible.value = false
  try {
    const [bindingList, vsTags, rsTags] = await Promise.all([
      getLvsBindings({ preprod_server_id: serverId.value }),
      getLvsVSTags(),
      getLvsTags(),
    ])
    bindings.value = bindingList || []
    vsTagOptions.value = [...new Set((vsTags || []).map(t => t.tag).filter(Boolean))]
    rsTagOptions.value = [...new Set((rsTags || []).map(t => t.tag).filter(Boolean))]
    bindingDialogVisible.value = true
  } catch (e) {
    ElMessage.error('加载绑定配置失败')
  }
}

function showAddBinding() {
  newBinding.value = { vs_tag: '', rs_env_tag: '' }
  addBindingVisible.value = true
}

async function handleAddBinding() {
  try {
    await updateLvsBinding({
      vs_tag: newBinding.value.vs_tag,
      rs_env_tag: newBinding.value.rs_env_tag,
      preprod_server_id: serverId.value,
    })
    ElMessage.success('绑定已保存')
    addBindingVisible.value = false
    // Reload bindings
    const list = await getLvsBindings({ preprod_server_id: serverId.value })
    bindings.value = list || []
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  }
}

async function handleDeleteBinding(id) {
  try {
    await ElMessageBox.confirm('确定要删除该绑定吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteLvsBinding(id)
    ElMessage.success('绑定已删除')
    bindings.value = bindings.value.filter(b => b.id !== id)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

function handleLvsCheckConfirm() {
  lvsCheckVisible.value = false
  if (lvsCheckCallback.value) {
    lvsCheckCallback.value(true)
    lvsCheckCallback.value = null
  }
}

function handleLvsCheckCancel() {
  if (lvsCheckCallback.value) {
    lvsCheckCallback.value(false)
    lvsCheckCallback.value = null
  }
}

async function handleBatchScaleDown() {
  const isFull = selectedIds.value.size === 0
  const pool = isFull ? filteredResources.value : selectedResources.value
  const targets = pool.filter(r => r.current > 0)
  const skipCount = pool.filter(r => r.current === 0).length
  const names = targets.map(r => r.name)
  if (names.length === 0) return

  // 检查 LVS RS 状态依赖
  try {
    const checkRes = await checkLvsForScaleDown({ preprod_server_id: serverId.value })
    if (checkRes.need_warning) {
      lvsCheckWarnings.value = checkRes.warnings
      lvsCheckVisible.value = true
      lvsCheckConfirmText.value = ''
      const confirmed = await new Promise(resolve => {
        lvsCheckCallback.value = resolve
      })
      if (!confirmed) return
    }
  } catch {
    // 检查失败不阻塞操作
  }

  // 全量操作时脚本自动处理依赖，跳过警告
  if (!isFull) {
    // 缩容 require 资源时，检查非 require 资源是否仍在运行
    const requireTargets = names.filter(n => requireSet.value.has(n))
    if (requireTargets.length > 0) {
      const nonRequireStillRunning = resources.value
        .filter(r => !requireSet.value.has(r.name) && r.current > 0)
        .map(r => r.name)
      // 过滤掉已包含在本次缩容列表中的非 require 资源
      const stillRunning = nonRequireStillRunning.filter(n => !names.includes(n))
      if (stillRunning.length > 0) {
        showDepWarning(
          '依赖(require)服务停止可能会影响其它服务运行！',
          stillRunning,
          () => doBatchScaleDown(names, skipCount, pool.length, isFull)
        )
        return
      }
    }
  }

  doBatchScaleDown(names, skipCount, pool.length, isFull)
}

function doBatchScaleDown(names, skipCount, total, isFull) {
  batchConfirmNames.value = names
  batchConfirmAction.value = 'scaledown'
  batchConfirmText.value = ''
  batchConfirmSkipCount.value = skipCount
  batchConfirmTotalCount.value = total
  batchConfirmIsFull.value = isFull
  batchConfirmVisible.value = true
}

async function handleBatchScaleUp() {
  const isFull = selectedIds.value.size === 0
  const pool = isFull ? filteredResources.value : selectedResources.value
  const targets = pool.filter(r => r.current < r.target_replicas)
  const skipCount = pool.filter(r => r.current >= r.target_replicas).length
  const names = targets.map(r => r.name)
  if (names.length === 0) return

  // 全量操作时脚本自动处理依赖，跳过警告
  if (!isFull) {
    // 扩容非 require 资源时，检查 require 资源是否都在运行
    const nonRequireTargets = names.filter(n => !requireSet.value.has(n))
    if (nonRequireTargets.length > 0) {
      const requireNotRunning = resources.value
        .filter(r => requireSet.value.has(r.name) && r.current === 0)
        .map(r => r.name)
      // 过滤掉已包含在本次扩容列表中的 require 资源
      const stillMissing = requireNotRunning.filter(n => !names.includes(n))
      if (stillMissing.length > 0) {
        showDepWarning(
          '依赖(require)服务未运行，运行所选服务可能会发生异常！',
          stillMissing,
          () => doBatchScaleUp(names, skipCount, pool.length, isFull)
        )
        return
      }
    }
  }

  doBatchScaleUp(names, skipCount, pool.length, isFull)
}

function doBatchScaleUp(names, skipCount, total, isFull) {
  batchConfirmNames.value = names
  batchConfirmAction.value = 'scaleup'
  batchConfirmText.value = ''
  batchConfirmSkipCount.value = skipCount
  batchConfirmTotalCount.value = total
  batchConfirmIsFull.value = isFull
  batchConfirmVisible.value = true
}

async function onBatchConfirm() {
  const action = batchConfirmAction.value
  // 全量模式传空数组，让脚本操作全部资源；批量模式传选中的资源名
  const names = batchConfirmIsFull.value ? [] : batchConfirmNames.value
  batchConfirmVisible.value = false
  await doPreview(action, names)
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

function executePreview() {
  executing.value = true
  previewVisible.value = false

  const url = getWebSocketUrl('/api/ws/exec', userStore.token)
  wsConnect(url, previewId.value, {
    token: userStore.token,
    onDone: async () => {
      executing.value = false
      ElMessage.success('执行成功')
      await loadData()
    },
    onError: (msg) => {
      executing.value = false
      ElMessage.error(msg || '执行失败')
    },
    onLockError: (msg) => {
      executing.value = false
      ElMessage.error(msg)
    },
  })
}
</script>

<style scoped>
:deep(.el-card__header) {
  border-bottom: none;
  padding-bottom: 0;
}
.text-warning {
  color: #e6a23c;
  font-weight: bold;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 12px;
}
.toolbar :deep(.el-dropdown) {
  display: inline-flex;
}
</style>
