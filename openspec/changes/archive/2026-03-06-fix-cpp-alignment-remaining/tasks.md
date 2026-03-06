## 1. SetCheckCancelQuantity（HIGH）

- [x] 1.1 在 ExecutionStrategy.java 添加 `checkCancelQuantity` boolean 字段
- [x] 1.2 实现 `setCheckCancelQuantity()` 方法：FORTS/KRX/SFE/SGX → true，其他 → false
- [x] 1.3 在构造函数末尾调用 setCheckCancelQuantity()

## 2. FillMsg（HIGH）

- [x] 2.1 读取 C++ ExecutionStrategy::FillMsg() 完整代码
- [x] 2.2 在 ExecutionStrategy.java 添加 TBRequest 内部类 + `fillMsg()` 方法
- [x] 2.3 FillMsg 作为独立方法，sendNewOrder 逻辑不变（FillMsg 供 ExtraStrategy 复用）

## 3. SendInfraReqUpdate（HIGH）

- [x] 3.1 在 CommonClient.java 添加 `sendInfraReqUpdate()` 空壳方法
- [x] 3.2 添加注释说明依赖 CommonBook（中国期货不启用）

## 4. SimConfig currDate（MEDIUM）

- [x] 4.1 在 SimConfig.java 添加 `currDate` 字段
- [x] 4.2 在 initDateConfigEpoch() 中初始化为进程启动日期（YYYYMMDD）

## 5. lastTradePx 条件更新（MEDIUM）

- [x] 5.1 确认 SHM MarketUpdateNew 中 updateType 字段偏移 (Types.MDD_UPDATE_TYPE_VH, offset 713)
- [x] 5.2 修改 Instrument.fillOrderBook() 添加 updateType 条件判断
- [x] 5.3 仅 TRADE/TRADE_IMPLIED/TRADE_INFO 类型更新 lastTradePx/lastTradeQty

## 6. Connector OrderID 溢出防护（MEDIUM）

- [x] 6.1 在 Connector.java getUniqueOrderNumber() 添加溢出检测
- [x] 6.2 溢出时记录 SEVERE 日志告警并返回 -1

## 7. Instrument tickType 枚举（MEDIUM）

- [x] 7.1 在 Instrument.java 添加 TickType 枚举和 tickType 字段
- [x] 7.2 在 fillOrderBook() 后根据 updateType 设置 tickType

## 8. 验证

- [x] 8.1 编译通过 build_deploy_java.sh
- [x] 8.2 模拟测试运行正常
