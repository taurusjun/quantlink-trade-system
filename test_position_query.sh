#!/bin/bash
# CTP持仓查询测试脚本

echo "=========================================="
echo "CTP持仓查询工具测试"
echo "=========================================="
echo

# 1. 查询所有持仓
echo "1. 查询所有持仓"
echo "----------------------------------------"
./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml

echo
echo

# 2. 查询指定合约（ag2603）
echo "2. 查询ag2603持仓"
echo "----------------------------------------"
./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml ag2603

echo
echo

# 3. 平仓示例（如果有持仓）
echo "3. 平仓ag2603（如果有持仓）"
echo "----------------------------------------"
echo "运行命令: ./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml ag2603 close"
echo "（暂不执行，仅演示）"

echo
echo "=========================================="
echo "测试完成"
echo "=========================================="
