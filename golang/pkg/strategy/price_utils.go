package strategy

import (
	"math"
)

// InstrumentSpec 品种规格
type InstrumentSpec struct {
	TickSize           float64 // 最小变动价位
	ContractMultiplier int64   // 合约乘数
	Exchange           string  // 交易所
}

// DefaultInstrumentSpecs 默认品种规格
var DefaultInstrumentSpecs = map[string]InstrumentSpec{
	// 白银
	"ag2603": {TickSize: 1.0, ContractMultiplier: 15, Exchange: "SHFE"},
	"ag2605": {TickSize: 1.0, ContractMultiplier: 15, Exchange: "SHFE"},
	"ag2612": {TickSize: 1.0, ContractMultiplier: 15, Exchange: "SHFE"},

	// 黄金
	"au2604": {TickSize: 0.02, ContractMultiplier: 1000, Exchange: "SHFE"},
	"au2606": {TickSize: 0.02, ContractMultiplier: 1000, Exchange: "SHFE"},
	"au2612": {TickSize: 0.02, ContractMultiplier: 1000, Exchange: "SHFE"},

	// 螺纹钢
	"rb2605": {TickSize: 1.0, ContractMultiplier: 10, Exchange: "SHFE"},
	"rb2610": {TickSize: 1.0, ContractMultiplier: 10, Exchange: "SHFE"},

	// 铜
	"cu2604": {TickSize: 10.0, ContractMultiplier: 5, Exchange: "SHFE"},
	"cu2606": {TickSize: 10.0, ContractMultiplier: 5, Exchange: "SHFE"},
}

// RoundToTickSize 将价格四舍五入到tick size的倍数
func RoundToTickSize(price float64, tickSize float64) float64 {
	if tickSize == 0 {
		return price
	}
	return math.Round(price/tickSize) * tickSize
}

// FloorToTickSize 将价格向下取整到tick size的倍数
func FloorToTickSize(price float64, tickSize float64) float64 {
	if tickSize == 0 {
		return price
	}
	return math.Floor(price/tickSize) * tickSize
}

// CeilToTickSize 将价格向上取整到tick size的倍数
func CeilToTickSize(price float64, tickSize float64) float64 {
	if tickSize == 0 {
		return price
	}
	return math.Ceil(price/tickSize) * tickSize
}

// GetOrderPrice 根据订单方向和市场数据计算合适的委托价格
//
// 参数:
//   - side: 订单方向 (Buy/Sell)
//   - bid: 买一价
//   - ask: 卖一价
//   - symbol: 品种代码
//   - slippageTicks: 滑点(tick数), 0表示使用对手价
//   - aggressive: 是否主动成交（true=对手价，false=挂单价）
//
// 返回:
//   - 调整后的委托价格（已对齐tick size）
func GetOrderPrice(side OrderSide, bid float64, ask float64, symbol string, slippageTicks int, aggressive bool) float64 {
	// 获取品种规格
	spec, ok := DefaultInstrumentSpecs[symbol]
	if !ok {
		// 如果没有配置，使用mid price并尝试默认tick size
		midPrice := (bid + ask) / 2.0
		// 默认tick size: 根据价格大小判断
		tickSize := 0.01
		if midPrice > 1000 {
			tickSize = 0.02
		} else if midPrice > 100 {
			tickSize = 0.1
		} else if midPrice > 10 {
			tickSize = 1.0
		}
		return RoundToTickSize(midPrice, tickSize)
	}

	tickSize := spec.TickSize
	var basePrice float64

	if side == OrderSideBuy {
		if aggressive {
			// 买单：主动成交，使用卖一价（ask）
			basePrice = ask
		} else {
			// 买单：挂单，使用买一价（bid）或更低
			basePrice = bid
		}
		// 加上滑点（买单：价格向上）
		basePrice += float64(slippageTicks) * tickSize
	} else { // OrderSideSell
		if aggressive {
			// 卖单：主动成交，使用买一价（bid）
			basePrice = bid
		} else {
			// 卖单：挂单，使用卖一价（ask）或更高
			basePrice = ask
		}
		// 减去滑点（卖单：价格向下）
		basePrice -= float64(slippageTicks) * tickSize
	}

	// 确保价格对齐到tick size
	return RoundToTickSize(basePrice, tickSize)
}

// GetTickSize 获取品种的tick size
func GetTickSize(symbol string) float64 {
	if spec, ok := DefaultInstrumentSpecs[symbol]; ok {
		return spec.TickSize
	}
	// 默认返回0.01
	return 0.01
}

// ValidatePrice 验证价格是否是tick size的倍数
func ValidatePrice(price float64, symbol string) bool {
	tickSize := GetTickSize(symbol)
	if tickSize == 0 {
		return true
	}

	// 检查是否是tick size的倍数（允许浮点误差）
	remainder := math.Mod(price, tickSize)
	tolerance := tickSize * 0.01 // 1%的容差

	return remainder < tolerance || (tickSize-remainder) < tolerance
}

// GetContractMultiplier 获取品种的合约乘数
// C++: m_instru->m_priceMultiplier
func GetContractMultiplier(symbol string) float64 {
	if spec, ok := DefaultInstrumentSpecs[symbol]; ok {
		return float64(spec.ContractMultiplier)
	}
	// 默认返回1（不乘）
	return 1.0
}

// GetInstrumentSpec 获取品种规格
func GetInstrumentSpec(symbol string) (InstrumentSpec, bool) {
	spec, ok := DefaultInstrumentSpecs[symbol]
	return spec, ok
}
