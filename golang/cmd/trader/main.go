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
	// New system flags
	configFile   = flag.String("config", "./config/trader.yaml", "Configuration file path")
	strategyID   = flag.String("strategy-id", "", "Strategy ID (overrides config)")
	strategyType = flag.String("strategy-type", "", "Strategy type: passive, aggressive, hedging, pairwise_arb (overrides config)")
	mode         = flag.String("mode", "", "Run mode: live, backtest, simulation (overrides config)")
	logFile      = flag.String("log-file", "", "Log file path (overrides config)")
	logLevel     = flag.String("log-level", "", "Log level: debug, info, warn, error (overrides config)")
	watchConfig  = flag.Bool("watch-config", false, "Watch config file for changes and hot reload")
	version      = flag.Bool("version", false, "Print version and exit")
	help         = flag.Bool("help", false, "Print help and exit")

	// Legacy system compatibility flags (旧系统兼容参数)
	legacyLive       = flag.Bool("Live", false, "Legacy: Live trading mode")
	legacyBacktest   = flag.Bool("Backtest", false, "Legacy: Backtest mode")
	legacySimulation = flag.Bool("Simulation", false, "Legacy: Simulation mode")
	controlFile      = flag.String("controlFile", "", "Legacy: Control file path (symbol + model)")
	legacyConfigFile = flag.String("configFile", "", "Legacy: Legacy config file path (deprecated)")
	strategyIDLegacy = flag.String("strategyID", "", "Legacy: Strategy ID (same as --strategy-id)")
	adjustLTP        = flag.Int("adjustLTP", 0, "Legacy: Adjust last trade price (deprecated)")
	printMod         = flag.Int("printMod", 0, "Legacy: Print mode (deprecated)")
	updateInterval   = flag.Int("updateInterval", 300000, "Legacy: Update interval in microseconds (deprecated)")
	logFileLegacy    = flag.String("logFile", "", "Legacy: Log file path (same as --log-file)")
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

	// Detect legacy mode (旧系统兼容模式检测)
	isLegacyMode := *controlFile != ""

	var cfg *config.TraderConfig
	var err error

	if isLegacyMode {
		// Legacy mode: 使用 controlFile + model 文件
		log.Println("[Main] ════════════════════════════════════════════════════════════")
		log.Println("[Main] Running in LEGACY COMPATIBILITY MODE")
		log.Println("[Main] Converting old system config to new format...")
		log.Println("[Main] ════════════════════════════════════════════════════════════")
		cfg, err = loadLegacyConfig()
		if err != nil {
			log.Fatalf("[Main] Failed to load legacy config: %v", err)
		}
		log.Println("[Main] ✓ Legacy configuration converted successfully")
	} else {
		// New system mode: 使用 YAML 配置文件
		log.Printf("[Main] Loading configuration from: %s", *configFile)
		cfg, err = config.LoadTraderConfig(*configFile)
		if err != nil {
			log.Fatalf("[Main] Failed to load config: %v", err)
		}
		log.Println("[Main] ✓ Configuration loaded successfully")
	}

	// Override config with command line flags (新旧系统通用)
	applyCommandLineOverrides(cfg)

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

	// 多策略模式
	if cfg.System.MultiStrategy {
		log.Printf("[Main] Mode:              Multi-Strategy")
		log.Printf("[Main] Run Mode:          %s", cfg.System.Mode)
		log.Printf("[Main] Strategies:        %d", len(cfg.Strategies))
		for i, s := range cfg.Strategies {
			log.Printf("[Main]   [%d] %s (%s) - %v", i+1, s.ID, s.Type, s.Symbols)
		}
	} else {
		// 单策略模式
		log.Printf("[Main] Strategy ID:       %s", cfg.System.StrategyID)
		log.Printf("[Main] Strategy Type:     %s", cfg.Strategy.Type)
		log.Printf("[Main] Run Mode:          %s", cfg.System.Mode)
		log.Printf("[Main] Symbols:           %v", cfg.Strategy.Symbols)
		log.Printf("[Main] Exchanges:         %v", cfg.Strategy.Exchanges)
		log.Printf("[Main] Max Position:      %d", cfg.Strategy.MaxPositionSize)
		log.Printf("[Main] Max Exposure:      %.2f", cfg.Strategy.MaxExposure)
	}

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

	// Position information (estimated)
	if position, ok := status["position"].(*strategy.EstimatedPosition); ok {
		log.Printf("[Main] Estimated Position: %d (Long: %d, Short: %d)",
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

// loadLegacyConfig 加载旧系统配置（control + model 文件）
func loadLegacyConfig() (*config.TraderConfig, error) {
	// 解析 control 文件
	log.Printf("[Main] Parsing control file: %s", *controlFile)
	control, err := config.ParseControlFile(*controlFile)
	if err != nil {
		return nil, fmt.Errorf("parse control file: %w", err)
	}
	log.Printf("[Main] ✓ Control file parsed: %s/%s, model=%s",
		control.Symbol1, control.Symbol2, control.ModelFilePath)

	// 确定运行模式
	runMode := "live"
	if *legacyBacktest {
		runMode = "backtest"
	} else if *legacySimulation {
		runMode = "simulation"
	} else if *legacyLive {
		runMode = "live"
	}

	// 确定策略ID（优先级：--strategyID > --strategy-id）
	sid := *strategyID
	if *strategyIDLegacy != "" {
		sid = *strategyIDLegacy
	}
	if sid == "" {
		return nil, fmt.Errorf("strategy ID is required (use --strategyID or --strategy-id)")
	}

	// 确定日志文件（优先级：--logFile > --log-file > 自动生成）
	logPath := *logFile
	if *logFileLegacy != "" {
		logPath = *logFileLegacy
	}
	if logPath == "" {
		// 生成旧系统格式的日志文件名
		date := time.Now().Format("20060102")
		controlFileName := filepath.Base(*controlFile)
		logPath = config.GenerateLegacyLogFileName(controlFileName, sid, date)
	}

	log.Printf("[Main] Strategy ID: %s, Mode: %s, Log: %s", sid, runMode, logPath)

	// 转换为新系统配置
	cfg, err := config.ConvertLegacyToTraderConfig(control, sid, runMode, logPath)
	if err != nil {
		return nil, fmt.Errorf("convert legacy config: %w", err)
	}

	log.Println("[Main] ────────────────────────────────────────────────────────────")
	log.Println("[Main] Legacy Config Summary:")
	log.Printf("[Main]   Control File:  %s", *controlFile)
	log.Printf("[Main]   Model File:    %s", control.ModelFilePath)
	log.Printf("[Main]   Symbols:       %v", cfg.Strategy.Symbols)
	log.Printf("[Main]   Strategy Type: %s", cfg.Strategy.Type)
	log.Printf("[Main]   Session:       %s - %s", cfg.Session.StartTime, cfg.Session.EndTime)
	log.Println("[Main] ────────────────────────────────────────────────────────────")

	return cfg, nil
}

// applyCommandLineOverrides 应用命令行参数覆盖
func applyCommandLineOverrides(cfg *config.TraderConfig) {
	// 新系统参数
	if *strategyID != "" {
		cfg.System.StrategyID = *strategyID
		log.Printf("[Main] Strategy ID overridden: %s", *strategyID)
	}
	if *strategyIDLegacy != "" {
		cfg.System.StrategyID = *strategyIDLegacy
		log.Printf("[Main] Strategy ID overridden (legacy): %s", *strategyIDLegacy)
	}

	if *strategyType != "" && !cfg.System.MultiStrategy {
		cfg.Strategy.Type = *strategyType
		log.Printf("[Main] Strategy type overridden: %s", *strategyType)
	}

	if *mode != "" {
		cfg.System.Mode = *mode
		log.Printf("[Main] Mode overridden: %s", *mode)
	}

	// Mode overrides from legacy flags
	if *legacyLive {
		cfg.System.Mode = "live"
		log.Println("[Main] Mode set to 'live' (--Live flag)")
	}
	if *legacyBacktest {
		cfg.System.Mode = "backtest"
		log.Println("[Main] Mode set to 'backtest' (--Backtest flag)")
	}
	if *legacySimulation {
		cfg.System.Mode = "simulation"
		log.Println("[Main] Mode set to 'simulation' (--Simulation flag)")
	}

	if *logFile != "" {
		cfg.Logging.File = *logFile
		log.Printf("[Main] Log file overridden: %s", *logFile)
	}
	if *logFileLegacy != "" {
		cfg.Logging.File = *logFileLegacy
		log.Printf("[Main] Log file overridden (legacy): %s", *logFileLegacy)
	}

	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
		log.Printf("[Main] Log level overridden: %s", *logLevel)
	}

	// Legacy parameters (deprecated, just log warnings)
	if *adjustLTP != 0 {
		log.Printf("[Main] Warning: --adjustLTP is deprecated (value: %d)", *adjustLTP)
	}
	if *printMod != 0 {
		log.Printf("[Main] Warning: --printMod is deprecated (value: %d)", *printMod)
	}
	if *updateInterval != 300000 {
		log.Printf("[Main] Warning: --updateInterval is deprecated (value: %d μs)", *updateInterval)
		// 可以转换为 TimerInterval
		cfg.Engine.TimerInterval = time.Duration(*updateInterval) * time.Microsecond
	}
	if *legacyConfigFile != "" {
		log.Printf("[Main] Warning: --configFile (legacy) is deprecated and ignored")
	}
}
