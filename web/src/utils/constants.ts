/** localStorage 键名 */
export const STORAGE_KEYS = {
  TOKEN: 'token',
  ROLE: 'role',
  THEME: 'theme',
  SIDEBAR_COLLAPSED: 'sidebarCollapse',
  K8S_SERVER: 'k8s_server',
  LVS_SERVER: 'lvs_server',
  LVS_VS_FILTER: 'lvs_vs_filter',
  NGINX_SERVER: 'nginx_server',
  PREPROD_SERVER: 'preprod_server',
  LVS_CONN_SERVER: 'lvs_conn_server',
  LVS_CONN_VS: 'lvs_conn_vs',
  LVS_CONN_RS: 'lvs_conn_rs',
  LVS_CONN_DURATION: 'lvs_conn_duration',
  nginxConfig: (serverId: number | string) => `nginx_config_${serverId}`,
} as const

/** 模块显示名称 */
export const MODULE_LABELS: Record<string, string> = {
  lvs: 'LVS',
  nginx: 'Nginx',
  k8s: 'K8S',
  preprod: '预生产',
  server: '服务器',
  user: '用户',
  auth: '认证',
}

/** 模块对应的 Tag 类型 */
export const MODULE_TAG_TYPES: Record<string, string> = {
  lvs: '',
  nginx: 'success',
  k8s: 'warning',
  preprod: 'info',
  server: 'danger',
  user: '',
  auth: 'info',
}

/** 分页大小选项 */
export const PAGE_SIZES = [10, 20, 50, 100] as const

/** 默认分页大小 */
export const DEFAULT_PAGE_SIZE = 10

/** 自动刷新间隔（毫秒） */
export const AUTO_REFRESH_INTERVAL_MS = 300000 // 5 分钟

/** 备份请求超时（毫秒） */
export const BACKUP_FETCH_TIMEOUT_MS = 10000 // 10 秒

/** WebSocket 连接超时（毫秒） */
export const WS_CONNECT_TIMEOUT_MS = 10000 // 10 秒

/** 批量操作确认阈值 */
export const BATCH_CONFIRM_THRESHOLD = 10

/** 批量操作确认文字 */
export const BATCH_CONFIRM_TEXT = '确认执行'

/** 服务器类型 */
export const SERVER_TYPES = ['lvs', 'nginx', 'kubernetes', 'preprod'] as const
export type ServerType = (typeof SERVER_TYPES)[number]
