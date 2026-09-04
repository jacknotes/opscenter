import { ref, computed, watch, nextTick, type ComputedRef, type Ref } from 'vue'
import type { TableInstance } from 'element-plus'

/**
 * 跨页选择管理组合式函数（Set-based）
 * 翻页/改分页大小/搜索后自动恢复当页行的勾选状态。
 *
 * @param keyField 行唯一标识字段名或提取函数 (row) => key
 * @param paginatedItems 当前分页数据
 * @param opts.search 搜索关键词（变化时重置页码并恢复选择）
 * @param opts.currentPage 当前页码
 */
export function useSelection<T>(
  keyField: keyof T | ((row: T) => string | number),
  paginatedItems: ComputedRef<T[]> | Ref<T[]>,
  opts: { search?: Ref<unknown>; currentPage?: Ref<number> } = {},
) {
  const selectedIds = ref(new Set<string | number>())
  const tableRef = ref<TableInstance | null>(null)
  const skipSelectionSync = ref(false)

  const getKey = typeof keyField === 'function' ? keyField : (row: T) => row[keyField] as string | number

  const allSelected = computed(() => {
    const items = paginatedItems.value
    return items.length > 0 && items.every((r) => selectedIds.value.has(getKey(r)))
  })

  function restoreSelection() {
    skipSelectionSync.value = true
    nextTick(() => {
      paginatedItems.value.forEach((row) => {
        if (selectedIds.value.has(getKey(row))) {
          tableRef.value?.toggleRowSelection(row, true)
        }
      })
      skipSelectionSync.value = false
    })
  }

  function handleSizeChange(size: number) {
    skipSelectionSync.value = true
    if (opts.currentPage) opts.currentPage.value = 1
    nextTick(() => restoreSelection())
  }

  function handleCurrentChange() {
    skipSelectionSync.value = true
    nextTick(() => restoreSelection())
  }

  function handleSelectionChange(rows: T[]) {
    if (skipSelectionSync.value) return
    const pageKeys = paginatedItems.value.map((r) => getKey(r))
    pageKeys.forEach((key) => selectedIds.value.delete(key))
    rows.forEach((r) => selectedIds.value.add(getKey(r)))
  }

  function toggleSelectAll(clearAll?: boolean) {
    if (clearAll || allSelected.value) {
      selectedIds.value.clear()
      tableRef.value?.clearSelection()
    } else {
      paginatedItems.value.forEach((row) => {
        selectedIds.value.add(getKey(row))
      })
      paginatedItems.value.forEach((row) => {
        tableRef.value?.toggleRowSelection(row, true)
      })
    }
  }

  // 搜索变化时重置页码并恢复选择
  if (opts.search && opts.currentPage) {
    watch(opts.search, () => {
      skipSelectionSync.value = true
      opts.currentPage.value = 1
      nextTick(() => restoreSelection())
    })
  }

  return {
    selectedIds,
    allSelected,
    tableRef,
    handleSelectionChange,
    handleSizeChange,
    handleCurrentChange,
    toggleSelectAll,
    restoreSelection,
  }
}
