#!/bin/bash

echo "🚀 开始构建 System Tune Agent (动态平衡版本)"
echo "================================================"

# 创建构建目录
mkdir -p build

# 获取版本信息
VERSION="v1.1.0-dynamic"
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

echo "📦 版本: ${VERSION}"
echo "⏰ 构建时间: ${BUILD_TIME}"
echo "🔗 Git提交: ${GIT_COMMIT}"
echo ""

# 构建函数
build_target() {
    local os=$1
    local arch=$2
    local ext=$3
    local output="build/system-tune-agent-${os}-${arch}${ext}"
    
    echo "🔨 构建 ${os}/${arch}..."
    GOOS=${os} GOARCH=${arch} go build -ldflags="${LDFLAGS}" -o "${output}" main.go
    
    if [ $? -eq 0 ]; then
        size=$(ls -lh "${output}" | awk '{print $5}')
        echo "✅ ${output} (${size})"
    else
        echo "❌ 构建失败: ${os}/${arch}"
    fi
}

# 构建各平台版本
echo "开始构建各平台版本..."
echo ""

# Linux
build_target "linux" "amd64" ""
build_target "linux" "arm64" ""

# Windows  
build_target "windows" "amd64" ".exe"
build_target "windows" "arm64" ".exe"

# macOS
build_target "darwin" "amd64" ""
build_target "darwin" "arm64" ""

echo ""
echo "📋 构建完成！文件列表："
ls -lh build/

echo ""
echo "📦 创建压缩包..."
cd build
for file in system-tune-agent-*; do
    if [[ "$file" == *.exe ]]; then
        zip "${file%.exe}.zip" "$file"
        echo "📦 ${file%.exe}.zip"
    else
        tar -czf "${file}.tar.gz" "$file"
        echo "📦 ${file}.tar.gz"
    fi
done

cd ..
echo ""
echo "🎉 所有构建完成！"
echo "📁 文件位置: ./build/"
echo ""
echo "🚀 使用方法："
echo "   Linux:   ./system-tune-agent-linux-amd64 -c 30 -m 75"
echo "   Windows: system-tune-agent-windows-amd64.exe -c 30 -m 75"
echo "   macOS:   ./system-tune-agent-darwin-amd64 -c 30 -m 75"