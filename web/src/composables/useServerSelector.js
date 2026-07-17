import { ref, onMounted } from 'vue'
import { getServers } from '../api'
import { ElMessage } from 'element-plus'
import { showLoadError } from '../utils/message'

/**
 * 服务器选择器组合式函数
 * @param {string} serverType - 服务器类型 (lvs/nginx/kubernetes/preprod)
 * @param {string} storageKey - localStorage 持久化键名
 * @param {Function} onChange - 服务器切换后的回调 (可选，返回 Promise)
 * @returns {{ servers, serverId, loading, initServers }}
 */
// 模块级缓存：避免短时间内重复请求服务器列表
const fetchCache = new Map() // serverType -> { data, ts }
const CACHE_TTL = 30_000 // 30 秒

/**
 * 清除指定类型或全部的服务器列表缓存。
 * 在 ServerManage 等页面修改服务器后调用，确保其他页面能获取最新列表。
 */
export function clearServerCache(serverType) {
  if (serverType) {
    fetchCache.delete(serverType)
  } else {
    fetchCache.clear()
  }
}

export function useServerSelector(serverType, storageKey, onChange) {
  const servers = ref([])
  const serverId = ref(null)
  const loading = ref(false)

  async function fetchServersCached() {
    const cached = fetchCache.get(serverType)
    if (cached && Date.now() - cached.ts < CACHE_TTL) {
      return cached.data
    }
    const data = (await getServers(serverType)) || []
    fetchCache.set(serverType, { data, ts: Date.now() })
    return data
  }

  async function initServers() {
    loading.value = true
    try {
      servers.value = await fetchServersCached()
      if (servers.value.length > 0) {
        const saved = localStorage.getItem(storageKey)
        if (saved && servers.value.some((s) => s.id === Number(saved))) {
          serverId.value = Number(saved)
        } else {
          serverId.value = servers.value[0].id
        }
        if (onChange) await onChange()
      }
    } catch (e) {
      showLoadError(e, '加载服务器列表失败')
    } finally {
      loading.value = false
    }
  }

  /** 保存当前选择到 localStorage */
  function saveSelection() {
    if (serverId.value != null) {
      localStorage.setItem(storageKey, serverId.value)
    }
  }

  /** 服务器切换处理（供 @change 使用） */
  async function handleServerChange() {
    saveSelection()
    if (onChange) await onChange()
  }

  /**
   * 刷新服务器列表（保留当前选择），供 keep-alive 页面在 onActivated 时调用。
   * 与 initServers 不同，此函数不会自动调用 onChange 回调。
   * 内置 30 秒缓存，短时间内切换页面不会重复请求。
   */
  async function refreshServers() {
    try {
      const list = await fetchServersCached()
      servers.value = list
      // 当前选择的服务器已不存在时，回退到第一个
      if (serverId.value && !list.some((s) => s.id === serverId.value)) {
        serverId.value = list.length > 0 ? list[0].id : null
      }
    } catch {
      // 刷新失败不影响已有列表
    }
  }

  return { servers, serverId, loading, initServers, refreshServers, saveSelection, handleServerChange }
}
