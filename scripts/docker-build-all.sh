#!/bin/bash
# askXuan-backend 批量构建 Docker 镜像
# 使用方式：./scripts/docker-build-all.sh [tag]
# 示例：./scripts/docker-build-all.sh v1.0.0

set -e

TAG=${1:-latest}

cd "$(dirname "$0")/.."

# 服务名:路径 映射（按业务域分组）
declare -A SERVICES=(
    ["gateway-service"]="services/platform/gateway-service"
    ["auth-service"]="services/platform/auth-service"
    ["user-service"]="services/platform/user-service"
    ["temple-service"]="services/content/temple-service"
    ["master-service"]="services/content/master-service"
    ["booking-service"]="services/content/booking-service"
    ["review-service"]="services/content/review-service"
    ["product-service"]="services/commerce/product-service"
    ["order-service"]="services/commerce/order-service"
    ["payment-service"]="services/commerce/payment-service"
    ["diy-service"]="services/commerce/diy-service"
    ["marketing-service"]="services/operation/marketing-service"
    ["logistics-service"]="services/operation/logistics-service"
    ["finance-service"]="services/operation/finance-service"
    ["audit-service"]="services/operation/audit-service"
    ["message-service"]="services/infrastructure/message-service"
    ["file-service"]="services/infrastructure/file-service"
    ["ai-service"]="services/infrastructure/ai-service"
)

echo "==> 批量构建 Docker 镜像 (tag=${TAG})"

for svc in "${!SERVICES[@]}"; do
    path="${SERVICES[$svc]}"
    binary=$(echo "$svc" | sed 's/-service$//')
    echo "  构建 askxuan/${svc}:${TAG} (path=${path}, binary=${binary})"
    docker build \
        -f build/docker/Dockerfile \
        --build-arg SERVICE="$path" \
        --build-arg BINARY="$binary" \
        -t "askxuan/${svc}:${TAG}" \
        . 2>&1 | tail -3
    echo "  完成: askxuan/${svc}:${TAG}"
done

echo "==> 全部构建完成 (${#SERVICES[@]} 个服务)"
