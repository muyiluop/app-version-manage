#!/bin/sh

# 检查 /app 是否为空或不存在
if [ ! -d /app ] || [ -z "$(ls -A /app 2>/dev/null)" ]; then
    echo "[INFO] /app 目录为空或不存在，正在初始化应用..."
    mkdir -p /app
    tar xzf /etc/appv/init.tar.gz -C /app
    if [ $? -ne 0 ]; then
        echo "[ERROR] 应用初始化失败"
        exit 1
    fi
    echo "[INFO] 应用初始化完成"
fi

# 确保必要的目录存在
mkdir -p /app/static/uploads /app/logs

# 设置权限
chmod +x /app/app

# 启动后端
echo "[INFO] 启动应用服务..."
/app/app -config /app/config.yaml &

# 启动 nginx
echo "[INFO] 启动 nginx..."
exec nginx -g 'daemon off;'
