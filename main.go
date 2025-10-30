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

	// 状态
	lastCPUUsage    float64
	lastMemoryUsage float64

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
	fmt.Printf("System Tune Agent 启动 (Go 版本)\n")
	fmt.Printf("CPU 阈值: %.1f%%\n", s.cpuThreshold)
	fmt.Printf("内存阈值: %.1f%%\n", s.memoryThreshold)
	fmt.Printf("监控间隔: %v\n", monitorInterval)
	fmt.Println("按 Ctrl+C 停止程序")
	fmt.Println()

	// 启动监控循环
	go s.monitorLoop()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n正在停止 System Tune Agent...")
	s.Stop()
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

	fmt.Printf("当前状态 - CPU: %.1f%%, 内存: %.1f%%\n", cpuUsage, memoryUsage)

	// 调整资源占用
	s.adjustCPUConsumption(cpuUsage)
	s.adjustMemoryConsumption(memoryUsage)

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

func (s *SystemTuneAgent) adjustCPUConsumption(currentCPUUsage float64) {
	cpuGap := s.cpuThreshold - currentCPUUsage

	s.cpuMutex.Lock()
	defer s.cpuMutex.Unlock()

	newCPUIntensity := s.cpuIntensity

	if math.Abs(cpuGap) > 0.5 {
		if cpuGap > 2 {
			// 差距较大，快速增加
			increase := int(math.Max(5, cpuGap*1.5))
			newCPUIntensity = int(math.Min(80, float64(s.cpuIntensity+increase)))
		} else if cpuGap > 0 {
			// 差距较小，缓慢增加
			increase := int(math.Max(1, cpuGap))
			newCPUIntensity = int(math.Min(60, float64(s.cpuIntensity+increase)))
		} else if cpuGap < -2 {
			// 超出较多，快速减少
			decrease := int(math.Min(-5, cpuGap*1.5))
			newCPUIntensity = int(math.Max(0, float64(s.cpuIntensity+decrease)))
		} else {
			// 轻微超出，缓慢减少
			decrease := int(math.Min(-1, cpuGap))
			newCPUIntensity = int(math.Max(0, float64(s.cpuIntensity+decrease)))
		}
	}

	if newCPUIntensity != s.cpuIntensity {
		s.cpuIntensity = newCPUIntensity
		fmt.Printf("调整 CPU 强度: %d%% (当前: %.1f%%, 目标: %.1f%%)\n",
			s.cpuIntensity, currentCPUUsage, s.cpuThreshold)

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

func (s *SystemTuneAgent) adjustMemoryConsumption(currentMemoryUsage float64) {
	memoryGap := s.memoryThreshold - currentMemoryUsage

	s.memoryMutex.Lock()
	defer s.memoryMutex.Unlock()

	fmt.Printf("内存状态 - 当前: %.1f%%, 目标: %.1f%%, 差距: %.1f%%, 已分配块: %d\n",
		currentMemoryUsage, s.memoryThreshold, memoryGap, len(s.memoryBlocks))

	if math.Abs(memoryGap) > 1 {
		if memoryGap > 3 && !s.shouldConsumeMemory {
			// 差距较大，开始增加内存占用
			s.shouldConsumeMemory = true
			go s.startMemoryConsumption()
			fmt.Printf("开始增加内存占用，目标: %.1f%%\n", s.memoryThreshold)
		} else if memoryGap < -1 && s.shouldConsumeMemory {
			// 超出阈值，逐步释放内存
			s.releasePartialMemory()
			fmt.Printf("逐步释放内存占用 (当前超出 %.1f%%)\n", math.Abs(memoryGap))

			// 如果超出太多，停止内存消费
			if memoryGap < -3 {
				s.shouldConsumeMemory = false
				fmt.Println("超出阈值过多，停止内存占用")
			}
		}
	}
}

func (s *SystemTuneAgent) startMemoryConsumption() {
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
		memoryGap := s.memoryThreshold - currentUsage

		if memoryGap <= 0 {
			time.Sleep(time.Second)
			continue
		}

		// 动态计算内存块大小 (5MB - 50MB)
		blockSizeMB := int(math.Max(5, math.Min(50, memoryGap*2)))
		blockSize := blockSizeMB * 1024 * 1024

		fmt.Printf("分配内存块: %dMB\n", blockSizeMB)

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

		// 控制内存块数量
		if len(s.memoryBlocks) > 100 {
			s.memoryBlocks = s.memoryBlocks[1:]
		}
		s.memoryMutex.Unlock()

		time.Sleep(300 * time.Millisecond)
	}
}

func (s *SystemTuneAgent) releasePartialMemory() {
	if len(s.memoryBlocks) == 0 {
		return
	}

	// 释放 30% 的内存块
	releaseCount := int(math.Max(1, float64(len(s.memoryBlocks))*0.3))

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
