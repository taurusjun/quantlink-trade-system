# Tasks: symbolID 路由对齐 C++ + CTP endPkt 修复

- [x] md_shm_feeder: 新增 BuildSymbolIDMap 按字母排序分配 symbolID
- [x] md_shm_feeder: simulator 模式调用 BuildSymbolIDMap 并写入 m_symbolID
- [x] md_shm_feeder: CTP 模式调用 BuildSymbolIDMap 并写入 m_symbolID
- [x] md_shm_feeder: CTP 模式 m_endPkt = 0 修复
- [x] ConfigParams: 新增 simConfigList[] 数组字段
- [x] SimConfig: 新增 instruList[] 数组字段
- [x] TraderMain: 构建 symbolID 排序映射数组
- [x] CommonClient: sendINDUpdate 改为 symbolID 数组索引路由
- [x] CommonClientTest: 更新测试适配 symbolID 路由
- [x] 清理 Connector/CommonClient/TraderMain 中的临时 debug 代码
- [x] build_deploy_new.sh: 移除 Java 相关代码（变量、参数、编译块、heredoc）
- [x] 模拟测试通过: ag2603/ag2605 行情正确分发
- [x] CTP 实盘测试通过: ag2603/ag2605 实盘行情正确
