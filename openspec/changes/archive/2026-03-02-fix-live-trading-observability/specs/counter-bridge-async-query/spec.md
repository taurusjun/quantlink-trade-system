## ADDED Requirements

### Requirement: HTTP Account 查询非阻塞
counter_bridge 的 HTTP GET /account 端点 SHALL 返回缓存的 account 数据，不直接调用 CTP 查询 API。

#### Scenario: 正常查询
- **WHEN** HTTP 客户端请求 GET /account
- **THEN** 在 < 100ms 内返回缓存的 account JSON，包含 balance、available、frozenMargin、commission、closeProfit、positionProfit、last_updated 字段

#### Scenario: 缓存尚未初始化
- **WHEN** HTTP 客户端请求 GET /account 且后台查询尚未完成首次刷新
- **THEN** 返回 HTTP 503 + JSON `{"error": "Account data not yet available", "retry_after": 10}`

#### Scenario: 缓存过期
- **WHEN** HTTP 客户端请求 GET /account 且缓存数据 last_updated 超过 30s
- **THEN** 返回 account JSON 但附加 `"stale": true` 标志

### Requirement: 后台 Account 缓存刷新
counter_bridge SHALL 在后台线程定期刷新 CTP account 缓存。

#### Scenario: 正常刷新周期
- **WHEN** counter_bridge 启动并成功连接 CTP
- **THEN** 后台线程每 10 秒调用一次 QueryAccount()，更新缓存

#### Scenario: 查询超时
- **WHEN** 后台 QueryAccount() 调用超时（5s 无响应）
- **THEN** 保留旧缓存数据不变，输出 warning 日志，下一周期重试

#### Scenario: 进程退出
- **WHEN** counter_bridge 收到停止信号
- **THEN** 后台刷新线程正常退出，不阻塞进程关闭

### Requirement: CTP 插件提供非阻塞 Account 访问
ctp_td_plugin SHALL 提供 GetCachedAccount() 方法，返回最近一次成功查询的 account 数据。

#### Scenario: 缓存可用
- **WHEN** 调用 GetCachedAccount() 且已有成功查询结果
- **THEN** 返回 true + 填充 account 结构体，不获取 m_query_mutex

#### Scenario: 缓存不可用
- **WHEN** 调用 GetCachedAccount() 且从未成功查询
- **THEN** 返回 false
