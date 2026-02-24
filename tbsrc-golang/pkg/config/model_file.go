package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"tbsrc-golang/pkg/types"
)

// IndicatorDef 对应 C++ model file 中的 indicator 行
// C++: LoadModelFile() in TradeBotUtils.cpp:1983-2276
// 格式: baseName type indName [args...]
type IndicatorDef struct {
	BaseName string // e.g. ag_F_3_SFE
	Type     string // e.g. FUTCOM
	IndName  string // e.g. Dependant
	Args     []string
}

// ModelConfig 对应 C++ model file 的解析结果
type ModelConfig struct {
	Thresholds map[string]string // C++ UPPER_CASE key -> string value
	Indicators []IndicatorDef
}

// ParseModelFile 解析 C++ model file (.par.txt 格式)
// C++: LoadModelFile() in TradeBotUtils.cpp:1983-2276
// 规则:
//   - # 开头为注释（特殊 #DEP_STD_DEV, #LOOKAHEAD, #TRGT_STD_DEV 除外）
//   - 3+ tokens: indicator 行
//   - 2 tokens: 阈值 key-value
//   - 1 token: 跳过
func ParseModelFile(path string) (*ModelConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("modelFile: open %s: %w", path, err)
	}
	defer f.Close()

	mc := &ModelConfig{
		Thresholds: make(map[string]string),
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// # 开头为注释，但 #DEP_STD_DEV, #LOOKAHEAD, #TRGT_STD_DEV 是特殊参数
		if strings.HasPrefix(line, "#") {
			// C++: 特殊前缀参数
			rest := strings.TrimPrefix(line, "#")
			tokens := strings.Fields(rest)
			if len(tokens) == 2 {
				key := tokens[0]
				switch key {
				case "DEP_STD_DEV", "LOOKAHEAD", "TRGT_STD_DEV":
					mc.Thresholds[key] = tokens[1]
				}
			}
			continue
		}

		tokens := strings.Fields(line)
		switch {
		case len(tokens) >= 3:
			// Indicator line: baseName type indName [args...]
			ind := IndicatorDef{
				BaseName: tokens[0],
				Type:     tokens[1],
				IndName:  tokens[2],
			}
			if len(tokens) > 3 {
				ind.Args = tokens[3:]
			}
			mc.Indicators = append(mc.Indicators, ind)
		case len(tokens) == 2:
			// Threshold: KEY VALUE
			mc.Thresholds[tokens[0]] = tokens[1]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("modelFile: read %s: %w", path, err)
	}

	return mc, nil
}

// LoadThresholdSet 从 ModelConfig 构建 ThresholdSet
// C++: ThresholdSet::AddThreshold() in TradeBotUtils.cpp:2700-3079
// 使用 C++ UPPER_CASE key 名
func LoadThresholdSet(mc *ModelConfig) *types.ThresholdSet {
	ts := types.NewThresholdSet()

	// 将 UPPER_CASE key 转为 snake_case 并用 LoadFromMap
	m := make(map[string]float64, len(mc.Thresholds))
	for k, v := range mc.Thresholds {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		// C++ UPPER_CASE → Go snake_case
		snakeKey := upperCaseToSnakeCase(k)
		m[snakeKey] = f
	}
	ts.LoadFromMap(m)
	return ts
}

// upperCaseToSnakeCase 将 C++ UPPER_CASE 转换为 Go snake_case
// e.g. BEGIN_PLACE → begin_place, MAX_SIZE → max_size
func upperCaseToSnakeCase(s string) string {
	return strings.ToLower(s)
}

// UpperToSnake 导出版本，供 main.go 热加载使用
func UpperToSnake(s string) string {
	return upperCaseToSnakeCase(s)
}
