import { ref } from 'vue'

/**
 * 服务器选择 + 列表加载。
 * 泛型 T 为列表项类型；loader 传入具体 API 调用（接收 { type, all } 之外的参数时自行闭包）。
 */
export function useServerSelector<T extends { id: number }>() {
  const servers = ref<T[]>([])
  const serverId = ref<number | undefined>(undefined)
  const loading = ref(false)

  async function load(loader: (params: Record<string, unknown>) => Promise<T[]>): Promise<void> {
    loading.value = true
    try {
      servers.value = await loader({})
      // 当前选中项不在列表中时清空
      if (serverId.value !== undefined && !servers.value.some((s) => s.id === serverId.value)) {
        serverId.value = undefined
      }
    } finally {
      loading.value = false
    }
  }

  return { servers, serverId, loading, load }
}
