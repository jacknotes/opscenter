import { watch } from 'vue'

/**
 * 输出缓存组合式函数
 * 在切换服务器/配置等场景时缓存和恢复执行输出
 * @param {Function|Function[]} keyFns - 返回缓存键的函数或函数数组（复合键）
 * @param {import('vue').Ref} outputRef - 输出内容的 ref
 * @param {object} opts
 * @param {Function} opts.getExtra - 获取额外缓存数据 (如 metadata)
 * @param {Function} opts.setExtra - 恢复额外缓存数据
 * @param {import('vue').Ref} opts.blockCondition - 阻止缓存切换的条件（如 WebSocket 活跃时）
 * @returns {{ outputCache }}
 */
export function useOutputCache(keyFns, outputRef, opts = {}) {
  const outputCache = new Map()
  const fns = Array.isArray(keyFns) ? keyFns : [keyFns]

  function buildKey() {
    return fns.map((fn) => fn()).join(':')
  }

  let lastKey = buildKey()

  watch(
    fns.map((fn) => fn),
    () => {
      // 阻止条件满足时不切换缓存
      if (opts.blockCondition?.value) return

      const newKey = buildKey()
      if (newKey === lastKey) return

      // 保存旧值到缓存
      const data = opts.getExtra ? { output: outputRef.value, extra: opts.getExtra() } : outputRef.value
      outputCache.set(lastKey, data)

      // 从缓存恢复新值
      const cached = outputCache.get(newKey)
      if (cached !== undefined) {
        if (opts.setExtra && typeof cached === 'object' && 'output' in cached) {
          outputRef.value = cached.output
          opts.setExtra(cached.extra)
        } else {
          outputRef.value = cached
        }
      } else {
        if (opts.setExtra) opts.setExtra(null)
        outputRef.value = ''
      }

      lastKey = newKey
    }
  )

  return { outputCache }
}
