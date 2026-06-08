import api from './client'

export const getLogs = (params) => api.get('/logs', { params })
