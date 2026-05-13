.PHONY: build frontend backend clean dev-frontend dev-backend

# Build frontend
frontend:
	cd web && npm install && npm run build

# Build backend (embed frontend)
backend:
	CGO_ENABLED=0 go build -o opscenter ./cmd/server/

# Full build
build: frontend backend

# Clean
clean:
	rm -f opscenter
	rm -rf web/dist web/node_modules

# Development mode
dev-frontend:
	cd web && npm run dev

dev-backend:
	go run ./cmd/server/
