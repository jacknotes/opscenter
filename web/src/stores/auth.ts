import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { authApi } from '@/api'
import type { User } from '@/api/types'
import { completeLogin, clearSession, getToken, getStoredUser } from '@/utils/session'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(getToken())
  const user = ref<User | null>(getStoredUser())

  const isLoggedIn = computed(() => Boolean(token.value))
  const isAdmin = computed(() => user.value?.role === 'admin')
  const displayName = computed(() => user.value?.name || user.value?.username || '')

  async function login(username: string, password: string): Promise<void> {
    const res = await authApi.login({ username, password })
    completeLogin(res.token, res.user)
    token.value = res.token
    user.value = res.user
  }

  async function logout(): Promise<void> {
    // 后端将 token 加入黑名单；无论请求成败，本地一律清空
    try {
      await authApi.logout()
    } catch {
      /* 忽略 */
    }
    clearSession()
    token.value = ''
    user.value = null
  }

  /** 拉取最新用户信息（角色/启用状态可能被管理员变更） */
  async function refreshUser(): Promise<void> {
    const info = await authApi.getUserInfo()
    user.value = info
  }

  return { token, user, isLoggedIn, isAdmin, displayName, login, logout, refreshUser }
})
