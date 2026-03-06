# Nightingale 开发环境热部署启动脚本
# 使用方法: .\dev.ps1

Write-Host "启动 Nightingale 热部署开发环境..." -ForegroundColor Green
Write-Host "按 Ctrl+C 停止服务" -ForegroundColor Yellow

# 检查 air 是否已安装
try {
    air --help | Out-Null
    Write-Host "✓ air 已安装" -ForegroundColor Green
} catch {
    Write-Host "✗ air 未安装，请先运行: go install github.com/air-verse/air@latest" -ForegroundColor Red
    exit 1
}

# 创建临时目录
if (!(Test-Path "tmp")) {
    New-Item -ItemType Directory -Path "tmp" | Out-Null
    Write-Host "✓ 创建临时目录 tmp/" -ForegroundColor Green
}

# 启动热部署
Write-Host "开始监听文件变化..." -ForegroundColor Cyan
air
