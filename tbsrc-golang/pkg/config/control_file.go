package config

import (
	"fmt"
	"os"
	"strings"
)

// ControlConfig 对应 C++ struct ControlConfig
// 参考: tbsrc/main/include/TradeBotUtils.h:602-613
// C++: LoadControlFile() in TradeBotUtils.cpp:1820-1865
// 格式: 单行空格分隔
//
//	baseName modelFile exchange id execStrat startTime endTime [secondName] [thirdName]
type ControlConfig struct {
	BaseName   string // Token[0]: e.g. ag_F_3_SFE
	ModelFile  string // Token[1]: e.g. ./models/model.ag2603.ag2605.par.txt.92201
	Exchange   string // Token[2]: e.g. SFE
	ID         string // Token[3]: e.g. 16
	ExecStrat  string // Token[4]: e.g. TB_PAIR_STRAT
	StartTime  string // Token[5]: e.g. 0900
	EndTime    string // Token[6]: e.g. 1500
	SecondName string // Token[7]: e.g. ag_F_5_SFE (pair strategies)
	ThirdName  string // Token[8]: e.g. (butterfly/arb strategies)
}

// ParseControlFile 解析 C++ controlFile 格式
// C++: LoadControlFile() in TradeBotUtils.cpp:1820-1865
// 读取文件第一个非空行，空格分隔为 tokens
func ParseControlFile(path string) (*ControlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("controlFile: read %s: %w", path, err)
	}

	// 取第一个非空行
	var line string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			line = l
			break
		}
	}
	if line == "" {
		return nil, fmt.Errorf("controlFile: %s: 文件为空或仅包含注释", path)
	}

	tokens := strings.Fields(line)
	if len(tokens) < 7 {
		return nil, fmt.Errorf("controlFile: %s: 至少需要 7 个字段，实际 %d: %s", path, len(tokens), line)
	}

	cc := &ControlConfig{
		BaseName:  tokens[0],
		ModelFile: tokens[1],
		Exchange:  tokens[2],
		ID:        tokens[3],
		ExecStrat: tokens[4],
		StartTime: tokens[5],
		EndTime:   tokens[6],
	}
	if len(tokens) >= 8 {
		cc.SecondName = tokens[7]
	}
	if len(tokens) >= 9 {
		cc.ThirdName = tokens[8]
	}
	return cc, nil
}

// BaseNameToSymbol 将 C++ baseName 转换为合约代码
// C++: ag_F_3_SFE → ag2603 (product + 年份后两位 + 月份两位)
// C++: au_F_4_SFE → au2604
// C++: rb_F_5_SFE → rb2605
// C++: au_O_C_10_1_576_SFE → (期权，暂不处理)
//
// 格式: <product>_F_<month>_<exchange>
// product: 小写字母（ag, au, al, rb...）
// month: 1-12
// year: 从当前年份推导（20xx → xx），由参数传入
func BaseNameToSymbol(baseName string, yearPrefix string) (string, error) {
	parts := strings.Split(baseName, "_")
	if len(parts) < 4 || parts[1] != "F" {
		return "", fmt.Errorf("baseName %q: 不是期货格式 (期望 <product>_F_<month>_<exchange>)", baseName)
	}
	product := strings.ToLower(parts[0])
	month := parts[2]
	// month 可能是 1-12，需要补零到两位
	if len(month) == 1 {
		month = "0" + month
	}
	return product + yearPrefix + month, nil
}
