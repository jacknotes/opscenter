import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * 应用级 UI 状态 Store。
 * 管理侧边栏折叠状态和主题模式，持久化到 localStorage。
 */
export const useAppStore = defineStore('app', () => {
  const isCollapse = ref(localStorage.getItem('sidebarCollapse') === 'true')
  const theme = ref(localStorage.getItem('theme') || 'dark')

  function toggleCollapse() {
    isCollapse.value = !isCollapse.value
    localStorage.setItem('sidebarCollapse', isCollapse.value)
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem('theme', theme.value)
    applyTheme()
  }

  function applyTheme() {
    if (theme.value === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  return {
    isCollapse,
    toggleCollapse,
    theme,
    toggleTheme,
    applyTheme
  }
})
