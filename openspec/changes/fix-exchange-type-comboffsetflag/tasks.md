# Tasks

## Bug 1: Java Exchange_Type 缺失修复

- [x] 1.1 CfgConfig 添加 `parseExchangeType(String)` 静态方法 — 映射交易所字符串到字节常量（C++ CommonClient.cpp:850-901）
- [x] 1.2 CommonClient 添加 `exchangeType` 字段和 `setExchangeType(byte)` setter（C++ CommonClient.h:122）
- [x] 1.3 CommonClient.sendNewOrder() 添加 `Types.REQ_EXCHANGE_TYPE_VH.set(reqMsg, 0L, exchangeType)`（C++ FillReqInfo L1117）
- [x] 1.4 CommonClient.sendModifyOrder() 添加 Exchange_Type 设置
- [x] 1.5 CommonClient.sendCancelOrder() 添加 Exchange_Type 设置
- [x] 1.6 TraderMain.init() 中创建 CommonClient 后调用 `client.setExchangeType(CfgConfig.parseExchangeType(cfgConfig.exchanges))`

## Bug 2: counter_bridge 持仓初始化 fallback

- [x] 2.1 counter_bridge.cpp L982-1023: QueryPositions 返回空时调用 GetCachedPositions 作为 fallback，填充 g_mapContractPos

## Bug 3: CTP GBK→UTF-8 转码

- [x] 3.1 ctp_td_plugin.cpp 和 ctp_md_plugin.cpp 添加 GbkToUtf8() 工具函数（使用 iconv）
- [x] 3.2 所有 pRspInfo->ErrorMsg 输出位置使用 GbkToUtf8() 转码

## 验证

- [x] 4.1 Java 单元测试通过（249 tests, 0 failures）
- [x] 4.2 C++ 编译通过（build_deploy_java.sh 成功）
- [x] 4.3 build_deploy_java.sh --mode live 部署成功
- [x] 4.4 确认 daily_init 持仓为 0（ag_F_6_SFE/ag_F_8_SFE, ytd=0/0）
