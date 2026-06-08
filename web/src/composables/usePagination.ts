import { ref, computed, watch } from 'vue'

interface UsePaginationOptions {
  pageSize?: number
  resetOn?: () => any[]
}

/**
 * 统一分页 composable
 * @param dataFn - 返回完整数据列表的函数
 * @param opts.pageSize - 每页条数（默认 20）
 * @param opts.resetOn - 返回依赖数组的函数，数据变化时重置页码
 */
export function usePagination<T>(dataFn: () => T[], opts: UsePaginationOptions = {}) {
  const currentPage = ref(1)
  const pageSize = ref(opts.pageSize ?? 20)

  const total = computed(() => dataFn().length)

  const paginated = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return dataFn().slice(start, start + pageSize.value)
  })

  // 依赖变化时重置页码
  if (opts.resetOn) {
    watch(opts.resetOn, () => {
      currentPage.value = 1
    })
  }

  function handlePageChange(page: number) {
    currentPage.value = page
  }

  function handleSizeChange(size: number) {
    pageSize.value = size
    currentPage.value = 1
  }

  return {
    currentPage,
    pageSize,
    total,
    paginated,
    handlePageChange,
    handleSizeChange,
  }
}
