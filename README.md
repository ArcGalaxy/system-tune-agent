# System Tune Agent

> 🚀 智能系统资源调节工具 - 动态平衡CPU和内存使用率

## 📋 项目简介

System Tune Agent 是一个智能的系统资源调节工具，能够动态调整系统的CPU和内存使用率到指定阈值。采用先进的子进程内存管理架构和智能内存分析算法，实现精确的资源控制和立即内存释放。

### 🎯 核心特性

- **🧠 智能内存管理**: 自动分析用户程序内存使用，智能让出和恢复内存
- **⚡ 立即内存释放**: 基于子进程架构，进程销毁时操作系统立即回收内存
- **🔄 动态平衡**: 实时监控系统资源，自动调整到目标阈值
- **🌍 跨平台支持**: 支持 Windows、Linux、macOS (x86_64/ARM64)
- **📊 详细日志**: 完整的操作日志和性能监控信息
- **🛡️ 安全可靠**: 优雅退出处理，自动清理所有资源

## 🏗️ 技术架构

### 内存管理架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   主进程         │    │   memory1       │    │   memory2       │
│  (监控调度)      │───▶│  (500MB内存)     │    │  (500MB内存)     │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  智能分析引擎     │    │   立即内存释放    │    │   立即内存释放    │
│ - 用户内存分析    │    │ - 进程销毁       │    │ - 进程销毁        │
│ - 动态调整策略    │    │ - 系统回收       │    │ - 系统回收        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 智能内存管理流程

1. **基准分析**: 启动时记录系统基准内存使用率
2. **实时监控**: 每2秒分析系统内存使用情况
3. **智能决策**: 
   - 用户内存增加 → 代理程序自动让出内存
   - 用户内存释放 → 代理程序恢复到目标阈值
4. **精确控制**: 5秒冷却时间，避免频繁调整

## 🚀 快速开始

### 下载和安装

从 [Releases](releases) 下载对应平台的可执行文件：

```bash
# Linux
wget https://github.com/your-repo/system-tune-agent/releases/latest/download/system-tune-agent-linux-amd64.tar.gz
tar -xzf system-tune-agent-linux-amd64.tar.gz

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/your-repo/system-tune-agent/releases/latest/download/system-tune-agent-windows-amd64.zip" -OutFile "system-tune-agent.zip"
Expand-Archive system-tune-agent.zip

# macOS
curl -L https://github.com/your-repo/system-tune-agent/releases/latest/download/system-tune-agent-darwin-amd64.tar.gz | tar -xz
```

### 基本使用

```bash
# 默认设置 (CPU: 75%, 内存: 75%)
./system-tune-agent

# 自定义阈值
./system-tune-agent -c 80 -m 70

# Windows
system-tune-agent-windows-amd64.exe -c 80 -m 70

# 查看帮助
./system-tune-agent --help
```

### 使用示例

```bash
# 高性能模式 (CPU: 90%, 内存: 85%)
./system-tune-agent -c 90 -m 85

# 节能模式 (CPU: 60%, 内存: 50%)
./system-tune-agent -c 60 -m 50

# 内存密集型 (CPU: 70%, 内存: 90%)
./system-tune-agent -c 70 -m 90
```

## 📊 运行效果

### 日志输出示例

```
[15:30:15.123] 🚀 System Tune Agent 启动 (子进程模式)
[15:30:15.124] 📋 配置: CPU阈值=75.0%, 内存阈值=75.0%
[15:30:17.125] 📊 系统状态: CPU 45.2% (目标: 75.0%) | 内存 58.3% (目标: 75.0%) | CPU强度: 0% | 子进程: 0个/0MB
[15:30:17.126] 🧠 内存详情: 用户 ~58.3% | 代理 ~0.0% | 基准 58.3% | 距离上次调整 0.0s
[15:30:17.127] 🎯 智能内存调整: 用户内存正常 (58.3%)，恢复到目标阈值
[15:30:17.128] 🚀 启动内存工作进程 memory1 (PID: 12345, 大小: 500MB)
[15:30:19.130] 📊 系统状态: CPU 47.1% (目标: 75.0%) | 内存 72.1% (目标: 75.0%) | CPU强度: 0% | 子进程: 1个/500MB
[15:30:19.131] 🧠 内存详情: 用户 ~58.3% | 代理 ~13.8% | 基准 58.3% | 距离上次调整 2.0s
```

### 智能内存管理演示

```
# 用户程序启动，占用大量内存
[15:32:15.456] 🔽 内存使用率过高 (82.1% > 80.0%)，减少内存消费
[15:32:15.457] 🎯 智能内存调整: 用户内存增加到 68.5%，让出内存
[15:32:15.458] 🗑️ 正在销毁内存工作进程 memory2 (PID: 12347, 500MB)...
[15:32:15.459] 💀 强制销毁内存工作进程 memory2 (PID: 12347, 运行时间: 120.3秒)

# 用户程序退出，释放内存
[15:35:20.789] 🔼 内存使用率过低 (62.3% < 70.0%)，增加内存消费
[15:35:20.790] 🎯 智能内存调整: 用户内存正常 (58.1%)，恢复到目标阈值
[15:35:20.791] 🚀 启动内存工作进程 memory3 (PID: 12350, 大小: 500MB)
```

## ⚙️ 命令行参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--cpu` | `-c` | 75 | CPU使用率阈值 (1-100) |
| `--memory` | `-m` | 75 | 内存使用率阈值 (1-100) |
| `--help` | `-h` | - | 显示帮助信息 |

### 内部参数 (自动使用)
- `--memory-worker <MB>`: 内存工作进程模式 (内部使用)

## 🖥️ 系统支持

### 操作系统支持

| 操作系统 | x86_64/amd64 | ARM64 | 状态 |
|----------|--------------|-------|------|
| **Linux** | ✅ | ✅ | 完全支持 |
| **Windows** | ✅ | ✅ | 完全支持 |
| **macOS** | ✅ | ✅ | 完全支持 |

### 系统要求

- **最低内存**: 1GB 可用内存
- **权限要求**: 
  - Linux/macOS: 普通用户权限
  - Windows: 建议管理员权限 (获得最佳性能)
- **网络要求**: 无需网络连接

## 🔧 高级配置

### 性能调优

```bash
# 高精度模式 (更频繁的监控)
./system-tune-agent -c 75 -m 75  # 默认2秒监控间隔

# 大内存系统优化
./system-tune-agent -c 80 -m 90  # 适合32GB+内存系统

# 服务器环境
./system-tune-agent -c 85 -m 80  # 高性能服务器配置
```

### Windows 特殊配置

如果在Windows上遇到子进程创建问题：

1. **以管理员身份运行**
2. **添加到杀毒软件白名单**
3. **程序会自动切换到备用模式**

详见: [Windows问题解决指南](WINDOWS_TROUBLESHOOTING.md)

## 📈 性能对比

### 内存释放速度对比

| 版本 | 释放方式 | 释放速度 | 释放完整性 | 适用场景 |
|------|----------|----------|------------|----------|
| **子进程版本** | 进程销毁 | **立即释放** | **100%** | 生产环境 |
| 备用Go版本 | GC回收 | 延迟释放 | 依赖GC | 兼容模式 |

### 资源控制精度

- **CPU控制精度**: ±2%
- **内存控制精度**: ±3%
- **响应时间**: 2-5秒
- **内存释放时间**: <1秒 (子进程模式)

## 🛠️ 开发和构建

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/your-repo/system-tune-agent.git
cd system-tune-agent

# 安装Go依赖
go mod tidy

# 构建当前平台
go build -o system-tune-agent main.go

# 构建所有平台
./build.sh
```

### 依赖项

```go
require (
    github.com/shirou/gopsutil/v3 v3.23.10
)
```

## 🔍 故障排除

### 常见问题

**Q: Windows上提示"cannot run executable found relative to current directory"**
A: 程序会自动切换到备用模式，功能完全相同。或者以管理员身份运行。

**Q: 内存使用率达不到设定阈值**
A: 检查系统可用内存是否充足，程序会智能避免导致系统不稳定。

**Q: CPU使用率波动较大**
A: 这是正常现象，程序会根据系统负载动态调整。

**Q: 程序退出后内存没有立即释放**
A: 子进程模式会立即释放，备用模式依赖Go GC，可能有延迟。

### 日志分析

程序会自动生成详细日志文件：`system-tune-agent_YYYYMMDD_HHMMSS.log`

关键日志标识：
- `🚀` 进程启动
- `📊` 系统状态
- `🧠` 智能分析
- `🎯` 调整决策
- `🔼🔽` 内存调整
- `⚠️❌` 错误信息

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📞 支持

- 📧 邮件: [your-email@example.com]
- 🐛 问题反馈: [GitHub Issues](https://github.com/your-repo/system-tune-agent/issues)
- 📖 文档: [项目Wiki](https://github.com/your-repo/system-tune-agent/wiki)

## 🎉 致谢

感谢以下开源项目：
- [gopsutil](https://github.com/shirou/gopsutil) - 系统信息获取
- [Go](https://golang.org/) - 编程语言

---

<div align="center">

**🌟 如果这个项目对你有帮助，请给个 Star！🌟**

Made with ❤️ by [Your Name]

</div>