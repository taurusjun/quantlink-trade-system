package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ModelFileParser model文件解析器
type ModelFileParser struct {
	FilePath string
}

// NewModelFileParser 创建model文件解析器
func NewModelFileParser(filePath string) *ModelFileParser {
	return &ModelFileParser{
		FilePath: filePath,
	}
}

// Parse 解析 model 文件
func (p *ModelFileParser) Parse() (map[string]interface{}, error) {
	file, err := os.Open(p.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open model file: %w", err)
	}
	defer file.Close()

	params := make(map[string]interface{})
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 跳过符号定义行（包含FUTCOM）
		if strings.Contains(line, "FUTCOM") || strings.Contains(line, "Dependant") {
			continue
		}

		// 解析参数行: KEY VALUE
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// 类型转换
		params[key] = parseValue(value)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan model file: %w", err)
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("no parameters found in model file")
	}

	return params, nil
}

// parseValue 解析值类型
func parseValue(s string) interface{} {
	// 尝试解析为整数
	if intVal, err := strconv.Atoi(s); err == nil {
		return intVal
	}

	// 尝试解析为浮点数
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal
	}

	// 默认字符串
	return s
}

// ValidateParameters 验证参数合法性
func ValidateParameters(params map[string]interface{}) error {
	// 必填参数检查
	required := []string{"BEGIN_PLACE", "BEGIN_REMOVE"}
	for _, key := range required {
		if _, exists := params[key]; !exists {
			return fmt.Errorf("missing required parameter: %s", key)
		}
	}

	// SIZE 参数检查
	if size, ok := params["SIZE"].(int); ok {
		if size <= 0 || size > 1000 {
			return fmt.Errorf("SIZE out of range [1, 1000]: %d", size)
		}
	}

	// MAX_SIZE 参数检查
	if maxSize, ok := params["MAX_SIZE"].(int); ok {
		if maxSize <= 0 || maxSize > 1000 {
			return fmt.Errorf("MAX_SIZE out of range [1, 1000]: %d", maxSize)
		}
	}

	// BEGIN_PLACE 参数检查
	if beginPlace, ok := params["BEGIN_PLACE"].(float64); ok {
		if beginPlace < 0 || beginPlace > 10 {
			return fmt.Errorf("BEGIN_PLACE out of range [0, 10]: %.2f", beginPlace)
		}
	}

	// BEGIN_REMOVE 参数检查
	if beginRemove, ok := params["BEGIN_REMOVE"].(float64); ok {
		if beginRemove < 0 || beginRemove > 10 {
			return fmt.Errorf("BEGIN_REMOVE out of range [0, 10]: %.2f", beginRemove)
		}
	}

	// STOP_LOSS 参数检查
	if stopLoss, ok := params["STOP_LOSS"].(float64); ok {
		if stopLoss < 0 {
			return fmt.Errorf("STOP_LOSS must be positive: %.2f", stopLoss)
		}
	}

	return nil
}

// ConvertModelToStrategyParams 转换model参数到策略参数
func ConvertModelToStrategyParams(modelParams map[string]interface{}) map[string]interface{} {
	params := make(map[string]interface{})

	// BEGIN_PLACE -> entry_zscore
	if val, ok := modelParams["BEGIN_PLACE"]; ok {
		params["entry_zscore"] = val
	}

	// BEGIN_REMOVE -> exit_zscore
	if val, ok := modelParams["BEGIN_REMOVE"]; ok {
		params["exit_zscore"] = val
	}

	// LONG_PLACE -> long_entry_zscore
	if val, ok := modelParams["LONG_PLACE"]; ok {
		params["long_entry_zscore"] = val
	}

	// SHORT_PLACE -> short_entry_zscore
	if val, ok := modelParams["SHORT_PLACE"]; ok {
		params["short_entry_zscore"] = val
	}

	// LONG_REMOVE -> long_exit_zscore
	if val, ok := modelParams["LONG_REMOVE"]; ok {
		params["long_exit_zscore"] = val
	}

	// SHORT_REMOVE -> short_exit_zscore
	if val, ok := modelParams["SHORT_REMOVE"]; ok {
		params["short_exit_zscore"] = val
	}

	// SIZE -> order_size
	if val, ok := modelParams["SIZE"]; ok {
		params["order_size"] = val
	}

	// MAX_SIZE -> max_position_size
	if val, ok := modelParams["MAX_SIZE"]; ok {
		params["max_position_size"] = val
	}

	// BID_SIZE
	if val, ok := modelParams["BID_SIZE"]; ok {
		params["bid_size"] = val
	}

	// ASK_SIZE
	if val, ok := modelParams["ASK_SIZE"]; ok {
		params["ask_size"] = val
	}

	// STOP_LOSS -> stop_loss
	if val, ok := modelParams["STOP_LOSS"]; ok {
		params["stop_loss"] = val
	}

	// MAX_LOSS -> max_loss
	if val, ok := modelParams["MAX_LOSS"]; ok {
		params["max_loss"] = val
	}

	// UPNL_LOSS -> upnl_loss
	if val, ok := modelParams["UPNL_LOSS"]; ok {
		params["upnl_loss"] = val
	}

	// ALPHA -> alpha
	if val, ok := modelParams["ALPHA"]; ok {
		params["alpha"] = val
	}

	// HEDGE_THRES -> hedge_threshold
	if val, ok := modelParams["HEDGE_THRES"]; ok {
		params["hedge_threshold"] = val
	}

	// HEDGE_SIZE_RATIO -> hedge_size_ratio
	if val, ok := modelParams["HEDGE_SIZE_RATIO"]; ok {
		params["hedge_size_ratio"] = val
	}

	return params
}

// GetFloatParam 获取浮点参数
func GetFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultVal
}

// GetIntParam 获取整数参数
func GetIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// GetFileInfo 获取文件信息
func GetFileInfo(filePath string) (os.FileInfo, error) {
	return os.Stat(filePath)
}
