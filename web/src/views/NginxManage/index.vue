<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { nginxApi, extractErrorMessage } from '@/api'
import type {
  NginxExecuteResult,
  NginxPreview,
  NginxUpstream,
  NginxBatchItem,
  NginxBatchPayload,
  NginxRollbackPayload,
  NginxTogglePayload,
  NginxUpstreamPayload,
  NginxSwapPayload,
} from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'
import OperateDialog from './OperateDialog.vue'
import BatchDialog from './BatchDialog.vue'
import RollbackDialog from './RollbackDialog.vue'
import ConfigViewer from './ConfigViewer.vue'

const t = i18n.global.t

// ---------- 服务器 / 配置文件 ----------
const serverId = ref<number>()
const configFiles = ref<string[]>([])
const configFile = ref('')
const configsLoading = ref(false)

watch(serverId, async (id) => {
  configFiles.value = []
  configFile.value = ''
  upstreams.value = []
  if (!id) return
  configsLoading.value = true
  try {
    configFiles.value = await nginxApi.configs(id)
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    configsLoading.value = false
  }
})

watch(configFile, () => {
  void loadUpstreams()
})

// ---------- Upstream 列表 ----------
const upstreams = ref<NginxUpstream[]>([])
const rawConfig = ref('')
const listLoading = ref(false)
const keyword = ref('')

async function loadUpstreams(): Promise<void> {
  if (!serverId.value || !configFile.value) {
    upstreams.value = []
    return
  }
  listLoading.value = true
  try {
    const res = await nginxApi.upstreams(serverId.value, configFile.value)
    upstreams.value = res.upstreams ?? []
    rawConfig.value = res.raw ?? ''
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    listLoading.value = false
  }
}

const filteredUpstreams = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return upstreams.value
  return upstreams.value.filter(
    (u) => u.name.toLowerCase().includes(kw) || u.servers.some((s) => s.ip.includes(kw)),
  )
})

const serverSummary = (u: NginxUpstream): string => {
  const up = u.servers.filter((s) => s.status === 'up').length
  return `${up}/${u.servers.length}`
}

const upCount = (u: NginxUpstream): number => u.servers.filter((s) => s.status === 'up').length
const downCount = (u: NginxUpstream): number => u.servers.length - upCount(u)

// ---------- 预览 → 执行 ----------
const previewVisible = ref(false)
const outputVisible = ref(false)
const outputResult = ref<string | string[]>('')
const outputStatus = ref<'success' | 'failed'>('success')

const pe = usePreviewExecute<NginxPreview>()

type NgAction = 'online' | 'offline' | 'swap' | 'toggle' | 'batch' | 'rollback'
const pendingAction = ref<NgAction>('online')

function buildPayload(
  action: NgAction,
  payload: NginxUpstreamPayload | NginxSwapPayload | NginxBatchItem[] | string,
): Record<string, unknown> {
  const base = { server_id: serverId.value, config_file: configFile.value }
  switch (action) {
    case 'online':
    case 'offline':
      return { ...base, ...(payload as NginxUpstreamPayload) }
    case 'swap':
      return { ...base, ...(payload as NginxSwapPayload) }
    case 'toggle':
      return { ...base, upstream_names: (payload as NginxUpstreamPayload).upstream_names }
    case 'batch':
      return { ...base, items: payload }
    case 'rollback':
      return { ...base, backup_file: payload }
  }
}

async function previewAction(
  action: NgAction,
  payload: NginxUpstreamPayload | NginxSwapPayload | NginxBatchItem[] | string,
): Promise<void> {
  if (!serverId.value || !configFile.value) return
  pendingAction.value = action
  const body = buildPayload(action, payload)
  const fn = (): Promise<NginxPreview> => {
    switch (action) {
      case 'online':
        return nginxApi.onlinePreview(body as unknown as NginxUpstreamPayload)
      case 'offline':
        return nginxApi.offlinePreview(body as unknown as NginxUpstreamPayload)
      case 'swap':
        return nginxApi.swapPreview(body as unknown as NginxSwapPayload)
      case 'toggle':
        return nginxApi.togglePreview(body as unknown as NginxTogglePayload)
      case 'batch':
        return nginxApi.batchPreview(body as unknown as NginxBatchPayload)
      case 'rollback':
        return nginxApi.rollbackPreview(body as unknown as NginxRollbackPayload)
    }
  }
  const ok = await pe.preview(fn)
  if (ok) previewVisible.value = true
}

async function executePreview(): Promise<void> {
  const action = pendingAction.value
  const result = await pe.execute((id) => nginxApi.execute(action, { preview_id: id }))
  if (result.ok && result.result) {
    previewVisible.value = false
    const res = result.result as NginxExecuteResult
    outputResult.value = res.output ?? res.message
    outputStatus.value = 'success'
    outputVisible.value = true
    ElMessage.success(t('common.execSuccess'))
    pe.reset()
    void loadUpstreams()
  } else if (result.error) {
    if (pe.expired.value) {
      ElMessage.warning(result.error)
    } else {
      // 契约：nginx -t 失败会自动回滚并返回 400，错误里带说明
      outputResult.value = result.error
      outputStatus.value = 'failed'
      outputVisible.value = true
    }
  }
}

function reproview(): void {
  ElMessage.warning(t('preview.expired'))
  previewVisible.value = false
  pe.reset()
}

// ---------- 操作对话框 ----------
const operateDialogRef = ref<InstanceType<typeof OperateDialog>>()
const batchDialogRef = ref<InstanceType<typeof BatchDialog>>()
const rollbackDialogRef = ref<InstanceType<typeof RollbackDialog>>()
const configVisible = ref(false)
const operateAction = ref<'online' | 'offline' | 'swap' | 'toggle'>('online')
const operateUpstream = ref('')

function openOperate(
  action: 'online' | 'offline' | 'swap' | 'toggle',
  upstream?: NginxUpstream,
): void {
  if (!serverId.value || !configFile.value) return
  operateAction.value = action
  operateUpstream.value = upstream?.name ?? ''
  operateDialogRef.value?.open()
}

function onOperateConfirm(payload: NginxUpstreamPayload | NginxSwapPayload): void {
  void previewAction(operateAction.value, payload)
}

function onBatchConfirm(items: NginxBatchItem[]): void {
  void previewAction('batch', items)
}

function onRollbackConfirm(backupFile: string): void {
  void previewAction('rollback', backupFile)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('nginx.title') }}</h1>
        <p class="page-subtitle">{{ t('nginx.subtitle') }}</p>
      </div>
      <div class="page-actions head-actions">
        <ServerSelect v-model:server-id="serverId" type="nginx" />
        <el-select
          v-model="configFile"
          :placeholder="t('nginx.configFile')"
          :loading="configsLoading"
          filterable
          :disabled="!serverId"
          style="width: 220px"
        >
          <el-option v-for="f in configFiles" :key="f" :value="f" :label="f" />
        </el-select>
        <el-button :disabled="!serverId || !configFile" :loading="listLoading" @click="loadUpstreams">
          {{ t('common.refresh') }}
        </el-button>
        <el-button :disabled="!serverId || !configFile" @click="rollbackDialogRef?.open()">
          {{ t('nginx.rollback') }}
        </el-button>
        <el-button :disabled="!serverId || !configFile" @click="configVisible = true">
          {{ t('nginx.config') }}
        </el-button>
      </div>
    </div>

    <template v-if="serverId && configFile">
      <div class="page-actions" style="margin-bottom: var(--space-4)">
        <el-input v-model="keyword" :placeholder="t('logs.keyword')" clearable style="width: 200px" />
        <el-button type="primary" :disabled="upstreams.length === 0" @click="batchDialogRef?.open()">
          {{ t('nginx.batch') }}
        </el-button>
      </div>

      <div v-loading="listLoading" class="card table-card reveal d-1">
        <el-empty v-if="filteredUpstreams.length === 0 && !listLoading" :description="t('common.empty')" />
        <el-table v-else :data="filteredUpstreams" row-key="name">
          <el-table-column type="expand">
            <template #default="{ row }">
              <div class="srv-wrap">
                <el-table :data="row.servers" size="small">
                  <el-table-column :label="t('nginx.backends')" min-width="180">
                    <template #default="{ row: s }">
                      <span class="mono">{{ s.ip }}:{{ s.port }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('logs.status')" width="100">
                    <template #default="{ row: s }">
                      <span class="dot-live" :class="{ 'is-offline': s.status !== 'up' }" />
                      <span class="status-text" :class="s.status === 'up' ? 'ok' : 'bad'">
                        {{ s.status === 'up' ? t('common.online') : t('common.offline') }}
                      </span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="weight" :label="t('lvs.weight')" width="100">
                    <template #default="{ row: s }">{{ s.weight || '-' }}</template>
                  </el-table-column>
                </el-table>
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="t('nginx.upstream')" min-width="200">
            <template #default="{ row }">
              <span class="mono upstream-name">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.online')" width="120">
            <template #default="{ row }">
              <span class="mono ok">{{ upCount(row as NginxUpstream) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.offline')" width="120">
            <template #default="{ row }">
              <span class="mono bad">{{ downCount(row as NginxUpstream) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.operation')" min-width="260" fixed="right">
            <template #default="{ row }">
              <el-button link type="success" size="small" @click="openOperate('online', row as NginxUpstream)">
                {{ t('nginx.online') }}
              </el-button>
              <el-button link type="danger" size="small" @click="openOperate('offline', row as NginxUpstream)">
                {{ t('nginx.offline') }}
              </el-button>
              <el-button link size="small" @click="openOperate('swap', row as NginxUpstream)">{{ t('nginx.swap') }}</el-button>
              <el-button link type="warning" size="small" @click="openOperate('toggle', row as NginxUpstream)">
                {{ t('nginx.toggle') }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </template>
    <el-empty v-else :description="t('common.serverPlaceholder')" />

    <!-- 预览执行 -->
    <PreviewDialog
      v-model:visible="previewVisible"
      :description="pe.previewData.value?.description ?? ''"
      :before="pe.previewData.value?.before ?? ''"
      :after="pe.previewData.value?.after ?? ''"
      :line-diffs="pe.previewData.value?.line_diffs ?? []"
      :countdown="pe.countdown.value"
      :expired="pe.expired.value"
      :executing="pe.executing.value"
      @execute="executePreview"
      @repreview="reproview"
    />

    <OutputDialog v-model:visible="outputVisible" :output="outputResult" :status="outputStatus" />

    <!-- 操作 / 批量 / 回滚 -->
    <OperateDialog
      ref="operateDialogRef"
      :action="operateAction"
      :upstreams="upstreams"
      :initial-upstream="operateUpstream"
      @confirm="onOperateConfirm"
    />
    <BatchDialog ref="batchDialogRef" :upstreams="upstreams" @confirm="onBatchConfirm" />
    <RollbackDialog
      ref="rollbackDialogRef"
      :server-id="serverId ?? 0"
      :config-file="configFile"
      @confirm="onRollbackConfirm"
    />
    <ConfigViewer v-model:visible="configVisible" :raw-config="rawConfig" :config-file="configFile" />
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

.srv-wrap {
  padding: var(--space-3) var(--space-6);
  background: var(--bg-deep);
}

.upstream-name {
  font-weight: 600;
}

.status-text {
  margin-left: var(--space-2);
  font-size: var(--text-xs);
}

.status-text.ok {
  color: var(--emerald-400);
}

.status-text.bad {
  color: var(--rose-400);
}

.ok {
  color: var(--emerald-400);
}

.bad {
  color: var(--rose-400);
}
</style>
