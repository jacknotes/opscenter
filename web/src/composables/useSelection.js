import { ref, computed, watch, nextTick } from 'vue'

/**
 * 跨页选择管理组合式函数（Set-based）
 * @param {string|Function} keyField - 行唯一标识字段名或提取函数 (row) => key
 * @param {import('vue').ComputedRef} paginatedItems - 当前分页数据
 * @param {object} opts
 * @param {import('vue').Ref} opts.search - 搜索关键词（变化时重置页码并恢复选择）
 * @param {import('vue').Ref} opts.currentPage - 当前页码
 * @returns {{ selectedIds, allSelected, tableRef, handleSelectionChange, handleSizeChange, handleCurrentChange, toggleSelectAll, restoreSelection }}
 */
export function useSelection(keyField, paginatedItems, opts = {}) {
  const selectedIds = ref(new Set())
  const tableRef = ref(null)
  const skipSelectionSync = ref(false)

  const getKey = typeof keyField === 'function' ? keyField : (row) => row[keyField]

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

  function handleSizeChange(size) {
    skipSelectionSync.value = true
    if (opts.currentPage) opts.currentPage.value = 1
    nextTick(() => restoreSelection())
  }

  function handleCurrentChange() {
    skipSelectionSync.value = true
    nextTick(() => restoreSelection())
  }

  function handleSelectionChange(rows) {
    if (skipSelectionSync.value) return
    const pageKeys = paginatedItems.value.map((r) => getKey(r))
    pageKeys.forEach((key) => selectedIds.value.delete(key))
    rows.forEach((r) => selectedIds.value.add(getKey(r)))
  }

  function toggleSelectAll(clearAll) {
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
