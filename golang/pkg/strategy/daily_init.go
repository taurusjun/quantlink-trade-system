// Package strategy provides trading strategy implementations
// C++: tbsrc/Strategies/PairwiseArbStrategy.cpp - LoadMatrix2, SaveMatrix2
package strategy

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// dataDir 全局数据目录，由配置设置
// 默认为 "data"，实盘和模拟盘应使用不同目录
var dataDir = "data"

// SetDataDir 设置数据目录
// 应在策略初始化前调用，通常由 Trader 根据配置设置
func SetDataDir(dir string) {
	if dir != "" {
		dataDir = dir
		log.Printf("[Strategy] Data directory set to: %s", dataDir)
	}
}

// GetDataDir 获取当前数据目录
func GetDataDir() string {
	return dataDir
}

// DailyInitRow 每日初始化数据行
// C++: std::map<std::string, std::string> row
type DailyInitRow struct {
	StrategyID     int32   // StrategyID
	TwoDay         int32   // 2day - 今日新开仓（通常为 0）
	AvgPx          float64 // avgPx - 价差均值（avgSpreadRatio_ori）
	OrigBaseName1  string  // m_origbaseName1 - Leg1 品种名
	OrigBaseName2  string  // m_origbaseName2 - Leg2 品种名
	Ytd1           int32   // ytd1 - Leg1 昨仓 (m_netpos_pass)
	Ytd2           int32   // ytd2 - Leg2 主动仓 (m_netpos_agg)
}

// MxDailyInit2 每日初始化数据映射
// C++: std::map<int32_t, std::map<std::string, std::string>> mx_daily_init2
type MxDailyInit2 map[int32]*DailyInitRow

// LoadMatrix2 加载每日初始化数据
// C++: PairwiseArbStrategy::LoadMatrix2(std::string filepath)
// 文件格式:
//
//	StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
//	92201 0 -24.441424 ag_F_2_SFE ag_F_4_SFE -2 2
func LoadMatrix2(filepath string) (MxDailyInit2, error) {
	mx := make(MxDailyInit2)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("LoadMatrix2: open %s failed: %w", filepath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 读取表头
	// C++: getline(fileInput, line); Tokenizer((char *)line.c_str(), mx_header, TokenCount, " ");
	var mxHeader []string
	if scanner.Scan() {
		mxHeader = strings.Fields(scanner.Text())
	}

	// 读取数据行
	// C++: while (getline(fileInput, line2)) { ... }
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		tokens := strings.Fields(line)
		if len(tokens) < 7 {
			log.Printf("LoadMatrix2: invalid line: %s", line)
			continue
		}

		// C++: int32_t strategyid = (atoi)(Tokens[0]);
		strategyID, _ := strconv.ParseInt(tokens[0], 10, 32)

		row := &DailyInitRow{
			StrategyID: int32(strategyID),
		}

		// C++: mx[strategyid].emplace(std::string{mx_header[i]}, std::string{Tokens[i]});
		for i := 1; i < len(tokens) && i < len(mxHeader); i++ {
			key := mxHeader[i]
			value := tokens[i]

			switch key {
			case "2day":
				v, _ := strconv.ParseInt(value, 10, 32)
				row.TwoDay = int32(v)
			case "avgPx":
				row.AvgPx, _ = strconv.ParseFloat(value, 64)
			case "m_origbaseName1":
				row.OrigBaseName1 = value
			case "m_origbaseName2":
				row.OrigBaseName2 = value
			case "ytd1":
				v, _ := strconv.ParseInt(value, 10, 32)
				row.Ytd1 = int32(v)
			case "ytd2":
				v, _ := strconv.ParseInt(value, 10, 32)
				row.Ytd2 = int32(v)
			}
		}

		mx[int32(strategyID)] = row
	}

	return mx, nil
}

// SaveMatrix2 保存每日初始化数据
// C++: PairwiseArbStrategy::SaveMatrix2(std::string filepath)
//
//	void PairwiseArbStrategy::SaveMatrix2(std::string filepath) {
//	    while (true) {
//	        auto fp = fopen(filepath.c_str(), "aw+");
//	        if (0 == flock(fileno(fp), LOCK_EX)) {
//	            std::ofstream out(filepath, ios::out);
//	            out << Head << std::endl;
//	            out << m_strategyID << " " << "0 " << avgSpreadRatio_ori << " " << ...;
//	            flock(fileno(fp), LOCK_UN);
//	            break;
//	        }
//	    }
//	}
func SaveMatrix2(filepath string, strategyID int32, avgSpreadRatio_ori float64,
	origBaseName1, origBaseName2 string, netpos_pass1, netpos_agg2 int32) error {

	// C++: while (true) { ... if (0 == flock(...)) break; else sleep(1); }
	for {
		file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("SaveMatrix2: open %s failed: %w", filepath, err)
		}

		// C++: flock(fileno(fp), LOCK_EX)
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != nil {
			file.Close()
			log.Printf("SaveMatrix2: waiting for lock on %s...", filepath)
			// C++: sleep(1)
			continue
		}

		// C++: const string Head = "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 ";
		// C++: out << Head << std::endl;
		head := "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 \n"
		file.WriteString(head)

		// C++: out << m_strategyID << " " << "0 " << avgSpreadRatio_ori << " "
		//          << m_firstStrat->m_instru->m_origbaseName << " "
		//          << m_secondStrat->m_instru->m_origbaseName << " "
		//          << m_firstStrat->m_netpos_pass << " "
		//          << m_secondStrat->m_netpos_agg << endl;
		line := fmt.Sprintf("%d 0 %f %s %s %d %d\n",
			strategyID,
			avgSpreadRatio_ori,
			origBaseName1,
			origBaseName2,
			netpos_pass1,
			netpos_agg2,
		)
		file.WriteString(line)

		// C++: flock(fileno(fp), LOCK_UN)
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		break
	}

	return nil
}

// GetDailyInitPath 获取 daily_init 文件路径
// C++: std::string("../data/daily_init.") + std::to_string(m_strategyID)
// 使用全局 dataDir 变量，支持实盘/模拟盘数据隔离
func GetDailyInitPath(strategyID int32) string {
	return filepath.Join(dataDir, fmt.Sprintf("daily_init.%d", strategyID))
}
