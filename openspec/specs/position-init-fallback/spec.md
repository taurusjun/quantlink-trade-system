## ADDED Requirements

### Requirement: counter_bridge SHALL fallback to GetCachedPositions when QueryPositions returns empty

当 `QueryPositions()` 返回空结果（0 个持仓）时，counter_bridge MUST 尝试调用 `GetCachedPositions()` 作为 fallback 来初始化 `g_mapContractPos`。

#### Scenario: QueryPositions 返回空但 GetCachedPositions 有数据
- **WHEN** counter_bridge 启动，`QueryPositions()` 因 CTP 限频返回空
- **AND** CTP 登录回调已通过 `UpdatePositionFromCTP()` 填充了 `m_positions`
- **THEN** SHALL 调用 `GetCachedPositions()` 获取持仓数据填充 `g_mapContractPos`

#### Scenario: 两个数据源都为空
- **WHEN** `QueryPositions()` 和 `GetCachedPositions()` 都返回空
- **THEN** SHALL 输出日志 "No positions loaded — all orders will default to OPEN"（与当前行为一致）

#### Scenario: QueryPositions 成功返回数据
- **WHEN** `QueryPositions()` 正常返回非空持仓
- **THEN** SHALL 使用 QueryPositions 的结果，不调用 GetCachedPositions（保持现有行为）
