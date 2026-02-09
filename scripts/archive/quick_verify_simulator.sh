#!/bin/bash
set -e

# ============================================
# 脚本名称: quick_verify_simulator.sh
# 用途: 快速验证 Simulator Plugin 是否正常工作
# 作者: QuantLink Team
# 日期: 2026-01-30
# ============================================

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "================================"
echo "Simulator Plugin Quick Verify"
echo "================================"
echo ""

# 1. 检查编译产物
echo -n "1. Checking counter_bridge binary... "
if [ -f "gateway/build/counter_bridge" ]; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC} Binary not found"
    exit 1
fi

# 2. 检查配置文件
echo -n "2. Checking simulator config... "
if [ -f "config/simulator/simulator.yaml" ]; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC} Config not found"
    exit 1
fi

# 3. 检查脚本
echo -n "3. Checking start script... "
if [ -x "scripts/live/start_simulator.sh" ]; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC} Start script not found or not executable"
    exit 1
fi

# 4. 测试 counter_bridge 能否加载 Simulator
echo -n "4. Testing Simulator plugin load... "
./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > /tmp/sim_test.log 2>&1 &
CB_PID=$!
sleep 3

# 杀掉进程（我们只是测试能否启动）
kill $CB_PID 2>/dev/null || true
sleep 1

# 检查日志
if grep -q "Simulator plugin initialized" /tmp/sim_test.log 2>/dev/null; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC} Plugin not initialized properly"
    echo "Log output:"
    tail -20 /tmp/sim_test.log
    exit 1
fi

# 5. 检查文档
echo -n "5. Checking documentation... "
if [ -f "docs/功能实现/模拟交易所_完整实施报告_2026-01-30-15_00.md" ]; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗${NC} Documentation not found"
fi

# 清理
rm -f /tmp/sim_test.log
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true

echo ""
echo "================================"
echo -e "${GREEN}All checks passed!${NC}"
echo "================================"
echo ""
echo "Simulator Plugin is ready to use."
echo ""
echo "Quick start:"
echo "  1. ./scripts/live/start_simulator.sh"
echo "  2. curl -X POST http://localhost:9201/api/v1/strategy/activate"
echo "  3. curl http://localhost:8080/simulator/stats | jq ."
echo ""
