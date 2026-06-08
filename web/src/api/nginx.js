import api from './client'

export const getNginxConfigs = (serverId) => api.get('/nginx/configs', { params: { server_id: serverId } })
export const getNginxUpstreams = (serverId, configFile) =>
  api.get('/nginx/upstreams', { params: { server_id: serverId, config_file: configFile } })
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
export const getNginxBackups = (serverId, opts) =>
  api.get('/nginx/backups', { params: { server_id: serverId }, ...opts })
