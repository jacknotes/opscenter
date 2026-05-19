.PHONY: build frontend backend clean dev-frontend dev-backend

# 构建前端（输出到 web/dist/）
frontend:
	cd web && npm install && npm run build

# 构建后端（通过 go:embed 嵌入 web/dist/ 中的前端资源）
backend:
	CGO_ENABLED=0 go build -o opscenter ./cmd/server/

# 完整构建（前端 + 后端）
build: frontend backend

# 清理构建产物
clean:
	rm -f opscenter
	rm -rf web/dist web/node_modules

# 开发模式：前端 Vite 开发服务器（:3000），代理 /api 到 :18080
dev-frontend:
	cd web && npm run dev

# 开发模式：Go 后端服务器（:18080）
dev-backend:
	go run ./cmd/server/
