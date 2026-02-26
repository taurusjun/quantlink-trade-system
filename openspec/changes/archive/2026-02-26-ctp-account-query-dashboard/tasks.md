## 1. C++ counter_bridge 改造

- [x] 1.1 counter_bridge HTTP 端口从 8080 改为 8082
- [x] 1.2 新增 `HandleAccount()` — 遍历 `g_brokers` 找第一个已登录插件调用 `QueryAccount()`
- [x] 1.3 注册 `GET /account` 端点，保留 `/simulator/account` 兼容
- [x] 1.4 更新状态 banner 显示 8082 端口信息

## 2. Java OverviewSnapshot 数据模型

- [x] 2.1 新增 `AccountRow` 内部类（broker, accountId, totalAsset, availCash, margin, riskPercent, closeProfit, positionProfit, commission）
- [x] 2.2 新增 `accounts` 列表字段
- [x] 2.3 新增重载 `aggregate()` 方法接受 `List<AccountRow>` 参数

## 3. Java OverviewServer 资金查询

- [x] 3.1 新增 `ScheduledExecutorService` 每 10 秒查询 `localhost:8082/account`
- [x] 3.2 实现 `queryCounterBridgeAccount()` — HTTP GET + JSON 解析 + AccountRow 构建
- [x] 3.3 查询结果缓存到 `cachedAccounts`，合并到所有 aggregate 调用
- [x] 3.4 资金更新后触发 `broadcastOverview()` 推送前端
- [x] 3.5 `stop()` 时关闭 accountQueryExecutor

## 4. 前端 Account Table

- [x] 4.1 Account Table 改为 Vue 绑定 `overview.accounts`
- [x] 4.2 列: Broker | AccountID | TotalAsset | AvailCash | Margin | Risk(%) | ClosePnL | PosPnL
- [x] 4.3 新增 `fmtMoney()` 格式化函数
- [x] 4.4 Risk > 50% 红色高亮
- [x] 4.5 无数据时显示 "Waiting for counter_bridge..."

## 5. 部署脚本

- [x] 5.1 `build_deploy_java.sh` 更新 counter_bridge 启动注释（端口 8082）

## 6. 验证

- [x] 6.1 Java 编译通过（`mvn compile`）
- [x] 6.2 Java 测试通过（`mvn test` — 185 tests, 0 failures）
- [x] 6.3 C++ 编译通过（`cmake && make counter_bridge`）
