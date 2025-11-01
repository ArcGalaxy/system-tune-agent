package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	defaultCPUThreshold    = 10
	defaultMemoryThreshold = 60
	maxCPUThreshold        = 30
	monitorInterval        = 2 * time.Second
)



// 简单日志输出函数
func logf(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s", timestamp, message)
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

	// CPU 控制 - 单线程
	cpuIntensity int
	cpuWorker    context.CancelFunc
	cpuMutex     sync.RWMutex

	// 内存控制 - 基于子进程
	memoryWorkers       []*MemoryWorker
	memoryMutex         sync.RWMutex

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
		memoryWorkers:            make([]*MemoryWorker, 0),
		memoryBlocks:             make([][]byte, 0),
		fallbackMode:             false,
		baselineMemoryUsage:      baselineUsage,
		targetMemoryUsage:        memoryThreshold,
		userMemoryUsage:          baselineUsage,
		agentMemoryUsage:         0.0,
		ctx:                      ctx,
		cancel:                   cancel,
	}
}

// 启动单线程CPU控制
func (s *SystemTuneAgent) startCPUWorker(intensity int) {
	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	// 清理现有工作线程
	if s.cpuWorker != nil {
		s.cpuWorker()
		s.cpuWorker = nil
	}

	// 计算工作/休息比例
	cycleMs := 100  // 100ms周期
	workMs := (intensity * cycleMs) / 100
	sleepMs := cycleMs - workMs
	
	// 确保最小工作时间
	if workMs < 1 {
		workMs = 1
		sleepMs = cycleMs - 1
	}
	
	ctx, cancel := context.WithCancel(s.ctx)
	s.cpuWorker = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 工作阶段：燃烧CPU
				start := time.Now()
				workDuration := time.Duration(workMs) * time.Millisecond
				for time.Since(start) < workDuration {
					// 紧凑空循环，燃烧CPU
					for j := 0; j < 10000; j++ {
						_ = j * j // 简单计算防止被优化
					}
				}
				
				// 休息阶段：睡眠
				if sleepMs > 0 {
					time.Sleep(time.Duration(sleepMs) * time.Millisecond)
				}
			}
		}
	}()

	s.cpuIntensity = intensity
}

// 停止 CPU 工作线程
func (s *SystemTuneAgent) stopCPUWorker() {
	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	if s.cpuWorker != nil {
		s.cpuWorker()
		s.cpuWorker = nil
	}
	s.cpuIntensity = 0
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

// 获取当前可执行文件路径
func getCurrentExecutablePath() (string, error) {
	// 优先使用 os.Executable() 获取绝对路径
	execPath, err := os.Executable()
	if err != nil {
		logf("WARNING: os.Executable() 失败: %v, 尝试使用 os.Args[0]\n", err)
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
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("启动内存工作进程失败: %v", err)
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
	
	return worker, nil
}

// 监控工作进程状态
func (s *SystemTuneAgent) monitorWorkerProcess(worker *MemoryWorker, cmd *exec.Cmd) {
	cmd.Wait()
	s.memoryMutex.Lock()
	worker.IsRunning = false
	s.memoryMutex.Unlock()
}

// 销毁内存工作进程
func (s *SystemTuneAgent) destroyMemoryWorker(worker *MemoryWorker) error {
	if !worker.IsRunning {
		return nil
	}
	
	// 首先尝试优雅终止 (SIGTERM)
	if runtime.GOOS != "windows" {
		err := worker.Process.Signal(syscall.SIGTERM)
		if err == nil {
			// 等待2秒看进程是否优雅退出
			done := make(chan bool, 1)
			go func() {
				worker.Process.Wait()
				done <- true
			}()
			
			select {
			case <-done:
				worker.IsRunning = false
				return nil
			case <-time.After(2 * time.Second):
				// 继续强制终止
			}
		}
	}
	
	// 强制终止子进程 (SIGKILL)
	err := worker.Process.Kill()
	if err != nil {
		return fmt.Errorf("强制终止进程失败: %v", err)
	}
	
	worker.Process.Wait()
	worker.IsRunning = false
	return nil
}

// 内存工作进程的主函数
func runMemoryWorker(sizeMB int) {
	// 分配内存
	memoryBlocks := make([][]byte, 0)
	blockSize := 10 * 1024 * 1024 // 10MB per block
	totalBlocks := sizeMB / 10
	
	if sizeMB%10 != 0 {
		totalBlocks++
	}
	
	for i := 0; i < totalBlocks; i++ {
		currentBlockSize := blockSize
		if i == totalBlocks-1 && sizeMB%10 != 0 {
			currentBlockSize = (sizeMB % 10) * 1024 * 1024
		}
		
		block := make([]byte, currentBlockSize)
		
		// 填充数据防止被优化
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
		
		if i%10 == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	
	// 定期访问内存防止被交换出去
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				for i, block := range memoryBlocks {
					if len(block) > 0 {
						_ = block[0]
						if i%100 == 0 {
							time.Sleep(time.Millisecond)
						}
					}
				}
			}
		}
	}()
	
	// 保持运行直到被终止
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	// 清理内存块
	for i := range memoryBlocks {
		memoryBlocks[i] = nil
	}
	runtime.GC()
}

// 分析用户内存使用情况
func (s *SystemTuneAgent) analyzeMemoryUsage(currentUsage float64) {
	currentAgentMemory := s.getCurrentWorkerMemory()
	agentMemoryPercent := float64(currentAgentMemory) / (float64(s.getTotalSystemMemoryMB()) / 100.0)
	
	estimatedUserUsage := currentUsage - agentMemoryPercent
	if estimatedUserUsage < 0 {
		estimatedUserUsage = 0
	}
	
	s.userMemoryUsage = estimatedUserUsage
	s.agentMemoryUsage = agentMemoryPercent
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
	var shouldAdjust bool = false
	
	if currentUsage > s.targetMemoryUsage + 2 {
		// 总内存超过阈值+2%，立即回收内存
		targetTotalUsage := s.targetMemoryUsage - 1
		currentAgentUsage := s.agentMemoryUsage
		
		needReduceUsage := currentUsage - targetTotalUsage
		maxReduceUsage := currentAgentUsage
		if needReduceUsage > maxReduceUsage {
			needReduceUsage = maxReduceUsage
		}
		
		targetAgentUsage = currentAgentUsage - needReduceUsage
		if targetAgentUsage < 0 {
			targetAgentUsage = 0
		}
		
		shouldAdjust = true
		
	} else if currentUsage < s.targetMemoryUsage - 2 {
		// 总内存低于阈值-2%，可以增加内存到目标
		targetAgentUsage = s.targetMemoryUsage - s.userMemoryUsage - 1
		if targetAgentUsage < 0 {
			targetAgentUsage = 0
		}
		
		shouldAdjust = true
		
	} else {
		return
	}
	
	if !shouldAdjust {
		return
	}
	
	// 计算需要的内存变化
	targetAgentMemoryMB := int(float64(totalMemoryMB) * targetAgentUsage / 100.0)
	memoryChangeMB := targetAgentMemoryMB - currentWorkerMemoryMB

	// 避免过小的调整
	if abs(memoryChangeMB) < 50 {
		return
	}

	// 执行内存调整
	if memoryChangeMB > 0 {
		s.addMemoryWorkers(memoryChangeMB)
	} else if memoryChangeMB < 0 {
		s.removeMemoryWorkers(-memoryChangeMB)
	}
	
	s.lastAdjustmentTime = time.Now()
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
	
	blockSize := 10 * 1024 * 1024 // 10MB per block
	numBlocks := sizeMB / 10
	
	for i := 0; i < numBlocks; i++ {
		block := make([]byte, blockSize)
		for j := 0; j < len(block); j += 4096 {
			block[j] = byte(i % 256)
		}
		s.memoryBlocks = append(s.memoryBlocks, block)
	}
}

// 释放备用内存
func (s *SystemTuneAgent) releaseFallbackMemory(targetMB int) {
	s.fallbackMutex.Lock()
	defer s.fallbackMutex.Unlock()
	
	blocksToRemove := targetMB / 10
	if blocksToRemove > len(s.memoryBlocks) {
		blocksToRemove = len(s.memoryBlocks)
	}
	
	for i := 0; i < blocksToRemove; i++ {
		s.memoryBlocks[len(s.memoryBlocks)-1-i] = nil
	}
	
	s.memoryBlocks = s.memoryBlocks[:len(s.memoryBlocks)-blocksToRemove]
	runtime.GC()
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
	
	// 实际销毁进程
	for _, worker := range workersToRemove {
		err := s.destroyMemoryWorker(worker)
		if err != nil {
			continue
		}
		
		// 从列表中移除
		for i, w := range s.memoryWorkers {
			if w.ID == worker.ID {
				s.memoryWorkers = append(s.memoryWorkers[:i], s.memoryWorkers[i+1:]...)
				break
			}
		}
	}
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
	}
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
		s.memoryWorkers = activeWorkers
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

// 动态平衡逻辑
func (s *SystemTuneAgent) performDynamicBalance() {
	cpuUsage, memoryUsage, err := s.getSystemUsage()
	if err != nil {
		logf("ERROR: 获取系统使用率失败: %v\n", err)
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
	
	s.analyzeMemoryUsage(memoryUsage)
	
	logf("STATUS: CPU %.1f%% (目标%.1f%%) | 内存 %.1f%% (目标%.1f%%) | CPU强度%d%% | %s:%d个/%dMB\n", 
		cpuUsage, s.cpuThreshold, memoryUsage, s.memoryThreshold, s.cpuIntensity, modeStr, activeWorkers, workerMemoryMB)

	// CPU 动态平衡
	if cpuUsage < s.cpuThreshold-5 {
		// CPU 使用率过低，启动 CPU 负载
		intensity := int((s.cpuThreshold - cpuUsage) * 1.5)
		if intensity > maxCPUThreshold {
			intensity = maxCPUThreshold // 限制最大强度
		}
		if intensity > 5 && s.cpuIntensity != intensity {
			s.startCPUWorker(intensity)
		}
	} else if cpuUsage > s.cpuThreshold+3 {
		// CPU 使用率过高，停止 CPU 负载
		if s.cpuIntensity > 0 {
			s.stopCPUWorker()
		}
	}

	// 智能内存动态平衡
	s.smartAdjustMemoryConsumption(memoryUsage)
}

// 启动代理
func (s *SystemTuneAgent) Start() error {
	logf("�  CPU阈值%.1f%% (最大%d%%) | 内存阈值%.1f%%\n", s.cpuThreshold, maxCPUThreshold, s.memoryThreshold)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动监控循环
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logf("INFO: 收到停止信号\n")
			return nil

		case <-sigChan:
			s.Stop()
			return nil

		case <-ticker.C:
			s.performDynamicBalance()
		}
	}
}

// 停止代理
func (s *SystemTuneAgent) Stop() {
	s.stopCPUWorker()
	s.cleanupMemoryWorkers()
	s.cancel()
}

func main() {
	var (
		cpuThreshold    = flag.Float64("cpu", defaultCPUThreshold, fmt.Sprintf("CPU 使用率阈值 (1-%d)", maxCPUThreshold))
		memoryThreshold = flag.Float64("memory", defaultMemoryThreshold, "内存使用率阈值 (1-100)")
		memoryWorker    = flag.String("memory-worker", "", "内存工作进程模式，指定分配的内存大小(MB)")
		showHelp        = flag.Bool("help", false, "显示帮助信息")
	)

	flag.Float64Var(cpuThreshold, "c", defaultCPUThreshold, fmt.Sprintf("CPU 使用率阈值 (简写, 1-%d)", maxCPUThreshold))
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
		fmt.Println("System Tune Agent - 系统资源调节工具 (简化单线程版本)")
		fmt.Println("用法: system-tune-agent [选项]")
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Printf("注意: CPU阈值限制在1-%d%%之间，默认%d%%\n", maxCPUThreshold, defaultCPUThreshold)
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  ./system-tune-agent -c 20 -m 70")
		fmt.Println("  ./system-tune-agent --cpu 15 --memory 85")
		return
	}

	// 验证参数
	if *cpuThreshold < 1 || *cpuThreshold > maxCPUThreshold {
		fmt.Fprintf(os.Stderr, "错误: CPU阈值必须在 1-%d 之间\n", maxCPUThreshold)
		os.Exit(1)
	}
	if *memoryThreshold < 1 || *memoryThreshold > 100 {
		fmt.Fprintf(os.Stderr, "错误: 内存阈值必须在 1-100 之间\n")
		os.Exit(1)
	}

	agent := NewSystemTuneAgent(*cpuThreshold, *memoryThreshold)
	if err := agent.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}