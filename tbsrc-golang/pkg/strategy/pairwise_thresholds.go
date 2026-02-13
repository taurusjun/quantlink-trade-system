package strategy

// setThresholds PairwiseArb 专用的阈值设置
// 使用 NetposPass（被动持仓）进行线性插值，与通用 ExecutionState.SetThresholds 不同
// 参考: PairwiseArbStrategy.cpp:902-947
//
// C++ 逻辑:
//   1. maxPos = max(BID_MAX_SIZE, ASK_MAX_SIZE)（sendInLots 模式）
//   2. 根据 m_netpos_pass 线性插值 BidPlace/BidRemove/AskPlace/AskRemove
//   3. 同时设置独立 BidSize/BidMaxPos/AskSize/AskMaxPos
func (pas *PairwiseArbStrategy) setThresholds() {
	inst1 := pas.Inst1
	inst2 := pas.Inst2
	thold := pas.Thold1
	state := pas.Leg1.State

	// C++: 计算仓位大小参数
	// 参考: PairwiseArbStrategy.cpp:904-919
	if inst1.SendInLots && inst2.SendInLots {
		// C++: maxPos = max(BID_MAX_SIZE, ASK_MAX_SIZE)
		maxPos := thold.BidMaxSize
		if thold.AskMaxSize > maxPos {
			maxPos = thold.AskMaxSize
		}
		state.TholdMaxPos = maxPos
		state.TholdBeginPos = thold.BeginSize
		state.TholdSize = thold.Size

		// C++: 独立 bid/ask 大小和限制
		state.TholdBidSize = thold.BidSize
		state.TholdBidMaxPos = thold.BidMaxSize
		state.TholdAskSize = thold.AskSize
		state.TholdAskMaxPos = thold.AskMaxSize
	} else {
		// C++: 按手数换算
		state.TholdMaxPos = thold.MaxSize * int32(inst1.LotSize)
		state.TholdBeginPos = thold.BeginSize * int32(inst1.LotSize)
		state.TholdSize = thold.Size * int32(inst1.LotSize)

		// 非 sendInLots 模式下，bid/ask 各自的大小默认等于 TholdSize
		state.TholdBidSize = state.TholdSize
		state.TholdBidMaxPos = state.TholdMaxPos
		state.TholdAskSize = state.TholdSize
		state.TholdAskMaxPos = state.TholdMaxPos
	}

	// C++: 阈值差值
	// auto long_place_diff_thold = LONG_PLACE - BEGIN_PLACE
	longPlaceDiff := thold.LongPlace - thold.BeginPlace
	// auto short_place_diff_thold = BEGIN_PLACE - SHORT_PLACE
	shortPlaceDiff := thold.BeginPlace - thold.ShortPlace
	// auto long_remove_diff_thold = LONG_REMOVE - BEGIN_REMOVE
	longRemoveDiff := thold.LongRemove - thold.BeginRemove
	// auto short_remove_diff_thold = BEGIN_REMOVE - SHORT_REMOVE
	shortRemoveDiff := thold.BeginRemove - thold.ShortRemove

	maxPos := float64(state.TholdMaxPos)
	if maxPos == 0 {
		return
	}
	netposPass := float64(state.NetposPass)

	// C++: 根据 m_netpos_pass 线性插值
	// 参考: PairwiseArbStrategy.cpp:927-947
	if state.NetposPass == 0 {
		// C++: flat position
		state.TholdBidPlace = thold.BeginPlace
		state.TholdBidRemove = thold.BeginRemove
		state.TholdAskPlace = thold.BeginPlace
		state.TholdAskRemove = thold.BeginRemove
	} else if state.NetposPass > 0 {
		// C++: 多头 — 线性插值
		// m_tholdBidPlace = BEGIN_PLACE + long_diff * netpos_pass / maxPos
		state.TholdBidPlace = thold.BeginPlace + longPlaceDiff*netposPass/maxPos
		state.TholdBidRemove = thold.BeginRemove + longRemoveDiff*netposPass/maxPos
		// m_tholdAskPlace = BEGIN_PLACE - short_diff * netpos_pass / maxPos
		state.TholdAskPlace = thold.BeginPlace - shortPlaceDiff*netposPass/maxPos
		state.TholdAskRemove = thold.BeginRemove - shortRemoveDiff*netposPass/maxPos
	} else {
		// C++: 空头 — 线性插值（netpos_pass < 0）
		// m_tholdBidPlace = BEGIN_PLACE + short_diff * netpos_pass / maxPos
		state.TholdBidPlace = thold.BeginPlace + shortPlaceDiff*netposPass/maxPos
		state.TholdBidRemove = thold.BeginRemove + shortRemoveDiff*netposPass/maxPos
		// m_tholdAskPlace = BEGIN_PLACE - long_diff * netpos_pass / maxPos
		state.TholdAskPlace = thold.BeginPlace - longPlaceDiff*netposPass/maxPos
		state.TholdAskRemove = thold.BeginRemove - longRemoveDiff*netposPass/maxPos
	}
}
