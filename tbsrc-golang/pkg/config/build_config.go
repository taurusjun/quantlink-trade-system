package config

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

// BuildParams 组合构建 Config 所需的参数
type BuildParams struct {
	ControlFile string
	ConfigFile  string
	StrategyID  int
	APIPort     int
	YearPrefix  string // 年份后两位 (e.g. "26")，用于 baseName → symbol 映射
}

// BuildFromCppFiles 从 C++ 格式文件组合构建 Config
// 流程:
//  1. ParseControlFile → ControlConfig (symbols, modelFile, exchange, strategyType)
//  2. ParseCfgFile → CfgConfig (SHM keys, PRODUCT)
//  3. ParseModelFile → ModelConfig (thresholds)
//  4. 组合 → Config
//
// 参考: tbsrc/main/main.cpp:372-998 (startup sequence)
func BuildFromCppFiles(params BuildParams) (*Config, *ControlConfig, error) {
	// 1. 解析 controlFile
	controlCfg, err := ParseControlFile(params.ControlFile)
	if err != nil {
		return nil, nil, fmt.Errorf("buildConfig: %w", err)
	}
	log.Printf("[config] controlFile: baseName=%s model=%s exchange=%s strat=%s second=%s",
		controlCfg.BaseName, controlCfg.ModelFile, controlCfg.Exchange, controlCfg.ExecStrat, controlCfg.SecondName)

	// 2. 解析 configFile
	cfgConfig, err := ParseCfgFile(params.ConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("buildConfig: %w", err)
	}
	log.Printf("[config] configFile: product=%s exchanges=%s", cfgConfig.Product, cfgConfig.Exchanges)

	// 3. 解析 modelFile (路径相对于 controlFile 所在目录)
	modelPath := controlCfg.ModelFile
	if !filepath.IsAbs(modelPath) {
		// C++: modelFile 路径相对于 binary 工作目录，不是 controlFile 所在目录
		// 保持原样让 os.Open 处理
	}
	modelCfg, err := ParseModelFile(modelPath)
	if err != nil {
		return nil, nil, fmt.Errorf("buildConfig: %w", err)
	}
	log.Printf("[config] modelFile: %d thresholds, %d indicators", len(modelCfg.Thresholds), len(modelCfg.Indicators))

	// 4. 确定年份前缀
	yearPrefix := params.YearPrefix
	if yearPrefix == "" {
		yearPrefix = fmt.Sprintf("%02d", time.Now().Year()%100)
	}

	// 5. baseName → symbol 映射
	sym1, err := BaseNameToSymbol(controlCfg.BaseName, yearPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("buildConfig: leg1 %w", err)
	}
	var sym2 string
	if controlCfg.SecondName != "" {
		sym2, err = BaseNameToSymbol(controlCfg.SecondName, yearPrefix)
		if err != nil {
			return nil, nil, fmt.Errorf("buildConfig: leg2 %w", err)
		}
	}
	log.Printf("[config] symbols: %s → %s, %s → %s", controlCfg.BaseName, sym1, controlCfg.SecondName, sym2)

	// 6. 获取交易所 SHM 配置
	exchangeSection := cfgConfig.Exchanges
	// C++: exchange from controlFile 是 "SFE"，configFile section 是 "CHINA_SHFE"
	// 映射: SFE → CHINA_SHFE (使用 configFile 中的 EXCHANGES 字段)
	mdKey, reqKey, respKey, clientStoreKey, mdSize, reqSize, respSize, err := cfgConfig.GetExchangeConfig(exchangeSection)
	if err != nil {
		return nil, nil, fmt.Errorf("buildConfig: %w", err)
	}

	// 7. 构建 Config
	cfg := &Config{
		ORS: ORSConfig{
			MDShmKey:          mdKey,
			MDQueueSize:       mdSize,
			ReqShmKey:         reqKey,
			ReqQueueSize:      reqSize,
			RespShmKey:        respKey,
			RespQueueSize:     respSize,
			ClientStoreShmKey: clientStoreKey,
		},
		Strategy: StrategyConfig{
			StrategyID: params.StrategyID,
			Product:    strings.ToUpper(cfgConfig.Product),
			Symbols:    []string{sym1},
		},
		System: SystemConfig{
			APIPort: params.APIPort,
		},
	}
	if sym2 != "" {
		cfg.Strategy.Symbols = append(cfg.Strategy.Symbols, sym2)
	}

	// 8. 从 model file 构建阈值 (第一组)
	tholdMap := make(map[string]float64)
	for k, v := range modelCfg.Thresholds {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		tholdMap[upperCaseToSnakeCase(k)] = f
	}
	cfg.Strategy.Thresholds = map[string]map[string]float64{
		"first":  tholdMap,
		"second": tholdMap, // C++: 默认 first == second
	}

	// 9. 从 controlFile exchange 推导 instrument config
	exchangeName := controlFileExchangeToName(controlCfg.Exchange)
	instrumentCfg := buildDefaultInstrumentConfig(exchangeName, sym1)
	cfg.Strategy.Instruments = map[string]InstrumentConfig{
		sym1: instrumentCfg,
	}
	if sym2 != "" {
		cfg.Strategy.Instruments[sym2] = buildDefaultInstrumentConfig(exchangeName, sym2)
	}

	// 10. 从 StrategyConfig.cfg 读取 ACCOUNT (如果存在)
	// C++: ExecutionStrategy.cpp:64 reads ./config/StrategyConfig.cfg
	stratCfg, err := ParseCfgFile("./config/StrategyConfig.cfg")
	if err == nil && stratCfg.GlobalKeys["ACCOUNT"] != "" {
		cfg.Strategy.Account = stratCfg.GlobalKeys["ACCOUNT"]
	}

	return cfg, controlCfg, nil
}

// controlFileExchangeToName 将 controlFile 中的交易所代码转为标准名
// C++: SFE → SHFE, ZCE → ZCE, DCE → DCE, CFFEX → CFFEX
func controlFileExchangeToName(exchange string) string {
	switch strings.ToUpper(exchange) {
	case "SFE":
		return "SHFE"
	case "ZCE", "CZCE":
		return "ZCE"
	case "DCE":
		return "DCE"
	case "CFFEX":
		return "CFFEX"
	case "GFEX":
		return "GFEX"
	default:
		return exchange
	}
}

// buildDefaultInstrumentConfig 根据交易所和合约名构建默认 InstrumentConfig
// 参数来自 C++ Instrument 初始化和 SimpleChinaInstruments
func buildDefaultInstrumentConfig(exchange string, symbol string) InstrumentConfig {
	product := extractProduct(symbol)

	// 默认值基于常见中国期货品种
	tickSize := 1.0
	lotSize := 1.0
	contractFactor := 1.0

	switch product {
	case "ag":
		tickSize = 1.0
		lotSize = 15
		contractFactor = 15.0
	case "au":
		tickSize = 0.02
		lotSize = 1000
		contractFactor = 1000.0
	case "al":
		tickSize = 5.0
		lotSize = 5
		contractFactor = 5.0
	case "cu":
		tickSize = 10.0
		lotSize = 5
		contractFactor = 5.0
	case "zn":
		tickSize = 5.0
		lotSize = 5
		contractFactor = 5.0
	case "rb":
		tickSize = 1.0
		lotSize = 10
		contractFactor = 10.0
	case "bu":
		tickSize = 2.0
		lotSize = 10
		contractFactor = 10.0
	case "sc":
		tickSize = 0.1
		lotSize = 1000
		contractFactor = 1000.0
	case "ss":
		tickSize = 5.0
		lotSize = 5
		contractFactor = 5.0
	}

	return InstrumentConfig{
		Exchange:        exchange,
		TickSize:        tickSize,
		LotSize:         lotSize,
		ContractFactor:  contractFactor,
		PriceMultiplier: lotSize,
		PriceFactor:     1.0,
		SendInLots:      true,
	}
}

// extractProduct 从合约名提取品种代码
// e.g. ag2603 → ag, au2604 → au, rb2505 → rb
func extractProduct(symbol string) string {
	for i, c := range symbol {
		if c >= '0' && c <= '9' {
			return symbol[:i]
		}
	}
	return symbol
}
