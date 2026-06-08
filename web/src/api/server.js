import api from './client'

export const getServers = (type, all) => api.get('/servers', { params: { type, all: all ? 'true' : undefined } })
export const getServer = (id) => api.get(`/servers/${id}`)
export const getServerForEdit = (id) => api.get(`/servers/${id}/edit`)
export const createServer = (data) => api.post('/servers', data)
export const updateServer = (id, data) => api.put(`/servers/${id}`, data)
export const deleteServer = (id) => api.delete(`/servers/${id}`)
export const testConnection = (id) => api.post(`/servers/${id}/test`)
export const toggleServerEnabled = (id) => api.put(`/servers/${id}/toggle`)
export const batchDeleteServers = (ids) => api.post('/servers/batch-delete', { ids })
export const batchToggleServers = (ids, enabled) => api.post('/servers/batch-toggle', { ids, enabled })
export const batchTestServers = (ids) => api.post('/servers/batch-test', { ids })
