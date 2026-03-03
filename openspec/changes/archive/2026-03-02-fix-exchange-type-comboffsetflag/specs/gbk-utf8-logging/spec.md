## ADDED Requirements

### Requirement: CTP plugin SHALL convert GBK error messages to UTF-8 before logging

`ctp_td_plugin.cpp` 中所有输出 `pRspInfo->ErrorMsg` 的位置 MUST 将 GBK 编码转为 UTF-8 后再输出，确保在 UTF-8 终端下日志可读。

#### Scenario: 订单被拒时错误消息可读
- **WHEN** CTP 返回 ErrorID=51（平仓位不足）
- **THEN** 日志 SHALL 显示 "平仓位不足" 而非 GBK 乱码 "ƽ���λ����"

#### Scenario: 登录失败错误消息可读
- **WHEN** CTP 认证或登录失败返回中文错误消息
- **THEN** 日志 SHALL 以 UTF-8 编码输出完整中文错误信息

#### Scenario: iconv 转码失败时 graceful fallback
- **WHEN** GBK→UTF-8 转码因任何原因失败
- **THEN** SHALL 输出原始 GBK 字符串（与当前行为一致，不会更差）
