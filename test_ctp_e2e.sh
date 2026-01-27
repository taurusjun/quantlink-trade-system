#!/bin/bash
# CTP网关端到端测试脚本

set -e

echo "=========================================="
echo "CTP Market Data Gateway E2E Test"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 清理函数
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    
    # 停止CTP网关
    if [ ! -z "$CTP_PID" ]; then
        kill -SIGINT $CTP_PID 2>/dev/null || true
        wait $CTP_PID 2>/dev/null || true
        echo -e "${GREEN}✓${NC} CTP gateway stopped"
    fi
    
    # 清理共享内存
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true
    echo -e "${GREEN}✓${NC} Shared memory cleaned"
}

# 设置退出时清理
trap cleanup EXIT

# 检查配置文件
echo "Step 1: Checking configuration files..."
if [ ! -f "config/ctp_md.yaml" ]; then
    echo -e "${RED}✗${NC} config/ctp_md.yaml not found"
    exit 1
fi
if [ ! -f "config/ctp_md.secret.yaml" ]; then
    echo -e "${RED}✗${NC} config/ctp_md.secret.yaml not found"
    exit 1
fi
echo -e "${GREEN}✓${NC} Configuration files exist"
echo ""

# 检查可执行文件
echo "Step 2: Checking CTP gateway binary..."
if [ ! -f "gateway/build/ctp_md_gateway" ]; then
    echo -e "${RED}✗${NC} ctp_md_gateway not found"
    echo "Please run: cd gateway/build && make ctp_md_gateway"
    exit 1
fi
echo -e "${GREEN}✓${NC} CTP gateway binary exists"
echo ""

# 创建必要目录
echo "Step 3: Creating required directories..."
mkdir -p ctp_flow log test_logs
echo -e "${GREEN}✓${NC} Directories created"
echo ""

# 启动CTP网关
echo "Step 4: Starting CTP gateway..."
./gateway/build/ctp_md_gateway -c config/ctp_md.yaml > test_logs/ctp_e2e.log 2>&1 &
CTP_PID=$!
echo "CTP gateway PID: $CTP_PID"
echo ""

# 等待连接和登录
echo "Step 5: Waiting for connection and login..."
for i in {1..30}; do
    if grep -q "Login successful" test_logs/ctp_e2e.log 2>/dev/null; then
        echo -e "${GREEN}✓${NC} CTP login successful"
        break
    fi
    if grep -q "Login failed" test_logs/ctp_e2e.log 2>/dev/null; then
        echo -e "${RED}✗${NC} CTP login failed"
        cat test_logs/ctp_e2e.log
        exit 1
    fi
    echo -n "."
    sleep 1
done
echo ""

# 检查是否登录成功
if ! grep -q "Login successful" test_logs/ctp_e2e.log; then
    echo -e "${RED}✗${NC} CTP login timeout"
    cat test_logs/ctp_e2e.log
    exit 1
fi
echo ""

# 等待订阅确认
echo "Step 6: Waiting for subscription confirmation..."
sleep 3
if grep -q "Subscribed:" test_logs/ctp_e2e.log; then
    echo -e "${GREEN}✓${NC} Instruments subscribed"
    grep "Subscribed:" test_logs/ctp_e2e.log
else
    echo -e "${YELLOW}⚠${NC}  No subscription confirmation found"
fi
echo ""

# 监控行情数据接收（30秒）
echo "Step 7: Monitoring market data reception (30 seconds)..."
echo "Waiting for market data..."

INITIAL_COUNT=0
for i in {1..30}; do
    if [ $i -eq 15 ]; then
        # 15秒后检查统计
        if grep -q "Stats:" test_logs/ctp_e2e.log; then
            INITIAL_COUNT=$(grep "Stats:" test_logs/ctp_e2e.log | tail -1 | grep -oP 'Count=\K[0-9]+' || echo 0)
            echo "Market data count at 15s: $INITIAL_COUNT"
        fi
    fi
    sleep 1
done
echo ""

# 检查最终统计
echo "Step 8: Checking final statistics..."
if grep -q "Stats:" test_logs/ctp_e2e.log; then
    FINAL_STATS=$(grep "Stats:" test_logs/ctp_e2e.log | tail -1)
    echo "$FINAL_STATS"
    
    FINAL_COUNT=$(echo "$FINAL_STATS" | grep -oP 'Count=\K[0-9]+' || echo 0)
    DROPPED=$(echo "$FINAL_STATS" | grep -oP 'Dropped=\K[0-9]+' || echo 0)
    
    if [ "$FINAL_COUNT" -gt 0 ]; then
        echo -e "${GREEN}✓${NC} Market data received: $FINAL_COUNT messages"
        echo -e "${GREEN}✓${NC} Dropped messages: $DROPPED"
    else
        echo -e "${YELLOW}⚠${NC}  No market data received (might be outside trading hours)"
    fi
else
    echo -e "${YELLOW}⚠${NC}  No statistics found"
fi
echo ""

# 检查共享内存队列
echo "Step 9: Checking shared memory queue..."
if ipcs -m | grep -q md_queue; then
    echo -e "${GREEN}✓${NC} Shared memory queue exists"
    ipcs -m | grep -A 1 "key"
else
    echo -e "${YELLOW}⚠${NC}  Shared memory queue not found"
fi
echo ""

# 测试总结
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""

PASSED=0
TOTAL=5

# 1. 连接测试
if grep -q "Connected to front server" test_logs/ctp_e2e.log; then
    echo -e "${GREEN}✓${NC} 1. CTP connection: PASSED"
    PASSED=$((PASSED+1))
else
    echo -e "${RED}✗${NC} 1. CTP connection: FAILED"
fi

# 2. 登录测试
if grep -q "Login successful" test_logs/ctp_e2e.log; then
    echo -e "${GREEN}✓${NC} 2. CTP login: PASSED"
    PASSED=$((PASSED+1))
else
    echo -e "${RED}✗${NC} 2. CTP login: FAILED"
fi

# 3. 订阅测试
if grep -q "Subscribed:" test_logs/ctp_e2e.log; then
    echo -e "${GREEN}✓${NC} 3. Instrument subscription: PASSED"
    PASSED=$((PASSED+1))
else
    echo -e "${RED}✗${NC} 3. Instrument subscription: FAILED"
fi

# 4. 共享内存测试
if grep -q "Shared memory queue opened" test_logs/ctp_e2e.log; then
    echo -e "${GREEN}✓${NC} 4. Shared memory queue: PASSED"
    PASSED=$((PASSED+1))
else
    echo -e "${RED}✗${NC} 4. Shared memory queue: FAILED"
fi

# 5. 行情数据测试
if [ "$FINAL_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓${NC} 5. Market data reception: PASSED ($FINAL_COUNT messages)"
    PASSED=$((PASSED+1))
else
    echo -e "${YELLOW}⚠${NC}  5. Market data reception: NO DATA (outside trading hours?)"
fi

echo ""
echo "Result: $PASSED/$TOTAL tests passed"
echo ""

# 显示日志位置
echo "Full log available at: test_logs/ctp_e2e.log"
echo ""

if [ $PASSED -eq $TOTAL ]; then
    echo -e "${GREEN}=========================================="
    echo "ALL TESTS PASSED!"
    echo -e "==========================================${NC}"
    exit 0
elif [ $PASSED -ge 4 ]; then
    echo -e "${YELLOW}=========================================="
    echo "MOSTLY PASSED (outside trading hours?)"
    echo -e "==========================================${NC}"
    exit 0
else
    echo -e "${RED}=========================================="
    echo "SOME TESTS FAILED"
    echo -e "==========================================${NC}"
    exit 1
fi
