#!/bin/bash

# run_backtest.sh - 单次回测脚本
# 用法: ./run_backtest.sh [config_file] [start_date] [end_date]

set -e

# 默认参数
CONFIG_FILE="${1:-./config/backtest.yaml}"
START_DATE="${2}"
END_DATE="${3}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}QuantLink 回测系统${NC}"
echo -e "${GREEN}========================================${NC}"

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}错误: 配置文件不存在: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}配置文件:${NC} $CONFIG_FILE"

# 检查 backtest 命令是否存在
if [ ! -f "./bin/backtest" ]; then
    echo -e "${YELLOW}编译 backtest 工具...${NC}"
    cd golang
    go build -o ../bin/backtest cmd/backtest/main.go
    cd ..
    echo -e "${GREEN}✓ 编译完成${NC}"
fi

# 检查 NATS 是否运行
if ! pgrep -x "nats-server" > /dev/null; then
    echo -e "${YELLOW}警告: NATS server 未运行${NC}"
    echo -e "${YELLOW}启动 NATS server...${NC}"
    nats-server &
    sleep 2
    echo -e "${GREEN}✓ NATS server 已启动${NC}"
fi

# 构建命令
CMD="./bin/backtest -config $CONFIG_FILE"

if [ -n "$START_DATE" ]; then
    CMD="$CMD -start-date $START_DATE"
fi

if [ -n "$END_DATE" ]; then
    CMD="$CMD -end-date $END_DATE"
fi

# 执行回测
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}开始回测...${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

$CMD

# 检查退出码
if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}✓ 回测完成${NC}"
    echo -e "${GREEN}========================================${NC}"
else
    echo ""
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}✗ 回测失败${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi
