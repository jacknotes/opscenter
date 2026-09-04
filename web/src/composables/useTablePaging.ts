import { ref, computed, type Ref } from 'vue'

/**
 * 客户端整表排序 + 分页。
 * - onSortChange 接 el-table 的 sort-change 事件
 * - compareValues：布尔按 0/1、数字/数字字符串按数值、其余 localeCompare('zh-Hans-CN')
 * - sortKey 支持派生列 getter（列值不在行对象上时）
 */
export function useTablePaging<T>(
  source: Ref<T[]>,
  initialPageSize = 20,
  opts: { sortKey?: (row: T) => string | number | boolean | null | undefined } = {},
) {
  const currentPage = ref(1)
  const pageSize = ref(initialPageSize)
  const sortBy = ref<keyof T & string | ''>('')
  const sortOrder = ref<'ascending' | 'descending' | null>(null)

  function compareValues(a: unknown, b: unknown): number {
    // 布尔按 0/1（避免 String(false) 排在 String(true) 前）
    if (typeof a === 'boolean' || typeof b === 'boolean') {
      const av = a === true ? 1 : a === false ? 0 : NaN
      const bv = b === true ? 1 : b === false ? 0 : NaN
      if (!Number.isNaN(av) && !Number.isNaN(bv)) return av - bv
    }
    // 数字 / 数字字符串按数值
    const na = typeof a === 'number' ? a : typeof a === 'string' && a !== '' && !Number.isNaN(Number(a)) ? Number(a) : NaN
    const nb = typeof b === 'number' ? b : typeof b === 'string' && b !== '' && !Number.isNaN(Number(b)) ? Number(b) : NaN
    if (!Number.isNaN(na) && !Number.isNaN(nb)) return na - nb
    return String(a ?? '').localeCompare(String(b ?? ''), 'zh-Hans-CN')
  }

  const sorted = computed(() => {
    const arr = [...source.value]
    if (!sortBy.value && !opts.sortKey) return arr
    const key = sortBy.value as string
    const getter = opts.sortKey
    const dir = sortOrder.value === 'descending' ? -1 : 1
    return arr.sort((x, y) => {
      const va = key && key in (x as object) ? (x as Record<string, unknown>)[key] : getter?.(x)
      const vb = key && key in (y as object) ? (y as Record<string, unknown>)[key] : getter?.(y)
      return dir * compareValues(va, vb)
    })
  })

  const total = computed(() => sorted.value.length)

  const paged = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return sorted.value.slice(start, start + pageSize.value)
  })

  function onSortChange({
    prop,
    order,
  }: {
    prop?: string | null
    order?: 'ascending' | 'descending' | null
  }): void {
    sortBy.value = (order ? (prop ?? '') : '') as keyof T & string
    sortOrder.value = order ?? null
  }

  return { currentPage, pageSize, sortBy, sortOrder, total, paged, onSortChange }
}
