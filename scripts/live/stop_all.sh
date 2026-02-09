#!/bin/bash

# ============================================
# 脚本名称: stop_all.sh
# 用途: 停止所有交易系统服务
# 作者: QuantLink Team
# 日期: 2026-02-09
# ============================================

echo "Stopping all services..."

# 停止所有相关进程
pkill -f ctp_md_gateway 2>/dev/null || true
pkill -f md_simulator 2>/dev/null || true
pkill -f md_gateway 2>/dev/null || true
pkill -f ors_gateway 2>/dev/null || true
pkill -f counter_bridge 2>/dev/null || true
pkill -f "trader -config" 2>/dev/null || true

sleep 1

# 清理共享内存
ipcs -m 2>/dev/null | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

echo "✓ All services stopped"
echo "✓ Shared memory cleaned"
