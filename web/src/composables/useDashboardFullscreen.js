import { ref, nextTick } from 'vue'

/**
 * 仪表盘图表全屏管理
 * - 管理全屏状态切换
 * - ESC 按键退出全屏
 * - 退出全屏后自动滚动到图表位置
 */
export function useDashboardFullscreen() {
  const fullscreenChart = ref(null)

  async function toggleFullscreen(chartName) {
    const wasFullscreen = fullscreenChart.value
    if (fullscreenChart.value === chartName) {
      fullscreenChart.value = null
    } else {
      fullscreenChart.value = chartName
    }
    // 退出全屏时，等待 Vue 重新渲染后再滚动到图表位置
    if (wasFullscreen && !fullscreenChart.value) {
      await nextTick()
      scrollToChart(wasFullscreen)
    }
  }

  function getFullscreenCardStyle(chartName) {
    if (fullscreenChart.value === chartName) {
      return { flex: '1', minHeight: '0', display: 'flex', flexDirection: 'column' }
    }
    return {}
  }

  function scrollToChart(chartName) {
    const el = document.querySelector(`[data-chart="${chartName}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }

  function handleKeydown(e) {
    if (e.key !== 'Escape') return
    // 输入框内不触发
    const tag = e.target.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target.isContentEditable) return
    if (!fullscreenChart.value) return
    toggleFullscreen(fullscreenChart.value) // async, fire-and-forget is fine for event handler
  }

  // 注册 ESC 监听
  document.addEventListener('keydown', handleKeydown)

  function cleanup() {
    document.removeEventListener('keydown', handleKeydown)
  }

  return {
    fullscreenChart,
    toggleFullscreen,
    getFullscreenCardStyle,
    cleanup,
  }
}
