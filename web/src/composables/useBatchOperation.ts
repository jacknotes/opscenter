import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface UseBatchOptions {
  /** 超过此数量需输入确认文字 */
  threshold?: number
  /** 确认文字（默认 "确认执行"） */
  confirmText?: string
}

/**
 * 统一批量操作 composable
 * 提供选择管理、确认对话框、批量执行能力
 */
export function useBatchOperation(opts: UseBatchOptions = {}) {
  const selectedRows = ref<Set<any>>(new Set())
  const selectedCount = computed(() => selectedRows.value.size)
  const loading = ref(false)

  function toggle(row: any) {
    if (selectedRows.value.has(row)) {
      selectedRows.value.delete(row)
    } else {
      selectedRows.value.add(row)
    }
  }

  function isSelected(row: any) {
    return selectedRows.value.has(row)
  }

  function clearAll() {
    selectedRows.value.clear()
  }

  function selectAll(rows: any[]) {
    rows.forEach((row) => selectedRows.value.add(row))
  }

  /**
   * 确认批量操作
   * @param actionName - 操作名称（用于提示）
   * @param onConfirm - 确认后的回调
   */
  async function confirmBatch(actionName: string, onConfirm: () => Promise<void>) {
    if (selectedRows.value.size === 0) {
      ElMessage.warning('请先选择操作项')
      return
    }

    const threshold = opts.threshold ?? 10
    const confirmText = opts.confirmText ?? '确认执行'

    if (selectedRows.value.size >= threshold) {
      try {
        await ElMessageBox.prompt(
          `当前选择了 ${selectedRows.value.size} 项，输入"${confirmText}"以继续`,
          `${actionName}确认`,
          {
            inputPattern: new RegExp(confirmText),
            inputErrorMessage: '输入不正确',
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning',
          }
        )
      } catch {
        return // 用户取消
      }
    }

    loading.value = true
    try {
      await onConfirm()
    } finally {
      loading.value = false
    }
  }

  return {
    selectedRows,
    selectedCount,
    loading,
    toggle,
    isSelected,
    clearAll,
    selectAll,
    confirmBatch,
  }
}
