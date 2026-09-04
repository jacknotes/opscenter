<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { k8sApi, extractErrorMessage } from '@/api'
import type { K8sExecuteResult, K8sPreview, Rollout } from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'

const t = i18n.global.t

// ---------- Rollout 列表 ----------
const serverId = ref<number>()
const rollouts = ref<Rollout[]>([])
const listLoading = ref(false)
const keyword = ref('')
const selected = ref<Rollout[]>([])

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

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return rollouts.value
  return rollouts.value.filter(
    (r) => r.name.toLowerCase().includes(kw) || r.namespace.toLowerCase().includes(kw),
  )
})

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
      <el-empty v-else-if="filtered.length === 0 && !listLoading" :description="t('common.empty')" />
      <el-table
        v-else
        :data="filtered"
        row-key="name"
        @selection-change="(rows: Rollout[]) => (selected = rows)"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column prop="namespace" :label="t('k8s.namespace')" min-width="120" />
        <el-table-column prop="name" :label="t('k8s.name')" min-width="180">
          <template #default="{ row }">
            <span class="mono proj-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="strategy" :label="t('k8s.strategy')" width="100" />
        <el-table-column :label="t('k8s.status')" min-width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)" effect="plain">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="step" :label="t('k8s.step')" width="80" />
        <el-table-column prop="set_weight" :label="t('k8s.setWeight')" width="90" />
        <el-table-column prop="ready" :label="t('k8s.ready')" width="90" />
        <el-table-column prop="desired" :label="t('k8s.desired')" width="80" sortable />
        <el-table-column prop="up_to_date" :label="t('k8s.upToDate')" width="90" sortable />
        <el-table-column prop="available" :label="t('k8s.available')" width="90" sortable />
      </el-table>
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
</style>
