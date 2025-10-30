#!/bin/bash

echo "🧪 测试动态平衡功能"
echo "===================="
echo ""

echo "📋 测试场景："
echo "1. 启动 system-tune-agent (目标: CPU 60%, 内存 60%)"
echo "2. 等待 10 秒观察正常工作"
echo "3. 启动额外的 CPU 和内存消费进程模拟其他程序"
echo "4. 观察 system-tune-agent 是否会自动减少自身占用"
echo ""

echo "🚀 启动 system-tune-agent..."
./system-tune-agent -c 60 -m 60 &
AGENT_PID=$!

echo "💤 等待 10 秒让 agent 稳定运行..."
sleep 10

echo ""
echo "🔥 启动额外的资源消费进程 (模拟其他程序占用增加)..."

# 启动 CPU 消费进程
echo "启动 CPU 消费进程..."
for i in {1..4}; do
    (while true; do echo "scale=5000; 4*a(1)" | bc -l > /dev/null 2>&1; done) &
    CPU_PIDS+=($!)
done

# 启动内存消费进程
echo "启动内存消费进程..."
python3 -c "
import time
data = []
print('分配内存中...')
for i in range(50):
    data.append(bytearray(10 * 1024 * 1024))  # 10MB per block
    time.sleep(0.1)
print('内存分配完成，保持占用...')
time.sleep(30)
" &
MEM_PID=$!

echo ""
echo "📊 观察 20 秒，看 system-tune-agent 如何响应..."
echo "预期行为：agent 应该检测到其他进程占用增加，并自动减少自身占用"
sleep 20

echo ""
echo "🧹 清理测试进程..."
kill $AGENT_PID 2>/dev/null
for pid in "${CPU_PIDS[@]}"; do
    kill $pid 2>/dev/null
done
kill $MEM_PID 2>/dev/null

echo "✅ 测试完成！"
echo ""
echo "💡 如果看到以下输出，说明动态平衡功能正常："
echo "   - '🔄 其他进程CPU占用过高，停止自身CPU占用'"
echo "   - '🔄 其他进程内存占用过高，释放自身内存占用'"
echo "   - CPU/内存强度自动调整为更低值"