import { describe, it, expect, beforeEach } from 'vitest'
import { useTheme } from './useTheme'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.setAttribute('data-theme', 'dark')
  })

  it('默认深色', () => {
    const { theme } = useTheme()
    expect(theme.value).toBe('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('切换主题并持久化', () => {
    const { theme, toggle } = useTheme()
    toggle()
    expect(theme.value).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    expect(localStorage.getItem('theme')).toBe('light')

    toggle()
    expect(theme.value).toBe('dark')
    expect(localStorage.getItem('theme')).toBe('dark')
  })

  it('从 localStorage 恢复 light 偏好', () => {
    localStorage.setItem('theme', 'light')
    const { theme } = useTheme()
    expect(theme.value).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })
})
