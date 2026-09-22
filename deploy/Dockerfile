# syntax=docker/dockerfile:1
############################################################
# 多阶段构建：前端(npm) -> 后端(go) -> 运行(nginx + 二进制)
# 运行镜像保持单镜像形态；持久化数据统一放在 /data 卷。
############################################################

### Stage 1: 前端构建
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --registry=https://registry.npmmirror.com
COPY frontend/ ./
RUN npm run build

### Stage 2: 后端构建
FROM golang:1.26-alpine AS backend-builder
WORKDIR /build/backend
COPY backend/go.mod backend/go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY backend/ ./
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags "-s -w" -o /out/appv ./cmd/server && \
    go build -trimpath -ldflags "-s -w" -o /out/appv-migrate ./cmd/migrate

### Stage 3: 运行镜像
FROM nginx:1.27-alpine

ARG VERSION=dev
LABEL org.opencontainers.image.title="app-version-manage" \
      org.opencontainers.image.version="${VERSION}"

# 时区与证书
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
# 使用不含密钥的模板；真实密钥通过 APPV_* 环境变量在运行时注入
COPY deploy/config.example.yaml /app/config.yaml

RUN chmod +x /app/appv /app/appv-migrate /docker-entrypoint.sh && mkdir -p /data/uploads

# 生产模式默认值；敏感项（JWT_SECRET 等）必须由运行时环境变量注入
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

ENTRYPOINT ["/docker-entrypoint.sh"]
