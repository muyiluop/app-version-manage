# 本地一键构建脚本
#
#   .\build.ps1              # 构建前端 + 后端（linux 二进制）
#   .\build.ps1 -BuildType web    # 只构建前端
#   .\build.ps1 -BuildType api    # 只构建后端
#   .\build.ps1 -BuildType docker # 构建 Docker 镜像
param(
    [ValidateSet("all", "web", "api", "docker")]
    [string]$BuildType = "all",
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

function Build-Web {
    Write-Host "==> 构建前端" -ForegroundColor Yellow
    Push-Location (Join-Path $root "frontend")
    try {
        npm.cmd ci
        npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    } finally { Pop-Location }

    $www = Join-Path $root "deploy\www"
    New-Item -ItemType Directory -Path $www -Force | Out-Null
    Copy-Item -Path (Join-Path $root "frontend\dist\*") -Destination $www -Recurse -Force
}

function Build-Api {
    Write-Host "==> 构建后端 (linux/amd64)" -ForegroundColor Yellow
    Push-Location (Join-Path $root "backend")
    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -trimpath -ldflags "-s -w" -o (Join-Path $root "deploy\appv") ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "后端构建失败" }
    } finally {
        Remove-Item env:CGO_ENABLED -ErrorAction SilentlyContinue
        Remove-Item env:GOOS -ErrorAction SilentlyContinue
        Remove-Item env:GOARCH -ErrorAction SilentlyContinue
        Pop-Location
    }
}

function Build-Docker {
    Write-Host "==> 构建 Docker 镜像" -ForegroundColor Yellow
    docker build -t "app-version-manage:$Version" --build-arg "VERSION=$Version" -f (Join-Path $root "deploy\Dockerfile") $root
    if ($LASTEXITCODE -ne 0) { throw "Docker 构建失败" }
}

switch ($BuildType) {
    "web"    { Build-Web }
    "api"    { Build-Api }
    "docker" { Build-Docker }
    "all"    { Build-Web; Build-Api }
}

Write-Host "构建完成 ($BuildType)" -ForegroundColor Green
