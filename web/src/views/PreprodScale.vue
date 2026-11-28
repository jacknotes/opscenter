<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px">
          <span class="filter-label">服务器:</span>
          <el-select
            v-model="serverId"
            v-select-fit
            placeholder="选择预生产服务器"
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
            <el-tag
              :type="resolveStatus(row).tagType"
              size="small"
              style="margin-left: 4px"
              :title="resolveStatus(row).title"
              >{{ resolveStatus(row).label }}</el-tag
            >
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

    <!-- 变更预览：风险提示 + 资源清单 + 命令 + 当前状态，是操作前的唯一出口。
         原先「LVS 缩容前检查」「依赖 require 警告」「批量缩容确认」三个弹框都合并到这里，
         风险和清单一次性看全，避免同一次操作连点三四个确认。 -->
    <ChangePreviewDialog
      v-model="previewVisible"
      :description="previewData?.description"
      :executing="executing"
      :require-gate="previewGate"
      :gate-reason="previewGateReason"
      :confirm-text="BATCH_CONFIRM_TEXT"
      @confirm="executePreview"
    >
      <div v-if="previewData">
        <!-- 风险摘要：有风险时列出明细，无风险时给一个明确的通过信号 -->
        <div v-if="previewRisks.length" class="cp-risk">
          <span>⚠</span>
          <span>
            <b>操作存在风险</b>
            <span v-for="(r, i) in previewRisks" :key="i" style="display: block; margin-top: 2px">{{ r }}</span>
          </span>
        </div>
        <div v-else class="cp-risk-ok">✓ 依赖检查通过（LVS RS 已下线，require 依赖无冲突）</div>

        <!-- 资源清单：全量模式下展示全部资源，与脚本实际执行范围保持一致 -->
        <template v-if="previewResourceNames.length">
          <p class="cp-label">
            将{{ previewAction === 'scaledown' ? '缩容' : '扩容' }}（{{ previewResourceNames.length }} 项）：
          </p>
          <div class="cp-table-scroll" style="max-height: 170px">
            <table class="cp-table">
              <thead>
                <tr>
                  <th style="width: 44px">#</th>
                  <th>资源名称</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(name, index) in previewResourceNames" :key="name">
                  <td>{{ index + 1 }}</td>
                  <td>{{ name }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="previewSkipNames.length" class="cp-label" style="margin-top: 10px">
            共 {{ previewResourceNames.length + previewSkipNames.length }} 项，其中
            <b style="color: var(--color-warning)"
              >{{ previewSkipNames.length }} 项{{ previewAction === 'scaledown' ? '已缩容' : '已扩容' }}将跳过</b
            >
          </p>
        </template>

        <p class="cp-label">命令：</p>
        <p>
          <code class="cp-inline-code">{{ previewData.command }}</code>
        </p>
        <div class="cp-divider"></div>
        <p class="cp-label">当前状态：</p>
        <pre class="cp-pre">{{ previewData.current_status }}</pre>
      </div>
    </ChangePreviewDialog>

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
import { resolveStatus } from './preprodStatus'
import { useServerSelector } from '../composables/useServerSelector'
import { useSelection } from '../composables/useSelection'
import StreamOutput from '../components/StreamOutput.vue'
import ChangePreviewDialog from '../components/common/ChangePreviewDialog.vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { showApiError, isAuthError, showLoadError } from '../utils/message'
import { ZoomOut } from '@element-plus/icons-vue'
import { STORAGE_KEYS, DEFAULT_PAGE_SIZE, BATCH_CONFIRM_THRESHOLD, BATCH_CONFIRM_TEXT } from '../utils/constants'

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

// 变更预览的附加信息：资源清单、跳过项、风险摘要
const previewAction = ref('')
const previewResourceNames = ref([])
const previewSkipNames = ref([])
const previewRisks = ref([])

// 需要手输确认门槛的情形：存在未消除的风险，或涉及数量超过阈值
const previewGate = computed(
  () => previewRisks.value.length > 0 || previewResourceNames.value.length > BATCH_CONFIRM_THRESHOLD
)

const previewGateReason = computed(() => {
  if (!previewGate.value) return ''
  const reasons = []
  if (previewRisks.value.length) reasons.push('本次操作存在风险')
  if (previewResourceNames.value.length > BATCH_CONFIRM_THRESHOLD) {
    reasons.push(`本次涉及 ${previewResourceNames.value.length} 项（超过 ${BATCH_CONFIRM_THRESHOLD} 项阈值）`)
  }
  return reasons.join('，')
})

// Binding config
const bindingDialogVisible = ref(false)
const bindings = ref([])
const addBindingVisible = ref(false)
const newBinding = ref({ vs_tag: '', rs_env_tag: '' })
const vsTagOptions = ref([])
const rsTagOptions = ref([])

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
    showApiError(e, '保存失败')
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
    showApiError(e, '删除失败')
  }
}

async function handleBatchScaleDown() {
  const isFull = selectedIds.value.size === 0
  // 全量 = 服务器上的全部资源，与搜索/状态筛选无关。
  // 之前用 filteredResources 会导致「清单显示筛选后的子集，实际却缩容全部」，是个隐患。
  const pool = isFull ? resources.value : selectedResources.value
  const targets = pool.filter((r) => r.current > 0)
  const skipNames = pool.filter((r) => r.current === 0).map((r) => r.name)
  const names = targets.map((r) => r.name)
  if (names.length === 0) {
    ElMessage.warning('所选范围内没有需要缩容的资源')
    return
  }

  // 风险收集：LVS RS 未下线 + require 依赖仍在运行，统一汇总后在预览里一次性展示
  const risks = []

  try {
    const checkRes = await checkLvsForScaleDown({ preprod_server_id: serverId.value })
    if (checkRes.need_warning && checkRes.warnings?.length) {
      for (const w of checkRes.warnings) {
        risks.push(
          `LVS 风险：${w.lvs_server} 上 VIP ${w.vs_tag} 的 RS ${w.rs_ip}（${w.rs_env_tag}）仍处于 ${w.status} 状态，建议先下线该 RS`
        )
      }
    }
  } catch (e) {
    if (!isAuthError(e)) ElMessage.error(`LVS 安全检查失败: ${e.response?.data?.error || e.message || '未知错误'}，操作中止`)
    return
  }

  if (!isFull) {
    const requireTargets = names.filter((n) => requireSet.value.has(n))
    if (requireTargets.length > 0) {
      const stillRunning = resources.value
        .filter((r) => !requireSet.value.has(r.name) && r.current > 0)
        .map((r) => r.name)
        .filter((n) => !names.includes(n))
      if (stillRunning.length > 0) {
        risks.push(
          `依赖风险：缩容 require 服务（${requireTargets.join('、')}）时，以下服务仍在运行，可能受影响：${stillRunning.join('、')}`
        )
      }
    }
  }

  openScalePreview('scaledown', names, skipNames, risks)
}

async function handleBatchScaleUp() {
  const isFull = selectedIds.value.size === 0
  const pool = isFull ? resources.value : selectedResources.value
  const targets = pool.filter((r) => r.current < r.target_replicas)
  const skipNames = pool.filter((r) => r.current >= r.target_replicas).map((r) => r.name)
  const names = targets.map((r) => r.name)
  if (names.length === 0) {
    ElMessage.warning('所选范围内没有需要扩容的资源')
    return
  }

  const risks = []
  if (!isFull) {
    const nonRequireTargets = names.filter((n) => !requireSet.value.has(n))
    if (nonRequireTargets.length > 0) {
      const stillMissing = resources.value
        .filter((r) => requireSet.value.has(r.name) && r.current === 0)
        .map((r) => r.name)
        .filter((n) => !names.includes(n))
      if (stillMissing.length > 0) {
        risks.push(`依赖风险：以下 require 服务未运行，所选服务可能需要它们：${stillMissing.join('、')}`)
      }
    }
  }

  openScalePreview('scaleup', names, skipNames, risks)
}

/**
 * 打开变更预览，并把风险、资源清单、确认门槛一并带上。
 *
 * 资源名单始终显式传给后端（即使是全量），保证「看到的」和「执行的」完全一致。
 */
async function openScalePreview(action, names, skipNames, risks) {
  try {
    const previewFn = action === 'scaledown' ? preprodScaleDownPreview : preprodScaleUpPreview
    const res = await previewFn({ server_id: serverId.value, resource_names: names })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewAction.value = action
    previewResourceNames.value = names
    previewSkipNames.value = skipNames
    previewRisks.value = risks
    previewVisible.value = true
  } catch (e) {
    showApiError(e, '预览失败')
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
