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

// 响应拦截器：提取 response.data，401 时自动登出并跳转登录页
api.interceptors.response.use(
  (response) => {
    const warning = response.headers['x-warning']
    if (warning) {
      ElMessage({ message: decodeURIComponent(warning), type: 'warning', duration: 5000 })
    }
    return response.data
  },
  (error) => {
    if (error.response?.status === 401) {
      const userStore = useUserStore()
      userStore.logout()
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login').catch(() => {})
      }
    }
    return Promise.reject(error)
  }
)

export default api
