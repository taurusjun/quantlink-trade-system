package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yourusername/quantlink-trade-system/pkg/backtest"
)

const (
	appName    = "QuantlinkBacktest"
	appVersion = "1.0.0"
)

var (
	// Command line flags
	configFile = flag.String("config", "./config/backtest.yaml", "Configuration file path")
	startDate  = flag.String("start-date", "", "Start date (YYYY-MM-DD, overrides config)")
	endDate    = flag.String("end-date", "", "End date (YYYY-MM-DD, overrides config)")
	dates      = flag.String("dates", "", "Comma-separated dates for batch backtest (YYYY-MM-DD,YYYY-MM-DD,...)")
	outputDir  = flag.String("output", "", "Output directory (overrides config)")
	version    = flag.Bool("version", false, "Print version and exit")
	help       = flag.Bool("help", false, "Print help and exit")
)

func main() {
	// Parse flags
	flag.Parse()

	// Print version
	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Print help
	if *help {
		printHelp()
		os.Exit(0)
	}

	// Print banner
	printBanner()

	// Load configuration
	log.Printf("[Main] Loading configuration from: %s", *configFile)
	config, err := backtest.LoadBacktestConfig(*configFile)
	if err != nil {
		log.Fatalf("[Main] Failed to load config: %v", err)
	}
	log.Println("[Main] ✓ Configuration loaded successfully")

	// Override with command line flags
	if *startDate != "" {
		config.Backtest.StartDate = *startDate
		log.Printf("[Main] Start date overridden: %s", *startDate)
	}
	if *endDate != "" {
		config.Backtest.EndDate = *endDate
		log.Printf("[Main] End date overridden: %s", *endDate)
	}
	if *outputDir != "" {
		config.Backtest.Output.ResultDir = *outputDir
		log.Printf("[Main] Output directory overridden: %s", *outputDir)
	}

	// Print configuration summary
	printConfigSummary(config)

	// Check if batch mode
	if *dates != "" {
		// Batch backtest
		dateList := strings.Split(*dates, ",")
		log.Printf("[Main] Running batch backtest for %d dates", len(dateList))

		results, err := backtest.RunBatch(config, dateList)
		if err != nil {
			log.Fatalf("[Main] Batch backtest failed: %v", err)
		}

		log.Printf("[Main] Batch backtest completed: %d results", len(results))

		// Save batch results
		if config.Backtest.Output.GenerateReport {
			log.Println("[Main] Saving batch results...")
			// TODO: Implement batch report generation
		}
	} else {
		// Single backtest
		log.Println("[Main] Running single backtest...")

		runner, err := backtest.NewBacktestRunner(config)
		if err != nil {
			log.Fatalf("[Main] Failed to create runner: %v", err)
		}

		result, err := runner.Run()
		if err != nil {
			log.Fatalf("[Main] Backtest failed: %v", err)
		}

		// Save results
		if config.Backtest.Output.GenerateReport {
			log.Println("[Main] Generating report...")
			reportGen := backtest.NewReportGenerator(config, result)

			if err := reportGen.GenerateMarkdown(); err != nil {
				log.Printf("[Main] Failed to generate markdown report: %v", err)
			}

			if err := reportGen.GenerateJSON(); err != nil {
				log.Printf("[Main] Failed to generate JSON report: %v", err)
			}

			log.Printf("[Main] Report saved to: %s", config.Backtest.Output.ResultDir)
		}

		log.Println("[Main] Backtest completed successfully!")
	}
}

func printBanner() {
	fmt.Println("========================================")
	fmt.Printf("%s v%s\n", appName, appVersion)
	fmt.Println("高性能量化回测系统")
	fmt.Println("========================================")
}

func printHelp() {
	fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  # Single day backtest")
	fmt.Println("  ./backtest -config config/backtest.yaml")
	fmt.Println()
	fmt.Println("  # Override dates")
	fmt.Println("  ./backtest -config config/backtest.yaml -start-date 2026-01-01 -end-date 2026-01-31")
	fmt.Println()
	fmt.Println("  # Batch backtest")
	fmt.Println("  ./backtest -config config/backtest.yaml -dates 2026-01-01,2026-01-02,2026-01-03")
	fmt.Println()
}

func printConfigSummary(config *backtest.BacktestConfig) {
	fmt.Println("\n========================================")
	fmt.Println("Configuration Summary")
	fmt.Println("========================================")
	fmt.Printf("Backtest Name:     %s\n", config.Backtest.Name)
	fmt.Printf("Date Range:        %s to %s\n", config.Backtest.StartDate, config.Backtest.EndDate)
	fmt.Printf("Time Range:        %s to %s\n", config.Backtest.StartTime, config.Backtest.EndTime)
	fmt.Printf("Symbols:           %v\n", config.Backtest.Data.Symbols)
	fmt.Printf("Data Path:         %s\n", config.Backtest.Data.DataPath)
	fmt.Printf("Replay Mode:       %s\n", config.Backtest.Replay.Mode)
	if config.Backtest.Replay.Mode == "fast" {
		fmt.Printf("Replay Speed:      %.1fx\n", config.Backtest.Replay.Speed)
	}
	fmt.Printf("Initial Capital:   %.2f\n", config.Backtest.Initial.Capital)
	fmt.Printf("Strategy:          %s\n", config.Strategy.Type)
	fmt.Printf("Output Directory:  %s\n", config.Backtest.Output.ResultDir)
	fmt.Println("========================================\n")
}
