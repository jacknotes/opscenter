# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在本仓库中工作时提供指引。

## 项目概述

OpsCenter 是一个运维发布管理系统，将 Nginx upstream 管理、LVS 上下线、K8s 部署、预生产缩扩容等操作可视化，提高发布效率和安全性。

## 技术栈

- **后端**: Go 1.25 + Gin + GORM + MySQL，运行时通过 `StaticFS` 读取 `web/dist/` 提供前端页面（Docker 构建时通过多阶段 COPY 打包）
- **前端**: Vue 3.5 + TypeScript（strict）+ Vite + Element Plus + Pinia + Vue Router + vue-i18n（仅中文，AOT 预编译）+ ECharts，版本 2.0.0
- **通信**: SSH 远程执行、WebSocket 流式输出
- **认证**: JWT（admin/user 两级权限），AES-256-GCM 加密敏感字段（密码、私钥）

## 构建与开发命令

```bash
# 完整构建（前端 dist 供后端运行时 StaticFS 提供服务）
make build

# 仅构建后端（需要前端已构建到 web/dist/）
make backend

# 仅构建前端（vue-tsc 类型检查 + Vitest 可选先行，见 web/package.json）
make frontend

# 开发模式（前后端分离运行）
make dev-frontend   # Vite 开发服务器 :3000，代理 /api 到 :18080
make dev-backend    # Go 服务器 :18080

# 运行
./opscenter -config config.yaml
```

## 架构

### 后端 (`internal/`)

后端采用分层架构：

- **`cmd/server/main.go`** — 程序入口。加载配置、通过 GORM 连接 MySQL、自动迁移模型、注册路由、优雅停机。
- **`internal/config/`** — YAML 配置加载到 `config.Global` 单例。支持环境变量覆盖（DB_HOST、DB_PORT、DB_PASSWORD、JWT_SECRET、CRYPTO_KEY、ADMIN_PASSWORD）。校验加密密钥长度（16/24/32 字节）。
- **`internal/model/`** — GORM 模型：`User`、`Server`、`OperationLog`。`Server` 模型通过 GORM 钩子（`BeforeSave`/`AfterFind`）自动对敏感字段（password、private_key、script_password）进行 AES-256-GCM 加解密。
- **`internal/handler/`** — HTTP 处理器，按业务域划分：`auth`、`server`、`lvs`、`k8s`、`preprod`、`nginx`、`log`、`ws`（WebSocket）。`audit.go` 提供审计日志写入和客户端 IP 提取的辅助函数。
- **`internal/service/`** — 业务逻辑层：
  - `SSHManager` — 按服务器的连接池管理、命令执行（单次和流式）、按服务器类型的命令白名单正则校验。
  - `PreviewManager` — 操作预览的内存存储（UUID 键，5 分钟过期）。预览→执行流程：handler 创建预览 → 返回 preview_id → 前端确认 → handler 用 preview_id 执行。
  - `LockManager` — 按服务器的分布式锁（sync.Map + CAS，10 分钟自动过期），防止对同一服务器的并发操作。
  - `LVSService` / `K8sService` / `NginxService` / `PreprodService` — 各业务域的服务实现，负责输出解析和预览生成。
- **`internal/middleware/`** — JWT 认证（`Auth()`）、CORS、管理员角色检查（`AdminRequired()`）、用户启用状态检查（`UserEnabledCheck()`）。
- **`internal/router/`** — 路由注册。API 前缀为 `/api`。受保护路由需 JWT。管理员路由额外使用 `AdminRequired` 中间件。
- **`internal/pkg/crypto/`** — AES-256-GCM 加解密工具。

### 前端 (`web/src/`, Vue3 + TS 重写版 v2.0.0)

- **`api/client.ts`** — 唯一 axios 实例（baseURL `/api`）：JWT 注入、401 清会话跳登录、响应头 `X-Warning` 解码提示。`extractErrorMessage()` 统一错误文案提取（后端契约：失败返回 `{"error": "中文"}` + HTTP 状态码，无响应包装层）。
- **`api/types.ts`** — 全部后端契约类型（依据 `docs/frontend-v2/api-contract.md` 生成）。
- **`api/index.ts`** — 按资源聚合的 API 方法（authApi/dashboardApi/lvsApi/k8sApi/preprodApi/nginxApi/serverApi/userApi/logApi）。
- **`router/index.ts`** — Vue Router，认证守卫（无 token 跳登录并带 redirect 回跳）与管理员守卫（`meta.adminOnly`）。
- **`utils/session.ts`** — 会话生命周期：localStorage 凭据 + 会话 Cookie + sessionStorage 窗口标记双判据（`initSession()` 处理浏览器重开/复制标签页竞态）。
- **`stores/auth.ts`** — Pinia 认证状态（login/logout/refreshUser，isAdmin 守卫依据）。
- **`composables/usePreviewExecute.ts`** — 预览→执行通用流程（含 5 分钟过期倒计时与过期重预览）。
- **`composables/useTablePaging.ts`** — 客户端排序分页（布尔 0/1、数值、localeCompare('zh-Hans-CN')，支持派生列 getter）。
- **`composables/useTheme.ts`** — 深色默认 + 亮色覆盖（`data-theme` 切换，设计令牌唯一事实源 `styles/tokens.css`）。
- **`components/`** — AppLayout（侧边栏/顶栏/移动端导航）、PreviewDialog（预览确认 + diff 展示 + 流式执行）、StreamOutput（WebSocket 终端）、ServerSelect、BaseChart（ECharts 封装）。
- **`views/`** — 页面组件：Dashboard、LvsManage、NginxManage、K8sDeploy、PreprodScale（WebSocket 流式）、OpLog、ServerManage（敏感字段 `__keep__` 哨兵）、UserManage、Login。
- **`locales/zh-CN.json`** — 全部 UI 文案（i18n 架构保留，当前仅中文）。
- **测试**：`npm run test`（Vitest）、`npm run type-check`（vue-tsc strict）。

### 核心模式：预览 → 执行

所有写操作（LVS 上下线/切换、K8s 部署、Nginx upstream 变更、预生产缩扩容）均遵循两步流程：
1. **预览** — 校验参数，展示变更内容，存储带 UUID 的 `PreviewData`，返回 preview_id。
2. **执行** — 前端回传 preview_id，后端检索预览记录、验证未过期后执行。预生产操作通过 WebSocket 流式输出。

### Shell 脚本 (`shell/`)

- `lvs.sh` — LVS 管理（list、op on/off、swap）
- `rollouts-online-rollback.sh` — K8s Argo Rollout 操作（list、single/full online/sync/rollback）
- `specific-project-scale.sh` — 预生产缩扩容（list、scaledown、scaleup）

脚本内置密码认证机制。`.example` 文件为模板；含密码的实际脚本已加入 .gitignore。

## 数据库

MySQL + GORM 自动迁移。三张表：`users`、`servers`、`operation_logs`。`servers` 表使用软删除（`DeletedAt`）。

## 配置

`config.yaml`（已 gitignore）— 从 `config.yaml.example` 复制。主要配置项：`server`（host/port/admin_password/allowed_origins/known_hosts_path）、`database`（MySQL 连接信息）、`jwt`（密钥/过期时间）、`crypto`（AES 密钥）。

### Admin 密码管理

- admin 用户受保护，不能通过 UI 删除、禁用或重置密码
- 重置 admin 密码：修改 `config.yaml` 的 `server.admin_password` 后重启服务，密码自动同步
- 支持环境变量 `ADMIN_PASSWORD` 覆盖
- 首次启动未配置时默认密码为 `Admin@123`

## 部署

- **Docker**: `Dockerfile` 为多阶段构建（Node 20 前端 → Go 1.25 后端 → Alpine 3.19 运行时）。`docker-compose.yaml` 提供一键部署。
- **Kubernetes**: `k8s/` 目录包含 Namespace、Secret、ConfigMap、Deployment（2 副本、健康检查、资源限制）、Service。详细文档见 `deploy/README.md`。

## API 文档

Swagger UI 挂载在 `/swagger/*any`，启动后访问 `http://localhost:18080/swagger/index.html`。Swagger 注解在 handler 文件中，通过 `swag init` 生成到 `docs/` 目录。

## 开发约定

- 所有面向用户的文案均使用中文（错误消息、日志、UI 文本）
- API 响应使用 JSON。服务器列表响应通过 `Server.ToResponse()` 脱敏
- 命令执行通过 `service/ssh.go:ValidateCommand()` 按服务器类型进行正则白名单校验
- WebSocket 端点 `/api/ws/exec` 通过 URL query 参数传递 token 认证（非 Header）
- LVS 和 Nginx 管理页面默认展开所有分组（VIP/upstream）
