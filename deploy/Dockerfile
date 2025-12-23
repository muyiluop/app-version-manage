#########################################
# Multi-stage Dockerfile
# - Stage 1: build frontend (Node.js + Vite)
# - Stage 2: build backend (Go)
# - Stage 3: runtime (nginx + backend)
#########################################

ARG VERSION=latest

### Stage 1: Frontend builder
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm install --registry=https://registry.npmmirror.com
COPY frontend/ .
RUN npm run build

### Stage 2: Backend builder
FROM golang:1.24.2-alpine AS backend-builder
WORKDIR /build/backend
COPY backend/go.mod backend/go.sum ./
ENV GOPROXY=https://goproxy.io,direct
RUN go mod download
COPY backend/ .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o app .

### Stage 3: Runtime (nginx + backend)
FROM nginx:1.27-alpine
ARG VERSION
LABEL version=${VERSION}

# 时区设置
RUN apk add --no-cache tzdata ca-certificates tar gzip shared-mime-info && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建临时目录用于打包
RUN mkdir -p /tmp/appv_init

# 复制前端构建产物
COPY --from=frontend-builder /build/frontend/dist /tmp/appv_init/www/

# 复制后端二进制
COPY --from=backend-builder /build/backend/app /tmp/appv_init/app
RUN chmod +x /tmp/appv_init/app

# 复制配置文件
COPY deploy/config.yaml /tmp/appv_init/config.yaml

# 打包成 tar.gz
RUN mkdir -p /etc/appv && \
    cd /tmp/appv_init && \
    tar czf /etc/appv/init.tar.gz . && \
    rm -rf /tmp/appv_init

# 复制 nginx 配置
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf

# 复制启动脚本
COPY deploy/entrypoint.sh /docker-entrypoint.sh
ENV GIN_MODE=release
EXPOSE 80
WORKDIR /app
ENTRYPOINT ["sh", "/docker-entrypoint.sh"]
