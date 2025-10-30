#!/bin/bash

echo "构建 System Tune Agent (Go 简化版 - 无外部依赖)..."

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go，请先安装 Go"
    echo "参考 INSTALL-GO.md 文件进行安装"
    exit 1
fi

echo "Go 版本: $(go version)"

# 构建简化版本 (无外部依赖)
echo "构建简化版本..."
go build -ldflags="-s -w" -o system-tune-agent-simple main-simple.go

# 构建不同平台版本
echo "构建跨平台版本..."

# Linux 64位
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-simple-linux-amd64 main-simple.go

# Windows 64位
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-simple-windows-amd64.exe main-simple.go

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o system-tune-agent-simple-darwin-amd64 main-simple.go

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o system-tune-agent-simple-darwin-arm64 main-simple.go

echo ""
echo "构建完成！生成的文件:"
ls -lh system-tune-agent-simple*

echo ""
echo "运行示例:"
echo "  ./system-tune-agent-simple -c 75 -m 75"
echo "  ./system-tune-agent-simple --help"

echo ""
echo "注意: 简化版本使用系统命令获取资源信息，适用于 macOS 和 Linux"