#!/bin/bash

# ============================================
# 脚本名称: stop_all.sh
# 用途: 停止所有交易系统服务
# 作者: QuantLink Team
# 日期: 2026-01-30
# ============================================

echo "Stopping all services..."

# 停止所有相关进程
pkill -f md_simulator || true
pkill -f md_gateway || true
pkill -f ors_gateway || true
pkill -f counter_bridge || true
pkill -f "trader -config" || true
pkill -f nats-server || true

sleep 1

# 清理共享内存
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true

echo "✓ All services stopped"
echo "✓ Shared memory cleaned"
