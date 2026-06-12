import api from './client'

export const getUsers = () => api.get('/users')
export const createUser = (data) => api.post('/users', data)
export const updateUser = (id, data) => api.put(`/users/${id}`, data)
export const deleteUser = (id) => api.delete(`/users/${id}`)
export const batchDeleteUsers = (ids) => api.post('/users/batch-delete', { ids })
export const batchToggleUsers = (ids, enabled) => api.post('/users/batch-toggle', { ids, enabled })
export const resetPassword = (id, data) => api.put(`/users/${id}/reset-password`, data)
export const changePassword = (id, data) => api.put(`/users/${id}/password`, data)
export const toggleUserEnabled = (id) => api.put(`/users/${id}/toggle`)
// LDAP Users
export const getLdapUsers = () => api.get('/users/ldap')
export const importLdapUsers = (users) => api.post('/users/ldap/import', { users })
export const unlockUser = (id) => api.put(`/users/${id}/unlock`)
export const kickUser = (id) => api.post(`/users/${id}/kick`)
export const batchUnlockUsers = (ids) => api.post('/users/batch-unlock', { ids })
export const batchKickUsers = (ids) => api.post('/users/batch-kick', { ids })
