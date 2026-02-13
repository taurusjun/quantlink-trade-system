package strategy

import (
	"tbsrc-golang/pkg/shm"
)

// Strategy 策略接口
// Phase 3 将实现 PairwiseArbStrategy
// 参考: tbsrc/Strategies/include/ExecutionStrategy.h (虚函数)
type Strategy interface {
	// SendOrder 发送交易指令（具体策略实现）
	SendOrder()

	// MDCallBack 行情更新回调
	MDCallBack(md *shm.MarketUpdateNew)

	// ORSCallBack ORS 响应回调
	ORSCallBack(resp *shm.ResponseMsg)

	// HandleSquareoff 处理平仓
	HandleSquareoff()

	// IsActive 返回策略是否激活
	IsActive() bool

	// SetActive 设置策略激活状态
	SetActive(active bool)
}
