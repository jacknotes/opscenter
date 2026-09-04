# OpsCenter 部署指南

## 目录结构

```
.
├── Dockerfile              # Docker 镜像构建文件
├── docker-compose.yaml     # 本地开发/测试用
├── .dockerignore           # Docker 构建忽略文件
└── k8s/
    ├── namespace.yaml      # K8s 命名空间
    ├── configmap.yaml      # OpsCenter 非敏感配置
    ├── deployment.yaml     # OpsCenter 应用部署
    ├── service.yaml        # OpsCenter 服务
    └── virtualservice.yaml # Istio VirtualService（可选）
```

## Docker 部署

### 构建镜像

```bash
# 构建镜像
docker build -t opscenter:latest .

# 推送到私有仓库（替换为你的仓库地址）
docker tag opscenter:latest your-registry.com/opscenter:latest
docker push your-registry.com/opscenter:latest
```

### 使用 Docker Compose 运行

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f opscenter

# 停止所有服务
docker-compose down
```

访问 http://localhost:18080 即可使用。

## Kubernetes 部署

### 前置要求

- Kubernetes 1.19+
- kubectl 已配置
- 集群支持 PersistentVolume（如使用云存储）

### 部署步骤

1. **创建命名空间**

```bash
kubectl apply -f k8s/namespace.yaml
```

2. **配置 OpsCenter**

编辑 `k8s/configmap.yaml`，修改数据库连接、JWT 密钥等配置：

```bash
vi k8s/configmap.yaml
kubectl apply -f k8s/configmap.yaml
```

敏感信息（密码、密钥等）通过环境变量注入，详见 `config.yaml.example`。

3. **部署 OpsCenter**

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

4. **配置 Istio VirtualService（可选）**

如使用 Istio 服务网格，编辑 `k8s/virtualservice.yaml` 配置域名后应用：

```bash
kubectl apply -f k8s/virtualservice.yaml
```

### 查看部署状态

```bash
# 查看所有资源
kubectl -n opscenter get all

# 查看 Pod 状态
kubectl -n opscenter get pods

# 查看日志
kubectl -n opscenter logs -l app=opscenter -f

# 查看服务
kubectl -n opscenter get svc

# 查看 Ingress
kubectl -n opscenter get ingress
```

### 访问应用

- **使用 Ingress**：配置域名解析后访问 http://opscenter.example.com
- **使用 NodePort**：修改 service.yaml 类型为 NodePort，访问 http://<节点IP>:<端口>
- **使用 Port Forward**：本地测试用

```bash
kubectl -n opscenter port-forward svc/opscenter 18080:80
```

然后访问 http://localhost:18080

## 配置说明

### 环境变量

| 环境变量 | 说明 | 必需 |
|---------|------|------|
| DB_PASSWORD | MySQL 用户密码 | 是 |
| JWT_SECRET | JWT 签名密钥 | 是 |
| CRYPTO_KEY | AES-256 加密密钥（16/24/32字节）| 是 |

### 配置文件

配置文件位于 `/app/config/config.yaml`，示例：

```yaml
server:
  port: 18080
  host: 0.0.0.0

database:
  host: mysql.opscenter.svc.cluster.local
  port: 3306
  username: opscenter
  dbname: opscenter
  charset: utf8mb4

jwt:
  secret: ""  # 从环境变量注入
  expire: 24h

crypto:
  key: ""  # 从环境变量注入
```

敏感字段（密码、密钥）会优先从环境变量读取，如未设置则使用配置文件中的值。

## 默认账号

首次启动时会自动创建管理员账号：

- 用户名：`admin`
- 密码：`Admin@123`（未配置 `admin_password` 时的默认值）

**请在生产环境中通过 `config.yaml` 的 `server.admin_password` 配置强密码！**

### 重置 Admin 密码

admin 用户受保护，不能通过 Web 界面删除、禁用或重置密码。如需重置：

1. 编辑 `config.yaml`，设置新的 `admin_password`
2. 重启 OpsCenter 服务，密码自动同步

```yaml
server:
  admin_password: your-new-password
```

Docker/K8s 环境可通过环境变量 `ADMIN_PASSWORD` 覆盖。

## 生产环境建议

1. **安全配置**
   - 修改所有默认密码和密钥
   - 使用 K8s Secret 存储敏感信息
   - 启用 HTTPS（配置 TLS 证书）

2. **高可用**
   - MySQL 使用主从复制或云数据库
   - OpsCenter 可部署多个副本

3. **监控**
   - 配置健康检查（已内置 /api/health）
   - 收集日志到 ELK 或 Loki
   - 配置告警规则

4. **备份**
   - 定期备份 MySQL 数据
   - 备份配置文件和 Secret

## 卸载

```bash
# 删除 K8s 资源
kubectl delete -f k8s/

# 删除命名空间（会删除所有资源）
kubectl delete namespace opscenter

# Docker Compose
docker-compose down -v  # -v 同时删除数据卷
```
