import type { Directive } from 'vue'

/**
 * el-table 水平滚动修复指令（对齐 v1 v-force-reflow）
 * 用 CSS contain 创建独立布局边界，避免数据更新时水平滚动条闪烁导致的回流抖动
 */
export const forceReflow: Directive<HTMLElement> = {
  mounted(el) {
    const scrollEl = el.querySelector<HTMLElement>('.el-table__inner-wrapper') ?? el
    scrollEl.style.contain = 'layout'
    scrollEl.style.overflowX = 'hidden'
    // 下一帧恢复，避免强制同步回流
    requestAnimationFrame(() => {
      scrollEl.style.overflowX = ''
    })
  },
}
