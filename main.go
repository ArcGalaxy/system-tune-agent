package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
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

type SystemTuneAgent struct {
	cpuThreshold    float64
	memoryThreshold float64

	// CPU 控制
	cpuIntensity int
	cpuWorkers   []context.CancelFunc
	cpuMutex     sync.RWMutex

	// 内存控制
	memoryBlocks        [][]byte
	memoryMutex         sync.RWMutex
	shouldConsumeMemory bool

	// 状态跟踪
	lastCPUUsage    float64
	lastMemoryUsage float64
	
	// 动态平衡控制
	baseCPUUsage    float64  // 不包含本程序的基础CPU使用率
	baseMemoryUsage float64  // 不包含本程序的基础内存使用率
	selfCPUUsage    float64  // 本程序的CPU占用估算
	selfMemoryUsage float64  // 本程序的内存占用估算

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

func NewSystemTuneAgent(cpuThreshold, memoryThreshold float64) *SystemTuneAgent {
	ctx, cancel := context.WithCancel(context.Background())

	return &SystemTuneAgent{
		cpuThreshold:    cpuThreshold,
		memoryThreshold: memoryThreshold,
		cpuWorkers:      make([]context.CancelFunc, 0),
		memoryBlocks:    make([][]byte, 0),
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (s *SystemTuneAgent) Start() error {
	fmt.Printf("🚀 System Tune Agent 启动 (Go 版本 - 动态平衡优化)\n")
	fmt.Printf("🎯 CPU 阈值: %.1f%%\n", s.cpuThreshold)
	fmt.Printf("🎯 内存阈值: %.1f%%\n", s.memoryThreshold)
	fmt.Printf("⏱️ 监控间隔: %v\n", monitorInterval)
	fmt.Printf("🖥️ CPU 核心数: %d\n", runtime.NumCPU())
	fmt.Println("✨ 动态平衡模式：当其他进程占用增加时，自动减少自身占用")
	fmt.Println("⚠️ 按 Ctrl+C 停止程序")
	fmt.Println()

	// 初始化基础状态
	if err := s.initializeBaseState(); err != nil {
		return fmt.Errorf("初始化失败: %v", err)
	}

	// 启动监控循环
	go s.monitorLoop()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n🛑 正在停止 System Tune Agent...")
	s.Stop()
	return nil
}

func (s *SystemTuneAgent) initializeBaseState() error {
	// 获取初始状态，作为基础参考
	cpuUsage, err := s.getCPUUsage()
	if err != nil {
		return fmt.Errorf("获取初始CPU使用率失败: %v", err)
	}

	memoryUsage, err := s.getMemoryUsage()
	if err != nil {
		return fmt.Errorf("获取初始内存使用率失败: %v", err)
	}

	// 初始状态下，所有占用都是基础占用
	s.baseCPUUsage = cpuUsage
	s.baseMemoryUsage = memoryUsage
	s.selfCPUUsage = 0
	s.selfMemoryUsage = 0

	fmt.Printf("📊 初始状态 - CPU: %.1f%%, 内存: %.1f%%\n", cpuUsage, memoryUsage)
	return nil
}

func (s *SystemTuneAgent) Stop() {
	s.cancel()
	s.stopCPUConsumption()
	s.stopMemoryConsumption()
	fmt.Println("System Tune Agent 已停止")
}

func (s *SystemTuneAgent) monitorLoop() {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.monitorAndAdjust()
		}
	}
}

func (s *SystemTuneAgent) monitorAndAdjust() {
	cpuUsage, err := s.getCPUUsage()
	if err != nil {
		log.Printf("获取 CPU 使用率失败: %v", err)
		return
	}

	memoryUsage, err := s.getMemoryUsage()
	if err != nil {
		log.Printf("获取内存使用率失败: %v", err)
		return
	}

	// 估算基础使用率（不包含本程序的占用）
	s.estimateBaseUsage(cpuUsage, memoryUsage)

	fmt.Printf("当前状态 - 总CPU: %.1f%%, 总内存: %.1f%% | 基础CPU: %.1f%%, 基础内存: %.1f%% | 自身CPU: %.1f%%, 自身内存: %.1f%%\n", 
		cpuUsage, memoryUsage, s.baseCPUUsage, s.baseMemoryUsage, s.selfCPUUsage, s.selfMemoryUsage)

	// 调整资源占用 - 基于动态平衡策略
	s.adjustCPUConsumptionDynamic(cpuUsage)
	s.adjustMemoryConsumptionDynamic(memoryUsage)

	s.lastCPUUsage = cpuUsage
	s.lastMemoryUsage = memoryUsage
}
func (s *SystemTuneAgent) getCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) == 0 {
		return 0, fmt.Errorf("无法获取 CPU 使用率")
	}
	return percentages[0], nil
}

func (s *SystemTuneAgent) getMemoryUsage() (float64, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return memInfo.UsedPercent, nil
}

// 估算基础使用率和自身占用
func (s *SystemTuneAgent) estimateBaseUsage(currentCPUUsage, currentMemoryUsage float64) {
	// 估算自身CPU占用（基于当前强度）
	s.cpuMutex.RLock()
	intensity := s.cpuIntensity
	s.cpuMutex.RUnlock()
	
	// CPU强度与实际占用的经验公式（可根据实际情况调整）
	s.selfCPUUsage = float64(intensity) * 0.8 * float64(runtime.NumCPU()) / 100.0
	if s.selfCPUUsage > currentCPUUsage {
		s.selfCPUUsage = currentCPUUsage * 0.9 // 防止估算过高
	}

	// 估算自身内存占用
	s.memoryMutex.RLock()
	memoryBlocks := len(s.memoryBlocks)
	s.memoryMutex.RUnlock()
	
	// 每个内存块平均大小估算
	avgBlockSizeMB := 25.0 // 平均块大小
	selfMemoryMB := float64(memoryBlocks) * avgBlockSizeMB
	
	// 获取总内存来计算百分比
	if memInfo, err := mem.VirtualMemory(); err == nil {
		totalMemoryMB := float64(memInfo.Total) / (1024 * 1024)
		s.selfMemoryUsage = selfMemoryMB / totalMemoryMB * 100
		if s.selfMemoryUsage > currentMemoryUsage {
			s.selfMemoryUsage = currentMemoryUsage * 0.9 // 防止估算过高
		}
	}

	// 计算基础使用率（其他进程的占用）
	s.baseCPUUsage = math.Max(0, currentCPUUsage - s.selfCPUUsage)
	s.baseMemoryUsage = math.Max(0, currentMemoryUsage - s.selfMemoryUsage)
}

// 动态平衡的CPU调整策略
func (s *SystemTuneAgent) adjustCPUConsumptionDynamic(currentCPUUsage float64) {
	// 计算需要的总占用和当前基础占用的差距
	targetSelfCPU := math.Max(0, s.cpuThreshold - s.baseCPUUsage)
	currentSelfCPU := s.selfCPUUsage
	
	// 如果基础占用已经超过阈值，立即停止自身占用
	if s.baseCPUUsage >= s.cpuThreshold {
		targetSelfCPU = 0
		fmt.Printf("🔄 其他进程CPU占用过高(%.1f%% >= %.1f%%)，停止自身CPU占用\n", 
			s.baseCPUUsage, s.cpuThreshold)
	}

	cpuGap := targetSelfCPU - currentSelfCPU

	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	newCPUIntensity := s.cpuIntensity

	if math.Abs(cpuGap) > 1.0 {
		// 根据差距调整强度
		intensityChange := int(cpuGap * 2.0) // 调整系数
		
		if cpuGap > 0 {
			// 需要增加占用
			if cpuGap > 5 {
				intensityChange = int(math.Min(15, cpuGap*1.5)) // 快速增加
			} else {
				intensityChange = int(math.Max(2, cpuGap*0.8)) // 缓慢增加
			}
			newCPUIntensity = int(math.Min(90, float64(s.cpuIntensity + intensityChange)))
		} else {
			// 需要减少占用
			if cpuGap < -5 {
				intensityChange = int(math.Max(-20, cpuGap*2)) // 快速减少
			} else {
				intensityChange = int(math.Min(-2, cpuGap*1.2)) // 缓慢减少
			}
			newCPUIntensity = int(math.Max(0, float64(s.cpuIntensity + intensityChange)))
		}
	}

	if newCPUIntensity != s.cpuIntensity {
		oldIntensity := s.cpuIntensity
		s.cpuIntensity = newCPUIntensity
		
		fmt.Printf("🎯 调整CPU强度: %d%% -> %d%% (目标自身占用: %.1f%%, 当前: %.1f%%)\n",
			oldIntensity, s.cpuIntensity, targetSelfCPU, currentSelfCPU)

		if s.cpuIntensity > 0 && len(s.cpuWorkers) == 0 {
			s.startCPUConsumption()
		} else if s.cpuIntensity == 0 && len(s.cpuWorkers) > 0 {
			s.stopCPUConsumption()
		}
	}
}

func (s *SystemTuneAgent) startCPUConsumption() {
	numCPU := runtime.NumCPU()

	for i := 0; i < numCPU; i++ {
		ctx, cancel := context.WithCancel(s.ctx)
		s.cpuWorkers = append(s.cpuWorkers, cancel)

		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					s.cpuMutex.RLock()
					intensity := s.cpuIntensity
					s.cpuMutex.RUnlock()

					if intensity > 0 {
						// 工作时间和休息时间的比例
						workDuration := time.Duration(intensity) * time.Millisecond
						restDuration := time.Duration(100-intensity) * time.Millisecond

						// CPU 密集计算
						start := time.Now()
						for time.Since(start) < workDuration {
							// 执行一些计算密集的操作
							for j := 0; j < 10000; j++ {
								math.Sin(float64(j) * 0.001)
								math.Cos(float64(j) * 0.001)
								math.Sqrt(float64(j % 1000))
							}

							// 检查是否需要停止
							select {
							case <-ctx.Done():
								return
							default:
							}
						}

						// 休息时间
						if restDuration > 0 {
							time.Sleep(restDuration)
						}
					} else {
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}(i)
	}

	fmt.Printf("启动 %d 个 CPU 消费线程\n", numCPU)
}

func (s *SystemTuneAgent) stopCPUConsumption() {
	for _, cancel := range s.cpuWorkers {
		cancel()
	}
	s.cpuWorkers = s.cpuWorkers[:0]
	fmt.Println("停止 CPU 占用")
}

// 动态平衡的内存调整策略
func (s *SystemTuneAgent) adjustMemoryConsumptionDynamic(currentMemoryUsage float64) {
	// 计算需要的自身内存占用
	targetSelfMemory := math.Max(0, s.memoryThreshold - s.baseMemoryUsage)
	currentSelfMemory := s.selfMemoryUsage
	
	// 如果基础占用已经超过阈值，立即释放自身占用
	if s.baseMemoryUsage >= s.memoryThreshold {
		targetSelfMemory = 0
		fmt.Printf("🔄 其他进程内存占用过高(%.1f%% >= %.1f%%)，释放自身内存占用\n", 
			s.baseMemoryUsage, s.memoryThreshold)
	}

	memoryGap := targetSelfMemory - currentSelfMemory

	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	fmt.Printf("📊 内存状态 - 总: %.1f%%, 基础: %.1f%%, 自身: %.1f%% -> 目标: %.1f%%, 差距: %.1f%%, 已分配块: %d\n",
		currentMemoryUsage, s.baseMemoryUsage, currentSelfMemory, targetSelfMemory, memoryGap, len(s.memoryBlocks))

	if math.Abs(memoryGap) > 1 {
		if memoryGap > 2 && !s.shouldConsumeMemory {
			// 需要增加内存占用
			s.shouldConsumeMemory = true
			go s.startMemoryConsumptionDynamic(targetSelfMemory)
			fmt.Printf("🚀 开始增加内存占用，目标自身占用: %.1f%%\n", targetSelfMemory)
		} else if memoryGap < -1 {
			// 需要减少内存占用
			if memoryGap < -3 {
				// 差距较大，快速释放
				s.releaseMemoryByPercentage(0.5) // 释放50%
				fmt.Printf("⚡ 快速释放内存 (差距: %.1f%%)\n", memoryGap)
			} else {
				// 差距较小，缓慢释放
				s.releaseMemoryByPercentage(0.2) // 释放20%
				fmt.Printf("🔽 缓慢释放内存 (差距: %.1f%%)\n", memoryGap)
			}

			// 如果目标为0或差距过大，停止内存消费
			if targetSelfMemory <= 0 || memoryGap < -5 {
				s.shouldConsumeMemory = false
				fmt.Println("⏹️ 停止内存占用")
			}
		}
	}
}

func (s *SystemTuneAgent) startMemoryConsumptionDynamic(targetSelfMemory float64) {
	for s.shouldConsumeMemory {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 获取当前内存使用情况
		memInfo, err := mem.VirtualMemory()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		currentUsage := memInfo.UsedPercent
		
		// 重新估算当前状态
		s.estimateBaseUsage(s.lastCPUUsage, currentUsage)
		
		// 检查是否还需要继续分配
		currentGap := targetSelfMemory - s.selfMemoryUsage
		if currentGap <= 1 || s.baseMemoryUsage >= s.memoryThreshold {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// 动态计算内存块大小，基于差距和当前基础占用情况
		blockSizeMB := int(math.Max(3, math.Min(30, currentGap*1.5)))
		
		// 如果基础占用接近阈值，使用更小的块
		if s.baseMemoryUsage > s.memoryThreshold*0.8 {
			blockSizeMB = int(math.Max(2, float64(blockSizeMB)*0.5))
		}
		
		blockSize := blockSizeMB * 1024 * 1024

		fmt.Printf("💾 分配内存块: %dMB (目标差距: %.1f%%)\n", blockSizeMB, currentGap)

		// 分配并填充内存
		memoryBlock := make([]byte, blockSize)
		for i := 0; i < len(memoryBlock); i += 1024 {
			// 填充随机数据防止被优化
			timestamp := time.Now().UnixNano()
			memoryBlock[i] = byte(timestamp % 256)
			if i+512 < len(memoryBlock) {
				memoryBlock[i+512] = byte((timestamp >> 8) % 256)
			}
		}

		s.memoryMutex.Lock()
		s.memoryBlocks = append(s.memoryBlocks, memoryBlock)

		// 控制内存块数量，防止过度分配
		if len(s.memoryBlocks) > 150 {
			s.memoryBlocks = s.memoryBlocks[1:]
		}
		s.memoryMutex.Unlock()

		// 动态调整分配间隔
		sleepTime := 200 * time.Millisecond
		if currentGap < 5 {
			sleepTime = 500 * time.Millisecond // 接近目标时放慢速度
		}
		time.Sleep(sleepTime)
	}
}

func (s *SystemTuneAgent) releasePartialMemory() {
	s.releaseMemoryByPercentage(0.3)
}

func (s *SystemTuneAgent) releaseMemoryByPercentage(percentage float64) {
	if len(s.memoryBlocks) == 0 {
		return
	}

	// 释放指定百分比的内存块
	releaseCount := int(math.Max(1, float64(len(s.memoryBlocks))*percentage))
	
	fmt.Printf("🗑️ 释放 %d 个内存块 (%.0f%%)\n", releaseCount, percentage*100)

	for i := 0; i < releaseCount && len(s.memoryBlocks) > 0; i++ {
		s.memoryBlocks = s.memoryBlocks[1:]
	}

	// 触发垃圾回收
	runtime.GC()
}

func (s *SystemTuneAgent) stopMemoryConsumption() {
	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	s.shouldConsumeMemory = false
	s.memoryBlocks = nil
	runtime.GC()
	fmt.Println("释放所有内存占用")
}

func main() {
	var (
		cpuThreshold    = flag.Float64("cpu", defaultCPUThreshold, "CPU 使用率阈值 (1-100)")
		memoryThreshold = flag.Float64("memory", defaultMemoryThreshold, "内存使用率阈值 (1-100)")
		showHelp        = flag.Bool("help", false, "显示帮助信息")
	)

	flag.Float64Var(cpuThreshold, "c", defaultCPUThreshold, "CPU 使用率阈值 (简写)")
	flag.Float64Var(memoryThreshold, "m", defaultMemoryThreshold, "内存使用率阈值 (简写)")
	flag.BoolVar(showHelp, "h", false, "显示帮助信息 (简写)")

	flag.Parse()

	if *showHelp {
		fmt.Println("System Tune Agent - 系统资源调节工具 (Go 版本)")
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

	agent := NewSystemTuneAgent(*cpuThreshold, *memoryThreshold)
	if err := agent.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
