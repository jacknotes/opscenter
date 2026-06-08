import api from './client'

export const getPreprodStatus = (serverId) => api.get('/preprod/status', { params: { server_id: serverId } })
export const preprodScaleDownPreview = (data) => api.post('/preprod/scaledown/preview', data)
export const preprodScaleDownExecute = (data) => api.post('/preprod/scaledown/execute', data, { timeout: 600000 })
export const preprodScaleUpPreview = (data) => api.post('/preprod/scaleup/preview', data)
export const preprodScaleUpExecute = (data) => api.post('/preprod/scaleup/execute', data, { timeout: 600000 })
