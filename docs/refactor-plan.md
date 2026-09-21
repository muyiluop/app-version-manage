# 版本发布系统 · 重构方案（v1 · 待确认）

> 状态：**待确认，尚未开始改动代码**
> 调研范围：`app-version-manage`（→ D:\projects\app-version-manage）
> 调研方式：全量源码走读 + 构建/部署配置核对 + 现有数据结构核对

---

## 一、现状评估

### 1.1 技术栈与规模

| 层 | 现状 |
| --- | --- |
| 后端 | Go 1.24.2 + Gin 1.10.1 + GORM 1.30 + glebarez/sqlite（纯 Go）+ JWT v5 + swag；17 个 `.go` 文件 / 约 1956 行 |
| 前端 | React 19 + Vite 6 + TS 5.8 + antd 5 + axios + react-router 7 + dayjs；15 个源文件 / 约 1966 行（`AppDetail.tsx` 单文件 939 行） |
| 部署 | 多阶段 Docker（node → golang → nginx），entrypoint 从 `/etc/appv/init.tar.gz` 自解压到 `/app`；Gitea + GitHub 双份 CI |
| 数据 | SQLite `backend/data/app_version.db`（约 100KB）；上传目录 `backend/static/uploads`（当前 3 个文件） |
| 测试 | **0**（后端 0 个 `_test.go`，前端 0 个测试、无 lint/test 门禁） |
| 文档 | README 154 行；Swagger 只覆盖 2 / 约 25 个接口，且 `main.go` **没有注册 swagger 路由** |

### 1.2 现有领域模型

    Application(id, name, identifier, logo, description, platforms[], timestamps)
    Version(appId, platform, version, filePath, fileName, fileSize, changelog, ext, forceUpdate, isActive)
    Template(appId, name, content)          # Go text/template 输出模板
    File(name, path, size, type, hash=md5)
    User(username, password=明文)
    Share(appId, token, password=可逆加密, expiresAt, isActive)

路由全部写在 `backend/main.go`；业务逻辑直接写在 handler（`backend/api/v1/*.go`）里，没有 service 层。

### 1.3 值得保留的设计（重构中不能丢）

1. **领域划分合理**：应用 / 版本 / 模板 / 文件 / 分享 / 用户，边界基本正确。
2. **模板化开放输出**（JSON/XML/HTML/YAML）对"对接各种自定义客户端"很有价值，是同类的亮点。
3. **分享门户的移动端适配**（deviceType 判断、Drawer bottom）已经考虑到真实使用场景。
4. **带 token 的受控下载**思路正确（只是实现要换）。
5. **交付方式简单**：一个镜像 + 自解压初始化，运维成本极低。

---

## 二、问题清单（按优先级，均可定位到文件）

### P0 · 安全（必须优先处理）

| # | 问题 | 证据 |
| --- | --- | --- |
| 1 | 管理员密码**明文存储 + 明文比对**，默认 `admin/123456` | `backend/repository/db.go:49-56`、`backend/api/v1/auth.go:36-39` |
| 2 | 密钥**硬编码**且全局复用：下载 token 与分享密码共用同一把 AES 密钥；JWT 默认密钥写死 | `backend/utils/secure_token.go:13`、`backend/config/config.go:52` |
| 3 | **明文/密钥/nonce 被打进日志** | `secure_token.go` 中 `fmt.Printf` 打印 nonce、明文、密文 |
| 4 | 分享密码用**可逆加密**而非哈希，库泄露即可还原 | `utils.HashPassword/VerifyPassword` |
| 5 | 整个存储目录**无鉴权公开**：`/static` 直出所有版本文件和 Logo（文件名 = md5 前缀）；`GET /files/download/*path` 未做路径穿越校验 | `backend/main.go` `r.Static`、`backend/api/v1/file.go:DownloadFile` |
| 6 | 登录、分享密码校验**无频率限制**，可离线/在线爆破 | `auth.go`、`share.go` |
| 7 | 下载响应头 `Content-Disposition: attachment; filename=%s` 未转义 | `backend/api/v1/file.go:SecureDownload` |
| 8 | JWT 无刷新/吊销；中间件不校验用户是否仍存在 | `backend/middleware/auth.go` |

### P1 · 正确性与数据一致性

9. **"最新版本"有两套口径**：开放接口用 `ORDER BY created_at DESC`（`open.go`），分享页用 `MAX(id)`（`share.go`）。补发/回填版本时二者都会选错。
10. 版本号（如 `1.1.26245.26815`）**没有语义化解析**，只靠创建时间排序。
11. `CreateVersion` **完全信任前端**传来的 `filePath/fileName/fileSize`；`(app_id, platform, version)` **无唯一约束** → 可重复发布、可挂任意文件。
12. 版本只能"发布 / 下架 / 删除"：**不能编辑、不能重新上架、不能回滚**；删除版本**不删文件也不删 File 记录** → 孤儿数据。
13. `VersionDetail.tsx` 调用 `GET /versions/:id`，但后端**没有注册该路由** → 必然 404。
14. 分页不一致：`ListVersions/ListFiles` **强制要求** page/pageSize（无默认值）；`ListApps` 无分页无搜索（前端表格却显示分页器）。
15. `CleanUnusedFiles`：`NOT IN (...)` 空集合行为不可靠；先删 DB 再删文件且吞异常；只比对 `file_path`/`logo`。
16. **缺少索引**：`versions(app_id, platform, is_active)`、`files(hash)`、`shares(token, is_active)` 等热点查询无复合索引。
17. 只有 GORM `AutoMigrate`，**无版本化迁移**，后续改字段/加非空列有数据风险。
18. SQLite **未开 WAL / busy_timeout**，并发写入易 `database is locked`。
19. 配置来源不一致：`backend/config.yaml` vs `deploy/config.yaml` vs 代码默认值三处漂移。

### P2 · 工程质量与可维护性

20. 路由堆在 `main.go`；无 service 层，逻辑与 HTTP 耦合，**无法单测**。
21. 无结构化日志、无 requestID、**无审计**（谁发布了哪个版本、谁禁用了哪个分享，查不到）。
22. 无统一错误码，前端靠中文文案 + HTTP 状态码判断；`share.go` 使用**非标准状态码 209** 表示"需要密码"。
23. 死代码/垃圾文件：`config/deploy.go`、`share.go:validateShareToken`、`backend/app.exe`(25MB)、`deploy/app`(24MB)、`frontend/build.ps1`(0 字节)、根目录 `Dockerfile copy`（`COPY ../frontend` 是无效路径）。
24. 前端：**无 API 层**（axios 调用散落在 6 个页面）、类型重复定义（`AppList.tsx` 内联 `Application` vs `types/index.ts`）、平台选项在 3 个文件重复、`AppDetail.tsx` 939 行 / 12 个 modal 状态、无请求缓存、无错误边界、无路由守卫。
25. 无用依赖：`@ant-design/pro-components`、`@emotion/react`。
26. 配置只支持 yaml + 命令行，**无环境变量**（容器/K8s 不友好）；无 graceful shutdown、无 `/healthz`、无上传体积限制（nginx 全局 4096M）、无类型白名单。
27. 前端构建用 `npm install`（应 `npm ci`）；CI 无 lint / test / 类型检查门禁；`.github` 与 `.gitea` 两份工作流重复维护。
28. **缺少"更新检查"能力**——而这正是"实现版本更新"的核心：客户端目前只能拉 latest 自己比大小，没有 `currentVersion → {hasUpdate, forceUpdate, downloadUrl, sha256}` 的语义化接口，也无法强制最低版本。

---

## 三、重构目标与原则

**一句话目标**：在不改变现有使用方式的前提下，把项目从"能跑的脚本式 demo"升级为**安全、可测、可演进、对客户端友好**的版本发布平台。

**五条原则**

1. **增量重构，不重写**：后端继续 Go + Gin + GORM，前端继续 React + Vite + antd。
2. **契约优先**：OpenAPI 为唯一事实来源，前端类型从契约生成。
3. **向后兼容**：`/api/open/*`、`/api/share/*` 的 v1 响应结构（含 209 语义）保持不变，新能力走 `/api/v2`。
4. **不破坏数据**：所有 DB 变更必须带默认值与回滚脚本，迁移前自动备份。
5. **每个阶段独立可交付、可回滚**。

### 3.1 目标目录结构

    backend/
      cmd/server/main.go
      internal/
        config/        # yaml + 环境变量覆盖，密钥只从 env 读
        server/        # gin engine、路由注册、优雅退出、健康检查
        middleware/    # auth / rbac / requestid / recovery / ratelimit / accesslog
        api/v1|v2/     # 只做参数绑定、校验、响应
        service/       # 业务用例（应用/版本/文件/分享/模板/更新检查）
        repository/    # GORM 数据访问
        model/ dto/
        storage/       # 存储抽象：local（未来可换 S3/MinIO）
        pkg/           # semver / token / hash / apierr
      migrations/      # 版本化 SQL
    frontend/src/
      api/             # 按资源分模块 + react-query hooks
      features/{apps,versions,shares,templates,files,share-portal}/
      components/      # PlatformSelect / LogoUpload / ArtifactUpload / DataTable
      hooks/  utils/  router/  types/(由 OpenAPI 生成)

### 3.2 关键选型建议

- DB：**继续 SQLite（开 WAL）**，但通过 `DB_DRIVER/DB_DSN` 预留 MySQL/PostgreSQL 切换能力——本次不迁移。
- 迁移：`golang-migrate` 或版本化 SQL 目录（不依赖 AutoMigrate）。
- 日志：标准库 `log/slog` + requestID；密码：`x/crypto/bcrypt`；限流：`x/time/rate`。
- 前端新增：`@tanstack/react-query`、`openapi-typescript`、`vitest` + `@testing-library/react`、`playwright`（冒烟）。

---

## 四、分阶段实施计划

### Phase 0 · 基线与清理（0.5 天）
- 备份现有 DB 与 uploads；打 tag。
- 删除垃圾文件与死代码（`app.exe`、`deploy/app`、`Dockerfile copy`、0 字节 build.ps1、`config/deploy.go`、`validateShareToken`）。
- 新增 `.env.example`；密钥改为从环境变量读取（暂时保留默认值 + 启动告警）。
- 确认本地一键可跑：后端 `go run` + 前端 `npm run dev`。
- **验收**：`go build` 通过；`npm ci && npm run build` 通过；现有 7 个应用数据可正常读取。

### Phase 1 · 后端分层骨架（2–3 天）
- `cmd/server` + `internal/*` 落地；路由集中注册；handler 瘦身为"绑定 → 调 service → 响应"。
- 统一响应与错误码（`apierr`）；**v1 保持旧结构**，v2 用新结构。
- `slog` 结构化日志 + requestID + panic recovery。
- 配置全量支持环境变量；启动时强校验（密钥缺失直接拒绝启动）。
- 版本化迁移 + 补齐索引。
- 单测：config / service / repository（内存 SQLite）。
- **验收**：`go vet` + `golangci-lint` + `go test ./...` 全绿；API 行为与现状逐条比对通过。

### Phase 2 · 安全加固（2–3 天，可与 Phase 1 并行）
- 密码改 bcrypt；初始管理员密码由 env 注入；首次登录强制改密。
- 密钥分离：`JWT_SECRET` / `DOWNLOAD_TOKEN_KEY` / `SHARE_PASSWORD_PEPPER` 各自独立，全部来自 env；**移除所有调试 `fmt.Printf`**。
- 分享密码改 bcrypt（不再用可逆加密）。
- 下载：统一走鉴权或 HMAC 签名 URL；`Content-Disposition` 按 RFC 5987 编码；路径规范化，禁止 `..` 与绝对路径；`/static` 不再裸奔。
- 限流：登录、分享密码校验、开放接口。
- JWT：access + refresh；支持吊销（`token_version`）；中间件校验用户存在。
- 审计表：`audit_logs(actor, action, target, payload, ip, created_at)`。
- **验收**：伪造 token、路径遍历、密码爆破三类定向测试全部失败。

### Phase 3 · 版本语义与领域模型（3–4 天）
- `versions` 增加：`channel`(stable/beta)、`sha256`、`published_at`、`status`(draft/published/archived)、`download_count`、`min_supported_version`。
- 唯一约束 `(app_id, platform, channel, version)`。
- semver 解析与比较；**统一"最新版本"的单一实现**（open/share 共用）。
- 版本编辑 / 重新上架 / 回滚；删除改软删 + 引用计数，异步 GC 清理孤儿文件。
- 文件：sha256 去重、分片目录（`xx/yy/hash`）、类型/体积白名单、配额统计。
- **新增更新检查接口**：
  `GET /api/v2/check?identifier=&platform=&channel=&currentVersion=`（Gin 同一路径段不能同时用 `:id` 与 `:identifier`，故独立成 `/api/v2/check`）
  → `{ hasUpdate, latest, forceUpdate, minSupportedVersion, releaseNotes, artifacts:[{url, sha256, size}] }`
- **验收**：最新版本判定、semver 比较、回滚、GC 均有单测；用现有数据跑通。

### Phase 4 · 对外 API 与文档（1–2 天）
- v2 契约定稿；`swag` 注解全量覆盖 → OpenAPI 3 自动生成（`/swagger` 仅 dev/内网）。
- v1 兼容层：`/api/open/latest|changelog|download`、`/api/share/*` 原样保留（含 209），内部转发到新 service。
- 接入文档 + 示例代码（"检查更新 → 下载 → 校验 sha256"全流程）。
- **验收**：v1 快照回归测试通过；OpenAPI 可导入 Postman/Apifox。

### Phase 5 · 前端重构（4–6 天）
- 建立 `api/` 层 + react-query；类型由 OpenAPI 生成，删除重复定义。
- 拆解 `AppDetail.tsx`：`features/versions`、`features/shares`、`features/templates`、`features/app-info`。
- 抽公共组件：`PlatformSelect`、`LogoUpload`、`ArtifactUpload`（进度 + 校验 + 自动回填）、`DataTable`、`ConfirmAction`。
- 路由守卫（含登录回跳）、错误边界、统一 `useDownload`。
- 分享门户打磨：密码错误提示、平台识别、复制下载链接、sha256 展示、移动端。
- 全局：面包屑、导航高亮修复（`selectedKeys` 由 pathname 派生）、时间格式统一、应用列表服务端分页 + 搜索。
- **验收**：`tsc --noEmit` + eslint + vitest 通过；Playwright 主链路冒烟（登录→建应用→传版本→发布→分享→下载）。

### Phase 6 · 交付与运维（2–3 天）
- 单一 Dockerfile（多阶段、`npm ci`、利用构建缓存）；清理双 CI。
- 环境变量化配置 + `docker-compose.yml`（挂载 data / uploads 卷）。
- `/healthz` `/readyz`；graceful shutdown；nginx 增加 gzip、静态缓存、安全头、上传限制。
- 备份/恢复脚本（SQLite 热备 + uploads 打包）。
- CI：lint + test + build + 镜像推送。
- **验收**：`docker compose up` 一把起；重启后数据不丢；健康检查通过。

### Phase 7 · 收尾（1 天）
- README / 架构文档 / 运维手册 / API 文档归位；清理死文件；发布 v2.0.0。

---

## 五、数据迁移方案（绝不破坏现有数据）

1. 迁移前自动备份 `app_version.db` + `uploads/`，并提供 dry-run 报表（打印将变更行数）。
2. **用户密码**：按 `$2a$` 前缀判断，未哈希的用一次性脚本转 bcrypt（原密码继续可用）。
3. **versions 新增列全部给默认值**：`channel='stable'`、`published_at=created_at`、`status` 由 `is_active` 推导、`sha256` 由物理文件重算回填、`min_supported_version` 置空。
4. **files**：`path` 保持原值（`hash_filename`），新增 `sha256` 列回填；**不搬迁物理文件**，仅新上传走新目录规范，老文件继续可读。
5. **分享密码**：可逆加密无法逆推为 bcrypt → 增加 `password_algo` 字段，旧格式仍可校验；用户下次编辑即自动升级，并提供"重置分享密码"操作。
6. **唯一约束前置清洗**：`(app_id, platform, version)` 重复项保留最新一条，其余置 `status='archived'`。
7. 所有迁移脚本提供 up/down。

## 六、兼容性与回滚

- **v1 公开接口保持不变**（字段名、`209` 语义均保留），由兼容层实现；`/api/open/latest` 的 `filePath` 继续返回下载 token。
- Phase 5 期间前后端并行开发，后端只做增量，旧前端始终可运行。
- 每个 Phase 一个 tag；DB 迁移带 down；镜像保留上一版本可秒回滚。
- v2 先内部使用，稳定后再推动接入方迁移；v1 至少保留 2 个版本周期。

## 七、测试与质量门禁

- 后端：service / repository 单测（内存 SQLite）+ httptest 接口测试；service 层覆盖率目标 ≥ 70%。
- 前端：vitest + testing-library 组件与工具测试；Playwright 冒烟 1 条主链路。
- 门禁：`golangci-lint` + `go test -race` + `tsc --noEmit` + `eslint` + `npm run build`，CI 全绿才可合并。
- 契约测试：v1 响应结构快照，防止无意破坏兼容。

## 八、里程碑与工作量

| 阶段 | 内容 | 估算 | 可交付 |
| --- | --- | --- | --- |
| P0 | 基线清理 | 0.5d | 干净可复现 |
| P1 | 后端分层 | 2–3d | 新骨架 + 测试 |
| P2 | 安全加固 | 2–3d | 安全达标 |
| P3 | 版本语义 | 3–4d | v2 check 接口 |
| P4 | API/兼容 | 1–2d | OpenAPI + v1 兼容 |
| P5 | 前端重构 | 4–6d | 新控制台 |
| P6 | 运维交付 | 2–3d | compose + CI |
| P7 | 收尾 | 1d | v2.0.0 |
| | **合计** | **约 16–23 人日** | |

推荐顺序 P0 → P1 → P2 → P3 → P4 → P5 → P6 → P7。

**最小可用优先方案（MVP-1，约 3 天）**：P0 + P2 安全项 + Phase 1 的索引/迁移，并修掉 3 个确定 bug（`/versions/:id` 404、最新版本口径、删除版本的孤儿文件）。

## 九、已确认决策（评审结论）

| # | 决策项 | 结论 |
| --- | --- | --- |
| 1 | 技术栈 | **保持 Go+Gin+GORM + React+Vite+antd，增量重构** |
| 2 | 数据库 | **多数据库支持**：默认 SQLite，可切 PostgreSQL / MySQL，由配置选择 |
| 3 | 版本维度 | **引入 channel**（stable/beta/hotfix…） |
| 4 | 用户权限 | **多用户 + 角色**（管理员/发布员/只读）+ 登录审计 |
| 5 | v1 契约 | **检测更新相关接口必须兼容 v1**，新能力走 v2 |
| 6 | 对外接口 | 按推荐：开放接口不加 AppKey（保留后续可插拔）；强制更新用既有 `forceUpdate` + 新增 `minSupportedVersion`；本期不做 webhook |
| 7 | 存储 | **多存储支持**：本地磁盘（默认）+ MinIO/S3 |
| 8 | 交付形态 | 按推荐：单镜像模式保留，同时提供 `docker compose` 分离部署 |
| 9 | 范围 | 全量 P0–P7，按 Phase 逐段确认 |
| 10 | 历史数据 | **保留并迁移**现有文件与旧分享链接 |

> 附：**所有改动均在分支 `refactor/v2` 上进行**。

## 十、落地方式承诺

- 严格按 Phase 推进，每个 Phase 一次确认 / 一个 PR，不做大爆炸式合并。
- 先更新文档与测试，再改实现。
- 所有破坏性变更（DB、API）必须先给出兼容与回滚方案才执行。

---

## 十一、基于确认结论的方案调整（已锁定）

### 11.1 多数据库设计

- 配置项：`database.driver` = `sqlite`（默认）| `postgres` | `mysql`；`database.dsn` 直接给连接串，或沿用 `database.path`（仅 sqlite）。
- 驱动：sqlite 继续 `glebarez/sqlite`（纯 Go、免 CGO）；postgres 用 `gorm.io/driver/postgres`；mysql 用 `gorm.io/driver/mysql`。
- **迁移必须三方言通用**：版本化 SQL 按 dialect 组织，避免 SQLite 专有语法；布尔用 `bool`、JSON 列统一用 TEXT + serializer、自增统一走 GORM 约定。
- 唯一约束 `(app_id, platform, channel, version)` 三种数据库都支持。
- CI 用 SQLite 跑单测；PG / MySQL 用容器跑集成测试（本地无 Docker 时自动跳过）。

### 11.2 channel 模型

- 新增 `channels` 表：`(app_id, key, name, is_default, sort)`，每个应用自动初始化 `stable`。
- `versions.channel` 存 channel key；唯一维度为 `(app_id, platform, channel, version)`。
- 更新检查按 channel 取最新；开放接口支持可选 `channel` 参数，**不传时用应用默认 channel，行为与现状一致（保证 v1 兼容）**。

### 11.3 多用户与角色

- `users`：`username`、`password`(bcrypt)、`role`、`display_name`、`is_active`、`last_login_at`、`token_version`。
- 角色：`admin`（全部 + 用户管理）、`releaser`（应用/版本/文件/分享/模板的增删改）、`viewer`（只读）。
- 中间件 `RequireRole(...)`；`audit_logs` 记录 actor / action / target / ip / ua / created_at。
- 用户管理接口 `/api/v2/users`（仅 admin）。

### 11.4 存储抽象

- 接口：`Put` / `Open` / `Delete` / `Stat` / `URL(ctx, key, ttl)`。
- 驱动：`local`（默认，兼容现有 `static/uploads`）+ `s3`（兼容 MinIO / AWS S3 及 S3 协议对象存储）。
- 统一下载出口：local 走后端流式代理（带鉴权/签名）；s3 走预签名 URL。
- 迁移策略：老文件 key 保持 `hash_filename` 不变，新文件走分片 key `xx/yy/<sha256>_<原名>`；两种 key 都必须可解析。

### 11.5 v1 兼容边界

- `/api/open/latest`、`/api/open/changelog`、`/api/open/download/:token`、`/api/share/*` 的**响应结构、字段名、`209` 语义全部保持不变**。
- 新增可选 `channel` 查询参数；不传时与现行为完全一致。
- v2 新增语义化检测更新：`GET /api/v2/check?identifier=&platform=&channel=&currentVersion=`（Gin 同一路径段不能同时用 `:id` 与 `:identifier`，故独立成 `/api/v2/check`）。

---

## 十二、实施状态（分支 refactor/v2）

| Phase | 状态 | 说明 |
| --- | --- | --- |
| P0 基线 | ✅ 完成 | 分支建立、垃圾文件与死代码清理、`.env.example` |
| P1 后端分层 | ✅ 完成 | `cmd/server` + `internal/*`；统一错误码；slog + requestId；配置 env 覆盖；版本化迁移 |
| P2 安全加固 | ✅ 完成 | bcrypt、密钥分离、HMAC 下载令牌、路径穿越防护、限流、令牌吊销、审计、三角色 |
| P3 领域模型 | ✅ 完成 | channel、semver、状态机、sha256 秒传、local/S3 存储抽象、检测更新接口 |
| P4 契约 | 🟡 部分 | `docs/api-v2.md` 完整契约；**OpenAPI 自动生成未做**（swag 注解量大，列为后续） |
| P5 前端 | 🚧 进行中 | API 层与类型已完成，feature 拆分与页面进行中 |
| P6 运维 | ✅ 完成 | Dockerfile/compose/nginx/健康检查/在线备份/CI |
| P7 收尾 | ⏳ 待前端完成 | 待验收前端后统一提交并发布 v2.0.0 |

### 已知偏离与取舍

1. **下载令牌不兼容旧令牌**：旧实现用硬编码 AES 密钥签发令牌；新实现改为独立 HMAC 密钥。升级前已签发的令牌在 24h 内失效，重新调用 `/api/open/latest` 即得新令牌。
2. **历史分享密码**：旧数据使用可逆 AES 加密，出于安全考虑不再解密，需管理员在后台重新设置访问密码（迁移已标记 `password_algo=legacy-aes`）。
3. **孤儿文件清理口径**：为避免误删仍可恢复的数据，被**软删除版本**引用的文件不会被清理。
4. **PostgreSQL / MySQL 未做运行时验证**：本机无 Docker，仅保证代码与迁移 SQL 使用三方言通用写法；建议在 CI 或预发环境跑一次集成验证。
