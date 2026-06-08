/**
 * 格式化时间为中文本地格式
 */
export function formatTime(t: string | Date | number): string {
  return new Date(t).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

/**
 * 格式化时间为简短格式（仅时分秒）
 */
export function formatTimeShort(t: string | Date | number): string {
  return new Date(t).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZone: 'Asia/Shanghai',
  })
}

/**
 * 格式化字节数
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

/**
 * 格式化持续时间（毫秒）
 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`
}
