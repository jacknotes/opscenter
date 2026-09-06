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
import { STORAGE_KEYS } from '@/utils/constants'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'
import OperateDialog from './OperateDialog.vue'
import BatchDialog from './BatchDialog.vue'
import RollbackDialog from './RollbackDialog.vue'
import SwapScopeDialog, { type SwapScopeItem } from './SwapScopeDialog.vue'
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
    // 恢复该服务器上次选择的配置文件（对齐 v1 nginx_config_<id> 记忆）
    const saved = localStorage.getItem(STORAGE_KEYS.nginxConfig(id))
    if (saved && configFiles.value.includes(saved)) configFile.value = saved
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    configsLoading.value = false
  }
})

watch(configFile, (f) => {
  if (serverId.value && f) localStorage.setItem(STORAGE_KEYS.nginxConfig(serverId.value), f)
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

const serverSummary = (u: NginxUpstream): string => {
  const up = u.servers.filter((s) => s.status === 'up').length
  return `${up}/${u.servers.length}`
}

const upCount = (u: NginxUpstream): number => u.servers.filter((s) => s.status === 'up').length
const downCount = (u: NginxUpstream): number => u.servers.length - upCount(u)

// 状态筛选：点击"离线"chip 仅显示含离线后端的 upstream（对齐 v1）
const statusFilter = ref<'all' | 'down'>('all')
function toggleStatusFilter(): void {
  statusFilter.value = statusFilter.value === 'down' ? 'all' : 'down'
}

const filteredUpstreams = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  let list = upstreams.value
  if (kw) {
    list = list.filter((u) => u.name.toLowerCase().includes(kw) || u.servers.some((s) => s.ip.includes(kw)))
  }
  if (statusFilter.value === 'down') list = list.filter((u) => downCount(u) > 0)
  return list
})

// 统计 chips（对齐 v1）
const totalUpCount = computed(() => filteredUpstreams.value.reduce((sum, u) => sum + upCount(u), 0))
const totalDownCount = computed(() => filteredUpstreams.value.reduce((sum, u) => sum + downCount(u), 0))

// 健康度行配色：全在线绿 / 混合黄 / 全离线红（对齐 v1 组级左边框）
const healthClass = (u: NginxUpstream): string =>
  downCount(u) === 0 ? 'health-healthy' : upCount(u) === 0 ? 'health-critical' : 'health-degraded'

const healthRowClass = ({ row }: { row: unknown }): string => healthClass(row as NginxUpstream)

// ---------- 预览 → 执行 ----------
const previewVisible = ref(false)
const outputVisible = ref(false)
const outputResult = ref<string | string[]>('')
const outputStatus = ref<'success' | 'failed'>('success')

const pe = usePreviewExecute<NginxPreview>()

type NgAction = 'online' | 'offline' | 'swap' | 'toggle' | 'batch' | 'rollback'
const pendingAction = ref<NgAction>('online')

const ACTION_LABELS = computed<Record<NgAction, string>>(() => ({
  online: t('nginx.online'),
  offline: t('nginx.offline'),
  swap: t('nginx.swap'),
  toggle: t('nginx.toggle'),
  batch: t('nginx.batch'),
  rollback: t('nginx.rollback'),
}))

// 执行结果元信息条（对齐 v1：操作/对象/配置文件/时间）
const outputMeta = ref('')
const pendingSummary = ref('')

function nowStr(): string {
  const d = new Date()
  const p = (n: number): string => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function payloadSummary(action: NgAction, payload: unknown): string {
  if (action === 'rollback') return String(payload)
  if (action === 'batch') return (payload as NginxBatchItem[]).map((i) => i.upstream_name).join('、')
  const p = payload as { upstream_names?: string[] } | null
  return (p?.upstream_names ?? []).join('、')
}

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
  pendingSummary.value = payloadSummary(action, payload)
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
  const metaBase = `${ACTION_LABELS.value[action]}${pendingSummary.value ? ` · ${pendingSummary.value}` : ''} · ${configFile.value}`
  if (result.ok && result.result) {
    previewVisible.value = false
    const res = result.result as NginxExecuteResult
    outputResult.value = res.output ?? res.message
    outputStatus.value = 'success'
    outputMeta.value = `${metaBase} · ${nowStr()}`
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
      outputMeta.value = `${metaBase}（失败）· ${nowStr()}`
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
  // 切换：跨全部 upstream 计算同时包含这两台服务器的组，勾选后一次切换（对齐 v1）
  if (operateAction.value === 'swap') {
    const p = payload as NginxSwapPayload
    const affected = computeAffectedSwap(p)
    if (affected.length > 0) {
      swapScopeIp.value = { offline: p.offline_ip, online: p.online_ip }
      swapScopeList.value = affected
      swapScopePending.value = p
      swapScopeVisible.value = true
      return
    }
  }
  void previewAction(operateAction.value, payload)
}

/** 计算受切换影响的 upstream 组（包含将下线的 up 服务器 + 将上线的 down 服务器） */
function computeAffectedSwap(p: NginxSwapPayload): SwapScopeItem[] {
  const items: SwapScopeItem[] = []
  for (const u of upstreams.value) {
    let hasOffline = false
    let hasOnline = false
    for (const s of u.servers) {
      if (s.ip === p.offline_ip && s.status === 'up') hasOffline = true
      if (s.ip === p.online_ip && s.status === 'down') hasOnline = true
    }
    if (hasOffline && hasOnline) {
      items.push({ name: u.name, total: u.servers.length, up: upCount(u), down: downCount(u), checked: true })
    }
  }
  return items
}

const swapScopeVisible = ref(false)
const swapScopeList = ref<SwapScopeItem[]>([])
const swapScopeIp = ref({ offline: '', online: '' })
const swapScopePending = ref<NginxSwapPayload | null>(null)

function onSwapScopeConfirm(names: string[]): void {
  swapScopeVisible.value = false
  const p = swapScopePending.value
  if (!p || names.length === 0) return
  void previewAction('swap', { ...p, upstream_names: names })
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
        <span class="chip-spacer" />
        <span class="stat-chip">Upstream <b>{{ filteredUpstreams.length }}</b></span>
        <span class="stat-chip stat-chip-success">在线 <b>{{ totalUpCount }}</b></span>
        <span
          class="stat-chip stat-chip-danger"
          :class="{ 'stat-chip-active': statusFilter === 'down' }"
          @click="toggleStatusFilter"
        >
          离线 <b>{{ totalDownCount }}</b>
        </span>
      </div>

      <div v-loading="listLoading" class="card table-card reveal d-1">
        <el-empty v-if="filteredUpstreams.length === 0 && !listLoading" :description="t('common.empty')" />
        <el-table v-else v-force-reflow :data="filteredUpstreams" row-key="name" :row-class-name="healthRowClass">
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

    <OutputDialog v-model:visible="outputVisible" :output="outputResult" :status="outputStatus" :meta="outputMeta" />

    <!-- 切换范围确认：跨 upstream 一次切多组（对齐 v1 SwapDialog） -->
    <SwapScopeDialog
      v-model:visible="swapScopeVisible"
      :offline-ip="swapScopeIp.offline"
      :online-ip="swapScopeIp.online"
      :affected="swapScopeList"
      @confirm="onSwapScopeConfirm"
    />

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

/* 统计 chips（对齐 v1） */
.chip-spacer {
  margin-left: auto;
}

.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: var(--radius-pill);
  font-size: var(--text-xs);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  background: var(--bg-input);
  white-space: nowrap;
}

.stat-chip b {
  color: var(--text-primary);
}

.stat-chip-success {
  border-color: var(--el-color-success, #67c23a);
  color: var(--el-color-success, #67c23a);
}

.stat-chip-success b {
  color: var(--el-color-success, #67c23a);
}

.stat-chip-danger {
  border-color: var(--el-color-danger, #f56c6c);
  color: var(--el-color-danger, #f56c6c);
  cursor: pointer;
  transition: background 0.2s;
}

.stat-chip-danger b {
  color: var(--el-color-danger, #f56c6c);
}

.stat-chip-danger:hover {
  background: rgba(245, 108, 108, 0.12);
}

.stat-chip-danger.stat-chip-active {
  background: rgba(245, 108, 108, 0.2);
}

/* 健康度行左边框（对齐 v1 组级配色）：全绿/混合/全红 */
.table-card :deep(tr.health-healthy td:first-child) {
  box-shadow: inset 3px 0 0 var(--el-color-success, #67c23a);
}

.table-card :deep(tr.health-degraded td:first-child) {
  box-shadow: inset 3px 0 0 var(--el-color-warning, #e6a23c);
}

.table-card :deep(tr.health-critical td:first-child) {
  box-shadow: inset 3px 0 0 var(--el-color-danger, #f56c6c);
}
</style>
