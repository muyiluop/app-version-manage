#########################################
# Multi-stage Dockerfile
# - frontend-builder: build React (Vite) frontend
# - backend-builder: build Go backend
# - final: nginx image containing static frontend and backend binary
#########################################

ARG VERSION=latest

### Frontend builder
FROM node:20-alpine AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm install --silent
COPY frontend/ .
# build output (Vite default: dist)
RUN npm run build

### Backend builder
FROM golang:1.24.2-alpine AS backend-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN mkdir -p /go/pkg && go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY backend/ .
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -o app .

### Final image
FROM nginx:1.28.0-alpine-slim
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
ARG VERSION
LABEL org.opencontainers.image.version=${VERSION}
WORKDIR /app

RUN set -eux && sed -i 's#https\?://dl-cdn.alpinelinux.org/alpine#https://mirrors.tuna.tsinghua.edu.cn/alpine#g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

RUN mkdir -p /app/static/uploads

# copy built frontend (assume Vite output at frontend/dist)
COPY --from=frontend-builder /src/frontend/dist /app

# copy backend binary
COPY --from=backend-builder /src/backend/app /app/app

# copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

RUN chmod +x /app/app || true

ENV GIN_MODE=release
EXPOSE 80

COPY entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

CMD ["/docker-entrypoint.sh"]
