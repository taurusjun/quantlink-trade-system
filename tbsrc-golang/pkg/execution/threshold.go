package execution

import (
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/types"
)

// SetThresholds 根据持仓离散调整入场/出场阈值
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:595-689
//
// C++ 逻辑：离散跳变
//   netpos == 0          → BEGIN_PLACE / BEGIN_REMOVE
//   0 < netpos < beginPos → bid=BEGIN, ask=SHORT
//   -beginPos < netpos < 0 → ask=BEGIN, bid=SHORT
//   netpos >= beginPos   → bid=LONG, ask=SHORT
//   netpos <= -beginPos  → ask=LONG, bid=SHORT
func (s *ExecutionState) SetThresholds(inst *instrument.Instrument, thold *types.ThresholdSet) {
	// 初始化
	s.TholdBidPlace = -1
	s.TholdBidRemove = -1
	s.TholdAskPlace = -1
	s.TholdAskRemove = -1
	s.TholdMaxPos = 0
	s.TholdBeginPos = 0
	s.TholdSize = 0

	// 计算仓位大小
	s.computeSizing(inst, thold)

	// 离散阈值调整
	if s.Netpos == 0 {
		// C++: flat position
		s.TholdBidPlace = s.beginPlaceValue(thold)
		s.TholdAskPlace = s.beginPlaceValue(thold)
		s.TholdBidRemove = thold.BeginRemove
		s.TholdAskRemove = thold.BeginRemove
	} else if s.Netpos > 0 && s.Netpos < s.TholdBeginPos {
		// C++: small long
		s.TholdBidPlace = s.beginPlaceValue(thold)
		s.TholdBidRemove = thold.BeginRemove
		s.TholdAskPlace = thold.ShortPlace
		s.TholdAskRemove = thold.ShortRemove
	} else if s.Netpos < 0 && s.Netpos > -s.TholdBeginPos {
		// C++: small short
		s.TholdAskPlace = s.beginPlaceValue(thold)
		s.TholdAskRemove = thold.BeginRemove
		s.TholdBidPlace = thold.ShortPlace
		s.TholdBidRemove = thold.ShortRemove
	} else if s.Netpos > 0 {
		// C++: large long — discrete jump to LONG
		if s.SetHigh != 0 {
			s.TholdBidPlace = thold.LongPlaceHigh
		} else {
			s.TholdBidPlace = thold.LongPlace
		}
		s.TholdBidRemove = thold.LongRemove
		s.TholdAskPlace = thold.ShortPlace
		s.TholdAskRemove = thold.ShortRemove
	} else if s.Netpos < 0 {
		// C++: large short — mirror
		if s.SetHigh != 0 {
			s.TholdAskPlace = thold.LongPlaceHigh
		} else {
			s.TholdAskPlace = thold.LongPlace
		}
		s.TholdAskRemove = thold.LongRemove
		s.TholdBidPlace = thold.ShortPlace
		s.TholdBidRemove = thold.ShortRemove
	}
}

// SetLinearThresholds 根据持仓线性插值调整入场/出场阈值
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:500-593
//
// C++ 逻辑：线性插值
//   netpos > beginPos 时:
//     bidPlace = BEGIN_PLACE + (LONG_PLACE - BEGIN_PLACE) * netpos / maxPos
//     askPlace = BEGIN_PLACE - (BEGIN_PLACE - SHORT_PLACE) * netpos / maxPos
func (s *ExecutionState) SetLinearThresholds(inst *instrument.Instrument, thold *types.ThresholdSet) {
	// 初始化
	s.TholdBidPlace = -1
	s.TholdBidRemove = -1
	s.TholdAskPlace = -1
	s.TholdAskRemove = -1
	s.TholdMaxPos = 0
	s.TholdBeginPos = 0
	s.TholdSize = 0

	// 计算仓位大小
	s.computeSizing(inst, thold)

	maxPos := float64(s.TholdMaxPos)
	if maxPos == 0 {
		return
	}
	netpos := float64(s.Netpos)

	if s.Netpos == 0 {
		// C++: flat
		s.TholdBidPlace = s.beginPlaceValue(thold)
		s.TholdAskPlace = s.beginPlaceValue(thold)
		s.TholdBidRemove = thold.BeginRemove
		s.TholdAskRemove = thold.BeginRemove
	} else if s.Netpos > 0 && s.Netpos < s.TholdBeginPos {
		// C++: small long
		s.TholdBidPlace = s.beginPlaceValue(thold)
		s.TholdBidRemove = thold.BeginRemove
		s.TholdAskPlace = thold.ShortPlace
		s.TholdAskRemove = thold.ShortRemove
	} else if s.Netpos < 0 && s.Netpos > -s.TholdBeginPos {
		// C++: small short
		s.TholdAskPlace = s.beginPlaceValue(thold)
		s.TholdAskRemove = thold.BeginRemove
		s.TholdBidPlace = thold.ShortPlace
		s.TholdBidRemove = thold.ShortRemove
	} else if s.Netpos > 0 {
		// C++: linear interpolation for long
		// bidPlace = BEGIN + (LONG - BEGIN) * netpos / maxPos
		if s.SetHigh != 0 {
			s.TholdBidPlace = thold.LongPlaceHigh
		} else {
			s.TholdBidPlace = thold.BeginPlace + (thold.LongPlace-thold.BeginPlace)*netpos/maxPos
		}
		s.TholdBidRemove = thold.BeginRemove + (thold.LongRemove-thold.BeginRemove)*netpos/maxPos
		// askPlace = BEGIN - (BEGIN - SHORT) * netpos / maxPos
		s.TholdAskPlace = thold.BeginPlace - (thold.BeginPlace-thold.ShortPlace)*netpos/maxPos
		s.TholdAskRemove = thold.BeginRemove - (thold.BeginRemove-thold.ShortRemove)*netpos/maxPos
	} else if s.Netpos < 0 {
		// C++: linear interpolation for short (mirror)
		absNetpos := -netpos
		if s.SetHigh != 0 {
			s.TholdAskPlace = thold.LongPlaceHigh
		} else {
			s.TholdAskPlace = thold.BeginPlace + (thold.LongPlace-thold.BeginPlace)*absNetpos/maxPos
		}
		s.TholdAskRemove = thold.BeginRemove + (thold.LongRemove-thold.BeginRemove)*absNetpos/maxPos
		s.TholdBidPlace = thold.BeginPlace - (thold.BeginPlace-thold.ShortPlace)*absNetpos/maxPos
		s.TholdBidRemove = thold.BeginRemove - (thold.BeginRemove-thold.ShortRemove)*absNetpos/maxPos
	}
}

// computeSizing 计算仓位大小参数
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:510-540 (SetLinearThresholds 中的 sizing 逻辑)
func (s *ExecutionState) computeSizing(inst *instrument.Instrument, thold *types.ThresholdSet) {
	if thold.UseNotional {
		// C++: notional-based sizing
		mktPx := (inst.BidPx[0] + inst.AskPx[0]) / 2.0
		contractVal := mktPx * inst.LotSize
		if contractVal > 0 {
			s.TholdMaxPos = int32(float64(thold.NotionalMaxSz) * inst.PriceFactor / contractVal) * int32(inst.LotSize)
			s.TholdSize = int32(float64(thold.NotionalSize) * inst.PriceFactor / contractVal) * int32(inst.LotSize)
		}
		if thold.NotionalSize > 0 {
			s.SMSRatio = int32(float64(thold.NotionalMaxSz) / float64(thold.NotionalSize))
		}
	} else {
		// C++: lot-based or unit-based
		if inst.SendInLots {
			s.TholdMaxPos = thold.MaxSize
			s.TholdBeginPos = thold.BeginSize
			s.TholdSize = thold.Size
		} else {
			s.TholdMaxPos = thold.MaxSize * int32(inst.LotSize)
			s.TholdBeginPos = thold.BeginSize * int32(inst.LotSize)
			s.TholdSize = thold.Size * int32(inst.LotSize)
		}
		if thold.Size > 0 {
			s.SMSRatio = int32(thold.MaxSize / thold.Size)
		}
	}
}

// beginPlaceValue 返回 BEGIN_PLACE 或 BEGIN_PLACE_HIGH
func (s *ExecutionState) beginPlaceValue(thold *types.ThresholdSet) float64 {
	if s.SetHigh != 0 {
		return thold.BeginPlaceHigh
	}
	return thold.BeginPlace
}
