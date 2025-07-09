# # 第一阶段：构建前端
# FROM node:20-alpine AS frontend-builder
# WORKDIR /app/frontend
# # 复制前端项目文件
# COPY ../frontend/package*.json ./
# RUN npm install
# COPY ../frontend/ .
# # 构建前端
# RUN npm run build

# # 第二阶段：构建后端
# FROM golang:1.24.2-alpine AS backend-builder
# WORKDIR /app/backend
# # 复制后端代码
# COPY ../backend/ .
# # 设置构建环境变量
# ENV CGO_ENABLED=0
# ENV GOOS=linux
# ENV GOARCH=amd64
# ENV GOPROXY=https://goproxy.cn,direct
# # 构建后端
# RUN go build -o app .

# 第三阶段：最终镜像
FROM nginx:1.28.0-alpine-slim
WORKDIR /app
RUN set -eux && sed -i 's#https\?://dl-cdn.alpinelinux.org/alpine#https://mirrors.tuna.tsinghua.edu.cn/alpine#g' /etc/apk/repositories
# 安装基本依赖
RUN apk --no-cache add ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建必要的目录
RUN mkdir -p /app/static/uploads

# 从前端构建阶段复制文件
COPY ./www /app

# 从后端构建阶段复制文件
COPY app /app/

# 复制 nginx 配置
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 设置权限
RUN chmod +x /app/app

# 设置环境变量
ENV GIN_MODE=release

# 暴露端口
EXPOSE 80

# 复制并设置启动脚本
COPY entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# 启动命令
CMD ["/docker-entrypoint.sh"]
