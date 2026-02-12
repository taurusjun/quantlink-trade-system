#!/bin/bash

# ============================================
# 脚本名称: stop_all.sh
# 用途: 停止所有交易系统服务（优雅停止，确保 daily_init 保存）
# 作者: QuantLink Team
# 日期: 2026-02-12
#
# 相关文档:
#   - @docs/核心文档/USAGE.md
# ============================================

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  停止 QuantLink Trade System"
echo "════════════════════════════════════════════════════════════"
echo ""

PROCS="trader counter_bridge ors_gateway md_gateway ctp_md_gateway md_simulator"

# 第一步：发送 SIGTERM 优雅停止
for proc in $PROCS; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        echo -e "${GREEN}[INFO]${NC} 发送 SIGTERM 到 $proc..."
        pkill -TERM -f "$proc" 2>/dev/null || true
    fi
done

# 第二步：等待进程优雅退出（最多等 10 秒）
echo -e "${GREEN}[INFO]${NC} 等待进程优雅退出（SaveMatrix2 保存 daily_init）..."
for i in {1..10}; do
    remaining=0
    for proc in $PROCS; do
        if pgrep -f "$proc" > /dev/null 2>&1; then
            remaining=$((remaining + 1))
        fi
    done
    if [ $remaining -eq 0 ]; then
        echo -e "${GREEN}[INFO]${NC} 所有进程已优雅退出"
        break
    fi
    echo -e "${GREEN}[INFO]${NC} 仍有 $remaining 个进程运行中，等待... ($i/10)"
    sleep 1
done

# 第三步：如果还有进程没退出，发送 SIGKILL 强制终止
for proc in $PROCS; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        echo -e "${YELLOW}[WARN]${NC} $proc 未响应 SIGTERM，发送 SIGKILL..."
        pkill -KILL -f "$proc" 2>/dev/null || true
    fi
done

sleep 1

# 清理共享内存
echo -e "${GREEN}[INFO]${NC} 清理共享内存..."
ipcs -m 2>/dev/null | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

echo ""
echo -e "${GREEN}[INFO]${NC} ✓ 所有服务已停止"
echo -e "${GREEN}[INFO]${NC} ✓ 共享内存已清理"
echo ""
