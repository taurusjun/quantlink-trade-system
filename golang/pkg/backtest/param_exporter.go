package backtest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// OptimalParams represents optimized parameters for production
type OptimalParams struct {
	// Metadata
	GeneratedAt   time.Time `yaml:"generated_at"`
	BacktestDate  string    `yaml:"backtest_date"`
	DataPeriod    string    `yaml:"data_period"`
	OptimizationGoal string `yaml:"optimization_goal"`

	// Strategy info
	Strategy StrategyInfo `yaml:"strategy"`

	// Optimized parameters
	Parameters map[string]interface{} `yaml:"parameters"`

	// Performance metrics
	Performance PerformanceMetrics `yaml:"performance"`

	// Risk parameters
	Risk RiskParams `yaml:"risk"`
}

// StrategyInfo contains strategy identification
type StrategyInfo struct {
	Type    string   `yaml:"type"`
	Symbols []string `yaml:"symbols"`
}

// PerformanceMetrics contains backtest performance
type PerformanceMetrics struct {
	SharpeRatio     float64 `yaml:"sharpe_ratio"`
	SortinoRatio    float64 `yaml:"sortino_ratio"`
	MaxDrawdown     float64 `yaml:"max_drawdown"`
	TotalReturn     float64 `yaml:"total_return"`
	AnnualizedReturn float64 `yaml:"annualized_return"`
	WinRate         float64 `yaml:"win_rate"`
	ProfitFactor    float64 `yaml:"profit_factor"`
	TotalTrades     int     `yaml:"total_trades"`
	TotalPNL        float64 `yaml:"total_pnl"`
	CalmarRatio     float64 `yaml:"calmar_ratio"`
}

// RiskParams contains risk management parameters
type RiskParams struct {
	StopLoss       float64 `yaml:"stop_loss"`
	MaxLoss        float64 `yaml:"max_loss"`
	MaxDrawdown    float64 `yaml:"max_drawdown"`
	DailyLossLimit float64 `yaml:"daily_loss_limit"`
}

// ParamExporter exports optimized parameters
type ParamExporter struct {
	outputDir string
}

// NewParamExporter creates a new parameter exporter
func NewParamExporter(outputDir string) *ParamExporter {
	return &ParamExporter{
		outputDir: outputDir,
	}
}

// ExportOptimalParams exports optimal parameters to YAML file
func (e *ParamExporter) ExportOptimalParams(
	config *BacktestConfig,
	result *BacktestResult,
	optimizationGoal string,
) (string, error) {
	// Create output directory if not exists
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build optimal params structure
	params := &OptimalParams{
		GeneratedAt:      time.Now(),
		BacktestDate:     time.Now().Format("2006-01-02"),
		DataPeriod:       fmt.Sprintf("%s to %s", config.Backtest.StartDate, config.Backtest.EndDate),
		OptimizationGoal: optimizationGoal,

		Strategy: StrategyInfo{
			Type:    config.Strategy.Type,
			Symbols: config.Strategy.Symbols,
		},

		Parameters: config.Strategy.Parameters,

		Performance: PerformanceMetrics{
			SharpeRatio:      result.SharpeRatio,
			SortinoRatio:     result.SortinoRatio,
			MaxDrawdown:      result.MaxDrawdown,
			TotalReturn:      result.TotalReturn,
			AnnualizedReturn: result.AnnualizedReturn,
			WinRate:          result.WinRate,
			ProfitFactor:     result.ProfitFactor,
			TotalTrades:      result.TotalTrades,
			TotalPNL:         result.TotalPNL,
			CalmarRatio:      result.CalmarRatio,
		},

		Risk: RiskParams{
			StopLoss:       100000,                    // Default
			MaxLoss:        100000,                    // Default
			MaxDrawdown:    result.MaxDrawdown * 1.5,  // 50% safety margin
			DailyLossLimit: result.TotalPNL * 0.1,     // 10% of total PNL
		},
	}

	// Generate filename
	symbolStr := ""
	for i, symbol := range config.Strategy.Symbols {
		if i > 0 {
			symbolStr += "_"
		}
		symbolStr += symbol
	}
	filename := fmt.Sprintf("optimal_params_%s_%s.yaml",
		symbolStr, time.Now().Format("20060102"))
	filepath := filepath.Join(e.outputDir, filename)

	// Write to YAML file
	data, err := yaml.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal parameters: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write parameters file: %w", err)
	}

	return filepath, nil
}

// ExportOptimizationResults exports all optimization results
func (e *ParamExporter) ExportOptimizationResults(
	config *BacktestConfig,
	results []*OptimizationResult,
	goal OptimizationGoal,
) (string, error) {
	// Create output directory
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build export structure
	export := struct {
		GeneratedAt      time.Time              `yaml:"generated_at"`
		OptimizationGoal string                 `yaml:"optimization_goal"`
		Strategy         StrategyInfo           `yaml:"strategy"`
		TotalTests       int                    `yaml:"total_tests"`
		Results          []*OptimizationResult  `yaml:"results"`
	}{
		GeneratedAt:      time.Now(),
		OptimizationGoal: string(goal),
		Strategy: StrategyInfo{
			Type:    config.Strategy.Type,
			Symbols: config.Strategy.Symbols,
		},
		TotalTests: len(results),
		Results:    results,
	}

	// Generate filename
	symbolStr := ""
	for i, symbol := range config.Strategy.Symbols {
		if i > 0 {
			symbolStr += "_"
		}
		symbolStr += symbol
	}
	filename := fmt.Sprintf("optimization_results_%s_%s.yaml",
		symbolStr, time.Now().Format("20060102_150405"))
	filepath := filepath.Join(e.outputDir, filename)

	// Write to YAML file
	data, err := yaml.Marshal(export)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write results file: %w", err)
	}

	return filepath, nil
}

// LoadOptimalParams loads optimal parameters from file
func LoadOptimalParams(filepath string) (*OptimalParams, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read parameters file: %w", err)
	}

	var params OptimalParams
	if err := yaml.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	return &params, nil
}

// ArchiveOptimalParams archives the current optimal params
func (e *ParamExporter) ArchiveOptimalParams(currentFile, archiveDir string) error {
	// Create archive directory
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Read current params
	params, err := LoadOptimalParams(currentFile)
	if err != nil {
		return fmt.Errorf("failed to load current params: %w", err)
	}

	// Generate archive filename with timestamp and score
	symbolStr := ""
	for i, symbol := range params.Strategy.Symbols {
		if i > 0 {
			symbolStr += "_"
		}
		symbolStr += symbol
	}
	archiveFilename := fmt.Sprintf("optimal_params_%s_%s_sharpe%.2f.yaml",
		symbolStr,
		time.Now().Format("20060102"),
		params.Performance.SharpeRatio)
	archivePath := filepath.Join(archiveDir, archiveFilename)

	// Copy to archive
	data, err := os.ReadFile(currentFile)
	if err != nil {
		return fmt.Errorf("failed to read current file: %w", err)
	}

	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write archive file: %w", err)
	}

	return nil
}

// CompareParams compares two parameter sets
func CompareParams(baseline, current *OptimalParams) string {
	result := fmt.Sprintf("Parameter Comparison\n")
	result += fmt.Sprintf("===================\n\n")

	// Strategy info
	result += fmt.Sprintf("Strategy: %s\n", current.Strategy.Type)
	result += fmt.Sprintf("Symbols: %v\n\n", current.Strategy.Symbols)

	// Performance comparison
	result += fmt.Sprintf("Performance Metrics:\n")
	result += fmt.Sprintf("  Sharpe Ratio:      %.4f -> %.4f (%.2f%%)\n",
		baseline.Performance.SharpeRatio,
		current.Performance.SharpeRatio,
		(current.Performance.SharpeRatio/baseline.Performance.SharpeRatio-1)*100)

	result += fmt.Sprintf("  Total PNL:         %.2f -> %.2f (%.2f%%)\n",
		baseline.Performance.TotalPNL,
		current.Performance.TotalPNL,
		(current.Performance.TotalPNL/baseline.Performance.TotalPNL-1)*100)

	result += fmt.Sprintf("  Max Drawdown:      %.4f -> %.4f\n",
		baseline.Performance.MaxDrawdown,
		current.Performance.MaxDrawdown)

	result += fmt.Sprintf("  Win Rate:          %.2f%% -> %.2f%%\n",
		baseline.Performance.WinRate*100,
		current.Performance.WinRate*100)

	result += fmt.Sprintf("\n")

	// Parameter changes
	result += fmt.Sprintf("Parameter Changes:\n")
	for key := range current.Parameters {
		baselineVal := baseline.Parameters[key]
		currentVal := current.Parameters[key]
		result += fmt.Sprintf("  %-20s: %v -> %v\n", key, baselineVal, currentVal)
	}

	return result
}
