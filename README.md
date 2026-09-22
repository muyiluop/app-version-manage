# 软件版本发布系统 (App Version Manage) · v2

面向内部软件分发的版本发布与更新管理平台：管理应用、按平台/通道发布版本、生成受控下载链接、
提供检测更新接口，并支持文件托管、分享门户与自定义输出模板。

- 后端：Go 1.26 + Gin + GORM，支持 **SQLite（默认）/ PostgreSQL / MySQL**
- 前端：React 19 + Vite 6 + TypeScript + Ant Design 5
- 存储：**本地磁盘（默认）/ S3 兼容对象存储（MinIO、AWS S3）**
- 交付：单个 Docker 镜像（nginx + 后端二进制），或 `docker compose` 组合部署
- 安全：bcrypt 口令、密钥分离、HMAC 签名下载、登录/分享限流、审计日志、角色权限

> 本分支为 v2 重构版本。协议细节见 [docs/api-v2.md](docs/api-v2.md)，重构方案、实施状态与已知取舍见 [docs/refactor-plan.md](docs/refactor-plan.md)。
>
> 运行时验证情况：SQLite 与 **PostgreSQL 18** 已端到端实测通过（含迁移、发布、检测更新、签名下载、分享、审计、中文 UTF-8）；**MySQL 尚未做运行时验证**，建议在 CI 或预发环境补测。

## 功能概览

- **应用管理**：应用标识、图标、描述、支持平台、默认通道
- **通道**：每个应用自动具备 `stable` / `beta`，可自定义（如 `hotfix`），更新检查按通道进行
- **版本管理**：语义化版本排序、状态机（草稿/已发布/已下架）、强制更新、最低支持版本、扩展信息（JSON）、发布/下架/回滚
- **文件托管**：SHA-256 秒传与去重、分片目录、孤儿文件清理、上传类型/大小限制
- **受控下载**：HMAC 签名短时效链接；本地存储走服务端流式代理，S3 走预签名直连
- **分享门户**：可设访问密码与有效期，移动端自适应
- **输出模板**：用 Go template 自定义开放接口返回（JSON/XML/HTML/YAML 等）
- **检测更新**：客户端上报当前版本，服务端返回是否更新、是否强制、最低支持版本与校验值
- **多用户与审计**：admin / releaser / viewer 三种角色，关键操作全部留痕

## 目录结构

```
backend/
  cmd/server/            服务入口（含 -migrate-only / -backup）
  internal/
    config/              配置加载（YAML + APPV_ 环境变量）与校验
    database/            多数据库连接、版本化迁移、初始化、在线备份
    model/               领域实体
    repository/          数据访问
    service/             业务逻辑
    storage/             存储抽象：local / s3
    middleware/          请求 ID、恢复、访问日志、CORS、JWT、角色、限流
    api/v2/              v2 后台接口
    api/v1compat/        v1 兼容接口（开放/分享/登录）
    api/common/          下载与平台识别等公共逻辑
    router/              路由装配与健康检查
    pkg/                 semver、token 等基础库
  config.yaml            本地开发配置
frontend/
  src/api/               按资源划分的接口层（统一信封解包、401/403/429 处理）
  src/features/          apps / channels / versions / shares / templates / files / users / audit / share-portal
  src/types/api.ts       与后端契约一致的类型
  src/hooks/             useRequest（react-query 薄封装）、useSubmit
  src/components/        Layout、ErrorBoundary
  src/pages/             路由页面（AppDetail 仅作装配层）
deploy/                  Dockerfile / nginx.conf / entrypoint.sh / 容器配置
scripts/                 备份与恢复
docs/                    方案与接口文档
```

## 快速开始

### 后端（开发）

```bash
cp .env.example .env      # 环境相关配置（连接串、密钥、内网地址）都放这里，不入库
cd backend
go run ./cmd/server -config config.yaml
# 默认监听 http://localhost:9080
```

**配置分两层，避免密钥进仓库：**

| 文件 | 放什么 | 是否入库 |
| --- | --- | --- |
| `config.yaml` | 非敏感默认值（端口、驱动类型、日志级别、上传上限） | ✅ 入库 |
| `.env` | 连接串、密钥、内网地址等**环境相关**信息 | ❌ 已被 .gitignore 忽略 |

后端启动时会自动载入 `.env`（依次尝试 `APPV_ENV_FILE` → `./.env` → `../.env`，
即在 `backend/` 下直接 `go run` 也能读到仓库根目录的 `.env`），
也可显式指定 `-env-file /path/to/.env`。**优先级：真实环境变量 > `.env` > `config.yaml` > 代码默认值** ——
已存在的环境变量不会被 `.env` 覆盖，所以 CI/容器里注入的值始终优先。

首次启动会自动建表、迁移历史数据并创建管理员账号：

- 若设置了 `APPV_ADMIN_INITIAL_PASSWORD`，使用该口令；
- 否则随机生成并在启动日志中以 WARN 打印一次，登录后请立即修改。

开发模式下未配置的密钥会临时随机生成（每次重启失效）；**生产模式缺失密钥会直接拒绝启动**。

### 前端（开发）

```bash
cd frontend
npm install
npm run dev      # Vite 开发服务器，/api 代理到 localhost:9080
```

### Docker

```bash
cp .env.example .env      # 填写 APPV_JWT_SECRET / APPV_DOWNLOAD_TOKEN_KEY 等
docker compose up -d --build
# 访问 http://localhost:8081
```

镜像构建细节（`deploy/Dockerfile`）：

- 多阶段：npm 构建前端 → go 构建后端（`appv` 与迁移工具 `appv-migrate`）→ 运行镜像（nginx + 后端）
- **构建 `linux/amd64`**。Dockerfile 用 `TARGETARCH` 而不是写死架构，手动 `--platform` 仍可构建
  arm64；但在 amd64 机器上构建 arm64 只能靠 QEMU 模拟，前端 `tsc`/`vite` 与 `go build`
  会被拖慢数倍（实测单是 arm64 前端阶段就 220 秒），且前端产物与架构无关、多平台时
  还会被重复构建一遍 —— 所以 CI 里不做 arm64。真需要 arm64 镜像请用原生 arm64 runner
- 依赖与编译使用 BuildKit cache mount，重复构建快很多（需要 BuildKit，即 `DOCKER_BUILDKIT=1`）
- 运行镜像内只有**不含密钥**的配置模板（`deploy/config.example.yaml`），密钥一律走 `APPV_*` 环境变量；
  完整配置说明同时打进镜像：`/app/config.full-example.yaml`
- 容器内 PID 1 是 `deploy/entrypoint.sh`，负责把停止信号转发给后端做优雅退出

可选组件：

```bash
docker compose --profile postgres up -d --build   # 使用 PostgreSQL
docker compose --profile s3 up -d --build         # 使用 MinIO
```

## 配置

优先级：**代码默认值 < YAML 文件 < `APPV_` 环境变量**。

- **全量配置示例**：[backend/config.example.yaml](backend/config.example.yaml) —— 每个配置项都有说明，
  并给出 **PostgreSQL / MySQL / MinIO / AWS S3** 的完整示例（含 DSN 写法与前置条件）。
- 环境变量清单：[.env.example](.env.example)。
- 生产环境建议：非敏感项写 YAML，密钥一律走环境变量。

### 首次启动会自动建表吗？

**会。** 具体行为：

| 事项 | 是否自动 |
| --- | --- |
| SQLite 数据库文件 + 父目录 | ✅ 自动创建 |
| PostgreSQL / MySQL 的**数据库本身** | ❌ 需先手工 `CREATE DATABASE` |
| 库里的**表、索引、默认通道** | ✅ 自动创建 |
| 初始管理员账号 | ✅ 自动创建（口令见下） |
| S3 / MinIO 的 bucket | ✅ 不存在时自动创建 |
| 本地存储目录 | ✅ 自动创建 |

- 迁移是**只增不改**：只新增表、列、索引，不重建表、不删数据 —— 因此升级后可直接回滚旧镜像。
- 初始管理员：设置了 `APPV_ADMIN_INITIAL_PASSWORD` 就用它，否则随机生成并在启动日志中打印一次；
  首次登录会要求改密。
- 想跳过自动建表：`database.autoMigrate: false`（或 `APPV_DATABASE_AUTO_MIGRATE=false`），
  适合由 DBA 预建表或只读副本；此时请先单独跑一次 `./appv -config config.yaml -migrate-only`。

常用项：

| 变量 | 说明 | 默认 |
| --- | --- | --- |
| `APPV_SERVER_PORT` | 监听端口 | 9080 |
| `APPV_SERVER_MODE` | `debug` / `release` | debug |
| `APPV_DATABASE_DRIVER` | `sqlite` / `postgres` / `mysql` | sqlite |
| `APPV_DATABASE_PATH` | SQLite 文件路径 | ./data/app_version.db |
| `APPV_DATABASE_DSN` | PG/MySQL 连接串 | — |
| `APPV_STORAGE_DRIVER` | `local` / `s3` | local |
| `APPV_STORAGE_LOCAL_ROOT` | 本地存储根目录 | ./static/uploads |
| `APPV_STORAGE_S3_*` | S3/MinIO 连接参数 | — |
| `APPV_JWT_SECRET` | JWT 密钥（生产必填，≥16 位） | 开发模式随机 |
| `APPV_DOWNLOAD_TOKEN_KEY` | 下载签名密钥（生产必填，≥16 位） | 开发模式随机 |
| `APPV_SHARE_PASSWORD_PEPPER` | 分享密码 pepper | — |
| `APPV_ADMIN_INITIAL_PASSWORD` | 首次启动的管理员口令 | 随机 |
| `APPV_UPLOAD_MAX_SIZE_MB` | 单文件上限 | 4096 |

### 图标与文件下载：为什么默认经后端代理

应用图标的地址是前端拼出来的同源路径：`/api/static/logos/<对象键>`。后端拿到请求后：

- **本地存储**：直接流式代理（一向如此）；
- **S3 / MinIO**：默认也走代理。**只有在配置了 `storage.s3.publicEndpoint`
  （浏览器可达的地址）时**，才会 302 跳到预签名直连地址。

为什么默认不直连：`storage.s3.endpoint` 通常是**内网 IP + 明文 HTTP**。
把它作为 `<img src>` 交给 HTTPS 页面，会被浏览器按「混合内容」直接拦掉
（控制台表现为图片请求被重定向到内网地址后失败）；即便不拦，公网/外网浏览器也访问不到内网地址。
后端代理虽然多占一点带宽，但**在 HTTPS 与内网对象存储下是唯一始终正确的做法**。

想恢复直连（让对象存储承担大文件流量）需要两个前提，缺一不可：

1. 对象存储对浏览器可达：独立域名 + HTTPS 反向代理到 MinIO（如 `files.example.com`）；
2. 配置 `APPV_STORAGE_S3_PUBLIC_ENDPOINT=https://files.example.com`。

地址必须带 `http://` 或 `https://`，且**不能带路径前缀**：SigV4 签名覆盖 Host 头，
不能先按内网地址签名再改写域名，只能用对外地址签名，因此客户端会另建一个
"仅用于签名"的连接。

配置后会**同时影响图片与文件下载**（二者共用同一条解析逻辑 `OpenDownload`）：

- 预签名地址已带上 `response-content-disposition`，下载文件名不会变；
- 下载计数在跳转前就已记录，统计不受影响；
- 前端"下载版本"走的是 XHR（axios blob），跳转到**跨域**的 MinIO 需要 CORS。
  实测 MinIO 的预签名 GET 默认返回 `Access-Control-Allow-Origin` 与完整的
  `Access-Control-Expose-Headers`，因此默认可直接工作；若你的对象存储关闭了 CORS，
  需放行前端域名，否则这条下载路径会失败；
- 页面是 HTTPS 时，`publicEndpoint` **也必须是 HTTPS**，否则图片会再次被
  「混合内容」拦掉、XHR 下载同样会被拦；
- 预签名地址的有效期由 `storage.signedUrlTtlMinutes` 决定（默认 30 分钟）：
  下载需在有效期内**开始**，开始后继续传输不再校验。

## 数据迁移（换库 / 换存储）

`cmd/migrate` 用于把**旧 SQLite + 本地目录**搬到**新数据库 + 新存储**（例如迁到
PostgreSQL + MinIO）。启动时的原地 schema 升级与它是两条路径，互不影响。

```bash
cd backend
# 先看计划（不写入任何数据）
go run ./cmd/migrate -src-db ../data/app_version.db -src-files ../static/uploads -dry-run
# 正式执行（目标由 -config / APPV_* 决定）
go run ./cmd/migrate -src-db ../data/app_version.db -src-files ../static/uploads
```

特性：保留旧 ID 与分享令牌（旧分享链接继续可用）、文件按 sha256 重新分片并去重、
可重复执行（目标已存在则跳过）、缺失对象逐条告警、自动修正 PostgreSQL/MySQL 自增序列。

完整参数、校验步骤、回滚与已知限制见 **[docs/migration.md](docs/migration.md)**。

## 数据迁移与兼容

- 迁移**只增不改**：只新增表、列与索引，绝不重建或删除已有数据，可安全回滚到旧镜像。
- 历史数据自动升级：明文口令 → bcrypt；`is_active` → `status`；版本号 → 语义化数值列；
  每个应用补齐默认通道；无管理员的库会把最早的用户提升为 admin。
- 同一 `(app, platform, version)` 的重复记录会保留最新一条并在日志中列出被删除的 ID（**物理文件不删除**）。
- 迁移前建议先备份：`scripts/backup.ps1`（SQLite 使用 `VACUUM INTO` 在线快照）。

## 接口速览

- 后台：`/api/v2/**`（统一 `{code,message,data,requestId}` 信封）
- 开放：`/api/open/latest`、`/api/open/changelog`、`/api/open/check`、`/api/open/download/:token`
- 分享：`POST /api/share/:token`、`POST /api/share/:token/versions`
- 健康：`/healthz`、`/readyz`

v1 开放与分享接口的结构、字段名与「需要密码返回 209」的行为**保持不变**，可直接被现有客户端继续使用。
完整契约见 [docs/api-v2.md](docs/api-v2.md)。

## 开发与验证

```bash
# 后端
cd backend
go vet ./...
go test ./...            # 迁移、发布流程、令牌/存储/版本号均有测试

# 前端
cd frontend
npm run typecheck        # 等价 tsc -b；注意 npx tsc 在 solution 风格 tsconfig 下不检查任何文件
npm run lint
npm run build            # tsc -b && vite build
```

CI 在每次推送/PR 时执行以上检查，规则集中在可复用的
[`.github/workflows/verify.yml`](.github/workflows/verify.yml) —— 发布镜像时用的是同一套，
避免两处规则漂移。发布 tag/release 时：

| 工作流 | 作用 |
| --- | --- |
| [`.github/workflows/docker-image.yml`](.github/workflows/docker-image.yml) | 过门禁 → 构建 linux/amd64 → 推送 GHCR |
| [`.gitea/workflows/docker-image.yml`](.gitea/workflows/docker-image.yml) | 同上（流程完全同构），推送自建 Gitea 镜像仓库 |

两条流程结构完全一致：`质量门禁 → 构建并推送`，只有镜像地址与凭据不同。
构建与推送统一通过 `docker/setup-buildx-action` + `docker/build-push-action` 完成，
**不使用手写的 `docker build` / `docker push` 命令**。

另外只有**正式发布**才会移动 `latest` 标签，预发布不会把 `latest` 指向测试版本。
镜像可用性请在部署环境自行确认（`docker compose up -d` 后看 `/readyz`）：
容器内部已带 `HEALTHCHECK`，`docker ps` 会显示健康状态。

## 运维

```bash
# 备份（默认本地模式；容器内使用 -Mode docker）
.\scripts\backup.ps1

# 仅执行迁移（升级前排障）
/app/appv -config /app/config.yaml -migrate-only
```

恢复步骤见 [scripts/restore.md](scripts/restore.md)。

## 许可证

MIT，详见 [LICENSE](LICENSE)。
