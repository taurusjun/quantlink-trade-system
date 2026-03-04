## 1. 修复 ConvertPosition

- [x] 1.1 修改 `ctp_td_plugin.cpp` 的 `ConvertPosition()`: 将 `pos_info.yesterday_volume = ctp_pos->YdPosition` 改为 `pos_info.yesterday_volume = std::max(0, ctp_pos->Position - ctp_pos->TodayPosition)`
- [x] 1.2 在 `ConvertPosition()` 中增加调试日志：打印原始 CTP 字段 `Position`、`TodayPosition`、`YdPosition` 和计算后的 `yesterday_volume`

## 2. 验证

- [x] 2.1 编译 C++ gateway，确认无编译错误
- [x] 2.2 运行模拟测试（sim 模式），确认 g_mapContractPos 初始化正确且无 ErrorID 51
