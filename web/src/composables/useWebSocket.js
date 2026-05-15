import { ref } from 'vue'

export function useWebSocket() {
  const outputLines = ref([])
  const status = ref('idle') // idle, connecting, streaming, done, error
  const wsRef = ref(null)
  let connectTimer = null

  function connect(url, previewId, { onDone, onError, onLockError } = {}) {
    cleanup()
    outputLines.value = []
    status.value = 'connecting'

    let ws
    try {
      ws = new WebSocket(url)
    } catch (e) {
      status.value = 'error'
      outputLines.value.push({ text: `[ERROR] 连接创建失败: ${e.message}`, stream: 'stderr' })
      onError?.('连接创建失败')
      return
    }
    wsRef.value = ws

    // 连接超时：10秒内未建立连接则报错
    connectTimer = setTimeout(() => {
      if (status.value === 'connecting') {
        status.value = 'error'
        outputLines.value.push({ text: '[ERROR] 连接超时', stream: 'stderr' })
        onError?.('连接超时')
        if (wsRef.value) {
          wsRef.value.close()
          wsRef.value = null
        }
      }
    }, 10000)

    ws.onopen = () => {
      clearTimeout(connectTimer)
      status.value = 'streaming'
      ws.send(JSON.stringify({ type: 'start', preview_id: previewId }))
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        switch (msg.type) {
          case 'output':
            outputLines.value.push({ text: msg.data, stream: msg.stream })
            break
          case 'done':
            status.value = 'done'
            onDone?.()
            break
          case 'error':
            status.value = 'error'
            outputLines.value.push({ text: `[ERROR] ${msg.message}`, stream: 'stderr' })
            onError?.(msg.message)
            break
          case 'lock_error':
            status.value = 'error'
            outputLines.value.push({ text: `[LOCK] ${msg.message}`, stream: 'stderr' })
            onLockError?.(msg.message, msg.holder)
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
      console.log('WebSocket closed, code:', event.code, 'reason:', event.reason, 'wasClean:', event.wasClean, 'status:', status.value)
      if (status.value === 'streaming' || status.value === 'connecting') {
        status.value = 'error'
        if (event.code === 1006) {
          outputLines.value.push({ text: '[ERROR] 连接失败，请检查认证状态或服务器是否可达', stream: 'stderr' })
          onError?.('连接失败，请检查认证状态或服务器是否可达')
        } else if (!event.wasClean) {
          outputLines.value.push({ text: `[ERROR] 连接异常断开 (code: ${event.code})`, stream: 'stderr' })
          onError?.(`连接异常断开 (code: ${event.code})`)
        } else {
          outputLines.value.push({ text: '[ERROR] 连接已断开', stream: 'stderr' })
          onError?.('连接已断开')
        }
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

  return {
    outputLines,
    status,
    connect,
    disconnect,
  }
}
