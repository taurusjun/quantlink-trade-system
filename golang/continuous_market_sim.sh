#!/bin/bash
# Continuous market data simulator for QuantlinkTrader
# Simulates realistic price movements for ag2502 and ag2504

API_URL="http://localhost:9201"

echo "========================================="
echo "Continuous Market Data Simulator"
echo "========================================="
echo "Press Ctrl+C to stop"
echo ""

# 基准价格
BASE_PRICE_2502=5000
BASE_PRICE_2504=5010

# 价差 (2504 - 2502)，我们会让价差波动
SPREAD=10

# 迭代计数器
ITERATION=0

# 信号生成逻辑：当价差偏离正常范围时，模拟交易信号
# 正常价差范围：8-12
# 当价差 > 15 或 < 5 时，应该触发交易条件

while true; do
  ITERATION=$((ITERATION + 1))

  # 生成随机价格波动 (-5 到 +5)
  RANDOM_MOVE_2502=$((RANDOM % 11 - 5))
  RANDOM_MOVE_2504=$((RANDOM % 11 - 5))

  # 每10次迭代，让价差有一个更大的偏离（模拟交易机会）
  if [ $((ITERATION % 10)) -eq 0 ]; then
    echo ""
    echo ">>> Simulating trading opportunity (spread deviation) <<<"
    # 让价差扩大或缩小
    if [ $((ITERATION % 20)) -eq 0 ]; then
      # 价差扩大
      RANDOM_MOVE_2502=$((RANDOM_MOVE_2502 - 8))
      RANDOM_MOVE_2504=$((RANDOM_MOVE_2504 + 8))
      echo ">>> Widening spread <<<"
    else
      # 价差缩小
      RANDOM_MOVE_2502=$((RANDOM_MOVE_2502 + 8))
      RANDOM_MOVE_2504=$((RANDOM_MOVE_2504 - 8))
      echo ">>> Narrowing spread <<<"
    fi
  fi

  # 计算新价格
  PRICE_2502=$((BASE_PRICE_2502 + RANDOM_MOVE_2502))
  PRICE_2504=$((BASE_PRICE_2504 + RANDOM_MOVE_2504))

  # 计算价差
  CURRENT_SPREAD=$((PRICE_2504 - PRICE_2502))

  # 打印状态
  echo "[Iter $ITERATION] ag2502: $PRICE_2502 | ag2504: $PRICE_2504 | Spread: $CURRENT_SPREAD"

  # 发送 ag2502 市场数据
  curl -s -X POST "${API_URL}/api/v1/test-market-data" \
    -H "Content-Type: application/json" \
    -d "{
      \"symbol\": \"ag2502\",
      \"exchange\": \"SHFE\",
      \"bid_price\": [$PRICE_2502, $((PRICE_2502 - 1)), $((PRICE_2502 - 2))],
      \"ask_price\": [$((PRICE_2502 + 2)), $((PRICE_2502 + 3)), $((PRICE_2502 + 4))],
      \"bid_qty\": [100, 80, 60],
      \"ask_qty\": [100, 80, 60]
    }" > /dev/null

  # 发送 ag2504 市场数据
  curl -s -X POST "${API_URL}/api/v1/test-market-data" \
    -H "Content-Type: application/json" \
    -d "{
      \"symbol\": \"ag2504\",
      \"exchange\": \"SHFE\",
      \"bid_price\": [$PRICE_2504, $((PRICE_2504 - 1)), $((PRICE_2504 - 2))],
      \"ask_price\": [$((PRICE_2504 + 2)), $((PRICE_2504 + 3)), $((PRICE_2504 + 4))],
      \"bid_qty\": [100, 80, 60],
      \"ask_qty\": [100, 80, 60]
    }" > /dev/null

  # 每5次迭代，查询一次状态
  if [ $((ITERATION % 5)) -eq 0 ]; then
    STATUS=$(curl -s "${API_URL}/api/v1/strategy/status" | jq -r '.data | "Conditions: \(.conditions_met) | Eligible: \(.eligible) | Signal: \(.signal_strength)"')
    echo "  └─ Status: $STATUS"
  fi

  # 等待1秒
  sleep 1
done
