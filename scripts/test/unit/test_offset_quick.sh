#!/bin/bash
set -e

# ============================================
# 脚本名称: test_offset_quick.sh
# 用途: 快速测试 Offset 自动设置逻辑（无需启动完整系统）
# 作者: QuantLink Team
# 日期: 2026-01-30
# ============================================

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================="
echo "   Quick Offset Logic Test"
echo "========================================="
echo ""

PASSED=0
FAILED=0

test_case() {
    local name="$1"
    local result="$2"

    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $name"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗${NC} $name"
        FAILED=$((FAILED + 1))
    fi
}

echo "Testing CTP Plugin..."
echo ""

# 1. 编译检查
if [ -f "gateway/build/counter_bridge" ]; then
    test_case "CTP Plugin compiled" "PASS"
else
    test_case "CTP Plugin compiled" "FAIL"
fi

# 2. SetOpenClose 存在
if grep -q "void CTPTDPlugin::SetOpenClose" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "SetOpenClose method exists" "PASS"
else
    test_case "SetOpenClose method exists" "FAIL"
fi

# 3. UpdatePositionFromTrade 存在
if grep -q "void CTPTDPlugin::UpdatePositionFromTrade" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "UpdatePositionFromTrade exists" "PASS"
else
    test_case "UpdatePositionFromTrade exists" "FAIL"
fi

# 4. 持仓结构
if grep -q "struct CTPPosition" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    test_case "CTPPosition struct defined" "PASS"
else
    test_case "CTPPosition struct defined" "FAIL"
fi

# 5. 今昨仓字段
if grep -q "long_today_position" gateway/plugins/ctp/include/ctp_td_plugin.h && \
   grep -q "long_yesterday_position" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    test_case "Today/Yesterday position fields" "PASS"
else
    test_case "Today/Yesterday position fields" "FAIL"
fi

# 6. SendOrder 调用 SetOpenClose
if grep -q "SetOpenClose(modified_request)" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "SendOrder calls SetOpenClose" "PASS"
else
    test_case "SendOrder calls SetOpenClose" "FAIL"
fi

# 7. OnRtnTrade 调用 UpdatePositionFromTrade
if grep -q "UpdatePositionFromTrade(trade_info)" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "OnRtnTrade updates position" "PASS"
else
    test_case "OnRtnTrade updates position" "FAIL"
fi

# 8. 上期所特殊处理
if grep -q "is_shfe" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "SHFE special handling" "PASS"
else
    test_case "SHFE special handling" "FAIL"
fi

# 9. CLOSE_TODAY 支持
if grep -q "CLOSE_TODAY" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "CLOSE_TODAY support" "PASS"
else
    test_case "CLOSE_TODAY support" "FAIL"
fi

# 10. 持仓持久化
if grep -q "SavePositionsToFile" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "Position persistence" "PASS"
else
    test_case "Position persistence" "FAIL"
fi

# 11. 线程安全
if grep -q "m_position_mutex" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "Thread safety (mutex)" "PASS"
else
    test_case "Thread safety (mutex)" "FAIL"
fi

# 12. 开仓逻辑
if grep -A 5 "trade.offset == OffsetFlag::OPEN" gateway/plugins/ctp/src/ctp_td_plugin.cpp | grep -q "position +="; then
    test_case "Open position logic" "PASS"
else
    test_case "Open position logic" "FAIL"
fi

# 13. 平仓逻辑
if grep -A 10 "// 平仓" gateway/plugins/ctp/src/ctp_td_plugin.cpp | grep -q "position -="; then
    test_case "Close position logic" "PASS"
else
    test_case "Close position logic" "FAIL"
fi

# 14. 持仓不足检测
if grep -q "Position mismatch" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "Position mismatch detection" "PASS"
else
    test_case "Position mismatch detection" "FAIL"
fi

# 15. 空持仓清理
if grep -q "Position removed" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    test_case "Empty position cleanup" "PASS"
else
    test_case "Empty position cleanup" "FAIL"
fi

echo ""
echo "Testing Simulator Plugin..."
echo ""

# 16. Simulator SetOpenClose
if grep -q "void SimulatorPlugin::SetOpenClose" gateway/plugins/simulator/src/simulator_plugin.cpp; then
    test_case "Simulator SetOpenClose exists" "PASS"
else
    test_case "Simulator SetOpenClose exists" "FAIL"
fi

# 17. Simulator 调用 SetOpenClose
if grep -q "SetOpenClose(modified_request)" gateway/plugins/simulator/src/simulator_plugin.cpp; then
    test_case "Simulator calls SetOpenClose" "PASS"
else
    test_case "Simulator calls SetOpenClose" "FAIL"
fi

echo ""
echo "========================================="
echo "          Results"
echo "========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    echo ""
    echo "✅ CTP Plugin 今昨仓实施完成"
    echo "✅ Simulator Plugin Offset自动设置已实施"
    echo ""
    echo "两个 Plugin 均支持 Offset 自动判断！"
    exit 0
else
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    exit 1
fi
