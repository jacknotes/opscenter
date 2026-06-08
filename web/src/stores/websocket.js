import { defineStore } from 'pinia'
import { ref, shallowRef, triggerRef } from 'vue'

/**
 * 全局 WebSocket store，用于跨页面保持命令执行状态。
 * 解决切换页面后命令执行变成 failed 的问题。
 *
 * 状态流转：idle → connecting → streaming → done/error
 * 组件通过 watch status 来响应状态变化，而非回调。
 */
export const useWebSocketStore = defineStore('websocket', () => {
  const MAX_OUTPUT_LINES = 2000
  const outputLines = shallowRef([])
  const status = ref('idle') // idle, connecting, streaming, done, error
  const lastError = ref('') // 最后一次错误信息，组件 watch 时读取后清空
  const wsRef = ref(null)
  let connectTimer = null
  let lineSeq = 0

  // 批量触发 reactive 更新，避免每条消息都触发一次 Vue diff
  let flushPending = false
  function scheduleFlush() {
    if (flushPending) return
    flushPending = true
    requestAnimationFrame(() => {
      flushPending = false
      triggerRef(outputLines)
    })
  }

  function appendLine(text, stream) {
    const lines = outputLines.value
    if (lines.length >= MAX_OUTPUT_LINES) {
      // 保留最近 1500 行，留出缓冲；整批替换时直接触发
      outputLines.value = lines.slice(-1500).concat({ id: ++lineSeq, text, stream })
    } else {
      lines.push({ id: ++lineSeq, text, stream })
      scheduleFlush()
    }
  }

  function connect(url, previewId, { token } = {}) {
    cleanup()
    outputLines.value = []
    status.value = 'connecting'
    lastError.value = ''

    let ws
    try {
      ws = new WebSocket(url)
    } catch (e) {
      status.value = 'error'
      lastError.value = `连接创建失败: ${e.message}`
      appendLine(`[ERROR] ${lastError.value}`, 'stderr')
      return
    }
    wsRef.value = ws

    // 连接超时：10秒内未建立连接则报错
    connectTimer = setTimeout(() => {
      if (status.value === 'connecting') {
        status.value = 'error'
        lastError.value = '连接超时'
        appendLine('[ERROR] 连接超时', 'stderr')
        if (wsRef.value) {
          wsRef.value.close()
          wsRef.value = null
        }
      }
    }, 10000)

    ws.onopen = () => {
      clearTimeout(connectTimer)
      status.value = 'streaming'
      ws.send(JSON.stringify({ type: 'start', preview_id: previewId, token: token || '' }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        switch (msg.type) {
          case 'output':
            appendLine(msg.data, msg.stream)
            break
          case 'done':
            status.value = 'done'
            break
          case 'error':
            status.value = 'error'
            lastError.value = msg.message || '执行失败'
            appendLine(`[ERROR] ${msg.message}`, 'stderr')
            break
          case 'lock_error':
            status.value = 'error'
            lastError.value = msg.message
            appendLine(`[LOCK] ${msg.message}`, 'stderr')
            break
        }
      } catch (e) {
        console.error('Failed to parse WS message:', e)
      }
    }

    ws.onerror = () => {
      console.error('WebSocket error, current status:', status.value)
    }

    ws.onclose = (event) => {
      clearTimeout(connectTimer)
      console.log(
        'WebSocket closed, code:',
        event.code,
        'reason:',
        event.reason,
        'wasClean:',
        event.wasClean,
        'status:',
        status.value
      )
      if (status.value === 'streaming' || status.value === 'connecting') {
        status.value = 'error'
        if (event.code === 1006) {
          lastError.value = '连接失败，请检查认证状态或服务器是否可达'
        } else if (!event.wasClean) {
          lastError.value = `连接异常断开 (code: ${event.code})`
        } else {
          lastError.value = '连接已断开'
        }
        appendLine(`[ERROR] ${lastError.value}`, 'stderr')
      }
      wsRef.value = null
    }
  }

  function cleanup() {
    clearTimeout(connectTimer)
    if (wsRef.value) {
      wsRef.value.onclose = null
      wsRef.value.close()
      wsRef.value = null
    }
  }

  function disconnect() {
    cleanup()
  }

  /** 重置为 idle 状态（组件 mount 时如果已完成/出错，可调用此方法清理） */
  function reset() {
    cleanup()
    outputLines.value = []
    status.value = 'idle'
    lastError.value = ''
  }

  /** 清空输出行（触发响应式更新） */
  function clearOutput() {
    outputLines.value = []
  }

  /** 恢复缓存的输出行（触发响应式更新） */
  function restoreOutput(lines) {
    // 确保恢复的行也有 id
    outputLines.value = lines.map((l) => (l.id ? l : { ...l, id: ++lineSeq }))
  }

  return {
    outputLines,
    status,
    lastError,
    connect,
    disconnect,
    reset,
    clearOutput,
    restoreOutput,
  }
})
