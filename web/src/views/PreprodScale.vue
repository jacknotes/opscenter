<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { preprodApi, lvsApi, extractErrorMessage } from '@/api'
import type { PreprodPreview, PreprodResource, LvsScaledownCheck, WsMessage } from '@/api/types'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { useOutputCache } from '@/composables/useOutputCache'
import { getToken } from '@/utils/session'
import { STORAGE_KEYS } from '@/utils/constants'
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

// 已有记忆的服务器时直接加载
onMounted(() => {
  if (serverId.value) void loadList()
})

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return resources.value
  return resources.value.filter((r) => r.name.toLowerCase().includes(kw))
})

const categoryType = (c: string): 'primary' | 'warning' | 'info' =>
  c === 'rollout' ? 'primary' : c === 'statefulset' ? 'warning' : 'info'

// ---------- 缩扩容（预览） ----------
const previewVisible = ref(false)

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
  if (ok) previewVisible.value = true
}

// ---------- WebSocket 流式执行（页级持有，支持输出缓存） ----------
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
  if (message) ElMessage.error(message)
  // 契约：执行失败时预览已删除（WS 语义），需要重新预览
  pe.reset()
}

function connectStream(previewId: string): void {
  disconnectStream()
  streamLines.value = []
  streamState.value = 'running'

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

onBeforeUnmount(disconnectStream)

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
