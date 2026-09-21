# 恢复指引

详见 deploy 文档；核心步骤：

1. 停止服务
2. 用最近一次备份的 `app_version-*.db` 覆盖 `/data/app_version.db`（同时删除 `-wal` / `-shm`）
3. 解压 `uploads.zip` 到 `/data/uploads/`
4. 启动服务并用 `GET /readyz` 校验

PostgreSQL 使用 `pg_dump` / `pg_restore`；对象存储使用 `mc mirror` 或 `aws s3 sync`。
数据库迁移只增不改，旧版本镜像可直接回滚。
