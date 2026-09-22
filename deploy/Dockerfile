# syntax=docker/dockerfile:1
###############################################################################
# 多阶段构建：前端(npm) -> 后端(go) -> 运行(nginx + 后端二进制)
#
# 特点：
#   · 单镜像形态（nginx 与后端同容器），持久化数据统一放 /data 卷；
#   · 用 TARGETOS/TARGETARCH 而非写死架构，可直接构建 arm64 镜像；
#   · 依赖与编译缓存走 BuildKit cache mount，重复构建显著更快；
#   · 运行镜像内的配置模板不含任何密钥，密钥由 APPV_* 环境变量注入。
#
# 构建示例：
#   docker build -f deploy/Dockerfile -t app-version-manage:dev .
#   docker buildx build --platform linux/amd64 -f deploy/Dockerfile .
#
# 关于 arm64：本文件支持（用 TARGETARCH 而非写死架构），但 CI 只出 amd64。
# 在 amd64 机器上构建 arm64 需要 QEMU 模拟，前端 tsc/vite 与 go build 会慢数倍；
# 且前端产物与架构无关，多平台构建会把它重复构建一遍。要用 arm64 请上原生 arm64 runner。
###############################################################################

ARG NODE_VERSION=20
ARG GO_VERSION=1.26
ARG NGINX_VERSION=1.27

### Stage 1: 前端构建
FROM node:${NODE_VERSION}-alpine AS frontend-builder
# 缓存按架构隔离：esbuild 等依赖含平台相关的原生二进制
ARG TARGETARCH
WORKDIR /build/frontend

# 先只拷贝依赖清单：源码变动时仍可复用依赖层
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,id=npm-${TARGETARCH},target=/root/.npm \
    npm ci --no-audit --no-fund --registry=https://registry.npmmirror.com

COPY frontend/ ./
RUN npm run build

### Stage 2: 后端构建
FROM golang:${GO_VERSION}-alpine AS backend-builder
WORKDIR /build/backend

# 由 buildx 注入目标平台；直接 go build 时回落到 linux/amd64
ARG TARGETOS
ARG TARGETARCH

COPY backend/go.mod backend/go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN --mount=type=cache,id=gomod-${TARGETARCH},target=/go/pkg/mod go mod download

COPY backend/ ./
ENV CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}

RUN --mount=type=cache,id=gomod-${TARGETARCH},target=/go/pkg/mod \
    --mount=type=cache,id=gobuild-${TARGETARCH},target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /out/appv ./cmd/server && \
    go build -trimpath -ldflags "-s -w" -o /out/appv-migrate ./cmd/migrate

### Stage 3: 运行镜像
FROM nginx:${NGINX_VERSION}-alpine

ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="app-version-manage" \
      org.opencontainers.image.description="软件版本发布系统（nginx + Go 单镜像）" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

# 时区与根证书（访问对象存储/数据库需要）
RUN apk add --no-cache tzdata ca-certificates && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=frontend-builder /build/frontend/dist /app/www
COPY --from=backend-builder /out/appv /app/appv
# 迁移工具：目标库/对象存储只在容器网络内可达时，可直接在容器里执行迁移
COPY --from=backend-builder /out/appv-migrate /app/appv-migrate

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/entrypoint.sh /docker-entrypoint.sh
# 运行时配置：不含密钥的模板，默认指向 /data 卷
COPY deploy/config.example.yaml /app/config.yaml
# 完整配置说明随镜像分发，便于容器内排障时查阅
COPY backend/config.example.yaml /app/config.full-example.yaml

RUN chmod +x /app/appv /app/appv-migrate /docker-entrypoint.sh && \
    mkdir -p /data/uploads

# 生产模式默认值；敏感项（JWT_SECRET / DOWNLOAD_TOKEN_KEY）必须由运行时环境变量注入
ENV APPV_SERVER_MODE=release \
    APPV_SERVER_PORT=9080 \
    APPV_DATABASE_DRIVER=sqlite \
    APPV_DATABASE_PATH=/data/app_version.db \
    APPV_STORAGE_DRIVER=local \
    APPV_STORAGE_LOCAL_ROOT=/data/uploads \
    APPV_LOG_FORMAT=json

VOLUME ["/data"]

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9080/healthz || exit 1

# PID 1 是 entrypoint 脚本：负责拉起后端与 nginx，并转发信号做优雅停机
ENTRYPOINT ["/docker-entrypoint.sh"]
