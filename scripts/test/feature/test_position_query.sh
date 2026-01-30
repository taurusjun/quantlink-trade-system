#!/bin/bash
# 持仓查询功能测试脚本
# Test Position Query Feature

set -e

echo "════════════════════════════════════════════════════════════"
echo "持仓查询功能测试"
echo "════════════════════════════════════════════════════════════"

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查Counter Bridge是否运行
echo -e "\n${YELLOW}[1/4]${NC} 检查Counter Bridge是否运行..."
if pgrep -f "counter_bridge" > /dev/null; then
    echo -e "${GREEN}✓${NC} Counter Bridge正在运行"
else
    echo -e "${RED}✗${NC} Counter Bridge未运行"
    echo "请先启动Counter Bridge:"
    echo "  ./gateway/build/counter_bridge ctp:config/ctp/ctp_td.yaml"
    exit 1
fi

# 测试HTTP健康检查
echo -e "\n${YELLOW}[2/4]${NC} 测试Counter Bridge HTTP服务..."
if curl -s http://localhost:8080/health | grep -q "ok"; then
    echo -e "${GREEN}✓${NC} Counter Bridge HTTP服务正常"
else
    echo -e "${RED}✗${NC} Counter Bridge HTTP服务异常"
    exit 1
fi

# 查询持仓
echo -e "\n${YELLOW}[3/4]${NC} 查询持仓信息..."
echo "查询URL: http://localhost:8080/positions"
response=$(curl -s http://localhost:8080/positions)
echo "$response" | jq '.' || echo "$response"

# 检查Trader是否运行
echo -e "\n${YELLOW}[4/4]${NC} 检查Trader是否运行..."
if pgrep -f "trader.*config" > /dev/null; then
    echo -e "${GREEN}✓${NC} Trader正在运行"

    echo -e "\n${YELLOW}测试Trader API...${NC}"

    # 测试/api/v1/positions
    echo -e "\n查询Trader持仓: http://localhost:9201/api/v1/positions"
    curl -s http://localhost:9201/api/v1/positions | jq '.' || echo "API未响应或JSON格式错误"

    # 测试/api/v1/positions/summary
    echo -e "\n查询持仓摘要: http://localhost:9201/api/v1/positions/summary"
    curl -s http://localhost:9201/api/v1/positions/summary | jq '.' || echo "API未响应或JSON格式错误"
else
    echo -e "${YELLOW}⚠${NC} Trader未运行（可选）"
    echo "如需测试Trader API，请启动Trader:"
    echo "  ./bin/trader -config config/trader.test.yaml"
fi

echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}测试完成！${NC}"
echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
echo -e "\n可用的API endpoints:"
echo "  - Counter Bridge持仓查询: http://localhost:8080/positions"
echo "  - Trader持仓查询:         http://localhost:9201/api/v1/positions"
echo "  - Trader持仓摘要:         http://localhost:9201/api/v1/positions/summary"
echo ""
