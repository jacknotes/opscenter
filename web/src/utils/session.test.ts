import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  getToken,
  getStoredUser,
  completeLogin,
  clearSession,
  initSession,
} from './session'
import type { User } from '@/api/types'

const mockUser: User = {
  id: 1,
  username: 'admin',
  name: '管理员',
  email: 'admin@example.com',
  role: 'admin',
  enabled: true,
  auth_source: 'local',
  failed_attempts: 0,
  locked: false,
  created_at: '2026-01-01T00:00:00+08:00',
  updated_at: '2026-01-01T00:00:00+08:00',
}

function seedLogin() {
  completeLogin('tok-123', mockUser)
}

describe('utils/session', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    document.cookie = 'opscenter_session=; path=/; max-age=0'
  })

  it('completeLogin 写入 token/user/Cookie/窗口标记', () => {
    seedLogin()
    expect(getToken()).toBe('tok-123')
    expect(getStoredUser()?.username).toBe('admin')
    expect(document.cookie).toContain('opscenter_session=1')
    expect(sessionStorage.getItem('opscenter_window_mark')).toBe('1')
  })

  it('clearSession 清空全部凭据', () => {
    seedLogin()
    clearSession()
    expect(getToken()).toBe('')
    expect(getStoredUser()).toBeNull()
    expect(document.cookie).not.toContain('opscenter_session=1')
    expect(sessionStorage.getItem('opscenter_window_mark')).toBeNull()
  })

  it('场景 1：Cookie 存活 → 保持登录并补种窗口标记', () => {
    seedLogin()
    // 模拟"没有窗口标记"（例如复制前页面已设置 Cookie 但 sessionStorage 空的场景不存在，此处验证 Cookie 判据优先）
    sessionStorage.clear()
    initSession()
    expect(getToken()).toBe('tok-123')
    expect(sessionStorage.getItem('opscenter_window_mark')).toBe('1')
  })

  it('场景 2：Cookie 缺失 + 窗口标记在 → 复制标签页竞态，补种 Cookie 保持登录', () => {
    seedLogin()
    // 模拟复制标签页：凭据共享，但 Cookie 未带走、窗口标记在
    document.cookie = 'opscenter_session=; path=/; max-age=0'
    initSession()
    expect(getToken()).toBe('tok-123')
    expect(document.cookie).toContain('opscenter_session=1')
  })

  it('场景 3：Cookie 缺失 + 无窗口标记 → 浏览器重开，清凭据', () => {
    seedLogin()
    document.cookie = 'opscenter_session=; path=/; max-age=0'
    sessionStorage.clear()
    initSession()
    expect(getToken()).toBe('')
    expect(getStoredUser()).toBeNull()
  })

  it('无 token 时 initSession 只做清理', () => {
    initSession()
    expect(getToken()).toBe('')
    expect(getStoredUser()).toBeNull()
  })

  it('getStoredUser 对损坏 JSON 返回 null', () => {
    localStorage.setItem('opscenter_user', '{bad json')
    expect(getStoredUser()).toBeNull()
  })

  it('会话 Cookie 独立于 localStorage（重开浏览器后过期）', () => {
    // 验证 clearSession 的 Cookie 过期写入不抛错且生效
    seedLogin()
    clearSession()
    const spy = vi.spyOn(document, 'cookie', 'get')
    expect(document.cookie).not.toContain('opscenter_session=1')
    spy.mockRestore()
  })
})
