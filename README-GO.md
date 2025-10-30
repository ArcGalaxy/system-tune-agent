# System Tune Agent (Go 版本)

一个用 Go 语言编写的轻量级系统资源监控和调节工具，能够动态调整自身的 CPU 和内存占用，使系统总体资源使用率维持在设定的阈值附近。

## 🚀 Go 版本的优势

- **无运行时依赖**: 编译成单一可执行文件，无需安装 Go 环境
- **跨平台支持**: 一次编译，支持 Linux、Windows、macOS
- **高性能**: 接近原生 C 的性能，比 Java 版本更高效
- **低内存占用**: 比 JVM 版本节省大量内存
- **快速启动**: 毫秒级启动，无 JVM 预热时间
- **精确控制**: 更精确的资源控制和实时响应

## 📦 快速开始

### 方式一：直接下载可执行文件

从 releases 页面下载对应平台的可执行文件：
- `system-tune-agent-linux-amd64` - Linux 64位
- `system-tune-agent-windows-amd64.exe` - Windows 64位  
- `system-tune-agent-darwin-amd64` - macOS Intel
- `system-tune-agent-darwin-arm64` - macOS Apple Silicon

### 方式二：从源码构建

```bash
# 克隆项目
git clone <repository-url>
cd system-tune-agent

# 构建 (需要 Go 1.19+)
./build.sh        # Linux/macOS
# 或
build.bat         # Windows

# 运行
./system-tune-agent -c 75 -m 75
```

## 🛠️ 使用方法

### 基本用法

```bash
# 使用默认设置 (CPU: 75%, 内存: 75%)
./system-tune-agent

# 自定义阈值
./system-tune-agent -c 80 -m 70

# 查看帮助
./system-tune-agent --help
```

### 命令行参数

| 参数 | 简写 | 描述 | 默认值 |
|------|------|------|--------|
| `--cpu` | `-c` | CPU 使用率阈值 (1-100) | 75 |
| `--memory` | `-m` | 内存使用率阈值 (1-100) | 75 |
| `--help` | `-h` | 显示帮助信息 | - |

## 📊 工作原理

### 智能算法
- **平滑调节**: 避免忽高忽低，维持稳定的资源使用率
- **动态强度**: CPU 强度 0-100% 可调，精确控制
- **渐进式内存管理**: 逐步分配和释放内存，避免突变
- **实时监控**: 2秒间隔监控，快速响应系统变化

### CPU 控制策略
```
差距 > 2%  → 快速调整 (±5-15%)
差距 ≤ 2%  → 缓慢调整 (±1-3%)
接近阈值   → 立即停止增加
```

### 内存控制策略
```
差距 > 3%  → 开始分配内存
差距 < -1% → 逐步释放 30%
差距 < -3% → 停止分配
```

## 🔧 系统要求

- **Linux**: 任何现代 Linux 发行版
- **Windows**: Windows 7 或更高版本
- **macOS**: macOS 10.12 或更高版本
- **内存**: 至少 50MB 可用内存
- **权限**: 普通用户权限即可

## 📈 性能对比

| 指标 | Go 版本 | Java 版本 |
|------|---------|-----------|
| 文件大小 | ~8MB | ~50MB+ |
| 启动时间 | <100ms | 2-5s |
| 内存占用 | ~10-20MB | ~100MB+ |
| CPU 开销 | 极低 | 中等 |
| 响应速度 | 毫秒级 | 秒级 |

## 🎯 使用场景

### 1. 服务器压力测试
```bash
# 模拟高负载环境
./system-tune-agent -c 90 -m 85
```

### 2. 性能基准测试
```bash
# 精确控制资源使用率
./system-tune-agent -c 75 -m 70
```

### 3. 系统预热
```bash
# 为突发负载预留资源
./system-tune-agent -c 60 -m 60
```

### 4. 开发环境模拟
```bash
# 模拟生产环境资源使用情况
./system-tune-agent -c 80 -m 75
```

## 🔍 监控输出示例

```
System Tune Agent 启动 (Go 版本)
CPU 阈值: 75.0%
内存阈值: 75.0%
监控间隔: 2s
按 Ctrl+C 停止程序

当前状态 - CPU: 45.2%, 内存: 32.1%
调整 CPU 强度: 45% (当前: 45.2%, 目标: 75.0%)
启动 8 个 CPU 消费线程
内存状态 - 当前: 32.1%, 目标: 75.0%, 差距: 42.9%, 已分配块: 0
开始增加内存占用，目标: 75.0%
分配内存块: 50MB

当前状态 - CPU: 72.8%, 内存: 68.5%
调整 CPU 强度: 48% (当前: 72.8%, 目标: 75.0%)
内存状态 - 当前: 68.5%, 目标: 75.0%, 差距: 6.5%, 已分配块: 15
分配内存块: 13MB

当前状态 - CPU: 74.9%, 内存: 74.2%
内存状态 - 当前: 74.2%, 目标: 75.0%, 差距: 0.8%, 已分配块: 18
```

## 🛡️ 安全说明

- 程序只消耗系统资源，不访问敏感数据
- 使用 Ctrl+C 可以安全停止并释放所有资源
- 不需要管理员权限
- 不会修改系统配置或文件

## 🔧 开发构建

### 环境要求
- Go 1.19 或更高版本
- 网络连接 (下载依赖)

### 构建命令
```bash
# 下载依赖
go mod tidy

# 构建当前平台
go build -o system-tune-agent main.go

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o system-tune-agent-linux main.go
GOOS=windows GOARCH=amd64 go build -o system-tune-agent.exe main.go
```

## 📄 许可证

MIT License