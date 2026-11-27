import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import router from '../router'

/**
 * 浏览器会话标记 cookie 名。
 * 设置为会话级 cookie（无 Expires/Max-Age），浏览器关闭即清除。
 * 用于在应用启动时检测"浏览器重开后遗留的 token"：
 * 若 localStorage 有 token 但无此 cookie，说明浏览器曾被关闭，应强制重新登录。
 * 多标签页共享同一 cookie，不影响多标签页协同登录。
 */
const SESSION_COOKIE = 'opscenter_session'

function setSessionCookie() {
  document.cookie = `${SESSION_COOKIE}=1; path=/; SameSite=Strict`
}

function clearSessionCookie() {
  document.cookie = `${SESSION_COOKIE}=; path=/; max-age=0; SameSite=Strict`
}

function hasSessionCookie() {
  return document.cookie.split(';').some((c) => c.trim().startsWith(`${SESSION_COOKIE}=`))
}

/**
 * 应用启动时校验会话：若 localStorage 残留 token 但浏览器会话标记已丢失
 * （说明浏览器曾被关闭/重启），则清除 token 强制重新登录。
 * 应在应用初始化最早处、路由守卫执行前调用。
 */
export function validateSession() {
  const hasToken = !!localStorage.getItem('token')
  if (hasToken && !hasSessionCookie()) {
    localStorage.removeItem('token')
    localStorage.removeItem('role')
  }
}

/**
 * 用户状态管理 Store。
 * 管理 JWT token、用户信息、登录状态和管理员权限判断。
 * token 和 role 同步持久化到 localStorage，并设置浏览器会话级 cookie 标记。
 *
 * 多标签页协同：
 * - 所有标签页共享 localStorage 中的 token，任一标签页登录后其它标签页通过
 *   storage 事件自动同步内存中的 token，无需各自重新登录。
 * - 关闭浏览器后会话 cookie 清失，重启后 validateSession 清除残留 token，
 *   强制重新登录，避免长期免登录的安全风险。
 */
export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const userInfo = ref(null)

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => userInfo.value?.role === 'admin')

  function setToken(newToken) {
    token.value = newToken
    localStorage.setItem('token', newToken)
    setSessionCookie()
  }

  function setUserInfo(info) {
    userInfo.value = info
    if (info?.role) {
      localStorage.setItem('role', info.role)
    }
  }

  function logout() {
    token.value = ''
    userInfo.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('role')
    clearSessionCookie()
  }

  // 跨标签页同步：监听 localStorage 的 storage 事件，当其它标签页更新或清除
  // token/role 时，同步本标签页内存中的状态，避免使用过期 token 触发 401 级联登出。
  if (typeof window !== 'undefined') {
    window.addEventListener('storage', (e) => {
      if (e.key === 'token') {
        token.value = e.newValue || ''
        if (!token.value) {
          userInfo.value = null
        }
      } else if (e.key === 'role') {
        // role 变化不直接重建 userInfo（缺少完整用户信息），仅保持 role 持久化一致
      }
    })
  }

  // 跨标签页登出协同：任一标签页登出/登录失效后，其它标签页的内存 token
  // 通过 storage 事件被清空，此处监听登录状态，自动跳转到登录页，
  // 避免其它标签页停留在"菜单退化但页面不跳转"的中间状态。
  // 注意：此处只跳转不弹提示——"登录已失效"提示由拦截器在真正收到 401 时
  // 统一弹出一次，避免同一标签页双重提示、或手动退出时误报"失效"。
  if (typeof window !== 'undefined') {
    watch(isLoggedIn, (loggedIn, wasLoggedIn) => {
      if (wasLoggedIn && !loggedIn && router.currentRoute.value.path !== '/login') {
        router.push('/login').catch(() => {})
      }
    })
  }

  return {
    token,
    userInfo,
    isLoggedIn,
    isAdmin,
    setToken,
    setUserInfo,
    logout,
  }
})
