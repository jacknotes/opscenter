/**
 * 后端 API 类型定义 —— 逐字段对照 docs/frontend-v2/api-contract.md。
 * 后端无 {code,message,data} 包装：成功直接是数据，失败是 {"error": "..."}。
 */

// ---------- 认证 ----------
export interface User {
  id: number
  username: string
  name: string
  email: string
  role: 'admin' | 'user'
  enabled: boolean
  auth_source: 'local' | 'ldap'
  /** 登录失败次数（达到阈值后锁定） */
  failed_attempts: number
  /** 是否被锁定（管理员可解锁） */
  locked: boolean
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  token: string
  user: User
}

/** GET /dashboard/online-users —— 在线用户（admin） */
export interface OnlineUser {
  username: string
  role: string
  login_time: string
  login_method: string
  last_active: string
}

export interface OnlineUsersResponse {
  users: OnlineUser[]
  total: number
}

// ---------- 服务器 ----------
export interface ServerResponse {
  id: number
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  has_password: boolean
  has_private_key: boolean
  has_script_password: boolean
  server_type: string // lvs | nginx | kubernetes | preprod
  env: string
  script_path: string
  config_path: string
  config_pattern: string
  backup_path: string
  description: string
  enabled: boolean
  created_at: string
  updated_at: string
}

/** GET /servers/:id/edit —— 敏感字段恒为空字符串 */
export interface ServerEdit extends Omit<ServerResponse, 'has_password' | 'has_private_key' | 'has_script_password'> {
  password: string
  private_key: string
  script_password: string
  has_password: boolean
  has_private_key: boolean
  has_script_password: boolean
}

/** Create/Update 请求体；Update 时敏感字段传 "__keep__" 表示保留原值 */
export interface ServerPayload {
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  password?: string
  private_key?: string
  server_type: string
  env?: string
  script_path?: string
  script_password?: string
  config_path?: string
  config_pattern?: string
  backup_path?: string
  description?: string
  enabled?: boolean
}

export interface TestResult {
  success: boolean
  message?: string
  error?: string
  output?: string
}

export interface BatchResult {
  message: string
  deleted?: number
  updated?: number
  failed: number
  success?: number
  results?: { id: number; name: string; success: boolean; error?: string }[]
}

// ---------- LVS ----------
export interface LvsRealServer {
  ip: string
  port: string
  forward: string
  weight: number
  active_conn: number
  inact_conn: number
  status: 'up' | 'down'
  tag?: string
  disabled?: boolean
  disabled_reason?: string
}

export interface VirtualServer {
  ip: string
  port: string
  protocol: string
  scheduler: string
  flags: string
  role?: 'master' | 'backup'
  tag?: string
  real_servers: LvsRealServer[]
}

export interface LvsStatusGroup {
  vs_ip: string
  vs_port: string
  real_servers: { ip: string; port: string; status: 'up' | 'down' }[]
}

export interface LvsRSTag {
  id: number
  rs_ip: string
  vs_ip: string
  tag: string
  disabled: boolean
  disabled_reason: string
  created_at: string
  updated_at: string
}

export interface LvsVSTag {
  id: number
  vs_ip: string
  tag: string
  created_at: string
  updated_at: string
}

export interface LvsPreprodBinding {
  id: number
  vs_tag: string
  rs_env_tag: string
  preprod_server_id: number
  created_at: string
  updated_at: string
}

/** LVS op/swap 预览响应 */
export interface LvsPreview {
  preview_id: string
  current_status: string
  command: string
  description: string
}

/** LVS/Preprod（HTTP）execute 响应：output 是 string */
export interface CommandExecuteResult {
  output: string
  status: string
}

export interface LvsScaledownWarning {
  vs_tag: string
  rs_env_tag: string
  rs_ip: string
  status: 'up' | 'down'
  lvs_server: string
}

export interface LvsScaledownCheck {
  need_warning: boolean
  warnings?: LvsScaledownWarning[]
}

// ---------- K8s ----------
export interface Rollout {
  namespace: string
  name: string
  strategy: string
  status: string
  step: string
  set_weight: string
  ready: string
  desired: number
  up_to_date: number
  available: number
}

export interface K8sProjectRef {
  name: string
  namespace: string
}

/** K8s 预览响应：commands 是数组 */
export interface K8sPreview {
  preview_id: string
  current_status: string
  commands: string[]
  description: string
}

/** K8s execute 响应：output 是 string 数组（失败时也是数组） */
export interface K8sExecuteResult {
  output: string[]
  status: string
}

// ---------- Preprod ----------
export interface PreprodResource {
  category: 'rollout' | 'deployment' | 'statefulset'
  name: string
  desired: number
  current: number
  up_to_date: number
  available: number
  age: string
  target_replicas: number
}

export interface PreprodPreview {
  preview_id: string
  current_status: string
  command: string
  description: string
}

export interface LvsOnlineWarning {
  name: string
  category: string
  current: number
  target: number
}

export interface LvsOnlineCheck {
  need_warning: boolean
  warnings?: LvsOnlineWarning[]
  vs_tag?: string
  rs_env_tag?: string
}

// ---------- Nginx ----------
export interface NginxServer {
  ip: string
  port: string
  status: 'up' | 'down'
  weight: string
}

export interface NginxUpstream {
  name: string
  servers: NginxServer[]
  config: string
}

export interface NginxUpstreamsResponse {
  upstreams: NginxUpstream[]
  raw: string
}

export interface LineDiff {
  line_num: number
  type: 'same' | 'added' | 'removed'
  content: string
}

/** Nginx 各类预览响应（rollback 无 line_diffs） */
export interface NginxPreview {
  preview_id: string
  before: string
  after: string
  line_diffs?: LineDiff[]
  description: string
}

/** Nginx online/offline 预览请求 */
export interface NginxUpstreamPayload {
  server_id: number
  config_file: string
  upstream_names: string[]
  backend_ip: string
}

export interface NginxSwapPayload {
  server_id: number
  config_file: string
  upstream_names: string[]
  offline_ip: string
  online_ip: string
}

export interface NginxTogglePayload {
  server_id: number
  config_file: string
  upstream_names: string[]
}

export interface NginxBatchItem {
  upstream_name: string
  action: 'online' | 'offline' | 'toggle'
  backend_ip?: string
}

export interface NginxBatchPayload {
  server_id: number
  config_file: string
  items: NginxBatchItem[]
}

export interface NginxRollbackPayload {
  server_id: number
  config_file: string
  backup_file: string
}

/** Nginx execute 成功响应：{message, output?}，rollback 无 output */
export interface NginxExecuteResult {
  message: string
  output?: string
}

// ---------- 日志 ----------
export interface OperationLog {
  id: number
  user_id: number
  username: string
  module: string // lvs | k8s | nginx | preprod | server | auth
  action: string
  target: string
  detail: string
  status: 'success' | 'failed'
  output: string
  preview_id: string
  server_id: number
  server_name: string
  ip: string
  project_names: string
  project_count: number
  created_at: string
}

/** 唯一分页包装：GET /api/logs */
export interface LogPage {
  total: number
  page: number
  size: number
  data: OperationLog[]
}

export interface LogQuery {
  page?: number
  size?: number
  module?: string
  server_id?: string
  username?: string
  status?: string
  action?: string
  keyword?: string
  start_time?: string // YYYY-MM-DD
  end_time?: string
  /** 服务端排序字段（后端白名单：created_at/username/module/status） */
  sort_by?: 'created_at' | 'username' | 'module' | 'status'
  sort_order?: 'asc' | 'desc'
}

// ---------- Dashboard ----------
export interface DashboardStats {
  servers: {
    total: number
    enabled: number
    disabled: number
    by_type: Record<string, number>
    by_env: Record<string, number>
  }
  /** 仅 admin 返回 */
  users?: {
    total: number
    enabled: number
    disabled: number
    by_role: Record<string, number>
  }
  /** 仅 admin 返回 */
  online_users?: number
}

export interface LvsRemoteStat {
  vs_count: number
  rs_online: number
  rs_offline: number
  total_active_conn: number
  total_inact_conn: number
}

export interface NginxRemoteStat {
  upstream_count: number
  server_online: number
  server_offline: number
}

export interface K8sRemoteStat {
  total_rollouts: number
  by_namespace: Record<string, number>
  pending: number
  online: number
}

export interface PreprodRemoteStat {
  total_resources: number
  scaled_down: number
  expanded: number
  normal: number
}

/** 任一模块查询失败时对应字段为 null */
export interface RemoteStats {
  lvs: LvsRemoteStat | null
  nginx: NginxRemoteStat | null
  k8s: K8sRemoteStat | null
  preprod: PreprodRemoteStat | null
}

/** Dashboard 日期范围，格式 YYYY-MM-DD */
export type DateRange = [string, string] | string[] | null
export type Granularity = 'day' | 'week' | 'month' | 'year'

export interface DeployStatPoint {
  period: string
  module: string
  count: number
}

export interface LoginStatPoint {
  period: string
  status: string
  count: number
}

export interface ActionStatPoint {
  module: string
  action: string
  count: number
}

export interface ActivityStats {
  deploy_stats: DeployStatPoint[]
  /** 仅 admin 返回真实数据；普通用户恒为空数组 */
  login_stats: LoginStatPoint[]
  action_stats: ActionStatPoint[]
}

export interface ProjectSummary {
  total: number
  success: number
  failed: number
  full_ops: number
}

export interface ProjectTrendPoint {
  period: string
  project: string
  count: number
}

export interface ProjectStat {
  project: string
  count: number
  success: number
  failed: number
}

export interface ActionStat {
  action: string
  count: number
}

export interface ProjectStatsResponse {
  summary: ProjectSummary
  trend: ProjectTrendPoint[]
  by_project: ProjectStat[]
  by_action: ActionStat[]
}

export interface LvsConnPoint {
  collected_at: string
  active_conn: number
  inact_conn: number
}

// ---------- 用户 ----------
export interface LdapUser {
  username: string
  name: string
  email: string
  dn: string
}

export interface LdapImportResult {
  message: string
  imported: number
  skipped: number
  failed: number
}

// ---------- WebSocket ----------
export type WsStream = 'stdout' | 'stderr'

export interface WsMessage {
  type: 'output' | 'done' | 'error' | 'lock_error' | 'start'
  token?: string
  preview_id?: string
  data?: string
  stream?: WsStream
  status?: string
  message?: string
  holder?: string
}

// ---------- 预览执行（execute 统一请求体） ----------
export interface PreviewExecutePayload {
  preview_id: string
}
