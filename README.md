# OpsCenter - 运维发布管理系统

一个用于自动化运维操作的 Web 发布系统，将 LVS 上下线、K8s 部署、预生产缩扩容等操作可视化，提高发布效率和安全性。

## 功能特性

- **LVS 管理**：Real Server 上下线、RS 切换（swap）
- **K8s 部署**：Argo Rollout 在线/同步/回滚，支持单个/批量/全量操作
- **预生产缩扩容**：并行缩扩容 Rollout/Deployment 资源
- **实时输出**：WebSocket 流式传输命令执行结果
- **操作审计**：完整的操作日志记录
- **安全认证**：JWT 认证，admin/user 两级权限

## 技术栈

| 层面 | 技术 |
|------|------|
| 后端 | Go + Gin + GORM + MySQL |
| 前端 | Vue 3 + Vite + Element Plus + Pinia |
| 通信 | SSH 远程执行、WebSocket |
| 部署 | Go embed 内嵌前端，单二进制 + systemd |

## 项目结构

```
opscenter/
├── cmd/server/           # 程序入口
├── internal/             # 后端核心代码
│   ├── config/           # 配置加载
│   ├── middleware/       # JWT/CORS 中间件
│   ├── model/            # 数据模型
│   ├── handler/          # HTTP 处理器
│   ├── service/          # 业务逻辑
│   ├── router/           # 路由注册
│   └── embed/            # 前端静态文件嵌入
├── web/                  # Vue 前端项目
├── shell/                # 运维脚本
│   ├── lvs.sh            # LVS 管理脚本
│   ├── rollouts-online-rollback.sh  # K8s 部署脚本
│   └── specific-project-scale.sh    # 缩扩容脚本
├── config.yaml           # 应用配置
└── Makefile              # 构建脚本
```

## 快速开始

### 前置条件

- Go 1.21+
- Node.js 18+
- MySQL 8.0+
- kubectl (K8s 环境)

### 安装

```bash
# 克隆项目
git clone <repo-url>
cd opscenter

# 安装后端依赖
go mod download

# 安装前端依赖
cd web && npm install && cd ..
```

### 配置

复制并修改配置文件：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml` 配置数据库连接、JWT 密钥等。

### 运行

```bash
# 开发模式
make dev

# 构建生产版本
make build

# 运行
./opscenter
```

## Shell 脚本使用

### LVS 管理

```bash
# 查看 LVS 状态
./shell/lvs.sh list

# 上线 RS
./shell/lvs.sh op 207 215 on

# 下线 RS
./shell/lvs.sh op 207 215 off

# 切换 RS
./shell/lvs.sh swap 207 215 209
```

### K8s 部署

```bash
# 查看 Rollout 列表
./shell/rollouts-online-rollback.sh list

# 单个服务上线
./shell/rollouts-online-rollback.sh single_online <project> <namespace>

# 全量上线
./shell/rollouts-online-rollback.sh full_online

# 全量回滚
./shell/rollouts-online-rollback.sh full_rollback
```

### 预生产缩扩容

```bash
# 查看资源状态
./shell/specific-project-scale.sh list

# 缩容
./shell/specific-project-scale.sh scaledown

# 扩容
./shell/specific-project-scale.sh scaleup
```

## 安全说明

- 所有写操作必须经过 **变更预览 → 执行人复核 → 确认执行** 流程
- Shell 脚本内置密码验证机制
- 操作日志完整记录所有变更

## License

MIT
