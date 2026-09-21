#!/bin/sh
# 启动脚本：先拉起后端，再用 nginx 作为前台进程。
# 配置全部来自环境变量（APPV_ 前缀），无需修改镜像内文件。
set -e

DB_PATH="${APPV_DATABASE_PATH:-/data/app_version.db}"
STORAGE_ROOT="${APPV_STORAGE_LOCAL_ROOT:-/data/uploads}"

if [ "${APPV_DATABASE_DRIVER:-sqlite}" = "sqlite" ]; then
    mkdir -p "$(dirname "$DB_PATH")"
fi
if [ "${APPV_STORAGE_DRIVER:-local}" = "local" ]; then
    mkdir -p "$STORAGE_ROOT"
fi

if [ -z "${APPV_JWT_SECRET}" ] || [ -z "${APPV_DOWNLOAD_TOKEN_KEY}" ]; then
    echo "[ERROR] 缺少 APPV_JWT_SECRET / APPV_DOWNLOAD_TOKEN_KEY，生产模式将拒绝启动" >&2
    echo "        生成示例: openssl rand -hex 32" >&2
fi

echo "[INFO] 启动后端 (driver=${APPV_DATABASE_DRIVER:-sqlite})"
/app/appv -config /app/config.yaml &
BACKEND_PID=$!

# 后端异常退出时结束容器，交由编排系统重启
( while kill -0 "$BACKEND_PID" 2>/dev/null; do sleep 5; done
  echo "[ERROR] 后端进程已退出" >&2
  kill -TERM 1 2>/dev/null ) &

echo "[INFO] 启动 nginx"
exec nginx -g 'daemon off;'
