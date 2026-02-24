package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DailyInit 对应 C++ daily_init 文件内容
// 参考: PairwiseArbStrategy.cpp SaveMatrix2 (line 653-686) / LoadMatrix2 (line 112-144)
// 文件格式: 第 1 行 header（空格分隔列名），第 2+ 行数据（空格分隔，首列为 strategyID）
//
// Header: "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 "
type DailyInit struct {
	StrategyID    int32
	Netpos2day1   int32   // header: "2day"
	AvgSpreadOri  float64 // header: "avgPx"
	OrigBaseName1 string  // header: "m_origbaseName1"
	OrigBaseName2 string  // header: "m_origbaseName2"
	NetposYtd1    int32   // header: "ytd1"
	NetposAgg2    int32   // header: "ytd2"
}

// LoadMatrix2 从文件加载 daily_init，与 C++ PairwiseArbStrategy::LoadMatrix2 一致
// C++: 第 1 行 Tokenize 为 header 列名数组，第 2+ 行按 header 索引存入 map<string,string>
// 参考: PairwiseArbStrategy.cpp:112-144
func LoadMatrix2(path string, strategyID int32) (*DailyInit, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在时返回零值
			return &DailyInit{}, nil
		}
		return nil, fmt.Errorf("daily_init: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("daily_init: read %s: %w", path, err)
	}

	if len(lines) < 2 {
		return nil, fmt.Errorf("daily_init: %s: 需要至少 2 行 (header + data)", path)
	}

	// C++: 第 1 行 Tokenize 为 header 列名数组
	headers := strings.Fields(lines[0])
	headerIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		headerIdx[h] = i
	}

	// C++: 第 2+ 行，Tokens[0] 为 strategyID，找到匹配行
	for _, dataLine := range lines[1:] {
		tokens := strings.Fields(dataLine)
		if len(tokens) == 0 {
			continue
		}

		sid, err := strconv.ParseInt(tokens[0], 10, 32)
		if err != nil {
			continue
		}
		if int32(sid) != strategyID {
			continue
		}

		// 构建 C++ 风格 map<string,string>
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(tokens) {
				row[h] = tokens[i]
			}
		}

		d := &DailyInit{StrategyID: strategyID}

		if v, ok := row["2day"]; ok {
			n, _ := strconv.ParseInt(v, 10, 32)
			d.Netpos2day1 = int32(n)
		}
		if v, ok := row["avgPx"]; ok {
			d.AvgSpreadOri, _ = strconv.ParseFloat(v, 64)
		}
		if v, ok := row["m_origbaseName1"]; ok {
			d.OrigBaseName1 = v
		}
		if v, ok := row["m_origbaseName2"]; ok {
			d.OrigBaseName2 = v
		}
		if v, ok := row["ytd1"]; ok {
			n, _ := strconv.ParseInt(v, 10, 32)
			d.NetposYtd1 = int32(n)
		}
		if v, ok := row["ytd2"]; ok {
			n, _ := strconv.ParseInt(v, 10, 32)
			d.NetposAgg2 = int32(n)
		}

		return d, nil
	}

	return nil, fmt.Errorf("daily_init: %s: strategyID %d 未找到", path, strategyID)
}

// SaveMatrix2 保存 daily_init 到文件，与 C++ PairwiseArbStrategy::SaveMatrix2 完全一致
// C++ header: "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 "
// C++ data:   strategyID << " " << "0 " << avgPx << " " << name1 << " " << name2 << " " << ytd1 << " " << ytd2
// 参考: PairwiseArbStrategy.cpp:653-686
func SaveMatrix2(path string, d *DailyInit) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("daily_init: create %s: %w", path, err)
	}
	defer f.Close()

	// C++ line 673: header（注意末尾空格，与 C++ 一致）
	_, err = fmt.Fprintf(f, "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 \n")
	if err != nil {
		return fmt.Errorf("daily_init: write header %s: %w", path, err)
	}

	// C++ line 675-676: data line
	// avgPx 使用 %f（Go 默认 6 位小数，与 C++ ios::fixed 默认精度一致）
	_, err = fmt.Fprintf(f, "%d %d %f %s %s %d %d\n",
		d.StrategyID, d.Netpos2day1, d.AvgSpreadOri,
		d.OrigBaseName1, d.OrigBaseName2,
		d.NetposYtd1, d.NetposAgg2)
	if err != nil {
		return fmt.Errorf("daily_init: write data %s: %w", path, err)
	}
	return nil
}

// DailyInitPath 返回 daily_init 文件路径
// C++: ../data/daily_init.<strategyID>
func DailyInitPath(dataDir string, strategyID int) string {
	return fmt.Sprintf("%s/daily_init.%d", dataDir, strategyID)
}
