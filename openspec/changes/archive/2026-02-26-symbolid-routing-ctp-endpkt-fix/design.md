# Design: symbolID 路由对齐 C++ + CTP endPkt 修复

## 技术方案

### 1. md_shm_feeder symbolID 映射

- 新增全局 `g_symbol_id_map` (std::map<string, uint16_t>)
- `BuildSymbolIDMap()`: 对 symbols 排序后按 0,1,2... 分配 ID
- 在 simulator 和 CTP 两个入口都调用此函数
- 写入 SHM 时设置 `md.m_symbolID = g_symbol_id_map[symbol]`

### 2. Java symbolID 路由

- ConfigParams 新增 `simConfigList[]` 数组（替代 Map 查找）
- SimConfig 新增 `instruList[]` 数组（替代 Map 查找）
- TraderMain 初始化时构建排序映射
- CommonClient.sendINDUpdate 读取 `m_symbolID` 直接索引

### 3. CTP endPkt 修复

- `md_shm_feeder.cpp` line 430: `m_endPkt = 1` → `m_endPkt = 0`
- 与 simulator 模式 (line 261) 保持一致

### 4. 构建脚本分离

- `build_deploy_new.sh`: 移除所有 Java 相关代码（变量、参数、编译块、heredoc 脚本）
- Java 使用已有的 `build_deploy_java.sh` 独立部署

## 验证

- 模拟测试通过: ag2603/ag2605 行情正确分发
- CTP 实盘测试通过: ag2603 bid=22298, ag2605 bid=21914
- 185 单元测试通过
