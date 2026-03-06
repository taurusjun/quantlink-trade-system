## 1. baseNameToSymbol 年份 rollover（HIGH）

- [ ] 1.1 读取 C++ FillChinaFields2() 原代码确认完整逻辑
- [ ] 1.2 修改 ConfigParser.baseNameToSymbol() 添加 month < currentMonth 时 year+1 推断
- [ ] 1.3 更新 TraderMain 调用（移除或调整 yearPrefix 传参）
- [ ] 1.4 更新单元测试覆盖 rollover 场景

## 2. fillOrderBook 缺失字段（MEDIUM）

- [ ] 2.1 读取 C++ CopyOrderBook() 和 FillOrderBook() 原代码
- [ ] 2.2 确认 MarketUpdateNew SHM 中 bidOrderCount/askOrderCount/validBids/validAsks 偏移
- [ ] 2.3 修改 Instrument.fillOrderBook() 补齐字段读取
- [ ] 2.4 添加 updateIndicators=true 标志设置
- [ ] 2.5 添加 lastTradeQty 读取

## 3. sendNewOrder 缺失字段（MEDIUM）

- [ ] 3.1 读取 C++ CommonClient::SendNewOrder() 原代码
- [ ] 3.2 确认 RequestMsg SHM 中各字段偏移
- [ ] 3.3 修改 CommonClient.sendNewOrder() 补齐 Token/Product/AccountID 等字段
- [ ] 3.4 添加 Duration FAK/DAY 条件逻辑
- [ ] 3.5 补齐 Contract_Description 子字段

## 4. m_sendMail 字段补齐（LOW）

- [ ] 4.1 在 ExecutionStrategy.java 添加 sendMail boolean 字段

## 5. 验证

- [ ] 5.1 编译通过 build_deploy_java.sh
- [ ] 5.2 与 C++ 原代码逐行对照验证
