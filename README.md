# OpsCenter - 运维发布管理系统

一个用于自动化运维操作的 Web 发布系统，将 Nginx upstream 管理、LVS 上下线、K8s 部署、预生产缩扩容等操作可视化，提高发布效率和安全性。

## 功能特性

- **Nginx 管理**：upstream 后端批量上线/下线、配置文件 Diff 预览、备份与回滚
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
│   ├── rollouts-online-rollback.sh.example  # K8s 部署脚本示例
│   └── specific-project-scale.sh.example    # 缩扩容脚本示例
├── config.yaml.example   # 应用配置示例
└── Makefile              # 构建脚本
```

## 快速开始

### 前置条件

- Go 1.25+
- Node.js 18+
- MySQL 5.7+
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

复制示例配置文件并修改：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，配置以下内容：

- `database` — MySQL 连接信息（host、port、username、password、dbname）
- `jwt.secret` — JWT 签名密钥（请修改为随机字符串）
- `crypto.key` — AES 加密密钥（32 字符）

### Shell 脚本配置

Shell 脚本内含密码验证机制，需要复制示例文件并设置密码：

```bash
cp shell/rollouts-online-rollback.sh.example shell/rollouts-online-rollback.sh
cp shell/specific-project-scale.sh.example shell/specific-project-scale.sh

# 编辑脚本，将 AUTH_PASSWORD='your-password-here' 改为实际密码
```

### 运行

```bash
# 开发模式
make dev

# 构建生产版本（前端 + 后端，输出单个二进制文件）
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
- 敏感配置文件（`config.yaml`、含密码的 Shell 脚本）已加入 `.gitignore`，不会提交到仓库

### 密码加密存储

数据库中的敏感字段（SSH 密码、私钥、脚本密码）采用 **AES-256-GCM** 认证加密，密钥通过 `config.yaml` 的 `crypto.key` 配置。

**加密算法**

- 算法：AES-GCM（Galois/Counter Mode）
- 密钥长度：32 字节（AES-256）
- Nonce：12 字节，每次加密随机生成
- 认证标签：16 字节，用于防篡改

**加密字段**

以下三个字段在写入数据库前自动加密，读取后自动解密：

| 字段 | 说明 |
|------|------|
| `password` | SSH 登录密码 |
| `private_key` | SSH 私钥 |
| `script_password` | 脚本执行密码 |

**工作流程**

```
写入数据库：
  明文密码 → 随机生成 12 字节 nonce → AES-256-GCM 加密 → 拼接(nonce + 密文 + 认证标签) → Base64 编码 → 存入数据库

读取数据库：
  Base64 解码 → 分离(nonce, 密文, 认证标签) → AES-256-GCM 解密 → 明文密码
```

**实现位置**

- 加密钩子：`internal/model/server.go` → `BeforeSave()` — GORM 写入前自动触发
- 解密钩子：`internal/model/server.go` → `AfterFind()` — GORM 读取后自动触发
- 加解密工具：`internal/pkg/crypto/crypto.go`

**密钥生成**

```bash
openssl rand -base64 32
```

**注意事项**

- 相同密码每次加密结果不同（随机 nonce），无法通过密文反推原文
- 密钥一旦配置并有数据入库后不可随意更换，否则已有加密数据将无法解密
- 应用启动时会校验密钥长度，不符合要求（非 16/24/32 字节）将拒绝启动

## License

MIT
