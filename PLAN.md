# OpsCenter Web发布系统实施计划

## 一、项目概述

运维工程师每天需要重复执行服务发布操作（LVS上下线、Nginx upstream管理、K8s部署、预生产缩扩容），手工操作有误操作风险。开发Web系统将操作可视化、自动化，提高发布效率和安全性。

**核心安全原则**：所有写操作执行前必须经过 **变更预览 → 执行人复核 → 确认执行** 流程，防止误操作导致生产事故。

---

## 二、技术选型

| 层面 | 技术 |
|------|------|
| 后端 | Go + Gin + GORM + MySQL |
| 前端 | Vue 3 + Vite + Element Plus + Pinia |
| 通信 | SSH远程执行，多账号按服务器配置 |
| 实时输出 | WebSocket流式传输 |
| 认证 | 用户名密码JWT，admin/user两级 |
| 部署 | 裸机，Go embed内嵌前端，单二进制 + systemd |
| 数据库 | 公司内部MySQL，utf8mb4 |

---

## 三、项目结构

```
opscenter/
├── cmd/
│   └── server/
│       └── main.go                     # 程序入口，初始化配置、数据库、路由，启动服务
├── internal/
│   ├── config/
│   │   └── config.go                   # 加载config.yaml，全局配置结构体
│   ├── middleware/
│   │   ├── auth.go                     # JWT认证中间件，解析token，注入用户信息
│   │   └── cors.go                     # CORS跨域中间件
│   ├── model/
│   │   ├── user.go                     # 用户模型（id, username, password, role）
│   │   ├── server.go                   # 服务器配置模型（host, port, username, auth_type, script_path...）
│   │   └── operation_log.go            # 操作日志模型（module, action, target, status, output, preview_id）
│   ├── handler/
│   │   ├── auth.go                     # 登录接口 POST /api/login、用户信息 GET /api/user/info
│   │   ├── server.go                   # 服务器CRUD接口 GET/POST/PUT/DELETE /api/servers
│   │   ├── lvs.go                      # LVS操作接口（list, status, op/preview, op/execute, swap/preview, swap/execute）
│   │   ├── nginx.go                    # Nginx管理接口（configs, upstreams, online/offline/rollback preview+execute, reload）
│   │   ├── k8s.go                      # K8s部署接口（rollouts, online/sync/rollback/full preview+execute）
│   │   ├── preprod.go                  # 预生产缩扩容接口（status, scaledown/scaleup preview+execute）
│   │   ├── log.go                      # 操作日志查询接口 GET /api/logs
│   │   └── ws.go                       # WebSocket接口 WS /api/ws/exec
│   ├── service/
│   │   ├── ssh.go                      # SSH连接管理器（连接池、命令执行、密码管道、白名单校验）
│   │   ├── lvs.go                      # LVS业务逻辑（调用lvs.sh，解析输出）
│   │   ├── nginx.go                    # Nginx配置解析与管理（读取/解析/修改upstream配置，diff生成）
│   │   ├── k8s.go                      # K8s部署逻辑（调用rollouts-online-rollback.sh，解析输出）
│   │   ├── preprod.go                  # 预生产缩扩容逻辑（调用specific-project-scale.sh）
│   │   └── preview.go                  # 预览管理器（生成preview_id，存储预览数据，TTL过期）
│   ├── router/
│   │   └── router.go                   # 路由注册，分组，中间件挂载
│   └── embed/
│       └── embed.go                    # go:embed打包前端静态文件
├── web/                                # Vue前端项目
│   ├── src/
│   │   ├── api/                        # API调用封装（axios实例、请求拦截器、各模块API）
│   │   ├── views/
│   │   │   ├── Login.vue               # 登录页
│   │   │   ├── Dashboard.vue           # 总览仪表盘
│   │   │   ├── LvsManage.vue           # LVS管理页
│   │   │   ├── NginxManage.vue         # Nginx upstream管理页
│   │   │   ├── K8sDeploy.vue           # K8s服务发布页
│   │   │   ├── PreprodScale.vue        # 预生产缩扩容页
│   │   │   ├── ServerManage.vue        # 服务器配置管理页（admin）
│   │   │   └── OpLog.vue               # 操作日志页
│   │   ├── components/
│   │   │   ├── Layout.vue              # 主布局（侧边栏+头部+内容区）
│   │   │   ├── Terminal.vue            # WebSocket实时命令输出组件
│   │   │   ├── ConfirmDialog.vue       # 变更预览+确认弹窗组件
│   │   │   └── DiffViewer.vue          # 配置diff对比组件（Nginx专用）
│   │   ├── router/
│   │   │   └── index.js                # 路由配置，路由守卫
│   │   └── stores/
│   │       └── user.js                 # Pinia用户状态管理
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── config.yaml                         # 应用配置文件
├── go.mod
├── go.sum
├── Makefile                            # 构建脚本
└── IMPLEMENTATION.md                   # 本文档
```

---

## 四、数据库设计（MySQL）

### 4.1 servers 表（服务器配置）

```sql
CREATE TABLE servers (
    id             INT AUTO_INCREMENT PRIMARY KEY,
    name           VARCHAR(100) NOT NULL COMMENT '服务器名称，如 lvs01, nginx-env1',
    host           VARCHAR(50) NOT NULL COMMENT 'IP地址',
    port           INT DEFAULT 22 COMMENT 'SSH端口',
    username       VARCHAR(50) NOT NULL COMMENT 'SSH用户名',
    auth_type      VARCHAR(20) NOT NULL COMMENT 'password 或 key',
    password       VARCHAR(255) COMMENT 'SSH密码（AES加密存储）',
    private_key    TEXT COMMENT 'SSH私钥内容',
    server_type    VARCHAR(30) NOT NULL COMMENT 'lvs / nginx / k8s_master',
    env            VARCHAR(20) COMMENT 'env1 / env2 / both',
    script_path    VARCHAR(255) COMMENT '脚本路径，如 /shell/lvs.sh',
    script_password VARCHAR(255) COMMENT '脚本内部密码（AES加密），用于管道传入auth()',
    config_path    VARCHAR(255) COMMENT 'Nginx配置目录路径',
    config_pattern VARCHAR(100) COMMENT 'Nginx配置文件匹配模式，如 upstreamserver_*.conf',
    backup_path    VARCHAR(255) COMMENT 'Nginx备份目录路径',
    description    VARCHAR(500) COMMENT '描述',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_server_type (server_type),
    INDEX idx_env (env)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务器配置表';
```

### 4.2 users 表

```sql
CREATE TABLE users (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    username   VARCHAR(50) UNIQUE NOT NULL COMMENT '用户名',
    password   VARCHAR(255) NOT NULL COMMENT 'bcrypt加密密码',
    role       VARCHAR(20) NOT NULL DEFAULT 'user' COMMENT 'admin / user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

### 4.3 operation_logs 表

```sql
CREATE TABLE operation_logs (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id     INT COMMENT '操作用户ID',
    username    VARCHAR(50) COMMENT '操作用户名',
    module      VARCHAR(20) NOT NULL COMMENT 'lvs / nginx / k8s / preprod',
    action      VARCHAR(30) NOT NULL COMMENT 'online / offline / swap / scale_down / scale_up / sync / rollback',
    target      VARCHAR(500) COMMENT '操作目标描述',
    detail      TEXT COMMENT '操作详情（命令/参数）',
    status      VARCHAR(20) NOT NULL COMMENT 'success / failed / running',
    output      TEXT COMMENT '命令输出',
    preview_id  VARCHAR(64) COMMENT '关联的预览ID，用于审计追溯',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_module (module),
    INDEX idx_created_at (created_at),
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志表';
```

---

## 五、配置文件设计

### 5.1 config.yaml

```yaml
server:
  port: 8080
  host: 0.0.0.0

database:
  host: 127.0.0.1
  port: 3306
  username: opscenter
  password: your-db-password
  dbname: opscenter
  charset: utf8mb4

jwt:
  secret: your-secret-key-change-in-production
  expire: 24h

# AES加密密钥，用于加密存储SSH密码和脚本密码
crypto:
  key: your-32-char-aes-key-here!!!!
```

---

## 六、API设计

### 6.1 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/login | 登录，返回JWT token |
| GET | /api/user/info | 获取当前用户信息 |

### 6.2 服务器管理（admin only）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/servers?type=lvs | 按类型获取服务器列表 |
| POST | /api/servers | 添加服务器 |
| PUT | /api/servers/:id | 更新服务器 |
| DELETE | /api/servers/:id | 删除服务器 |

### 6.3 LVS管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/lvs/list?server_id=1 | 获取LVS列表（执行 `lvs.sh list`） |
| GET | /api/lvs/status?server_id=1 | 获取LVS状态（执行 `lvs.sh status`） |
| POST | /api/lvs/op/preview | 上线/下线操作预览 |
| POST | /api/lvs/op/execute | 执行上线/下线（需preview_id） |
| POST | /api/lvs/swap/preview | 切换操作预览 |
| POST | /api/lvs/swap/execute | 执行切换（需preview_id） |

**请求/响应示例**：
```json
// POST /api/lvs/op/preview 请求
{"server_id": 1, "vs_ip": "207", "rs_ip": "215", "state": "off"}

// POST /api/lvs/op/preview 响应
{
  "preview_id": "abc123",
  "current_status": "TCP 192.168.13.207:80 -> 192.168.13.215:80 Route Weight 2\n...",
  "command": "/shell/lvs.sh op 207 215 off",
  "description": "将 192.168.13.215 从 192.168.13.207 的后端下线"
}

// POST /api/lvs/op/execute 请求
{"preview_id": "abc123"}
```

### 6.4 Nginx管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/nginx/configs?server_id=1 | 获取配置文件列表 |
| GET | /api/nginx/upstreams?server_id=1&config_file=xxx | 获取upstream列表 |
| POST | /api/nginx/upstream/online/preview | 上线预览（返回diff） |
| POST | /api/nginx/upstream/online/execute | 执行上线 |
| POST | /api/nginx/upstream/offline/preview | 下线预览（返回diff） |
| POST | /api/nginx/upstream/offline/execute | 执行下线 |
| POST | /api/nginx/reload | nginx -t && nginx -s reload |
| POST | /api/nginx/rollback/preview | 回滚预览（返回diff） |
| POST | /api/nginx/rollback/execute | 执行回滚 |
| GET | /api/nginx/backups?server_id=1 | 获取备份列表 |

**请求/响应示例**：
```json
// POST /api/nginx/upstream/offline/preview 请求
{
  "server_id": 1,
  "config_file": "upstreamserver_iis.conf",
  "upstream_names": ["backend_iphash", "backend_loop"],
  "backend_ip": "192.168.13.204"
}

// POST /api/nginx/upstream/offline/preview 响应
{
  "preview_id": "def456",
  "before": "upstream backend_iphash\n{\n        ip_hash;\n        server 192.168.13.204;\n        server 192.168.13.205;\n}",
  "after": "upstream backend_iphash\n{\n        ip_hash;\n        server 192.168.13.204 down;\n        server 192.168.13.205;\n}",
  "description": "将 192.168.13.204 在 backend_iphash 和 backend_loop 中下线（添加down标记）"
}
```

### 6.5 K8s部署

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/k8s/rollouts?server_id=1 | 获取rollout列表 |
| POST | /api/k8s/online/preview | 上线预览 |
| POST | /api/k8s/online/execute | 执行上线 |
| POST | /api/k8s/sync/preview | 同步预览 |
| POST | /api/k8s/sync/execute | 执行同步 |
| POST | /api/k8s/rollback/preview | 回滚预览 |
| POST | /api/k8s/rollback/execute | 执行回滚 |
| POST | /api/k8s/full_online/preview | 全量上线预览 |
| POST | /api/k8s/full_online/execute | 执行全量上线 |
| POST | /api/k8s/full_sync/preview | 全量同步预览 |
| POST | /api/k8s/full_sync/execute | 执行全量同步 |
| POST | /api/k8s/full_rollback/preview | 全量回滚预览 |
| POST | /api/k8s/full_rollback/execute | 执行全量回滚 |

**请求/响应示例**：
```json
// POST /api/k8s/online/preview 请求
{
  "server_id": 1,
  "projects": [
    {"name": "pro-frontend-cupid-homsom-com-rollout", "namespace": "pro-frontend"},
    {"name": "pro-java-accountcenter-service-hs-com-rollout", "namespace": "pro-java"}
  ]
}

// POST /api/k8s/online/preview 响应
{
  "preview_id": "ghi789",
  "current_status": "NAMESPACE    NAME                         STATUS   STEP\npro-frontend pro-frontend-xxx-rollout     Paused   1/5\npro-java     pro-java-xxx-rollout         Paused   1/5",
  "commands": [
    "echo 'homsom.com' | /root/rollouts-online-rollback.sh single_online pro-frontend-cupid-homsom-com-rollout pro-frontend",
    "echo 'homsom.com' | /root/rollouts-online-rollback.sh single_online pro-java-accountcenter-service-hs-com-rollout pro-java"
  ],
  "description": "上线 2 个项目的canary版本（step 1/5 → promote）"
}
```

### 6.6 预生产缩扩容

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/preprod/status?server_id=1 | 获取资源状态 |
| POST | /api/preprod/scaledown/preview | 缩容预览 |
| POST | /api/preprod/scaledown/execute | 执行缩容 |
| POST | /api/preprod/scaleup/preview | 扩容预览 |
| POST | /api/preprod/scaleup/execute | 执行扩容 |

### 6.7 操作日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/logs?page=1&size=20&module=lvs | 分页查询操作日志 |

### 6.8 WebSocket

| 方法 | 路径 | 说明 |
|------|------|------|
| WS | /api/ws/exec | 实时命令输出流 |

---

## 七、核心模块设计

### 7.1 SSH管理器（service/ssh.go）

**功能**：
- 连接池：按server_id缓存SSH连接，定期心跳检测，断线自动重连
- 执行命令：支持超时控制（可配置，默认30秒），输出通过channel流式回传
- 密码管道：执行K8s脚本时，通过stdin写入 `echo 'password' | script` 管道
- 安全：所有命令必须经过白名单校验，防止命令注入

**命令白名单**（按server_type区分）：
```go
// lvs类型服务器允许的命令
lvsCommands = []string{
    "<script_path> list",
    "<script_path> status",
    "<script_path> op {vs_ip} {rs_ip} {state}",
    "<script_path> swap {vs_ip} {rs_ip1} {rs_ip2}",
}

// k8s_master类型服务器允许的命令
k8sCommands = []string{
    "<script_path> list",
    "echo '<script_password>' | <script_path> single_online {name} {namespace}",
    "echo '<script_password>' | <script_path> single_sync {name} {namespace}",
    "echo '<script_password>' | <script_path> single_rollback {name} {namespace}",
    "echo '<script_password>' | <script_path> full_online",
    "echo '<script_password>' | <script_path> full_sync",
    "echo '<script_password>' | <script_path> full_rollback",
    "echo '<script_password>' | <script_path> scaledown",
    "echo '<script_password>' | <script_path> scaleup",
}

// nginx类型服务器允许的命令
nginxCommands = []string{
    "cat <config_path>",
    "cp <config_path> <backup_path>",
    "sed -i 's/<pattern>/<replacement>/' <config_path>",
    "nginx -t",
    "nginx -s reload",
    "ls <backup_path>",
}
```

**参数校验规则**：
- IP最后一位：纯数字，范围1-254
- project name：只允许字母、数字、连字符、下划线、点
- namespace：只允许字母、数字、连字符
- upstream name：只允许字母、数字、下划线

### 7.2 预览管理器（service/preview.go）

**功能**：
- 生成唯一preview_id（UUID或随机字符串）
- 将预览数据存储在内存中（sync.Map），带5分钟TTL过期
- execute时校验preview_id有效，且当前状态与预览时一致
- 防止预览后配置已变更导致误操作

**数据结构**：
```go
type PreviewData struct {
    ID         string
    Module     string                 // lvs / nginx / k8s / preprod
    Action     string                 // op / swap / online / offline / sync / rollback / scaledown / scaleup
    ServerID   uint
    Params     map[string]interface{} // 操作参数
    CreatedAt  time.Time
    ExpiresAt  time.Time
}
```

### 7.3 Nginx配置解析（service/nginx.go）

**解析逻辑**：
1. SSH到nginx服务器，用 `ls <config_path><config_pattern>` 列出所有配置文件
2. 逐个读取配置文件内容
3. 正则解析 `upstream <name> { ... }` 块
4. 提取每个server行的IP、端口、状态（正常/down/注释）
5. 自动识别upstream配对关系（`*_iphash` ↔ `*_loop`）

**修改逻辑**：
1. 定位目标配置文件中的目标upstream块
2. 找到目标server行
3. 上线：移除 ` down` 关键字；下线：追加 ` down` 关键字
4. 修改前备份：`cp <config_path> <backup_path>/<config_file>.bak.<timestamp>`
5. 写入修改后的内容
6. 执行 `nginx -t` 验证语法
7. 验证通过后执行 `nginx -s reload`

**diff生成**：
- 使用Go的diff库对比修改前后的配置内容
- 返回逐行diff，标记新增/删除/修改行

### 7.4 LVS输出解析（service/lvs.go）

**lvs.sh list 输出格式**：
```
-------------------------------
IP Virtual Server version 1.2.1 (size=1048576)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
TCP  192.168.13.207:80 sh persistent 600
  -> 192.168.13.215:80            Route   2      2929       2968
  -> 192.168.13.209:80            Route   1      9          8
-------------------------------
```

**解析为结构化数据**：
```go
type VirtualServer struct {
    IP       string
    Port     string
    Protocol string
    RealServers []RealServer
}

type RealServer struct {
    IP          string
    Port        string
    Forward     string
    Weight      int
    ActiveConn  int
    InActConn   int
}
```

### 7.5 K8s输出解析（service/k8s.go）

**rollouts-online-rollback.sh list 输出格式**：
```
NAMESPACE     NAME                                          STRATEGY   STATUS   STEP  SET-WEIGHT  READY  DESIRED  UP-TO-DATE  AVAILABLE
pro-frontend  pro-frontend-cupid-homsom-com-rollout         Canary     Paused   1/5   0           4/4    2        2           4
pro-java      pro-java-accountcenter-service-hs-com-rollout Canary     Paused   3/5   100         4/4    2        2           4
```

**解析为结构化数据**：
```go
type Rollout struct {
    Namespace   string
    Name        string
    Strategy    string
    Status      string
    Step        string
    SetWeight   string
    Ready       string
    Desired     int
    UpToDate    int
    Available   int
}
```

### 7.6 预生产输出解析（service/preprod.go）

**specific-project-scale.sh list 输出格式**：
```
=== Rollout状态 ===
NAME                                                 DESIRED  CURRENT  UP-TO-DATE  AVAILABLE  AGE
pro-java-flightrefund-order-service-hs-com-rollout   2        2        2           2          2y156d
----------------------------------------
=== Deployment状态 ===
NAME                                  READY    UP-TO-DATE  AVAILABLE  AGE
middleware-xxl-job-admin-deployment   0/0      0           0          2y179d
----------------------------------------
=== Require列表状态 ===
NAME                                              DESIRED  CURRENT  UP-TO-DATE  AVAILABLE  AGE
pro-dotnet-domaineventserviceapi-hs-com-rollout   4        4        4           4          2y156d
----------------------------------------
```

---

## 八、前端页面设计

### 8.1 登录页（Login.vue）
- 用户名/密码输入框
- 登录按钮，成功后跳转Dashboard
- JWT token存储到localStorage

### 8.2 主布局（Layout.vue）
- 左侧边栏：导航菜单（Dashboard、LVS管理、Nginx管理、K8s部署、预生产缩扩容、操作日志、服务器管理）
- 顶部头部：当前用户、退出登录
- 右侧内容区：router-view

### 8.3 Dashboard（Dashboard.vue）
- LVS状态概览卡片（哪些VIP→RS在线/离线）
- 最近操作日志列表（最近10条）
- 快捷入口按钮（跳转各模块）

### 8.4 LVS管理页（LvsManage.vue）
- 左侧：LVS服务器下拉选择
- 右侧：表格展示VIP→RS映射及状态
- 操作按钮：上线（绿色）、下线（红色）、切换（蓝色）
- **变更预览弹窗**：点击操作后弹出，显示当前状态 + 将要执行的命令 + 变更说明
- 确认后执行，底部WebSocket实时输出区

### 8.5 Nginx管理页（NginxManage.vue）
- 左侧：nginx服务器下拉 + 配置文件下拉
- 右侧：分组展示upstream配对（iphash + loop为一组）
- 每组内表格：backend IP、端口、状态（up/down标签）
- 操作：勾选backend → 上线/下线按钮
- **配置diff预览弹窗**：左右对比修改前/后配置，高亮变更行
- 底部：备份列表 + 回滚按钮（回滚也展示diff）+ WebSocket实时输出区

### 8.6 K8s部署页（K8sDeploy.vue）
- 表格：rollout列表（命名空间、名称、策略、状态、步骤、权重）
- checkbox多选列
- 顶部操作栏：批量上线 / 批量同步 / 批量回滚 / 全量上线 / 全量同步 / 全量回滚
- 每行操作按钮：上线 / 同步 / 回滚
- **变更预览弹窗**：当前状态 + 命令列表 + 操作说明
- 底部WebSocket实时输出区

### 8.7 预生产缩扩容页（PreprodScale.vue）
- 流程引导步骤条：
  - Step1：查看资源状态（表格展示三类资源当前副本数）
  - Step2：下线LVS（内联操作按钮，含变更预览）
  - Step3：缩容（点击后弹出预览：当前副本数→0，确认后执行）
  - Step4：部署验证（手动确认按钮）
  - Step5：扩容（点击后弹出预览：0→目标副本数，确认后执行）
  - Step6：上线LVS（内联操作按钮，含变更预览）
- 底部WebSocket实时输出区

### 8.8 服务器配置管理页（ServerManage.vue）（admin only）
- 表格展示所有服务器配置
- 添加/编辑/删除操作
- 表单：名称、IP、端口、用户名、认证类型、脚本路径、脚本密码、配置路径、备份路径等

### 8.9 操作日志页（OpLog.vue）
- 表格：时间、操作人、模块、动作、目标、状态
- 筛选：按模块、按时间范围
- 分页
- 点击行展开详情：命令、输出内容

---

## 九、脚本命令映射

### 9.1 lvs.sh

```bash
# 列出所有VS/RS
<server.script_path> list

# 查看VS配置文件
<server.script_path> status

# 上线/下线 RS（IP只传最后一位）
<server.script_path> op <vs_ip> <rs_ip> off
<server.script_path> op <vs_ip> <rs_ip> on

# 切换两个RS状态（要求一个在线一个离线）
<server.script_path> swap <vs_ip> <rs_ip1> <rs_ip2>
```

### 9.2 rollouts-online-rollback.sh

```bash
# 列出（不需要密码）
<server.script_path> list

# 单个操作（管道传密码）
echo '<server.script_password>' | <server.script_path> single_online <name> <namespace>
echo '<server.script_password>' | <server.script_path> single_sync <name> <namespace>
echo '<server.script_password>' | <server.script_path> single_rollback <name> <namespace>

# 全量操作（管道传密码）
echo '<server.script_password>' | <server.script_path> full_online
echo '<server.script_password>' | <server.script_path> full_sync
echo '<server.script_password>' | <server.script_path> full_rollback
```

### 9.3 specific-project-scale.sh

```bash
# 列出（不需要密码）
<server.script_path> list

# 缩容/扩容（管道传密码）
echo '<server.script_password>' | <server.script_path> scaledown
echo '<server.script_password>' | <server.script_path> scaleup
```

---

## 十、实施阶段

### Phase 1：基础框架

| 任务 | 内容 | 依赖 |
|------|------|------|
| 1.1 | 初始化Go项目，安装依赖（gin, gorm, mysql-driver, jwt-go, bcrypt, websocket） | 无 |
| 1.2 | 配置加载（config.yaml）、GORM连接MySQL、自动迁移表结构 | 1.1 |
| 1.3 | 用户认证：users model、POST /api/login（bcrypt+JWT）、GET /api/user/info、JWT中间件 | 1.2 |
| 1.4 | SSH管理器：连接池、命令执行、密码管道、白名单校验、超时控制 | 1.2 |
| 1.5 | 服务器配置CRUD：servers model、GET/POST/PUT/DELETE /api/servers、admin权限 | 1.3 |
| 1.6 | 操作日志：operation_logs model、自动记录机制、GET /api/logs接口 | 1.2 |
| 1.7 | Vue前端骨架：初始化项目、Element Plus、登录页、主布局、路由配置 | 无（可并行） |

### Phase 2：LVS管理模块

| 任务 | 内容 | 依赖 |
|------|------|------|
| 2.1 | LVS后端接口：list、status、op/preview+execute、swap/preview+execute、输出解析 | 1.4+1.5+1.6 |
| 2.2 | WebSocket实时输出：WS handler、前端Terminal组件 | 1.4 |
| 2.3 | LVS前端页面：状态表格、操作按钮、变更预览弹窗、实时输出区 | 2.1+2.2+1.7 |

### Phase 3：K8s部署模块

| 任务 | 内容 | 依赖 |
|------|------|------|
| 3.1 | K8s后端接口：rollouts列表、single/full online/sync/rollback preview+execute、输出解析 | 1.4+1.6 |
| 3.2 | K8s前端页面：rollout表格、checkbox多选、批量操作、变更预览弹窗 | 3.1+2.2 |

### Phase 4：预生产缩扩容模块

| 任务 | 内容 | 依赖 |
|------|------|------|
| 4.1 | 预生产后端接口：status、scaledown/scaleup preview+execute、输出解析 | 1.4+1.6 |
| 4.2 | 预生产前端页面：流程引导步骤条、每步含变更预览 | 4.1+2.2 |

### Phase 5：Nginx管理模块

| 任务 | 内容 | 依赖 |
|------|------|------|
| 5.1 | Nginx后端接口：configs列表、upstream解析、online/offline/rollback preview+execute、diff生成、reload、backups | 1.4+1.6 |
| 5.2 | Nginx前端页面：配置文件选择、upstream分组、backend状态、diff预览弹窗、备份回滚 | 5.1+2.2 |

### Phase 6：收尾

| 任务 | 内容 | 依赖 |
|------|------|------|
| 6.1 | Dashboard总览页：LVS状态概览、最近日志、快捷入口 | 2.3+3.2+5.2 |
| 6.2 | 操作日志页面：分页表格、筛选、详情展开 | 1.6 |
| 6.3 | 权限控制与收尾：路由守卫、admin菜单可见性、Go embed打包前端、Makefile | 6.1+6.2 |

---

## 十一、部署方式

### 11.1 构建流程

```bash
# 前端构建
cd web && npm install && npm run build    # 输出到 web/dist/

# 后端编译（内嵌前端静态文件）
cd .. && CGO_ENABLED=0 go build -o opscenter ./cmd/server/

# 产物：opscenter 二进制 + config.yaml
```

### 11.2 systemd服务

```ini
[Unit]
Description=OpsCenter Web Release System
After=network.target mysql.service

[Service]
Type=simple
ExecStart=/opt/opscenter/opscenter -config /opt/opscenter/config.yaml
WorkingDirectory=/opt/opscenter
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 11.3 部署步骤

1. 在MySQL中创建数据库和用户：
   ```sql
   CREATE DATABASE opscenter CHARACTER SET utf8mb4;
   CREATE USER 'opscenter'@'%' IDENTIFIED BY 'your-db-password';
   GRANT ALL PRIVILEGES ON opscenter.* TO 'opscenter'@'%';
   ```
2. 编译前端和后端
3. 将二进制和config.yaml放到服务器
4. 修改config.yaml中的数据库连接和JWT密钥
5. 启动服务，首次启动自动创建表结构
6. 访问 http://host:8080，使用默认管理员账号登录

---

## 十二、安全机制汇总

| 机制 | 说明 |
|------|------|
| 变更预览复核 | 所有写操作必须经过 preview → 人工确认 → execute 流程 |
| preview_id校验 | execute时校验preview_id有效且未过期（5分钟TTL） |
| 状态一致性校验 | execute时再次获取当前状态，与预览时对比，不一致则拒绝执行 |
| SSH命令白名单 | 只允许执行预定义的命令模板，参数严格正则校验 |
| 密码加密存储 | SSH密码和脚本密码使用AES加密存储在数据库 |
| JWT认证 | 所有API接口需要JWT token，过期自动跳转登录 |
| 角色权限 | admin可管理服务器配置，user只能执行操作 |
| 操作日志审计 | 所有操作记录到operation_logs表，含preview_id用于追溯 |
| Nginx自动备份 | 修改配置前自动备份到指定目录 |
| nginx -t验证 | Nginx配置修改后先验证语法再reload |

---

## 十三、Makefile

```makefile
.PHONY: build frontend backend clean

# 构建前端
frontend:
	cd web && npm install && npm run build

# 构建后端（内嵌前端）
backend:
	CGO_ENABLED=0 go build -o opscenter ./cmd/server/

# 完整构建
build: frontend backend

# 清理
clean:
	rm -f opscenter
	rm -rf web/dist web/node_modules

# 开发模式（分别启动前后端）
dev-frontend:
	cd web && npm run dev

dev-backend:
	go run ./cmd/server/
```
