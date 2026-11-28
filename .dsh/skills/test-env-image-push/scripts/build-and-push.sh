#!/usr/bin/env bash
# 在测试环境构建 OpsCenter 镜像并以服务版本号打 tag，推送到 Harbor。
# 用法：bash build-and-push.sh [--dry-run]
set -euo pipefail

# ---------- 可覆盖参数 ----------
REMOTE_HOST="${REMOTE_HOST:-root@172.168.2.12}"
REMOTE_DIR="${REMOTE_DIR:-/root/gitlab/opscenter}"
REGISTRY="${REGISTRY:-harborrepo.hs.com}"
REPO="${REPO:-ops/opscenter}"
BRANCH="${BRANCH:-main}"
GIT_PROXY="${GIT_PROXY:-http://172.168.2.219:10808}"
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=10)

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run|--no-push) DRY_RUN=1 ;;
    -h|--help)
      sed -n '2,3p' "$0" | sed 's/^# \{0,1\}//'
      echo
      echo "选项："
      echo "  --dry-run, --no-push   拉代码并构建，跳过 docker push"
      exit 0
      ;;
    *) echo "未知参数：$arg（可用 --dry-run）" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[31m[失败]\033[0m %s\n' "$*" >&2; exit 1; }

# ---------- 前置检查：SSH 可达 ----------
log "检查到 ${REMOTE_HOST} 的 SSH 连接"
ssh "${SSH_OPTS[@]}" -o ConnectTimeout=8 "$REMOTE_HOST" true \
  || die "无法 SSH 到 ${REMOTE_HOST}。确认测试机可达且本机公钥已授权。"

# ---------- 远端：拉代码 → 算版本 → 构建 → 校验 → 推送 ----------
# 参数以位置参数传入，脚本体用单引号 heredoc，避免本地变量被提前展开。
log "开始远端流程（dry-run=${DRY_RUN}）"
ssh "${SSH_OPTS[@]}" "$REMOTE_HOST" bash -s -- \
  "$REMOTE_DIR" "$BRANCH" "$REGISTRY" "$REPO" "$GIT_PROXY" "$DRY_RUN" <<'REMOTE'
set -euo pipefail

DIR="$1"; BRANCH="$2"; REGISTRY="$3"; REPO="$4"; PROXY="$5"; DRY="$6"

log()  { printf '  \033[36m·\033[0m %s\n' "$*"; }
die()  { printf '  \033[31m[失败]\033[0m %s\n' "$*" >&2; exit 1; }

[ -d "$DIR/.git" ] || die "仓库不存在：$DIR"

cd "$DIR"

# --- 1. 拉最新代码（github 必须走代理，逐条传入不改全局配置） ---
log "拉取 origin/$BRANCH"
git -c "http.proxy=$PROXY" -c "https.proxy=$PROXY" fetch origin "$BRANCH" \
  || die "git fetch 失败。检查代理 $PROXY 是否可达。"

# 工作区若有本地改动，merge 会被拒绝——这是有意的，不自动丢弃用户改动
if ! git diff-index --quiet HEAD -- 2>/dev/null; then
  printf '  \033[33m[警告]\033[0m 工作区有未提交改动：\n' >&2
  git status --porcelain | sed 's/^/    /' >&2
fi

git merge --ff-only "origin/$BRANCH" \
  || die "无法快进到 origin/$BRANCH（本地有分叉或改动）。请先人工确认仓库状态，不要盲目 reset。"

SHA_SHORT=$(git rev-parse --short=7 HEAD)
log "当前提交：$(git log --oneline -1)"

# --- 2. 计算版本号，规则与 web/build/appVersion.js 保持一致 ---
# 有可达 semver tag → v<tag>-<提交数>-g<hash>
# 无可达 tag（describe 退化为裸哈希）→ v<package.json 版本>-<总提交数>-g<hash>
# 两者都可带 -dirty 后缀
PKG_VERSION=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' web/package.json | head -1)
[ -n "$PKG_VERSION" ] || PKG_VERSION="0.0.0"

DESCRIBE=$(git describe --tags --long --always --dirty 2>/dev/null || true)
[ -n "$DESCRIBE" ] || die "git describe 无输出，仓库状态异常"

if [[ "$DESCRIBE" =~ ^v?[0-9]+(\.[0-9]+)+(-[0-9]+-g[0-9a-f]+)?(-dirty)?$ ]]; then
  # 有可达 tag：规范化掉重复的 v 前缀
  VERSION="v${DESCRIBE#v}"
else
  DIRTY=""
  CORE="$DESCRIBE"
  if [[ "$CORE" == *-dirty ]]; then DIRTY="-dirty"; CORE="${CORE%-dirty}"; fi
  if [[ "$CORE" =~ ^[0-9a-f]{4,40}$ ]]; then
    COUNT=$(git rev-list --count HEAD)
    VERSION="v${PKG_VERSION}-${COUNT}-g${CORE}${DIRTY}"
  else
    die "无法识别的 git describe 输出：$DESCRIBE"
  fi
fi

IMAGE="${REGISTRY}/${REPO}:${VERSION}"
log "版本号：$VERSION"
log "镜像：$IMAGE"

# --- 3. 构建前校验：Dockerfile 必须支持版本号注入 ---
# .dockerignore 排除了 .git，构建上下文里没有 git 元数据，
# 不通过 build-arg 传版本号，镜像里只会显示 package.json 的回退值。
grep -q 'ARG APP_VERSION' Dockerfile \
  || die "当前 Dockerfile 没有 'ARG APP_VERSION'（该提交早于版本号功能）。请先更新代码或检查分支。"

# --- 4. 构建 ---
log "docker build（多阶段，含前端 + 后端）"
docker build --build-arg "APP_VERSION=$VERSION" -t "$IMAGE" . \
  || die "镜像构建失败。查看上方 docker 输出定位。"

# --- 5. 校验镜像内的版本号与 tag 一致 ---
log "校验镜像内前端版本号"
FOUND=$(docker run --rm --entrypoint sh "$IMAGE" -c \
  "grep -ohE 'v[0-9]+(\.[0-9]+)+-[0-9]+-g[0-9a-f]+(-dirty)?' /app/web/dist/assets/*.js 2>/dev/null | sort -u" || true)

if [ -z "$FOUND" ]; then
  printf '  \033[33m[警告]\033[0m 未能在镜像内定位版本号字符串，跳过一致性校验\n' >&2
elif printf '%s\n' "$FOUND" | grep -qx "$VERSION"; then
  log "版本号一致：$VERSION"
else
  printf '%s\n' "$FOUND" | sed 's/^/    镜像内实际: /' >&2
  die "镜像内版本号与 tag 不一致（期望 $VERSION）"
fi

# --- 6. 推送 ---
if [ "$DRY" = "1" ]; then
  printf '  \033[33m[dry-run]\033[0m 跳过 docker push\n'
else
  log "docker push $IMAGE"
  docker push "$IMAGE" || die "推送失败。确认测试机已 docker login $REGISTRY。"
fi

echo
echo "RESULT_VERSION=$VERSION"
echo "RESULT_IMAGE=$IMAGE"
echo "RESULT_COMMIT=$SHA_SHORT"
echo "RESULT_PUSHED=$([ "$DRY" = "1" ] && echo no || echo yes)"
REMOTE
