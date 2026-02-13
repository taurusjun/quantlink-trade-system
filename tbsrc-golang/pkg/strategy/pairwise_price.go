package strategy

import (
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/types"
)

// GetBidPrice 对应 C++ GetBidPrice_first
// 在 invisible book 模式下，如果行情簿有空档且价差仍满足阈值，
// 则改善价格一个 tick 以跳过队列
// 参考: PairwiseArbStrategy.cpp:802-820
//
// C++ 逻辑:
//  1. price = bidPx[level]
//  2. if UseInvisibleBook && level != 0 && bidPx[level] < bidPx[level-1] - tickSize:
//     a. bidInv = bidPx[level] - leg2.bidPx[0] + tickSize
//     b. if bidInv <= avgSpread - BEGIN_PLACE:
//        c. if existing order at this price has quantAhead > lotSize:
//           → price = bidPx[level] + tickSize (improve one tick)
func (pas *PairwiseArbStrategy) GetBidPrice(price float64, ordType types.OrderHitType, level int32) (float64, types.OrderHitType) {
	if !pas.UseInvisibleBook || level == 0 {
		return price, ordType
	}

	inst1 := pas.Inst1
	inst2 := pas.Inst2
	tickSize := inst1.TickSize

	// C++: check for gap in order book
	if level >= int32(instrument.BookDepth) || price >= inst1.BidPx[level-1]-tickSize {
		return price, ordType
	}

	// C++: bidInv = bidPx[level] - leg2.bidPx[0] + tickSize
	bidInv := price - inst2.BidPx[0] + tickSize

	// C++: if bidInv <= avgSpreadRatio - BEGIN_PLACE
	if bidInv <= pas.Spread.AvgSpread-pas.Thold1.BeginPlace {
		// C++: check if there is already an order at this price with sufficient queue
		if ord, exists := pas.Leg1.Orders.BidMap[price]; exists {
			if ord.QuantAhead > inst1.LotSize {
				// C++: improve by one tick
				price = inst1.BidPx[level] + tickSize
			}
		}
	}

	return price, ordType
}

// GetAskPrice 对应 C++ GetAskPrice_first
// 在 invisible book 模式下，如果行情簿有空档且价差仍满足阈值，
// 则改善价格一个 tick 以跳过队列
// 参考: PairwiseArbStrategy.cpp:822-840
//
// C++ 逻辑:
//  1. price = askPx[level]
//  2. if UseInvisibleBook && level != 0 && askPx[level] > askPx[level-1] + tickSize:
//     a. askInv = askPx[level] - leg2.askPx[0] - tickSize
//     b. if askInv >= avgSpread + BEGIN_PLACE:
//        c. if existing order at this price has quantAhead > lotSize:
//           → price = askPx[level] - tickSize (improve one tick)
func (pas *PairwiseArbStrategy) GetAskPrice(price float64, ordType types.OrderHitType, level int32) (float64, types.OrderHitType) {
	if !pas.UseInvisibleBook || level == 0 {
		return price, ordType
	}

	inst1 := pas.Inst1
	inst2 := pas.Inst2
	tickSize := inst1.TickSize

	// C++: check for gap in order book
	if level >= int32(instrument.BookDepth) || price <= inst1.AskPx[level-1]+tickSize {
		return price, ordType
	}

	// C++: askInv = askPx[level] - leg2.askPx[0] - tickSize
	askInv := price - inst2.AskPx[0] - tickSize

	// C++: if askInv >= avgSpreadRatio + BEGIN_PLACE
	if askInv >= pas.Spread.AvgSpread+pas.Thold1.BeginPlace {
		// C++: check if there is already an order at this price with sufficient queue
		if ord, exists := pas.Leg1.Orders.AskMap[price]; exists {
			if ord.QuantAhead > inst1.LotSize {
				// C++: improve by one tick
				price = inst1.AskPx[level] - tickSize
			}
		}
	}

	return price, ordType
}
