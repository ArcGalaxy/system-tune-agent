# System Tune Agent

> 智能系统资源调节工具 - 精确控制CPU和内存使用率

## 项目简介

System Tune Agent 是一个智能的系统资源调节工具，具备弹性伸缩能力，能够精确控制系统的CPU和内存使用率到指定阈值。当用户程序占用资源增加时，自动释放让出资源；当系统空闲时，自动恢复到目标阈值保持平衡。采用多线程CPU控制和子进程内存管理架构，实现高精度的资源控制和智能负载均衡。

### 核心特性

- **弹性伸缩内存**: 智能检测用户程序内存变化，自动让出和回收资源
- **智能负载均衡**: 用户占用升高时自动释放，系统空闲时恢复到目标阈值
- **精确CPU控制**: 多线程架构，动态适配CPU核心数，精度±2%
- **子进程架构**: 立即内存释放，进程销毁时操作系统瞬间回收内存
- **安全限制**: CPU最大30%，避免系统过载，确保用户程序优先级
- **跨平台支持**: 支持 Windows、Linux、macOS (AMD64/ARM64)
- **自动适配**: 动态检测CPU核心数，自动优化线程策略

## 技术架构

### CPU控制架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│    主进程        │    │    CPU线程1      │    │   CPU线程N      │
│  （监控调度）     │───▶│  （核心1控制）    │    │  （核心N控制）    │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  动态核心检测     │    │   工作30ms       │    │   工作30ms      │
│ -runtime.NumCPU │    │   休息70ms       │    │   休息70ms      │
│ -自动适配线程数   │    │   (30%强度)      │    │   (30%强度)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 内存管理架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   主进程         │    │   memory1       │    │   memory2       │
│  (智能分析)      │───▶│  (500MB子进程)   │    │  (500MB子进程)   │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  用户内存分析     │    │   立即释放       │    │   立即释放       │
│ - 基准内存记录    │    │ - 进程销毁       │    │ - 进程销毁       │
│ - 动态调整策略    │    │ - 系统回收       │    │ - 系统回收       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 弹性伸缩机制

```
用户程序内存变化 → 智能检测 → 自动调整 → 保持平衡

┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  用户程序启动     │───▶│  内存使用率上升   │───▶│  自动释放内存     │
│  (大量内存占用)   │    │  (超过阈值+2%)   │    │  (让出给用户)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         ▲                                             │
         │              ┌─────────────────┐            │
         │              │  保持动态平衡     │◀───────────┘
         │              │  (实时监控)      │
         │              └─────────────────┘
         │                       │
         │              ┌─────────────────┐            ┌─────────────────┐
         └──────────────│  用户程序退出     │◀───────────│  自动恢复内存     │
                        │  (释放内存)      │            │  (回到目标阈值)   │
                        └─────────────────┘            └─────────────────┘
```

### 控制流程

1. **启动检测**: 动态检测CPU核心数，启动对应数量的工作线程
2. **基准分析**: 记录启动时的系统内存基准使用率
3. **实时监控**: 每2秒监控系统CPU和内存使用率
4. **弹性伸缩**: 
   - **用户内存增加** → 代理程序立即释放内存让出资源
   - **用户内存减少** → 代理程序恢复到目标阈值保持平衡
   - **CPU动态调整** → 根据系统负载自动启停CPU工作线程
5. **智能分析**: 区分用户程序和代理程序的资源占用
6. **安全保护**: CPU最大30%限制，确保用户程序优先级

## 快速开始

### 快速下载

**最新版本：v1.0.0** | [查看所有版本](https://github.com/ArcGalaxy/system-tune-agent/releases)

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| **Linux** | AMD64 | [system-tune-agent-linux-amd64.tar.gz](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-linux-amd64.tar.gz) |
| **Linux** | ARM64 | [system-tune-agent-linux-arm64.tar.gz](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-linux-arm64.tar.gz) |
| **Windows** | AMD64 | [system-tune-agent-windows-amd64.zip](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-windows-amd64.zip) |
| **Windows** | ARM64 | [system-tune-agent-windows-arm64.zip](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-windows-arm64.zip) |
| **macOS** | Intel | [system-tune-agent-darwin-amd64.tar.gz](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-darwin-amd64.tar.gz) |
| **macOS** | Apple Silicon | [system-tune-agent-darwin-arm64.tar.gz](https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-darwin-arm64.tar.gz) |

### 命令行下载

从 [Releases](https://github.com/ArcGalaxy/system-tune-agent/releases) 下载对应平台的可执行文件：

```bash
# Linux AMD64
wget https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-linux-amd64.tar.gz
tar -xzf system-tune-agent-linux-amd64.tar.gz

# Linux ARM64 (如树莓派)
wget https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-linux-arm64.tar.gz
tar -xzf system-tune-agent-linux-arm64.tar.gz

# Windows AMD64
wget https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-windows-amd64.zip
# 解压 system-tune-agent-windows-amd64.zip

# Windows ARM64
wget https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-windows-arm64.zip
# 解压 system-tune-agent-windows-arm64.zip

# macOS Intel
curl -L https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-darwin-amd64.tar.gz | tar -xz

# macOS Apple Silicon (M1/M2)
curl -L https://github.com/ArcGalaxy/system-tune-agent/releases/download/1.0.0/system-tune-agent-darwin-arm64.tar.gz | tar -xz
```

### 基本使用

```bash
# 默认设置 (CPU: 10%, 内存: 60%)
./system-tune-agent

# 自定义阈值 (CPU最大30%)
./system-tune-agent -c 20 -m 50

# Windows
system-tune-agent-windows-amd64.exe -c 20 -m 50

# 查看帮助
./system-tune-agent --help
```

### Linux后台启动

#### 方法1: 使用nohup (推荐)
```bash
# 启动之前先对对应的文件进行赋权
chmod +x ./system-tune-agent-xxxxx（对应平台的文件名称）
```
**带日志输出版本：**
```bash
# 后台启动，输出重定向到日志文件
nohup ./system-tune-agent -c 20 -m 70 > system-tune-agent.log 2>&1 &

# 查看进程状态
ps aux | grep system-tune-agent

# 查看实时日志
tail -f system-tune-agent.log
```

**不带日志输出版本（静默运行）：**
```bash
# 后台启动，丢弃所有输出
nohup ./system-tune-agent -c 20 -m 70 > /dev/null 2>&1 &

# 或者只保留错误输出
nohup ./system-tune-agent -c 20 -m 70 > /dev/null 2> error.log &

# 完全静默（不推荐，无法排查问题）
nohup ./system-tune-agent -c 20 -m 70 &> /dev/null &
```

**进程管理：**
```bash
# 查看进程状态
ps aux | grep system-tune-agent | grep -v grep

# 获取进程PID
PID=$(pgrep -f system-tune-agent)
echo "进程PID: $PID"

# 查看进程详细信息
ps -p $PID -o pid,ppid,cmd,etime,time,%cpu,%mem

# 停止程序的多种方法
# 方法1: 使用pkill (推荐)
pkill system-tune-agent

# 方法2: 使用killall
killall system-tune-agent

# 方法3: 使用PID停止
kill $(pgrep -f system-tune-agent)

# 方法4: 强制停止 (谨慎使用)
pkill -9 system-tune-agent

# 方法5: 精确匹配进程名停止
kill $(ps aux | grep '[s]ystem-tune-agent' | awk '{print $2}')

# 验证程序已停止
if pgrep -f system-tune-agent > /dev/null; then
    echo "程序仍在运行"
else
    echo "程序已停止"
fi
```

#### 方法2: 使用screen会话

```bash
# 创建新的screen会话
screen -S system-tune-agent

# 在screen中启动程序
./system-tune-agent -c 20 -m 70

# 分离会话 (Ctrl+A, 然后按D)
# 重新连接会话
screen -r system-tune-agent

# 列出所有会话
screen -ls
```

#### 方法3: 使用tmux会话

```bash
# 创建新的tmux会话
tmux new-session -d -s system-tune-agent

# 在tmux中启动程序
tmux send-keys -t system-tune-agent './system-tune-agent -c 20 -m 70' Enter

# 查看会话
tmux list-sessions

# 连接到会话
tmux attach-session -t system-tune-agent

# 分离会话 (Ctrl+B, 然后按D)
```

#### 方法4: 创建systemd服务 (推荐生产环境)

创建服务文件：
```bash
sudo nano /etc/systemd/system/system-tune-agent.service
```

服务文件内容：
```ini
[Unit]
Description=System Tune Agent - Resource Management Tool
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/system-tune-agent
ExecStart=/opt/system-tune-agent/system-tune-agent -c 20 -m 70
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/system-tune-agent

[Install]
WantedBy=multi-user.target
```

启用和管理服务：
```bash
# 复制程序到系统目录
sudo mkdir -p /opt/system-tune-agent
sudo cp system-tune-agent /opt/system-tune-agent/
sudo chmod +x /opt/system-tune-agent/system-tune-agent

# 重新加载systemd配置
sudo systemctl daemon-reload

# 启用服务（开机自启）
sudo systemctl enable system-tune-agent

# 启动服务
sudo systemctl start system-tune-agent

# 查看服务状态
sudo systemctl status system-tune-agent

# 查看日志
sudo journalctl -u system-tune-agent -f

# 停止服务
sudo systemctl stop system-tune-agent

# 禁用服务
sudo systemctl disable system-tune-agent
```

### 使用示例

```bash
# 前台运行 - 轻度负载
./system-tune-agent -c 15 -m 70

# 后台运行 - 中度负载 (带日志)
nohup ./system-tune-agent -c 25 -m 80 > tune-agent.log 2>&1 &

# 后台运行 - 静默模式 (无日志)
nohup ./system-tune-agent -c 25 -m 80 > /dev/null 2>&1 &

# 生产环境 - 最大负载
sudo systemctl start system-tune-agent  # 需要先配置systemd服务

# 临时测试 - 内存专项
screen -S memory-test
./system-tune-agent -c 10 -m 95
```

## 运行效果

### 日志输出示例

```
[11:30:05] CONFIG: CPU阈值30.0% (最大30%) | 内存阈值20.0%
[11:30:08] STATUS: CPU 2.0% (目标30.0%) | 内存 13.0% (目标20.0%) | CPU强度0% | 子进程:0个/0MB
[11:30:10] STATUS: CPU 4.5% (目标30.0%) | 内存 19.0% (目标20.0%) | CPU强度0% | 子进程:0个/0MB
[11:30:12] STATUS: CPU 3.7% (目标30.0%) | 内存 19.0% (目标20.0%) | CPU强度0% | 子进程:0个/0MB
[11:30:14] START: 启动24个CPU工作线程 (目标30%, 总核心24, 每核心工作30ms/休息70ms)
[11:30:16] STATUS: CPU 31.0% (目标30.0%) | 内存 19.0% (目标20.0%) | CPU强度30% | 子进程:8个/3397MB
[11:30:18] STATUS: CPU 30.6% (目标30.0%) | 内存 19.0% (目标20.0%) | CPU强度30% | 子进程:8个/3397MB
```

### 弹性伸缩演示

**场景1: 用户程序启动，占用大量内存**
```
# 系统空闲时，代理程序占用内存到目标阈值
[11:30:25] STATUS: 内存 19.8% (目标20.0%) | 子进程:1个/500MB
[11:30:27] ANALYZE: 用户 ~15.0% | 代理 ~4.8% | 基准 15.0%

# 用户程序启动，内存使用率快速上升
[11:32:15] STATUS: 内存 23.5% (目标20.0%) | 子进程:1个/500MB  
[11:32:17] ANALYZE: 用户 ~18.7% | 代理 ~4.8% | 基准 15.0%
[11:32:19] TARGET: 用户内存增加，立即释放代理内存
[11:32:21] DECREASE: 减少内存消费 500MB (让出给用户程序)
[11:32:23] STATUS: 内存 18.9% (目标20.0%) | 子进程:0个/0MB
```

**场景2: 用户程序退出，释放内存**
```
# 用户程序退出，内存使用率下降
[11:45:30] STATUS: 内存 15.2% (目标20.0%) | 子进程:0个/0MB
[11:45:32] ANALYZE: 用户 ~15.2% | 代理 ~0.0% | 基准 15.0%
[11:45:34] TARGET: 用户内存正常，恢复到目标阈值
[11:45:36] INCREASE: 增加内存消费 500MB (恢复平衡)
[11:45:38] STATUS: 内存 19.9% (目标20.0%) | 子进程:1个/500MB
```

**场景3: 持续的弹性伸缩**
```
# 多个用户程序交替运行，代理程序智能调整
[12:00:00] STATUS: 内存 19.8% | 子进程:1个/500MB    # 平衡状态
[12:05:15] STATUS: 内存 15.1% | 子进程:0个/0MB      # 让出资源
[12:10:30] STATUS: 内存 20.2% | 子进程:1个/500MB    # 恢复平衡
[12:15:45] STATUS: 内存 14.8% | 子进程:0个/0MB      # 再次让出
[12:20:00] STATUS: 内存 19.9% | 子进程:1个/500MB    # 再次恢复
```

## 命令行参数

| 参数 | 简写 | 默认值 | 范围 | 说明 |
|------|------|--------|------|------|
| `--cpu` | `-c` | 10 | 1-30 | CPU使用率阈值 (安全限制最大30%) |
| `--memory` | `-m` | 60 | 1-100 | 内存使用率阈值 |
| `--help` | `-h` | - | - | 显示帮助信息 |


### 参数说明

**CPU阈值限制原因：**
- 最大30%是为了避免系统过载
- 保证系统响应性和稳定性
- 适合长期运行的资源调节需求

## 系统支持

### 操作系统支持

| 操作系统 | AMD64 (x86_64) | ARM64 | 测试状态 |
|----------|----------------|-------|----------|
| **Linux** | ✅ 完全支持 | ✅ 完全支持 | 生产就绪 |
| **Windows** | ✅ 完全支持 | ✅ 完全支持 | 生产就绪 |
| **macOS** | ✅ 完全支持 | ✅ 完全支持 | 生产就绪 |

### 架构差异

**AMD64 (Intel/AMD):**
- 传统x86-64架构
- 适用于大部分服务器和PC
- 成熟稳定，性能优异

**ARM64:**
- 现代ARM架构
- 适用于Apple Silicon (M1/M2)、ARM服务器
- 功耗优化，性能出色

### 系统要求

- **最低内存**: 512MB 可用内存
- **CPU核心**: 1核心以上 (自动检测适配)
- **权限要求**: 
  - Linux/macOS: 普通用户权限即可
  - Windows: 普通用户权限 (管理员权限获得更好兼容性)
- **网络要求**: 无需网络连接，完全离线运行

## 后台运行监控

### 进程监控

```bash
# 查看程序是否运行
ps aux | grep system-tune-agent | grep -v grep

# 查看程序资源占用
top -p $(pgrep system-tune-agent)

# 查看程序详细信息
htop -p $(pgrep system-tune-agent)

# 查看程序启动时间和运行状态
ps -eo pid,ppid,cmd,etime,time | grep system-tune-agent
```

### 日志监控

```bash
# 实时查看nohup日志
tail -f system-tune-agent.log

# 查看systemd服务日志
sudo journalctl -u system-tune-agent -f

# 查看最近100行日志
sudo journalctl -u system-tune-agent -n 100

# 查看特定时间段日志
sudo journalctl -u system-tune-agent --since "2024-01-01 10:00:00"
```

### 性能监控脚本

创建监控脚本 `monitor.sh`：
```bash
#!/bin/bash
# System Tune Agent 监控脚本

PROCESS_NAME="system-tune-agent"
LOG_FILE="/var/log/system-tune-agent-monitor.log"

while true; do
    # 检查进程是否运行
    if pgrep -f "$PROCESS_NAME" > /dev/null; then
        # 获取进程信息
        PID=$(pgrep -f "$PROCESS_NAME")
        CPU_USAGE=$(ps -p $PID -o %cpu --no-headers)
        MEM_USAGE=$(ps -p $PID -o %mem --no-headers)
        RUNTIME=$(ps -p $PID -o etime --no-headers)
        
        echo "$(date): 进程运行正常 - PID:$PID CPU:${CPU_USAGE}% MEM:${MEM_USAGE}% 运行时间:$RUNTIME" >> $LOG_FILE
    else
        echo "$(date): 警告 - $PROCESS_NAME 进程未运行!" >> $LOG_FILE
        
        # 可选：自动重启
        # systemctl start system-tune-agent
    fi
    
    sleep 60  # 每分钟检查一次
done
```

### 自动重启配置

#### systemd服务自动重启
```ini
# 在服务文件中添加
[Service]
Restart=always
RestartSec=10
StartLimitInterval=300
StartLimitBurst=5
```

### Windows 兼容性

如果在Windows上遇到子进程创建问题：

1. **程序会自动切换到备用模式** (Go内存管理)
2. **功能完全相同**，只是内存释放可能有延迟
3. **可选方案**：以管理员身份运行获得最佳性能

## 性能指标

### CPU控制精度

| 目标值 | 实际范围 | 精度 | 响应时间 |
|--------|----------|------|----------|
| 10% | 8-12% | ±2% | 2-5秒 |
| 20% | 18-22% | ±2% | 2-5秒 |
| 30% | 28-32% | ±2% | 2-5秒 |

### 弹性伸缩性能

| 场景 | 检测时间 | 响应时间 | 释放速度 | 恢复时间 |
|------|----------|----------|----------|----------|
| **用户内存增加** | 2秒 | **立即** | **<1秒** | - |
| **用户内存减少** | 2秒 | 2-5秒 | - | **<3秒** |
| **持续伸缩** | 实时 | 动态 | **<1秒** | **<3秒** |

### 内存管理性能

| 模式 | 释放方式 | 释放速度 | 释放完整性 | 弹性伸缩 |
|------|----------|----------|------------|----------|
| **子进程模式** | 进程销毁 | **<1秒** | **100%** | **支持** |
| 备用Go模式 | GC回收 | 1-5秒 | 依赖GC | 支持 |

### 多核心扩展性

| CPU核心数 | 工作线程数 | CPU控制效果 | 内存控制效果 |
|-----------|------------|-------------|-------------|
| 4核心 | 4线程 | 优秀 | 优秀 |
| 8核心 | 8线程 | 优秀 | 优秀 |
| 16核心 | 16线程 | 优秀 | 优秀 |
| 24核心+ | 24线程+ | 优秀 | 优秀 |

### 系统资源占用

- **程序本身内存占用**: ~10MB
- **CPU占用**: 目标值范围内
- **启动时间**: <1秒
- **停止时间**: <2秒 (完全清理)

## 开发和构建

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/ArcGalaxy/system-tune-agent.git
cd system-tune-agent

# 安装Go依赖
go mod tidy

# 构建当前平台
go build -o system-tune-agent main.go

# 构建所有平台 (Linux/Windows/macOS, AMD64/ARM64)
chmod +x build.sh
./build.sh
```

### 构建输出

构建脚本会生成以下文件：

```
build/
├── system-tune-agent-linux-amd64          # Linux AMD64
├── system-tune-agent-linux-arm64          # Linux ARM64  
├── system-tune-agent-windows-amd64.exe    # Windows AMD64
├── system-tune-agent-windows-arm64.exe    # Windows ARM64
├── system-tune-agent-darwin-amd64         # macOS Intel
├── system-tune-agent-darwin-arm64         # macOS Apple Silicon
├── *.tar.gz                               # Linux/macOS 压缩包
└── *.zip                                  # Windows 压缩包
```

### 依赖项

```go
require (
    github.com/shirou/gopsutil/v3 v3.23.10  // 系统信息获取
)
```

### 技术栈

- **语言**: Go 1.19+
- **架构**: 多线程 + 子进程
- **依赖**: 最小化依赖，仅系统信息库
- **编译**: 静态编译，无外部依赖

## 故障排除

### 常见问题

**Q: CPU使用率达不到设定值**
A: 检查是否超过30%限制。程序会自动限制最大CPU使用率确保系统稳定。

**Q: 内存使用率达不到设定阈值**  
A: 检查系统可用内存是否充足。程序会智能避免导致系统不稳定。

**Q: Windows上子进程创建失败**
A: 程序会自动切换到备用Go内存模式，功能完全相同，只是内存释放可能有1-5秒延迟。

**Q: CPU使用率波动较大**
A: 这是正常现象。程序采用工作/休息循环模式，会产生规律性波动。

**Q: 程序退出后内存没有立即释放**
A: 子进程模式会立即释放，备用模式依赖Go GC，可能有延迟。

**Q: 多核心系统CPU控制不准确**
A: 程序会自动检测CPU核心数并启动对应数量的工作线程，确保精确控制。

**Q: 为什么内存使用率会自动下降？**
A: 这是弹性伸缩功能。当检测到用户程序内存增加时，代理程序会自动释放内存让出资源。

**Q: 内存使用率没有达到设定阈值**
A: 程序会智能分析用户程序内存使用情况。如果用户程序占用较多内存，代理程序会自动让出资源确保系统稳定。

**Q: 内存使用率频繁波动**
A: 这是正常的弹性伸缩行为。程序会根据用户程序的内存变化自动调整，保持动态平衡。

**Q: 如何禁用弹性伸缩功能？**
A: 弹性伸缩是核心功能，无法禁用。这确保了用户程序始终有足够的资源，代理程序不会影响正常工作。

### 后台运行问题

**Q: nohup启动后如何查看程序状态？**
A: 使用 `ps aux | grep system-tune-agent` 查看进程，`tail -f system-tune-agent.log` 查看日志。

**Q: nohup静默启动后如何确认程序正在运行？**
A: 使用 `pgrep -f system-tune-agent` 查看PID，或 `ps aux | grep system-tune-agent | grep -v grep` 查看详细信息。

**Q: pkill无法停止程序怎么办？**
A: 
1. 先尝试：`pkill -TERM system-tune-agent`
2. 等待5秒后：`pkill -KILL system-tune-agent`
3. 或直接：`kill -9 $(pgrep -f system-tune-agent)`

**Q: 如何批量停止多个system-tune-agent进程？**
A: 使用 `pkill -f system-tune-agent` 或 `killall system-tune-agent`，会停止所有匹配的进程。

**Q: systemd服务启动失败**
A: 检查服务文件路径和权限，使用 `sudo systemctl status system-tune-agent` 查看详细错误信息。

**Q: 后台运行的程序突然停止**
A: 检查系统日志 `sudo journalctl -u system-tune-agent`，可能是内存不足或权限问题。建议使用systemd的自动重启功能。

**Q: 如何安全停止后台运行的程序？**
A: 
- **nohup方式**：
  - 推荐：`pkill system-tune-agent`
  - 备选：`killall system-tune-agent`
  - 强制：`pkill -9 system-tune-agent` (谨慎使用)
- **systemd方式**：`sudo systemctl stop system-tune-agent`
- **screen/tmux**：先连接会话再Ctrl+C

**Q: 程序在后台运行时日志文件过大**
A: 使用logrotate配置日志轮转，或定期清理日志文件。systemd服务的日志会自动管理。

**Q: 如何设置开机自启动？**
A: 使用systemd服务：`sudo systemctl enable system-tune-agent`，或在crontab中添加 `@reboot` 任务。

**Q: 后台运行时如何监控程序性能？**
A: 使用提供的监控脚本，或通过 `htop -p $(pgrep system-tune-agent)` 实时查看资源占用。

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 贡献

欢迎提交 Issue 和 Pull Request！


## 支持

- 问题反馈: [GitHub Issues](https://github.com/ArcGalaxy/system-tune-agent/issues)
- 功能建议: [GitHub Discussions](https://github.com/ArcGalaxy/system-tune-agent/discussions)
- 文档: [项目Wiki](https://github.com/ArcGalaxy/system-tune-agent/wiki)

## 致谢

感谢以下开源项目：
- [gopsutil](https://github.com/shirou/gopsutil) - 系统信息获取库
- [Go](https://golang.org/) - 编程语言

---

<div align="center">

**如果这个项目对你有帮助，请给个 Star！**

System Tune Agent - 精确的系统资源调节工具

</div>