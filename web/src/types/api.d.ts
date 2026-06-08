/** 通用 API 响应 */
export interface ApiResponse<T = any> {
  data?: T
  message?: string
  error?: string
}

/** 分页参数 */
export interface PaginationParams {
  page?: number
  page_size?: number
}

/** 批量操作结果 */
export interface BatchResult {
  message?: string
  deleted?: number
  updated?: number
  failed?: number
}

/** 预览响应 */
export interface PreviewResponse {
  preview_id: string
  preview_data?: any
  message?: string
}

/** 执行请求 */
export interface ExecuteRequest {
  preview_id: string
}

/** WebSocket 消息 */
export interface WsMessage {
  type: 'output' | 'done' | 'error' | 'lock_error'
  data?: string
  stream?: 'stdout' | 'stderr'
  message?: string
}

/** WebSocket 输出行 */
export interface OutputLine {
  text: string
  stream: 'stdout' | 'stderr'
}
