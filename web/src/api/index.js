import axios from 'axios'
import { useUserStore } from '../stores/user'
import router from '../router'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000
})

api.interceptors.request.use(config => {
  const userStore = useUserStore()
  if (userStore.token) {
    config.headers.Authorization = `Bearer ${userStore.token}`
  }
  return config
})

api.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response?.status === 401) {
      const userStore = useUserStore()
      userStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

// Auth
export const login = (data) => api.post('/login', data)
export const getUserInfo = () => api.get('/user/info')

// Servers
export const getServers = (type) => api.get('/servers', { params: { type } })
export const getServer = (id) => api.get(`/servers/${id}`)
export const getServerForEdit = (id) => api.get(`/servers/${id}/edit`)
export const createServer = (data) => api.post('/servers', data)
export const updateServer = (id, data) => api.put(`/servers/${id}`, data)
export const deleteServer = (id) => api.delete(`/servers/${id}`)
export const testConnection = (id) => api.post(`/servers/${id}/test`)

// LVS
export const getLvsList = (serverId) => api.get('/lvs/list', { params: { server_id: serverId } })
export const getLvsStatus = (serverId) => api.get('/lvs/status', { params: { server_id: serverId } })
export const lvsOpPreview = (data) => api.post('/lvs/op/preview', data)
export const lvsOpExecute = (data) => api.post('/lvs/op/execute', data)
export const lvsSwapPreview = (data) => api.post('/lvs/swap/preview', data)
export const lvsSwapExecute = (data) => api.post('/lvs/swap/execute', data)

// K8s
export const getK8sRollouts = (serverId) => api.get('/k8s/rollouts', { params: { server_id: serverId } })
export const k8sOnlinePreview = (data) => api.post('/k8s/online/preview', data)
export const k8sOnlineExecute = (data) => api.post('/k8s/online/execute', data)
export const k8sSyncPreview = (data) => api.post('/k8s/sync/preview', data)
export const k8sSyncExecute = (data) => api.post('/k8s/sync/execute', data)
export const k8sRollbackPreview = (data) => api.post('/k8s/rollback/preview', data)
export const k8sRollbackExecute = (data) => api.post('/k8s/rollback/execute', data)
export const k8sFullOnlinePreview = (data) => api.post('/k8s/full_online/preview', data)
export const k8sFullOnlineExecute = (data) => api.post('/k8s/full_online/execute', data)
export const k8sFullSyncPreview = (data) => api.post('/k8s/full_sync/preview', data)
export const k8sFullSyncExecute = (data) => api.post('/k8s/full_sync/execute', data)
export const k8sFullRollbackPreview = (data) => api.post('/k8s/full_rollback/preview', data)
export const k8sFullRollbackExecute = (data) => api.post('/k8s/full_rollback/execute', data)

// Preprod
export const getPreprodStatus = (serverId) => api.get('/preprod/status', { params: { server_id: serverId } })
export const preprodScaleDownPreview = (data) => api.post('/preprod/scaledown/preview', data)
export const preprodScaleDownExecute = (data) => api.post('/preprod/scaledown/execute', data)
export const preprodScaleUpPreview = (data) => api.post('/preprod/scaleup/preview', data)
export const preprodScaleUpExecute = (data) => api.post('/preprod/scaleup/execute', data)

// Nginx
export const getNginxConfigs = (serverId) => api.get('/nginx/configs', { params: { server_id: serverId } })
export const getNginxUpstreams = (serverId, configFile) => api.get('/nginx/upstreams', { params: { server_id: serverId, config_file: configFile } })
export const nginxOnlinePreview = (data) => api.post('/nginx/upstream/online/preview', data)
export const nginxOnlineExecute = (data) => api.post('/nginx/upstream/online/execute', data)
export const nginxOfflinePreview = (data) => api.post('/nginx/upstream/offline/preview', data)
export const nginxOfflineExecute = (data) => api.post('/nginx/upstream/offline/execute', data)
export const nginxReload = (data) => api.post('/nginx/reload', data)
export const nginxRollbackPreview = (data) => api.post('/nginx/rollback/preview', data)
export const nginxRollbackExecute = (data) => api.post('/nginx/rollback/execute', data)
export const getNginxBackups = (serverId) => api.get('/nginx/backups', { params: { server_id: serverId } })

// Logs
export const getLogs = (params) => api.get('/logs', { params })

export default api
