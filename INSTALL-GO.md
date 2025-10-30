# Go 安装指南

## macOS 安装 Go

### 方式一：使用 Homebrew (推荐)
```bash
# 安装 Homebrew (如果还没有)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 安装 Go
brew install go

# 验证安装
go version
```

### 方式二：官方安装包
1. 访问 https://golang.org/dl/
2. 下载 macOS 版本 (go1.21.x.darwin-amd64.pkg 或 go1.21.x.darwin-arm64.pkg)
3. 双击安装包，按提示安装
4. 打开新的终端窗口
5. 验证：`go version`

### 方式三：手动安装
```bash
# 下载 Go (Apple Silicon Mac)
curl -O https://go.dev/dl/go1.21.5.darwin-arm64.tar.gz

# 或者 Intel Mac
curl -O https://go.dev/dl/go1.21.5.darwin-amd64.tar.gz

# 解压到 /usr/local
sudo tar -C /usr/local -xzf go1.21.5.darwin-*.tar.gz

# 添加到 PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc
source ~/.zshrc

# 验证
go version
```

## 安装完成后

```bash
# 回到项目目录
cd ~/work/code/SystemTuneAgent

# 初始化依赖
go mod tidy

# 构建项目
go build -o system-tune-agent main.go

# 运行
./system-tune-agent -c 75 -m 75
```

## 如果不想安装 Go

我也可以为你提供一个预编译的可执行文件，或者用其他语言重写这个工具。