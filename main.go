package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	defaultCPUThreshold    = 75
	defaultMemoryThreshold = 75
	monitorInterval        = 2 * time.Second
)

var (
	globalLogWriter io.Writer = os.Stdout
	globalLogFile   *os.File
)

// 全局日志输出函数
func logf(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05.000")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(globalLogWriter, "[%s] %s", timestamp, message)
}

// 初始化全局日志
func initGlobalLogging() {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("获取当前目录失败: %v", err)
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("system-tune-agent_%s.log", timestamp)
	logFilePath := filepath.Join(currentDir, logFileName)

	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Printf("创建日志文件失败: %v", err)
		return
	}

	globalLogFile = logFile
	globalLogWriter = io.MultiWriter(os.Stdout, logFile)
	
	fmt.Fprintf(globalLogWriter, "📝 日志文件创建: %s\n", logFilePath)
	fmt.Fprintf(globalLogWriter, "🕐 启动时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(globalLogWriter, "==========================================\n")
}

// MemoryWorker 表示一个内存消费子进程
type MemoryWorker struct {
	ID        int
	SizeMB    int
	Process   *os.Process
	StartTime time.Time
	IsRunning bool
}

type SystemTuneAgent struct {
	cpuThreshold    float64
	memoryThreshold float64

	// CPU 控制
	cpuIntensity int
	cpuWorkers   []context.CancelFunc
	cpuMutex     sync.RWMutex

	// 内存控制 - 基于子进程
	memoryWorkers       []*MemoryWorker
	memoryMutex         sync.RWMutex
	shouldConsumeMemory bool

	// 备用内存控制 - Go内存块 (当子进程失败时使用)
	memoryBlocks    [][]byte
	fallbackMode    bool
	fallbackMutex   sync.RWMutex

	// 状态跟踪
	lastCPUUsage    float64
	lastMemoryUsage float64
	
	// 智能内存管理
	baselineMemoryUsage    float64  // 启动时的基准内存使用率
	targetMemoryUsage      float64  // 目标内存使用率 (设定的阈值)
	userMemoryUsage        float64  // 用户程序的内存使用率
	agentMemoryUsage       float64  // 本程序的内存使用率
	lastAdjustmentTime     time.Time // 上次调整时间
	memoryAdjustmentCooldown time.Duration // 调整冷却时间
	
	// 动态平衡控制
	baseCPUUsage    float64
	baseMemoryUsage float64
	selfCPUUsage    float64
	selfMemoryUsage float64

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

func NewSystemTuneAgent(cpuThreshold, memoryThreshold float64) *SystemTuneAgent {
	ctx, cancel := context.WithCancel(context.Background())

	// 获取基准内存使用率
	memInfo, _ := mem.VirtualMemory()
	baselineUsage := 0.0
	if memInfo != nil {
		baselineUsage = memInfo.UsedPercent
	}

	return &SystemTuneAgent{
		cpuThreshold:             cpuThreshold,
		memoryThreshold:          memoryThreshold,
		cpuWorkers:               make([]context.CancelFunc, 0),
		memoryWorkers:            make([]*MemoryWorker, 0),
		memoryBlocks:             make([][]byte, 0),
		fallbackMode:             false,
		baselineMemoryUsage:      baselineUsage,
		targetMemoryUsage:        memoryThreshold,
		userMemoryUsage:          baselineUsage,
		agentMemoryUsage:         0.0,
		memoryAdjustmentCooldown: 5 * time.Second, // 5秒冷却时间
		ctx:                      ctx,
		cancel:                   cancel,
	}
}

// 获取当前可执行文件路径
func getCurrentExecutablePath() (string, error) {
	// 优先使用 os.Executable() 获取绝对路径
	execPath, err := os.Executable()
	if err != nil {
		logf("⚠️ os.Executable() 失败: %v, 尝试使用 os.Args[0]\n", err)
		execPath = os.Args[0]
	}
	
	// 确保路径是绝对路径
	if !filepath.IsAbs(execPath) {
		absPath, err := filepath.Abs(execPath)
		if err != nil {
			return "", fmt.Errorf("无法获取绝对路径: %v", err)
		}
		execPath = absPath
	}
	
	// 检查文件是否存在
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		return "", fmt.Errorf("可执行文件不存在: %s", execPath)
	}
	
	logf("🔍 使用可执行文件路径: %s\n", execPath)
	return execPath, nil
}

// 创建内存消费子进程
func (s *SystemTuneAgent) createMemoryWorker(sizeMB int) (*MemoryWorker, error) {
	workerID := len(s.memoryWorkers) + 1
	
	// 获取当前可执行文件路径
	execPath, err := getCurrentExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("获取可执行文件路径失败: %v", err)
	}
	
	// 创建子进程命令
	workerName := fmt.Sprintf("memory%d", workerID)
	cmd := exec.Command(execPath, "--memory-worker", fmt.Sprintf("%d", sizeMB))
	cmd.Env = append(os.Environ(), 
		fmt.Sprintf("WORKER_ID=%d", workerID),
		fmt.Sprintf("WORKER_NAME=%s", workerName))
	
	// Windows特定设置
	if runtime.GOOS == "windows" {
		// 设置工作目录为可执行文件所在目录
		cmd.Dir = filepath.Dir(execPath)
	}
	
	// 重定向输出到父进程
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// 启动子进程
	logf("🔧 启动子进程: %s --memory-worker %d (工作目录: %s)\n", execPath, sizeMB, cmd.Dir)
	err = cmd.Start()
	if err != nil {
		// Windows特定的错误处理
		if runtime.GOOS == "windows" {
			logf("⚠️ Windows子进程启动失败，可能的原因:\n")
			logf("   1. 杀毒软件拦截\n")
			logf("   2. 权限不足 (尝试以管理员身份运行)\n")
			logf("   3. 路径包含特殊字符\n")
		}
		return nil, fmt.Errorf("启动内存工作进程失败 (路径: %s): %v", execPath, err)
	}
	
	worker := &MemoryWorker{
		ID:        workerID,
		SizeMB:    sizeMB,
		Process:   cmd.Process,
		StartTime: time.Now(),
		IsRunning: true,
	}
	
	// 启动进程监控
	go s.monitorWorkerProcess(worker, cmd)
	
	logf("🚀 启动内存工作进程 %s (PID: %d, 大小: %dMB)\n", workerName, cmd.Process.Pid, sizeMB)
	return worker, nil
}

// 监控工作进程状态
func (s *SystemTuneAgent) monitorWorkerProcess(worker *MemoryWorker, cmd *exec.Cmd) {
	// 等待进程结束
	err := cmd.Wait()
	
	s.memoryMutex.Lock()
	worker.IsRunning = false
	s.memoryMutex.Unlock()
	
	if err != nil {
		logf("⚠️ 内存工作进程 memory%d (PID: %d) 异常退出: %v\n", worker.ID, worker.Process.Pid, err)
	} else {
		logf("✅ 内存工作进程 memory%d (PID: %d) 正常退出\n", worker.ID, worker.Process.Pid)
	}
}

// 销毁内存工作进程
func (s *SystemTuneAgent) destroyMemoryWorker(worker *MemoryWorker) error {
	if !worker.IsRunning {
		logf("⚠️ 工作进程 memory%d 已经停止\n", worker.ID)
		return nil
	}
	
	logf("🔪 发送终止信号给进程 memory%d (PID: %d)\n", worker.ID, worker.Process.Pid)
	
	// 首先尝试优雅终止 (SIGTERM)
	if runtime.GOOS != "windows" {
		err := worker.Process.Signal(syscall.SIGTERM)
		if err != nil {
			logf("⚠️ 发送 SIGTERM 失败: %v，尝试强制终止\n", err)
		} else {
			// 等待2秒看进程是否优雅退出
			done := make(chan bool, 1)
			go func() {
				worker.Process.Wait()
				done <- true
			}()
			
			select {
			case <-done:
				worker.IsRunning = false
				logf("✅ 进程 memory%d 优雅退出\n", worker.ID)
				return nil
			case <-time.After(2 * time.Second):
				logf("⏰ 进程 memory%d 未在2秒内退出，强制终止\n", worker.ID)
			}
		}
	}
	
	// 强制终止子进程 (SIGKILL)
	err := worker.Process.Kill()
	if err != nil {
		return fmt.Errorf("强制终止进程失败: %v", err)
	}
	
	// 等待进程结束
	_, err = worker.Process.Wait()
	if err != nil {
		logf("⚠️ 等待进程结束时出错: %v\n", err)
	}
	
	worker.IsRunning = false
	logf("💀 强制销毁内存工作进程 memory%d (PID: %d, 运行时间: %.1f秒)\n", 
		worker.ID, worker.Process.Pid, time.Since(worker.StartTime).Seconds())
	
	return nil
}

// 内存工作进程的主函数
func runMemoryWorker(sizeMB int) {
	workerName := os.Getenv("WORKER_NAME")
	if workerName == "" {
		workerID := os.Getenv("WORKER_ID")
		if workerID == "" {
			workerName = "memory_unknown"
		} else {
			workerName = fmt.Sprintf("memory%s", workerID)
		}
	}
	
	logf("💾 内存工作进程 %s 开始分配 %dMB 内存\n", workerName, sizeMB)
	
	// 分配内存
	memoryBlocks := make([][]byte, 0)
	blockSize := 10 * 1024 * 1024 // 10MB per block
	totalBlocks := sizeMB / 10
	
	// 处理不足10MB的情况
	if sizeMB%10 != 0 {
		totalBlocks++
	}
	
	allocatedMB := 0
	
	for i := 0; i < totalBlocks; i++ {
		currentBlockSize := blockSize
		if i == totalBlocks-1 && sizeMB%10 != 0 {
			// 最后一个块可能小于10MB
			currentBlockSize = (sizeMB % 10) * 1024 * 1024
		}
		
		block := make([]byte, currentBlockSize)
		
		// 填充数据防止被优化，使用更高效的方式
		pattern := byte(i % 256)
		for j := 0; j < len(block); j += 4096 {
			if j+4096 <= len(block) {
				for k := 0; k < 4096; k++ {
					block[j+k] = pattern
				}
			} else {
				for k := j; k < len(block); k++ {
					block[k] = pattern
				}
			}
		}
		
		memoryBlocks = append(memoryBlocks, block)
		allocatedMB += currentBlockSize / (1024 * 1024)
		
		if i%5 == 0 || i == totalBlocks-1 {
			logf("💾 工作进程 %s 已分配 %dMB / %dMB (%.1f%%)\n", 
				workerName, allocatedMB, sizeMB, float64(allocatedMB)/float64(sizeMB)*100)
		}
		
		// 避免过快分配导致系统卡顿
		if i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	
	logf("✅ 内存工作进程 %s 分配完成，总计 %dMB (%d个块)\n", workerName, allocatedMB, len(memoryBlocks))
	
	// 定期访问内存防止被交换出去
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// 轻量级内存访问
				for i, block := range memoryBlocks {
					if len(block) > 0 {
						_ = block[0] // 访问第一个字节
						if i%100 == 0 {
							time.Sleep(time.Millisecond) // 避免过度占用CPU
						}
					}
				}
				logf("🔄 工作进程 %s 内存保活检查完成\n", workerName)
			}
		}
	}()
	
	// 保持运行直到被终止
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	logf("🛑 内存工作进程 %s 收到终止信号，开始清理...\n", workerName)
	
	// 清理内存块
	for i := range memoryBlocks {
		memoryBlocks[i] = nil
	}
	memoryBlocks = nil
	
	// 强制GC
	runtime.GC()
	
	logf("🗑️ 内存工作进程 %s 清理完成，退出\n", workerName)
}

// 启动 CPU 工作线程
func (s *SystemTuneAgent) startCPUWorkers(intensity int) {
	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	// 清理现有工作线程
	for _, cancel := range s.cpuWorkers {
		cancel()
	}
	s.cpuWorkers = s.cpuWorkers[:0]

	// 启动新的工作线程
	numWorkers := runtime.NumCPU()
	for i := 0; i < numWorkers; i++ {
		ctx, cancel := context.WithCancel(s.ctx)
		s.cpuWorkers = append(s.cpuWorkers, cancel)

		go func(workerID int) {
			logf("🔥 启动 CPU 工作线程 #%d (强度: %d%%)\n", workerID, intensity)
			
			for {
				select {
				case <-ctx.Done():
					logf("🛑 停止 CPU 工作线程 #%d\n", workerID)
					return
				default:
					// 根据强度调整工作负载
					workDuration := time.Duration(intensity) * time.Microsecond * 10
					sleepDuration := time.Duration(100-intensity) * time.Microsecond * 10
					
					// 执行计算密集型任务
					start := time.Now()
					for time.Since(start) < workDuration {
						_ = math.Sqrt(float64(time.Now().UnixNano()))
					}
					
					// 休息
					if sleepDuration > 0 {
						time.Sleep(sleepDuration)
					}
				}
			}
		}(i)
	}

	s.cpuIntensity = intensity
}

// 停止 CPU 工作线程
func (s *SystemTuneAgent) stopCPUWorkers() {
	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	for i, cancel := range s.cpuWorkers {
		cancel()
		logf("🛑 停止 CPU 工作线程 #%d\n", i)
	}
	s.cpuWorkers = s.cpuWorkers[:0]
	s.cpuIntensity = 0
}

// 智能内存管理 - 分析用户内存使用情况
func (s *SystemTuneAgent) analyzeMemoryUsage(currentUsage float64) {
	// 计算当前代理程序占用的内存
	currentAgentMemory := s.getCurrentWorkerMemory()
	agentMemoryPercent := float64(currentAgentMemory) / (float64(s.getTotalSystemMemoryMB()) / 100.0)
	
	// 估算用户程序的内存使用率
	estimatedUserUsage := currentUsage - agentMemoryPercent
	if estimatedUserUsage < 0 {
		estimatedUserUsage = 0
	}
	
	s.userMemoryUsage = estimatedUserUsage
	s.agentMemoryUsage = agentMemoryPercent
	
	logf("🧠 内存分析: 总计 %.1f%% | 用户程序 ~%.1f%% | 代理程序 ~%.1f%% (%dMB)\n", 
		currentUsage, estimatedUserUsage, agentMemoryPercent, currentAgentMemory)
}

// 获取系统总内存MB
func (s *SystemTuneAgent) getTotalSystemMemoryMB() int {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 8192 // 默认8GB
	}
	return int(memInfo.Total / 1024 / 1024)
}

// 动态内存调整 - 基于实时总内存使用率
func (s *SystemTuneAgent) smartAdjustMemoryConsumption(currentUsage float64) {
	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	// 检查冷却时间 (缩短为1秒，更快响应)
	cooldown := 1 * time.Second
	if time.Since(s.lastAdjustmentTime) < cooldown {
		return
	}

	// 分析内存使用情况
	s.analyzeMemoryUsage(currentUsage)
	
	// 计算当前工作进程占用的内存
	currentWorkerMemoryMB := s.getCurrentWorkerMemory()
	totalMemoryMB := s.getTotalSystemMemoryMB()
	
	// 动态响应逻辑 - 基于实时总内存使用率
	var targetAgentUsage float64
	var reason string
	var shouldAdjust bool = false
	
	if currentUsage > s.targetMemoryUsage + 2 {
		// 总内存超过阈值+2%，立即回收内存
		// 计算需要回收多少内存才能降到目标-1%
		targetTotalUsage := s.targetMemoryUsage - 1
		currentAgentUsage := s.agentMemoryUsage
		
		// 需要减少的总内存 = 当前使用 - 目标使用
		needReduceUsage := currentUsage - targetTotalUsage
		
		// 代理程序最多能减少的内存就是当前占用的内存
		maxReduceUsage := currentAgentUsage
		if needReduceUsage > maxReduceUsage {
			needReduceUsage = maxReduceUsage
		}
		
		targetAgentUsage = currentAgentUsage - needReduceUsage
		if targetAgentUsage < 0 {
			targetAgentUsage = 0
		}
		
		reason = fmt.Sprintf("总内存超标 %.1f%% > %.1f%%，立即回收", currentUsage, s.targetMemoryUsage+2)
		shouldAdjust = true
		
	} else if currentUsage < s.targetMemoryUsage - 2 {
		// 总内存低于阈值-2%，可以增加内存到目标
		targetAgentUsage = s.targetMemoryUsage - s.userMemoryUsage - 1 // 留1%缓冲给用户
		if targetAgentUsage < 0 {
			targetAgentUsage = 0
		}
		
		reason = fmt.Sprintf("总内存充足 %.1f%% < %.1f%%，增加到目标", currentUsage, s.targetMemoryUsage-2)
		shouldAdjust = true
		
	} else {
		// 在目标范围内，不调整
		logf("✅ 内存使用率在目标范围内: %.1f%% (目标: %.1f%% ±2%%)\n", currentUsage, s.targetMemoryUsage)
		return
	}
	
	if !shouldAdjust {
		return
	}
	
	// 计算需要的内存变化
	targetAgentMemoryMB := int(float64(totalMemoryMB) * targetAgentUsage / 100.0)
	memoryChangeMB := targetAgentMemoryMB - currentWorkerMemoryMB
	
	logf("🎯 动态内存调整: %s\n", reason)
	logf("📊 目标代理内存: %.1f%% (%dMB) | 当前: %.1f%% (%dMB) | 变化: %+dMB\n", 
		targetAgentUsage, targetAgentMemoryMB, s.agentMemoryUsage, currentWorkerMemoryMB, memoryChangeMB)

	// 避免过小的调整
	if abs(memoryChangeMB) < 50 {
		logf("⏸️ 内存变化太小 (%dMB)，跳过调整\n", memoryChangeMB)
		return
	}

	// 执行内存调整
	if memoryChangeMB > 0 {
		logf("🔼 增加代理内存消费 %dMB\n", memoryChangeMB)
		s.addMemoryWorkers(memoryChangeMB)
	} else if memoryChangeMB < 0 {
		logf("🔽 减少代理内存消费 %dMB (紧急回收)\n", -memoryChangeMB)
		s.removeMemoryWorkers(-memoryChangeMB)
	}
	
	s.lastAdjustmentTime = time.Now()
}

// 调整内存消费 (保留原有接口用于兼容)
func (s *SystemTuneAgent) adjustMemoryConsumption(targetUsage float64) {
	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	// 获取当前系统内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logf("❌ 获取内存信息失败: %v\n", err)
		return
	}

	currentUsage := memInfo.UsedPercent
	totalMemoryMB := int(memInfo.Total / 1024 / 1024)
	
	// 计算当前工作进程占用的内存
	currentWorkerMemoryMB := s.getCurrentWorkerMemory()
	
	// 计算需要的内存变化
	usageDiff := targetUsage - currentUsage
	memoryChangeMB := int(float64(totalMemoryMB) * usageDiff / 100.0)
	
	logf("📊 内存调整: 当前 %.1f%% -> 目标 %.1f%% (变化: %+dMB, 工作进程: %dMB)\n", 
		currentUsage, targetUsage, memoryChangeMB, currentWorkerMemoryMB)

	// 避免过小的调整
	if abs(memoryChangeMB) < 50 {
		return
	}

	if memoryChangeMB > 0 {
		// 需要增加内存消费
		s.addMemoryWorkers(memoryChangeMB)
	} else if memoryChangeMB < 0 {
		// 需要减少内存消费
		s.removeMemoryWorkers(-memoryChangeMB)
	}
}

// 获取当前工作进程占用的内存总量
func (s *SystemTuneAgent) getCurrentWorkerMemory() int {
	totalMB := 0
	
	// 子进程内存
	for _, worker := range s.memoryWorkers {
		if worker.IsRunning {
			totalMB += worker.SizeMB
		}
	}
	
	// 备用内存块
	if s.fallbackMode {
		s.fallbackMutex.RLock()
		totalMB += len(s.memoryBlocks) * 10 // 每个块10MB
		s.fallbackMutex.RUnlock()
	}
	
	return totalMB
}

// 绝对值函数
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// 备用内存分配 (Go内存块)
func (s *SystemTuneAgent) allocateFallbackMemory(sizeMB int) {
	s.fallbackMutex.Lock()
	defer s.fallbackMutex.Unlock()
	
	logf("🔄 使用备用内存分配模式: %dMB\n", sizeMB)
	
	blockSize := 10 * 1024 * 1024 // 10MB per block
	numBlocks := sizeMB / 10
	
	for i := 0; i < numBlocks; i++ {
		block := make([]byte, blockSize)
		// 填充数据防止被优化
		for j := 0; j < len(block); j += 4096 {
			block[j] = byte(i % 256)
		}
		s.memoryBlocks = append(s.memoryBlocks, block)
	}
	
	logf("✅ 备用内存分配完成: %d个块, 总计%dMB\n", len(s.memoryBlocks), len(s.memoryBlocks)*10)
}

// 释放备用内存
func (s *SystemTuneAgent) releaseFallbackMemory(targetMB int) {
	s.fallbackMutex.Lock()
	defer s.fallbackMutex.Unlock()
	
	blocksToRemove := targetMB / 10
	if blocksToRemove > len(s.memoryBlocks) {
		blocksToRemove = len(s.memoryBlocks)
	}
	
	// 释放内存块
	for i := 0; i < blocksToRemove; i++ {
		s.memoryBlocks[len(s.memoryBlocks)-1-i] = nil
	}
	
	s.memoryBlocks = s.memoryBlocks[:len(s.memoryBlocks)-blocksToRemove]
	
	// 强制GC
	runtime.GC()
	
	logf("🗑️ 备用内存释放完成: 释放%d个块, 剩余%d个块\n", blocksToRemove, len(s.memoryBlocks))
}

// 添加内存工作进程
func (s *SystemTuneAgent) addMemoryWorkers(totalMB int) {
	const maxWorkerSize = 500 // 每个工作进程最大500MB
	
	// 检查是否已经进入备用模式
	if s.fallbackMode {
		s.allocateFallbackMemory(totalMB)
		return
	}
	
	for totalMB > 0 {
		workerSize := totalMB
		if workerSize > maxWorkerSize {
			workerSize = maxWorkerSize
		}
		
		worker, err := s.createMemoryWorker(workerSize)
		if err != nil {
			logf("❌ 创建内存工作进程失败: %v\n", err)
			logf("🔄 切换到备用内存管理模式\n")
			s.fallbackMode = true
			s.allocateFallbackMemory(totalMB)
			break
		}
		
		s.memoryWorkers = append(s.memoryWorkers, worker)
		totalMB -= workerSize
		
		// 给进程一些时间启动
		time.Sleep(100 * time.Millisecond)
	}
}

// 移除内存工作进程
func (s *SystemTuneAgent) removeMemoryWorkers(targetMB int) {
	// 如果在备用模式，使用Go内存管理
	if s.fallbackMode {
		s.releaseFallbackMemory(targetMB)
		return
	}
	
	removedMB := 0
	workersToRemove := make([]*MemoryWorker, 0)
	
	// 从后往前选择要移除的工作进程
	for i := len(s.memoryWorkers) - 1; i >= 0 && removedMB < targetMB; i-- {
		worker := s.memoryWorkers[i]
		if worker.IsRunning {
			workersToRemove = append(workersToRemove, worker)
			removedMB += worker.SizeMB
		}
	}
	
	logf("🎯 准备移除 %d 个内存工作进程，释放 %dMB 内存 (目标: %dMB)\n", 
		len(workersToRemove), removedMB, targetMB)
	
	// 实际销毁进程
	actualRemovedMB := 0
	for _, worker := range workersToRemove {
		logf("🗑️ 正在销毁内存工作进程 memory%d (PID: %d, %dMB)...\n", 
			worker.ID, worker.Process.Pid, worker.SizeMB)
		
		err := s.destroyMemoryWorker(worker)
		if err != nil {
			logf("❌ 销毁内存工作进程 memory%d 失败: %v\n", worker.ID, err)
			continue
		}
		
		actualRemovedMB += worker.SizeMB
		
		// 从列表中移除
		for i, w := range s.memoryWorkers {
			if w.ID == worker.ID {
				s.memoryWorkers = append(s.memoryWorkers[:i], s.memoryWorkers[i+1:]...)
				break
			}
		}
	}
	
	logf("✅ 成功释放 %dMB 内存，剩余 %d 个工作进程\n", actualRemovedMB, len(s.memoryWorkers))
}

// 清理所有内存工作进程
func (s *SystemTuneAgent) cleanupMemoryWorkers() {
	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	// 清理子进程
	for _, worker := range s.memoryWorkers {
		if worker.IsRunning {
			s.destroyMemoryWorker(worker)
		}
	}
	s.memoryWorkers = s.memoryWorkers[:0]
	
	// 清理备用内存块
	if s.fallbackMode {
		s.fallbackMutex.Lock()
		for i := range s.memoryBlocks {
			s.memoryBlocks[i] = nil
		}
		s.memoryBlocks = s.memoryBlocks[:0]
		s.fallbackMutex.Unlock()
		runtime.GC()
		logf("🧹 清理备用内存块完成\n")
	}
}

// 获取系统资源使用情况
func (s *SystemTuneAgent) getSystemUsage() (float64, float64, error) {
	// CPU 使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, 0, fmt.Errorf("获取 CPU 使用率失败: %v", err)
	}

	// 内存使用率
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, fmt.Errorf("获取内存使用率失败: %v", err)
	}

	return cpuPercent[0], memInfo.UsedPercent, nil
}

// 动态平衡逻辑
func (s *SystemTuneAgent) performDynamicBalance() {
	cpuUsage, memoryUsage, err := s.getSystemUsage()
	if err != nil {
		logf("❌ 获取系统使用率失败: %v\n", err)
		return
	}

	// 更新历史数据
	s.lastCPUUsage = cpuUsage
	s.lastMemoryUsage = memoryUsage

	// 清理已退出的工作进程
	s.cleanupDeadWorkers()
	
	// 获取当前活跃的工作进程数量和内存
	activeWorkers := s.getActiveWorkerCount()
	workerMemoryMB := s.getCurrentWorkerMemory()

	modeStr := "子进程"
	if s.fallbackMode {
		modeStr = "备用Go内存"
	}
	
	// 分析当前内存使用情况
	s.analyzeMemoryUsage(memoryUsage)
	
	logf("📊 系统状态: CPU %.1f%% (目标: %.1f%%) | 内存 %.1f%% (目标: %.1f%%) | CPU强度: %d%% | %s: %d个/%dMB\n", 
		cpuUsage, s.cpuThreshold, memoryUsage, s.memoryThreshold, s.cpuIntensity, modeStr, activeWorkers, workerMemoryMB)
	
	logf("🧠 内存详情: 用户 ~%.1f%% | 代理 ~%.1f%% | 基准 %.1f%% | 距离上次调整 %.1fs\n", 
		s.userMemoryUsage, s.agentMemoryUsage, s.baselineMemoryUsage, time.Since(s.lastAdjustmentTime).Seconds())

	// CPU 动态平衡
	if cpuUsage < s.cpuThreshold-10 {
		// CPU 使用率过低，启动 CPU 负载
		intensity := int((s.cpuThreshold - cpuUsage) * 1.5)
		if intensity > 70 {
			intensity = 70 // 限制最大强度避免系统过载
		}
		if intensity > 10 && s.cpuIntensity != intensity {
			s.startCPUWorkers(intensity)
		}
	} else if cpuUsage > s.cpuThreshold+5 {
		// CPU 使用率过高，停止 CPU 负载
		if s.cpuIntensity > 0 {
			s.stopCPUWorkers()
		}
	}

	// 智能内存动态平衡
	s.smartAdjustMemoryConsumption(memoryUsage)
}

// 清理已退出的工作进程
func (s *SystemTuneAgent) cleanupDeadWorkers() {
	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()
	
	activeWorkers := make([]*MemoryWorker, 0)
	for _, worker := range s.memoryWorkers {
		if worker.IsRunning {
			activeWorkers = append(activeWorkers, worker)
		}
	}
	
	if len(activeWorkers) != len(s.memoryWorkers) {
		removedCount := len(s.memoryWorkers) - len(activeWorkers)
		s.memoryWorkers = activeWorkers
		logf("🧹 清理了 %d 个已退出的工作进程\n", removedCount)
	}
}

// 获取活跃工作进程数量
func (s *SystemTuneAgent) getActiveWorkerCount() int {
	s.memoryMutex.RLock()
	defer s.memoryMutex.RUnlock()
	
	count := 0
	for _, worker := range s.memoryWorkers {
		if worker.IsRunning {
			count++
		}
	}
	return count
}

// 启动代理
func (s *SystemTuneAgent) Start() error {
	logf("🚀 System Tune Agent 启动 (子进程模式)\n")
	logf("📋 配置: CPU阈值=%.1f%%, 内存阈值=%.1f%%\n", s.cpuThreshold, s.memoryThreshold)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动监控循环
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logf("🛑 收到停止信号\n")
			return nil

		case <-sigChan:
			logf("🛑 收到系统信号，正在清理...\n")
			s.Stop()
			return nil

		case <-ticker.C:
			s.performDynamicBalance()
		}
	}
}

// 停止代理
func (s *SystemTuneAgent) Stop() {
	logf("🛑 正在停止 System Tune Agent...\n")
	
	// 停止 CPU 工作线程
	s.stopCPUWorkers()
	
	// 清理内存工作进程
	s.cleanupMemoryWorkers()
	
	// 取消上下文
	s.cancel()
	
	// 关闭日志文件
	if globalLogFile != nil {
		globalLogFile.Close()
	}
	
	logf("✅ System Tune Agent 已停止\n")
}

func main() {
	var (
		cpuThreshold    = flag.Float64("cpu", defaultCPUThreshold, "CPU 使用率阈值 (1-100)")
		memoryThreshold = flag.Float64("memory", defaultMemoryThreshold, "内存使用率阈值 (1-100)")
		memoryWorker    = flag.String("memory-worker", "", "内存工作进程模式，指定分配的内存大小(MB)")
		showHelp        = flag.Bool("help", false, "显示帮助信息")
	)

	flag.Float64Var(cpuThreshold, "c", defaultCPUThreshold, "CPU 使用率阈值 (简写)")
	flag.Float64Var(memoryThreshold, "m", defaultMemoryThreshold, "内存使用率阈值 (简写)")
	flag.BoolVar(showHelp, "h", false, "显示帮助信息 (简写)")

	flag.Parse()

	// 检查是否是内存工作进程模式
	if *memoryWorker != "" {
		sizeMB, err := strconv.Atoi(*memoryWorker)
		if err != nil {
			log.Fatalf("无效的内存大小: %v", err)
		}
		runMemoryWorker(sizeMB)
		return
	}

	if *showHelp {
		fmt.Println("System Tune Agent - 系统资源调节工具 (Go 版本 - 子进程模式)")
		fmt.Println("用法: system-tune-agent [选项]")
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  ./system-tune-agent -c 80 -m 70")
		fmt.Println("  ./system-tune-agent --cpu 90 --memory 85")
		return
	}

	// 验证参数
	if *cpuThreshold < 1 || *cpuThreshold > 100 || *memoryThreshold < 1 || *memoryThreshold > 100 {
		fmt.Fprintf(os.Stderr, "错误: 阈值必须在 1-100 之间\n")
		os.Exit(1)
	}

	// 初始化日志记录
	initGlobalLogging()
	
	logf("🔧 使用子进程内存管理模式\n")

	agent := NewSystemTuneAgent(*cpuThreshold, *memoryThreshold)
	if err := agent.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}