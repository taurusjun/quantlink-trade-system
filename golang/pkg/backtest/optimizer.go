package backtest

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// ParameterOptimizer performs parameter optimization using grid search
type ParameterOptimizer struct {
	baseConfig    *BacktestConfig
	paramRanges   map[string]*ParamRange
	goal          OptimizationGoal
	maxWorkers    int
	resultChannel chan *OptimizationResult
}

// ParamRange defines the range for a parameter
type ParamRange struct {
	Name  string
	Min   float64
	Max   float64
	Step  float64
	Type  ParamType
}

// ParamType indicates how to interpret the parameter
type ParamType int

const (
	ParamTypeFloat ParamType = iota
	ParamTypeInt
)

// OptimizationGoal defines the optimization objective
type OptimizationGoal string

const (
	GoalSharpeRatio  OptimizationGoal = "sharpe"
	GoalTotalPNL     OptimizationGoal = "pnl"
	GoalWinRate      OptimizationGoal = "win_rate"
	GoalProfitFactor OptimizationGoal = "profit_factor"
	GoalCalmarRatio  OptimizationGoal = "calmar"
)

// OptimizationResult stores the result of a single parameter combination
type OptimizationResult struct {
	Parameters map[string]float64
	Metrics    OptimizationMetrics
	Rank       int
	Score      float64
}

// OptimizationMetrics contains key performance metrics
type OptimizationMetrics struct {
	SharpeRatio  float64
	TotalPNL     float64
	TotalReturn  float64
	MaxDrawdown  float64
	WinRate      float64
	ProfitFactor float64
	CalmarRatio  float64
	TotalTrades  int
}

// NewParameterOptimizer creates a new parameter optimizer
func NewParameterOptimizer(baseConfig *BacktestConfig) *ParameterOptimizer {
	return &ParameterOptimizer{
		baseConfig:    baseConfig,
		paramRanges:   make(map[string]*ParamRange),
		goal:          GoalSharpeRatio,
		maxWorkers:    4, // Default: 4 parallel workers
		resultChannel: make(chan *OptimizationResult, 100),
	}
}

// AddParamRange adds a parameter range for optimization
func (opt *ParameterOptimizer) AddParamRange(name string, min, max, step float64, paramType ParamType) {
	opt.paramRanges[name] = &ParamRange{
		Name: name,
		Min:  min,
		Max:  max,
		Step: step,
		Type: paramType,
	}
}

// SetOptimizationGoal sets the optimization objective
func (opt *ParameterOptimizer) SetOptimizationGoal(goal OptimizationGoal) {
	opt.goal = goal
}

// SetMaxWorkers sets the maximum number of parallel workers
func (opt *ParameterOptimizer) SetMaxWorkers(workers int) {
	if workers < 1 {
		workers = 1
	}
	if workers > 16 {
		workers = 16
	}
	opt.maxWorkers = workers
}

// GridSearch performs grid search optimization
func (opt *ParameterOptimizer) GridSearch() ([]*OptimizationResult, error) {
	log.Println("[Optimizer] Starting grid search optimization...")
	log.Printf("[Optimizer] Optimization goal: %s", opt.goal)
	log.Printf("[Optimizer] Max workers: %d", opt.maxWorkers)

	// Generate all parameter combinations
	combinations := opt.generateCombinations()
	totalCombinations := len(combinations)
	log.Printf("[Optimizer] Total parameter combinations: %d", totalCombinations)

	if totalCombinations == 0 {
		return nil, fmt.Errorf("no parameter combinations to test")
	}

	// Run backtests in parallel
	results := make([]*OptimizationResult, 0, totalCombinations)
	var resultsMutex sync.Mutex
	var wg sync.WaitGroup

	// Create worker pool
	semaphore := make(chan struct{}, opt.maxWorkers)
	startTime := time.Now()

	for i, params := range combinations {
		wg.Add(1)
		go func(idx int, paramSet map[string]float64) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Run backtest with this parameter set
			result, err := opt.runBacktestWithParams(paramSet)
			if err != nil {
				log.Printf("[Optimizer] Error testing combination %d/%d: %v", idx+1, totalCombinations, err)
				return
			}

			// Store result
			resultsMutex.Lock()
			results = append(results, result)
			progress := float64(len(results)) / float64(totalCombinations) * 100
			log.Printf("[Optimizer] Progress: %d/%d (%.1f%%) - Score: %.4f",
				len(results), totalCombinations, progress, result.Score)
			resultsMutex.Unlock()
		}(i, params)
	}

	// Wait for all workers to complete
	wg.Wait()
	duration := time.Since(startTime)

	log.Printf("[Optimizer] Grid search completed in %v", duration)
	log.Printf("[Optimizer] Successful tests: %d/%d", len(results), totalCombinations)

	// Sort results by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Assign ranks
	for i, result := range results {
		result.Rank = i + 1
	}

	log.Println("[Optimizer] Top 5 parameter combinations:")
	for i := 0; i < 5 && i < len(results); i++ {
		result := results[i]
		log.Printf("[Optimizer]   #%d: Score=%.4f, Sharpe=%.2f, PNL=%.2f, Params=%v",
			result.Rank, result.Score, result.Metrics.SharpeRatio,
			result.Metrics.TotalPNL, result.Parameters)
	}

	return results, nil
}

// generateCombinations generates all parameter combinations
func (opt *ParameterOptimizer) generateCombinations() []map[string]float64 {
	// Get sorted parameter names for consistent ordering
	paramNames := make([]string, 0, len(opt.paramRanges))
	for name := range opt.paramRanges {
		paramNames = append(paramNames, name)
	}
	sort.Strings(paramNames)

	// Generate value arrays for each parameter
	paramValues := make([][]float64, len(paramNames))
	for i, name := range paramNames {
		paramRange := opt.paramRanges[name]
		values := make([]float64, 0)
		for v := paramRange.Min; v <= paramRange.Max; v += paramRange.Step {
			if paramRange.Type == ParamTypeInt {
				values = append(values, float64(int(v)))
			} else {
				values = append(values, v)
			}
		}
		paramValues[i] = values
	}

	// Generate all combinations
	combinations := make([]map[string]float64, 0)
	opt.generateCombinationsRecursive(paramNames, paramValues, 0, make(map[string]float64), &combinations)

	return combinations
}

// generateCombinationsRecursive recursively generates combinations
func (opt *ParameterOptimizer) generateCombinationsRecursive(
	paramNames []string,
	paramValues [][]float64,
	depth int,
	current map[string]float64,
	result *[]map[string]float64,
) {
	if depth == len(paramNames) {
		// Copy current combination
		combo := make(map[string]float64)
		for k, v := range current {
			combo[k] = v
		}
		*result = append(*result, combo)
		return
	}

	// Try all values for current parameter
	paramName := paramNames[depth]
	for _, value := range paramValues[depth] {
		current[paramName] = value
		opt.generateCombinationsRecursive(paramNames, paramValues, depth+1, current, result)
	}
}

// runBacktestWithParams runs a backtest with given parameters
func (opt *ParameterOptimizer) runBacktestWithParams(params map[string]float64) (*OptimizationResult, error) {
	// Create a copy of base config
	testConfig := opt.copyConfig()

	// Apply parameter overrides
	for name, value := range params {
		testConfig.Strategy.Parameters[name] = value
	}

	// Run backtest
	runner, err := NewBacktestRunner(testConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	backtestResult, err := runner.Run()
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %w", err)
	}

	// Extract metrics
	metrics := OptimizationMetrics{
		SharpeRatio:  backtestResult.SharpeRatio,
		TotalPNL:     backtestResult.TotalPNL,
		TotalReturn:  backtestResult.TotalReturn,
		MaxDrawdown:  backtestResult.MaxDrawdown,
		WinRate:      backtestResult.WinRate,
		ProfitFactor: backtestResult.ProfitFactor,
		CalmarRatio:  backtestResult.CalmarRatio,
		TotalTrades:  backtestResult.TotalTrades,
	}

	// Calculate score based on optimization goal
	score := opt.calculateScore(&metrics)

	return &OptimizationResult{
		Parameters: params,
		Metrics:    metrics,
		Score:      score,
	}, nil
}

// calculateScore calculates the optimization score
func (opt *ParameterOptimizer) calculateScore(metrics *OptimizationMetrics) float64 {
	switch opt.goal {
	case GoalSharpeRatio:
		return metrics.SharpeRatio
	case GoalTotalPNL:
		return metrics.TotalPNL
	case GoalWinRate:
		return metrics.WinRate
	case GoalProfitFactor:
		return metrics.ProfitFactor
	case GoalCalmarRatio:
		return metrics.CalmarRatio
	default:
		return metrics.SharpeRatio
	}
}

// copyConfig creates a deep copy of the config
func (opt *ParameterOptimizer) copyConfig() *BacktestConfig {
	config := &BacktestConfig{
		Backtest: opt.baseConfig.Backtest,
		Strategy: StrategySettings{
			Type:       opt.baseConfig.Strategy.Type,
			Symbols:    make([]string, len(opt.baseConfig.Strategy.Symbols)),
			Parameters: make(map[string]interface{}),
		},
		Engine: opt.baseConfig.Engine,
	}

	// Force disable Trader for optimization mode (no gRPC server)
	config.Backtest.EnableTrader = false

	copy(config.Strategy.Symbols, opt.baseConfig.Strategy.Symbols)
	for k, v := range opt.baseConfig.Strategy.Parameters {
		config.Strategy.Parameters[k] = v
	}

	return config
}

// GetBestResult returns the best optimization result
func GetBestResult(results []*OptimizationResult) *OptimizationResult {
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

// GetTopNResults returns the top N results
func GetTopNResults(results []*OptimizationResult, n int) []*OptimizationResult {
	if n > len(results) {
		n = len(results)
	}
	return results[:n]
}
