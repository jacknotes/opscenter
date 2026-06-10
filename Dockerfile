# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

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
