/** 服务器 */
export interface Server {
  id: number
  name: string
  host: string
  port: number
  type: string // lvs | nginx | kubernetes | preprod
  username: string
  enabled: boolean
  created_at: string
  updated_at: string
}

/** 服务器表单（创建/编辑） */
export interface ServerForm {
  name: string
  host: string
  port: number
  type: string
  username: string
  password?: string
  private_key?: string
  script_password?: string
  enabled?: boolean
}

/** 用户 */
export interface User {
  id: number
  username: string
  role: 'admin' | 'user'
  enabled: boolean
  created_at: string
  updated_at: string
}

/** 用户表单 */
export interface UserForm {
  username: string
  password?: string
  role: 'admin' | 'user'
  enabled?: boolean
}

/** 操作日志 */
export interface OperationLog {
  id: number
  user_id: number
  username: string
  module: string
  action: string
  target: string
  detail: string
  ip: string
  created_at: string
}

/** LVS VIP */
export interface LvsVip {
  ip: string
  port: string
  status: 'up' | 'down'
  rs: LvsRs[]
}

/** LVS RealServer */
export interface LvsRs {
  ip: string
  port: string
  status: 'up' | 'down'
  weight?: number
  tag?: string
  disabled?: boolean
  disabledReason?: string
}

/** Nginx Upstream */
export interface NginxUpstream {
  name: string
  servers: NginxServer[]
  upCount: number
  downCount: number
  hasBoth: boolean
}

/** Nginx Server */
export interface NginxServer {
  ip: string
  port: string
  status: 'up' | 'down'
  backup?: boolean
}

/** K8S Rollout */
export interface K8sRollout {
  name: string
  namespace: string
  status: string
  replicas?: number
  updated?: number
  ready?: number
}

/** 预生产状态 */
export interface PreprodStatus {
  project: string
  replicas: number
  desired: number
  status: string
}

/** Dashboard 统计 */
export interface DashboardStats {
  online_users: number
  login_success: number
  login_failed: number
}

/** Dashboard 远程统计 */
export interface RemoteStats {
  lvs: { up: number; down: number }
  nginx: { up: number; down: number }
  k8s: { online: number; total: number }
  preprod: { online: number; total: number }
}

/** 活动统计 */
export interface ActivityStat {
  date: string
  count: number
}

/** 项目统计 */
export interface ProjectStat {
  project: string
  count: number
}

/** 仪表盘标签映射 */
export type ModuleType = 'lvs' | 'nginx' | 'k8s' | 'preprod' | 'server' | 'user' | 'auth'

/** LDAP 用户 */
export interface LdapUser {
  username: string
  dn: string
  selected?: boolean
}
