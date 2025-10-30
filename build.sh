#!/bin/bash

echo "构建 System Tune Agent (Go 版本)..."

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go，请先安装 Go 1.19 或更高版本"
    echo "下载地址: https://golang.org/dl/"
    exit 1
fi

# 显示 Go 版本
echo "Go 版本: $(go version)"

# 下载依赖
echo "下载依赖..."
go mod tidy

# 构建不同平台的可执行文件
echo "构建可执行文件..."

# 当前平台
echo "构建当前平台版本..."
go build -ldflags="-s -w" -o system-tune-agent main.go

# Linux 64位
echo "构建 Linux 64位版本..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-linux-amd64 main.go

# Windows 64位
echo "构建 Windows 64位版本..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-windows-amd64.exe main.go

# macOS 64位 (Intel)
echo "构建 macOS Intel 版本..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-darwin-amd64 main.go

# macOS ARM64 (Apple Silicon)
echo "构建 macOS Apple Silicon 版本..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o system-tune-agent-darwin-arm64 main.go

echo ""
echo "构建完成！生成的文件:"
ls -lh system-tune-agent*

echo ""
echo "运行示例:"
echo "  ./system-tune-agent -c 75 -m 75"
echo "  ./system-tune-agent --help"

echo ""
echo "跨平台文件说明:"
echo "  system-tune-agent-linux-amd64     - Linux 64位"
echo "  system-tune-agent-windows-amd64.exe - Windows 64位"
echo "  system-tune-agent-darwin-amd64     - macOS Intel"
echo "  system-tune-agent-darwin-arm64     - macOS Apple Silicon"