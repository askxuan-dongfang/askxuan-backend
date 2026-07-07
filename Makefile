# askXuan-backend Go 微服务 - 便捷命令
# 使用方式：在 askXuan-backend/ 目录下执行 make <target>
#
# 依赖：Go 1.22+，建议配置 GOPROXY=https://goproxy.cn,direct
# 基础设施：`docker compose up -d` 启动 MySQL/Redis/RabbitMQ/MinIO/etcd

SHELL := /bin/bash

# ===== 服务路径映射（按业务域分组）=====
GATEWAY_PATH    := services/platform/gateway-service
AUTH_PATH       := services/platform/auth-service
USER_PATH       := services/platform/user-service
TEMPLE_PATH     := services/content/temple-service
MASTER_PATH     := services/content/master-service
BOOKING_PATH    := services/content/booking-service
REVIEW_PATH     := services/content/review-service
PRODUCT_PATH    := services/commerce/product-service
ORDER_PATH      := services/commerce/order-service
PAYMENT_PATH    := services/commerce/payment-service
DIY_PATH        := services/commerce/diy-service
MARKETING_PATH  := services/operation/marketing-service
LOGISTICS_PATH  := services/operation/logistics-service
FINANCE_PATH    := services/operation/finance-service
AUDIT_PATH      := services/operation/audit-service
MESSAGE_PATH    := services/infrastructure/message-service
FILE_PATH       := services/infrastructure/file-service
AI_PATH         := services/infrastructure/ai-service

# 全部服务路径列表
SERVICE_PATHS := $(GATEWAY_PATH) $(AUTH_PATH) $(USER_PATH) $(TEMPLE_PATH) \
                 $(MASTER_PATH) $(BOOKING_PATH) $(REVIEW_PATH) $(PRODUCT_PATH) \
                 $(ORDER_PATH) $(PAYMENT_PATH) $(DIY_PATH) $(MARKETING_PATH) \
                 $(LOGISTICS_PATH) $(FINANCE_PATH) $(AUDIT_PATH) $(MESSAGE_PATH) \
                 $(FILE_PATH) $(AI_PATH)

SERVICE_BINS := gateway auth user temple master booking review product order payment diy marketing logistics finance audit message file ai
SERVICE_PORTS := gateway:8080 auth:8081 user:8082 temple:8083 master:8084 booking:8085 product:8086 diy:8088 order:8089 payment:8090 finance:8091 review:8092 audit:8093 message:8094 logistics:8095 marketing:8096 file:8097 ai:8098
CORE_SERVICE_PATHS := $(GATEWAY_PATH) $(AUTH_PATH) $(MESSAGE_PATH)
CORE_SERVICE_BINS := gateway auth message
CORE_SERVICE_PORTS := gateway:8080 auth:8081 message:8094

# 默认 GOPROXY 加速
export GOPROXY ?= https://goproxy.cn,direct
LOG_DIR := $(CURDIR)/logs
PID_DIR := $(LOG_DIR)/pids

.PHONY: help tidy build run-all start-all start-core stop-core stop-all db-init db-reset clean \
        test test-verbose vet lint fmt docker-build docker-build-all swagger \
        docker-config docker-up docker-down docker-restart docker-ps docker-logs \
        stack-preflight stack-up stack-down stack-restart stack-check stack-ps stack-logs \
        start-gateway start-auth start-user start-temple start-master start-booking \
        start-message start-file start-product start-diy start-order start-payment \
        start-finance start-review start-audit start-logistics start-marketing start-ai \
        build-gateway build-auth build-user build-temple build-master build-booking \
        build-message build-file build-product build-diy build-order build-payment \
        build-finance build-review build-audit build-logistics build-marketing build-ai

help: ## 查看所有命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

tidy: ## 拉取/整理所有模块依赖
	@for path in $(SERVICE_PATHS) common; do \
		echo "==> go mod tidy: $$path"; \
		(cd $$path && go mod tidy) || echo "  (跳过 $$path)"; \
	done

# ---------- 单服务启动 ----------
start-gateway: ## 启动网关 (8080)
	cd $(GATEWAY_PATH) && go run gateway.go -f etc/gateway.yaml

start-auth: ## 启动认证服务 (8081)
	cd $(AUTH_PATH) && go run auth.go -f etc/auth.yaml

start-user: ## 启动用户服务 (8082)
	cd $(USER_PATH) && go run user.go -f etc/user.yaml

start-temple: ## 启动寺院服务 (8083)
	cd $(TEMPLE_PATH) && go run temple.go -f etc/temple.yaml

start-master: ## 启动法师服务 (8084)
	cd $(MASTER_PATH) && go run master.go -f etc/master.yaml

start-booking: ## 启动预约服务 (8085)
	cd $(BOOKING_PATH) && go run booking.go -f etc/booking.yaml

start-message: ## 启动消息服务 (8094)
	cd $(MESSAGE_PATH) && go run message.go -f etc/message.yaml

start-file: ## 启动文件服务 (8097)
	cd $(FILE_PATH) && go run file.go -f etc/file.yaml

start-product: ## 启动商品服务 (8086)
	cd $(PRODUCT_PATH) && go run product.go -f etc/product.yaml

start-diy: ## 启动DIY服务 (8088)
	cd $(DIY_PATH) && go run diy.go -f etc/diy.yaml

start-order: ## 启动订单服务 (8089)
	cd $(ORDER_PATH) && go run order.go -f etc/order.yaml

start-payment: ## 启动支付服务 (8090)
	cd $(PAYMENT_PATH) && go run payment.go -f etc/payment.yaml

start-finance: ## 启动财务服务 (8091)
	cd $(FINANCE_PATH) && go run finance.go -f etc/finance.yaml

start-review: ## 启动评价服务 (8092)
	cd $(REVIEW_PATH) && go run review.go -f etc/review.yaml

start-audit: ## 启动审核服务 (8093)
	cd $(AUDIT_PATH) && go run audit.go -f etc/audit.yaml

start-logistics: ## 启动物流服务 (8095)
	cd $(LOGISTICS_PATH) && go run logistics.go -f etc/logistics.yaml

start-marketing: ## 启动营销服务 (8096)
	cd $(MARKETING_PATH) && go run marketing.go -f etc/marketing.yaml

start-ai: ## 启动AI服务 (8098)
	cd $(AI_PATH) && go run ai.go -f etc/ai.yaml

# ---------- 单服务编译 ----------
build-gateway:
	cd $(GATEWAY_PATH) && go build -o gateway gateway.go

build-auth:
	cd $(AUTH_PATH) && go build -o auth auth.go

build-user:
	cd $(USER_PATH) && go build -o user user.go

build-temple:
	cd $(TEMPLE_PATH) && go build -o temple temple.go

build-master:
	cd $(MASTER_PATH) && go build -o master master.go

build-booking:
	cd $(BOOKING_PATH) && go build -o booking booking.go

build-message:
	cd $(MESSAGE_PATH) && go build -o message message.go

build-file:
	cd $(FILE_PATH) && go build -o file file.go

build-product:
	cd $(PRODUCT_PATH) && go build -o product product.go

build-diy:
	cd $(DIY_PATH) && go build -o diy diy.go

build-order:
	cd $(ORDER_PATH) && go build -o order order.go

build-payment:
	cd $(PAYMENT_PATH) && go build -o payment payment.go

build-finance:
	cd $(FINANCE_PATH) && go build -o finance finance.go

build-review:
	cd $(REVIEW_PATH) && go build -o review review.go

build-audit:
	cd $(AUDIT_PATH) && go build -o audit audit.go

build-logistics:
	cd $(LOGISTICS_PATH) && go build -o logistics logistics.go

build-marketing:
	cd $(MARKETING_PATH) && go build -o marketing marketing.go

build-ai:
	cd $(AI_PATH) && go build -o ai ai.go

build: ## 编译所有服务
build: build-gateway build-auth build-user build-temple build-master build-booking \
       build-message build-file build-product build-diy build-order build-payment \
       build-finance build-review build-audit build-logistics build-marketing build-ai
	@echo "==> 所有服务编译完成"

# ---------- 全部启动（后台并发，日志输出到 logs/） ----------
start-all: build ## 编译并后台启动所有服务（PID 写入 logs/pids/）
	@mkdir -p $(PID_DIR)
	@echo "==> 停止旧服务..."
	@$(MAKE) -s stop-all
	@echo "==> 预检服务端口..."
	@for pair in $(SERVICE_PORTS); do \
		bin=$$(printf '%s\n' "$$pair" | cut -d: -f1); \
		port=$$(printf '%s\n' "$$pair" | cut -d: -f2); \
		pids=$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true); \
		if [ -n "$$pids" ]; then \
			echo "  端口 $$port 被占用（$$bin），清理 PID: $$pids"; \
			kill $$pids 2>/dev/null || true; \
			sleep 1; \
		fi; \
	done
	@echo "==> 后台启动所有服务，日志见 logs/"
	@for path in $(SERVICE_PATHS); do \
		bin=$$(ls $$path/*.go | head -1 | sed 's|.*/||;s|\.go$$||'); \
		echo "  启动 $$path ($$bin)"; \
		(cd $$path && nohup ./$$bin -f etc/$$bin.yaml > $(LOG_DIR)/$$bin.log 2>&1 & echo $$! > $(PID_DIR)/$$bin.pid) || exit 1; \
	done
	@echo "==> 等待 gateway 健康检查..."
	@ok=0; \
	for i in $$(seq 1 30); do \
		if curl -fsS http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then ok=1; break; fi; \
		sleep 1; \
	done; \
	if [ "$$ok" != "1" ]; then \
		echo "==> gateway 启动失败，最近日志："; \
		tail -80 $(LOG_DIR)/gateway.log 2>/dev/null || true; \
		exit 1; \
	fi
	@echo "==> 已启动。健康检查：http://127.0.0.1:8080/api/v1/health"
	@echo "==> 停止：make stop-all"

start-core: build-gateway build-auth build-message ## 低内存启动核心通信服务（gateway/auth/message）
	@mkdir -p $(PID_DIR)
	@echo "==> 停止旧核心服务..."
	@$(MAKE) -s stop-core
	@echo "==> 预检核心服务端口..."
	@for pair in $(CORE_SERVICE_PORTS); do \
		bin=$$(printf '%s\n' "$$pair" | cut -d: -f1); \
		port=$$(printf '%s\n' "$$pair" | cut -d: -f2); \
		pids=$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true); \
		if [ -n "$$pids" ]; then \
			echo "  端口 $$port 被占用（$$bin），清理 PID: $$pids"; \
			kill $$pids 2>/dev/null || true; \
			sleep 1; \
		fi; \
	done
	@echo "==> 后台启动核心通信服务，日志见 logs/"
	@for path in $(CORE_SERVICE_PATHS); do \
		bin=$$(ls $$path/*.go | head -1 | sed 's|.*/||;s|\.go$$||'); \
		echo "  启动 $$path ($$bin)"; \
		(cd $$path && nohup ./$$bin -f etc/$$bin.yaml > $(LOG_DIR)/$$bin.log 2>&1 & echo $$! > $(PID_DIR)/$$bin.pid) || exit 1; \
	done
	@echo "==> 等待 gateway 健康检查..."
	@ok=0; \
	for i in $$(seq 1 30); do \
		if curl -fsS http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then ok=1; break; fi; \
		sleep 1; \
	done; \
	if [ "$$ok" != "1" ]; then \
		echo "==> gateway 启动失败，最近日志："; \
		tail -80 $(LOG_DIR)/gateway.log 2>/dev/null || true; \
		exit 1; \
	fi
	@echo "==> 核心通信服务已启动。健康检查：http://127.0.0.1:8080/api/v1/health"

stop-core: ## 停止核心通信服务
	@echo "==> 停止核心通信服务..."
	@for bin in $(CORE_SERVICE_BINS); do \
		if [ -f $(PID_DIR)/$$bin.pid ]; then \
			pid=$$(cat $(PID_DIR)/$$bin.pid); \
			kill $$pid 2>/dev/null || true; \
			rm -f $(PID_DIR)/$$bin.pid; \
		fi; \
	done
	@echo "==> 核心通信服务已停止"

stop-all: ## 停止所有服务
	@echo "==> 停止所有服务..."
	@for bin in $(SERVICE_BINS); do \
		if [ -f $(PID_DIR)/$$bin.pid ]; then \
			pid=$$(cat $(PID_DIR)/$$bin.pid); \
			kill $$pid 2>/dev/null || true; \
			rm -f $(PID_DIR)/$$bin.pid; \
		fi; \
	done
	@-pkill -f "/askXuan-backend/services/.*/\(gateway\|auth\|user\|temple\|master\|booking\|review\|product\|order\|payment\|diy\|marketing\|logistics\|finance\|audit\|message\|file\|ai\) " 2>/dev/null || true
	@echo "==> 已停止"

# ---------- 数据库 ----------
db-init: ## 初始化数据库（建表 + 种子数据）
	@echo "==> 初始化数据库..."
	@docker exec -i askxuan-mysql mysql -uroot -proot123 askxuan < db/init.sql && echo "==> 数据库初始化完成" || echo "==> 失败：请先执行 docker compose up -d"

db-reset: ## 重置本地数据库（危险：删除 askxuan* 业务库后重新初始化）
	@echo "==> 即将删除 askxuan* 业务库并重新初始化..."
	@docker exec askxuan-mysql mysql -uroot -proot123 -e "SET FOREIGN_KEY_CHECKS=0; DROP DATABASE IF EXISTS askxuan; DROP DATABASE IF EXISTS askxuan_auth; DROP DATABASE IF EXISTS askxuan_user; DROP DATABASE IF EXISTS askxuan_temple; DROP DATABASE IF EXISTS askxuan_master; DROP DATABASE IF EXISTS askxuan_booking; DROP DATABASE IF EXISTS askxuan_message; DROP DATABASE IF EXISTS askxuan_shop; DROP DATABASE IF EXISTS askxuan_diy; DROP DATABASE IF EXISTS askxuan_finance; DROP DATABASE IF EXISTS askxuan_review; DROP DATABASE IF EXISTS askxuan_audit; DROP DATABASE IF EXISTS askxuan_logistics; DROP DATABASE IF EXISTS askxuan_marketing; DROP DATABASE IF EXISTS askxuan_ai; DROP DATABASE IF EXISTS askxuan_system; CREATE DATABASE askxuan DEFAULT CHARACTER SET utf8mb4; SET FOREIGN_KEY_CHECKS=1;"
	@$(MAKE) -s db-init

# ---------- 清理 ----------
clean: ## 清理编译产物
	@-find services -type f \( -name "gateway" -o -name "auth" -o -name "user" -o -name "temple" \
		-o -name "master" -o -name "booking" -o -name "message" -o -name "file" \
		-o -name "product" -o -name "diy" -o -name "order" -o -name "payment" \
		-o -name "finance" -o -name "review" -o -name "audit" \
		-o -name "logistics" -o -name "marketing" -o -name "ai" \) -delete
	@-rm -rf logs
	@echo "==> 已清理"

# ---------- 测试 ----------
test: ## 运行所有模块单元测试
	@for path in $(SERVICE_PATHS) common; do \
		echo "==> go test: $$path"; \
		(cd $$path && go test ./... -count=1) || echo "  (测试失败 $$path)"; \
	done

test-verbose: ## 运行所有模块单元测试（详细输出）
	@for path in $(SERVICE_PATHS) common; do \
		echo "==> go test -v: $$path"; \
		(cd $$path && go test ./... -v -count=1) || echo "  (测试失败 $$path)"; \
	done

# ---------- 代码检查 ----------
vet: ## 运行 go vet 静态检查
	@for path in $(SERVICE_PATHS) common; do \
		echo "==> go vet: $$path"; \
		(cd $$path && go vet ./...) || echo "  (vet 失败 $$path)"; \
	done

lint: ## 运行 golangci-lint（需先安装: brew install golangci-lint）
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		echo "==> golangci-lint 未安装，请执行: brew install golangci-lint"; \
		exit 1; \
	fi
	@for path in $(SERVICE_PATHS) common; do \
		echo "==> lint: $$path"; \
		(cd $$path && golangci-lint run ./... --timeout 5m) || echo "  (lint 警告 $$path)"; \
	done

fmt: ## 格式化所有 Go 代码
	@for path in $(SERVICE_PATHS) common; do \
		(cd $$path && go fmt ./...) || true; \
	done
	@echo "==> 代码格式化完成"

# ---------- Docker ----------
docker-build: ## 构建单个服务 Docker 镜像（用法: make docker-build SVC=auth-service TAG=latest）
	@if [ -z "$(SVC)" ]; then echo "用法: make docker-build SVC=auth-service [TAG=latest]"; exit 1; fi
	@path=$$(find services -name "$(SVC)" -type d | head -1); \
	binary=$$(echo "$(SVC)" | sed 's/-service$$//'); \
	echo "==> 构建 askxuan/$(SVC):$(or $(TAG),latest) (path=$$path, binary=$$binary)"; \
	docker build -f build/docker/Dockerfile \
		--build-arg SERVICE=$$path \
		--build-arg BINARY=$$binary \
		-t askxuan/$(SVC):$(or $(TAG),latest) .

docker-build-all: ## 构建所有服务 Docker 镜像
	@TAG=$(or $(TAG),latest); bash scripts/docker-build-all.sh $$TAG

docker-config: ## 生成 Docker Compose 容器网络配置（.docker/etc）
	@bash scripts/dev/render-docker-configs.sh

docker-up: ## 一键启动中间件 + 首次初始化数据库 + 18 个后端服务（Docker Compose）
	@bash scripts/dev/docker-up-all.sh

docker-down: ## 停止 Docker Compose 全量后端（保留 MySQL/Redis 等数据卷）
	@docker compose -f docker-compose.yml -f docker-compose.full.yml down

docker-restart: docker-down docker-up ## 重启 Docker Compose 全量后端

docker-ps: ## 查看 Docker Compose 全量后端容器状态
	@docker compose -f docker-compose.yml -f docker-compose.full.yml ps

docker-logs: ## 查看 Docker Compose 后端日志（可传 SVC=gateway-service）
	@if [ -n "$(SVC)" ]; then \
		docker compose -f docker-compose.yml -f docker-compose.full.yml logs -f --tail=200 $(SVC); \
	else \
		docker compose -f docker-compose.yml -f docker-compose.full.yml logs -f --tail=200; \
	fi

stack-preflight: ## 全栈启动前预检端口/Docker/Compose（askXuan + OpenIM）
	@bash scripts/dev/stack-preflight.sh

stack-up: ## 一键启动完整后端栈：OpenIM + askXuan 中间件 + askXuan 18 个服务
	@bash scripts/dev/stack-up.sh

stack-down: ## 停止完整后端栈（保留数据）
	@bash scripts/dev/stack-down.sh

stack-restart: stack-down stack-up ## 重启完整后端栈

stack-check: ## 检查完整后端栈健康状态
	@bash scripts/dev/stack-check.sh

stack-ps: ## 查看 askXuan 与 OpenIM 容器状态
	@docker compose -f docker-compose.yml -f docker-compose.full.yml ps
	@echo ""
	@OPENIM_DIR=".local/openim/open-im-server-3.8.3"; \
	if [ -f "$$OPENIM_DIR/docker-compose.yml" ]; then \
		docker compose -f "$$OPENIM_DIR/docker-compose.yml" ps; \
	else \
		echo "OpenIM compose file not found: $$OPENIM_DIR/docker-compose.yml"; \
	fi

stack-logs: ## 查看全栈日志（askXuan 用 SVC=xxx；OpenIM 请用 openim 自带 compose 查看）
	@if [ -n "$(SVC)" ]; then \
		docker compose -f docker-compose.yml -f docker-compose.full.yml logs -f --tail=200 $(SVC); \
	else \
		docker compose -f docker-compose.yml -f docker-compose.full.yml logs -f --tail=200; \
	fi

# ---------- Swagger 文档 ----------
swagger: ## 生成 Swagger API 文档（需安装 goctl-swagger 插件）
	@echo "==> 生成 Swagger 文档..."
	@for path in $(SERVICE_PATHS); do \
		if [ -f $$path/api/*.api ]; then \
			echo "  生成 $$path Swagger..."; \
			(cd $$path && goctl api plugin -plugin goctl-swagger="swagger -filename $$(basename $$(pwd)).json" -api api/*.api -dir ../../docs/api 2>/dev/null) || true; \
		fi; \
	done
	@echo "==> Swagger 文档生成完成（见 docs/api/）"
