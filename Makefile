.PHONY: build frontend backend swagger clean dev-frontend dev-backend deps

# 生成 swagger 文档
swagger:
	swag init -d cmd/server,internal/handler,internal/model,internal/service -o docs

# 构建前端（输出到 web/dist/）
frontend:
	cd web && npm install && npm run build

# 构建后端（通过 go:embed 嵌入 web/dist/ 中的前端资源）
backend: swagger
	CGO_ENABLED=0 go build -o opscenter ./cmd/server/

# 完整构建（前端 + 后端）
build: frontend backend

# 清理构建产物
clean:
	rm -f opscenter
	rm -rf web/dist web/node_modules docs

# 安装依赖（前端 npm + 后端 go mod）
deps:
	cd web && npm install
	go mod tidy

# 开发模式：前端 Vite 开发服务器（:3000），代理 /api 到 :18080
dev-frontend:
	cd web && npm run dev

# 开发模式：Go 后端服务器（:18080）
dev-backend: swagger
	GIN_MODE=debug go run ./cmd/server/

# 生产模式：Go 后端服务器（:18080）
prod-backend: swagger
	GIN_MODE=release go run ./cmd/server/
