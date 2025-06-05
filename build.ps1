# 设置错误时停止执行
$ErrorActionPreference = "Stop"

Write-Host "开始构建应用..." -ForegroundColor Green

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

# 设置新的环境变量
$env:GOOS = "linux"
$env:GOARCH = "amd64"

# 执行构建
go build -o "$PSScriptRoot\deploy\app" .

# 检查构建结果
if ($LASTEXITCODE -ne 0) {
    # 恢复环境变量
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    Write-Host "后端构建失败！" -ForegroundColor Red
    exit 1
}

# 恢复环境变量
$env:GOOS = $originalGOOS
$env:GOARCH = $originalGOARCH

Write-Host "构建完成！" -ForegroundColor Green
Write-Host "部署文件位于: $PSScriptRoot\deploy\" -ForegroundColor Green
