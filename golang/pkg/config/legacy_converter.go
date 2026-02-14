package config

import (
	"fmt"
	"time"
)

// ConvertLegacyToTraderConfig 转换旧配置到新配置
func ConvertLegacyToTraderConfig(
	controlFile *ControlFile,
	strategyID string,
	mode string,
	logFile string,
) (*TraderConfig, error) {

	// 解析 model 文件
	modelParser := NewModelFileParser(controlFile.ModelFilePath)
	modelParams, err := modelParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse model file: %w", err)
	}

	// 验证 model 参数
	if err := ValidateParameters(modelParams); err != nil {
		return nil, fmt.Errorf("validate model parameters: %w", err)
	}

	// 转换为策略参数
	strategyParams := ConvertModelToStrategyParams(modelParams)

	// 注入 strategy_id 到参数中（供 PairwiseArbStrategy.Initialize 读取）
	// C++: m_strategyID 用于 daily_init 文件名（如 daily_init.92201）
	sid, err := fmt.Sscanf(strategyID, "%d", new(int))
	if err == nil && sid > 0 {
		var numID int
		fmt.Sscanf(strategyID, "%d", &numID)
		strategyParams["strategy_id"] = float64(numID)
	}

	// 创建新配置
	cfg := &TraderConfig{
		System: SystemConfig{
			StrategyID: strategyID,
			Mode:       mode,
		},
		Strategy: StrategyConfig{
			Type:            ConvertStrategyType(controlFile.StrategyType),
			Symbols:         []string{controlFile.Symbol1, controlFile.Symbol2},
			Exchanges:       []string{controlFile.Exchange},
			Parameters:      strategyParams,
			MaxPositionSize: int64(GetIntParam(modelParams, "MAX_SIZE", 50)),
			MaxExposure:     GetFloatParam(modelParams, "MAX_EXPOSURE", 1000000),

			// Model 热加载配置
			ModelFile: controlFile.ModelFilePath,
			HotReload: HotReloadConfig{
				Enabled: true, // 自动启用热加载
			},
		},
		Session: SessionConfig{
			StartTime:    controlFile.StartTime,
			EndTime:      controlFile.EndTime,
			Timezone:     "Asia/Shanghai",
			AutoStart:    false,
			AutoStop:     true,
			AutoActivate: false, // 推荐手动激活
		},
		Risk: RiskConfig{
			StopLoss:        GetFloatParam(modelParams, "STOP_LOSS", 100000),
			MaxLoss:         GetFloatParam(modelParams, "MAX_LOSS", 100000),
			DailyLossLimit:  0,
			MaxRejectCount:  10,
			CheckIntervalMs: 5000, // 5秒检查一次风控
		},
		Engine: EngineConfig{
			ORSGatewayAddr:      "localhost:50051",
			NATSAddr:            "nats://localhost:4222",
			CounterBridgeAddr:   "localhost:8080",
			OrderQueueSize:      100,
			TimerInterval:       5 * time.Second,
			MaxConcurrentOrders: 10,
		},
		Portfolio: PortfolioConfig{
			TotalCapital:         1000000,
			StrategyAllocation:   map[string]float64{},
			RebalanceIntervalSec: 0,
			MinAllocation:        0,
			MaxAllocation:        1,
			EnableAutoRebalance:  false,
			EnableCorrelation:    true,
		},
		API: APIConfig{
			Enabled: true,
			Port:    9200 + mustAtoi(strategyID[len(strategyID)-2:]), // 92201 -> 9201
			Host:    "localhost",
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       logFile,
			MaxSizeMB:  100,
			MaxBackups: 10,
			MaxAgeDays: 30,
			Compress:   false,
			Console:    true,
			JSONFormat: false,
		},
	}

	return cfg, nil
}

// mustAtoi 字符串转整数，失败返回0
func mustAtoi(s string) int {
	i, err := fmt.Sscanf(s, "%d", new(int))
	if err != nil || i == 0 {
		return 0
	}
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// GenerateLegacyLogFileName 生成旧系统格式的日志文件名
// 格式: ./log/log.control.{symbol1}.{symbol2}.par.txt.{strategyID}.{date}
func GenerateLegacyLogFileName(controlFileName string, strategyID string, date string) string {
	return fmt.Sprintf("./log/log.%s.%s", controlFileName, date)
}
