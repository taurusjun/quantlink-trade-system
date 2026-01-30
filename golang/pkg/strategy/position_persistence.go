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
}

// PositionInitializer 接口：支持从外部初始化持仓
type PositionInitializer interface {
	InitializePositions(positions map[string]int64) error
}

// PositionProvider 接口：提供当前持仓
type PositionProvider interface {
	GetPositionsBySymbol() map[string]int64
}

const positionDataDir = "data/positions"

// SavePositionSnapshot 保存持仓快照到文件
func SavePositionSnapshot(snapshot PositionSnapshot) error {
	// 确保目录存在
	if err := os.MkdirAll(positionDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create position data directory: %w", err)
	}

	filename := filepath.Join(positionDataDir, fmt.Sprintf("%s.json", snapshot.StrategyID))

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
	filename := filepath.Join(positionDataDir, fmt.Sprintf("%s.json", strategyID))

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
	filename := filepath.Join(positionDataDir, fmt.Sprintf("%s.json", strategyID))
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
