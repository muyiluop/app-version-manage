# 数据迁移：旧版 SQLite + 本地文件 → 新数据库 + 新存储

`cmd/migrate` 用于**跨数据库、跨存储**的搬迁，例如：

- 旧 SQLite → PostgreSQL / MySQL
- 本地磁盘 → MinIO / S3（或反过来）
- 同时更换数据库与存储

> 与它不同：服务启动时会自动做**原地 schema 升级**（同一个库文件内加表/加列/补数据），
> 那条路径不需要本工具。本工具面对的是「换一个库、换一个存储」的场景。

## 1. 迁移前

1. **备份**旧库与旧文件目录（不可省）
   ```powershell
   .\scripts\backup.ps1
   ```
2. 准备目标环境：数据库可连、bucket/目录可写；密钥这类必填项已配置
   （`APPV_JWT_SECRET`、`APPV_DOWNLOAD_TOKEN_KEY`，生产模式缺失会拒绝启动）。
3. 建议**先跑 dry-run**，确认统计与缺失对象清单符合预期。

## 2. 用法

```bash
cd backend

# 1) 先看计划（不写入任何数据/文件）
go run ./cmd/migrate \
  -src-db  ../data/app_version.db \
  -src-files ../static/uploads \
  -config  ./config.yaml \
  -dry-run

# 2) 正式执行
go run ./cmd/migrate \
  -src-db  ../data/app_version.db \
  -src-files ../static/uploads \
  -config  ./config.yaml
```

目标由 `-config` 指向的配置文件（以及优先级更高的 `APPV_*` 环境变量）决定：

```bash
# 例：迁到 PostgreSQL + MinIO
APPV_DATABASE_DRIVER=postgres \
APPV_DATABASE_DSN='host=pg user=appv password=*** dbname=appv port=5432 sslmode=disable' \
APPV_STORAGE_DRIVER=s3 \
APPV_STORAGE_S3_ENDPOINT=minio:9000 \
APPV_STORAGE_S3_BUCKET=appv \
APPV_STORAGE_S3_ACCESS_KEY=*** APPV_STORAGE_S3_SECRET_KEY=*** \
APPV_STORAGE_S3_FORCE_PATH_STYLE=true \
  go run ./cmd/migrate -src-db old.db -src-files old-uploads
```

### 参数

| 参数 | 说明 |
| --- | --- |
| `-src-db` | **必填**，旧 SQLite 文件路径 |
| `-src-files` | 旧上传目录；不填则只迁数据、不搬文件 |
| `-config` | 目标配置（数据库 + 存储），默认 `config.yaml` |
| `-dry-run` | 只输出计划与统计，不写入任何数据或文件 |
| `-overwrite` | 目标库非空时允许执行（会先清空目标库业务数据） |
| `-keep-old-keys` | 保留旧对象键：文件照样复制到新存储，但版本/图标引用不改写 |
| `-default-role` | 旧用户迁移后的角色，默认 `admin`（v1 无角色概念） |
| `-batch` | 批量写入大小，默认 200 |

### Docker / Compose 部署下怎么迁移

镜像里已经带了迁移工具：`/app/appv-migrate`。两种跑法，按「目标能否从宿主机直连」选：

**方式 A：在宿主机直接跑（目标可直连时最省事）**

适合目标数据库与对象存储能从宿主机直接访问（例如内网独立部署的 PostgreSQL + MinIO）。
`-config` 指向的文件只提供非敏感默认值，真正的「目标」由环境文件与 `APPV_*` 决定：

```powershell
cd backend

# 先排练：只输出统计与告警，不写入任何数据与文件
go run ./cmd/migrate `
  -src-db    D:\旧版数据\data\app_version.db `
  -src-files D:\旧版数据\static\uploads `
  -config    config.yaml `
  -dry-run

# 确认统计无误后正式执行（去掉 -dry-run）
go run ./cmd/migrate `
  -src-db    D:\旧版数据\data\app_version.db `
  -src-files D:\旧版数据\static\uploads `
  -config    config.yaml
```

> 目标来自仓库根目录的 `.env`（在 `backend/` 下运行会自动读取 `../.env`）：
> `APPV_DATABASE_DRIVER=postgres`、`APPV_DATABASE_DSN=...`、`APPV_STORAGE_DRIVER=s3`、
> `APPV_STORAGE_S3_*`。因此 `config.yaml` 里写着 sqlite 也没关系 —— 环境变量优先级更高。

**方式 B：在 compose 项目里跑一次性容器（目标只在容器网络内可达时）**

```bash
# 迁移期间先停掉新服务，避免并发写入
docker compose stop app

# --entrypoint 必须显式指定：
# 镜像默认入口是启动脚本（会拉起 nginx + 后端，且不转发参数）
docker compose run --rm --no-deps `
  -v "D:/旧版数据:/old:ro" `
  --entrypoint /app/appv-migrate `
  app `
  -src-db    /old/data/app_version.db `
  -src-files /old/static/uploads `
  -config    /app/config.yaml `
  -dry-run

# 去掉 -dry-run 即为正式执行；完成后 docker compose start app
```

一次性容器同样会读取 `env_file: .env` 与 `environment:`，所以**目标配置与主服务完全一致**，
不必重复填写连接串。若目标数据库/对象存储本身就是 compose 里的服务
（`--profile postgres` / `--profile s3`），先确保它们已启动；`--no-deps` 只是阻止顺带拉起 `app`。

Windows 上挂载路径必须是 Docker Desktop 已共享的盘符；旧数据目录建议只读挂载（`:ro`）。

**顺序与注意**

1. **先备份**旧库（连同 `-wal` / `-shm`）与旧上传目录；
2. 最好在**新服务首次启用之前**迁移 —— 目标库为空，无需 `-overwrite`；
3. 若已经开始在新系统里录数据：`-overwrite` 会**清空目标库业务数据**，且本工具不是合并工具，
   两条数据流无法自动合并；
4. 迁移完成后启动服务并逐项校验（见第 4 节）。

## 3. 迁移做了什么

**数据**

- 应用、通道（每个应用自动补齐 `stable`/`beta`）、版本、模板、文件记录、用户、分享
- **保留旧主键 ID**：外部系统若引用了应用/版本 ID 不会失效
- **保留分享令牌**：旧的 `/share/<token>` 链接迁移后仍可打开
- 版本：解析语义化版本号、按 `is_active` 回填状态、通道归到 `stable`、回填 `published_at`
- 用户：明文口令升级为 bcrypt，统一置为 `-default-role` 并标记「下次登录需改密」
- 分享密码：v1 用的是可逆加密，无法转成哈希；会保留原密文并标记 `legacy-aes`，
  旧链接仍可打开，但**需要在后台重新设置访问密码**
- 版本重复：v1 允许同一 (应用/平台/版本号) 多条记录，v2 有唯一约束，
  迁移时保留 id 最大的一条并在日志中列出被跳过的条数

**文件**

- 逐个文件计算 sha256，按 `<前2位>/<次2位>/<sha256>_<安全文件名>` 重新分片写入目标存储
- 相同内容只存一份（sha256 去重），版本/图标引用自动改写为新键
- **幂等**：目标已存在的对象跳过，可反复执行；`-keep-old-keys` 时文件仍复制但引用不改
- 源目录中缺失的引用对象会**保留原键**并逐条列为告警（这些下载会 404，需要补文件）

**其它**

- 目标为 PostgreSQL / MySQL 时会修正自增序列，保证迁移后新增记录不会主键冲突
- 迁移结束时若目标库没有任何用户，会自动创建默认管理员（口令见启动日志）

## 4. 迁移后校验

```bash
# 起服务（用同一份目标配置）
go run ./cmd/server -config <目标配置>

curl -s localhost:9080/readyz                                  # {"status":"ready"}
curl -s "localhost:9080/api/open/latest?identifier=<你的标识>&platform=windows"
curl -s "localhost:9080/api/open/check?identifier=<你的标识>&platform=windows&currentVersion=1.0.0"
```

后台逐项确认：应用列表与平台、版本列表与状态、文件列表、分享链接可打开、用户可登录。
文件建议抽查几个大包的实际下载。

## 5. 回滚

迁移**不修改源库与源目录**，因此回滚只需：

1. 把服务指回旧配置（旧 SQLite + 旧目录）即可；
2. 或清空目标库后重新迁移（`-overwrite`）。

## 6. 已知限制

| 限制 | 说明 |
| --- | --- |
| 旧分享密码 | 可逆加密无法自动转哈希，需在后台重设（时间窗内旧链接仍可打开） |
| 源文件缺失 | 日志会逐条列出；这些版本下载会 404，需从生产环境补齐文件后重跑（幂等，可重复执行） |
| 用户角色 | v1 无角色，默认全部迁为 `admin`，建议迁移后按需降级 |
| 大目录耗时 | 文件按内容哈希流式处理，不会全量读入内存，但 GB 级仍需耐心等待 |
| MySQL | 代码路径与三方言通用 SQL 均已就绪，但**未做运行时实测**（已验证 SQLite 与 PostgreSQL） |
