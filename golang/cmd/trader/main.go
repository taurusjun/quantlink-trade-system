package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	"github.com/yourusername/quantlink-trade-system/pkg/trader"
)

const (
	appName    = "QuantlinkTrader"
	appVersion = "1.0.0"
)

var (
	// Command line flags
	configFile  = flag.String("config", "./config/trader.yaml", "Configuration file path")
	strategyID  = flag.String("strategy-id", "", "Strategy ID (overrides config)")
	strategyType = flag.String("strategy-type", "", "Strategy type: passive, aggressive, hedging, pairwise_arb (overrides config)")
	mode        = flag.String("mode", "", "Run mode: live, backtest, simulation (overrides config)")
	logFile     = flag.String("log-file", "", "Log file path (overrides config)")
	logLevel    = flag.String("log-level", "", "Log level: debug, info, warn, error (overrides config)")
	watchConfig = flag.Bool("watch-config", false, "Watch config file for changes and hot reload")
	version     = flag.Bool("version", false, "Print version and exit")
	help        = flag.Bool("help", false, "Print help and exit")
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
	cfg, err := config.LoadTraderConfig(*configFile)
	if err != nil {
		log.Fatalf("[Main] Failed to load config: %v", err)
	}
	log.Println("[Main] ✓ Configuration loaded successfully")

	// Override config with command line flags
	if *strategyID != "" {
		cfg.System.StrategyID = *strategyID
		log.Printf("[Main] Strategy ID overridden: %s", *strategyID)
	}
	if *strategyType != "" {
		cfg.Strategy.Type = *strategyType
		log.Printf("[Main] Strategy type overridden: %s", *strategyType)
	}
	if *mode != "" {
		cfg.System.Mode = *mode
		log.Printf("[Main] Mode overridden: %s", *mode)
	}
	if *logFile != "" {
		cfg.Logging.File = *logFile
		log.Printf("[Main] Log file overridden: %s", *logFile)
	}
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
		log.Printf("[Main] Log level overridden: %s", *logLevel)
	}

	// Setup logging
	if cfg.Logging.File != "" {
		setupFileLogging(cfg.Logging.File)
	}

	// Print configuration summary
	printConfigSummary(cfg)

	// Create trader
	log.Println("[Main] Creating trader instance...")
	t, err := trader.NewTrader(cfg)
	if err != nil {
		log.Fatalf("[Main] Failed to create trader: %v", err)
	}
	log.Println("[Main] ✓ Trader instance created")

	// Initialize trader
	log.Println("[Main] Initializing trader...")
	if err := t.Initialize(); err != nil {
		log.Fatalf("[Main] Failed to initialize trader: %v", err)
	}
	log.Println("[Main] ✓ Trader initialized successfully")

	// Start config watcher (if enabled)
	if *watchConfig {
		go watchConfigFile(*configFile, t)
	}

	// Start trader
	log.Println("[Main] Starting trader...")
	if err := t.Start(); err != nil {
		log.Fatalf("[Main] Failed to start trader: %v", err)
	}

	// Print status periodically
	go printStatusPeriodically(t, 30*time.Second)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("[Main] ════════════════════════════════════════════════════════════")
	log.Println("[Main] Trader is running. Press Ctrl+C to stop...")
	log.Println("[Main] ════════════════════════════════════════════════════════════")

	// Wait for signal
	sig := <-sigChan
	log.Printf("[Main] Received signal: %v", sig)

	// Shutdown
	log.Println("[Main] Shutting down trader...")
	if err := t.Stop(); err != nil {
		log.Printf("[Main] Error during shutdown: %v", err)
		os.Exit(1)
	}

	// Print final status
	log.Println("[Main] ════════════════════════════════════════════════════════════")
	log.Println("[Main] Final Status")
	log.Println("[Main] ════════════════════════════════════════════════════════════")
	printStatus(t)

	log.Println("[Main] ✓ Trader stopped successfully")
	log.Println("[Main] Goodbye!")
}

func printBanner() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  %s v%-44s║\n", appName, appVersion)
	fmt.Println("║  Production Trading System                                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func printHelp() {
	fmt.Printf("Usage: %s [OPTIONS]\n\n", appName)
	fmt.Println("A production-ready trading system for quantitative strategies.")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Printf("  # Run with default config\n")
	fmt.Printf("  %s --config ./config/trader.yaml\n\n", appName)
	fmt.Printf("  # Run with custom strategy ID and mode\n")
	fmt.Printf("  %s --config ./config/trader.yaml --strategy-id 92201 --mode live\n\n", appName)
	fmt.Printf("  # Run with config hot reload\n")
	fmt.Printf("  %s --config ./config/trader.yaml --watch-config\n\n", appName)
}

func printConfigSummary(cfg *config.TraderConfig) {
	log.Println("[Main] ────────────────────────────────────────────────────────────")
	log.Println("[Main] Configuration Summary")
	log.Println("[Main] ────────────────────────────────────────────────────────────")
	log.Printf("[Main] Strategy ID:       %s", cfg.System.StrategyID)
	log.Printf("[Main] Strategy Type:     %s", cfg.Strategy.Type)
	log.Printf("[Main] Run Mode:          %s", cfg.System.Mode)
	log.Printf("[Main] Symbols:           %v", cfg.Strategy.Symbols)
	log.Printf("[Main] Exchanges:         %v", cfg.Strategy.Exchanges)
	log.Printf("[Main] Max Position:      %d", cfg.Strategy.MaxPositionSize)
	log.Printf("[Main] Max Exposure:      %.2f", cfg.Strategy.MaxExposure)
	if cfg.Session.StartTime != "" && cfg.Session.EndTime != "" {
		log.Printf("[Main] Trading Hours:     %s - %s (%s)",
			cfg.Session.StartTime, cfg.Session.EndTime, cfg.Session.Timezone)
		log.Printf("[Main] Auto Start/Stop:   %v / %v",
			cfg.Session.AutoStart, cfg.Session.AutoStop)
	}
	log.Println("[Main] ────────────────────────────────────────────────────────────")
}

func setupFileLogging(logFilePath string) {
	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[Main] Warning: Failed to create log directory: %v", err)
		return
	}

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[Main] Warning: Failed to open log file: %v", err)
		return
	}

	// Set log output to file (note: this is simple logging, for production use structured logging)
	log.SetOutput(logFile)
	log.Printf("[Main] ✓ Logging to file: %s", logFilePath)
}

func printStatus(t *trader.Trader) {
	status := t.GetStatus()

	log.Printf("[Main] Running:        %v", status["running"])
	log.Printf("[Main] Strategy ID:    %s", status["strategy_id"])
	log.Printf("[Main] Mode:           %s", status["mode"])

	if strategyStatus, ok := status["strategy"].(map[string]interface{}); ok {
		log.Printf("[Main] Strategy:       %v", strategyStatus["strategy_id"])
		log.Printf("[Main]   Running:      %v", strategyStatus["is_running"])
	}

	// Position information
	if position, ok := status["position"].(*strategy.Position); ok {
		log.Printf("[Main] Position:       %d (Long: %d, Short: %d)",
			position.NetQty, position.LongQty, position.ShortQty)
	}

	// P&L information
	if pnl, ok := status["pnl"].(*strategy.PNL); ok {
		log.Printf("[Main] P&L:            %.2f (Realized: %.2f, Unrealized: %.2f)",
			pnl.TotalPnL, pnl.RealizedPnL, pnl.UnrealizedPnL)
	}
}

func printStatusPeriodically(t *trader.Trader, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if !t.IsRunning() {
			return
		}

		log.Println("[Main] ════════════════════════════════════════════════════════════")
		log.Printf("[Main] Periodic Status Update - %s", time.Now().Format("15:04:05"))
		log.Println("[Main] ────────────────────────────────────────────────────────────")
		printStatus(t)
		log.Println("[Main] ════════════════════════════════════════════════════════════")
	}
}

func watchConfigFile(configPath string, t *trader.Trader) {
	log.Printf("[Main] Watching config file for changes: %s", configPath)

	// Get initial file info
	initialStat, err := os.Stat(configPath)
	if err != nil {
		log.Printf("[Main] Error watching config: %v", err)
		return
	}

	lastModTime := initialStat.ModTime()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !t.IsRunning() {
			return
		}

		stat, err := os.Stat(configPath)
		if err != nil {
			continue
		}

		if stat.ModTime().After(lastModTime) {
			log.Println("[Main] Config file changed, reloading...")
			lastModTime = stat.ModTime()

			// Load new config
			newCfg, err := config.LoadTraderConfig(configPath)
			if err != nil {
				log.Printf("[Main] Error loading new config: %v", err)
				continue
			}

			// TODO: Implement hot reload logic
			// For now, just log the change
			log.Printf("[Main] New config loaded (hot reload not yet implemented)")
			log.Printf("[Main] Strategy: %s, Mode: %s", newCfg.Strategy.Type, newCfg.System.Mode)
		}
	}
}
