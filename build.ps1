# 设置错误时停止执行
$ErrorActionPreference = "Stop"

# 定义构建类型参数
param(
    [ValidateSet("docker", "local", "all")]
    [string]$BuildType = "docker"
)

Write-Host "开始构建应用 (构建类型: $BuildType)..." -ForegroundColor Green

# 本地构建流程（前端 + 后端）
if ($BuildType -eq "local" -or $BuildType -eq "all") {
    # 构建前端
    Write-Host "构建前端项目..." -ForegroundColor Yellow
    Set-Location -Path "$PSScriptRoot\frontend"
    npm install
    npm run build

    if ($LASTEXITCODE -ne 0) {
        Write-Host "前端构建失败！" -ForegroundColor Red
        exit 1
    }

    # 清理旧的部署文件
    Write-Host "清理部署目录..." -ForegroundColor Yellow
    Remove-Item -Path "$PSScriptRoot\deploy\www\*" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path "$PSScriptRoot\deploy\www" -Force | Out-Null

    # 复制前端构建文件到部署目录
    Write-Host "复制前端文件到部署目录..." -ForegroundColor Yellow
    Copy-Item -Path "$PSScriptRoot\frontend\dist\*" -Destination "$PSScriptRoot\deploy\www\" -Recurse -Force

    # 构建后端
    Write-Host "构建后端项目..." -ForegroundColor Yellow
    Set-Location -Path "$PSScriptRoot\backend"

    # 保存当前环境变量
    $originalGOOS = $env:GOOS
    $originalGOARCH = $env:GOARCH

    try {
        # 设置交叉编译环境变量
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:GOPROXY = "https://goproxy.io,direct"
        
        # 执行构建
        go build -o "$PSScriptRoot\deploy\app" .
        
        # 检查构建结果
        if ($LASTEXITCODE -ne 0) {
            Write-Host "后端构建失败！" -ForegroundColor Red
            exit 1
        }
    }
    finally {
        # 确保无论如何都会恢复环境变量
        if ($originalGOOS) { $env:GOOS = $originalGOOS } else { Remove-Item env:GOOS -ErrorAction SilentlyContinue }
        if ($originalGOARCH) { $env:GOARCH = $originalGOARCH } else { Remove-Item env:GOARCH -ErrorAction SilentlyContinue }
    }
    
    Write-Host "本地构建完成！" -ForegroundColor Green
}

# Docker 构建流程
if ($BuildType -eq "docker" -or $BuildType -eq "all") {
    Write-Host "构建 Docker 镜像..." -ForegroundColor Yellow
    Set-Location -Path "$PSScriptRoot"

    docker build -t app-version-manage:latest -f "$PSScriptRoot\deploy\Dockerfile" .

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Docker 构建失败！" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "Docker 构建完成！镜像: app-version-manage:latest" -ForegroundColor Green
}

Write-Host "全部构建完成！" -ForegroundColor Green
