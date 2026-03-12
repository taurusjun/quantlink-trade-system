# Known Patterns and Issues

## Bridge Price vs CTP Fill Price Discrepancy
**Severity: CRITICAL**
**Found: 2026-03-11**

The counter_bridge sends `type=4` (TRADE_CONFIRM) responses with the ORDER's limit price, NOT the actual CTP fill price. This is a systematic bug.

Evidence:
- OID=0: Bridge sends `price=22440`, CTP filled at `22391` (diff=49)
- OID=2: Bridge sends `price=22438`, CTP filled at `22399` (diff=39)
- OID=3: Bridge sends `price=22425`, CTP filled at `22399` (diff=26)
- OID=126: Bridge sends `price=22418`, CTP filled at `22422` (diff=-4, SELL side)
- OID=127: Bridge sends `price=22333`, CTP filled at `22339` (diff=-6, SELL side)

Pattern: For BUY orders the Bridge price is HIGHER than actual (order limit price), for SELL orders it can go either way. The Bridge is clearly using the order's submitted price rather than the actual fill price from OnRtnTrade.

Impact: Strategy's PNL calculation is WRONG. Strategy thinks BUY fills are more expensive than reality (overestimates cost) and may miscalculate spreads.

## Delayed Fill on OID=1
**Severity: HIGH**
**Found: 2026-03-11**

OID=1 (ag2608 SELL 1@22380) was sent at 13:30:14 but only filled at 14:13:03 (43 minutes later). The CTP order ref `1--49650651-000000000003` remained in `status=3` (queued) for the entire duration. During this time, the strategy continued trading, creating a large one-sided exposure on ag2606.

This appears to be a CTP exchange queuing issue, not a system bug. However, the strategy should have a mechanism to detect and handle stale unfilled orders.

## CTP ErrorID=26 (Cancel Failed)
**Severity: Low (expected)**
These are "order already fully traded, cannot cancel" errors. They occur when a cancel request races with a fill. 10 occurrences on 2026-03-11 is normal for an active strategy.
