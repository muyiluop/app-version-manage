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
cd backend
go run ./cmd/server -config config.yaml
# 默认监听 http://localhost:9080
```

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

可选组件：

```bash
docker compose --profile postgres up -d --build   # 使用 PostgreSQL
docker compose --profile s3 up -d --build         # 使用 MinIO
```

## 配置

优先级：**代码默认值 < YAML 文件 < `APPV_` 环境变量**。完整清单见 [.env.example](.env.example)。

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

CI（`.github/workflows/ci.yml`）在每次推送时执行以上检查。

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
