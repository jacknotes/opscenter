<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px">
          <span class="filter-label">服务器:</span>
          <el-select
            v-model="serverId"
            placeholder="选择预生产服务器"
            style="width: 150px"
            @change="handleServerChange"
          >
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span style="margin-left: auto"></span>
          <el-input v-model="search" placeholder="搜索类型/名称" clearable style="width: 250px" />
        </div>
        <!-- 批量操作按钮 -->
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
                <el-dropdown-item command="up">已扩容</el-dropdown-item>
                <el-dropdown-item command="down">已缩容</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="info" class="el-button--cyan" @click="handleToggleSelect">{{
            allSelected ? '取消' : '全选'
          }}</el-button>
          <el-button
            type="danger"
            :disabled="selectedIds.size > 0 ? !canBatchScaleDown : !canFullScaleDown"
            @click="handleBatchScaleDown"
          >
            {{ selectedIds.size > 0 ? '批量缩容' : '全量缩容' }}
          </el-button>
          <el-button
            type="success"
            :disabled="selectedIds.size > 0 ? !canBatchScaleUp : !canFullScaleUp"
            @click="handleBatchScaleUp"
          >
            {{ selectedIds.size > 0 ? '批量扩容' : '全量扩容' }}
          </el-button>
          <el-button type="info" class="el-button--cyan" @click="openBindingDialog">依赖配置</el-button>
          <el-button type="info" class="el-button--cyan" @click="handleRefresh">刷新</el-button>
        </div>
      </template>

      <el-table
        ref="tableRef"
        v-force-reflow
        v-loading="loading"
        :data="paginatedResources"
        :row-key="(row) => row.name"
        stripe
        border
        max-height="calc(100vh - 280px)"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="45" />
        <el-table-column prop="category" label="类型" width="100" sortable>
          <template #default="{ row }">
            <el-tag
              :type="row.category === 'rollout' ? 'primary' : row.category === 'deployment' ? 'success' : 'warning'"
              >{{ row.category }}</el-tag
            >
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="300" sortable />
        <el-table-column prop="current" label="当前副本" width="100" align="center" sortable>
          <template #default="{ row }">
            <span>{{ row.current }}</span>
            <el-tag v-if="row.ready_desired === 0 && row.ready === 0" type="info" size="small" style="margin-left: 4px">已缩容</el-tag>
            <el-tag
              v-else-if="row.ready > 0 && row.ready === row.ready_desired"
              type="success"
              size="small"
              style="margin-left: 4px"
              >正常</el-tag
            >
            <el-tag v-else type="warning" size="small" style="margin-left: 4px">启动中</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_replicas" label="目标副本" width="90" align="center" sortable>
          <template #default="{ row }">
            <span :class="{ 'text-warning': row.ready_desired > 0 && row.ready < row.ready_desired }">
              {{ row.target_replicas || '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="ready" label="就绪副本" width="100" align="center" sortable>
          <template #default="{ row }">
            <span>{{ row.ready }}/{{ row.ready_desired }}</span>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="!loading && paginatedResources.length === 0" class="empty-state">
        <el-icon class="empty-state-icon"><ZoomOut /></el-icon>
        <span class="empty-state-text">{{ search || statusFilter !== 'all' ? '没有匹配的资源' : '暂无资源数据' }}</span>
      </div>

      <div class="pagination-wrapper">
        <div class="pagination-left">
          <span v-if="selectedIds.size > 0" class="selection-count">已选 {{ selectedIds.size }} 项</span>
        </div>
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

      <!-- Streaming Output Area -->
      <div v-if="wsStore.status !== 'idle'" class="output-section">
        <div class="stream-header">
          <span class="chart-title">执行输出</span>
        </div>
        <StreamOutput
          :lines="wsStore.outputLines"
          :status="wsStore.status"
          :show-cancel="false"
        />
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="min(650px, 90vw)" align-center>
      <div v-if="previewData">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <p>
          <strong>命令：</strong><code>{{ previewData.command }}</code>
        </p>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre class="preview-pre">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Batch Confirm Dialog (unified for normal and large batches) -->
    <el-dialog v-model="batchConfirmVisible" :title="batchConfirmTitle" width="min(580px, 90vw)" align-center>
      <div style="margin-bottom: 16px">
        <el-alert
          v-if="batchConfirmNames.length > BATCH_THRESHOLD"
          type="warning"
          :closable="false"
          show-icon
          style="margin-bottom: 12px"
        >
          <template #title>
            当前 <b>{{ batchConfirmNames.length }}</b> 个资源，请输入 <b>确认执行</b> 以继续
          </template>
        </el-alert>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
          <span style="font-size: 14px; color: var(--text-primary)">{{
            batchConfirmAction === 'scaledown' ? '以下资源将缩容至 0 副本:' : '以下资源将扩容至目标副本数:'
          }}</span>
          <el-tag size="small" type="info">共 {{ batchConfirmNames.length }} 项</el-tag>
        </div>
        <el-scrollbar max-height="320px">
          <div
            style="
              background: var(--bg-elevated);
              padding: 8px 12px;
              border-radius: 6px;
              border: 1px solid var(--border-default);
            "
          >
            <div
              v-for="(name, idx) in batchConfirmNames"
              :key="name"
              style="
                font-size: 13px;
                line-height: 2;
                padding: 0 4px;
                display: flex;
                align-items: center;
                border-bottom: 1px dashed var(--border-default);
              "
            >
              <span style="color: #64748b; font-size: 12px; margin-right: 8px; min-width: 28px">{{ idx + 1 }}.</span>
              <span>{{ name }}</span>
            </div>
          </div>
        </el-scrollbar>
      </div>
      <div v-if="batchConfirmSkipCount > 0" style="margin-bottom: 12px">
        <el-text type="info" size="small">
          {{ batchConfirmIsFull ? '共' : '已选' }} {{ batchConfirmTotalCount }} 项，其中
          {{ batchConfirmSkipCount }} 项{{ batchConfirmAction === 'scaledown' ? '已缩容' : '已扩容' }}将跳过
        </el-text>
      </div>
      <el-input
        v-if="batchConfirmNames.length > BATCH_THRESHOLD"
        v-model="batchConfirmText"
        placeholder='请输入"确认执行"'
      />
      <template #footer>
        <el-button @click="batchConfirmVisible = false">取消</el-button>
        <el-button
          type="primary"
          :disabled="batchConfirmNames.length > BATCH_THRESHOLD && batchConfirmText !== '确认执行'"
          @click="onBatchConfirm"
        >
          确认{{ batchConfirmAction === 'scaledown' ? '缩容' : '扩容' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Dependency Warning Dialog -->
    <el-dialog v-model="depWarningVisible" title="操作警告" width="min(520px, 90vw)" align-center :close-on-click-modal="false">
      <div style="margin-bottom: 16px">
        <div style="display: flex; align-items: flex-start; gap: 10px; margin-bottom: 16px">
          <span style="color: #f56c6c; font-size: 20px; line-height: 1">⚠</span>
          <div>
            <div style="color: #f56c6c; font-weight: bold; font-size: 14px; line-height: 1.6; margin-bottom: 8px">
              {{ depWarningText }}
            </div>
            <div style="color: #94a3b8; font-size: 13px; line-height: 1.6">涉及资源：</div>
          </div>
        </div>
        <el-scrollbar max-height="200px">
          <div
            style="
              background: rgba(245, 108, 108, 0.1);
              padding: 8px 12px;
              border-radius: 6px;
              border: 1px solid rgba(245, 108, 108, 0.3);
            "
          >
            <div
              v-for="(name, idx) in depWarningAffected"
              :key="name"
              style="
                font-size: 13px;
                line-height: 2;
                padding: 0 4px;
                display: flex;
                align-items: center;
                border-bottom: 1px dashed rgba(245, 108, 108, 0.2);
              "
            >
              <span style="color: #64748b; font-size: 12px; margin-right: 8px; min-width: 28px">{{ idx + 1 }}.</span>
              <span>{{ name }}</span>
            </div>
          </div>
        </el-scrollbar>
        <div style="margin-top: 12px; color: #64748b; font-size: 12px">
          如果确认执行，请在下方输入框中输入 <b>确认执行</b>
        </div>
        <el-input v-model="depWarningConfirmText" placeholder='请输入"确认执行"' style="margin-top: 8px" />
      </div>
      <template #footer>
        <el-button @click="depWarningVisible = false">取消</el-button>
        <el-button type="danger" :disabled="depWarningConfirmText !== '确认执行'" @click="onDepWarningConfirm"
          >确认执行</el-button
        >
      </template>
    </el-dialog>

    <!-- Binding Config Dialog -->
    <el-dialog v-model="bindingDialogVisible" title="LVS-Preprod 依赖配置" width="min(650px, 90vw)" align-center>
      <div style="margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center">
        <span style="font-size: 14px; color: #94a3b8">配置 VS 标签和 RS 环境标签的绑定关系</span>
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
      <div
        v-if="addBindingVisible"
        style="
          margin-top: 16px;
          padding: 12px;
          background: var(--bg-elevated);
          border-radius: 6px;
          border: 1px solid var(--border-default);
        "
      >
        <el-form label-width="100px" size="small">
          <el-form-item label="VS 标签">
            <el-select
              v-model="newBinding.vs_tag"
              filterable
              allow-create
              clearable
              placeholder="选择或输入 VS 标签"
              style="width: 100%"
            >
              <el-option v-for="opt in vsTagOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
          <el-form-item label="RS 环境标签">
            <el-select
              v-model="newBinding.rs_env_tag"
              filterable
              allow-create
              clearable
              placeholder="选择或输入 RS 环境标签"
              style="width: 100%"
            >
              <el-option v-for="opt in rsTagOptions" :key="opt" :label="opt" :value="opt" />
            </el-select>
          </el-form-item>
        </el-form>
        <div style="text-align: right; margin-top: 8px">
          <el-button size="small" @click="addBindingVisible = false">取消</el-button>
          <el-button
            type="primary"
            size="small"
            :disabled="!newBinding.vs_tag || !newBinding.rs_env_tag"
            @click="handleAddBinding"
            >保存</el-button
          >
        </div>
      </div>
    </el-dialog>

    <!-- LVS Scale Down Check Warning Dialog -->
    <el-dialog
      v-model="lvsCheckVisible"
      title="缩容前检查"
      width="min(600px, 90vw)"
      align-center
      @close="handleLvsCheckCancel"
    >
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        <template #title> 以下 LVS RS 仍处于上线状态，缩容前请确认已下线： </template>
      </el-alert>
      <div v-if="lvsCheckWarnings">
        <el-table :data="lvsCheckWarnings" stripe size="small" border max-height="300">
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 'Up' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="lvs_server" label="LVS服务器" min-width="100" />
          <el-table-column prop="vs_tag" label="VS标签" min-width="100" />
          <el-table-column prop="rs_env_tag" label="RS标签" min-width="100" />
          <el-table-column prop="rs_ip" label="RS IP" min-width="120" />
        </el-table>
      </div>
      <el-alert type="info" :closable="false" style="margin-top: 12px"> 请输入"确认执行"以继续缩容操作 </el-alert>
      <el-input v-model="lvsCheckConfirmText" placeholder="请输入 确认执行" style="margin-top: 8px" />
      <template #footer>
        <el-button @click="lvsCheckVisible = false">取消</el-button>
        <el-button type="primary" :disabled="lvsCheckConfirmText !== '确认执行'" @click="handleLvsCheckConfirm"
          >确认执行</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, watch, onMounted, onActivated } from 'vue'
import {
  getPreprodStatus,
  preprodScaleDownPreview,
  preprodScaleUpPreview,
  getWebSocketUrl,
  getLvsBindings,
  updateLvsBinding,
  deleteLvsBinding,
  getLvsVSTags,
  getLvsTags,
  checkLvsForScaleDown,
} from '../api'
import { useWebSocketStore } from '../stores/websocket'
import { useUserStore } from '../stores/user'
import { useServerSelector } from '../composables/useServerSelector'
import { useSelection } from '../composables/useSelection'
import StreamOutput from '../components/StreamOutput.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { showLoadError } from '../utils/message'
import { ZoomOut } from '@element-plus/icons-vue'
import { STORAGE_KEYS, DEFAULT_PAGE_SIZE } from '../utils/constants'

const BATCH_THRESHOLD = 10

const resources = shallowRef([])
const search = ref('')
const currentPage = ref(1)
const pageSize = ref(DEFAULT_PAGE_SIZE)
const statusFilter = ref('all')
const loading = ref(false)
const statusFilterLabels = { all: '全部', up: '已扩容', down: '已缩容' }
const statusFilterLabel = computed(() => statusFilterLabels[statusFilter.value] || '全部')

// --- 必须在 composable 调用之前定义，避免暂时性死区 ---
const filteredResources = computed(() => {
  let list = resources.value
  if (statusFilter.value === 'up') {
    list = list.filter((r) => r.ready_desired > 0 && r.ready >= r.ready_desired)
  } else if (statusFilter.value === 'down') {
    list = list.filter((r) => r.ready_desired === 0 && r.ready === 0)
  }
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter((r) => r.category.toLowerCase().includes(q) || r.name.toLowerCase().includes(q))
  }
  return list
})

const paginatedResources = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredResources.value.slice(start, start + pageSize.value)
})

// --- 组合式函数 ---
const {
  servers,
  serverId,
  initServers,
  refreshServers,
  saveSelection,
  handleServerChange: onServerChange,
} = useServerSelector('preprod', STORAGE_KEYS.PREPROD_SERVER, loadData)
const {
  selectedIds,
  allSelected,
  tableRef,
  handleSelectionChange,
  handleSizeChange,
  handleCurrentChange,
  toggleSelectAll,
} = useSelection('name', paginatedResources, { search, currentPage })

// --- WebSocket ---
const userStore = useUserStore()
const wsStore = useWebSocketStore()
const outputCache = new Map()
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const currentAction = ref('')

// 监听 WebSocket 状态变化（支持页面切换后继续执行）
watch(
  () => wsStore.status,
  async (newStatus) => {
    if (newStatus === 'done') {
      executing.value = false
      ElMessage.success('执行成功')
      await loadData()
    } else if (newStatus === 'error') {
      executing.value = false
      ElMessage.error(wsStore.lastError || '执行失败')
    }
  }
)

// 切换服务器时，缓存/恢复执行结果
watch(serverId, (newVal, oldVal) => {
  if (wsStore.status !== 'idle') return
  if (oldVal != null) {
    outputCache.set(oldVal, [...wsStore.outputLines])
  }
  const cached = outputCache.get(newVal)
  if (cached) {
    wsStore.restoreOutput(cached)
  } else {
    wsStore.clearOutput()
  }
})

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

const requireSet = computed(() => new Set(resources.value.filter((r) => r.category === 'require').map((r) => r.name)))

const selectedResources = computed(() => filteredResources.value.filter((r) => selectedIds.value.has(r.name)))

const canBatchScaleDown = computed(() => selectedResources.value.some((r) => r.current > 0))

const canBatchScaleUp = computed(() => selectedResources.value.some((r) => r.current < r.target_replicas))

const canFullScaleDown = computed(() => resources.value.some((r) => r.current > 0))

const canFullScaleUp = computed(() => resources.value.some((r) => r.current < r.target_replicas))

async function handleServerChange() {
  saveSelection()
  await loadData()
}

function handleToggleSelect() {
  toggleSelectAll()
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
    // loadData 已处理错误提示
  }
}

onMounted(async () => {
  await initServers()
})

onActivated(async () => {
  // 重置上次操作的终端状态，防止 watch 立即触发过期的成功/失败提示
  if (wsStore.status === 'done' || wsStore.status === 'error') {
    wsStore.reset()
  }
  await refreshServers()
  if (serverId.value) {
    loadData()
  } else {
    // 服务器全部禁用时清空旧数据，避免显示过期信息
    resources.value = []
    bindings.value = []
    vsTagOptions.value = []
    rsTagOptions.value = []
  }
})

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem(STORAGE_KEYS.PREPROD_SERVER, serverId.value)
  loading.value = true
  try {
    resources.value = await getPreprodStatus(serverId.value)
    selectedIds.value.clear()
    tableRef.value?.clearSelection()
  } catch (e) {
    showLoadError(e, '加载数据失败')
  } finally {
    loading.value = false
  }
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
    vsTagOptions.value = [...new Set((vsTags || []).map((t) => t.tag).filter(Boolean))]
    rsTagOptions.value = [...new Set((rsTags || []).map((t) => t.tag).filter(Boolean))]
    bindingDialogVisible.value = true
  } catch (e) {
    showLoadError(e, '加载绑定配置失败')
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
    bindings.value = bindings.value.filter((b) => b.id !== id)
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
  const targets = pool.filter((r) => r.current > 0)
  const skipCount = pool.filter((r) => r.current === 0).length
  const names = targets.map((r) => r.name)
  if (names.length === 0) return

  // 检查 LVS RS 状态依赖
  try {
    const checkRes = await checkLvsForScaleDown({ preprod_server_id: serverId.value })
    if (checkRes.need_warning) {
      lvsCheckWarnings.value = checkRes.warnings
      lvsCheckVisible.value = true
      lvsCheckConfirmText.value = ''
      const confirmed = await new Promise((resolve) => {
        lvsCheckCallback.value = resolve
      })
      if (!confirmed) return
    }
  } catch (e) {
    ElMessage.error(`LVS 安全检查失败: ${e.response?.data?.error || e.message || '未知错误'}，操作中止`)
    return
  }

  // 全量操作时脚本自动处理依赖，跳过警告
  if (!isFull) {
    const requireTargets = names.filter((n) => requireSet.value.has(n))
    if (requireTargets.length > 0) {
      const nonRequireStillRunning = resources.value
        .filter((r) => !requireSet.value.has(r.name) && r.current > 0)
        .map((r) => r.name)
      const stillRunning = nonRequireStillRunning.filter((n) => !names.includes(n))
      if (stillRunning.length > 0) {
        showDepWarning('依赖(require)服务停止可能会影响其它服务运行！', stillRunning, () =>
          doBatchScaleDown(names, skipCount, pool.length, isFull)
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
  const targets = pool.filter((r) => r.current < r.target_replicas)
  const skipCount = pool.filter((r) => r.current >= r.target_replicas).length
  const names = targets.map((r) => r.name)
  if (names.length === 0) return

  if (!isFull) {
    const nonRequireTargets = names.filter((n) => !requireSet.value.has(n))
    if (nonRequireTargets.length > 0) {
      const requireNotRunning = resources.value
        .filter((r) => requireSet.value.has(r.name) && r.current === 0)
        .map((r) => r.name)
      const stillMissing = requireNotRunning.filter((n) => !names.includes(n))
      if (stillMissing.length > 0) {
        showDepWarning('依赖(require)服务未运行，运行所选服务可能会发生异常！', stillMissing, () =>
          doBatchScaleUp(names, skipCount, pool.length, isFull)
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
  wsStore.connect(url, previewId.value, {
    token: userStore.token,
  })
}
</script>

<style scoped>
.text-warning {
  color: var(--color-warning);
  font-weight: var(--weight-bold);
}
.output-section {
  margin-top: 16px;
  border: 1px solid var(--border-default, rgba(255, 255, 255, 0.06));
  border-radius: 8px;
  padding: 16px;
}
.chart-title {
  font-weight: 600;
  font-size: 14px;
}
</style>
