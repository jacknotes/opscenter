# Stage 1: Build frontend
FROM node:24-alpine AS frontend-builder

WORKDIR /app/web

# 可选：镜像内无 .git，可在此显式注入版本号（如 v2.0.1-7-g401385d），
# 不传则由 web/build/appVersion.js 回退到 package.json 版本
ARG APP_VERSION
ENV APP_VERSION=${APP_VERSION}

COPY web/package*.json ./
RUN npm config set registry https://registry.npmmirror.com && npm ci

COPY web/ .
RUN npm run build

# Stage 2: Build backend with embedded frontend
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 生成 swagger 文档
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -d cmd/server,internal/handler,internal/model,internal/service -o docs

RUN CGO_ENABLED=0 GOOS=linux go build -o opscenter ./cmd/server/

# Stage 3: Final minimal image
FROM alpine:3.19

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata

# 设置默认时区为 Asia/Shanghai，确保 Go 的 time.Now() 和 MySQL DATE_FORMAT 使用一致的时区
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=backend-builder /app/opscenter .
COPY --from=frontend-builder /app/web/dist ./web/dist
COPY conf/ /app/config/
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 18080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
