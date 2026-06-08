import api from './client'

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
