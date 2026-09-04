# OpsCenter 后端 API 契约文档（前端 v2 重构参考）

> 依据代码生成，源码版本：`internal/handler/*`、`internal/router/router.go`、`internal/model/*`、`internal/middleware/*`、`internal/service/*`。
> 每个字段名均有代码依据（Gin handler 的 `json` tag / `gin.H` 字面量）。

---

## 目录

1. [通用约定](#1-通用约定)
2. [认证机制](#2-认证机制)
3. [Dashboard（6 个接口）](#3-dashboard)
4. [LVS](#4-lvs)
5. [K8s](#5-k8s)
6. [预生产 Preprod](#6-预生产-preprod)
7. [Nginx](#7-nginx)
8. [服务器管理 Servers](#8-服务器管理-servers)
9. [用户管理 Users](#9-用户管理-users)
10. [操作日志 Logs](#10-操作日志-logs)
11. [预览 → 执行通用流程](#11-预览--执行通用流程)
12. [WebSocket 协议 /api/ws/exec](#12-websocket-协议-apiwsexec)
13. [数据模型](#13-数据模型)
14. [易踩坑清单](#14-易踩坑清单)

---

## 1. 通用约定

### 1.1 基础信息

| 项目 | 值 |
|---|---|
| API 前缀 | `/api` |
| Content-Type | `application/json` |
| 认证方式 | `Authorization: Bearer <token>`（HTTP）或 `?token=<token>`（WebSocket 及 Auth 中间件兜底） |
| 后端默认端口 | 18080 |
| CORS | `allowed_origins` 为空数组时允许所有来源；`Access-Control-Expose-Headers: X-Warning` |
| 文档 | Swagger UI 挂载于 `/swagger/*any` |

### 1.2 响应包装（重要：没有 {code, message, data} 包装）

**后端没有统一的响应包装层。** 实际惯例如下（来源：各 handler 的 `c.JSON` 调用）：

- **成功**：直接返回业务数据本身（对象 / 数组 / 顶层字段），HTTP 状态码 200（创建类为 201）。
- **失败**：返回 `{"error": "中文错误信息"}`，HTTP 状态码即错误码（400/401/403/404/409/429/500）。部分失败响应会附加额外字段，如 LVS 执行失败返回 `{"error": "...", "output": "..."}`。
- **唯一有分页包装的接口**：`GET /api/logs`，返回 `{total, page, size, data}`。
- **错误码约定**：

| 状态码 | 场景 | 响应体 |
|---|---|---|
| 400 | 参数错误 / 校验失败 / 预览过期 / IP 非法等 | `{"error": "..."}` |
| 401 | 未提供 token / token 无效 / token 已撤销 / 用户不存在 | `{"error": "未提供认证令牌" \| "无效的认证令牌" \| "认证令牌已被撤销" \| "用户不存在"}` |
| 403 | 非 admin 访问 admin 接口、账户被禁用 | `{"error": "需要管理员权限" \| "账户已被禁用"}` |
| 404 | 资源不存在（服务器/用户等） | `{"error": "..."}` |
| 409 | Preprod 执行时分布式锁冲突 | `{"error": "操作正在进行中，请等待 (当前操作人: xxx)"}` |
| 429 | 登录限流触发 | `{"error": "登录尝试过于频繁，请稍后再试"}` |
| 500 | 执行失败 / 查询失败 | `{"error": "...", "output": ...}`（视接口而定） |

- **特例 1**：`GET /api/lvs/list` 在 SSH 失败时仍返回 **200 + 空数组**，警告信息放在响应头 `X-Warning`（URL 转义后的中文，前端需 `decodeURIComponent`）。CORS 已 Expose 该头。
- **特例 2**：`POST /api/servers/:id/test` 连接失败也返回 **200**，用 `{"success": false, "error": "..."}` 区分。

### 1.3 认证中间件行为（`middleware/auth.go`）

- `Auth()`：从 `Authorization` Header（去掉 `Bearer ` 前缀）取 token；Header 为空则从 URL query `token` 取。二者都空返回 401。
- JWT 使用 HS256 签名，Claims 包含 `user_id`、`username`、`role`、`jti`（UUID）、`exp`。过期时间由 `config.yaml` 的 `jwt.expire` 决定（示例配置 `24h`）。
- Logout 会将 token 的 `jti` 写入 Redis 黑名单（TTL = JWT 过期时长），此后该 token 返回 401 `认证令牌已被撤销`。
- `UserEnabledCheck()`：每个受保护请求都会查库校验用户存在且 `enabled=true`，否则 401/403。
- `AdminRequired()`：查库校验 `role == "admin"`，否则 403 `{"error": "需要管理员权限"}`。

### 1.4 路由权限分组（`router.go`）

| 分组 | 中间件 |
|---|---|
| 公开 | `GET /api/health`、`POST /api/login` |
| 受保护（普通用户即可） | ws/exec、user/info、logout、logs、dashboard 全部、lvs 读写列表类、lvs op/swap/check、k8s 全部、preprod 全部、nginx 全部、`GET /servers`、`GET /servers/:id`、`PUT /users/:id/password` |
| 仅 admin | lvs tags/vs_tags/bindings 的 PUT/DELETE、servers 全部管理接口（POST/PUT/DELETE/test/batch）、users 全部管理接口（含 ldap） |

---

## 2. 认证机制

### 2.1 登录 `POST /api/login`（公开，无需 token）

请求体（`LoginRequest`）：

```json
{
  "username": "admin",     // string, 必填
  "password": "Admin@123"  // string, 必填
}
```

成功响应 **200**（`LoginResponse`，直接返回，无包装）：

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "username": "admin",
    "name": "管理员",
    "email": "admin@example.com",
    "role": "admin",
    "enabled": true,
    "auth_source": "local",
    "created_at": "2024-01-01T00:00:00+08:00",
    "updated_at": "2024-01-01T00:00:00+08:00"
  }
}
```

- `user` 即 `model.User` 的 JSON（见 13.1）；`password`、`ldap_dn`、`deleted_at` 带 `json:"-"` 不会返回。
- **登录限流**：同 IP 失败次数达 `auth.max_login_attempts`（默认 10）后在 `auth.login_lock_duration`（默认 1m）内返回 429。
- **LDAP**：`ldap.enabled=true` 且用户名非 admin 时优先走 LDAP；`auth_source=ldap` 的用户不能使用本地密码登录（401 `LDAP 用户请使用域账号登录`）；未导入的 LDAP 用户返回 401 `用户未授权，请联系管理员导入 LDAP 账户`。
- 失败：400 `{"error":"参数错误"}`；401 `{"error":"用户名或密码错误"}` / `{"error":"账户已被禁用，请联系管理员"}` 等。

### 2.2 获取当前用户信息 `GET /api/user/info`

成功 **200**：直接返回 `model.User` 对象（字段同上表 `user`）。
失败：401 `{"error":"未认证"}`；404 `{"error":"用户不存在"}`。

### 2.3 登出 `POST /api/logout`

成功 **200**：

```json
{ "message": "登出成功" }
```

同时将当前 token 加入黑名单（前端应清空本地存储并跳转登录页）。

### 2.4 Token 传递方式汇总

| 场景 | 方式 |
|---|---|
| 普通 HTTP 请求 | `Authorization: Bearer <token>`（Auth 中间件解析，带不带 Bearer 前缀都会 TrimPrefix，但规范写法带前缀） |
| WebSocket 升级请求 | URL query：`/api/ws/exec?token=<token>`（受保护路由先过 Auth 中间件）；另外 WS 首条消息体内也可带 `token` 字段（见第 12 节，消息内 token 优先） |

---

## 3. Dashboard

全部为 `GET`，需登录。`granularity` 取值均为 `day`（默认）/`week`/`month`/`year`。

### 3.1 `GET /api/dashboard/stats`

MySQL 聚合统计。**响应因角色而异**（代码依据：`role == "admin"` 分支）：

```json
{
  "servers": {
    "total": 10,           // int64，全部服务器数（含禁用）
    "enabled": 8,          // int64
    "disabled": 2,         // int64
    "by_type": { "lvs": 2, "nginx": 3, "kubernetes": 4, "preprod": 1 },  // map[string]int64，仅统计 enabled=true
    "by_env":  { "prod": 5, "pre": 3 }   // map[string]int64，排除空 env，仅 enabled=true
  },
  "users": {               // 仅 admin 角色返回；普通用户无此键
    "total": 5, "enabled": 4, "disabled": 1,
    "by_role": { "admin": 1, "user": 4 }
  },
  "online_users": 3        // int64，仅 admin 返回；Redis 活跃用户数
}
```

### 3.2 `GET /api/dashboard/remote-stats`

通过 SSH 并行查询各类型（`lvs`/`nginx`/`kubernetes`/`preprod`）已启用服务器，整体超时 60s（超时也可能返回部分结果）。**任一模块查询失败时对应字段为 `null`**：

```json
{
  "lvs":     { "vs_count": 12, "rs_online": 30, "rs_offline": 5, "total_active_conn": 1234, "total_inact_conn": 56 },
  "nginx":   { "upstream_count": 8, "server_online": 20, "server_offline": 4 },
  "k8s":     { "total_rollouts": 15, "by_namespace": { "default": 8, "ops": 7 }, "pending": 2, "online": 9 },
  "preprod": { "total_resources": 40, "scaled_down": 10, "expanded": 2, "normal": 28 }
}
```

注意：k8s 的 `pending` = step=="1/5" 的数量，`online` = step=="3/5" 的数量。

### 3.3 `GET /api/dashboard/activity-stats`

Query：`granularity`（默认 day）、`action_granularity`（可选，操作动作统计独立粒度，缺省同 granularity）。

- 时间范围按 granularity 固定：day→近 30 天、week→近 12 周、month→近 365 天、year→近 5 年；action 统计则从当前自然周期起点（本周一/本月 1 日/今年 1 月 1 日/今天 0 点）起算。

```json
{
  "deploy_stats": [ { "period": "2025-01-01", "module": "lvs", "count": 3 } ],
  "login_stats":  [ { "period": "2025-01-01", "status": "success", "count": 10 } ],
  "action_stats": [ { "module": "k8s", "action": "online", "count": 7 } ]
}
```

- 模块范围：lvs/nginx/k8s/preprod。
- **`login_stats` 仅 admin 返回真实数据；普通用户恒为空数组 `[]`**。

### 3.4 `GET /api/dashboard/k8s-project-stats`

Query：`granularity`、`server_name`（可选，按服务器名过滤）。

```json
{
  "summary": { "total": 20, "success": 18, "failed": 2, "full_ops": 3 },
  "trend":      [ { "period": "2025-01", "project": "user-service", "count": 4 } ],
  "by_project": [ { "project": "user-service", "count": 12, "success": 11, "failed": 1 } ],
  "by_action":  [ { "action": "online", "count": 15 } ]
}
```

- `project_names` 为 `*` 或空的日志视为"全量操作"，只计入 `summary.total/success/failed` 和 `summary.full_ops`，不进入 trend/by_project。

### 3.5 `GET /api/dashboard/preprod-project-stats`

Query 与响应结构同 3.4，数据源为 `module=preprod` 的日志。

### 3.6 `GET /api/dashboard/lvs-conn-stats`

Query：`server_id`（必填）、`vs_ip`（必填）、`rs_ip`（必填）、`duration`（可选，分钟，仅接受 `5/15/30/60`，默认 15；其他值静默回退 15）。

```json
{
  "data": [ { "collected_at": "2025-01-01T10:00:00+08:00", "active_conn": 120, "inact_conn": 30 } ]
}
```

- 数据来自后台采集器（默认 30s 一次写入 `lvs_conn_stats` 表），按 `collected_at` 分组求和后升序返回。

---

## 4. LVS

### 4.1 `GET /api/lvs/list?server_id=` （必填）

返回 `service.VirtualServer` 数组（已合并 status 下线 RS、注入 RS/VS 标签、检测主备角色）：

```json
[
  {
    "ip": "10.0.0.100",          // VS IP；本机未绑定（FWM 泛条目）时可能为 "0.0.0.0"
    "port": "443",
    "protocol": "TCP",
    "scheduler": "rr",
    "flags": "FWM 100",
    "role": "master",            // omitempty；"master" 或 "backup"
    "tag": "web",                // omitempty；VS 标签（lvs_vs_tags 表）
    "real_servers": [
      {
        "ip": "10.0.0.1",
        "port": "443",
        "forward": "Route",
        "weight": 1,
        "active_conn": 15,        // 离线补充的 RS 这三个字段为 0
        "inact_conn": 2,
        "status": "up",           // "up" | "down"
        "tag": "prod",            // omitempty；RS 标签
        "disabled": false,        // omitempty；是否被标记禁用
        "disabled_reason": ""     // omitempty
      }
    ]
  }
]
```

**易踩坑**：SSH 执行失败时返回 **200 + `[]`**，警告在响应头 `X-Warning`（`url.PathEscape` 过，需解码）。

### 4.2 `GET /api/lvs/status?server_id=`

```json
{
  "output": "原始脚本 status 输出文本",
  "groups": [
    { "vs_ip": "10.0.0.100", "vs_port": "443", "real_servers": [ { "ip": "10.0.0.1", "port": "443", "status": "up" } ] }
  ]
}
```

### 4.3 `GET /api/lvs/tags`

Query（均可选）：`rs_ips`（逗号分隔）、`vs_ip`。返回 `LvsRSTag` 数组（见 13.4）。

### 4.4 `GET /api/lvs/vs_tags`

Query（可选）：`vs_ips`（逗号分隔）。返回 `LvsVSTag` 数组（见 13.5）。

### 4.5 `GET /api/lvs/bindings`

Query（可选）：`preprod_server_id`（int，格式错误返回 400）。返回 `LvsPreprodBinding` 数组（见 13.6）。

### 4.6 `POST /api/lvs/op/preview`

请求（`LVSOpRequest`）：

```json
{
  "server_id": 1,          // uint, 必填
  "vs_ip": "10.0.0.100",   // string, 必填, 校验 IP 格式
  "rs_ip": "10.0.0.1",     // string, 必填, 校验 IP 格式
  "state": "on"            // string, 必填, 仅允许 "on" | "off"
}
```

响应 **200**：

```json
{
  "preview_id": "uuid-v4",
  "current_status": "ipvsadm list 原始输出（失败时为空）",
  "command": "IP_PREFIX=10.0.0. /path/lvs.sh op 100 1 on",
  "description": "将 10.0.0.1 从 10.0.0.100 的后端上线"
}
```

校验失败：400 `{"error":"IP格式错误"}` / `{"error":"状态必须是 on 或 off"}` / `{"error":"RS 10.0.0.1 已被禁用: xxx"}`；404 `{"error":"服务器不存在"}`。

### 4.7 `POST /api/lvs/op/execute`

请求：`{ "preview_id": "uuid-v4" }`（必填）。
成功 **200**：`{ "output": "命令输出文本", "status": "success" }`。
失败：400 `{"error":"预览已过期或不存在"}` / `{"error":"预览类型不匹配"}`；500 `{"error":"执行失败: ...", "output": "..."}`（**执行失败时预览记录不删除，可重试**）。

### 4.8 `POST /api/lvs/swap/preview`

请求（`LVSSwapRequest`）：

```json
{ "server_id": 1, "vs_ip": "10.0.0.100", "rs_ip1": "10.0.0.1", "rs_ip2": "10.0.0.2" }
```

响应结构同 4.6，`description` 形如 `"切换 10.0.0.1 和 10.0.0.2 在 10.0.0.100 的状态"`。两个 RS 任一被禁用即 400。

### 4.9 `POST /api/lvs/swap/execute`

同 4.7，成功 `{ "output": "...", "status": "success" }`。

### 4.10 `POST /api/lvs/check/scaledown`（普通用户）

> 注意：路由在 `/api/lvs/check/scaledown`（router.go），swagger 注解里写的路径 `/preprod/check/lvs_scaledown` 与实际路由不一致，以 router 为准。

请求：`{ "preprod_server_id": 5 }`（uint，必填）。

无绑定风险时：`{ "need_warning": false }`
有绑定且对应 RS 在线时：**只返回第一条命中**（代码里 `done` 置位后停止）：

```json
{
  "need_warning": true,
  "warnings": [
    { "vs_tag": "web", "rs_env_tag": "prod", "rs_ip": "10.0.0.1", "status": "up", "lvs_server": "lvs-master-01" }
  ]
}
```

### 4.11 Admin 标签/绑定管理

| 方法/路径 | 请求体 / 参数 | 成功响应 |
|---|---|---|
| `PUT /api/lvs/tags` | `{ "rs_ip": "10.0.0.1", "vs_ip": "10.0.0.100", "tag": "prod", "disabled": false, "disabled_reason": "" }`（rs_ip/vs_ip 必填且校验 IP；`disabled=true` 时 `disabled_reason` 必填） | `{"message": "标签已保存"}`（upsert，键为 rs_ip+vs_ip） |
| `DELETE /api/lvs/tags/:vs_ip/:rs_ip` | 路径参数 | `{"message": "标签已删除"}`；无记录 404 `{"error":"标签不存在"}` |
| `PUT /api/lvs/vs_tags` | `{ "vs_ip": "10.0.0.100", "tag": "web" }`（vs_ip 必填） | `{"message": "VS标签已保存"}`（upsert，键为 vs_ip） |
| `DELETE /api/lvs/vs_tags/:vs_ip` | 路径参数 | `{"message": "VS标签已删除"}`；无记录 404 |
| `PUT /api/lvs/bindings` | `{ "vs_tag": "web", "rs_env_tag": "prod", "preprod_server_id": 5 }`（三个均必填） | `{"message": "绑定关系已保存"}`（upsert，键为 vs_tag+rs_env_tag） |
| `DELETE /api/lvs/bindings/:id` | 路径参数（uint） | `{"message": "绑定关系已删除"}`；无记录 404 |

---

## 5. K8s

### 5.1 `GET /api/k8s/rollouts?server_id=`

返回 `service.Rollout` 数组：

```json
[
  {
    "namespace": "default",
    "name": "user-service",
    "strategy": "canary",
    "status": "Paused",
    "step": "2/5",
    "set_weight": "20",
    "ready": "2/10",
    "desired": 10,
    "up_to_date": 2,
    "available": 10
  }
]
```

### 5.2 批量操作预览（3 组）

| 路径 | 请求体（`K8sBatchRequest`） |
|---|---|
| `POST /api/k8s/online/preview` | `{ "server_id": 3, "projects": [ { "name": "user-service", "namespace": "default" } ] }` |
| `POST /api/k8s/sync/preview` | 同上 |
| `POST /api/k8s/rollback/preview` | 同上 |

- `server_id`、`projects` 必填；`name`/`namespace` 会做非法字符校验（400 `项目名称 [x] 包含非法字符`）。

响应 **200**：

```json
{
  "preview_id": "uuid-v4",
  "current_status": "rollout list 原始输出（失败时为空）",
  "commands": [ "/path/rollouts.sh single_online user-service default" ],
  "description": "上线 1 个项目的 canary 版本"
}
```

### 5.3 全量操作预览（3 组）

| 路径 | 请求体（`K8sFullRequest`） |
|---|---|
| `POST /api/k8s/full_online/preview` | `{ "server_id": 3 }` |
| `POST /api/k8s/full_sync/preview` | 同上 |
| `POST /api/k8s/full_rollback/preview` | 同上 |

响应 **200**（commands 恒为单元素数组）：

```json
{
  "preview_id": "uuid-v4",
  "current_status": "...",
  "commands": [ "/path/rollouts.sh full_online" ],
  "description": "全量上线所有 paused 状态的 rollout (step 1/5 → promote)"
}
```

### 5.4 执行（6 个 execute，请求体统一）

`POST /api/k8s/{online|sync|rollback|full_online|full_sync|full_rollback}/execute`

请求：`{ "preview_id": "uuid-v4" }`

成功 **200**（**注意 output 是数组**）：

```json
{ "output": [ "命令输出1", "命令输出2" ], "status": "success" }
```

失败：
- 400 `{"error":"预览已过期或不存在"}` / `{"error":"预览类型不匹配"}`（只校验 module=="k8s"，**不校验 action**，即 online 预览可以拿到 sync execute 执行）/ `{"error":"预览命令为空"}`
- 404 `{"error":"服务器不存在"}`
- 500 `{"error":"执行失败: ...", "output": ["..."]}`（数组；失败即中断后续命令）

### 5.5 Preprod 的 LVS 在线检查（挂在 preprod 域）

见 6.4。

---

## 6. 预生产 Preprod

### 6.1 `GET /api/preprod/status?server_id=`

返回 `service.PreprodResource` 数组（已合并 `list-targets` 的目标副本数）：

```json
[
  {
    "category": "rollout",      // "rollout" | "deployment" | "statefulset"
    "name": "user-service",
    "desired": 10,
    "current": 2,
    "up_to_date": 2,
    "available": 2,
    "age": "5d",
    "target_replicas": 10        // 来自 list-targets；未匹配到时为 0
  }
]
```

### 6.2 缩容/扩容预览

| 路径 | 请求体（`PreprodScaleRequest`） |
|---|---|
| `POST /api/preprod/scaledown/preview` | `{ "server_id": 5, "resource_names": ["user-service"] }` |
| `POST /api/preprod/scaleup/preview` | 同上 |

- `resource_names` **可选**：为空/缺省时操作所有资源（全量）；非空时逐个校验非法字符。
- 响应 **200**：

```json
{
  "preview_id": "uuid-v4",
  "current_status": "list 原始输出",
  "command": "/path/specific-project-scale.sh scaledown user-service",
  "description": "缩容选中的 user-service 副本数至 0"
}
```

### 6.3 缩容/扩容执行（HTTP）

`POST /api/preprod/scaledown/execute` / `POST /api/preprod/scaleup/execute`

请求：`{ "preview_id": "uuid-v4" }`

- 成功 **200**：`{ "output": "命令输出", "status": "success" }`
- 400 预览过期/不匹配/命令为空；404 服务器不存在
- **409 锁冲突**：`{ "error": "操作正在进行中，请等待 (当前操作人: xxx)" }`（Redis 分布式锁，默认 10 分钟）
- 500 `{"error":"执行失败: ...", "output": "..."}`

> **前端注意**：实际产品流程中 preprod 执行走 **WebSocket（第 12 节）** 流式输出；HTTP execute 接口功能等价但无流式。

### 6.4 `POST /api/preprod/check/lvs_online`

请求（`CheckLvsOnlineRequest`）：`{ "vs_ip": "10.0.0.100", "rs_ip": "10.0.0.1" }`（均必填）

用于 LVS 上线前检查预生产资源副本是否正常：

```json
{
  "need_warning": true,
  "warnings": [ { "name": "user-service", "category": "rollout", "current": 2, "target": 10 } ],
  "vs_tag": "web",
  "rs_env_tag": "prod"
}
```

任意环节查不到（无 VS 标签/无 RS 标签/无绑定/查询失败）均返回 `{ "need_warning": false }`。无警告时也只有 `need_warning: false`。

---

## 7. Nginx

### 7.1 `GET /api/nginx/configs?server_id=`

返回文件名数组（按 `config_pattern` 的 include/`!`exclude 模式过滤，默认 `*.conf`）：

```json
[ "app-backend.conf", "app-frontend.conf" ]
```

### 7.2 `GET /api/nginx/upstreams?server_id=&config_file=`

```json
{
  "upstreams": [
    {
      "name": "app_backend",
      "servers": [
        { "ip": "10.0.0.1", "port": "80", "status": "up", "weight": "10" },
        { "ip": "10.0.0.2", "port": "8080", "status": "down", "weight": "" }
      ],
      "config": "upstream app_backend {...}"   // 该 upstream 块的原文
    }
  ],
  "raw": "整个配置文件的原文"
}
```

- `status`：未注释的 `server` 行为 `"up"`，`#server` 注释行为 `"down"`；port 未写时为 `"80"`。
- `config_file` 会做路径穿越/注入校验（400 `非法的配置文件名: xxx`）。

### 7.3 上线/下线预览

`POST /api/nginx/upstream/online/preview` / `POST /api/nginx/upstream/offline/preview`

请求（`NginxUpstreamRequest`）：

```json
{
  "server_id": 4,                       // uint, 必填
  "config_file": "app-backend.conf",    // string, 必填
  "upstream_names": ["app_backend"],    // []string, 必填, 逐个校验非法字符
  "backend_ip": "10.0.0.1:80,10.0.0.2"  // string, 必填, 支持逗号分隔多个 IP；":80" 会被剥离
}
```

响应 **200**：

```json
{
  "preview_id": "uuid-v4",
  "before": "修改前完整配置",
  "after": "修改后完整配置",
  "line_diffs": [
    { "line_num": 5, "type": "same", "content": "upstream app_backend {" },
    { "line_num": 6, "type": "removed", "content": "    #server 10.0.0.1:80;" },
    { "line_num": 6, "type": "added", "content": "    server 10.0.0.1:80;" }
  ],
  "description": "将 10.0.0.1:80,10.0.0.2 在 [app_backend] 中上线（去掉注释）"
}
```

- `line_diffs[].type`: `"same" | "added" | "removed"`。下线（offline）时 `removed` 是原行、`added` 是注释行。
- offline 预览校验"至少保留一台在线"：400 `禁止操作：upstream [x] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`。

### 7.4 切换预览

`POST /api/nginx/upstream/swap/preview`

请求（`NginxSwapRequest`）：

```json
{ "server_id": 4, "config_file": "app.conf", "upstream_names": ["app_backend"], "offline_ip": "10.0.0.1", "online_ip": "10.0.0.2" }
```

响应结构同 7.3。校验失败 400：upstream 不存在 / offline 服务器不在在线状态 / online 服务器不在离线状态 / 未找到对应服务器。

### 7.5 状态反转预览

`POST /api/nginx/upstream/toggle/preview`

请求（`NginxToggleRequest`）：

```json
{ "server_id": 4, "config_file": "app.conf", "upstream_names": ["app_backend"] }
```

- 校验每个 upstream 必须同时存在 up 和 down 的 server，否则 400 `upstream [x] 中所有服务器状态相同，无需切换`。
- 响应结构同 7.3，`description` 形如 `"切换 [app_backend] 中所有服务器状态"`。

### 7.6 批量操作预览

`POST /api/nginx/upstream/batch/preview`

请求（`NginxBatchRequestV2`）：

```json
{
  "server_id": 4,
  "config_file": "app.conf",
  "items": [
    { "upstream_name": "app_backend", "action": "offline", "backend_ip": "10.0.0.1" },
    { "upstream_name": "app_backend", "action": "online",  "backend_ip": "10.0.0.2" },
    { "upstream_name": "app_frontend", "action": "toggle" }
  ]
}
```

- `items[].action`: `"online" | "offline" | "toggle"`；`backend_ip` 在 online/offline 时需要，toggle 时忽略。
- 按 items 顺序**累积**应用 diff（前一 item 的 after 是下一个的 before），并做累积下线校验。
- 响应结构同 7.3，`description` 形如 `"批量操作：[app_backend] 10.0.0.1 下线；[app_backend] 10.0.0.2 上线；[app_frontend] 切换所有状态"`。

### 7.7 回滚预览

`POST /api/nginx/rollback/preview`

请求（`NginxRollbackRequest`）：

```json
{ "server_id": 4, "config_file": "app.conf", "backup_file": "app.conf.bak.20250101120000" }
```

响应 **200**（**没有 line_diffs**）：

```json
{
  "preview_id": "uuid-v4",
  "before": "当前配置",
  "after": "备份配置内容",
  "description": "回滚 app.conf 到备份 app.conf.bak.20250101120000"
}
```

### 7.8 执行（6 个 execute，请求体统一 `{ "preview_id": "..." }`）

| 路径 | 成功 200 响应 |
|---|---|
| `/api/nginx/upstream/online/execute` | `{ "message": "online成功", "output": "成功将 10.0.0.1 在 [app_backend] 中上线" }` |
| `/api/nginx/upstream/offline/execute` | `{ "message": "offline成功", "output": "..." }` |
| `/api/nginx/upstream/swap/execute` | `{ "message": "切换成功", "output": "切换成功: [app_backend] 10.0.0.1 下线 → 10.0.0.2 上线" }` |
| `/api/nginx/upstream/toggle/execute` | `{ "message": "切换成功", "output": "切换成功: [app_backend] 中所有服务器状态已反转" }` |
| `/api/nginx/upstream/batch/execute` | `{ "message": "批量操作成功", "output": "批量操作成功：1 个上线，1 个下线，0 个切换" }` |
| `/api/nginx/rollback/execute` | `{ "message": "回滚成功" }`（**无 output 字段**） |

执行类接口统一流程：先备份配置（`mkdir -p` + 带时间戳 cp，自动清理超量备份，默认保留 `nginx.max_backups` 10 份）→ sed 修改 → `nginx -t` → 失败则自动回滚并返回 **400** `{"error":"配置语法错误，已自动回滚到备份: ..."}` → 成功则 `systemctl reload nginx`。

### 7.9 `GET /api/nginx/backups?server_id=`

返回备份文件名数组（`ls -t` 输出按时间倒序）：

```json
[ "app.conf.bak.20250101120000", "app.conf.bak.20250101100000" ]
```

---

## 8. 服务器管理 Servers

### 8.1 列表与详情（普通用户可用）

- `GET /api/servers?type=lvs&all=true` → `[ServerResponse]` 数组
  - `type`：按 server_type 过滤（lvs/nginx/kubernetes/preprod 等）；`all=true` 时包含禁用服务器（默认只返回 enabled=true）。
- `GET /api/servers/:id` → 单个 `ServerResponse`。

`ServerResponse`（脱敏，见 13.2）：

```json
{
  "id": 1, "name": "lvs-master-01", "host": "10.0.0.10", "port": 22,
  "username": "root", "auth_type": "password",
  "has_password": true, "has_private_key": false, "has_script_password": true,
  "server_type": "lvs", "env": "prod", "script_path": "/opt/scripts/lvs.sh",
  "config_path": "", "config_pattern": "", "backup_path": "",
  "description": "", "enabled": true,
  "created_at": "2024-01-01T00:00:00+08:00", "updated_at": "2024-01-01T00:00:00+08:00"
}
```

### 8.2 Admin 管理接口

| 方法/路径 | 请求体 | 成功响应 |
|---|---|---|
| `GET /api/servers/:id/edit` | - | 完整字段对象（见下） |
| `POST /api/servers` | `model.Server` 全字段（见 13.2 的 Server 字段，含 password/private_key/script_password） | **201** + `ServerResponse` |
| `PUT /api/servers/:id` | 同上；**敏感字段传字符串 `"__keep__"` 表示保留原值** | `ServerResponse` |
| `DELETE /api/servers/:id` | - | `{"message": "删除成功"}`（软删除） |
| `PUT /api/servers/:id/toggle` | - | `{"message": "已启用"\|"已禁用", "enabled": true\|false}` |
| `POST /api/servers/:id/test` | - | 见下（**失败也 200**） |
| `POST /api/servers/batch-delete` | `{ "ids": [1,2] }`（必填，非空） | `{"message": "批量删除完成: 成功 1, 失败 0\n失败: ...", "deleted": 1, "failed": 0}` |
| `POST /api/servers/batch-toggle` | `{ "ids": [1,2], "enabled": false }` | `{"message": "...", "updated": 2, "failed": 0}` |
| `POST /api/servers/batch-test` | `{ "ids": [1,2] }` | 见下 |

`GET /servers/:id/edit` 响应（password/private_key/script_password 恒为空字符串，用 has_* 判断）：

```json
{
  "id": 1, "name": "...", "host": "...", "port": 22, "username": "...", "auth_type": "password",
  "password": "", "private_key": "", "script_password": "",
  "server_type": "lvs", "env": "prod", "script_path": "...",
  "config_path": "...", "config_pattern": "...", "backup_path": "...",
  "description": "...", "enabled": true,
  "has_password": true, "has_private_key": false, "has_script_password": true
}
```

`POST /servers/:id/test`：

```json
// 成功
{ "success": true, "message": "连接成功 (root@10.0.0.10:22)", "output": "ok\n" }
// 失败（也是 200）
{ "success": false, "error": "连接失败: dial tcp ...: connection refused" }
```

`POST /servers/batch-test`：

```json
{
  "message": "批量测试完成: 成功 1, 失败 1",
  "success": 1, "failed": 1,
  "results": [
    { "id": 1, "name": "lvs-master-01", "success": true },
    { "id": 2, "name": "", "success": false, "error": "服务器不存在" }
  ]
}
```

- Create/Update 会对 `config_path`/`backup_path`/`config_pattern` 做非法字符校验（400 `配置路径 [x] 包含非法字符` 等）。

---

## 9. 用户管理 Users

用户对象均为 `model.User` JSON：`{id, username, name, email, role, enabled, auth_source, created_at, updated_at}`（不含 password/ldap_dn）。

| 方法/路径（均 admin） | 请求体 | 成功响应 |
|---|---|---|
| `GET /api/users` | - | `200` + `[User]` 数组（按 id 升序） |
| `POST /api/users` | `{ "username": "tom", "password": "Abc@1234", "name": "汤姆", "email": "tom@x.com", "role": "user" }` 全必填；role 只能 `admin|user`；密码强度校验（≥8 位，含大小写+数字+特殊符号）；邮箱格式校验 | **201** + `User` |
| `PUT /api/users/:id` | `{ "username": "tom", "name": "汤姆", "email": "tom@x.com", "role": "user", "enabled": true }`（前四个必填；enabled 可选布尔） | `200` + `User` |
| `DELETE /api/users/:id` | - | `{"message": "删除成功"}`（不能删自己/admin，400） |
| `POST /api/users/batch-delete` | `{ "ids": [2,3] }` | `{"message": "批量删除完成: 成功 2, 失败 0\n失败: ...", "deleted": 2, "failed": 0}`（admin 与自己会失败并在 message 列出） |
| `POST /api/users/batch-toggle` | `{ "ids": [2,3], "enabled": false }` | `{"message": "批量禁用完成: 成功 2, 失败 0", "updated": 2, "failed": 0}` |
| `PUT /api/users/:id/reset-password` | `{ "password": "New@12345" }`（强度校验） | `{"message": "密码重置成功"}`（admin 用户 403；LDAP 用户 403） |
| `PUT /api/users/:id/toggle` | - | `{"enabled": false}`（**只有 enabled 字段**，无 message；不能禁自己/admin，400） |
| `PUT /api/users/:id/password`（**普通用户，仅本人**，:id 必须等于当前 user_id） | `{ "old_password": "...", "new_password": "New@12345" }` | `{"message": "密码修改成功"}`；原密码错误 400 `{"error":"原密码错误"}`；LDAP 用户 403 |
| `GET /api/users/ldap` | - | `[ { "username": "tom", "name": "汤姆", "email": "tom@x.com", "dn": "CN=...,DC=hs,DC=com" } ]`（LDAP 未启用 400） |
| `POST /api/users/ldap/import` | `{ "users": [ { "username": "tom", "name": "汤姆", "email": "...", "dn": "CN=..." } ] }`（username、dn 必填） | `{"message": "导入完成: 成功 2, 跳过 1, 失败 0", "imported": 2, "skipped": 1, "failed": 0}` |

---

## 10. 操作日志 Logs

### `GET /api/logs`

Query 参数（全部可选）：

| 参数 | 类型 | 说明 |
|---|---|---|
| `page` | int | 默认 1，<1 归 1 |
| `size` | int | 默认 20；<1 或 >100 归 20 |
| `module` | string | `lvs/k8s/nginx/preprod/server/auth` 精确匹配 |
| `server_id` | string | 精确匹配 |
| `username` | string | LIKE 模糊 |
| `status` | string | `success/failed` |
| `action` | string | 精确匹配 |
| `keyword` | string | 对 username/action/target/server_name/ip 五个字段 OR 模糊 |
| `start_time` | string | `2006-01-02` 格式（即 `YYYY-MM-DD`），解析失败则忽略 |
| `end_time` | string | 同上，实际过滤 `< end_time+24h` |

响应 **200**（唯一分页包装）：

```json
{
  "total": 123,
  "page": 1,
  "size": 20,
  "data": [
    {
      "id": 456,
      "user_id": 1,
      "username": "admin",
      "module": "k8s",
      "action": "online",
      "target": "Commands: [...]",
      "detail": "脱敏后的命令",
      "status": "success",
      "output": "命令输出（可能很长）",
      "preview_id": "",
      "server_id": 3,
      "server_name": "k8s-node-01",
      "ip": "10.10.10.10, 10.10.10.1",
      "project_names": "user-service,order-service",
      "project_count": 2,
      "created_at": "2025-01-01T10:00:00+08:00"
    }
  ]
}
```

- `ip` 字段：优先取 `X-Forwarded-For` 完整链（逗号+空格连接），其次 `X-Real-IP`，兜底 Gin ClientIP。
- `detail` 中的 password/secret/token 等敏感串已由后端正则脱敏为 `***`。
- **权限**：非 admin 自动排除 `auth`、`server` 模块的日志。
- `project_names`：批量操作为逗号分隔服务名；全量操作为 `*`；其他模块为空。

---

## 11. 预览 → 执行通用流程

### 11.1 机制（`service/preview.go`）

- 后端创建预览：`PreviewManager.Create(module, action, serverID, params)` 生成 **UUID v4** 作为 `preview_id`，连同完整参数序列化存入 Redis（key `opscenter:preview:{uuid}`）。
- **过期时间**：TTL = `timeouts.preview`，默认 **5 分钟**。过期后 execute 返回 400 `{"error": "预览已过期或不存在"}`。
- Redis 存储 JSON 结构（供了解，不直接暴露给前端）：

```json
{
  "id": "uuid",
  "module": "lvs",           // lvs / k8s / preprod / nginx
  "action": "op",            // 见各业务域
  "server_id": 1,
  "params": { "...": "..." },
  "created_at": "...",
  "expires_at": "..."
}
```

### 11.2 关键契约点

1. **execute 只传 `preview_id`，不传任何业务参数**。所有业务参数（命令、目标 IP、config_file 等）在 preview 阶段由后端存入预览记录，execute 时从 Redis 取回并重新生成命令执行——前端无法通过 execute 篡改参数。
2. execute 会校验 `preview.Module` 与 `preview.Action`（K8s 例外：只校验 Module 不校验 Action），不匹配返回 400 `{"error":"预览类型不匹配"}`。
3. **执行成功后预览记录被删除**；执行失败（500）时预览记录保留，前端可重试 execute（在 5 分钟内）。
4. preview 响应中的展示字段（各业务域差异）：

| 业务域 | preview 响应字段 | params 中存的键 |
|---|---|---|
| LVS op | `preview_id, current_status, command, description` | `vs_ip, rs_ip, state` |
| LVS swap | 同上 | `vs_ip, rs_ip1, rs_ip2` |
| K8s 批量 | `preview_id, current_status, commands[], description` | `projects, commands` |
| K8s 全量 | 同上（commands 单元素） | `command` |
| Preprod 缩扩容 | `preview_id, current_status, command, description` | `command, resource_names` |
| Nginx online/offline | `preview_id, before, after, line_diffs[], description` | `config_file, upstream_names, backend_ip` |
| Nginx swap | 同上 | `config_file, upstream_names, offline_ip, online_ip` |
| Nginx toggle | 同上 | `config_file, upstream_names` |
| Nginx batch | 同上 | `config_file, items` |
| Nginx rollback | `preview_id, before, after, description`（无 line_diffs） | `config_file, backup_file` |

5. **preview 阶段的 `current_status`**：LVS/K8s/Preprod 是脚本 `list` 的原始输出文本（SSH 失败时为空字符串，不报错）；Nginx 用 `before/after/line_diffs` 替代。
6. 服务重启时 `PreviewManager.ClearAll()` 清空所有遗留预览。
7. LVS 的 op/swap 预览会额外校验 RS 是否被标签禁用（400）；Nginx 的 swap/toggle/batch 预览会校验 server 当前状态。

---

## 12. WebSocket 协议 `/api/ws/exec`

### 12.1 连接

- URL：`ws(s)://host:18080/api/ws/exec?token=<JWT>`
- 路由挂在受保护组下，升级请求本身要过 `Auth()` 中间件（支持 query token）。
- Origin 校验：`allowed_origins` 为空允许所有，否则必须在列表内（否则升级失败）。
- 心跳：服务端每 `ws_ping`（默认 30s）发 Ping；客户端需回 Pong，服务端读超时 `ws_read`（默认 60s）内未收到任何消息（含 Pong）即断开。

### 12.2 客户端首条消息（连接后必须先发）

```json
{
  "type": "start",            // 必须，其他值返回 error "无效的请求"
  "preview_id": "uuid-v4",    // 必填
  "token": "eyJ..."           // 可选；缺省时回退用 URL query 的 token
}
```

校验失败/异常时服务端推一条 `error` 后关闭连接。

### 12.3 消息结构（`WSMessage`，客户端和服务端共用同一结构）

```go
{
  "type": "",       // 见下表
  "token": "",      // 仅客户端首条消息用
  "preview_id": "", // 仅客户端首条消息用
  "data": "",       // 仅 type=output：单行输出内容
  "stream": "",     // 仅 type=output："stdout" | "stderr"
  "status": "",     // 仅 type=done："success"
  "message": "",    // error/lock_error 的说明文本
  "holder": ""      // 仅 lock_error：当前持有锁的用户名
}
```

所有字段 `omitempty`，实际报文只包含相关字段。

### 12.4 服务端推送消息类型

| type | 触发时机 | JSON 示例 |
|---|---|---|
| `output` | SSH 命令输出流，逐行推送 | `{"type":"output","data":"[2025-01-01 10:00:00] Start scaling...","stream":"stdout"}` |
| `done` | 执行成功，**最后一条**，之后服务端关闭连接 | `{"type":"done","status":"success"}` |
| `error` | 任何失败（token 无效、预览过期、执行失败、客户端断开等），**终止标志** | `{"type":"error","message":"预览已过期或不存在，请重新预览"}` |
| `lock_error` | 服务器锁被占用（错误子类型，连接也关闭） | `{"type":"lock_error","message":"操作正在进行中，请等待 (当前操作人: alice)","holder":"alice"}` |

错误类关键 message（前端应识别）：
- `"无效的请求"`（首条消息格式错误）
- `"未提供认证令牌"` / `"无效的认证令牌"` / `"认证令牌已被撤销"` / `"用户不存在"` / `"账户已被禁用"`
- `"预览已过期或不存在，请重新预览"`
- `"预览类型不匹配"`（WS **只接受 module=preprod** 的预览）
- `"服务器不存在"` / `"预览命令为空"`
- 命令执行失败时为 SSH 错误文本（如 `命令执行失败: exit status 1`）；客户端断开为 `"客户端断开连接"`

### 12.5 执行语义

- 预览的 `params.command` 会被流式执行（带 `script_password` 自动应答）。
- 执行期间持有该服务器的分布式锁（同 HTTP execute 的 409 逻辑）；执行结束后无论成败均**删除预览记录**并写审计日志（action 为 `batch_scaledown`/`full_scaledown` 等，按 resource_names 是否为空区分）。
- 结束标志判定：收到 `done` → 成功；收到任意 `error` → 失败；连接异常关闭且无 done → 需按失败处理。

---

## 13. 数据模型

### 13.1 `User`（表 `users`，软删除）

| JSON 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uint | 主键 |
| `username` | string | 唯一，≤50 |
| `name` | string | ≤50，非空 |
| `email` | string | ≤100 |
| `role` | string | `admin` / `user`，默认 user |
| `enabled` | bool | 默认 true |
| `auth_source` | string | `local` / `ldap`，默认 local |
| `created_at` / `updated_at` | time | RFC3339 |
| `password` | - | `json:"-"` 不返回 |
| `ldap_dn` | - | `json:"-"` 不返回 |
| `deleted_at` | - | `json:"-"` 不返回 |

### 13.2 `Server`（表 `servers`，软删除）与 `ServerResponse`（脱敏）

`Server` 可写字段（Create/Update 请求体，同时是存储字段）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | ≤100，非空 |
| `host` | string | ≤50，非空 |
| `port` | int | 默认 22 |
| `username` | string | ≤50，非空 |
| `auth_type` | string | ≤20，非空；`password` / `key` |
| `password` | string | AES-256-GCM 加密存储；Update 传 `__keep__` 保留 |
| `private_key` | string | text，同上 |
| `server_type` | string | ≤30，非空，有索引（`lvs/nginx/kubernetes/preprod` 等） |
| `env` | string | ≤20 |
| `script_path` | string | ≤255 |
| `script_password` | string | 脚本内嵌密码（用于 sudo 应答），同上 |
| `config_path` | string | ≤255，nginx 用 |
| `config_pattern` | string | ≤100，nginx 文件 glob（支持 `!` 排除、逗号分隔） |
| `backup_path` | string | ≤255，nginx 备份目录 |
| `description` | string | ≤500 |
| `enabled` | bool | 默认 true |

`ServerResponse`（List/Get/Create/Update 的返回）见 8.1 示例；三个敏感字段只返回 `has_password` / `has_private_key` / `has_script_password` 布尔值。

### 13.3 `OperationLog`（表 `operation_logs`）

字段见第 10 节响应示例（无软删除）。

### 13.4 `LvsRSTag`（表 `lvs_rs_tags`，键 rs_ip+vs_ip）

```json
{ "id": 1, "rs_ip": "10.0.0.1", "vs_ip": "10.0.0.100", "tag": "prod", "disabled": false, "disabled_reason": "", "created_at": "...", "updated_at": "..." }
```

### 13.5 `LvsVSTag`（表 `lvs_vs_tags`，键 vs_ip）

```json
{ "id": 1, "vs_ip": "10.0.0.100", "tag": "web", "created_at": "...", "updated_at": "..." }
```

### 13.6 `LvsPreprodBinding`（表 `lvs_preprod_bindings`，键 vs_tag+rs_env_tag）

```json
{ "id": 1, "vs_tag": "web", "rs_env_tag": "prod", "preprod_server_id": 5, "created_at": "...", "updated_at": "..." }
```

### 13.7 `LvsConnStat`（表 `lvs_conn_stats`，时序数据，后台采集）

字段：`id, server_id, server_name, vs_ip, vs_port, rs_ip, rs_port, active_conn, inact_conn, collected_at`。

---

## 14. 易踩坑清单

1. **没有 `{code,message,data}` 包装**。成功直接是数据，失败统一 `{"error": "..."}`；错误语义靠 HTTP 状态码。不要在前端写 `res.data.code` 判断。
2. **输出字段类型不统一**：
   - LVS / Preprod（HTTP）execute 的 `output` 是 **string**；
   - K8s execute 的 `output` 是 **string 数组**（且失败时也是数组）；
   - Nginx 各 execute 成功响应是 `{message, output}`，rollback 只有 `{message}`。
3. **`/lvs/list` 与 `/servers/:id/test` 失败也返回 200**，需按响应体/`X-Warning` 头判断。
4. **Preview 5 分钟过期**，execute 需处理 400 `预览已过期或不存在` 并引导重新预览；执行失败（500）时预览未删除，可原 preview_id 重试。
5. **K8s execute 不校验 action**（只校验 module），前端必须自己保证 preview 与 execute 配对。
6. **Preprod 有分布式锁**：HTTP execute 锁冲突返回 **409**（带当前操作人）；WS 则推送 `lock_error`。其他模块没有锁。
7. **WebSocket 首条消息必须是 `{"type":"start","preview_id":"..."}`**，且只支持 preprod 预览；token 可放消息体或 query，消息体优先；结束以 `done`/`error` 为准。
8. **JWT 过期时间由配置 `jwt.expire` 决定**（示例 24h），前端不要硬编码；登出后 token 进黑名单，重放 401。
9. **stats / activity-stats / logs 的字段随角色变化**：普通用户看不到 `users`/`online_users`、`login_stats` 恒为空数组、日志排除 auth/server 模块。
10. **`X-Warning` 响应头**：CORS 已 Expose，但值为 `url.PathEscape` 编码，需解码。
11. **Server Update 的敏感字段哨兵值是字符串 `"__keep__"`**，不是省略字段。
12. **`PUT /users/:id/toggle` 成功响应只有 `{"enabled": bool}`**，与其他 `{message}` 风格不一致。
13. **`/lvs/check/scaledown` 实际路径在 `/api/lvs/` 下**（swagger 注解路径与路由不一致，以 router.go 为准）；且只返回第一条 warning。
14. **Nginx IP 惯例**：`backend_ip` 支持逗号分隔多 IP、`:80` 端口自动剥离；预览 diff 中下线显示为 removed+added 成对行。
15. 登录接口存在 **IP 级限流（429）**，前端应针对 429 做友好提示。
