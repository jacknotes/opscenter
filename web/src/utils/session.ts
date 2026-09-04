/**
 * 会话生命周期：
 * - 凭据（token/user）存 localStorage —— 同浏览器多标签页共享
 * - 会话 Cookie（opscenter_session，无过期）标记浏览器进程存活
 * - sessionStorage 窗口标记（opscenter_window_mark）标记"来自复制标签页"
 *
 * initSession() 三场景：
 * 1. Cookie 在            → 保持登录
 * 2. Cookie 缺 + 窗口标记在 → 复制标签页竞态，补种 Cookie 保持登录
 * 3. Cookie 缺 + 无标记    → 浏览器重开，清凭据需重新登录
 */

import type { User } from '@/api/types'

const SESSION_COOKIE = 'opscenter_session'
const WINDOW_MARK = 'opscenter_window_mark'
const TOKEN_KEY = 'opscenter_token'
const USER_KEY = 'opscenter_user'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getStoredUser(): User | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as User) : null
  } catch {
    return null
  }
}

export function setStoredUser(user: User): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

function setSessionCookie(): void {
  // 无过期时间的会话 Cookie：浏览器关闭即失效
  document.cookie = `${SESSION_COOKIE}=1; path=/; SameSite=Lax`
}

function hasSessionCookie(): boolean {
  return document.cookie.split(';').some((c) => c.trim().startsWith(`${SESSION_COOKIE}=`))
}

function seedWindowMark(): void {
  try {
    sessionStorage.setItem(WINDOW_MARK, '1')
  } catch {
    /* sessionStorage 不可用时忽略 */
  }
}

function hasWindowMark(): boolean {
  try {
    return sessionStorage.getItem(WINDOW_MARK) === '1'
  } catch {
    return false
  }
}

/** 应用启动时调用，判定会话是否仍然有效 */
export function initSession(): void {
  const loggedIn = Boolean(getToken())
  if (!loggedIn) {
    // 未登录：只负责清理残留
    clearSession()
    return
  }
  if (hasSessionCookie()) {
    // 场景 1：浏览器进程未中断，保持登录
    seedWindowMark()
    return
  }
  if (hasWindowMark()) {
    // 场景 2：复制标签页竞态，补种 Cookie 保持登录
    setSessionCookie()
    return
  }
  // 场景 3：浏览器重开，凭据视为过期
  clearSession()
}

/** 登录成功：写凭据 + 种 Cookie + 种窗口标记 */
export function completeLogin(token: string, user: User): void {
  setToken(token)
  setStoredUser(user)
  setSessionCookie()
  seedWindowMark()
}

/** 清空全部会话凭据 */
export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  try {
    sessionStorage.removeItem(WINDOW_MARK)
  } catch {
    /* 忽略 */
  }
  // 过期 Cookie
  document.cookie = `${SESSION_COOKIE}=; path=/; max-age=0`
}
