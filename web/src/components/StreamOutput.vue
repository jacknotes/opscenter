<script setup lang="ts">
import { ref, watch, onBeforeUnmount, nextTick } from 'vue'
import { getToken } from '@/utils/session'
import { i18n } from '@/i18n'
import type { WsMessage } from '@/api/types'

const t = i18n.global.t

const props = defineProps<{
  /** 传入非空 previewId 即开始执行流；置空则断开 */
  previewId: string
}>()

const emit = defineEmits<{
  (e: 'done'): void
  (e: 'failed', message: string): void
}>()

interface StreamLine {
  text: string
  stream: 'stdout' | 'stderr' | 'system'
}

const lines = ref<StreamLine[]>([])
const running = ref(false)
const finished = ref<null | 'success' | 'failed'>(null)
const terminalRef = ref<HTMLElement>()

let ws: WebSocket | null = null

function push(line: StreamLine): void {
  lines.value.push(line)
  void nextTick(() => {
    const el = terminalRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function connect(previewId: string): void {
  disconnect()
  lines.value = []
  finished.value = null
  running.value = true

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
        push({ text: msg.data ?? '', stream: msg.stream === 'stderr' ? 'stderr' : 'stdout' })
        break
      case 'done':
        finished.value = 'success'
        running.value = false
        push({ text: `✔ ${t('common.execSuccess')}`, stream: 'system' })
        emit('done')
        disconnect()
        break
      case 'lock_error':
        finished.value = 'failed'
        running.value = false
        push({ text: `✖ ${t('ws.lockError', { holder: msg.holder ?? '' })}`, stream: 'system' })
        emit('failed', msg.message ?? '')
        disconnect()
        break
      case 'error':
        finished.value = 'failed'
        running.value = false
        push({ text: `✖ ${msg.message ?? t('common.execFailed')}`, stream: 'system' })
        emit('failed', msg.message ?? '')
        disconnect()
        break
    }
  }

  ws.onerror = () => {
    if (running.value) {
      finished.value = 'failed'
      running.value = false
      push({ text: `✖ ${t('ws.disconnected')}`, stream: 'system' })
      emit('failed', t('ws.disconnected'))
    }
  }

  ws.onclose = () => {
    // 连接异常关闭且无 done → 按失败处理
    if (running.value) {
      running.value = false
      if (finished.value === null) {
        finished.value = 'failed'
        push({ text: `✖ ${t('ws.disconnected')}`, stream: 'system' })
        emit('failed', t('ws.disconnected'))
      }
    }
  }
}

function disconnect(): void {
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

watch(
  () => props.previewId,
  (id) => {
    if (id) connect(id)
    else disconnect()
  },
)

onBeforeUnmount(disconnect)

defineExpose({ disconnect })
</script>

<template>
  <div class="stream-output">
    <div class="stream-header mono">
      <span class="dot-live" :class="{ 'is-offline': finished !== null }" />
      <span v-if="running">{{ t('ws.connected') }} · {{ t('preview.executing') }}</span>
      <span v-else-if="finished === 'success'">{{ t('common.execSuccess') }}</span>
      <span v-else-if="finished === 'failed'">{{ t('common.execFailed') }}</span>
      <span v-else>{{ t('ws.connecting') }}</span>
    </div>
    <div ref="terminalRef" class="terminal mono">
      <div v-for="(line, idx) in lines" :key="idx" class="stream-line" :class="`stream-${line.stream}`">
        {{ line.text }}
      </div>
      <div v-if="lines.length === 0" class="stream-line stream-system">{{ t('ws.connecting') }}</div>
    </div>
  </div>
</template>

<style scoped>
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

.terminal {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: 0 0 var(--radius-sm) var(--radius-sm);
  padding: var(--space-3);
  height: 380px;
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
