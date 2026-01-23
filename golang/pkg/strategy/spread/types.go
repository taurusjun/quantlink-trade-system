// Package spread provides tools for analyzing price spreads between instruments
package spread

// SpreadType 定义 spread 计算类型
type SpreadType string

const (
	// SpreadTypeDifference 差价 spread: price1 - hedgeRatio * price2
	SpreadTypeDifference SpreadType = "difference"

	// SpreadTypeRatio 比率 spread: price1 / price2
	SpreadTypeRatio SpreadType = "ratio"

	// SpreadTypeLog 对数 spread: log(price1) - log(price2)
	// 常用于协整分析
	SpreadTypeLog SpreadType = "log"
)

// SpreadStats spread 统计信息
type SpreadStats struct {
	CurrentSpread float64 // 当前 spread 值
	Mean          float64 // Spread 均值
	Std           float64 // Spread 标准差
	ZScore        float64 // Z-Score
	Correlation   float64 // 价格相关系数
	HedgeRatio    float64 // 对冲比率（Beta）
}
