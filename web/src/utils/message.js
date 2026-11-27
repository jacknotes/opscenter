import { ElMessage } from 'element-plus'

/**
 * 显示加载失败提示，401 错误自动跳过（已由拦截器统一处理）
 * @param {any} error - 捕获的错误对象
 * @param {string} message - 提示信息
 */
export function showLoadError(error, message) {
  if (isAuthError(error)) return
  ElMessage.error(message)
}

/**
 * 判断是否为已由拦截器统一处理的认证/权限错误
 * （401 未登录或登录失效、403 权限不足，拦截器已弹出对应提示）
 * @param {any} error - 捕获的错误对象
 * @returns {boolean}
 */
export function isAuthError(error) {
  const status = error?.response?.status
  return status === 401 || status === 403
}

/**
 * 显示 API 操作失败提示（读取后端 error 字段）。
 * 401/403 已由拦截器统一提示，此处跳过，避免出现重复弹窗
 * （如登录失效时同时弹出"当前登录已失效"和"未提供认证令牌"）。
 * @param {any} error - 捕获的错误对象
 * @param {string} fallback - 后端未返回错误信息时的兜底文案
 */
export function showApiError(error, fallback = '操作失败') {
  if (isAuthError(error)) return
  ElMessage.error(error?.response?.data?.error || fallback)
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

export default { showLoadError, isAuthError, showApiError, escapeHtml, showBatchResult }
