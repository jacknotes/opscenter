export const forceReflow = {
  mounted(el) {
    requestAnimationFrame(() => {
      const scrollEl = el.querySelector('.el-table__inner-wrapper') || el
      scrollEl.style.overflowX = 'hidden'
      void scrollEl.offsetHeight
      scrollEl.style.overflowX = ''
    })
  }
}
