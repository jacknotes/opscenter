import { ElMessage } from 'element-plus'

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
