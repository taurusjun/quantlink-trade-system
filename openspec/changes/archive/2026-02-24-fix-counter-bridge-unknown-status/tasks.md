## 1. counter_bridge 过滤中间状态

- [x] 1.1 在 `OnBrokerOrderCallback` 开头对 `UNKNOWN` 和 `SUBMITTING` 状态 early return，添加日志记录被过滤的回调
- [x] 1.2 编译验证 counter_bridge

## 2. 回退 Go 侧 workaround

- [x] 2.1 回退 `ors_callback.go` 中 processNewReject 的修改，恢复为调用 `RemoveOrder`（与 C++ 一致）
- [x] 2.2 回退 `ors_callback.go` 中 ProcessORSResponse 的 `wasRejected` / `restoreAfterReject` 逻辑
- [x] 2.3 移除 `order_manager.go` 中的 `CleanupRejectedOrders` 方法
- [x] 2.4 移除 `pairwise_arb.go` handleSquareoffLocked 中对 CleanupRejectedOrders 的调用（如已接线）

## 3. 验证

- [x] 3.1 运行 `go test -race ./pkg/execution/...` 和 `go test -race ./pkg/strategy/...` 确认测试通过
- [x] 3.2 运行 `go build ./cmd/trader/...` 确认编译通过
