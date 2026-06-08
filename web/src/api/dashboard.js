import api from './client'

export const getDashboardStats = () => api.get('/dashboard/stats')
export const getDashboardRemoteStats = () => api.get('/dashboard/remote-stats', { timeout: 35000 })
export const getActivityStats = (params) => api.get('/dashboard/activity-stats', { params })
export const getK8sProjectStats = (params) => api.get('/dashboard/k8s-project-stats', { params })
export const getPreprodProjectStats = (params) => api.get('/dashboard/preprod-project-stats', { params })
