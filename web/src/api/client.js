import axios from 'axios'
import { useUserStore } from '../stores/user'
import router from '../router'
import { ElMessage } from 'element-plus'

// 创建 Axios 实例，所有 API 请求的基础配置
const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 请求拦截器：自动注入 JWT token，GET 请求禁止缓存
api.interceptors.request.use((config) => {
  const userStore = useUserStore()
  if (userStore.token) {
    config.headers.Authorization = `Bearer ${userStore.token}`
  }
  // 使用 Cache-Control 头替代 URL 参数破坏缓存，更符合 HTTP 规范
  if (config.method === 'get') {
    config.headers['Cache-Control'] = 'no-cache'
    config.headers['Pragma'] = 'no-cache'
  }
  return config
})

// 响应拦截器：提取 response.data，401 时智能处理多标签页登录协同
let loginExpiredShown = false
api.interceptors.response.use(
  (response) => {
    const warning = response.headers['x-warning']
    if (warning) {
      ElMessage({ message: decodeURIComponent(warning), type: 'warning', duration: 5000 })
    }
    return response.data
  },
  async (error) => {
    if (error.response?.status === 401) {
      const userStore = useUserStore()
      const currentToken = userStore.token
      const storedToken = localStorage.getItem('token') || ''

      // 多标签页协同：若 localStorage 中的 token 与本页内存 token 不同，
      // 说明其它标签页已重新登录并写入了新 token。此时本页的 401 是旧 token 导致，
      // 应同步新 token 并重试原请求，而非清空共享 localStorage（否则会级联登出其它标签页）。
      if (
        currentToken &&
        storedToken &&
        currentToken !== storedToken &&
        !error.config._retried &&
        !error.config.url?.includes('/login')
      ) {
        error.config._retried = true
        // 同步新 token 到内存（storage 事件通常也会触发，这里确保立即生效）
        userStore.token = storedToken
        error.config.headers.Authorization = `Bearer ${storedToken}`
        return api.request(error.config)
      }

      // 本页 token 确实失效（与 localStorage 一致，或 localStorage 已空），执行登出
      userStore.logout()
      if (!loginExpiredShown && router.currentRoute.value.path !== '/login') {
        loginExpiredShown = true
        ElMessage.warning('当前登录已失效，请重新登录')
        router.push('/login').catch(() => {}).finally(() => {
          setTimeout(() => { loginExpiredShown = false }, 1000)
        })
      }
    }
    return Promise.reject(error)
  }
)

export default api
