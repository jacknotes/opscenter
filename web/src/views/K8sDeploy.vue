<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { k8sApi, extractErrorMessage } from '@/api'
import type { K8sExecuteResult, K8sPreview, Rollout } from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { useOutputCache } from '@/composables/useOutputCache'
import { useSelection } from '@/composables/useSelection'
import { PAGE_SIZES, STORAGE_KEYS } from '@/utils/constants'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'

const t = i18n.global.t

// ---------- Rollout 列表 ----------
const serverId = ref<number>()
// 服务器选择记忆（v1 useServerSelector 行为）
const savedK8sServer = localStorage.getItem(STORAGE_KEYS.K8S_SERVER)
if (savedK8sServer) serverId.value = Number(savedK8sServer)
watch(serverId, (v) => {
  if (v) localStorage.setItem(STORAGE_KEYS.K8S_SERVER, String(v))
})
const rollouts = ref<Rollout[]>([])
const listLoading = ref(false)
const keyword = ref('')
// 状态筛选：全部/待发布（step 1/x）/已上线（对齐 v1）
const statusFilter = ref<'all' | 'pending' | 'online'>('all')

async function loadList(): Promise<void> {
  if (!serverId.value) {
    rollouts.value = []
    return
  }
  listLoading.value = true
  try {
    rollouts.value = await k8sApi.rollouts(serverId.value)
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    listLoading.value = false
  }
}

watch(serverId, loadList)

// 已有记忆的服务器时直接加载
onMounted(() => {
  if (serverId.value) void loadList()
})

// 仅展示 Paused 状态的 Rollout（对齐 v1：该页面用于发布 Paused 的灰度项目）
const filtered = computed(() => {
  let list = rollouts.value.filter((r) => r.status === 'Paused')
  if (statusFilter.value === 'pending') list = list.filter((r) => r.step.startsWith('1/'))
  else if (statusFilter.value === 'online') list = list.filter((r) => !r.step.startsWith('1/'))
  const kw = keyword.value.trim().toLowerCase()
  if (kw) {
    list = list.filter(
      (r) =>
        r.namespace.toLowerCase().includes(kw) ||
        r.name.toLowerCase().includes(kw) ||
        r.strategy.toLowerCase().includes(kw) ||
        r.step.toLowerCase().includes(kw),
    )
  }
  return list
})

// ---------- 分页 + 跨页勾选 ----------
const currentPage = ref(1)
const pageSize = ref(20)
const total = computed(() => filtered.value.length)
const paged = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filtered.value.slice(start, start + pageSize.value)
})

const { selectedIds, tableRef, handleSelectionChange, handleSizeChange, handleCurrentChange } = useSelection<Rollout>(
  'name',
  paged,
  { search: keyword, currentPage },
)

const selected = computed(() => filtered.value.filter((r) => selectedIds.value.has(r.name)))

// step/ready 数值化排序（'2/3' 按分子排序）
function parseNum(s: string | number): number {
  const m = String(s ?? '').match(/^(\d+)/)
  return m ? parseInt(m[1], 10) : 0
}

const sortByNum =
  (field: 'step' | 'ready') =>
  (a: Rollout, b: Rollout): number =>
    parseNum(a[field]) - parseNum(b[field])

const statusType = (status: string): 'success' | 'warning' | 'danger' | 'info' => {
  const s = status.toLowerCase()
  if (s === 'healthy' || s === 'available') return 'success'
  if (s === 'paused' || s === 'progressing') return 'warning'
  if (s === 'degraded' || s === 'progressdeadlineexceeded') return 'danger'
  return 'info'
}

// ---------- 预览 → 执行 ----------
const previewVisible = ref(false)
const outputVisible = ref(false)
const outputResult = ref<string | string[]>('')
const outputStatus = ref<'success' | 'failed'>('success')

// 切换服务器时缓存/恢复上一次执行输出
useOutputCache([() => serverId.value ?? ''], outputResult, {
  getExtra: () => outputStatus.value,
  setExtra: (extra) => {
    outputStatus.value = extra ?? 'success'
  },
})

const pe = usePreviewExecute<K8sPreview>()

type BatchAction = 'online' | 'sync' | 'rollback'
type FullAction = 'full_online' | 'full_sync' | 'full_rollback'
type K8sAction = BatchAction | FullAction

const pendingAction = ref<K8sAction>('online')

const ACTION_LABEL_KEY: Record<K8sAction, string> = {
  online: 'k8s.online',
  sync: 'k8s.sync',
  rollback: 'k8s.rollback',
  full_online: 'k8s.fullOnline',
  full_sync: 'k8s.fullSync',
  full_rollback: 'k8s.fullRollback',
}

async function batchPreview(action: BatchAction): Promise<void> {
  if (!serverId.value) return
  if (selected.value.length === 0) {
    ElMessage.warning(t('k8s.selectedProjects', { count: 0 }))
    return
  }
  pendingAction.value = action
  const sid = serverId.value
  const projects = selected.value.map((r) => ({ name: r.name, namespace: r.namespace }))
  const ok = await pe.preview(() => k8sApi.batchPreview(action, { server_id: sid, projects }))
  if (ok) previewVisible.value = true
}

async function fullPreview(action: FullAction): Promise<void> {
  if (!serverId.value) return
  pendingAction.value = action
  const sid = serverId.value
  // 全量操作先二次确认
  try {
    await ElMessageBox.confirm(t('k8s.fullConfirm', { action: t(ACTION_LABEL_KEY[action]) }), t('common.confirm'), {
      type: 'warning',
    })
  } catch {
    return
  }
  const ok = await pe.preview(() => k8sApi.fullPreview(action, { server_id: sid }))
  if (ok) previewVisible.value = true
}

async function executePreview(): Promise<void> {
  const action = pendingAction.value
  const result = await pe.execute((id) =>
    (action.startsWith('full_')
      ? k8sApi.fullExecute(action as FullAction, { preview_id: id })
      : k8sApi.batchExecute(action as BatchAction, { preview_id: id })) as Promise<K8sExecuteResult>,
  )
  if (result.ok && result.result) {
    previewVisible.value = false
    outputResult.value = result.result.output
    outputStatus.value = 'success'
    outputVisible.value = true
    ElMessage.success(t('common.execSuccess'))
    pe.reset()
    void loadList()
  } else if (result.error) {
    if (pe.expired.value) {
      ElMessage.warning(result.error)
    } else {
      // K8s execute 失败时 output 是数组，展示已产生的输出
      outputResult.value = (result.result as K8sExecuteResult | undefined)?.output ?? result.error
      outputStatus.value = 'failed'
      outputVisible.value = true
    }
  }
}

function reproview(): void {
  ElMessage.warning(t('preview.expired'))
  previewVisible.value = false
  pe.reset()
  const action = pendingAction.value
  if (action.startsWith('full_')) void fullPreview(action as FullAction)
  else void batchPreview(action as BatchAction)
}

// ---------- page-refresh 全局刷新（AppLayout 快捷键 r 触发） ----------
async function handlePageRefresh(): Promise<void> {
  if (!serverId.value) return
  try {
    await loadList()
    ElMessage.success('刷新成功')
  } catch {
    // loadList 已提示错误
  }
}

onMounted(() => window.addEventListener('page-refresh', handlePageRefresh))
onUnmounted(() => window.removeEventListener('page-refresh', handlePageRefresh))
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('k8s.title') }}</h1>
        <p class="page-subtitle">{{ t('k8s.subtitle') }}</p>
      </div>
      <div class="page-actions head-actions">
        <ServerSelect v-model:server-id="serverId" type="kubernetes" />
        <el-select v-model="statusFilter" style="width: 110px">
          <el-option value="all" label="全部" />
          <el-option value="pending" label="待发布" />
          <el-option value="online" label="已上线" />
        </el-select>
        <el-input v-model="keyword" :placeholder="t('logs.keyword')" clearable style="width: 180px" />
        <el-button :disabled="!serverId" :loading="listLoading" @click="loadList">
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <div v-if="serverId" class="page-actions" style="margin-bottom: var(--space-4)">
      <span class="mono selected-info">{{ t('k8s.selectedProjects', { count: selected.length }) }}</span>
      <el-divider direction="vertical" />
      <el-button type="success" size="small" :disabled="selected.length === 0" @click="batchPreview('online')">
        {{ t('k8s.online') }}
      </el-button>
      <el-button type="primary" size="small" :disabled="selected.length === 0" @click="batchPreview('sync')">
        {{ t('k8s.sync') }}
      </el-button>
      <el-button type="warning" size="small" :disabled="selected.length === 0" @click="batchPreview('rollback')">
        {{ t('k8s.rollback') }}
      </el-button>
      <el-divider direction="vertical" />
      <el-button size="small" @click="fullPreview('full_online')">{{ t('k8s.fullOnline') }}</el-button>
      <el-button size="small" @click="fullPreview('full_sync')">{{ t('k8s.fullSync') }}</el-button>
      <el-button size="small" type="danger" plain @click="fullPreview('full_rollback')">
        {{ t('k8s.fullRollback') }}
      </el-button>
    </div>

    <div v-loading="listLoading" class="card table-card reveal d-1">
      <el-empty v-if="!serverId" :description="t('common.serverPlaceholder')" />
      <el-empty
        v-else-if="filtered.length === 0 && !listLoading"
        :description="keyword || statusFilter !== 'all' ? t('common.empty') : '暂无 Paused 状态的 Rollout'"
      />
      <template v-else>
        <el-table
          ref="tableRef"
          :data="paged"
          row-key="name"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="42" />
          <el-table-column prop="namespace" :label="t('k8s.namespace')" min-width="120" sortable />
          <el-table-column prop="name" :label="t('k8s.name')" min-width="180" sortable>
            <template #default="{ row }">
              <span class="mono proj-name">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="strategy" :label="t('k8s.strategy')" width="100" sortable />
          <el-table-column :label="t('k8s.status')" min-width="140">
            <template #default="{ row }">
              <el-tag size="small" :type="statusType(row.status)" effect="plain">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="step" :label="t('k8s.step')" width="80" sortable :sort-method="sortByNum('step')" />
          <el-table-column prop="set_weight" :label="t('k8s.setWeight')" width="90" />
          <el-table-column prop="ready" :label="t('k8s.ready')" width="90" sortable :sort-method="sortByNum('ready')" />
          <el-table-column prop="desired" :label="t('k8s.desired')" width="80" sortable />
          <el-table-column prop="up_to_date" :label="t('k8s.upToDate')" width="90" sortable />
          <el-table-column prop="available" :label="t('k8s.available')" width="90" sortable />
        </el-table>
        <div class="pagination-bar">
          <el-pagination
            layout="total, sizes, prev, pager, next, jumper"
            :total="total"
            :current-page="currentPage"
            :page-size="pageSize"
            :page-sizes="[...PAGE_SIZES]"
            @current-change="(p: number) => { currentPage = p; handleCurrentChange() }"
            @size-change="(s: number) => { pageSize = s; handleSizeChange(s) }"
          />
        </div>
      </template>
    </div>

    <!-- 预览执行 -->
    <PreviewDialog
      v-model:visible="previewVisible"
      :description="pe.previewData.value?.description ?? ''"
      :current-status="pe.previewData.value?.current_status ?? ''"
      :commands="pe.previewData.value?.commands ?? []"
      :countdown="pe.countdown.value"
      :expired="pe.expired.value"
      :executing="pe.executing.value"
      @execute="executePreview"
      @repreview="reproview"
    />

    <OutputDialog v-model:visible="outputVisible" :output="outputResult" :status="outputStatus" />
  </div>
</template>

<style scoped>
.head-actions {
  flex-wrap: wrap;
}

.table-card {
  padding: var(--space-3);
  min-height: 200px;
}

.proj-name {
  font-weight: 600;
}

.selected-info {
  color: var(--text-secondary);
  font-size: var(--text-sm);
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  padding-top: var(--space-3);
}
</style>
