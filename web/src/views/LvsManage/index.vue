<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { lvsApi, preprodApi, extractErrorMessage } from '@/api'
import type { CommandExecuteResult, LvsPreview, VirtualServer, LvsRealServer } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { AUTO_REFRESH_INTERVAL_MS, STORAGE_KEYS } from '@/utils/constants'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'
import StatusDialog from './StatusDialog.vue'
import SwapDialog from './SwapDialog.vue'
import TagDialog from './TagDialog.vue'
import BindingsDialog from './BindingsDialog.vue'

const t = i18n.global.t
const auth = useAuthStore()

// ---------- 数据加载 ----------
const serverSelectRef = ref<InstanceType<typeof ServerSelect>>()
const serverId = ref<number>()
const vsList = ref<VirtualServer[]>([])
const listLoading = ref(false)
const keyword = ref('')

// 服务器选择记忆（v1 useServerSelector 行为）
const savedServer = localStorage.getItem(STORAGE_KEYS.LVS_SERVER)
if (savedServer) serverId.value = Number(savedServer)
watch(serverId, (v) => {
  if (v) localStorage.setItem(STORAGE_KEYS.LVS_SERVER, String(v))
})

async function loadList(): Promise<void> {
  if (!serverId.value) {
    vsList.value = []
    return
  }
  listLoading.value = true
  try {
    vsList.value = await lvsApi.list(serverId.value)
    // 过滤条件失效时清空
    if (vsFilter.value && !vsList.value.some((vs) => vs.ip === vsFilter.value)) vsFilter.value = ''
    pruneBatchSelection()
    // 刷新后按当前折叠状态恢复展开行
    expandedKeys.value = allExpanded.value ? vsList.value.map((vs) => vs.ip) : []
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    listLoading.value = false
  }
}

watch(serverId, loadList)

// ---------- VS 筛选（localStorage 记忆） ----------
const vsFilter = ref(localStorage.getItem(STORAGE_KEYS.LVS_VS_FILTER) || '')
watch(vsFilter, (val) => {
  if (val) localStorage.setItem(STORAGE_KEYS.LVS_VS_FILTER, val)
  else localStorage.removeItem(STORAGE_KEYS.LVS_VS_FILTER)
})

const vsOptions = computed(() => vsList.value.map((vs) => vs.ip))

const filteredVs = computed(() => {
  let list = vsList.value
  if (vsFilter.value) list = list.filter((vs) => vs.ip === vsFilter.value)
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return list
  return list.filter(
    (vs) =>
      vs.ip.includes(kw) ||
      (vs.tag ?? '').toLowerCase().includes(kw) ||
      vs.real_servers.some((rs) => rs.ip.includes(kw) || (rs.tag ?? '').toLowerCase().includes(kw)),
  )
})

const rsCount = (vs: VirtualServer): string =>
  `${vs.real_servers.filter((r) => r.status === 'up').length}/${vs.real_servers.length}`

// ---------- 一键展开 / 折叠全部 VS 分组 ----------
const expandedKeys = ref<string[]>([])
const allExpanded = ref(true)

function toggleExpandAll(): void {
  allExpanded.value = !allExpanded.value
  expandedKeys.value = allExpanded.value ? filteredVs.value.map((vs) => vs.ip) : []
}

function onExpandChange(_row: VirtualServer, expanded: VirtualServer[] | boolean): void {
  if (!Array.isArray(expanded)) return
  expandedKeys.value = expanded.map((r) => r.ip)
  allExpanded.value =
    filteredVs.value.length > 0 && filteredVs.value.every((vs) => expandedKeys.value.includes(vs.ip))
}

// ---------- 批量选择（跨分组，key = vip:rsip） ----------
const batchSelected = ref(new Set<string>())
const rsKey = (vsIp: string, rsIp: string): string => `${vsIp}:${rsIp}`

/** 过滤后列表中可选择的 RS key（排除禁用） */
const validRsKeys = computed(() => {
  const keys: string[] = []
  for (const vs of filteredVs.value) {
    for (const rs of vs.real_servers) {
      if (!rs.disabled) keys.push(rsKey(vs.ip, rs.ip))
    }
  }
  return keys
})

const selectedCount = computed(() => batchSelected.value.size)
const isAllFilteredSelected = computed(
  () => validRsKeys.value.length > 0 && validRsKeys.value.every((k) => batchSelected.value.has(k)),
)

function isRsSelected(vs: VirtualServer, rs: LvsRealServer): boolean {
  return batchSelected.value.has(rsKey(vs.ip, rs.ip))
}

function toggleRs(vs: VirtualServer, rs: LvsRealServer): void {
  const key = rsKey(vs.ip, rs.ip)
  const next = new Set(batchSelected.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  batchSelected.value = next
}

/** 对过滤后列表全选 / 取消全选 */
function toggleAllFiltered(): void {
  const next = new Set(batchSelected.value)
  if (isAllFilteredSelected.value) {
    for (const k of validRsKeys.value) next.delete(k)
  } else {
    for (const k of validRsKeys.value) next.add(k)
  }
  batchSelected.value = next
}

/** 刷新后清理已失效的选择（服务器切换 / 数据变化） */
function pruneBatchSelection(): void {
  const valid = new Set<string>()
  for (const vs of vsList.value) {
    for (const rs of vs.real_servers) valid.add(rsKey(vs.ip, rs.ip))
  }
  const next = new Set([...batchSelected.value].filter((k) => valid.has(k)))
  if (next.size !== batchSelected.value.size) batchSelected.value = next
}

// ---------- 5 分钟静默自动刷新 ----------
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

async function silentRefresh(): Promise<void> {
  // 页面不可见时跳过
  if (!serverId.value || document.hidden) return
  try {
    vsList.value = await lvsApi.list(serverId.value)
    if (vsFilter.value && !vsList.value.some((vs) => vs.ip === vsFilter.value)) vsFilter.value = ''
    pruneBatchSelection()
    if (allExpanded.value) expandedKeys.value = vsList.value.map((vs) => vs.ip)
  } catch {
    // 静默失败
  }
}

onMounted(() => {
  autoRefreshTimer = setInterval(silentRefresh, AUTO_REFRESH_INTERVAL_MS)
  // 已有记忆的服务器时直接加载
  if (serverId.value) void loadList()
})

onUnmounted(() => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
})

// ---------- 预览 → 执行（单台 op / swap） ----------
const previewVisible = ref(false)
const outputVisible = ref(false)
const outputResult = ref<string | string[]>('')
const outputStatus = ref<'success' | 'failed'>('success')

interface PreviewShape {
  preview_id: string
  current_status: string
  command: string
  description: string
}

const pe = usePreviewExecute<PreviewShape>()

interface PendingOp {
  kind: 'op' | 'swap'
  vs: VirtualServer
  rs?: LvsRealServer
  state?: 'on' | 'off'
  swap?: { rs_ip1: string; rs_ip2: string }
}

const pending = ref<PendingOp | null>(null)

function openOutput(res: CommandExecuteResult, ok: boolean): void {
  outputResult.value = res.output
  outputStatus.value = ok ? 'success' : 'failed'
  outputVisible.value = true
}

async function requestOp(vs: VirtualServer, rs: LvsRealServer, state: 'on' | 'off'): Promise<void> {
  if (rs.disabled) {
    ElMessage.warning(`${t('lvs.disabledWarn')}${rs.disabled_reason ? `: ${rs.disabled_reason}` : ''}`)
    return
  }
  if (!serverId.value) return

  // 上线前检查预生产资源副本（need_warning 时人工确认）
  if (state === 'on') {
    try {
      const check = await preprodApi.checkLvsOnline({ vs_ip: vs.ip, rs_ip: rs.ip })
      if (check.need_warning && check.warnings?.length) {
        const lines = check.warnings
          .map((w) => `${w.name} (${w.category}): ${w.current} → ${t('lvs.targetReplicas')} ${w.target}`)
          .join('\n')
        await ElMessageBox.confirm(lines, t('common.confirm'), { type: 'warning' })
      }
    } catch {
      return // 用户取消确认框
    }
  }

  pending.value = { kind: 'op', vs, rs, state }
  const sid = serverId.value
  const ok = await pe.preview(() => lvsApi.opPreview({ server_id: sid, vs_ip: vs.ip, rs_ip: rs.ip, state }))
  if (ok) previewVisible.value = true
}

function requestSwap(vs: VirtualServer, payload: { rs_ip1: string; rs_ip2: string }): void {
  if (!serverId.value) return
  pending.value = { kind: 'swap', vs, swap: payload }
  const sid = serverId.value
  void pe
    .preview(() =>
      lvsApi.swapPreview({ server_id: sid, vs_ip: vs.ip, rs_ip1: payload.rs_ip1, rs_ip2: payload.rs_ip2 }),
    )
    .then((ok) => {
      if (ok) previewVisible.value = true
    })
}

async function executePreview(): Promise<void> {
  // 批量分支
  if (batchPreviews.value.length > 0) {
    await executeBatch()
    return
  }
  const p = pending.value
  if (!p) return
  const result =
    p.kind === 'op'
      ? await pe.execute((id) => lvsApi.opExecute({ preview_id: id }))
      : await pe.execute((id) => lvsApi.swapExecute({ preview_id: id }))

  if (result.ok && result.result) {
    previewVisible.value = false
    openOutput(result.result, true)
    ElMessage.success(t('common.execSuccess'))
    pe.reset()
    void loadList()
  } else if (result.error) {
    if (pe.expired.value) {
      ElMessage.warning(result.error)
    } else {
      ElMessage.error(result.error)
    }
  }
}

function reproview(): void {
  // 批量分支：按目标重新预览
  if (batchPreviews.value.length > 0) {
    previewVisible.value = false
    void createBatchPreviews(batchState.value, batchTargets.value)
    return
  }
  const p = pending.value
  if (!p) return
  previewVisible.value = false
  pe.reset()
  if (p.kind === 'op' && p.rs && p.state) {
    void requestOp(p.vs, p.rs, p.state)
  } else if (p.kind === 'swap' && p.swap) {
    requestSwap(p.vs, p.swap)
  }
}

// ---------- 批量上线 / 批量下线（逐台预览、逐台执行） ----------
interface BatchTarget {
  vs: VirtualServer
  rs: LvsRealServer
}

const batchPreviews = ref<LvsPreview[]>([])
const batchTargets = ref<BatchTarget[]>([])
const batchState = ref<'on' | 'off'>('on')
const batchLoading = ref(false)
const batchExecuting = ref(false)

/** 批量模式下弹窗展示首个预览内容 */
const activePreview = computed<PreviewShape | null>(() => batchPreviews.value[0] ?? pe.previewData.value)
const activeExecuting = computed(() => (batchPreviews.value.length > 0 ? batchExecuting.value : pe.executing.value))

function selectedTargets(): BatchTarget[] {
  const targets: BatchTarget[] = []
  for (const vs of filteredVs.value) {
    for (const rs of vs.real_servers) {
      if (batchSelected.value.has(rsKey(vs.ip, rs.ip))) targets.push({ vs, rs })
    }
  }
  return targets
}

async function createBatchPreviews(state: 'on' | 'off', targets: BatchTarget[]): Promise<void> {
  if (!serverId.value || targets.length === 0) return
  batchLoading.value = true
  try {
    const sid = serverId.value
    const previews: LvsPreview[] = []
    for (const { vs, rs } of targets) {
      previews.push(await lvsApi.opPreview({ server_id: sid, vs_ip: vs.ip, rs_ip: rs.ip, state }))
    }
    batchPreviews.value = previews
    batchTargets.value = targets
    batchState.value = state
    previewVisible.value = true
  } catch (err) {
    ElMessage.error(extractErrorMessage(err) || '预览失败')
  } finally {
    batchLoading.value = false
  }
}

async function requestBatchOnline(): Promise<void> {
  const all = selectedTargets()
  const targets = all.filter(({ rs }) => rs.status !== 'up')
  if (targets.length === 0) {
    ElMessage.warning('所选 RS 均已在线，无需上线')
    return
  }
  const skipped = all.length - targets.length
  if (skipped > 0) ElMessage.warning(`${skipped} 台已在线将跳过，将上线 ${targets.length} 台离线 RS`)

  // 上线前逐台检查预生产资源副本（need_warning 时统一确认一次）
  const warnings: string[] = []
  for (const { vs, rs } of targets) {
    try {
      const check = await preprodApi.checkLvsOnline({ vs_ip: vs.ip, rs_ip: rs.ip })
      if (check.need_warning && check.warnings?.length) {
        warnings.push(
          ...check.warnings.map((w) => `${w.name} (${w.category}): ${w.current} → ${t('lvs.targetReplicas')} ${w.target}`),
        )
      }
    } catch (err) {
      ElMessage.error(`预生产安全检查失败: ${extractErrorMessage(err)}，操作中止`)
      return
    }
  }
  if (warnings.length > 0) {
    try {
      await ElMessageBox.confirm(warnings.join('\n'), t('common.confirm'), { type: 'warning' })
    } catch {
      return
    }
  }
  await createBatchPreviews('on', targets)
}

async function requestBatchOffline(): Promise<void> {
  const all = selectedTargets()
  const targets = all.filter(({ rs }) => rs.status === 'up')
  if (targets.length === 0) {
    ElMessage.warning('所选 RS 均已离线，无需下线')
    return
  }
  // 每个 VS 至少保留一台在线 RS
  const byVs = new Map<string, BatchTarget[]>()
  for (const item of targets) {
    const list = byVs.get(item.vs.ip) ?? []
    list.push(item)
    byVs.set(item.vs.ip, list)
  }
  for (const [vip, list] of byVs) {
    const group = vsList.value.find((vs) => vs.ip === vip)
    if (!group) continue
    const upCount = group.real_servers.filter((r) => r.status === 'up').length
    if (upCount - list.length < 1) {
      ElMessage.warning(`VIP ${vip} 下线后将无在线服务器，至少需要保留一台`)
      return
    }
  }
  const skipped = all.length - targets.length
  if (skipped > 0) ElMessage.warning(`${skipped} 台已离线将自动跳过`)
  await createBatchPreviews('off', targets)
}

async function executeBatch(): Promise<void> {
  batchExecuting.value = true
  let allOutput = ''
  let hasError = false
  let expired = false
  try {
    for (const p of batchPreviews.value) {
      try {
        const res = await lvsApi.opExecute({ preview_id: p.preview_id })
        allOutput += (res.output || '') + '\n'
      } catch (err) {
        const msg = extractErrorMessage(err)
        allOutput += `[错误] ${msg}\n`
        hasError = true
        if (msg.includes('预览已过期')) expired = true
      }
    }
  } finally {
    batchExecuting.value = false
  }
  if (expired) ElMessage.warning(t('preview.expired'))
  previewVisible.value = false
  outputResult.value = allOutput
  outputStatus.value = hasError ? 'failed' : 'success'
  outputVisible.value = true
  if (!hasError) ElMessage.success(t('common.execSuccess'))
  batchSelected.value = new Set()
  batchPreviews.value = []
  batchTargets.value = []
  void loadList()
}

// ---------- 状态 / 切换 / 标签 / 绑定 ----------
const statusDialogRef = ref<InstanceType<typeof StatusDialog>>()
const swapDialogRef = ref<InstanceType<typeof SwapDialog>>()
const tagDialogRef = ref<InstanceType<typeof TagDialog>>()
const bindingsVisible = ref(false)
const swapVs = ref<VirtualServer | null>(null)
const tagMode = ref<'rs' | 'vs'>('rs')

function openSwap(vs: VirtualServer): void {
  swapVs.value = vs
  swapDialogRef.value?.open()
}

function openRsTag(vs: VirtualServer, rs: LvsRealServer): void {
  tagMode.value = 'rs'
  tagDialogRef.value?.open({
    rs_ip: rs.ip,
    vs_ip: vs.ip,
    tag: rs.tag ?? '',
    disabled: rs.disabled ?? false,
    disabled_reason: rs.disabled_reason ?? '',
  })
}

function openVsTag(vs: VirtualServer): void {
  tagMode.value = 'vs'
  tagDialogRef.value?.open({ vs_ip: vs.ip, tag: vs.tag ?? '' })
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('lvs.title') }}</h1>
        <p class="page-subtitle">{{ t('lvs.subtitle') }}</p>
      </div>
      <div class="page-actions head-actions">
        <ServerSelect ref="serverSelectRef" v-model:server-id="serverId" type="lvs" />
        <el-select
          v-model="vsFilter"
          :placeholder="t('lvs.vs')"
          clearable
          filterable
          style="width: 170px"
        >
          <el-option v-for="ip in vsOptions" :key="ip" :label="ip" :value="ip" />
        </el-select>
        <el-input v-model="keyword" :placeholder="t('logs.keyword')" clearable style="width: 180px" />
        <el-button :loading="listLoading" :disabled="!serverId" @click="loadList">
          {{ t('common.refresh') }}
        </el-button>
        <el-button :disabled="!serverId" @click="statusDialogRef?.open(serverId!)">
          {{ t('lvs.status') }}
        </el-button>
        <el-button v-if="auth.isAdmin" @click="bindingsVisible = true">{{ t('lvs.bindings') }}</el-button>
      </div>
    </div>

    <!-- 批量操作栏 -->
    <div v-if="serverId" class="page-actions" style="margin-bottom: var(--space-4)">
      <el-button size="small" @click="toggleExpandAll">
        {{ allExpanded ? t('common.collapseAll') : t('common.expandAll') }}
      </el-button>
      <el-button size="small" :disabled="validRsKeys.length === 0" @click="toggleAllFiltered">
        {{ isAllFilteredSelected ? '取消全选' : t('common.selectAll') }}
      </el-button>
      <el-divider direction="vertical" />
      <el-button type="success" size="small" :disabled="selectedCount === 0" :loading="batchLoading" @click="requestBatchOnline">
        批量上线
      </el-button>
      <el-button type="danger" size="small" :disabled="selectedCount === 0" :loading="batchLoading" @click="requestBatchOffline">
        批量下线
      </el-button>
      <span class="mono selected-info">已选 {{ selectedCount }} 项</span>
    </div>

    <div v-loading="listLoading" class="card table-card reveal d-1">
      <el-empty v-if="!serverId" :description="t('common.serverPlaceholder')" />
      <el-empty v-else-if="filteredVs.length === 0 && !listLoading" :description="t('common.empty')" />
      <el-table
        v-else
        :data="filteredVs"
        row-key="ip"
        :expand-row-keys="expandedKeys"
        @expand-change="onExpandChange"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="rs-wrap">
              <el-table :data="row.real_servers" size="small">
                <el-table-column width="42" align="center">
                  <template #default="{ row: rs }">
                    <el-checkbox
                      v-if="!(rs as LvsRealServer).disabled"
                      :model-value="isRsSelected(row as VirtualServer, rs as LvsRealServer)"
                      @change="toggleRs(row as VirtualServer, rs as LvsRealServer)"
                    />
                  </template>
                </el-table-column>
                <el-table-column :label="`RS : ${t('lvs.port')}`" min-width="170">
                  <template #default="{ row: rs }">
                    <span class="mono">{{ rs.ip }}:{{ rs.port }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('logs.status')" width="100">
                  <template #default="{ row: rs }">
                    <span class="dot-live" :class="{ 'is-offline': rs.status !== 'up' }" />
                    <span class="status-text" :class="rs.status === 'up' ? 'ok' : 'bad'">
                      {{ rs.status === 'up' ? t('common.online') : t('common.offline') }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column prop="weight" :label="t('lvs.weight')" width="80" />
                <el-table-column prop="active_conn" :label="t('lvs.activeConn')" width="100" sortable />
                <el-table-column prop="inact_conn" :label="t('lvs.inactConn')" width="110" sortable />
                <el-table-column :label="t('lvs.tag')" min-width="140">
                  <template #default="{ row: rs }">
                    <el-tag v-if="rs.tag" size="small" type="info" effect="plain" class="mr">{{
                      rs.tag
                    }}</el-tag>
                    <el-tooltip v-if="rs.disabled" :content="rs.disabled_reason" placement="top">
                      <el-tag size="small" type="danger" effect="plain">{{ t('common.disabled') }}</el-tag>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column :label="t('common.operation')" width="220" fixed="right">
                  <template #default="{ row: rs }">
                    <el-button
                      link
                      type="success"
                      size="small"
                      :disabled="rs.status === 'up' || Boolean(rs.disabled)"
                      @click="requestOp(row as VirtualServer, rs as LvsRealServer, 'on')"
                    >
                      {{ t('lvs.online') }}
                    </el-button>
                    <el-button
                      link
                      type="danger"
                      size="small"
                      :disabled="rs.status !== 'up' || Boolean(rs.disabled)"
                      @click="requestOp(row as VirtualServer, rs as LvsRealServer, 'off')"
                    >
                      {{ t('lvs.offline') }}
                    </el-button>
                    <el-button link size="small" @click="openSwap(row as VirtualServer)">{{ t('lvs.swap') }}</el-button>
                    <el-button v-if="auth.isAdmin" link size="small" @click="openRsTag(row as VirtualServer, rs as LvsRealServer)">
                      {{ t('common.edit') }}
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="VS : Port" min-width="160">
          <template #default="{ row }">
            <span class="mono vs-ip">{{ row.ip }}:{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="protocol" :label="t('lvs.protocol')" width="90" />
        <el-table-column prop="scheduler" :label="t('lvs.scheduler')" width="100" />
        <el-table-column :label="t('lvs.role')" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.role" size="small" :type="row.role === 'master' ? 'primary' : 'info'" effect="plain">
              {{ row.role === 'master' ? t('lvs.master') : t('lvs.backup') }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('lvs.tag')" min-width="140">
          <template #default="{ row }">
            <el-tag v-if="row.tag" size="small" effect="plain" class="mr">{{ row.tag }}</el-tag>
            <el-button v-if="auth.isAdmin" link size="small" @click="openVsTag(row as VirtualServer)">
              {{ row.tag ? t('common.edit') : t('common.add') }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column :label="`RS (${t('common.online')})`" width="120">
          <template #default="{ row }">
            <span class="mono">{{ rsCount(row as VirtualServer) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 预览执行（单台 / 批量共用） -->
    <PreviewDialog
      v-model:visible="previewVisible"
      :description="activePreview?.description ?? ''"
      :current-status="activePreview?.current_status ?? ''"
      :commands="activePreview ? [activePreview.command] : []"
      :countdown="batchPreviews.length > 0 ? 0 : pe.countdown.value"
      :expired="batchPreviews.length > 0 ? false : pe.expired.value"
      :executing="activeExecuting"
      @execute="executePreview"
      @repreview="reproview"
    />

    <OutputDialog v-model:visible="outputVisible" :output="outputResult" :status="outputStatus" />

    <!-- 状态 / 切换 / 标签 / 绑定 -->
    <StatusDialog ref="statusDialogRef" />
    <SwapDialog ref="swapDialogRef" :vs="swapVs" @confirm="requestSwap(swapVs!, $event)" />
    <TagDialog ref="tagDialogRef" :mode="tagMode" @saved="loadList" />
    <BindingsDialog v-model:visible="bindingsVisible" @changed="loadList" />
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

.selected-info {
  color: var(--text-secondary);
  font-size: var(--text-sm);
}

.vs-table :deep(.el-table__expanded-cell) {
  padding: 0;
}

.rs-wrap {
  padding: var(--space-3) var(--space-6);
  background: var(--bg-deep);
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

.vs-ip {
  font-weight: 600;
}

.mr {
  margin-right: var(--space-2);
}
</style>
