import client from './client'
import type {
  ActivityStats,
  BatchResult,
  CommandExecuteResult,
  DashboardStats,
  K8sExecuteResult,
  K8sPreview,
  K8sProjectRef,
  LdapImportResult,
  LdapUser,
  LogPage,
  LogQuery,
  LoginResponse,
  LvsOnlineCheck,
  LvsPreview,
  LvsRSTag,
  LvsScaledownCheck,
  LvsStatusGroup,
  LvsPreprodBinding,
  LvsVSTag,
  NginxBatchPayload,
  NginxExecuteResult,
  NginxPreview,
  NginxRollbackPayload,
  NginxSwapPayload,
  NginxTogglePayload,
  NginxUpstreamPayload,
  NginxUpstreamsResponse,
  OnlineUsersResponse,
  OperationLog,
  PreprodPreview,
  PreprodResource,
  PreviewExecutePayload,
  ProjectStatsResponse,
  RemoteStats,
  Rollout,
  ServerEdit,
  ServerPayload,
  ServerResponse,
  TestResult,
  User,
  VirtualServer,
} from './types'

// Go 后端的空列表（nil 切片）会序列化为 JSON null，这里统一兜底为空数组
function listOf<T>(data: T[] | null): T[] {
  return data ?? []
}

// ---------- 认证 ----------
export const authApi = {
  login: (data: { username: string; password: string }) =>
    client.post<LoginResponse>('/login', data).then((r) => r.data),
  getUserInfo: () => client.get<User>('/user/info').then((r) => r.data),
  logout: () => client.post<{ message: string }>('/logout').then((r) => r.data),
  changeMyPassword: (id: number, data: { old_password: string; new_password: string }) =>
    client.put<{ message: string }>(`/users/${id}/password`, data).then((r) => r.data),
}

// ---------- Dashboard ----------
export const dashboardApi = {
  stats: () => client.get<DashboardStats>('/dashboard/stats').then((r) => r.data),
  remoteStats: () =>
    client
      .get<RemoteStats>('/dashboard/remote-stats', { timeout: 70000 })
      .then((r) => r.data),
  activityStats: (params: { start_date: string; end_date: string }) =>
    client.get<ActivityStats>('/dashboard/activity-stats', { params }).then((r) => r.data),
  k8sProjectStats: (params: { start_date: string; end_date: string; server_name?: string }) =>
    client.get<ProjectStatsResponse>('/dashboard/k8s-project-stats', { params }).then((r) => r.data),
  preprodProjectStats: (params: { start_date: string; end_date: string; server_name?: string }) =>
    client.get<ProjectStatsResponse>('/dashboard/preprod-project-stats', { params }).then((r) => r.data),
  /** 在线用户列表（admin，Redis 会话） */
  onlineUsers: () =>
    client.get<OnlineUsersResponse>('/dashboard/online-users').then((r) => r.data),
  lvsConnStats: (params: { server_id: number | string; vs_ip: string; rs_ip: string; duration?: number }) =>
    client.get<{ data: { collected_at: string; active_conn: number; inact_conn: number }[] }>(
      '/dashboard/lvs-conn-stats',
      { params },
    ).then((r) => r.data),
}

// ---------- LVS ----------
export const lvsApi = {
  list: (serverId: number | string) =>
    client
      .get<VirtualServer[] | null>('/lvs/list', { params: { server_id: serverId } })
      .then((r) => listOf(r.data)),
  status: (serverId: number | string) =>
    client
      .get<{ output: string; groups: LvsStatusGroup[] }>('/lvs/status', { params: { server_id: serverId } })
      .then((r) => r.data),
  listRSTags: (params?: { rs_ips?: string; vs_ip?: string }) =>
    client.get<LvsRSTag[] | null>('/lvs/tags', { params }).then((r) => listOf(r.data)),
  listVSTags: (params?: { vs_ips?: string }) =>
    client.get<LvsVSTag[] | null>('/lvs/vs_tags', { params }).then((r) => listOf(r.data)),
  listBindings: (params?: { preprod_server_id?: number }) =>
    client.get<LvsPreprodBinding[] | null>('/lvs/bindings', { params }).then((r) => listOf(r.data)),
  opPreview: (data: { server_id: number; vs_ip: string; rs_ip: string; state: 'on' | 'off' }) =>
    client.post<LvsPreview>('/lvs/op/preview', data).then((r) => r.data),
  opExecute: (data: PreviewExecutePayload) =>
    client.post<CommandExecuteResult>('/lvs/op/execute', data).then((r) => r.data),
  swapPreview: (data: { server_id: number; vs_ip: string; rs_ip1: string; rs_ip2: string }) =>
    client.post<LvsPreview>('/lvs/swap/preview', data).then((r) => r.data),
  swapExecute: (data: PreviewExecutePayload) =>
    client.post<CommandExecuteResult>('/lvs/swap/execute', data).then((r) => r.data),
  checkScaledown: (preprodServerId: number) =>
    client.post<LvsScaledownCheck>('/lvs/check/scaledown', { preprod_server_id: preprodServerId }).then((r) => r.data),
  saveRSTag: (data: { rs_ip: string; vs_ip: string; tag: string; disabled: boolean; disabled_reason: string }) =>
    client.put<{ message: string }>('/lvs/tags', data).then((r) => r.data),
  deleteRSTag: (vsIp: string, rsIp: string) =>
    client.delete<{ message: string }>(`/lvs/tags/${encodeURIComponent(vsIp)}/${encodeURIComponent(rsIp)}`).then((r) => r.data),
  saveVSTag: (data: { vs_ip: string; tag: string }) =>
    client.put<{ message: string }>('/lvs/vs_tags', data).then((r) => r.data),
  deleteVSTag: (vsIp: string) =>
    client.delete<{ message: string }>(`/lvs/vs_tags/${encodeURIComponent(vsIp)}`).then((r) => r.data),
  saveBinding: (data: { vs_tag: string; rs_env_tag: string; preprod_server_id: number }) =>
    client.put<{ message: string }>('/lvs/bindings', data).then((r) => r.data),
  deleteBinding: (id: number) =>
    client.delete<{ message: string }>(`/lvs/bindings/${id}`).then((r) => r.data),
}

// ---------- K8s ----------
export const k8sApi = {
  rollouts: (serverId: number | string) =>
    client
      .get<Rollout[] | null>('/k8s/rollouts', { params: { server_id: serverId } })
      .then((r) => listOf(r.data)),
  batchPreview: (
    action: 'online' | 'sync' | 'rollback',
    data: { server_id: number; projects: K8sProjectRef[] },
  ) => client.post<K8sPreview>(`/k8s/${action}/preview`, data).then((r) => r.data),
  // 后端同步串行执行全部命令，旧前端超时 600s，全局 30s 会误报超时（后端仍在执行）
  batchExecute: (action: 'online' | 'sync' | 'rollback', data: PreviewExecutePayload) =>
    client
      .post<K8sExecuteResult>(`/k8s/${action}/execute`, data, { timeout: 600000 })
      .then((r) => r.data),
  fullPreview: (action: 'full_online' | 'full_sync' | 'full_rollback', data: { server_id: number }) =>
    client.post<K8sPreview>(`/k8s/${action}/preview`, data).then((r) => r.data),
  fullExecute: (action: 'full_online' | 'full_sync' | 'full_rollback', data: PreviewExecutePayload) =>
    client
      .post<K8sExecuteResult>(`/k8s/${action}/execute`, data, { timeout: 600000 })
      .then((r) => r.data),
}

// ---------- Preprod ----------
export const preprodApi = {
  status: (serverId: number | string) =>
    client
      .get<PreprodResource[] | null>('/preprod/status', { params: { server_id: serverId } })
      .then((r) => listOf(r.data)),
  scaledownPreview: (data: { server_id: number; resource_names?: string[] }) =>
    client.post<PreprodPreview>('/preprod/scaledown/preview', data).then((r) => r.data),
  scaleupPreview: (data: { server_id: number; resource_names?: string[] }) =>
    client.post<PreprodPreview>('/preprod/scaleup/preview', data).then((r) => r.data),
  // HTTP execute 功能等价但无流式；产品流程走 WebSocket
  scaledownExecute: (data: PreviewExecutePayload) =>
    client.post<CommandExecuteResult>('/preprod/scaledown/execute', data).then((r) => r.data),
  scaleupExecute: (data: PreviewExecutePayload) =>
    client.post<CommandExecuteResult>('/preprod/scaleup/execute', data).then((r) => r.data),
  checkLvsOnline: (data: { vs_ip: string; rs_ip: string }) =>
    client.post<LvsOnlineCheck>('/preprod/check/lvs_online', data).then((r) => r.data),
}

// ---------- Nginx ----------
export const nginxApi = {
  configs: (serverId: number | string) =>
    client
      .get<string[] | null>('/nginx/configs', { params: { server_id: serverId } })
      .then((r) => listOf(r.data)),
  upstreams: (serverId: number | string, configFile: string) =>
    client
      .get<NginxUpstreamsResponse>('/nginx/upstreams', { params: { server_id: serverId, config_file: configFile } })
      .then((r) => r.data),
  backups: (serverId: number | string) =>
    client
      .get<string[] | null>('/nginx/backups', { params: { server_id: serverId } })
      .then((r) => listOf(r.data)),
  onlinePreview: (data: NginxUpstreamPayload) =>
    client.post<NginxPreview>('/nginx/upstream/online/preview', data).then((r) => r.data),
  offlinePreview: (data: NginxUpstreamPayload) =>
    client.post<NginxPreview>('/nginx/upstream/offline/preview', data).then((r) => r.data),
  swapPreview: (data: NginxSwapPayload) =>
    client.post<NginxPreview>('/nginx/upstream/swap/preview', data).then((r) => r.data),
  togglePreview: (data: NginxTogglePayload) =>
    client.post<NginxPreview>('/nginx/upstream/toggle/preview', data).then((r) => r.data),
  batchPreview: (data: NginxBatchPayload) =>
    client.post<NginxPreview>('/nginx/upstream/batch/preview', data).then((r) => r.data),
  rollbackPreview: (data: NginxRollbackPayload) =>
    client.post<NginxPreview>('/nginx/rollback/preview', data).then((r) => r.data),
  execute: (path: 'online' | 'offline' | 'swap' | 'toggle' | 'batch' | 'rollback', data: PreviewExecutePayload) =>
    // 后端 rollback 不在 /upstream 前缀下（POST /nginx/rollback/execute），单独分支
    client
      .post<NginxExecuteResult>(
        path === 'rollback' ? '/nginx/rollback/execute' : `/nginx/upstream/${path}/execute`,
        data,
      )
      .then((r) => r.data),
}

// ---------- 服务器 ----------
export const serverApi = {
  list: (params?: { type?: string; all?: boolean }) =>
    client.get<ServerResponse[] | null>('/servers', { params }).then((r) => listOf(r.data)),
  get: (id: number | string) => client.get<ServerResponse>(`/servers/${id}`).then((r) => r.data),
  getForEdit: (id: number | string) => client.get<ServerEdit>(`/servers/${id}/edit`).then((r) => r.data),
  create: (data: ServerPayload) => client.post<ServerResponse>('/servers', data).then((r) => r.data),
  update: (id: number | string, data: Partial<ServerPayload>) =>
    client.put<ServerResponse>(`/servers/${id}`, data).then((r) => r.data),
  delete: (id: number | string) => client.delete<{ message: string }>(`/servers/${id}`).then((r) => r.data),
  toggle: (id: number | string) =>
    client.put<{ message: string; enabled: boolean }>(`/servers/${id}/toggle`).then((r) => r.data),
  test: (id: number | string) => client.post<TestResult>(`/servers/${id}/test`).then((r) => r.data),
  batchDelete: (ids: number[]) =>
    client.post<BatchResult>('/servers/batch-delete', { ids }).then((r) => r.data),
  batchToggle: (ids: number[], enabled: boolean) =>
    client.post<BatchResult>('/servers/batch-toggle', { ids, enabled }).then((r) => r.data),
  batchTest: (ids: number[]) =>
    client.post<BatchResult>('/servers/batch-test', { ids }).then((r) => r.data),
}

// ---------- 用户（admin） ----------
export const userApi = {
  list: () => client.get<User[] | null>('/users').then((r) => listOf(r.data)),
  create: (data: { username: string; password: string; name: string; email: string; role: 'admin' | 'user' }) =>
    client.post<User>('/users', data).then((r) => r.data),
  update: (id: number, data: { username: string; name: string; email: string; role: 'admin' | 'user'; enabled?: boolean }) =>
    client.put<User>(`/users/${id}`, data).then((r) => r.data),
  delete: (id: number) => client.delete<{ message: string }>(`/users/${id}`).then((r) => r.data),
  batchDelete: (ids: number[]) =>
    client.post<BatchResult>('/users/batch-delete', { ids }).then((r) => r.data),
  batchToggle: (ids: number[], enabled: boolean) =>
    client.post<BatchResult>('/users/batch-toggle', { ids, enabled }).then((r) => r.data),
  resetPassword: (id: number, password: string) =>
    client.put<{ message: string }>(`/users/${id}/reset-password`, { password }).then((r) => r.data),
  toggle: (id: number) => client.put<{ enabled: boolean }>(`/users/${id}/toggle`).then((r) => r.data),
  listLdap: () => client.get<LdapUser[] | null>('/users/ldap').then((r) => listOf(r.data)),
  importLdap: (users: LdapUser[]) =>
    client.post<LdapImportResult>('/users/ldap/import', { users }).then((r) => r.data),
  /** 解锁被锁定的用户 */
  unlockUser: (id: number) => client.put<{ message: string }>(`/users/${id}/unlock`).then((r) => r.data),
  /** 强制下线在线用户（用户不在线返回 400） */
  kickUser: (id: number) => client.post<{ message: string }>(`/users/${id}/kick`).then((r) => r.data),
  batchUnlockUsers: (ids: number[]) =>
    client.post<BatchResult>('/users/batch-unlock', { ids }).then((r) => r.data),
  batchKickUsers: (ids: number[]) =>
    client.post<BatchResult>('/users/batch-kick', { ids }).then((r) => r.data),
}

// ---------- 日志 ----------
export const logApi = {
  list: (params: LogQuery) => client.get<LogPage>('/logs', { params }).then((r) => r.data),
  getDetail: (id: number): Promise<OperationLog | undefined> =>
    // 日志无单条接口：详情从分页数据中取，这里保留方法占位便于页面组装
    Promise.resolve(undefined),
}

export { extractErrorMessage } from './client'
export * from './types'
