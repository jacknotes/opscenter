# 用户锁定与强制下线功能设计

日期：2026-06-12

## 概述

两个功能：
1. **用户级登录锁定** — 连续登录失败 N 次后锁定账号，管理员可手动解锁
2. **强制下线** — 管理员可将在线用户踢下线，禁用用户时自动触发

## 数据模型变更

### User 模型新增字段

`internal/model/user.go`：

```go
FailedAttempts int  `json:"failed_attempts" gorm:"default:0"`  // 连续登录失败次数
Locked         bool `json:"locked" gorm:"default:false"`       // 是否被锁定
```

GORM AutoMigrate 自动添加列。

### ActiveUserInfo 新增字段

`internal/middleware/auth.go`：

```go
type ActiveUserInfo struct {
    Role        string `json:"role"`
    LoginTime   string `json:"login_time"`
    LoginMethod string `json:"login_method"`
    LastActive  string `json:"last_active"`
    JTI         string `json:"jti"`  // 新增：用于强制下线时作废 token
}
```

### 配置变更

`internal/config/config.go` 的 `AuthConfig` 新增：

```go
MaxUserAttempts int `yaml:"max_user_attempts"` // 用户级锁定阈值，默认 5
```

`config.yaml.example` 对应更新。

## 登录流程变更

`internal/handler/auth.go` 的 `Login` 方法修改：

```
用户登录
  ├─ IP 限流检查（保留现有逻辑不变）
  ├─ 认证（LDAP 或本地密码）
  │   ├─ 成功：
  │   │   ├─ 检查 locked 字段 → locked=true 时返回 403 "账号已锁定，请联系管理员解锁"
  │   │   ├─ 重置 failed_attempts=0
  │   │   └─ 正常登录流程（生成 JWT、TrackActiveUser 含 jti）
  │   └─ 失败：
  │       ├─ 仅当用户名存在于数据库时递增 failed_attempts
  │       ├─ 如果 failed_attempts >= MaxUserAttempts → 设 locked=true
  │       └─ 返回登录失败
  └─ 返回结果
```

关键规则：
- **admin 账号不参与锁定**：登录失败时不递增 admin 的 `failed_attempts`
- **成功登录自动重置**：`failed_attempts` 归零
- **不存在的用户名**：不递增任何用户的 `failed_attempts`（防止通过枚举用户名锁定他人）

## 强制下线机制

### 工作原理

1. **登录时**：`TrackActiveUser` 写入的 `ActiveUserInfo` 包含当前 token 的 `jti`
2. **强制下线时**（管理员操作）：
   - 从 Redis 读取 `opscenter:active_user:<username>`，取出 `jti`
   - 调用 `BlacklistToken(jti)` 将 token 加入黑名单
   - 调用 `UntrackActiveUser(username)` 清除在线状态
   - 用户下一个请求被 `Auth()` 中间件拒绝（"token revoked"）
3. **禁用用户时自动下线**：`ToggleUserEnabled` 和 `BatchToggleUsers` 中，禁用操作会自动检查在线状态并执行强制下线

### WebSocket 处理

- 已有 WebSocket 连接不会被立即断开（长连接不经过 HTTP 中间件）
- WebSocket 需要重新认证时，被拉黑的 token 会被拒绝
- 这是可接受的妥协——强制断开 WebSocket 需要额外的连接管理机制

## API 端点

### 新增端点

| 方法 | 路径 | 说明 | 权限 |
|---|---|---|---|
| PUT | `/api/users/:id/unlock` | 解锁用户（重置 failed_attempts=0, locked=false） | admin |
| POST | `/api/users/:id/kick` | 强制下线（作废 token + 清除在线状态） | admin |
| POST | `/api/users/batch-unlock` | 批量解锁 | admin |

### 变更端点

| 方法 | 路径 | 变更 |
|---|---|---|
| PUT | `/api/users/:id/toggle` | 禁用时自动执行强制下线逻辑 |
| POST | `/api/users/batch-toggle` | 禁用时自动执行强制下线逻辑 |
| GET | `/api/users` | 响应包含 `locked`、`failed_attempts` 字段 |
| POST | `/login` | 失败时递增 `failed_attempts`，成功时重置；被锁定时返回 403 |

### 业务规则

- admin 账号不能被锁定、不能被强制下线
- 不能对自己执行强制下线
- 解锁和强制下线操作写入审计日志

## 前端变更

### UserManage.vue

1. **新增列**：
   - 锁定状态：Tag 组件，locked=true 红色"已锁定"，否则绿色"正常"
   - 连续失败次数：显示数字

2. **操作列新增按钮**：
   - 解锁按钮：仅当 `locked=true` 时显示，确认对话框后调用 API
   - 强制下线按钮：仅当用户在线时显示，确认对话框后调用 API

3. **批量操作**：
   - 新增「批量解锁」按钮

4. **在线状态**：
   - 后端在用户列表接口中返回在线状态（基于 Redis active_user 数据）

### API 层 (`web/src/api/user.js`)

```js
export const unlockUser = (id) => http.put(`/users/${id}/unlock`)
export const kickUser = (id) => http.post(`/users/${id}/kick`)
export const batchUnlockUsers = (ids) => http.post('/users/batch-unlock', { ids })
```

## 审计日志

- 解锁操作：`"解锁用户 <username>"`
- 强制下线：`"强制下线用户 <username>"`
- 批量解锁：`"批量解锁用户 <count> 个"`
- 禁用时自动下线：复用现有禁用审计日志，不额外记录

## 涉及文件

| 文件 | 变更类型 |
|---|---|
| `internal/model/user.go` | 新增字段 |
| `internal/config/config.go` | 新增配置项 |
| `config.yaml.example` | 新增配置示例 |
| `internal/middleware/auth.go` | ActiveUserInfo 新增 JTI 字段 |
| `internal/handler/auth.go` | 登录流程变更、新增解锁/下线/批量解锁 handler |
| `internal/router/router.go` | 注册新路由 |
| `web/src/api/user.js` | 新增 API 函数 |
| `web/src/views/UserManage.vue` | UI 变更 |
