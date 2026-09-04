import { watch, type Ref } from 'vue'

interface OutputCacheOptions<T, E> {
  /** 获取额外缓存数据（如执行状态等元信息） */
  getExtra?: () => E
  /** 恢复额外缓存数据（无缓存时以 null 调用） */
  setExtra?: (extra: E | null) => void
  /** 阻止缓存切换的条件（如 WebSocket 执行中时） */
  blockCondition?: Ref<boolean>
  /** 无缓存时的空值工厂（默认空字符串） */
  emptyValue?: () => T
}

interface CachedWithExtra<T, E> {
  output: T
  extra: E
}

/**
 * 输出缓存组合式函数
 * 在切换服务器/配置等场景时缓存和恢复执行输出
 *
 * @param keyFns - 返回缓存键的函数或函数数组（复合键）
 * @param outputRef - 输出内容的 ref
 * @param opts - getExtra / setExtra / blockCondition / emptyValue
 * @returns {{ outputCache }} 缓存 Map（键为各 keyFn 返回值以 ":" 拼接）
 */
export function useOutputCache<T, E = unknown>(
  keyFns: (() => string | number | undefined | null) | Array<() => string | number | undefined | null>,
  outputRef: Ref<T>,
  opts: OutputCacheOptions<T, E> = {},
): { outputCache: Map<string, T | CachedWithExtra<T, E>> } {
  const outputCache = new Map<string, T | CachedWithExtra<T, E>>()
  const fns = Array.isArray(keyFns) ? keyFns : [keyFns]

  function buildKey(): string {
    return fns
      .map((fn) => fn())
      .join(':')
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
      const data: T | CachedWithExtra<T, E> = opts.getExtra
        ? { output: outputRef.value, extra: opts.getExtra() }
        : outputRef.value
      outputCache.set(lastKey, data)

      // 从缓存恢复新值
      const cached = outputCache.get(newKey)
      if (cached !== undefined) {
        if (opts.setExtra && typeof cached === 'object' && cached !== null && 'output' in cached) {
          outputRef.value = cached.output
          opts.setExtra(cached.extra)
        } else {
          outputRef.value = cached as T
        }
      } else {
        if (opts.setExtra) opts.setExtra(null)
        outputRef.value = opts.emptyValue ? opts.emptyValue() : ('' as unknown as T)
      }

      lastKey = newKey
    },
  )

  return { outputCache }
}
