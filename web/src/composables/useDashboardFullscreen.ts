import { ref, nextTick, onScopeDispose } from 'vue'

/**
 * 仪表盘图表全屏管理
 * - 管理全屏状态切换
 * - ESC 按键退出全屏（输入框内不触发）
 * - 退出全屏后自动滚动到图表位置
 */
export function useDashboardFullscreen() {
  const fullscreenChart = ref<string | null>(null)

  async function toggleFullscreen(chartName: string) {
    const wasFullscreen = fullscreenChart.value
    fullscreenChart.value = fullscreenChart.value === chartName ? null : chartName
    // 退出全屏时，等待 Vue 重新渲染后再滚动到图表位置
    if (wasFullscreen && !fullscreenChart.value) {
      await nextTick()
      scrollToChart(wasFullscreen)
    }
  }

  function getFullscreenCardStyle(chartName: string): Record<string, string> {
    if (fullscreenChart.value === chartName) {
      return { flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' }
    }
    return {}
  }

  function scrollToChart(chartName: string) {
    const el = document.querySelector(`[data-chart="${chartName}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    // 输入框内不触发
    const target = e.target as HTMLElement | null
    const tag = target?.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || target?.isContentEditable) return
    if (!fullscreenChart.value) return
    void toggleFullscreen(fullscreenChart.value)
  }

  // 注册 ESC 监听，组件卸载时自动清理
  document.addEventListener('keydown', handleKeydown)
  onScopeDispose(() => {
    document.removeEventListener('keydown', handleKeydown)
  })

  return {
    fullscreenChart,
    toggleFullscreen,
    getFullscreenCardStyle,
  }
}
