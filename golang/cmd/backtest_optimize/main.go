package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/yourusername/quantlink-trade-system/pkg/backtest"
)

var (
	configFile   = flag.String("config", "config/backtest.yaml", "Backtest configuration file")
	action       = flag.String("action", "optimize", "Action: optimize, export, compare")
	params       = flag.String("params", "", "Parameters to optimize (format: name:min:max:step,name:min:max:step)")
	goal         = flag.String("goal", "sharpe", "Optimization goal: sharpe, pnl, win_rate, profit_factor, calmar")
	workers      = flag.Int("workers", 4, "Number of parallel workers")
	outputDir    = flag.String("output", "backtest_results/optimal_params", "Output directory")
	strategyID   = flag.String("strategy-id", "92201", "Strategy ID for production config")
	currentFile  = flag.String("current", "", "Current optimal params file (for compare action)")
	baselineFile = flag.String("baseline", "", "Baseline optimal params file (for compare action)")
	topN         = flag.Int("top", 10, "Number of top results to export")
)

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	switch *action {
	case "optimize":
		runOptimization()
	case "export":
		runExport()
	case "compare":
		runCompare()
	default:
		log.Fatalf("Unknown action: %s", *action)
	}
}

func runOptimization() {
	log.Println("========================================")
	log.Println("Parameter Optimization Tool")
	log.Println("========================================")

	// Load config
	config, err := backtest.LoadBacktestConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create optimizer
	optimizer := backtest.NewParameterOptimizer(config)
	optimizer.SetMaxWorkers(*workers)

	// Parse optimization goal
	var optGoal backtest.OptimizationGoal
	switch *goal {
	case "sharpe":
		optGoal = backtest.GoalSharpeRatio
	case "pnl":
		optGoal = backtest.GoalTotalPNL
	case "win_rate":
		optGoal = backtest.GoalWinRate
	case "profit_factor":
		optGoal = backtest.GoalProfitFactor
	case "calmar":
		optGoal = backtest.GoalCalmarRatio
	default:
		log.Fatalf("Unknown optimization goal: %s", *goal)
	}
	optimizer.SetOptimizationGoal(optGoal)

	// Parse parameter ranges
	if *params == "" {
		log.Fatal("No parameters specified. Use -params flag (e.g., -params entry_zscore:1.5:3.0:0.1)")
	}

	paramSpecs := strings.Split(*params, ",")
	for _, spec := range paramSpecs {
		parts := strings.Split(strings.TrimSpace(spec), ":")
		if len(parts) != 4 {
			log.Fatalf("Invalid parameter spec: %s (expected format: name:min:max:step)", spec)
		}

		name := parts[0]
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			log.Fatalf("Invalid min value for %s: %v", name, err)
		}
		max, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			log.Fatalf("Invalid max value for %s: %v", name, err)
		}
		step, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			log.Fatalf("Invalid step value for %s: %v", name, err)
		}

		// Determine param type (int if step >= 1, otherwise float)
		paramType := backtest.ParamTypeFloat
		if step >= 1.0 && min == float64(int(min)) && max == float64(int(max)) {
			paramType = backtest.ParamTypeInt
		}

		optimizer.AddParamRange(name, min, max, step, paramType)
		log.Printf("Added parameter range: %s [%.2f, %.2f] step %.2f", name, min, max, step)
	}

	// Run optimization
	results, err := optimizer.GridSearch()
	if err != nil {
		log.Fatalf("Optimization failed: %v", err)
	}

	// Export results
	exporter := backtest.NewParamExporter(*outputDir)

	// Export all results
	resultsFile, err := exporter.ExportOptimizationResults(config, results, optGoal)
	if err != nil {
		log.Fatalf("Failed to export results: %v", err)
	}
	log.Printf("Exported all results to: %s", resultsFile)

	// Export top N results as separate optimal params files
	topResults := backtest.GetTopNResults(results, *topN)
	log.Printf("\nExporting top %d results:", len(topResults))

	for i, result := range topResults {
		// Create a temporary config with optimized parameters
		tempConfig := config
		for k, v := range result.Parameters {
			tempConfig.Strategy.Parameters[k] = v
		}

		// Create a mock backtest result
		backtestResult := &backtest.BacktestResult{
			SharpeRatio:      result.Metrics.SharpeRatio,
			SortinoRatio:     result.Metrics.SharpeRatio * 0.9, // Approximation
			MaxDrawdown:      result.Metrics.MaxDrawdown,
			TotalReturn:      result.Metrics.TotalReturn,
			AnnualizedReturn: result.Metrics.TotalReturn * 2, // Approximation
			WinRate:          result.Metrics.WinRate,
			ProfitFactor:     result.Metrics.ProfitFactor,
			TotalTrades:      result.Metrics.TotalTrades,
			TotalPNL:         result.Metrics.TotalPNL,
			CalmarRatio:      result.Metrics.CalmarRatio,
		}

		paramFile, err := exporter.ExportOptimalParams(tempConfig, backtestResult, string(optGoal))
		if err != nil {
			log.Printf("  Warning: Failed to export #%d: %v", i+1, err)
			continue
		}
		log.Printf("  #%d: %s (Score: %.4f)", i+1, paramFile, result.Score)
	}

	// Print best result
	best := backtest.GetBestResult(results)
	if best != nil {
		log.Println("\n========================================")
		log.Println("Best Parameter Combination")
		log.Println("========================================")
		log.Printf("Optimization Score: %.4f", best.Score)
		log.Printf("Sharpe Ratio:       %.4f", best.Metrics.SharpeRatio)
		log.Printf("Total PNL:          %.2f", best.Metrics.TotalPNL)
		log.Printf("Max Drawdown:       %.4f", best.Metrics.MaxDrawdown)
		log.Printf("Win Rate:           %.2f%%", best.Metrics.WinRate*100)
		log.Printf("Profit Factor:      %.2f", best.Metrics.ProfitFactor)
		log.Printf("Total Trades:       %d", best.Metrics.TotalTrades)
		log.Println("\nOptimal Parameters:")
		for k, v := range best.Parameters {
			log.Printf("  %-20s: %v", k, v)
		}
		log.Println("========================================")
	}
}

func runExport() {
	log.Println("========================================")
	log.Println("Production Configuration Export")
	log.Println("========================================")

	if *currentFile == "" {
		log.Fatal("No optimal params file specified. Use -current flag")
	}

	// Load optimal params
	params, err := backtest.LoadOptimalParams(*currentFile)
	if err != nil {
		log.Fatalf("Failed to load optimal params: %v", err)
	}

	log.Printf("Loaded optimal params from: %s", *currentFile)
	log.Printf("Strategy: %s", params.Strategy.Type)
	log.Printf("Symbols: %v", params.Strategy.Symbols)
	log.Printf("Sharpe Ratio: %.4f", params.Performance.SharpeRatio)

	// Generate production files
	generator := backtest.NewProductionConfigGenerator(*outputDir)
	files, err := generator.GenerateAllProductionFiles(params, *strategyID)
	if err != nil {
		log.Fatalf("Failed to generate production files: %v", err)
	}

	log.Println("\nGenerated production files:")
	log.Printf("  Trader Config: %s", files["trader_config"])
	log.Printf("  Control File:  %s", files["control_file"])
	log.Printf("  Model File:    %s", files["model_file"])
	log.Printf("  Start Script:  %s", files["start_script"])

	log.Println("\n========================================")
	log.Println("Production files ready!")
	log.Printf("To deploy, run: bash %s", files["start_script"])
	log.Println("========================================")
}

func runCompare() {
	log.Println("========================================")
	log.Println("Parameter Comparison")
	log.Println("========================================")

	if *baselineFile == "" || *currentFile == "" {
		log.Fatal("Both baseline and current files required. Use -baseline and -current flags")
	}

	// Load both params
	baseline, err := backtest.LoadOptimalParams(*baselineFile)
	if err != nil {
		log.Fatalf("Failed to load baseline: %v", err)
	}

	current, err := backtest.LoadOptimalParams(*currentFile)
	if err != nil {
		log.Fatalf("Failed to load current: %v", err)
	}

	// Compare
	comparison := backtest.CompareParams(baseline, current)
	fmt.Println(comparison)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Backtest Parameter Optimization Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Actions:\n")
		fmt.Fprintf(os.Stderr, "  optimize  - Run parameter optimization (grid search)\n")
		fmt.Fprintf(os.Stderr, "  export    - Export optimal params to production config\n")
		fmt.Fprintf(os.Stderr, "  compare   - Compare two parameter sets\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Optimize parameters\n")
		fmt.Fprintf(os.Stderr, "  %s -action optimize \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -config config/backtest.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    -params entry_zscore:1.5:3.0:0.1,exit_zscore:0.5:1.5:0.1 \\\n")
		fmt.Fprintf(os.Stderr, "    -goal sharpe \\\n")
		fmt.Fprintf(os.Stderr, "    -workers 8\n\n")
		fmt.Fprintf(os.Stderr, "  # Export to production config\n")
		fmt.Fprintf(os.Stderr, "  %s -action export \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -current backtest_results/optimal_params/optimal_params_ag2502_ag2504_20260124.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    -strategy-id 92201\n\n")
		fmt.Fprintf(os.Stderr, "  # Compare parameters\n")
		fmt.Fprintf(os.Stderr, "  %s -action compare \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -baseline optimal_params/archive/old.yaml \\\n")
		fmt.Fprintf(os.Stderr, "    -current optimal_params/current/new.yaml\n\n")
	}
}
