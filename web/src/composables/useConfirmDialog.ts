import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'

/**
 * 危险操作确认 composable
 * 封装 ElMessageBox.confirm，提供统一的确认/取消处理
 */
export function useConfirmDialog() {
  const loading = ref(false)

  /**
   * 显示确认对话框
   * @param options.title - 对话框标题
   * @param options.message - 确认消息
   * @param options.type - 类型（warning/error/info）
   * @param options.confirmText - 确认按钮文字
   * @param options.onConfirm - 确认后的异步回调
   */
  async function confirm(options: {
    title: string
    message: string
    type?: 'warning' | 'error' | 'info'
    confirmText?: string
    onConfirm: () => Promise<void>
  }) {
    try {
      await ElMessageBox.confirm(options.message, options.title, {
        type: options.type || 'warning',
        confirmButtonText: options.confirmText || '确定',
        cancelButtonText: '取消',
      })
      loading.value = true
      await options.onConfirm()
    } catch {
      // 用户取消，不做任何事
    } finally {
      loading.value = false
    }
  }

  return { loading, confirm }
}
