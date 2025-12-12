# AI Agent Instructions for App Version Manage

## Project Overview

App Version Manage is a hybrid Go + React application for managing application versions, file storage, and controlled sharing with time-limited download tokens.

**Architecture**:

- **Backend**: Go (Gin framework) REST API with SQLite database and JWT authentication
- **Frontend**: React + Vite TypeScript SPA
- **Deployment**: Docker (nginx + Go binary in `deploy/Dockerfile`)

## Critical Architecture Patterns

### 1. Configuration & Startup

- Global config loaded in `backend/config/config.go` → `config.GlobalConfig` (used throughout)
- Database auto-migration happens in `repository.InitDB()` before server starts
- All routes defined in `backend/main.go` - no separate routing files
- Supported platforms are enum constants in `backend/model/model.go`: `android|ios|windows|macos|linux|harmony`

### 2. API Response Patterns

- **Public endpoints**: No auth required (`/api/open/*`, `/api/share/*`)
- **Protected endpoints**: Require JWT token in `Authorization: Bearer <token>` header
- **Error responses**: Always use `gin.H{"error": "message"}` structure
- **Middleware**: `AuthMiddleware()` in `middleware/auth.go` extracts & validates JWT; stores userID in `c.Set("userID", claims.UserID)`

### 3. Database & Models

- **ORM**: GORM with SQLite
- **Models** defined in `backend/model/model.go`:
  - `Application` (name, identifier, logo, description, platforms array, timestamps)
  - `Version` (appID, platform, version string, filePath, fileSize, changelog, forceUpdate flag, isActive)
  - `Template` (output templates for JSON/XML/HTML/YAML)
  - `File` (tracking uploaded files)
  - `User` (authentication, default admin created if none exist)
  - `Share` (应用分享管理：存储 token、加密密码、可选有效期、启用状态)
- Foreign key: Version.AppID → Application.ID; Share.AppID → Application.ID

### 4. Key Security Patterns

- **Token Generation**: `middleware.GenerateToken(userID)` creates JWT with `JWTClaims` containing UserID and expiry
- **Secure Downloads**: `utils.GenerateSecureToken()` creates encrypted AES-GCM tokens with embedded expiry timestamp (4-byte unix + data), base64-encoded
- **Token Validation**: `ValidateSecureToken()` decrypts and checks expiry; returns underlying data or error
- **Share Passwords**: `utils.HashPassword()` encrypts share passwords with AES-GCM; `VerifyPassword()` validates them
- **Share Token Validation**: `validateShareToken(token, password)` in `share.go` verifies share token validity, expiry, and password (if set)

### 5. File Handling

- Storage path: `config.GlobalConfig.Storage.Path` (default: `static/uploads`)
- Uploaded files mapped to `/static` route in Gin
- Backend auto-creates upload directory in `InitDB()`
- File APIs in `backend/api/v1/file.go`: upload, list, clean unused

### 6. API Composition Pattern

- All v1 API handlers in `backend/api/v1/*.go` (separate files per resource: app.go, version.go, file.go, etc.)
- Handler pattern: Parse request → Validate → DB operation → Return JSON response
- Request structs use `binding:"required"` struct tags for validation

### 7. Frontend Request Handling

- Base URL configured via `VITE_API_BASE_URL` env var
- Axios interceptors in `frontend/src/utils/request.ts` auto-attach JWT from localStorage
- 401 → redirect to login, other errors → toast message via `showMessage` util
- Token stored as plain string (no prefix) in localStorage

## Build & Deployment

### Local Development

```powershell
# Backend (from root or backend/ dir)
cd backend
go run main.go  # listens on config port (default 8080)

# Frontend (from root or frontend/ dir)
cd frontend
npm install
npm run dev  # Vite dev server
```

### Production Build

```powershell
# Run build.ps1 (from root) - handles:
# 1. Frontend build (npm run build) → copies dist to deploy/www
# 2. Backend cross-compile (GOOS=linux GOARCH=amd64) → deploy/app
# 3. Docker build (deploy/Dockerfile creates nginx + Go image)
```

**Container setup**:

- Nginx serves frontend from `/`
- Backend API at `/api` (reverse proxy to port 8080 internally)
- Exposes port 80

## Common Development Tasks

### Adding a New API Endpoint

1. Define request/response structs in new file in `backend/api/v1/`
2. Implement handler function (follows `func Name(c *gin.Context)` pattern)
3. Add route in `backend/main.go` (protected routes go in `auth := apiGroup.Group("/", middleware.AuthMiddleware())`)
4. If new data model needed, define in `backend/model/model.go` and add to `InitDB()` migration

### Querying Database

- Use `repository.DB` (global GORM instance) in handlers
- Example: `repository.DB.Where("identifier = ?", id).First(&app)`
- Always check `.Error` after DB operations

### Token-based Resource Access

- For download tokens: use `GenerateSecureToken(resourceID, duration)`
- For share links with password protection:
  - Create share: `GenerateShareLink` saves `Share` record with token and encrypted password (optional)
  - Access share: `GetSharedApp`/`GetSharedAppVersions` (POST endpoints) use `validateShareToken(token, password)` to verify both token validity, expiry, and password match
  - Password encryption: `HashPassword()` uses AES-GCM, `VerifyPassword()` decrypts and compares
  - Token stored in DB allows share management: list, deactivate, delete shares per app

## Project-Specific Conventions

- Chinese variable/comment names throughout (部分代码采用中文命名)
- No error wrapping layers - errors handled directly in handlers with generic "操作失败" messages
- Platform filtering: when querying versions, filter by `platform` field explicitly (not inferred)
- Timestamps: Go's `time.Time` → JSON serializes to RFC3339 format
- File naming: uploaded files stored by original name in config.Storage.Path
- Share password validation: Uses `validateShareToken()` helper that checks token existence, expiry, and password in single validation

## External Dependencies

- **Gin**: HTTP framework with built-in validation & routing
- **GORM**: ORM for SQLite database access
- **JWT**: `github.com/golang-jwt/jwt/v5` for token signing
- **Axios**: Frontend HTTP client with request/response interceptors
- **Vite**: Frontend build tool (config in `vite.config.ts`)

## Key Files by Purpose

| Purpose             | Files                                                        |
| ------------------- | ------------------------------------------------------------ |
| Routes & startup    | `backend/main.go`                                            |
| Models & DB schema  | `backend/model/model.go`, `backend/repository/db.go`         |
| Configuration       | `backend/config/config.go`                                   |
| Authentication      | `backend/middleware/auth.go`, `backend/api/v1/auth.go`       |
| API implementations | `backend/api/v1/*.go`                                        |
| Frontend UI         | `frontend/src/pages/*.tsx`, `frontend/src/components/`       |
| Frontend utils      | `frontend/src/utils/request.ts` (HTTP), `message.ts` (toast) |
| Build & deploy      | `build.ps1`, `deploy/Dockerfile`, `deploy/entrypoint.sh`     |
