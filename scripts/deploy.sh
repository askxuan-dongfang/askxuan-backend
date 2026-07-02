#!/bin/bash
# askXuan-backend 部署脚本
# 使用方式：./scripts/deploy.sh <service-name> [tag]
# 示例：./scripts/deploy.sh auth-service v1.0.0

set -e

SERVICE=$1
TAG=${2:-latest}

if [ -z "$SERVICE" ]; then
    echo "Usage: $0 <service-name> [tag]"
    echo "Available services: gateway-service auth-service user-service temple-service ..."
    exit 1
fi

# 从服务名推导二进制名（去掉 -service 后缀）
BINARY=$(echo "$SERVICE" | sed 's/-service$//')

echo "==> 构建并推送 Docker 镜像: askxuan/${SERVICE}:${TAG}"
echo "    服务: $SERVICE"
echo "    二进制: $BINARY"

cd "$(dirname "$0")/.."

docker build \
    -f build/docker/Dockerfile \
    --build-arg SERVICE="$SERVICE" \
    --build-arg BINARY="$BINARY" \
    -t "askxuan/${SERVICE}:${TAG}" \
    .

echo "==> 构建完成: askxuan/${SERVICE}:${TAG}"
echo "==> 推送到镜像仓库（如需）:"
echo "    docker push askxuan/${SERVICE}:${TAG}"
