import { ref } from 'vue'

export type Theme = 'dark' | 'light'

const THEME_KEY = 'theme'

const current = ref<Theme>('dark')

function apply(theme: Theme): void {
  current.value = theme
  document.documentElement.setAttribute('data-theme', theme)
  try {
    localStorage.setItem(THEME_KEY, theme)
  } catch {
    /* 忽略持久化失败 */
  }
}

/** 日夜主题单例：默认深色，读写 localStorage.theme，设 <html data-theme> */
export function useTheme() {
  // 模块加载时同步应用一次（index.html 内联脚本已先应用，这里双保险）
  let stored: string | null = null
  try {
    stored = localStorage.getItem(THEME_KEY)
  } catch {
    /* 忽略 */
  }
  current.value = stored === 'light' ? 'light' : 'dark'
  document.documentElement.setAttribute('data-theme', current.value)

  function toggle(): void {
    apply(current.value === 'dark' ? 'light' : 'dark')
  }

  return { theme: current, toggle }
}
