import axios from 'axios'
import { useUserStore } from '../stores/user'
import router from '../router'
import { ElMessage } from 'element-plus'

// 创建 Axios 实例，所有 API 请求的基础配置
const api = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器：自动注入 JWT token，GET 请求添加缓存破坏参数
api.interceptors.request.use(config => {
  const userStore = useUserStore()
  if (userStore.token) {
    config.headers.Authorization = `Bearer ${userStore.token}`
  }
  // 为GET请求添加cache-busting参数
  if (config.method === 'get') {
    config.params = { ...config.params, _t: Date.now() }
  }
  return config
})

// 响应拦截器：提取 response.data，401 时自动登出并跳转登录页
api.interceptors.response.use(
  response => {
    const warning = response.headers['x-warning']
    if (warning) {
      ElMessage({ message: decodeURIComponent(warning), type: 'warning', duration: 5000 })
    }
    return response.data
  },
  error => {
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

// Auth
export const login = (data) => api.post('/login', data)
export const logout = () => api.post('/logout')
export const getUserInfo = () => api.get('/user/info')

// WebSocket URL helper
export const getWebSocketUrl = (path, token) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const url = `${protocol}//${host}${path}`
  return token ? `${url}?token=${encodeURIComponent(token)}` : url
}

// Servers
export const getServers = (type, all) => api.get('/servers', { params: { type, all: all ? 'true' : undefined } })
export const getServer = (id) => api.get(`/servers/${id}`)
export const getServerForEdit = (id) => api.get(`/servers/${id}/edit`)
export const createServer = (data) => api.post('/servers', data)
export const updateServer = (id, data) => api.put(`/servers/${id}`, data)
export const deleteServer = (id) => api.delete(`/servers/${id}`)
export const testConnection = (id) => api.post(`/servers/${id}/test`)
export const toggleServerEnabled = (id) => api.put(`/servers/${id}/toggle`)

// LVS
export const getLvsList = (serverId) => api.get('/lvs/list', { params: { server_id: serverId } })
export const getLvsStatus = (serverId) => api.get('/lvs/status', { params: { server_id: serverId } })
export const lvsOpPreview = (data) => api.post('/lvs/op/preview', data)
export const lvsOpExecute = (data) => api.post('/lvs/op/execute', data)
export const lvsSwapPreview = (data) => api.post('/lvs/swap/preview', data)
export const lvsSwapExecute = (data) => api.post('/lvs/swap/execute', data)
export const getLvsTags = (params) => api.get('/lvs/tags', { params })
export const updateLvsTag = (data) => api.put('/lvs/tags', data)
export const deleteLvsTag = (vsIp, rsIp) => api.delete(`/lvs/tags/${encodeURIComponent(vsIp)}/${encodeURIComponent(rsIp)}`)
export const getLvsVSTags = (params) => api.get('/lvs/vs_tags', { params })
export const updateLvsVSTag = (data) => api.put('/lvs/vs_tags', data)
export const deleteLvsVSTag = (vsIp) => api.delete(`/lvs/vs_tags/${encodeURIComponent(vsIp)}`)
export const getLvsBindings = (params) => api.get('/lvs/bindings', { params })
export const updateLvsBinding = (data) => api.put('/lvs/bindings', data)
export const deleteLvsBinding = (id) => api.delete(`/lvs/bindings/${id}`)
export const checkLvsForScaleDown = (data) => api.post('/lvs/check/scaledown', data)
export const checkLvsOnlineForPreprod = (data) => api.post('/preprod/check/lvs_online', data)

// K8s
export const getK8sRollouts = (serverId) => api.get('/k8s/rollouts', { params: { server_id: serverId } })
export const k8sOnlinePreview = (data) => api.post('/k8s/online/preview', data)
export const k8sOnlineExecute = (data) => api.post('/k8s/online/execute', data, { timeout: 600000 })
export const k8sSyncPreview = (data) => api.post('/k8s/sync/preview', data)
export const k8sSyncExecute = (data) => api.post('/k8s/sync/execute', data, { timeout: 600000 })
export const k8sRollbackPreview = (data) => api.post('/k8s/rollback/preview', data)
export const k8sRollbackExecute = (data) => api.post('/k8s/rollback/execute', data, { timeout: 600000 })
export const k8sFullOnlinePreview = (data) => api.post('/k8s/full_online/preview', data)
export const k8sFullOnlineExecute = (data) => api.post('/k8s/full_online/execute', data, { timeout: 600000 })
export const k8sFullSyncPreview = (data) => api.post('/k8s/full_sync/preview', data)
export const k8sFullSyncExecute = (data) => api.post('/k8s/full_sync/execute', data, { timeout: 600000 })
export const k8sFullRollbackPreview = (data) => api.post('/k8s/full_rollback/preview', data)
export const k8sFullRollbackExecute = (data) => api.post('/k8s/full_rollback/execute', data, { timeout: 600000 })

// Preprod
export const getPreprodStatus = (serverId) => api.get('/preprod/status', { params: { server_id: serverId } })
export const preprodScaleDownPreview = (data) => api.post('/preprod/scaledown/preview', data)
export const preprodScaleDownExecute = (data) => api.post('/preprod/scaledown/execute', data, { timeout: 600000 })
export const preprodScaleUpPreview = (data) => api.post('/preprod/scaleup/preview', data)
export const preprodScaleUpExecute = (data) => api.post('/preprod/scaleup/execute', data, { timeout: 600000 })

// Nginx
export const getNginxConfigs = (serverId) => api.get('/nginx/configs', { params: { server_id: serverId } })
export const getNginxUpstreams = (serverId, configFile) => api.get('/nginx/upstreams', { params: { server_id: serverId, config_file: configFile } })
export const nginxOnlinePreview = (data) => api.post('/nginx/upstream/online/preview', data)
export const nginxOnlineExecute = (data) => api.post('/nginx/upstream/online/execute', data)
export const nginxOfflinePreview = (data) => api.post('/nginx/upstream/offline/preview', data)
export const nginxOfflineExecute = (data) => api.post('/nginx/upstream/offline/execute', data)
export const nginxSwapPreview = (data) => api.post('/nginx/upstream/swap/preview', data)
export const nginxSwapExecute = (data) => api.post('/nginx/upstream/swap/execute', data)
export const nginxTogglePreview = (data) => api.post('/nginx/upstream/toggle/preview', data)
export const nginxToggleExecute = (data) => api.post('/nginx/upstream/toggle/execute', data)
export const nginxBatchPreview = (data) => api.post('/nginx/upstream/batch/preview', data)
export const nginxBatchExecute = (data) => api.post('/nginx/upstream/batch/execute', data)
export const nginxRollbackPreview = (data) => api.post('/nginx/rollback/preview', data)
export const nginxRollbackExecute = (data) => api.post('/nginx/rollback/execute', data)
export const getNginxBackups = (serverId, opts) => api.get('/nginx/backups', { params: { server_id: serverId }, ...opts })

// Logs
export const getLogs = (params) => api.get('/logs', { params })

// Dashboard
export const getDashboardStats = () => api.get('/dashboard/stats')
export const getDashboardRemoteStats = () => api.get('/dashboard/remote-stats', { timeout: 35000 })

// Users
export const getUsers = () => api.get('/users')
export const createUser = (data) => api.post('/users', data)
export const updateUser = (id, data) => api.put(`/users/${id}`, data)
export const deleteUser = (id) => api.delete(`/users/${id}`)
export const resetPassword = (id, data) => api.put(`/users/${id}/reset-password`, data)
export const changePassword = (id, data) => api.put(`/users/${id}/password`, data)
export const toggleUserEnabled = (id) => api.put(`/users/${id}/toggle`)

export default api
