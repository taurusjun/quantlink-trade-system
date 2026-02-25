# Phase 2 Tasks: 应用框架层

## 2.1 Instrument 行情数据模型
- [x] 2.1.1 创建 `core/Instrument.java` — 20 档订单簿 + 合约属性 + 价格计算
- [x] 2.1.2 创建 `InstrumentTest.java` — 订单簿填充、价格计算测试

## 2.2 订单状态追踪
- [x] 2.2.1 创建 `core/OrderStats.java` — 订单状态结构 + Status/HitType 枚举
- [x] 2.2.2 创建 `OrderStatsTest.java` — 订单状态转换测试

## 2.3 配置框架
- [x] 2.3.1 创建 `core/ThresholdSet.java` — ~120 参数阈值集（C++ 默认值完全保留）
- [x] 2.3.2 创建 `core/SimConfig.java` — 每策略配置容器
- [x] 2.3.3 创建 `core/ConfigParams.java` — 全局配置单例
- [x] 2.3.4 创建 `ThresholdSetTest.java` — 默认值验证测试

## 2.4 CommonClient 回调分发
- [x] 2.4.1 创建 `core/CommonClient.java` — MD/ORS 回调分发 + 发单封装
- [x] 2.4.2 创建 `CommonClientTest.java` — MD 按 symbolID 路由、ORS 回调、发单接口测试

## 2.5 编译验证
- [x] 2.5.1 全量编译通过（`mvn compile`）
- [x] 2.5.2 全量测试通过（`mvn test`）
