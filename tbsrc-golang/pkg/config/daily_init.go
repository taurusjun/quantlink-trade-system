package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DailyInit 对应 C++ daily_init 文件内容
// 参考: PairwiseArbStrategy.cpp:18-43
// 文件格式: 每行一个数值，顺序固定:
//
//	line 0: avgSpreadRatio_ori
//	line 1: netpos_ytd1
//	line 2: netpos_2day1
//	line 3: netpos_agg2
type DailyInit struct {
	AvgSpreadOri float64
	NetposYtd1   int32
	Netpos2day1  int32
	NetposAgg2   int32
}

// LoadDailyInit 从文件加载 daily_init
// C++ 路径: ../data/daily_init.<strategyID>
func LoadDailyInit(path string) (*DailyInit, error) {
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
	lines := make([]string, 0, 4)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("daily_init: read %s: %w", path, err)
	}

	d := &DailyInit{}

	if len(lines) >= 1 {
		d.AvgSpreadOri, err = strconv.ParseFloat(lines[0], 64)
		if err != nil {
			return nil, fmt.Errorf("daily_init: parse avgSpreadOri: %w", err)
		}
	}
	if len(lines) >= 2 {
		v, err := strconv.ParseInt(lines[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("daily_init: parse netposYtd1: %w", err)
		}
		d.NetposYtd1 = int32(v)
	}
	if len(lines) >= 3 {
		v, err := strconv.ParseInt(lines[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("daily_init: parse netpos2day1: %w", err)
		}
		d.Netpos2day1 = int32(v)
	}
	if len(lines) >= 4 {
		v, err := strconv.ParseInt(lines[3], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("daily_init: parse netposAgg2: %w", err)
		}
		d.NetposAgg2 = int32(v)
	}

	return d, nil
}

// SaveDailyInit 保存 daily_init 到文件
func SaveDailyInit(path string, d *DailyInit) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("daily_init: create %s: %w", path, err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%.8f\n%d\n%d\n%d\n",
		d.AvgSpreadOri, d.NetposYtd1, d.Netpos2day1, d.NetposAgg2)
	if err != nil {
		return fmt.Errorf("daily_init: write %s: %w", path, err)
	}
	return nil
}

// DailyInitPath 返回 daily_init 文件路径
// C++: ../data/daily_init.<strategyID>
func DailyInitPath(dataDir string, strategyID int) string {
	return fmt.Sprintf("%s/daily_init.%d", dataDir, strategyID)
}
