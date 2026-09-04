import { ElMessage } from 'element-plus'

/**
 * 显示加载失败提示，401 错误自动跳过（已由拦截器统一处理）
 * @param {any} error - 捕获的错误对象
 * @param {string} message - 提示信息
 */
export function showLoadError(error, message) {
  if (error?.response?.status === 401) return
  ElMessage.error(message)
}

/**
 * HTML 转义，防止 XSS
 */
export function escapeHtml(str) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * 显示批量操作结果消息
 * @param {object} res - 包含 message, deleted/updated, failed 字段的响应
 */
export function showBatchResult(res) {
  const message = res.message || '操作完成'
  const success = res.deleted || res.updated || 0
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
