#!/bin/sh
# 容器启动脚本（PID 1）。
#
# 为什么由脚本当 PID 1 而不是直接 exec nginx：
#   停容器时需要把信号转给后端进程，让它走自带的优雅退出流程（sweep 15s），
#   否则后端会被直接 SIGKILL——进行中的请求被硬断、SQLite 的 WAL 来不及回收。
set -e

DB_PATH="${APPV_DATABASE_PATH:-/data/app_version.db}"
STORAGE_ROOT="${APPV_STORAGE_LOCAL_ROOT:-/data/uploads}"

if [ "${APPV_DATABASE_DRIVER:-sqlite}" = "sqlite" ]; then
    mkdir -p "$(dirname "$DB_PATH")"
fi
if [ "${APPV_STORAGE_DRIVER:-local}" = "local" ]; then
    mkdir -p "$STORAGE_ROOT"
fi

# 生产模式缺少密钥会拒绝启动，这里提前给出可执行的修复提示
if [ -z "${APPV_JWT_SECRET}" ] || [ -z "${APPV_DOWNLOAD_TOKEN_KEY}" ]; then
    echo "[ERROR] 缺少 APPV_JWT_SECRET / APPV_DOWNLOAD_TOKEN_KEY，生产模式将拒绝启动" >&2
    echo "        生成示例: openssl rand -hex 32" >&2
fi

NGINX_PID=""
BACKEND_PID=""
stopping=0

shutdown() {
    if [ "$stopping" = "1" ]; then
        return
    fi
    stopping=1
    echo "[INFO] 收到停止信号，正在优雅退出…"

    # 先让 nginx 停止接收新连接，再通知后端退出
    if [ -n "$NGINX_PID" ]; then
        kill -QUIT "$NGINX_PID" 2>/dev/null || true
    fi
    if [ -n "$BACKEND_PID" ]; then
        kill -TERM "$BACKEND_PID" 2>/dev/null || true
        # 给后端留出优雅退出时间（其内部上限 15s），超时再强杀
        i=0
        while [ "$i" -lt 20 ]; do
            kill -0 "$BACKEND_PID" 2>/dev/null || break
            i=$((i + 1))
            sleep 1
        done
        kill -KILL "$BACKEND_PID" 2>/dev/null || true
    fi

    echo "[INFO] 已退出"
    exit 0
}

trap shutdown TERM INT QUIT

echo "[INFO] 启动后端 (db=${APPV_DATABASE_DRIVER:-sqlite}, storage=${APPV_STORAGE_DRIVER:-local})"
/app/appv -config /app/config.yaml &
BACKEND_PID=$!

echo "[INFO] 启动 nginx"
nginx -g 'daemon off;' &
NGINX_PID=$!

# 任一进程退出都结束容器，交由编排系统按 restart 策略重启
while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$NGINX_PID" 2>/dev/null; do
    sleep 2
done

if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "[ERROR] 后端进程已退出" >&2
else
    echo "[ERROR] nginx 进程已退出" >&2
fi
shutdown
