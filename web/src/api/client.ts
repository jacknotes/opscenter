import axios, { AxiosError, type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { getToken, clearSession } from '@/utils/session'

/**
 * 唯一 axios 实例。
 * 后端契约：成功直接返回数据；失败返回 {"error": "..."} + 对应 HTTP 状态码。
 * 401 是会话失效的唯一可靠信号 → 清会话并跳登录页。
 */

const client: AxiosInstance = axios.create({
  baseURL: '/api',
  // dashboard/remote-stats 后端整体超时 60s、连接测试也偏慢，全局放宽
  timeout: 30000,
})

/** 请求附带的内部标记：记录发出请求时用的 token，用于多标签页 token 刷新后重试 */
type RetryConfig = InternalAxiosRequestConfig & { _token?: string; _retried?: boolean }

client.interceptors.request.use((cfg) => {
  const c = cfg as RetryConfig
  const token = getToken()
  if (token) {
    c.headers.Authorization = `Bearer ${token}`
    c._token = token
  }
  // 对齐 v1：GET 加 no-cache，避免代理/浏览器缓存陈旧列表
  if (c.method === 'get') {
    c.headers['Cache-Control'] = 'no-cache'
  }
  return c
})

client.interceptors.response.use(
  (res) => {
    // 契约特例：/lvs/list SSH 失败仍返回 200，警告放在 X-Warning 头（url.PathEscape 编码）
    const warning = res.headers?.['x-warning']
    if (typeof warning === 'string' && warning) {
      try {
        ElMessage.warning({ message: decodeURIComponent(warning), duration: 5000 })
      } catch {
        ElMessage.warning({ message: warning, duration: 5000 })
      }
    }
    return res
  },
  async (err: AxiosError<{ error?: string }>) => {
    const cfg = err.config as RetryConfig | undefined
    if (err.response?.status === 401 && cfg) {
      // 多标签页场景：token 已被其他标签页登录刷新 → 用新 token 重试一次（对齐 v1）
      const fresh = getToken()
      if (fresh && cfg._token && fresh !== cfg._token && !cfg._retried) {
        cfg._retried = true
        cfg.headers.Authorization = `Bearer ${fresh}`
        return client.request(cfg)
      }
      const msg = err.response.data?.error
      clearSession()
      if (!location.pathname.startsWith('/login')) {
        ElMessage.error(msg || '登录已过期，请重新登录')
        // 跳转带回调路径，登录后回跳
        const returnTo = encodeURIComponent(location.pathname + location.search)
        location.href = `/login?redirect=${returnTo}`
      }
    }
    return Promise.reject(err)
  },
)

/** 从错误中提取后端 error 文案（前端统一用这个展示） */
export function extractErrorMessage(err: unknown, fallback = '请求失败'): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { error?: string } | undefined
    if (data?.error) return data.error
    if (err.code === 'ECONNABORTED') return '请求超时，请稍后重试'
    return err.message || fallback
  }
  if (err instanceof Error) return err.message
  return fallback
}

export default client
