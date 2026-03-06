#!/bin/bash
# Nightingale 开发环境热部署启动脚本
# 使用方法: ./dev.sh

echo "启动 Nightingale 热部署开发环境..."
echo "按 Ctrl+C 停止服务"

# 检查 air 是否已安装
if command -v air &> /dev/null; then
    echo "✓ air 已安装"
else
    echo "✗ air 未安装，请先运行: go install github.com/air-verse/air@latest"
    exit 1
fi

# 创建临时目录
if [ ! -d "tmp" ]; then
    mkdir -p tmp
    echo "✓ 创建临时目录 tmp/"
fi

# 启动热部署
echo "开始监听文件变化..."
air
