#!/bin/bash
# 停止CTP端到端测试的所有进程

PROJECT_ROOT="/Users/user/PWorks/RD/quantlink-trade-system"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "╔═══════════════════════════════════════════════════════╗"
echo "║            停止CTP端到端测试                            ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 停止所有进程
echo -e "${YELLOW}停止所有测试进程...${NC}"
pkill -f ctp_md_gateway
pkill -f md_gateway
pkill -f ors_gateway
pkill -f counter_bridge
pkill -f "trader -config"
sleep 2

# 清理共享内存
echo -e "${YELLOW}清理共享内存...${NC}"
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

# 清理PID文件
rm -f /tmp/ctp_e2e_pids.txt

# 检查是否还有残留进程
REMAINING=$(ps aux | grep -E "ctp_md_gateway|md_gateway|ors_gateway|counter_bridge|trader -config" | grep -v grep | wc -l)

if [ $REMAINING -eq 0 ]; then
    echo -e "${GREEN}✓ 所有进程已停止${NC}"
else
    echo -e "${YELLOW}⚠ 还有 $REMAINING 个进程残留${NC}"
    ps aux | grep -E "ctp_md_gateway|md_gateway|ors_gateway|counter_bridge|trader -config" | grep -v grep
fi

echo ""
echo "完成！"
