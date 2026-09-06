APP_NAME := bill
APP_PORT ?= 8907
VERSION := $(shell cat VERSION | tr -d '\n')
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X smallgo/server/version.Version=$(VERSION) -X smallgo/server/version.BuildTime=$(BUILD_TIME) -X smallgo/server/version.GitCommit=$(GIT_COMMIT) -X smallgo/server/version.AppName=$(APP_NAME)
LOCAL_GOCACHE ?= /tmp/bill-go-build

.PHONY: help dev start build build-frontend build-backend build-linux build-docker build-all fnpack clean

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  help            Show this help"
	@echo "  dev             Build to dev/ directory and start (simulates production)"
	@echo "  start           Start the server directly"
	@echo "  build           Build frontend + backend"
	@echo "  build-frontend  Build frontend only"
	@echo "  build-backend   Build backend only"
	@echo "  build-linux     Build for linux/amd64"
	@echo "  build-docker    Build an offline local image for this Docker architecture"
	@echo "  build-all       Build for all platforms"
	@echo "  fnpack          Build fnOS packages"
	@echo "  clean           Clean build artifacts"

dev:
	@printf "\n\033[1;36m▶ 账单开发环境\033[0m\n\n"
	@printf "\033[1;34m→\033[0m 清理开发目录（保留数据）...\n"
	@rm -rf dev/static dev/$(APP_NAME)
	@mkdir -p dev/static/dist dev/data
	@printf "\033[1;32m✓\033[0m 开发目录已就绪\n"
	@printf "\033[1;34m→\033[0m 构建前端...\n"
	@cd web && npm ci --loglevel=error --no-audit --no-fund >/dev/null && npm run build --silent >/dev/null
	@cp -r web/dist/* dev/static/dist/
	@printf "\033[1;32m✓\033[0m 前端构建完成\n"
	@printf "\033[1;34m→\033[0m 构建后端...\n"
	@cd server && GOCACHE=$(LOCAL_GOCACHE) go build -ldflags "$(LDFLAGS)" -o ../dev/$(APP_NAME) .
	@printf "\033[1;32m✓\033[0m 后端构建完成\n"
	@printf "\033[1;34m→\033[0m 检查端口 $(APP_PORT)...\n"
	@bash scripts/stop-port.sh "$(APP_PORT)" >/dev/null
	@printf "\033[1;32m✓\033[0m 端口已就绪\n"
	@lan_ip="$$(bash scripts/lan-ip.sh)"; \
		printf "\n\033[1;32m✓ 构建完成\033[0m\n\033[1m  访问地址\033[0m\n"; \
		printf "  本机地址  \033[1;36mhttp://localhost:$(APP_PORT)\033[0m\n"; \
		printf "  回环地址  \033[1;36mhttp://127.0.0.1:$(APP_PORT)\033[0m\n"; \
		if [ -n "$$lan_ip" ]; then \
			printf "  内网地址  \033[1;36mhttp://%s:$(APP_PORT)\033[0m\n" "$$lan_ip"; \
		else \
			printf "  内网地址  \033[2m未检测到可用的私有 IPv4\033[0m\n"; \
		fi; \
		printf "\n  按 Ctrl+C 停止服务\n\n"
	@cd dev && ./$(APP_NAME) -port="$(APP_PORT)" -data-dir=./data -web-dir=./static/dist

start:
	cd server && GOCACHE=$(LOCAL_GOCACHE) go run -ldflags "$(LDFLAGS)" .

build: build-frontend build-backend

build-frontend:
	cd web && npm ci && npm run build
	rm -rf server/static/dist
	mkdir -p server/static
	cp -r web/dist server/static/dist

build-backend:
	cd server && GOCACHE=$(LOCAL_GOCACHE) go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

build-linux:
	docker run --rm \
		-v "$(CURDIR)/server:/src" \
		-v "go-build-cache:/root/.cache/go-build" \
		-v "go-mod-cache:/go/pkg/mod" \
		-w /src \
		--platform linux/amd64 \
		-e "LDFLAGS=$(LDFLAGS)" \
		golang:1.26-alpine \
		sh -c 'apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags "$$LDFLAGS -extldflags -static" -o bill-linux-amd64 .'

build-docker:
	bash scripts/build-docker.sh

build-all:
	bash scripts/build-all.sh

fnpack:
	bash scripts/build-fnpack.sh

clean:
	rm -f server/$(APP_NAME) server/$(APP_NAME)-*
	rm -rf server/static/dist
	rm -rf web/dist
	rm -rf build/
	rm -rf dev/static dev/$(APP_NAME)
