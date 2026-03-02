## 1. 策略层订单生命周期日志（Java）

- [x] 1.1 ExecutionStrategy.sendNewOrder() 添加发单 log.info：`[ORDER-NEW] symbol side price qty orderID`
- [x] 1.2 ExecutionStrategy.sendCancelOrder() 添加撤单请求 log.info：`[ORDER-CANCEL] orderID symbol`
- [x] 1.3 ExecutionStrategy.processTrade() 添加成交 log.info：`[TRADE] symbol side price qty orderID cumQty remainQty`
- [x] 1.4 ExecutionStrategy.processCancelConfirm() 添加撤单确认 log.info：`[CANCEL-CONFIRM] orderID symbol`
- [x] 1.5 ExecutionStrategy.processNewReject() 添加新单拒绝 log.warning：`[ORDER-REJECT] orderID symbol`
- [x] 1.6 ExecutionStrategy.processCancelReject() 添加撤单拒绝 log.warning：`[CANCEL-REJECT] orderID symbol`
- [x] 1.7 ExecutionStrategy.checkSquareoff() 添加状态变化 log.info：onFlat/onExit/active 变化时输出 `[STATE] flag: old -> new, reason=...`

## 2. PairwiseArbStrategy 配对级日志（Java）

- [x] 2.1 PairwiseArbStrategy.sendAggressiveOrder() 添加追单 log.info：`[AGG-ORDER] leg side price qty aggRepeat`
- [x] 2.2 PairwiseArbStrategy.orsCallBack() 添加回报路由 log.info：`[PAIR-ORS] orderID routed=firstStrat/secondStrat type`
- [x] 2.3 PairwiseArbStrategy.handleSquareoff() 添加停用 log.warning：`[PAIR-EXIT] active=false avgSpread ytd1 ytd2`

## 3. avgSpreadRatio 跨天漂移修复（Java）

- [x] 3.1 PairwiseArbStrategy.mdCallBack() 修改 AVG_SPREAD_AWAY 检查：active=false 时仅 log.warning 不触发 exit
- [x] 3.2 PairwiseArbStrategy.handleSquareON() 添加漂移检测日志：|oldAvg - liveSpread| > AVG_SPREAD_AWAY 时输出 `[AVG-SPREAD-DRIFT]` warning

## 4. counter_bridge HTTP 查询异步化（C++）

- [x] 4.1 ctp_td_plugin: 添加 GetCachedAccount() 非阻塞方法（类比已有的 GetCachedPositions()）
- [x] 4.2 counter_bridge HandleAccount(): 改为调用缓存读取代替 QueryAccount()，附加 last_updated/stale 字段
- [x] 4.3 counter_bridge: 添加后台查询线程，每 10s 调用 QueryAccount() 刷新缓存
- [x] 4.4 counter_bridge: 后台线程在进程退出时正常停止

## 5. 测试验证

- [x] 5.1 Java 单元测试：ExecutionStrategy 日志输出验证（mock logger 或检查 log handler）
- [x] 5.2 Java 单元测试：PairwiseArbStrategy AVG_SPREAD_AWAY inactive 跳过 exit 验证
- [x] 5.3 Java mvn test 全量回归通过（249 tests, 0 failures）
