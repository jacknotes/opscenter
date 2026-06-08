/**
 * el-table 水平滚动修复指令
 * 使用 CSS contain 替代 JS 强制回流，避免 layout thrashing
 */
export const forceReflow = {
  mounted(el) {
    const scrollEl = el.querySelector('.el-table__inner-wrapper') || el
    // 使用 CSS contain 创建独立的布局边界，避免全局回流
    scrollEl.style.contain = 'layout'
    scrollEl.style.overflowX = 'hidden'
    // 使用 requestAnimationFrame 确保在下一帧恢复，而非强制同步回流
    requestAnimationFrame(() => {
      scrollEl.style.overflowX = ''
    })
  },
}
