package execution

import (
	"tbsrc-golang/pkg/instrument"
)

// CalculatePNL 计算未实现 PNL 和净 PNL
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:2124-2148
//
// C++ 逻辑:
//   多头: unrealised = netpos * ((bid[0] - buyPrice - bid[0]*sellExchTx) * priceMultiplier - sellExchContractTx)
//   空头: unrealised = -netpos * ((sellPrice - ask[0] - ask[0]*buyExchTx) * priceMultiplier - buyExchContractTx)
//   加上已平部分的 PNL: qty * (sellPrice - buyPrice) * priceMultiplier
//   减去当前腿手续费: -= transValue
//   grossPNL = realisedPNL + unrealisedPNL
//   netPNL = grossPNL - transTotalValue
func (s *ExecutionState) CalculatePNL(inst *instrument.Instrument) {
	// C++: int32_t qty = m_netpos > 0 ? m_sellQty : m_buyQty
	var closedQty float64
	if s.Netpos > 0 {
		closedQty = s.SellQty
	} else {
		closedQty = s.BuyQty
	}

	mult := inst.PriceMultiplier

	if s.Netpos > 0 {
		// 多头: mark-to-market 对 best bid
		// C++: m_netpos * ((bidPx[0] - m_buyPrice - bidPx[0]*m_sellExchTx) * priceMultiplier - m_sellExchContractTx)
		netposF := float64(s.Netpos)
		s.UnrealisedPNL = netposF * ((inst.BidPx[0] - s.BuyPrice - inst.BidPx[0]*s.SellExchTx) * mult - s.SellExchContractTx)
	} else if s.Netpos < 0 {
		// 空头: mark-to-market 对 best ask
		// C++: -1 * m_netpos * ((m_sellPrice - askPx[0] - askPx[0]*m_buyExchTx) * priceMultiplier - m_buyExchContractTx)
		absNetpos := float64(-s.Netpos)
		s.UnrealisedPNL = absNetpos * ((s.SellPrice - inst.AskPx[0] - inst.AskPx[0]*s.BuyExchTx) * mult - s.BuyExchContractTx)
	} else {
		s.UnrealisedPNL = 0
	}

	// C++: += qty * (m_sellPrice - m_buyPrice) * priceMultiplier
	// 已平仓部分的未实现利润（当前腿中已对冲的部分）
	s.UnrealisedPNL += closedQty * (s.SellPrice - s.BuyPrice) * mult

	// C++: -= m_transValue
	s.UnrealisedPNL -= s.TransValue

	// C++: m_grossPNL = m_realisedPNL + m_unrealisedPNL
	s.GrossPNL = s.RealisedPNL + s.UnrealisedPNL

	// C++: m_netPNL = m_grossPNL - m_transTotalValue
	s.NetPNL = s.GrossPNL - s.TransTotalValue

	// C++: if (m_netPNL > m_maxPNL) m_maxPNL = m_netPNL
	if s.NetPNL > s.MaxPNL {
		s.MaxPNL = s.NetPNL
	}

	// C++: m_drawdown = m_netPNL - m_maxPNL
	s.Drawdown = s.NetPNL - s.MaxPNL
}
