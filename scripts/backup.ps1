# 数据库与上传文件的备份脚本。
#
# 用法（在仓库根目录执行）：
#   .\scripts\backup.ps1              # 本地: 备份 backend/data + backend/static/uploads
#   .\scripts\backup.ps1 -Mode docker # 容器: 通过 docker compose 备份 /data
#
# SQLite 使用 VACUUM INTO 生成一致性快照，可在服务运行中执行。
param(
    [ValidateSet("local", "docker")]
    [string]$Mode = "local",
    [string]$OutDir = "backups",
    [string]$ComposeService = "app"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$target = Join-Path $root (Join-Path $OutDir $stamp)
New-Item -ItemType Directory -Path $target -Force | Out-Null

if ($Mode -eq "docker") {
    Write-Host "容器模式：备份 /data 到 $target" -ForegroundColor Yellow
    docker compose exec -T $ComposeService /app/appv -config /app/config.yaml -backup /data/backups
    docker compose cp "${ComposeService}:/data/backups/." $target
    Write-Host "完成: $target" -ForegroundColor Green
    exit 0
}

$backend = Join-Path $root "backend"
$dbDir = Join-Path $backend "data"
$uploadDir = Join-Path $backend "static\uploads"

if (-not (Test-Path $dbDir)) {
    throw "未找到数据库目录: $dbDir"
}

# 复用后端内置的在线备份能力，避免直接复制进行中的 WAL 文件
Push-Location $backend
try {
    & go run ./cmd/server -config config.yaml -backup (Join-Path $target "db")
} finally {
    Pop-Location
}

if (Test-Path $uploadDir) {
    $zip = Join-Path $target "uploads.zip"
    Compress-Archive -Path (Join-Path $uploadDir "*") -DestinationPath $zip -Force
    Write-Host "上传文件已打包: $zip" -ForegroundColor Green
}

Write-Host "备份完成: $target" -ForegroundColor Green
