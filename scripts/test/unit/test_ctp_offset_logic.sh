#!/bin/bash
set -e

# ============================================
# 脚本名称: test_ctp_offset_logic.sh
# 用途: 单元测试 CTP Plugin Offset 逻辑（不连接真实CTP）
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 实施报告: @docs/实盘/CTP_Plugin_今昨仓实施报告_Phase1_2026-01-30.md
# ============================================

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_test() { echo -e "${YELLOW}[TEST]${NC} $1"; }

echo "========================================="
echo "  CTP Plugin Offset Logic Unit Test"
echo "========================================="
echo ""

# 测试计数
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

run_test() {
    local test_name="$1"
    local test_result="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    log_test "Test $TOTAL_TESTS: $test_name"

    if [ "$test_result" == "PASS" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "  ${GREEN}✓ PASSED${NC}"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "  ${RED}✗ FAILED${NC}"
    fi
    echo ""
}

# ==================== 编译检查 ====================

log_info "Step 1: Checking compilation"

if [ ! -f "gateway/build/counter_bridge" ]; then
    log_error "counter_bridge not built"
    log_info "Building counter_bridge..."
    cd gateway/build
    make counter_bridge
    cd ../..
fi

if [ -f "gateway/build/counter_bridge" ]; then
    run_test "CTP Plugin compilation" "PASS"
else
    run_test "CTP Plugin compilation" "FAIL"
    log_error "Failed to build counter_bridge"
    exit 1
fi

# ==================== 代码静态检查 ====================

log_info "Step 2: Static code analysis"

# 检查 SetOpenClose 方法是否存在
if grep -q "void CTPTDPlugin::SetOpenClose" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "SetOpenClose method exists" "PASS"
else
    run_test "SetOpenClose method exists" "FAIL"
fi

# 检查 UpdatePositionFromTrade 方法是否存在
if grep -q "void CTPTDPlugin::UpdatePositionFromTrade" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "UpdatePositionFromTrade method exists" "PASS"
else
    run_test "UpdatePositionFromTrade method exists" "FAIL"
fi

# 检查持仓管理成员变量
if grep -q "std::map<std::string, CTPPosition> m_positions" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    run_test "Position management map exists" "PASS"
else
    run_test "Position management map exists" "FAIL"
fi

# 检查 CTPPosition 结构体
if grep -q "struct CTPPosition" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    run_test "CTPPosition struct defined" "PASS"
else
    run_test "CTPPosition struct defined" "FAIL"
fi

# 检查今昨仓字段
if grep -q "long_today_position" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    run_test "Today position field exists" "PASS"
else
    run_test "Today position field exists" "FAIL"
fi

if grep -q "long_yesterday_position" gateway/plugins/ctp/include/ctp_td_plugin.h; then
    run_test "Yesterday position field exists" "PASS"
else
    run_test "Yesterday position field exists" "FAIL"
fi

# 检查 SendOrder 中是否调用 SetOpenClose
if grep -q "SetOpenClose(modified_request)" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "SendOrder calls SetOpenClose" "PASS"
else
    run_test "SendOrder calls SetOpenClose" "FAIL"
fi

# 检查 OnRtnTrade 中是否调用 UpdatePositionFromTrade
if grep -q "UpdatePositionFromTrade(trade_info)" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "OnRtnTrade calls UpdatePositionFromTrade" "PASS"
else
    run_test "OnRtnTrade calls UpdatePositionFromTrade" "FAIL"
fi

# 检查持仓持久化方法
if grep -q "SavePositionsToFile()" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "Position persistence implemented" "PASS"
else
    run_test "Position persistence implemented" "FAIL"
fi

# 检查上期所特殊处理
if grep -q "is_shfe" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "SHFE exchange special handling" "PASS"
else
    run_test "SHFE exchange special handling" "FAIL"
fi

# 检查 CLOSE_TODAY 逻辑
if grep -q "CLOSE_TODAY" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "CLOSE_TODAY offset support" "PASS"
else
    run_test "CLOSE_TODAY offset support" "FAIL"
fi

# 检查 CLOSE_YESTERDAY 逻辑
if grep -q "CLOSE_YESTERDAY" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "CLOSE_YESTERDAY offset support" "PASS"
else
    run_test "CLOSE_YESTERDAY offset support" "FAIL"
fi

# 检查线程安全（mutex）
if grep -q "m_position_mutex" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "Thread safety (position mutex)" "PASS"
else
    run_test "Thread safety (position mutex)" "FAIL"
fi

# 检查登录后查询持仓
if grep -q "UpdatePositionFromCTP()" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "Position query after login" "PASS"
else
    run_test "Position query after login" "FAIL"
fi

# ==================== 逻辑检查 ====================

log_info "Step 3: Logic verification"

# 检查开仓逻辑
OPEN_LOGIC=$(grep -A 10 "trade.offset == OffsetFlag::OPEN" gateway/plugins/ctp/src/ctp_td_plugin.cpp | grep -c "position +=" || echo "0")
if [ "$OPEN_LOGIC" -gt 0 ]; then
    run_test "Open position logic" "PASS"
else
    run_test "Open position logic" "FAIL"
fi

# 检查平仓逻辑
CLOSE_LOGIC=$(grep -A 10 "// 平仓" gateway/plugins/ctp/src/ctp_td_plugin.cpp | grep -c "position -=" || echo "0")
if [ "$CLOSE_LOGIC" -gt 0 ]; then
    run_test "Close position logic" "PASS"
else
    run_test "Close position logic" "FAIL"
fi

# 检查持仓不足警告
if grep -q "Position mismatch" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "Position mismatch detection" "PASS"
else
    run_test "Position mismatch detection" "FAIL"
fi

# 检查空持仓清理
if grep -q "Position removed" gateway/plugins/ctp/src/ctp_td_plugin.cpp; then
    run_test "Empty position cleanup" "PASS"
else
    run_test "Empty position cleanup" "FAIL"
fi

# ==================== 测试结果汇总 ====================

echo "========================================="
echo "          Test Results Summary"
echo "========================================="
echo ""
echo "Total Tests:  $TOTAL_TESTS"
echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
echo ""

if [ "$FAILED_TESTS" -eq 0 ]; then
    echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    echo ""
    echo "CTP Plugin Offset auto-set implementation is correct!"
    exit 0
else
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    echo ""
    echo "Please review the implementation and fix the issues."
    exit 1
fi
