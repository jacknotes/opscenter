import { ref } from 'vue'
import { defineStore } from 'pinia'

/**
 * 全局执行状态摘要（预生产 WebSocket 流式执行）。
 *
 * 输出行与 WS 连接保留在 PreprodScale 页面组件内 —— AppLayout 的 keep-alive
 * 保证切页后组件不销毁、连接不断、输出继续累积；本 store 只承载跨页可见的
 * 状态摘要（状态 / 服务器 / 输出行数），供 AppLayout 底部执行状态栏展示。
 */
export const useExecStreamStore = defineStore('execStream', () => {
  const state = ref<'idle' | 'running' | 'done' | 'failed'>('idle')
  const serverId = ref<number | null>(null)
  const lineCount = ref(0)

  /** 开始一次流式执行 */
  function begin(sid: number): void {
    serverId.value = sid
    state.value = 'running'
    lineCount.value = 0
  }

  /** 页面推送输出行数（底栏展示进度用） */
  function report(lines: number): void {
    lineCount.value = lines
  }

  function finish(next: 'done' | 'failed'): void {
    state.value = next
  }

  /** 清空会话（回到 idle；登出或页面真正销毁时调用） */
  function clear(): void {
    state.value = 'idle'
    serverId.value = null
    lineCount.value = 0
  }

  return { state, serverId, lineCount, begin, report, finish, clear }
})
