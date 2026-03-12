# Trading Review Advisor Memory

## Critical Bugs Found

### Counter Bridge Price Bug (2026-03-11 confirmed)
- `counter_bridge.cpp` line 574: `resp.Price = order_info.price;`
- `order_info.price` comes from `OnRtnOrder` -> `ConvertOrder` -> `ctp_order->LimitPrice` (line 1098)
- This is the ORDER LIMIT PRICE, not the CTP actual trade price
- `OnBrokerTradeCallback` (line 613-617) only logs trade_info but does NOT update ResponseMsg
- The TRADE_CONFIRM response uses OrderInfo (from OnRtnOrder) not TradeInfo (from OnRtnTrade)
- Impact: Strategy PnL calculations are wrong; all fill prices reported to strategy are order prices
- File: `/Users/user/PWorks/RD/quantlink-trade-system/gateway/src/counter_bridge.cpp`

## Strategy 92201 Observations

### ag2606/ag2608 Pair (2026-03-11)
- Remote month (ag2608) has severe liquidity issues: 36-43 min fill delays
- Aggressive orders only sent on leg1 (ag2606), leg2 relies on passive fills
- Daily init file: `/Users/user/PWorks/RD/quantlink-trade-system/data_new/live/data/daily_init.92201`
- Model file: `/Users/user/PWorks/RD/quantlink-trade-system/data_new/live/models/model.ag2606.ag2608.par.txt.92201`
- Day control: `/Users/user/PWorks/RD/quantlink-trade-system/data_new/live/controls/day/control.ag2606.ag2608.par.txt.92201`

### Key Parameter Values (2026-03-11)
- MAX_SIZE=30, SIZE=1, BEGIN_PLACE=6, LONG_PLACE=10, SHORT_PLACE=1.5
- BEGIN_REMOVE=4.5, LONG_REMOVE=8.5, SHORT_REMOVE=0
- CROSS=3, SUPPORTING_ORDERS=2, ALPHA=0.0000240672
- STOP_LOSS/MAX_LOSS/UPNL_LOSS all set to 1000000 (effectively disabled)
- AVG_SPREAD_AWAY=110

### Performance Pattern
- 56.8% fill rate (79/139), 70 cancels, 0 rejects
- Net PnL +70.80 (but unreliable due to price bug)
- Asymmetric fills: ag2606 BUY=42 vs SELL=27; ag2608 BUY=1 vs SELL=16
- avgSpread drifted -1.062 (-1.3%) during session

## File Locations
- Strategy logs: `deploy_new/nohup.out.92201`
- Counter bridge logs: `deploy_new/log/counter_bridge.YYYYMMDD.log`
- C++ strategy reference: `/Users/user/PWorks/RD/tbsrc/Strategies/PairwiseArbStrategy.cpp`
