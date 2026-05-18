# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在本仓库中工作时提供指引。

## 项目概述

OpsCenter 是一个运维发布管理系统，将 Nginx upstream 管理、LVS 上下线、K8s 部署、预生产缩扩容等操作可视化，提高发布效率和安全性。

## 技术栈

- **后端**: Go 1.25 + Gin + GORM + MySQL，通过 `go:embed` 将前端嵌入单二进制文件
- **前端**: Vue 3 + Vite + Element Plus + Pinia + Vue Router
- **通信**: SSH 远程执行、WebSocket 流式输出
- **认证**: JWT（admin/user 两级权限），AES-256-GCM 加密敏感字段（密码、私钥）

## 构建与开发命令

```bash
# 完整构建（前端嵌入 Go 二进制）
make build

# 仅构建后端（需要前端已构建到 web/dist/）
make backend

# 仅构建前端
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

- **`cmd/server/main.go`** — 程序入口。加载配置、通过 GORM 连接 MySQL、自动迁移模型、注册路由。
- **`internal/config/`** — YAML 配置加载到 `config.Global` 单例。校验加密密钥长度（16/24/32 字节）。
- **`internal/model/`** — GORM 模型：`User`、`Server`、`OperationLog`。`Server` 模型通过 GORM 钩子（`BeforeSave`/`AfterFind`）自动对敏感字段（password、private_key、script_password）进行 AES-256-GCM 加解密。
- **`internal/handler/`** — HTTP 处理器，按业务域划分：`auth`、`server`、`lvs`、`k8s`、`preprod`、`nginx`、`log`、`ws`（WebSocket）。
- **`internal/service/`** — 业务逻辑层：
  - `SSHManager` — 按服务器的连接池管理、命令执行（单次和流式）、按服务器类型的命令白名单正则校验。
  - `PreviewManager` — 操作预览的内存存储（UUID 键，5 分钟过期）。预览→执行流程：handler 创建预览 → 返回 preview_id → 前端确认 → handler 用 preview_id 执行。
  - `LockManager` — 按服务器的分布式锁（sync.Map + CAS，10 分钟自动过期），防止对同一服务器的并发操作。
- **`internal/middleware/`** — JWT 认证（`Auth()`）、CORS、管理员角色检查（`AdminRequired()`）、用户启用状态检查。
- **`internal/router/`** — 路由注册。API 前缀为 `/api`。受保护路由需 JWT。管理员路由额外使用 `AdminRequired` 中间件。
- **`internal/pkg/crypto/`** — AES-256-GCM 加解密工具。

### 前端 (`web/src/`)

- **`api/index.js`** — Axios 实例，带 JWT 拦截器。所有 API 函数在此导出。GET 请求自动添加 `_t` 缓存破坏参数。
- **`router/index.js`** — Vue Router，含认证守卫（检查 `localStorage.token`）和管理员守卫（`localStorage.role`）。
- **`stores/user.js`** — Pinia 状态管理，管理认证状态。
- **`views/`** — 页面组件：Dashboard、LvsManage、NginxManage、K8sDeploy、PreprodScale、OpLog、ServerManage、UserManage、Login。
- **`components/`** — Layout（侧边导航）、StreamOutput（WebSocket 流式输出展示）。
- **`composables/useWebSocket.js`** — WebSocket 组合式函数，用于实时命令输出。

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

`config.yaml`（已 gitignore）— 从 `config.yaml.example` 复制。主要配置项：`server`（host/port）、`database`（MySQL 连接信息）、`jwt`（密钥/过期时间）、`crypto`（AES 密钥）。

## 开发约定

- 所有面向用户的文案均使用中文（错误消息、日志、UI 文本）
- API 响应使用 JSON。服务器列表响应通过 `Server.ToResponse()` 脱敏
- 命令执行通过 `service/ssh.go:ValidateCommand()` 按服务器类型进行正则白名单校验
- WebSocket 端点 `/api/ws/exec` 通过 URL query 参数传递 token 认证（非 Header）
