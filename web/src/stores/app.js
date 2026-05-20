import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * 应用级 UI 状态 Store。
 * 管理侧边栏折叠状态，持久化到 localStorage。
 */
export const useAppStore = defineStore('app', () => {
  const isCollapse = ref(localStorage.getItem('sidebarCollapse') === 'true')

  function toggleCollapse() {
    isCollapse.value = !isCollapse.value
    localStorage.setItem('sidebarCollapse', isCollapse.value)
  }

  return {
    isCollapse,
    toggleCollapse
  }
})
