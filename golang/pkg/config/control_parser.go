package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ControlFile control文件结构
type ControlFile struct {
	Symbol1       string
	Symbol2       string
	ModelFilePath string
	Exchange      string
	StrategyType  string
	StartTime     string
	EndTime       string
}

// ParseControlFile 解析control文件
func ParseControlFile(filePath string) (*ControlFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open control file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty control file")
	}

	line := strings.TrimSpace(scanner.Text())
	parts := strings.Fields(line)

	if len(parts) < 8 {
		return nil, fmt.Errorf("invalid control file format: expected 8 fields, got %d", len(parts))
	}

	control := &ControlFile{
		Symbol1:       convertInternalSymbol(parts[0]),
		ModelFilePath: parts[1],
		Exchange:      parts[2],
		StrategyType:  parts[4],
		StartTime:     formatTime(parts[5]),
		EndTime:       formatTime(parts[6]),
		Symbol2:       convertInternalSymbol(parts[7]),
	}

	return control, nil
}

// convertInternalSymbol 转换内部合约代码到标准代码
// ag_F_2_SFE -> ag2502
func convertInternalSymbol(internal string) string {
	// 示例: ag_F_2_SFE -> ag + 25 + 02 -> ag2502
	parts := strings.Split(internal, "_")
	if len(parts) < 3 {
		return internal
	}

	symbol := parts[0] // ag

	// 如果包含 _F_ 格式，尝试解析月份
	if len(parts) >= 3 && parts[1] == "F" {
		month := parts[2] // 2 (表示2月)

		// 根据月份推算合约代码
		// 简化实现：假设是近期合约
		monthInt, err := strconv.Atoi(month)
		if err != nil {
			return internal
		}

		// 推算年份（简化：使用25年）
		year := "25"
		monthStr := fmt.Sprintf("%02d", monthInt)

		return symbol + year + monthStr
	}

	return internal
}

// formatTime 格式化时间 0900 -> 09:00:00
func formatTime(t string) string {
	if len(t) != 4 {
		return t
	}
	return t[:2] + ":" + t[2:] + ":00"
}

// ConvertStrategyType 转换策略类型
func ConvertStrategyType(legacyType string) string {
	switch legacyType {
	case "TB_PAIR_STRAT":
		return "pairwise_arb"
	case "TB_HEDGE_STRAT":
		return "hedging"
	case "TB_PASSIVE_STRAT":
		return "passive"
	case "TB_AGGRESSIVE_STRAT":
		return "aggressive"
	default:
		return "passive" // 默认被动策略
	}
}
