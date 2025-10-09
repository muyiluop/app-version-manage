## 应用版本管理系统 (App Version Manage)

这是一个用于管理应用版本发布、文件存储与共享的开源后台服务与简单前端界面。

核心功能

- 管理应用与多个平台（android/ios/windows/macos/linux/harmony）的版本信息
- 生成受控下载链接（带过期 token）
- 支持模板化输出（JSON/XML/HTML/YAML 等）用于自定义对接
- 文件上传、列举、与清理未使用文件
- 生成分享链接并获取分享的应用及版本列表

项目结构（重要目录）

- `backend/` - Go 后端服务源码
  - `main.go` - 后端入口，路由与服务器启动逻辑位于此
  - `api/v1/` - REST 接口实现（包括开放接口、认证接口、文件/版本管理等）
  - `config/` - 配置加载与全局配置结构（`config.GlobalConfig`）
  - `repository/` - 数据库初始化与 ORM（GORM）相关代码
  - `utils/` - 工具函数（例如生成安全 token、文件处理等）
- `frontend/` - React + Vite 前端源码
  - `package.json` - 启动 / 构建脚本
  - `src/` - 页面与组件
- `deploy/` - 容器化部署相关文件
  - `Dockerfile` - 最终镜像构建（基于 nginx，拷贝 `www` 静态和后端可执行文件）
  - `entrypoint.sh` - 镜像启动脚本

快速开始（开发）

先决条件

- Go (>= 1.20 推荐)
- Node.js + npm
- Docker（用于容器化部署，可选）

启动后端（本地开发）

```powershell
# 在仓库根或进入 backend 目录
cd backend
# 使用 go run 直接运行（读取配置文件以获取端口）
go run main.go
```

说明：后端使用 `config.GlobalConfig.Server.Port` 指定监听端口。默认在 `main.go` 的注释示例中使用 8080（请查看 `config/config.go` 以确认或覆盖）。后端会将静态文件目录挂载到 `/static`（映射到配置中的 storage.path）。

启动前端（本地开发）

```powershell
cd frontend
npm install
npm run dev
```

构建前端

```powershell
cd frontend
npm install
npm run build
```

生产镜像（Docker）
项目包含 `deploy/Dockerfile`，该 Dockerfile 最终生成一个基于 nginx 的镜像：

- 将前端构建目录（`www`）复制到 nginx 静态目录
- 将后端可执行文件 `app` 复制到镜像内部 `/app` 并通过 `entrypoint.sh` 启动

示例：构建并运行镜像

```powershell
# 在仓库根（假设你已在本地完成前端构建并把产物放到根的 www/ 目录，且后端可执行文件生成放在 deploy 构建上下文的 app 文件）
docker build -f deploy/Dockerfile -t app-version-manage:latest .
docker run -p 80:80 --name app-version-manage app-version-manage:latest
```

镜像暴露端口：80（nginx）。后端 API 的基础路径为 `/api`（例如：`/api/open/latest`）。

配置说明

- `config/` 目录包含配置结构与加载逻辑（例如 `Server.Port`、`Storage.Path`、数据库配置等）。
- 请在生产部署时通过环境变量或配置文件设置数据库连接、存储路径与密钥等敏感信息。

存储与文件系统

- 后端在 `main.go` 中把静态目录 `config.GlobalConfig.Storage.Path` 挂载为 `/static`，并在容器镜像中创建 `/app/static/uploads` 目录用于保存上传文件。
- 文件上传、下载与清理接口实现位于 `api/v1` 的文件处理相关代码（`file.go`）。

主要 API 概览（来自 `backend/main.go` 与 `api/v1`）

- 开放接口（无需认证）

  - GET /api/open/latest?identifier=...&platform=... - 获取某应用在特定平台的最新版本信息（支持 format 模板输出）
  - GET /api/open/changelog?identifier=...&platform=... - 获取版本变更历史
  - GET /api/open/download/:token - 根据安全 token 下载文件（token 会在后端生成并含有效期）
  - GET /api/share/:token - 获取分享的应用信息
  - GET /api/share/:token/versions - 获取分享应用的版本列表

- 认证接口

  - POST /api/auth/login - 登录，返回鉴权信息（用于后续需要认证的接口）

- 需要鉴权的接口（通过中间件 `middleware.AuthMiddleware()`）
  - 应用管理：POST /api/apps, PUT /api/apps/:id, GET /api/apps/:id, GET /api/apps
  - 版本管理：POST /api/versions, PUT /api/versions/:id/deactivate, DELETE /api/versions/:id, GET /api/versions
  - 模板管理：POST /api/templates, PUT /api/templates/:id, DELETE /api/templates/:id, GET /api/templates
  - 文件管理：POST /api/files/upload, GET /api/files, POST /api/files/clean, GET /api/files/download/\*path

模板与自定义输出

- 后端支持用数据库中保存的模板（`model.Template`）渲染输出。当 `GET /api/open/latest?format=xxx` 指定模板名时，后端会查找对应模板并用 `text/template` 渲染，模板可产生 JSON、XML、HTML 或 YAML（由模板名后缀决定 Content-Type）。

安全与令牌

- 文件下载使用短期有效的安全 token（见 `utils.GenerateSecureToken`），默认示例中 token 有 24 小时有效期（在 `api/v1/open.go` 中可见示例）。

开发建议与调试

- 本地开发时可分别运行前后端：前端使用 Vite 的 `dev`，后端使用 `go run`。
- 若前端需要调用本地后端 API，请在前端请求中使用 `http://localhost:<后端端口>/api`，或通过 Vite 的代理配置将 API 转发到后端。

常见命令汇总（PowerShell）

```powershell
# 后端
cd backend
go run main.go

# 前端（开发）
cd frontend
npm install
npm run dev

# 前端（构建）
npm run build

# Docker（构建并运行）
docker build -f deploy/Dockerfile -t app-version-manage:latest .
docker run -p 80:80 app-version-manage:latest
```

示例：获取最新版本（开放接口）

```powershell
curl "http://localhost:8080/api/open/latest?identifier=com.example.app&platform=android"
```

贡献与联系

- 欢迎提出 issue 或 PR。请在贡献前先运行本地测试与 lint（后端可添加单元测试，前端使用 ESLint）。

许可证

本项目采用 MIT License 开源协议，详见仓库根目录 LICENSE 文件。
