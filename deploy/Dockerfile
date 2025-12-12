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
RUN apk add --no-cache tzdata ca-certificates && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建应用目录和上传目录
RUN mkdir -p /app/static/uploads

# 复制前端构建产物到 nginx 根目录
COPY --from=frontend-builder /build/frontend/dist /usr/share/nginx/html/

# 复制后端二进制
COPY --from=backend-builder /build/backend/app /app/app

# 复制 nginx 配置
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf

# 复制启动脚本
COPY deploy/entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh /app/app

# 环境变量
ENV GIN_MODE=release
EXPOSE 80

CMD ["/docker-entrypoint.sh"]
