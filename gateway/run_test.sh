#!/bin/bash
cd /Users/user/PWorks/RD/quantlink-trade-system/gateway
./test_ctp_login 142266 "t0t0tity_wJ" &
PID=$!
sleep 10
kill -9 $PID 2>/dev/null
wait $PID 2>/dev/null
rm -rf ctp_test_flow
