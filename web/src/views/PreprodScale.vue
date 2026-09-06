<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { preprodApi, lvsApi, extractErrorMessage } from '@/api'
import type { PreprodPreview, PreprodResource, LvsScaledownCheck, LvsScaledownWarning, WsMessage } from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { useOutputCache } from '@/composables/useOutputCache'
import { useTablePaging } from '@/composables/useTablePaging'
import { useSelection } from '@/composables/useSelection'
import { useExecStreamStore } from '@/stores/execStream'
import { PAGE_SIZES, STORAGE_KEYS } from '@/utils/constants'
import { getToken } from '@/utils/session'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'

const t = i18n.global.t

// ---------- 资源列表 ----------
const serverId = ref<number>()
// 服务器选择记忆（v1 useServerSelector 行为）
const savedPreprodServer = localStorage.getItem(STORAGE_KEYS.PREPROD_SERVER)
if (savedPreprodServer) serverId.value = Number(savedPreprodServer)
watch(serverId, (v) => {
  if (v) localStorage.setItem(STORAGE_KEYS.PREPROD_SERVER, String(v))
})
const resources = ref<PreprodResource[]>([])
const listLoading = ref(false)
const keyword = ref('')
const statusFilter = ref<'all' | 'up' | 'down'>('all')

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

// 已有记忆的服务器时直接加载
onMounted(() => {
  if (serverId.value) void loadList()
})

// 状态判定（对齐 v1）：ready_desired/ready 反映 Pod 真实就绪状态
function scaleStatus(r: PreprodResource): 'down' | 'ok' | 'starting' {
  if (r.ready_desired === 0 && r.ready === 0) return 'down'
  if (r.ready > 0 && r.ready === r.ready_desired) return 'ok'
  return 'starting'
}

const statusTagType = (s: 'down' | 'ok' | 'starting'): 'info' | 'success' | 'warning' =>
  s === 'down' ? 'info' : s === 'ok' ? 'success' : 'warning'

const statusLabels: Record<'all' | 'up' | 'down' | 'ok' | 'starting', string> = {
  all: '全部',
  up: '已扩容',
  down: '已缩容',
  ok: '正常',
  starting: '启动中',
}

const filtered = computed(() => {
  let list = resources.value
  if (statusFilter.value === 'up') list = list.filter((r) => r.ready_desired > 0 && r.ready >= r.ready_desired)
  else if (statusFilter.value === 'down') list = list.filter((r) => r.ready_desired === 0 && r.ready === 0)
  const kw = keyword.value.trim().toLowerCase()
  if (kw) list = list.filter((r) => r.name.toLowerCase().includes(kw) || r.category.toLowerCase().includes(kw))
  return list
})

// ---------- 分页 + 跨页勾选 ----------
const { currentPage, pageSize, total, paged } = useTablePaging(filtered)
const { selectedIds, tableRef, handleSelectionChange, handleSizeChange, handleCurrentChange, restoreSelection } =
  useSelection<PreprodResource>('name', paged, { search: keyword, currentPage })

const selected = computed(() => filtered.value.filter((r) => selectedIds.value.has(r.name)))

watch([statusFilter, () => resources.value], () => nextTick(() => restoreSelection()))

const categoryType = (c: string): 'primary' | 'warning' | 'info' =>
  c === 'rollout' ? 'primary' : c === 'statefulset' ? 'warning' : 'info'

// ---------- 批量操作安全确认（对齐 v1：LVS 检查 → require 依赖警告 → 资源清单确认） ----------
type ScaleAction = 'scaledown' | 'scaleup'
const BATCH_THRESHOLD = 10

// -- 缩容前 LVS 检查 --
const lvsCheckVisible = ref(false)
const lvsCheckWarnings = ref<LvsScaledownWarning[]>([])
const lvsCheckConfirmText = ref('')
let lvsCheckResolve: ((ok: boolean) => void) | null = null

function settleLvsCheck(ok: boolean): void {
  lvsCheckVisible.value = false
  lvsCheckResolve?.(ok)
  lvsCheckResolve = null
}

/** 缩容前的 LVS RS 上线检查；need_warning 时弹表格 + 强确认，检查接口失败则中止操作 */
async function lvsCheckGate(): Promise<boolean> {
  if (!serverId.value) return false
  try {
    const check: LvsScaledownCheck = await lvsApi.checkScaledown(serverId.value)
    if (!check.need_warning || !check.warnings?.length) return true
    lvsCheckWarnings.value = check.warnings
    lvsCheckConfirmText.value = ''
    lvsCheckVisible.value = true
    return new Promise<boolean>((resolve) => {
      lvsCheckResolve = resolve
    })
  } catch (err) {
    ElMessage.error(`LVS 安全检查失败: ${extractErrorMessage(err)}，操作中止`)
    return false
  }
}

// -- require 依赖服务警告 --
const depWarningVisible = ref(false)
const depWarningText = ref('')
const depWarningAffected = ref<string[]>([])
const depWarningConfirmText = ref('')
let depWarningProceed: (() => void) | null = null

function showDepWarning(text: string, affected: string[], proceed: () => void): void {
  depWarningText.value = text
  depWarningAffected.value = affected
  depWarningConfirmText.value = ''
  depWarningProceed = proceed
  depWarningVisible.value = true
}

function onDepWarningConfirm(): void {
  depWarningVisible.value = false
  depWarningProceed?.()
  depWarningProceed = null
}

const requireSet = computed(
  () => new Set(resources.value.filter((r) => r.category === 'require').map((r) => r.name)),
)

// -- 资源清单确认（超过阈值需输入"确认执行"） --
const batchConfirmVisible = ref(false)
const batchConfirmAction = ref<ScaleAction>('scaledown')
const batchConfirmNames = ref<string[]>([])
const batchConfirmText = ref('')
const batchConfirmSkipCount = ref(0)
const batchConfirmTotalCount = ref(0)
const batchConfirmIsFull = ref(false)

function doBatchConfirm(action: ScaleAction, names: string[], skipCount: number, total_: number, isFull: boolean): void {
  batchConfirmAction.value = action
  batchConfirmNames.value = names
  batchConfirmText.value = ''
  batchConfirmSkipCount.value = skipCount
  batchConfirmTotalCount.value = total_
  batchConfirmIsFull.value = isFull
  batchConfirmVisible.value = true
}

function onBatchConfirm(): void {
  const action = batchConfirmAction.value
  const names = batchConfirmIsFull.value ? [] : batchConfirmNames.value
  batchConfirmVisible.value = false
  void doPreview(action, names)
}

// ---------- 缩扩容（预览） ----------
const previewVisible = ref(false)

const pe = usePreviewExecute<PreprodPreview>()

const pendingAction = ref<ScaleAction>('scaledown')

async function doPreview(action: ScaleAction, names: string[]): Promise<void> {
  const sid = serverId.value
  if (!sid) return
  const ok = await pe.preview(() =>
    action === 'scaledown'
      ? preprodApi.scaledownPreview({ server_id: sid, resource_names: names })
      : preprodApi.scaleupPreview({ server_id: sid, resource_names: names }),
  )
  if (ok) previewVisible.value = true
}

async function scalePreview(action: ScaleAction): Promise<void> {
  if (!serverId.value) return
  pendingAction.value = action

  // 未勾选 = 全量（契约：resource_names 为空时操作所有资源）
  const isFull = selected.value.length === 0
  const pool = isFull ? filtered.value : selected.value
  const targets =
    action === 'scaledown' ? pool.filter((r) => r.current > 0) : pool.filter((r) => r.current < r.target_replicas)
  const skipCount =
    action === 'scaledown'
      ? pool.filter((r) => r.current === 0).length
      : pool.filter((r) => r.current >= r.target_replicas).length
  const names = targets.map((r) => r.name)
  if (names.length === 0) return

  if (action === 'scaledown') {
    // 1) LVS RS 上线状态检查
    const passed = await lvsCheckGate()
    if (!passed) return
    // 2) 依赖(require)服务警告：全量时脚本自动处理依赖，仅批量操作提示
    if (!isFull) {
      const requireTargets = names.filter((n) => requireSet.value.has(n))
      if (requireTargets.length > 0) {
        const stillRunning = resources.value
          .filter((r) => !requireSet.value.has(r.name) && r.current > 0)
          .map((r) => r.name)
          .filter((n) => !names.includes(n))
        if (stillRunning.length > 0) {
          showDepWarning('依赖(require)服务停止可能会影响其它服务运行！', stillRunning, () =>
            doBatchConfirm(action, names, skipCount, pool.length, isFull),
          )
          return
        }
      }
    }
  } else if (!isFull) {
    // 扩容：勾选了非依赖服务，但存在未运行的依赖服务且不在本次扩容名单中
    const nonRequireTargets = names.filter((n) => !requireSet.value.has(n))
    if (nonRequireTargets.length > 0) {
      const stillMissing = resources.value
        .filter((r) => requireSet.value.has(r.name) && r.current === 0)
        .map((r) => r.name)
        .filter((n) => !names.includes(n))
      if (stillMissing.length > 0) {
        showDepWarning('依赖(require)服务未运行，运行所选服务可能会发生异常！', stillMissing, () =>
          doBatchConfirm(action, names, skipCount, pool.length, isFull),
        )
        return
      }
    }
  }

  doBatchConfirm(action, names, skipCount, pool.length, isFull)
}

// ---------- WebSocket 流式执行（页级持有，支持输出缓存） ----------
// keep-alive 下切页组件不销毁：WS 连接与输出天然跨页存活；
// execStream store 承载状态摘要，供 AppLayout 底部状态栏跨页展示
const exec = useExecStreamStore()

interface StreamLine {
  text: string
  stream: 'stdout' | 'stderr' | 'system'
}

type StreamState = 'idle' | 'running' | 'done' | 'failed'

const streamLines = ref<StreamLine[]>([])
const streamState = ref<StreamState>('idle')
const terminalRef = ref<HTMLElement>()

let ws: WebSocket | null = null

function pushLine(line: StreamLine): void {
  streamLines.value.push(line)
  exec.report(streamLines.value.length)
  void nextTick(() => {
    const el = terminalRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function disconnectStream(): void {
  if (ws) {
    ws.onopen = ws.onmessage = ws.onerror = ws.onclose = null
    try {
      ws.close()
    } catch {
      /* 忽略 */
    }
    ws = null
  }
}

function streamFail(message: string): void {
  streamState.value = 'failed'
  exec.finish('failed')
  if (message) ElMessage.error(message)
  // 契约：执行失败时预览已删除（WS 语义），需要重新预览
  pe.reset()
}

function connectStream(previewId: string): void {
  disconnectStream()
  streamLines.value = []
  streamState.value = 'running'
  if (serverId.value) exec.begin(serverId.value)

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/api/ws/exec?token=${encodeURIComponent(getToken())}`
  ws = new WebSocket(url)

  ws.onopen = () => {
    // 契约：连接后首条消息必须是 start
    ws?.send(JSON.stringify({ type: 'start', preview_id: previewId }))
  }

  ws.onmessage = (ev: MessageEvent) => {
    let msg: WsMessage
    try {
      msg = JSON.parse(ev.data as string) as WsMessage
    } catch {
      return
    }
    switch (msg.type) {
      case 'output':
        pushLine({ text: msg.data ?? '', stream: msg.stream === 'stderr' ? 'stderr' : 'stdout' })
        break
      case 'done':
        streamState.value = 'done'
        exec.finish('done')
        pushLine({ text: `✔ ${t('common.execSuccess')}`, stream: 'system' })
        ElMessage.success(t('common.execSuccess'))
        pe.reset()
        disconnectStream()
        void loadList()
        break
      case 'lock_error':
        pushLine({ text: `✖ ${t('ws.lockError', { holder: msg.holder ?? '' })}`, stream: 'system' })
        streamFail(msg.message ?? '')
        disconnectStream()
        break
      case 'error':
        pushLine({ text: `✖ ${msg.message ?? t('common.execFailed')}`, stream: 'system' })
        streamFail(msg.message ?? '')
        disconnectStream()
        break
    }
  }

  ws.onerror = () => {
    if (streamState.value === 'running') {
      pushLine({ text: `✖ ${t('ws.disconnected')}`, stream: 'system' })
      streamFail(t('ws.disconnected'))
    }
  }

  ws.onclose = () => {
    // 连接异常关闭且无 done → 按失败处理
    if (streamState.value === 'running') {
      pushLine({ text: `✖ ${t('ws.disconnected')}`, stream: 'system' })
      streamFail(t('ws.disconnected'))
    }
  }
}

onBeforeUnmount(() => {
  disconnectStream()
  exec.clear()
})

/** 确认后启动 WebSocket 流式执行 */
function startStream(): void {
  const id = pe.previewData.value?.preview_id
  if (!id) return
  previewVisible.value = false
  connectStream(id)
}

function reproview(): void {
  previewVisible.value = false
  pe.reset()
  void scalePreview(pendingAction.value)
}

// 切换服务器时缓存/恢复执行输出（执行中不切换，与 v1 行为一致）
useOutputCache([() => serverId.value ?? ''], streamLines, {
  getExtra: () => streamState.value,
  setExtra: (extra) => {
    streamState.value = extra ?? 'idle'
  },
  blockCondition: computed(() => streamState.value === 'running'),
  emptyValue: () => [],
})
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
        <el-select v-model="statusFilter" style="width: 110px">
          <el-option value="all" :label="statusLabels.all" />
          <el-option value="up" :label="statusLabels.up" />
          <el-option value="down" :label="statusLabels.down" />
        </el-select>
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
      <template v-else>
        <el-table
          ref="tableRef"
          :data="paged"
          row-key="name"
          @selection-change="handleSelectionChange"
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
          <el-table-column prop="desired" :label="t('preprod.desired')" width="90" sortable />
          <el-table-column :label="t('preprod.current')" width="150" sortable prop="current">
            <template #default="{ row }">
              <span class="mono">{{ row.current }}</span>
              <el-tag :type="statusTagType(scaleStatus(row as PreprodResource))" size="small" style="margin-left: 4px">
                {{ statusLabels[scaleStatus(row as PreprodResource)] }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="就绪" width="90">
            <template #default="{ row }">
              <span class="mono">{{ row.ready }}/{{ row.ready_desired }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="up_to_date" :label="t('preprod.upToDate')" width="90" />
          <el-table-column prop="available" :label="t('preprod.available')" width="90" />
          <el-table-column prop="age" :label="t('preprod.age')" width="90" />
          <el-table-column :label="t('preprod.targetReplicas')" width="110">
            <template #default="{ row }">
              <span class="mono">{{ row.target_replicas || '-' }}</span>
            </template>
          </el-table-column>
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

    <!-- 执行输出（页级持有，切换服务器时缓存/恢复） -->
    <div v-if="streamState !== 'idle'" class="card output-card reveal d-1">
      <div class="stream-header mono">
        <span class="dot-live" :class="{ 'is-offline': streamState !== 'running' }" />
        <span v-if="streamState === 'running'">{{ t('ws.connected') }} · {{ t('preview.executing') }}</span>
        <span v-else-if="streamState === 'done'">{{ t('common.execSuccess') }}</span>
        <span v-else>{{ t('common.execFailed') }}</span>
        <span class="stream-title">{{ t('preprod.streamOutput') }}</span>
      </div>
      <div ref="terminalRef" class="terminal mono">
        <div v-for="(line, idx) in streamLines" :key="idx" class="stream-line" :class="`stream-${line.stream}`">
          {{ line.text }}
        </div>
        <div v-if="streamLines.length === 0" class="stream-line stream-system">{{ t('ws.connecting') }}</div>
      </div>
    </div>

    <!-- 预览；确认后关闭弹窗，由页级 WebSocket 流式执行 -->
    <PreviewDialog
      v-model:visible="previewVisible"
      :description="pe.previewData.value?.description ?? ''"
      :current-status="pe.previewData.value?.current_status ?? ''"
      :commands="pe.previewData.value ? [pe.previewData.value.command] : []"
      :countdown="pe.countdown.value"
      :expired="pe.expired.value"
      :executing="pe.executing.value"
      @execute="startStream"
      @repreview="reproview"
    />

    <!-- 缩容前 LVS 检查（全量警告表格 + 强确认） -->
    <el-dialog v-model="lvsCheckVisible" title="缩容前检查" width="min(640px, 92vw)" align-center @close="settleLvsCheck(false)">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        <template #title>以下 LVS RS 仍处于上线状态，缩容前请确认已下线：</template>
      </el-alert>
      <el-table :data="lvsCheckWarnings" stripe size="small" border max-height="300">
        <el-table-column prop="status" label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'up' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="lvs_server" label="LVS服务器" min-width="110" />
        <el-table-column prop="vs_tag" label="VS标签" min-width="110" />
        <el-table-column prop="rs_env_tag" label="RS标签" min-width="110" />
        <el-table-column prop="rs_ip" label="RS IP" min-width="130" />
      </el-table>
      <el-alert type="info" :closable="false" style="margin-top: 12px"> 请输入"确认执行"以继续缩容操作 </el-alert>
      <el-input v-model="lvsCheckConfirmText" placeholder="请输入 确认执行" style="margin-top: 8px" />
      <template #footer>
        <el-button @click="settleLvsCheck(false)">取消</el-button>
        <el-button type="primary" :disabled="lvsCheckConfirmText !== '确认执行'" @click="settleLvsCheck(true)">
          确认执行
        </el-button>
      </template>
    </el-dialog>

    <!-- require 依赖服务警告 -->
    <el-dialog
      v-model="depWarningVisible"
      title="操作警告"
      width="min(520px, 90vw)"
      align-center
      :close-on-click-modal="false"
    >
      <div style="margin-bottom: 16px">
        <div style="display: flex; align-items: flex-start; gap: 10px; margin-bottom: 16px">
          <span class="dep-warn-icon">⚠</span>
          <div>
            <div class="dep-warn-text">{{ depWarningText }}</div>
            <div class="dep-warn-sub">涉及资源：</div>
          </div>
        </div>
        <el-scrollbar max-height="200px">
          <div class="dep-warn-list">
            <div v-for="(name, idx) in depWarningAffected" :key="name" class="dep-warn-item">
              <span class="dep-warn-idx">{{ idx + 1 }}.</span>
              <span>{{ name }}</span>
            </div>
          </div>
        </el-scrollbar>
        <div class="dep-warn-hint">如果确认执行，请在下方输入框中输入 <b>确认执行</b></div>
        <el-input v-model="depWarningConfirmText" placeholder='请输入"确认执行"' style="margin-top: 8px" />
      </div>
      <template #footer>
        <el-button @click="depWarningVisible = false">取消</el-button>
        <el-button type="danger" :disabled="depWarningConfirmText !== '确认执行'" @click="onDepWarningConfirm">
          确认执行
        </el-button>
      </template>
    </el-dialog>

    <!-- 批量操作资源清单确认 -->
    <el-dialog v-model="batchConfirmVisible" :title="batchConfirmAction === 'scaledown' ? '批量缩容确认' : '批量扩容确认'" width="min(580px, 90vw)" align-center>
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
          <span style="font-size: 14px">
            {{ batchConfirmAction === 'scaledown' ? '以下资源将缩容至 0 副本:' : '以下资源将扩容至目标副本数:' }}
          </span>
          <el-tag size="small" type="info">共 {{ batchConfirmNames.length }} 项</el-tag>
        </div>
        <el-scrollbar max-height="320px">
          <div class="confirm-name-list">
            <div v-for="(name, idx) in batchConfirmNames" :key="name" class="confirm-name-item">
              <span class="confirm-name-idx">{{ idx + 1 }}.</span>
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

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  padding-top: var(--space-3);
}

.dep-warn-icon {
  color: var(--el-color-danger, #f56c6c);
  font-size: 20px;
  line-height: 1;
}

.dep-warn-text {
  color: var(--el-color-danger, #f56c6c);
  font-weight: 600;
  font-size: 14px;
  line-height: 1.6;
  margin-bottom: 8px;
}

.dep-warn-sub {
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.6;
}

.dep-warn-list {
  background: rgba(245, 108, 108, 0.1);
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid rgba(245, 108, 108, 0.3);
}

.dep-warn-item {
  font-size: 13px;
  line-height: 2;
  padding: 0 4px;
  display: flex;
  align-items: center;
  border-bottom: 1px dashed rgba(245, 108, 108, 0.2);
}

.dep-warn-idx {
  color: var(--text-muted);
  font-size: 12px;
  margin-right: 8px;
  min-width: 28px;
}

.dep-warn-hint {
  margin-top: 12px;
  color: var(--text-muted);
  font-size: 12px;
}

.confirm-name-list {
  background: var(--bg-deep, var(--bg-elevated, #f8fafc));
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
}

.confirm-name-item {
  font-size: 13px;
  line-height: 2;
  padding: 0 4px;
  display: flex;
  align-items: center;
  border-bottom: 1px dashed var(--border);
}

.confirm-name-idx {
  color: var(--text-muted);
  font-size: 12px;
  margin-right: 8px;
  min-width: 28px;
}

.output-card {
  margin-top: var(--space-4);
  padding: var(--space-3);
}

.stream-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--text-secondary);
  font-size: var(--text-xs);
  padding: var(--space-2) var(--space-3);
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-bottom: none;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
}

.stream-title {
  margin-left: auto;
  color: var(--text-muted);
}

.terminal {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: 0 0 var(--radius-sm) var(--radius-sm);
  padding: var(--space-3);
  height: 320px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.75;
}

.stream-line {
  white-space: pre-wrap;
  word-break: break-all;
}

.stream-stdout {
  color: var(--text-primary);
}

.stream-stderr {
  color: var(--amber-400);
}

.stream-system {
  color: var(--text-muted);
}
</style>
