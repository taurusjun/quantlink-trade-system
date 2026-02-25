package instrument

import (
	"tbsrc-golang/pkg/shm"
)

const (
	// BookDepth 行情簿深度，对应 C++ InterestLevels = 20
	BookDepth = 20
)

// Instrument 对应 C++ class Instrument（简化版，仅 SHM 路径所需字段）
// 参考: tbsrc/common/include/Instrument.h
type Instrument struct {
	// 合约基本信息
	Symbol       string // m_symbol (e.g. "ag2603")
	OrigBaseName string // m_origbaseName (e.g. "ag_F_3_SFE")，来自 controlFile
	Exchange     string // m_exchange
	TickSize        float64 // m_tickSize
	LotSize         float64 // m_lotSize
	ContractFactor  float64 // m_contractFactor
	PriceMultiplier float64 // m_priceMultiplier
	PriceFactor     float64 // m_priceFactor
	SendInLots      bool    // m_sendInLots
	Token           int32   // m_token
	ExpiryDate      int32   // m_expiryDate
	SymbolID        uint16  // m_symbolID

	// 20 档行情簿
	// C++: double bidPx[20], askPx[20], bidQty[20], askQty[20]
	BidPx  [BookDepth]float64
	BidQty [BookDepth]float64
	AskPx  [BookDepth]float64
	AskQty [BookDepth]float64

	// 有效档位数
	ValidBids int32 // m_validBids
	ValidAsks int32 // m_validAsks

	// 最新成交
	LastTradePx  float64 // lastTradePx
	LastTradeQty float64 // lastTradeqty
}

// NewFromConfig 从配置创建 Instrument
func NewFromConfig(symbol string, exchange string, tickSize, lotSize, contractFactor, priceMultiplier, priceFactor float64, sendInLots bool, token, expiryDate int32) *Instrument {
	return &Instrument{
		Symbol:          symbol,
		Exchange:        exchange,
		TickSize:        tickSize,
		LotSize:         lotSize,
		ContractFactor:  contractFactor,
		PriceMultiplier: priceMultiplier,
		PriceFactor:     priceFactor,
		SendInLots:      sendInLots,
		Token:           token,
		ExpiryDate:      expiryDate,
	}
}

// UpdateFromMD 从 SHM MarketUpdateNew 更新行情簿
// 参考: tbsrc/common/include/Instrument.h FillOrderBook()
func (inst *Instrument) UpdateFromMD(md *shm.MarketUpdateNew) {
	data := &md.Data

	// 更新有效档位数
	inst.ValidBids = int32(data.ValidBids)
	inst.ValidAsks = int32(data.ValidAsks)

	// 更新 20 档 bid
	for i := range BookDepth {
		inst.BidPx[i] = data.BidUpdates[i].Price
		inst.BidQty[i] = float64(data.BidUpdates[i].Quantity)
	}

	// 更新 20 档 ask
	for i := range BookDepth {
		inst.AskPx[i] = data.AskUpdates[i].Price
		inst.AskQty[i] = float64(data.AskUpdates[i].Quantity)
	}

	// 更新最新成交
	inst.LastTradePx = data.LastTradedPrice
	inst.LastTradeQty = float64(data.LastTradedQuantity)
}

// MidPrice 返回中间价
// C++: (bidPx[0] + askPx[0]) / 2
func (inst *Instrument) MidPrice() float64 {
	return (inst.BidPx[0] + inst.AskPx[0]) / 2.0
}

// MSWPrice 返回市场量加权价（Market Size Weighted Price）
// C++: MSWPrice_ = (askQty[0]*bidPx[0] + bidQty[0]*askPx[0]) / (askQty[0]+bidQty[0])
// 参考: tbsrc/common/include/Instrument.h CalculatePrices()
func (inst *Instrument) MSWPrice() float64 {
	totalQty := inst.AskQty[0] + inst.BidQty[0]
	if totalQty == 0 {
		return inst.MidPrice()
	}
	return (inst.AskQty[0]*inst.BidPx[0] + inst.BidQty[0]*inst.AskPx[0]) / totalQty
}

// HasValidBook 检查行情簿是否有效（bid > 0 且 ask > 0）
func (inst *Instrument) HasValidBook() bool {
	return inst.BidPx[0] > 0 && inst.AskPx[0] > 0
}

// Spread 返回买卖价差
func (inst *Instrument) Spread() float64 {
	return inst.AskPx[0] - inst.BidPx[0]
}
