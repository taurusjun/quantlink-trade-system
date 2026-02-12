// Package strategy provides position persistence functionality
package strategy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PositionSnapshot 持仓快照数据结构
type PositionSnapshot struct {
	StrategyID    string            `json:"strategy_id"`
	Timestamp     time.Time         `json:"timestamp"`
	SymbolsPos    map[string]int64  `json:"symbols_position"`  // symbol -> net_position
	TotalLongQty  int64             `json:"total_long_qty"`
	TotalShortQty int64             `json:"total_short_qty"`
	TotalNetQty   int64             `json:"total_net_qty"`
	AvgLongPrice  float64           `json:"avg_long_price"`
	AvgShortPrice float64           `json:"avg_short_price"`
	RealizedPnL   float64           `json:"realized_pnl"`
	// 昨仓净值（中国期货特有）
	// C++: m_netpos_pass_ytd - 昨日收盘时的净持仓
	// 今仓净值 = SymbolsPos[symbol] - SymbolsYesterdayPos[symbol]
	SymbolsYesterdayPos map[string]int64 `json:"symbols_yesterday_position,omitempty"` // symbol -> ytd_position
}

// PositionWithCost 持仓信息（含成本价）
// 注意：C++ 原代码中，m_buyPrice/m_sellPrice 是当天成交均价，开盘时为 0
// Go 代码增加此结构以支持从 CTP 获取昨仓成本价，这是与 C++ 的差异
// C++ 的 P&L 只计算当天交易产生的盈亏，昨仓成本为 0
// Go 代码使用 CTP 返回的成本价来计算完整的浮动盈亏，便于风控和监控
type PositionWithCost struct {
	Quantity int64   // 持仓数量（正=多头，负=空头）
	AvgCost  float64 // 平均成本价
}

// PositionInitializer 接口：支持从外部初始化持仓
type PositionInitializer interface {
	InitializePositions(positions map[string]int64) error
	// InitializePositionsWithCost 使用成本价初始化持仓
	// 注意：此方法是 Go 代码新增的，C++ 原代码中没有对应实现
	// C++ 的昨仓成本为 0，只计算当天交易产生的盈亏
	// Go 代码使用 CTP 返回的成本价来计算完整的浮动盈亏
	InitializePositionsWithCost(positions map[string]PositionWithCost) error
}

// PositionProvider 接口：提供当前持仓
type PositionProvider interface {
	GetPositionsBySymbol() map[string]int64
}

// getPositionDataDir 获取持仓数据目录
// 使用全局 dataDir 变量，支持实盘/模拟盘数据隔离
func getPositionDataDir() string {
	return filepath.Join(GetDataDir(), "positions")
}

// SavePositionSnapshot 保存持仓快照到文件
func SavePositionSnapshot(snapshot PositionSnapshot) error {
	posDir := getPositionDataDir()
	// 确保目录存在
	if err := os.MkdirAll(posDir, 0755); err != nil {
		return fmt.Errorf("failed to create position data directory: %w", err)
	}

	filename := filepath.Join(posDir, fmt.Sprintf("%s.json", snapshot.StrategyID))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal position snapshot: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write position snapshot: %w", err)
	}

	return nil
}

// LoadPositionSnapshot 从文件加载持仓快照
func LoadPositionSnapshot(strategyID string) (*PositionSnapshot, error) {
	filename := filepath.Join(getPositionDataDir(), fmt.Sprintf("%s.json", strategyID))

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在，返回nil（不是错误）
		}
		return nil, fmt.Errorf("failed to read position snapshot: %w", err)
	}

	var snapshot PositionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal position snapshot: %w", err)
	}

	return &snapshot, nil
}

// DeletePositionSnapshot 删除持仓快照文件
func DeletePositionSnapshot(strategyID string) error {
	filename := filepath.Join(getPositionDataDir(), fmt.Sprintf("%s.json", strategyID))
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete position snapshot: %w", err)
	}
	return nil
}

// SaveStrategyPosition 保存策略持仓（辅助函数）
func SaveStrategyPosition(strategy Strategy) error {
	pos := strategy.GetEstimatedPosition() // Get estimated position

	snapshot := PositionSnapshot{
		StrategyID:    strategy.GetID(),
		Timestamp:     time.Now(),
		TotalLongQty:  pos.LongQty,
		TotalShortQty: pos.ShortQty,
		TotalNetQty:   pos.NetQty,
		AvgLongPrice:  pos.AvgLongPrice,
		AvgShortPrice: pos.AvgShortPrice,
		RealizedPnL:   strategy.GetPNL().RealizedPnL,
		SymbolsPos:    make(map[string]int64),
	}

	// 如果策略实现了PositionProvider接口，获取按品种的持仓
	if provider, ok := strategy.(PositionProvider); ok {
		snapshot.SymbolsPos = provider.GetPositionsBySymbol()
	}

	return SavePositionSnapshot(snapshot)
}

// LoadStrategyPosition 加载策略持仓（辅助函数）
func LoadStrategyPosition(strategy Strategy) (*PositionSnapshot, error) {
	return LoadPositionSnapshot(strategy.GetID())
}
