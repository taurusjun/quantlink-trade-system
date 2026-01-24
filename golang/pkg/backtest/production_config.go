package backtest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
)

// ProductionConfigGenerator generates production trading config from backtest results
type ProductionConfigGenerator struct {
	outputDir string
}

// NewProductionConfigGenerator creates a new production config generator
func NewProductionConfigGenerator(outputDir string) *ProductionConfigGenerator {
	return &ProductionConfigGenerator{
		outputDir: outputDir,
	}
}

// GenerateTraderConfig generates trader.yaml from optimal parameters
func (g *ProductionConfigGenerator) GenerateTraderConfig(
	params *OptimalParams,
	strategyID string,
) (string, error) {
	// Create output directory
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build trader config
	traderConfig := &config.TraderConfig{
		System: config.SystemConfig{
			StrategyID: strategyID,
			Mode:       "simulation", // Default to simulation mode
		},
		Strategy: config.StrategyConfig{
			Type:       params.Strategy.Type,
			Symbols:    params.Strategy.Symbols,
			Parameters: params.Parameters,
			// Set conservative position limits
			MaxPositionSize: int64(getIntParam(params.Parameters, "max_position_size", 10)),
			MaxExposure:     1000000.0, // 1M default
		},
		Session: config.SessionConfig{
			StartTime:    "09:00:00",
			EndTime:      "15:00:00",
			Timezone:     "Asia/Shanghai",
			AutoStart:    false,
			AutoStop:     true,
			AutoActivate: false, // Require manual activation for safety
		},
		Risk: config.RiskConfig{
			MaxDrawdown:    params.Risk.MaxDrawdown,
			StopLoss:       params.Risk.StopLoss,
			MaxLoss:        params.Risk.MaxLoss,
			DailyLossLimit: params.Risk.DailyLossLimit,
			MaxRejectCount: 10,
			CheckIntervalMs: 100,
		},
		Engine: config.EngineConfig{
			ORSGatewayAddr:      "localhost:50051", // Default ORS address
			NATSAddr:            "nats://localhost:4222",
			OrderQueueSize:      100,
			TimerInterval:       5 * time.Second,
			MaxConcurrentOrders: 10,
		},
		Portfolio: config.PortfolioConfig{
			TotalCapital:         1000000.0, // 1M default
			MinAllocation:        0.0,
			MaxAllocation:        1.0,
			EnableAutoRebalance:  false,
			EnableCorrelation:    true,
		},
		API: config.APIConfig{
			Enabled: true,
			Port:    9201,
			Host:    "localhost",
		},
		Logging: config.LoggingConfig{
			Level:      "info",
			File:       fmt.Sprintf("log/trader_%s.log", strategyID),
			MaxSizeMB:  100,
			MaxBackups: 10,
			MaxAgeDays: 30,
			Console:    true,
			JSONFormat: false,
		},
	}

	// Generate filename
	filename := fmt.Sprintf("trader_%s_optimized_%s.yaml",
		strategyID, time.Now().Format("20060102"))
	filepath := filepath.Join(g.outputDir, filename)

	// Write to YAML file
	if err := config.SaveTraderConfig(filepath, traderConfig); err != nil {
		return "", fmt.Errorf("failed to save trader config: %w", err)
	}

	return filepath, nil
}

// GenerateControlFile generates control file (similar to old system)
func (g *ProductionConfigGenerator) GenerateControlFile(
	params *OptimalParams,
	strategyID string,
) (string, error) {
	if len(params.Strategy.Symbols) < 2 {
		return "", fmt.Errorf("at least 2 symbols required for control file")
	}

	// Create output directory
	controlDir := filepath.Join(g.outputDir, "controls")
	if err := os.MkdirAll(controlDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create control directory: %w", err)
	}

	// Build control file content
	symbol1 := params.Strategy.Symbols[0]
	symbol2 := params.Strategy.Symbols[1]

	// Generate model filename
	modelFilename := fmt.Sprintf("model.%s.%s.par.txt.%s",
		symbol1, symbol2, strategyID)

	// Control file format (similar to old system)
	// Format: symbol1 modelPath exchange size strategyType startTime endTime symbol2
	controlContent := fmt.Sprintf("%s_F_2_SFE ./models/%s SFE %d TB_PAIR_STRAT 0900 1500 %s_F_4_SFE\n",
		symbol1[:2], // Extract base symbol (e.g., "ag" from "ag2502")
		modelFilename,
		getIntParam(params.Parameters, "position_size", 4),
		symbol2[:2],
	)

	// Write control file
	controlFilename := fmt.Sprintf("control.%s.%s.par.txt.%s",
		symbol1, symbol2, strategyID)
	controlPath := filepath.Join(controlDir, controlFilename)

	if err := os.WriteFile(controlPath, []byte(controlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write control file: %w", err)
	}

	return controlPath, nil
}

// GenerateModelFile generates model file with optimized parameters
func (g *ProductionConfigGenerator) GenerateModelFile(
	params *OptimalParams,
	strategyID string,
) (string, error) {
	if len(params.Strategy.Symbols) < 2 {
		return "", fmt.Errorf("at least 2 symbols required for model file")
	}

	// Create output directory
	modelDir := filepath.Join(g.outputDir, "models")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	symbol1 := params.Strategy.Symbols[0]
	symbol2 := params.Strategy.Symbols[1]

	// Build model file content (similar to old system format)
	content := fmt.Sprintf("%s_F_2_SFE FUTCOM Dependant 0 MID_PX\n", symbol1[:2])
	content += fmt.Sprintf("%s_F_4_SFE FUTCOM Dependant 0 MID_PX\n", symbol2[:2])
	content += "MAX_QUOTE_LEVEL 3\n"

	// Position sizing
	content += fmt.Sprintf("SIZE %d\n", getIntParam(params.Parameters, "position_size", 4))
	content += fmt.Sprintf("MAX_SIZE %d\n", getIntParam(params.Parameters, "max_position_size", 16))
	content += fmt.Sprintf("BID_SIZE %d\n", getIntParam(params.Parameters, "position_size", 4))
	content += fmt.Sprintf("BID_MAX_SIZE %d\n", getIntParam(params.Parameters, "max_position_size", 32))
	content += fmt.Sprintf("ASK_SIZE %d\n", getIntParam(params.Parameters, "position_size", 4))
	content += "ASK_MAX_SIZE 0\n"

	// Trading thresholds (from optimized parameters)
	entryZscore := getFloatParam(params.Parameters, "entry_zscore", 2.0)
	exitZscore := getFloatParam(params.Parameters, "exit_zscore", 0.5)

	content += fmt.Sprintf("BEGIN_PLACE %.6f\n", entryZscore)
	content += fmt.Sprintf("LONG_PLACE %.6f\n", entryZscore*1.5)
	content += fmt.Sprintf("SHORT_PLACE %.6f\n", entryZscore*0.5)
	content += fmt.Sprintf("BEGIN_REMOVE %.6f\n", exitZscore)
	content += fmt.Sprintf("LONG_REMOVE %.6f\n", exitZscore*1.5)
	content += fmt.Sprintf("SHORT_REMOVE %.6f\n", exitZscore*0.5)

	content += "CROSS 3\n"
	content += "SUPPORTING_ORDERS 2\n"

	// Risk parameters
	content += fmt.Sprintf("UPNL_LOSS %.0f\n", params.Risk.StopLoss)
	content += fmt.Sprintf("STOP_LOSS %.0f\n", params.Risk.StopLoss)
	content += fmt.Sprintf("MAX_LOSS %.0f\n", params.Risk.MaxLoss)
	content += "PIL_FACTOR 0\n"
	content += "OPP_QTY 1000\n"
	content += "PRICE_RATIO 1\n"
	content += "HEDGE_THRES 1\n"
	content += "HEDGE_SIZE_RATIO 1\n"

	// Alpha coefficient (if available)
	alpha := getFloatParam(params.Parameters, "alpha", 0.00002407)
	content += fmt.Sprintf("ALPHA %.10f\n", alpha)

	// Spread parameters
	lookback := getIntParam(params.Parameters, "lookback_window", 100)
	content += fmt.Sprintf("AVG_SPREAD_AWAY %d\n", lookback/5)

	// Write model file
	modelFilename := fmt.Sprintf("model.%s.%s.par.txt.%s",
		symbol1, symbol2, strategyID)
	modelPath := filepath.Join(modelDir, modelFilename)

	if err := os.WriteFile(modelPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write model file: %w", err)
	}

	return modelPath, nil
}

// GenerateStartScript generates start script
func (g *ProductionConfigGenerator) GenerateStartScript(
	params *OptimalParams,
	strategyID string,
) (string, error) {
	if len(params.Strategy.Symbols) < 2 {
		return "", fmt.Errorf("at least 2 symbols required")
	}

	// Create output directory
	scriptDir := filepath.Join(g.outputDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create script directory: %w", err)
	}

	symbol1 := params.Strategy.Symbols[0]
	symbol2 := params.Strategy.Symbols[1]

	// Build script content
	content := "#!/bin/bash\n"
	content += "# Auto-generated start script from backtest optimization\n"
	content += fmt.Sprintf("# Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	content += fmt.Sprintf("# Optimization goal: %s\n", params.OptimizationGoal)
	content += fmt.Sprintf("# Sharpe Ratio: %.2f\n\n", params.Performance.SharpeRatio)

	// Trader config file
	traderConfigFile := fmt.Sprintf("config/trader_%s_optimized_%s.yaml",
		strategyID, time.Now().Format("20060102"))

	content += "# Check if NATS is running\n"
	content += "if ! pgrep -x \"nats-server\" > /dev/null; then\n"
	content += "    echo \"Starting NATS server...\"\n"
	content += "    nats-server &\n"
	content += "    sleep 2\n"
	content += "fi\n\n"

	content += "# Start trader\n"
	content += fmt.Sprintf("echo \"Starting trader for %s/%s (Strategy %s)...\"\n",
		symbol1, symbol2, strategyID)
	content += fmt.Sprintf("nohup ./bin/trader -config %s > log/nohup_%s.out 2>&1 &\n\n",
		traderConfigFile, strategyID)

	content += "# Display process info\n"
	content += "sleep 2\n"
	content += "echo \"Trader started. PID: $(pgrep -f trader | tail -1)\"\n"
	content += fmt.Sprintf("echo \"Log file: log/trader_%s.log\"\n", strategyID)
	content += fmt.Sprintf("echo \"API endpoint: http://localhost:9201\"\n")
	content += "echo \"\"\n"
	content += "echo \"To activate strategy, run:\"\n"
	content += fmt.Sprintf("echo \"  curl -X POST http://localhost:9201/api/v1/strategy/activate\"\n")

	// Write script file
	scriptFilename := fmt.Sprintf("start_%s_%s.sh", symbol1, symbol2)
	scriptPath := filepath.Join(scriptDir, scriptFilename)

	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}

	return scriptPath, nil
}

// GenerateAllProductionFiles generates all production files at once
func (g *ProductionConfigGenerator) GenerateAllProductionFiles(
	params *OptimalParams,
	strategyID string,
) (map[string]string, error) {
	files := make(map[string]string)

	// Generate trader config
	traderConfigPath, err := g.GenerateTraderConfig(params, strategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate trader config: %w", err)
	}
	files["trader_config"] = traderConfigPath

	// Generate control file
	controlPath, err := g.GenerateControlFile(params, strategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate control file: %w", err)
	}
	files["control_file"] = controlPath

	// Generate model file
	modelPath, err := g.GenerateModelFile(params, strategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate model file: %w", err)
	}
	files["model_file"] = modelPath

	// Generate start script
	scriptPath, err := g.GenerateStartScript(params, strategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate start script: %w", err)
	}
	files["start_script"] = scriptPath

	return files, nil
}

// Helper functions to extract parameters with defaults
func getFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case float32:
			return int(v)
		}
	}
	return defaultVal
}
