## ADDED Requirements

### Requirement: daily_init 保存字段对齐 C++ SaveMatrix2

HandleSquareoff 保存 daily_init 时，`NetposYtd1` 字段 SHALL 使用 `NetposPass`（全部仓位），`Netpos2day1` 字段 SHALL 固定为 0。这与 C++ `SaveMatrix2` 的跨日重启语义一致：关机时全部仓位变成"昨仓"。

#### Scenario: HandleSquareoff 保存 daily_init 字段正确
- **WHEN** 策略持有 ytd=2, today=3 (total NetposPass=5), NetposAgg2=-3 的仓位，触发 HandleSquareoff
- **THEN** 保存的 daily_init 文件中 NetposYtd1=5, Netpos2day1=0, NetposAgg2=-3

### Requirement: shutdown 不重复保存 daily_init

main.go shutdown 流程 SHALL NOT 独立保存 daily_init。daily_init 的保存 SHALL 仅在 HandleSquareoff 内部执行，避免覆盖已保存的正确值。

#### Scenario: kill 进程不覆盖已保存的 daily_init
- **WHEN** AVG_SPREAD_AWAY 触发 HandleSquareoff 并保存 daily_init 后，进程收到 SIGTERM
- **THEN** shutdown 流程不会再次写入 daily_init 文件
