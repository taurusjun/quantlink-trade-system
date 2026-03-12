# Review Feasibility Checker - Agent Memory

## Verified Bugs
- **counter_bridge.cpp TRADE_CONFIRM price bug**: Line 574 uses `order_info.price` (= `ctp_order->LimitPrice`, the order limit price), NOT the actual trade price. The correct trade price is available in `OnBrokerTradeCallback` via `TradeInfo.price` (= `ctp_trade->Price`), but that callback only logs and does not generate a ResponseMsg. This is a confirmed P0 bug.

## Parameter Ranges (ag pair strategy 92201)
- Current: BEGIN_PLACE=6, LONG_PLACE=10, SHORT_PLACE=1.5, MAX_SIZE=30, SIZE=1
- STOP_LOSS/MAX_LOSS/UPNL_LOSS = 1000000 (effectively disabled)
- ALPHA=0.0000240672, AVG_SPREAD_AWAY=110

## daily_init File Locations
- Active (deploy_java): `/deploy_java/live/data/daily_init.92201` -- ytd1=16, ytd2=-16 (current correct)
- Template (data_new): `/data_new/live/data/daily_init.92201` -- ytd1=-70, ytd2=70 (stale/old)
- Multiple other locations exist (deploy_new, deploy, data, etc.) -- potential confusion source

## Key Architecture Facts
- OnBrokerOrderCallback generates TRADE_CONFIRM for PARTIAL_FILLED/FILLED status
- OnBrokerTradeCallback (from CTP OnRtnTrade) only logs, does NOT write to MWMR response queue
- ag2608 is a far-month contract with known low liquidity on SHFE
- ag contracts: tick size = 1 yuan, multiplier = 15 kg/lot
