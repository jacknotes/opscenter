// 前端常量定义

// localStorage keys
export const STORAGE_KEYS = {
  K8S_SERVER: 'k8s_server',
  LVS_SERVER: 'lvs_server',
  LVS_VS_FILTER: 'lvs_vs_filter',
  NGINX_SERVER: 'nginx_server',
  PREPROD_SERVER: 'preprod_server',
  nginxConfig: (serverId) => `nginx_config_${serverId}`,
}

// 分页
export const DEFAULT_PAGE_SIZE = 20

// 自动刷新间隔（毫秒）
export const AUTO_REFRESH_INTERVAL_MS = 300000

// 备份列表请求超时（毫秒）
export const BACKUP_FETCH_TIMEOUT_MS = 10000
