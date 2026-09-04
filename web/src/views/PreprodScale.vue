<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { preprodApi, lvsApi, extractErrorMessage } from '@/api'
import type { PreprodPreview, PreprodResource, LvsScaledownCheck } from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'

const t = i18n.global.t

// ---------- 资源列表 ----------
const serverId = ref<number>()
const resources = ref<PreprodResource[]>([])
const listLoading = ref(false)
const keyword = ref('')
const selected = ref<PreprodResource[]>([])

async function loadList(): Promise<void> {
  if (!serverId.value) {
    resources.value = []
    return
  }
  listLoading.value = true
  try {
    resources.value = await preprodApi.status(serverId.value)
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    listLoading.value = false
  }
}

watch(serverId, loadList)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return resources.value
  return resources.value.filter((r) => r.name.toLowerCase().includes(kw))
})

const categoryType = (c: string): 'primary' | 'warning' | 'info' =>
  c === 'rollout' ? 'primary' : c === 'statefulset' ? 'warning' : 'info'

// ---------- 缩扩容（流式执行） ----------
const previewVisible = ref(false)
const streamPreviewId = ref('')

const pe = usePreviewExecute<PreprodPreview>()

type ScaleAction = 'scaledown' | 'scaleup'
const pendingAction = ref<ScaleAction>('scaledown')

/** 资源名列表：未勾选 = 全量（契约：resource_names 为空时操作所有资源） */
const resourceNames = computed(() => selected.value.map((r) => r.name))

async function scalePreview(action: ScaleAction): Promise<void> {
  if (!serverId.value) return
  pendingAction.value = action

  // 缩容前检查 LVS 绑定的生产 RS（契约：POST /lvs/check/scaledown）
  if (action === 'scaledown') {
    try {
      const check: LvsScaledownCheck = await lvsApi.checkScaledown(serverId.value)
      if (check.need_warning && check.warnings?.length) {
        const w = check.warnings[0]
        const lines = `VS[${w.vs_tag}] ↔ RS[${w.rs_env_tag}] ${w.rs_ip} (${w.status}) @ ${w.lvs_server}`
        await ElMessageBox.confirm(lines, t('preprod.lvsWarning'), { type: 'warning' })
      }
    } catch {
      return // 用户取消
    }
  }

  const sid = serverId.value
  const names = resourceNames.value
  const ok = await pe.preview(() =>
    action === 'scaledown'
      ? preprodApi.scaledownPreview({ server_id: sid, resource_names: names })
      : preprodApi.scaleupPreview({ server_id: sid, resource_names: names }),
  )
  if (ok) {
    streamPreviewId.value = ''
    previewVisible.value = true
  }
}

/** 确认后启动 WebSocket 流式执行 */
function startStream(): void {
  const id = pe.previewData.value?.preview_id
  if (!id) return
  streamPreviewId.value = id
}

function onStreamDone(): void {
  ElMessage.success(t('common.execSuccess'))
  pe.reset()
  streamPreviewId.value = ''
  previewVisible.value = false
  void loadList()
}

function onStreamFailed(message: string): void {
  if (message) ElMessage.error(message)
  // 契约：执行失败时预览已删除（WS 语义），需要重新预览
  pe.reset()
  streamPreviewId.value = ''
}

function reproview(): void {
  previewVisible.value = false
  pe.reset()
  streamPreviewId.value = ''
  void scalePreview(pendingAction.value)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('preprod.title') }}</h1>
        <p class="page-subtitle">{{ t('preprod.subtitle') }}</p>
      </div>
      <div class="page-actions head-actions">
        <ServerSelect v-model:server-id="serverId" type="preprod" />
        <el-input v-model="keyword" :placeholder="t('logs.keyword')" clearable style="width: 180px" />
        <el-button :disabled="!serverId" :loading="listLoading" @click="loadList">
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <div v-if="serverId" class="page-actions" style="margin-bottom: var(--space-4)">
      <span class="mono selected-info">
        {{ selected.length > 0 ? t('k8s.selectedProjects', { count: selected.length }) : t('preprod.resourceNamesHint') }}
      </span>
      <el-divider direction="vertical" />
      <el-button type="danger" size="small" @click="scalePreview('scaledown')">
        {{ t('preprod.scaledown') }}
      </el-button>
      <el-button type="success" size="small" @click="scalePreview('scaleup')">
        {{ t('preprod.scaleup') }}
      </el-button>
    </div>

    <div v-loading="listLoading" class="card table-card reveal d-1">
      <el-empty v-if="!serverId" :description="t('common.serverPlaceholder')" />
      <el-empty v-else-if="filtered.length === 0 && !listLoading" :description="t('common.empty')" />
      <el-table
        v-else
        :data="filtered"
        row-key="name"
        @selection-change="(rows: PreprodResource[]) => (selected = rows)"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column :label="t('preprod.category')" width="130">
          <template #default="{ row }">
            <el-tag size="small" :type="categoryType(row.category)" effect="plain">{{ row.category }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('preprod.resource')" min-width="200">
          <template #default="{ row }">
            <span class="mono res-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="desired" :label="t('preprod.desired')" width="100" sortable />
        <el-table-column prop="current" :label="t('preprod.current')" width="100" sortable />
        <el-table-column prop="up_to_date" :label="t('preprod.upToDate')" width="100" />
        <el-table-column prop="available" :label="t('preprod.available')" width="100" />
        <el-table-column prop="age" :label="t('preprod.age')" width="90" />
        <el-table-column :label="t('preprod.targetReplicas')" width="120">
          <template #default="{ row }">
            <span class="mono">{{ row.target_replicas || '-' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 预览 + WebSocket 流式执行 -->
    <PreviewDialog
      v-model:visible="previewVisible"
      streaming
      :stream-preview-id="streamPreviewId"
      :description="pe.previewData.value?.description ?? ''"
      :current-status="pe.previewData.value?.current_status ?? ''"
      :commands="pe.previewData.value ? [pe.previewData.value.command] : []"
      :countdown="pe.countdown.value"
      :expired="pe.expired.value"
      :executing="pe.executing.value"
      @execute="startStream"
      @repreview="reproview"
      @stream-done="onStreamDone"
      @stream-failed="onStreamFailed"
    />
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

.res-name {
  font-weight: 600;
}

.selected-info {
  color: var(--text-secondary);
  font-size: var(--text-sm);
}
</style>
