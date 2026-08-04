# syntax=docker/dockerfile:1
# askXuan-backend 通用 Dockerfile（多阶段构建）
# 使用方式（在 askXuan-backend/ 根目录执行）：
#   docker build -f build/docker/Dockerfile --build-arg SERVICE=auth-service --build-arg BINARY=auth -t askxuan/auth-service:latest .
#   docker build -f build/docker/Dockerfile --build-arg SERVICE=gateway-service --build-arg BINARY=gateway -t askxuan/gateway-service:latest .

# ===== Shared runtime base =====
FROM alpine:3.19 AS runtime-base

ENV TZ=Asia/Shanghai

RUN set -eux; \
    for attempt in 1 2 3; do \
      apk add --no-cache ca-certificates tzdata && break; \
      if [ "$attempt" -eq 3 ]; then exit 1; fi; \
      sleep 2; \
    done; \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime; \
    echo "Asia/Shanghai" > /etc/timezone

# ===== Stage 1: builder =====
FROM golang:1.22-alpine AS builder

ARG SERVICE
ARG BINARY

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct

WORKDIR /build

# 复制整个 workspace（go.work 模式需要全部模块）
COPY . .

# 编译指定服务的二进制文件；多服务构建共享模块与编译缓存。
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    cd ${SERVICE} && go build -o /build/${BINARY} ${BINARY}.go

# ===== Stage 2: runner =====
FROM runtime-base

ARG SERVICE
ARG BINARY

RUN if [ "$SERVICE" = "services/infrastructure/file-service" ]; then \
      sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.aliyun.com/alpine|g' /etc/apk/repositories; \
      apk add --no-cache mariadb-client; \
    fi

ENV BINARY=${BINARY}

WORKDIR /app

# 从 builder 阶段复制编译产物
COPY --from=builder /build/${BINARY} /app/${BINARY}
COPY --from=builder /build/${SERVICE}/etc /app/etc

# 非 root 用户运行
RUN adduser -D -u 1000 appuser
USER appuser

EXPOSE 8080

# 健康检查从服务配置读取实际端口。各服务没有统一 HTTP 健康路由，
# 因此在容器内探测监听端口，网关另由启动脚本检查业务健康响应。
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD port="$(awk '$1 == "Port:" { print $2; exit }' /app/etc/*.yaml)" && \
        test -n "$port" && nc -z 127.0.0.1 "$port" || exit 1

# shell 形式以展开 BINARY 变量
CMD /app/${BINARY} -f etc/${BINARY}.yaml
