#!/bin/bash
# 获取ag2603实时行情价格

SYMBOL="ag2603"
LOG_FILE="/tmp/ctp_md_live.log"

echo "=========================================="
echo "获取 $SYMBOL 实时行情"
echo "=========================================="
echo ""

# 清理旧日志
rm -f "$LOG_FILE"

# 启动行情网关
echo "🚀 启动CTP行情网关..."
./gateway/build/plugins/ctp/ctp_md_plugin --config config/ctp/ctp_md.yaml > "$LOG_FILE" 2>&1 &
MD_PID=$!
echo "   PID: $MD_PID"

# 等待连接和订阅
echo "⏳ 等待行情数据..."
sleep 8

# 检查是否收到行情
echo ""
echo "📊 查找 $SYMBOL 行情数据..."
echo ""

# 从日志中提取行情信息
# CTP行情网关应该会打印行情tick数据
grep -i "$SYMBOL" "$LOG_FILE" | tail -20

echo ""
echo "----------------------------------------"
echo "完整日志保存在: $LOG_FILE"
echo "行情网关 PID: $MD_PID"
echo ""
echo "💡 使用 'kill $MD_PID' 停止行情网关"
echo "💡 使用 'tail -f $LOG_FILE' 查看实时行情"
echo "=========================================="
