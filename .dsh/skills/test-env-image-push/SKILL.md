---
name: test-env-image-push
description: 在测试环境（172.168.2.12）拉取最新代码、构建 OpsCenter 镜像并以服务版本号打 tag、推送到 Harbor。当用户提到「打镜像」「构建镜像」「推镜像」「发版镜像」「推送 harbor」「build image」「docker push」，或要求把某个 commit / 分支部署到测试环境镜像仓库时使用本 skill。
---

# 测试环境镜像构建与推送

把当前代码变成一个带版本号 tag 的镜像并推进 Harbor，全程在测试机 `172.168.2.12` 上完成；本地 WSL 只负责驱动 SSH。

## 一条命令

```bash
wsl.exe -d Ubuntu-26.04 -- bash .dsh/skills/test-env-image-push/scripts/build-and-push.sh
```

先干跑（拉代码 + 构建 + 校验，跳过 push）确认版本号正确：

```bash
wsl.exe -d Ubuntu-26.04 -- bash .dsh/skills/test-env-image-push/scripts/build-and-push.sh --dry-run
```

脚本会打印 `VERSION`、`IMAGE` 和最终 push 结果。**把最终的镜像地址和版本号回报给用户**，用户最关心的就是这两项。

## 关键约束

**镜像 tag 必须等于服务版本号**（例如 `v2.0.0-284-gfc887d7`），不能是 `latest` 或手写的 `v4.9` 之类。原因：侧边栏底部显示的就是这个版本号，tag 与页面显示不一致时，线上排查会分不清跑的是哪份代码。

版本号在**测试机的 git 工作区**里算出来（那里有 `.git`），再通过 `--build-arg APP_VERSION=` 传进镜像。这一步是必须的：`.dockerignore` 排除了 `.git`，构建上下文里没有 git 元数据，`web/build/appVersion.js` 在容器内只能回退到 `package.json` 的 `v2.0.0`，丢掉 `-284-gfc887d7` 这段。

## 脚本干了什么

| 步骤 | 动作 | 说明 |
|---|---|---|
| 1 | `git fetch` + `merge --ff-only origin/main` | 拉最新代码 |
| 2 | 按 `web/build/appVersion.js` 的同一套规则算版本号 | 与前端展示保持同一口径 |
| 3 | `docker build --build-arg APP_VERSION=<版本号>` | 多阶段构建，前端和后端都在容器内完成 |
| 4 | 在镜像里 grep 版本号字符串 | 证明 tag 与页面实际显示的版本一致 |
| 5 | `docker push harborrepo.hs.com/ops/opscenter:<版本号>` | `--dry-run` 时跳过 |

## 环境事实（已核实）

- 仓库路径：`/root/gitlab/opscenter`，远端 `https://github.com/jacknotes/opscenter.git`
- **git 访问 github 必须走代理** `http://172.168.2.219:10808`，直连会 `Connection timed out`。脚本用 `git -c http.proxy=...` 逐条传入，不改测试机的 git 配置。
- 测试机**没有 node 和 go**，构建完全依赖 Docker 多阶段镜像，这是正常的。
- Harbor 已 `docker login`（凭据在 `~/.docker/config.json`），脚本不碰密码。
- Docker 19.03.15，Harbor 解析到 `192.168.13.197`。

## 怎么确认推送真的成功了

**不要用 `curl` 去查 Harbor API**。这台 Harbor 只开了 80 端口（443 直接 `Connection refused`），且被配成了 `insecure-registries`，`curl https://.../v2/...` 会返回 `000`，走 http 也只会拿到 401/404，看起来像失败，其实镜像好好的，容易误判。

可靠办法是让 Docker 自己验证，用 digest 对齐：

```bash
# 删掉本地镜像再拉回来（若已推上去，digest 会与 push 时打印的一致）
docker rmi harborrepo.hs.com/ops/opscenter:<版本号>
docker pull harborrepo.hs.com/ops/opscenter:<版本号>
```

push 输出的 `digest: sha256:...` 与 pull 回来的 digest 相同，就说明镜像确实在 Harbor 上且内容完整。

## 失败怎么办

| 现象 | 原因 / 处理 |
|---|---|
| `Connection timed out` 拉 github | 代理不通。确认 `172.168.2.219:10808` 可达，或用 `GIT_PROXY=` 覆盖 |
| `merge --ff-only` 失败 | 测试机有本地改动或被强推过。**不要**自动 `reset --hard`，先 `git -C /root/gitlab/opscenter status` 看清楚，把发现告诉用户由他决定 |
| Dockerfile 缺 `ARG APP_VERSION` | 拉到的 commit 早于版本号功能。脚本会直接报错退出，避免推出一个版本号错误的镜像 |
| push 报 `unauthorized` | 在测试机重新 `docker login harborrepo.hs.com` |
| 镜像内 grep 不到版本号 | 构建产物与 tag 不一致，脚本会失败。通常意味着 `APP_VERSION` 没传进前端阶段 |
| 磁盘不足 | `/` 只剩约 15G，必要时 `docker image prune -f`（**先问用户**，prune 会删掉未使用的镜像） |

## 参数

脚本头部的变量可用环境变量覆盖，一般不用改：

```bash
REMOTE_HOST=root@172.168.2.12
REMOTE_DIR=/root/gitlab/opscenter
REGISTRY=harborrepo.hs.com
REPO=ops/opscenter
BRANCH=main
GIT_PROXY=http://172.168.2.219:10808
```

## 不在本 skill 范围

本 skill 只构建并推送镜像，**不重启容器、不改 k8s 部署、不做回滚**。用户要发布到环境时，问清楚目标环境后单独操作。

同版本号重复推送会覆盖 Harbor 上的同名 tag。这是预期行为（同一个 commit 本该产生同一个镜像），不需要额外确认。
