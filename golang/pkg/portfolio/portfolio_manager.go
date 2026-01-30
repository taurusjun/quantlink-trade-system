// Package portfolio provides portfolio management functionality
package portfolio

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// PortfolioConfig represents portfolio configuration
type PortfolioConfig struct {
	TotalCapital          float64            // Total capital
	StrategyAllocation    map[string]float64 // strategy_id -> allocation percentage (0-1)
	RebalanceIntervalSec  int                // Rebalancing interval in seconds
	MinAllocation         float64            // Minimum allocation per strategy
	MaxAllocation         float64            // Maximum allocation per strategy
	EnableAutoRebalance   bool               // Enable automatic rebalancing
	EnableCorrelationCalc bool               // Enable correlation calculation
}

// PortfolioStats represents portfolio statistics
type PortfolioStats struct {
	TotalCapital     float64
	AllocatedCapital float64
	FreeCapital      float64
	TotalPnL         float64
	TotalReturn      float64 // Percentage return
	SharpeRatio      float64
	MaxDrawdown      float64
	NumStrategies    int
	NumActiveStrategies int
	Timestamp        time.Time
}

// StrategyAllocation represents allocation for a single strategy
type StrategyAllocation struct {
	StrategyID         string
	AllocatedCapital   float64
	AllocationPercent  float64
	CurrentPnL         float64
	CurrentReturn      float64
	CurrentExposure    float64
	PositionSize       int64
	IsActive           bool
	LastUpdate         time.Time
}

// CorrelationMatrix represents correlation between strategies
type CorrelationMatrix struct {
	StrategyIDs []string
	Matrix      [][]float64 // correlation coefficients
	Timestamp   time.Time
}

// PortfolioManager manages capital allocation across strategies
type PortfolioManager struct {
	config      *PortfolioConfig
	strategies  map[string]strategy.Strategy
	allocations map[string]*StrategyAllocation

	// Portfolio statistics
	stats        *PortfolioStats
	correlation  *CorrelationMatrix

	// PnL history for Sharpe calculation
	pnlHistory   []float64
	maxPnLHistory int

	mu           sync.RWMutex
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isRunning    bool
}

// NewPortfolioManager creates a new portfolio manager
func NewPortfolioManager(config *PortfolioConfig) *PortfolioManager {
	if config == nil {
		config = &PortfolioConfig{
			TotalCapital:         1000000.0, // Default 100万
			StrategyAllocation:   make(map[string]float64),
			RebalanceIntervalSec: 3600, // 1 hour
			MinAllocation:        0.05,  // 5%
			MaxAllocation:        0.50,  // 50%
			EnableAutoRebalance:  true,
			EnableCorrelationCalc: true,
		}
	}

	// Ensure StrategyAllocation map is initialized
	if config.StrategyAllocation == nil {
		config.StrategyAllocation = make(map[string]float64)
	}

	return &PortfolioManager{
		config:        config,
		strategies:    make(map[string]strategy.Strategy),
		allocations:   make(map[string]*StrategyAllocation),
		stats:         &PortfolioStats{},
		pnlHistory:    make([]float64, 0, 1000),
		maxPnLHistory: 1000,
		stopChan:      make(chan struct{}),
	}
}

// Initialize initializes the portfolio manager
func (pm *PortfolioManager) Initialize() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.stats.TotalCapital = pm.config.TotalCapital
	pm.stats.Timestamp = time.Now()

	log.Printf("[PortfolioManager] Initialized with capital: %.2f", pm.config.TotalCapital)
	return nil
}

// Start starts the portfolio manager
func (pm *PortfolioManager) Start() error {
	pm.mu.Lock()
	if pm.isRunning {
		pm.mu.Unlock()
		return fmt.Errorf("portfolio manager already running")
	}
	pm.isRunning = true
	pm.mu.Unlock()

	// Start rebalancing loop
	if pm.config.EnableAutoRebalance {
		pm.wg.Add(1)
		go pm.rebalanceLoop()
	}

	log.Println("[PortfolioManager] Started")
	return nil
}

// Stop stops the portfolio manager
func (pm *PortfolioManager) Stop() error {
	pm.mu.Lock()
	if !pm.isRunning {
		pm.mu.Unlock()
		return fmt.Errorf("portfolio manager not running")
	}
	pm.isRunning = false
	pm.mu.Unlock()

	close(pm.stopChan)
	pm.wg.Wait()

	log.Println("[PortfolioManager] Stopped")
	return nil
}

// AddStrategy adds a strategy to the portfolio
func (pm *PortfolioManager) AddStrategy(s strategy.Strategy, allocationPercent float64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := s.GetID()

	// Validate allocation
	if allocationPercent < pm.config.MinAllocation || allocationPercent > pm.config.MaxAllocation {
		return fmt.Errorf("allocation %.2f%% outside range [%.2f%%, %.2f%%]",
			allocationPercent*100, pm.config.MinAllocation*100, pm.config.MaxAllocation*100)
	}

	// Check total allocation doesn't exceed 100%
	totalAlloc := allocationPercent
	for _, alloc := range pm.allocations {
		totalAlloc += alloc.AllocationPercent
	}
	if totalAlloc > 1.0 {
		return fmt.Errorf("total allocation would exceed 100%% (currently: %.2f%%)", totalAlloc*100)
	}

	pm.strategies[id] = s

	allocation := &StrategyAllocation{
		StrategyID:        id,
		AllocationPercent: allocationPercent,
		AllocatedCapital:  pm.config.TotalCapital * allocationPercent,
		IsActive:          s.IsRunning(),
		LastUpdate:        time.Now(),
	}

	pm.allocations[id] = allocation
	pm.config.StrategyAllocation[id] = allocationPercent

	log.Printf("[PortfolioManager] Added strategy %s with %.2f%% allocation (%.2f capital)",
		id, allocationPercent*100, allocation.AllocatedCapital)

	return nil
}

// RemoveStrategy removes a strategy from the portfolio
func (pm *PortfolioManager) RemoveStrategy(strategyID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.strategies[strategyID]; !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	delete(pm.strategies, strategyID)
	delete(pm.allocations, strategyID)
	delete(pm.config.StrategyAllocation, strategyID)

	log.Printf("[PortfolioManager] Removed strategy %s", strategyID)
	return nil
}

// UpdateAllocations updates portfolio allocations
func (pm *PortfolioManager) UpdateAllocations() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	totalPnL := 0.0
	totalExposure := 0.0
	activeCount := 0

	// Update strategy allocations
	for id, s := range pm.strategies {
		alloc := pm.allocations[id]

		pnl := s.GetPNL()
		position := s.GetEstimatedPosition() // Estimated position (NOT real CTP!)
		riskMetrics := s.GetRiskMetrics()

		alloc.CurrentPnL = pnl.TotalPnL
		alloc.CurrentReturn = 0
		if alloc.AllocatedCapital > 0 {
			alloc.CurrentReturn = pnl.TotalPnL / alloc.AllocatedCapital
		}
		alloc.CurrentExposure = riskMetrics.ExposureValue
		alloc.PositionSize = position.NetQty // Estimated position
		alloc.IsActive = s.IsRunning()
		alloc.LastUpdate = time.Now()

		totalPnL += pnl.TotalPnL
		totalExposure += riskMetrics.ExposureValue

		if s.IsRunning() {
			activeCount++
		}
	}

	// Update portfolio stats
	pm.stats.TotalPnL = totalPnL
	pm.stats.TotalReturn = totalPnL / pm.config.TotalCapital
	pm.stats.NumStrategies = len(pm.strategies)
	pm.stats.NumActiveStrategies = activeCount
	pm.stats.Timestamp = time.Now()

	// Update PnL history for Sharpe calculation
	pm.pnlHistory = append(pm.pnlHistory, totalPnL)
	if len(pm.pnlHistory) > pm.maxPnLHistory {
		pm.pnlHistory = pm.pnlHistory[1:]
	}

	// Calculate Sharpe ratio
	pm.stats.SharpeRatio = pm.calculateSharpeRatio()

	// Calculate allocated vs free capital
	allocatedCapital := 0.0
	for _, alloc := range pm.allocations {
		if alloc.IsActive {
			allocatedCapital += alloc.AllocatedCapital
		}
	}
	pm.stats.AllocatedCapital = allocatedCapital
	pm.stats.FreeCapital = pm.config.TotalCapital - allocatedCapital

	return nil
}

// Rebalance rebalances the portfolio
func (pm *PortfolioManager) Rebalance() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	log.Println("[PortfolioManager] Rebalancing portfolio...")

	// Simple equal-weight rebalancing
	// More sophisticated methods could be implemented:
	// - Risk parity
	// - Mean-variance optimization
	// - Kelly criterion

	numStrategies := len(pm.strategies)
	if numStrategies == 0 {
		return nil
	}

	// Equal weight allocation
	equalWeight := 1.0 / float64(numStrategies)

	// Ensure within bounds
	if equalWeight < pm.config.MinAllocation {
		equalWeight = pm.config.MinAllocation
	}
	if equalWeight > pm.config.MaxAllocation {
		equalWeight = pm.config.MaxAllocation
	}

	// Update allocations
	for id := range pm.strategies {
		alloc := pm.allocations[id]
		alloc.AllocationPercent = equalWeight
		alloc.AllocatedCapital = pm.config.TotalCapital * equalWeight
		pm.config.StrategyAllocation[id] = equalWeight

		log.Printf("[PortfolioManager] Rebalanced %s: %.2f%% (%.2f capital)",
			id, equalWeight*100, alloc.AllocatedCapital)
	}

	return nil
}

// CalculateCorrelation calculates correlation matrix between strategies
func (pm *PortfolioManager) CalculateCorrelation() (*CorrelationMatrix, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.config.EnableCorrelationCalc {
		return nil, fmt.Errorf("correlation calculation disabled")
	}

	n := len(pm.strategies)
	if n < 2 {
		return nil, fmt.Errorf("need at least 2 strategies for correlation")
	}

	// Get strategy IDs
	ids := make([]string, 0, n)
	for id := range pm.strategies {
		ids = append(ids, id)
	}

	// Initialize correlation matrix
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		// Diagonal is 1.0 (self-correlation)
		matrix[i][i] = 1.0
	}

	// Calculate pairwise correlations
	// Note: This is a simplified implementation
	// Real implementation would need return time series
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Placeholder: random correlation for demo
			// Real implementation: corr := calculatePearsonCorrelation(returns1, returns2)
			corr := 0.0 // Simplified: assume uncorrelated

			matrix[i][j] = corr
			matrix[j][i] = corr
		}
	}

	correlation := &CorrelationMatrix{
		StrategyIDs: ids,
		Matrix:      matrix,
		Timestamp:   time.Now(),
	}

	pm.correlation = correlation

	return correlation, nil
}

// rebalanceLoop periodically rebalances the portfolio
func (pm *PortfolioManager) rebalanceLoop() {
	defer pm.wg.Done()

	ticker := time.NewTicker(time.Duration(pm.config.RebalanceIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := pm.Rebalance(); err != nil {
				log.Printf("[PortfolioManager] Rebalancing error: %v", err)
			}

			if pm.config.EnableCorrelationCalc {
				if _, err := pm.CalculateCorrelation(); err != nil {
					log.Printf("[PortfolioManager] Correlation calculation error: %v", err)
				}
			}

		case <-pm.stopChan:
			return
		}
	}
}

// calculateSharpeRatio calculates Sharpe ratio from PnL history
func (pm *PortfolioManager) calculateSharpeRatio() float64 {
	if len(pm.pnlHistory) < 2 {
		return 0.0
	}

	// Calculate returns
	returns := make([]float64, len(pm.pnlHistory)-1)
	for i := 1; i < len(pm.pnlHistory); i++ {
		returns[i-1] = pm.pnlHistory[i] - pm.pnlHistory[i-1]
	}

	// Calculate mean return
	var sum float64
	for _, r := range returns {
		sum += r
	}
	meanReturn := sum / float64(len(returns))

	// Calculate standard deviation
	var variance float64
	for _, r := range returns {
		diff := r - meanReturn
		variance += diff * diff
	}
	variance /= float64(len(returns))
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0.0
	}

	// Sharpe ratio (assuming risk-free rate = 0)
	sharpeRatio := meanReturn / stdDev

	// Annualize (assuming daily returns)
	sharpeRatio *= math.Sqrt(252)

	return sharpeRatio
}

// GetStats returns portfolio statistics
func (pm *PortfolioManager) GetStats() *PortfolioStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Create a copy
	stats := *pm.stats
	return &stats
}

// GetAllocation returns allocation for a strategy
func (pm *PortfolioManager) GetAllocation(strategyID string) (*StrategyAllocation, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	alloc, ok := pm.allocations[strategyID]
	if !ok {
		return nil, fmt.Errorf("strategy %s not found", strategyID)
	}

	// Return a copy
	allocCopy := *alloc
	return &allocCopy, nil
}

// GetAllAllocations returns all strategy allocations
func (pm *PortfolioManager) GetAllAllocations() map[string]*StrategyAllocation {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*StrategyAllocation)
	for id, alloc := range pm.allocations {
		allocCopy := *alloc
		result[id] = &allocCopy
	}

	return result
}

// GetCorrelationMatrix returns the correlation matrix
func (pm *PortfolioManager) GetCorrelationMatrix() *CorrelationMatrix {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.correlation == nil {
		return nil
	}

	// Return a copy
	return pm.correlation
}

// PrintReport prints a portfolio report
func (pm *PortfolioManager) PrintReport() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║               Portfolio Manager Report                     ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Total Capital:      %-39.2f║\n", pm.stats.TotalCapital)
	fmt.Printf("║ Allocated:          %-39.2f║\n", pm.stats.AllocatedCapital)
	fmt.Printf("║ Free:               %-39.2f║\n", pm.stats.FreeCapital)
	fmt.Printf("║ Total P&L:          %-39.2f║\n", pm.stats.TotalPnL)
	fmt.Printf("║ Return:             %-38.2f%%║\n", pm.stats.TotalReturn*100)
	fmt.Printf("║ Sharpe Ratio:       %-39.2f║\n", pm.stats.SharpeRatio)
	fmt.Printf("║ Max Drawdown:       %-39.2f║\n", pm.stats.MaxDrawdown)
	fmt.Printf("║ Strategies:         %-39d║\n", pm.stats.NumStrategies)
	fmt.Printf("║ Active:             %-39d║\n", pm.stats.NumActiveStrategies)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║                   Strategy Allocations                     ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")

	for id, alloc := range pm.allocations {
		status := "INACTIVE"
		if alloc.IsActive {
			status = "ACTIVE"
		}
		fmt.Printf("║ %-15s %-8s %6.2f%% %12.2f %8.2f%%║\n",
			id, status,
			alloc.AllocationPercent*100,
			alloc.AllocatedCapital,
			alloc.CurrentReturn*100)
	}

	fmt.Println("╚════════════════════════════════════════════════════════════╝")
}
