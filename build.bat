@echo off
echo 构建 System Tune Agent (Go 版本)...

REM 检查 Go 是否安装
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo 错误: 未找到 Go，请先安装 Go 1.19 或更高版本
    echo 下载地址: https://golang.org/dl/
    pause
    exit /b 1
)

REM 显示 Go 版本
echo Go 版本:
go version

REM 下载依赖
echo 下载依赖...
go mod tidy

REM 构建不同平台的可执行文件
echo 构建可执行文件...

REM 当前平台 (Windows)
echo 构建 Windows 版本...
go build -ldflags="-s -w" -o system-tune-agent.exe main.go

REM Linux 64位
echo 构建 Linux 64位版本...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o system-tune-agent-linux-amd64 main.go

REM Windows 64位
echo 构建 Windows 64位版本...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o system-tune-agent-windows-amd64.exe main.go

REM macOS 64位 (Intel)
echo 构建 macOS Intel 版本...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o system-tune-agent-darwin-amd64 main.go

REM macOS ARM64 (Apple Silicon)
echo 构建 macOS Apple Silicon 版本...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o system-tune-agent-darwin-arm64 main.go

REM 重置环境变量
set GOOS=
set GOARCH=

echo.
echo 构建完成！生成的文件:
dir system-tune-agent*

echo.
echo 运行示例:
echo   system-tune-agent.exe -c 75 -m 75
echo   system-tune-agent.exe --help

echo.
echo 跨平台文件说明:
echo   system-tune-agent-linux-amd64.exe     - Linux 64位
echo   system-tune-agent-windows-amd64.exe   - Windows 64位
echo   system-tune-agent-darwin-amd64.exe    - macOS Intel
echo   system-tune-agent-darwin-arm64.exe    - macOS Apple Silicon

pause