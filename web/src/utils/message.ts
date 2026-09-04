import { ElMessage } from 'element-plus'

/**
 * 显示加载失败提示，401 错误自动跳过（已由拦截器统一处理）
 */
export function showLoadError(error: unknown, message: string): void {
  const status = (error as { response?: { status?: number } } | null)?.response?.status
  if (status === 401) return
  ElMessage.error(message)
}

/**
 * HTML 转义，防止 XSS
 */
export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * 显示批量操作结果消息（按成功/部分失败/全部失败分色，支持多行详情）
 */
export function showBatchResult(res: {
  message?: string
  deleted?: number
  updated?: number
  success?: number
  failed?: number
}): void {
  const message = res.message || '操作完成'
  const success = res.deleted || res.updated || res.success || 0
  const failed = res.failed || 0

  const htmlMessage = escapeHtml(message).replace(/\n/g, '<br>')

  if (failed === 0) {
    ElMessage({ message: htmlMessage, type: 'success', dangerouslyUseHTMLString: true })
  } else if (success === 0) {
    ElMessage({ message: htmlMessage, type: 'error', dangerouslyUseHTMLString: true })
  } else {
    ElMessage({ message: htmlMessage, type: 'warning', dangerouslyUseHTMLString: true, duration: 5000 })
  }
}
