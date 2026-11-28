/**
 * v-select-fit: el-select 宽度随选中项文本自适应。
 *
 * 背景：Element Plus 2.x 将 .el-select__selection 设为 flex: 1 1 0%，
 * 内部元素对固有宽度的贡献为 0，导致外层 width: fit-content / max-content
 * 均塌缩为 0（实测），只能通过测量文本宽度显式设定宽度。
 *
 * 用法：<el-select v-select-fit ... />
 * 行为：宽度 = 选中项文本宽度 + 边框/内边距/箭头等装饰宽度，
 *       下限 150px；配合 max-width:100% 防溢出（见 global.css）。
 * 时机：mounted / updated（选中值变化触发组件重渲染）+ 字体加载完成后兜底重测。
 */
const canvas = typeof document !== 'undefined' ? document.createElement('canvas') : null
const ctx = canvas ? canvas.getContext('2d') : null
const MIN_WIDTH = 150

function measureTextWidth(text, style) {
  if (!ctx) return text.length * 12
  ctx.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`
  return Math.ceil(ctx.measureText(text).width)
}

function findLabelEl(el) {
  // EP 2.13+：选中文本/占位符在 .el-select__placeholder > span（结构实测）；
  // 兼容旧版 select 基于 .el-input__inner 的结构。
  return (
    el.querySelector('.el-select__placeholder span') ||
    el.querySelector('.el-select__selected-item span') ||
    el.querySelector('.el-input__inner')
  )
}

function updateWidth(el) {
  const labelEl = findLabelEl(el)
  if (!labelEl) return
  const text = (labelEl.textContent || labelEl.value || '').trim()
  if (!text) return

  const textWidth = measureTextWidth(text, getComputedStyle(labelEl))
  const wrapper = el.querySelector('.el-select__wrapper')
  const selection = el.querySelector('.el-select__selection')
  // 装饰宽度 = 边框 + 左右内边距 + 箭头图标，实测约 44px，留浮动区间防极端主题
  let chrome = 48
  if (wrapper && selection && wrapper.offsetWidth > selection.offsetWidth) {
    chrome = Math.min(80, Math.max(40, wrapper.offsetWidth - selection.offsetWidth))
  }
  const target = Math.max(MIN_WIDTH, textWidth + chrome)
  const current = parseFloat(el.style.width) || 0
  if (Math.abs(current - target) > 1) {
    el.style.width = `${target}px`
  }
}

export default {
  mounted(el) {
    // 挂标让 global.css 的约束（min-width:150px / max-width:100%，移动端 100%）生效
    el.classList.add('select-fit')
    requestAnimationFrame(() => updateWidth(el))
    // 字体异步加载完成后度量会变，兜底重测一次
    if (document.fonts && document.fonts.ready) {
      document.fonts.ready.then(() => updateWidth(el))
    }
  },
  updated(el) {
    requestAnimationFrame(() => updateWidth(el))
  },
  unmounted(el) {
    el.classList.remove('select-fit')
  },
}
