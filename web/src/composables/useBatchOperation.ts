import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { Ref } from 'vue'
import { BATCH_CONFIRM_TEXT, BATCH_CONFIRM_THRESHOLD } from '../utils/constants'

interface UseBatchOptions {
  /** 超过此数量需输入确认文字（默认 10） */
  threshold?: number
  /** 确认文字（默认 "确认执行"） */
  confirmText?: string
}

/**
 * 统一批量操作 composable
 * 提供选择管理、大数量文字确认对话框、批量执行 loading 态
 */
export function useBatchOperation<T = unknown>(opts: UseBatchOptions = {}) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Set 元素类型经 ref 包装后 TS 无法自动推导，显式断言
  const selectedRows = ref(new Set()) as Ref<Set<T>>
  const selectedCount = computed(() => selectedRows.value.size)
  const loading = ref(false)

  function toggle(row: T) {
    if (selectedRows.value.has(row)) {
      selectedRows.value.delete(row)
    } else {
      selectedRows.value.add(row)
    }
  }

  function isSelected(row: T) {
    return selectedRows.value.has(row)
  }

  function clearAll() {
    selectedRows.value.clear()
  }

  function selectAll(rows: T[]) {
    rows.forEach((row) => selectedRows.value.add(row))
  }

  /**
   * 确认批量操作：空选择提示；超过阈值需输入确认文字
   * @param actionName 操作名称（用于提示标题）
   * @param onConfirm 确认后的回调
   */
  async function confirmBatch(actionName: string, onConfirm: () => Promise<void>) {
    if (selectedRows.value.size === 0) {
      ElMessage.warning('请先选择操作项')
      return
    }

    const threshold = opts.threshold ?? BATCH_CONFIRM_THRESHOLD
    const confirmText = opts.confirmText ?? BATCH_CONFIRM_TEXT

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
          },
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
