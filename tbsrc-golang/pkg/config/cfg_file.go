package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CfgConfig 对应 C++ configFile (.cfg INI 格式) 中的配置
// 参考: tbsrc/main/main.cpp:424-436 (LoadCfg)
// 参考: hftbase Configfile (illuminati::Configfile)
// 格式:
//
//	KEY = VALUE (全局)
//	[SECTION]
//	KEY = VALUE (section 内)
type CfgConfig struct {
	// 全局参数
	Product    string // PRODUCT
	Exchanges  string // EXCHANGES (e.g. "CHINA_SHFE")
	GlobalKeys map[string]string

	// Per-exchange section 参数 (e.g. [CHINA_SHFE])
	Sections map[string]map[string]string
}

// ParseCfgFile 解析 C++ .cfg INI 格式配置文件
// C++: illuminati::Configfile::LoadCfg() in hftbase
// 格式: KEY = VALUE，支持 [SECTION]，# 和 ; 为注释
func ParseCfgFile(path string) (*CfgConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cfgFile: open %s: %w", path, err)
	}
	defer f.Close()

	cfg := &CfgConfig{
		GlobalKeys: make(map[string]string),
		Sections:   make(map[string]map[string]string),
	}

	scanner := bufio.NewScanner(f)
	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// [SECTION] header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			if _, ok := cfg.Sections[currentSection]; !ok {
				cfg.Sections[currentSection] = make(map[string]string)
			}
			continue
		}

		// KEY = VALUE or KEY=VALUE
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		if currentSection == "" {
			cfg.GlobalKeys[key] = value
		} else {
			cfg.Sections[currentSection][key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cfgFile: read %s: %w", path, err)
	}

	// 提取常用全局字段
	cfg.Product = cfg.GlobalKeys["PRODUCT"]
	cfg.Exchanges = cfg.GlobalKeys["EXCHANGES"]

	return cfg, nil
}

// GetExchangeConfig 获取指定交易所 section 的 SHM 配置
// 如果 exchange 为空，使用 cfg.Exchanges 中第一个
func (cfg *CfgConfig) GetExchangeConfig(exchange string) (mdKey, reqKey, respKey, clientStoreKey, mdSize, reqSize, respSize int, err error) {
	if exchange == "" {
		exchange = cfg.Exchanges
	}

	section, ok := cfg.Sections[exchange]
	if !ok {
		err = fmt.Errorf("cfgFile: section [%s] 不存在", exchange)
		return
	}

	mdKey, _ = strconv.Atoi(section["MDSHMKEY"])
	reqKey, _ = strconv.Atoi(section["ORSREQUESTSHMKEY"])
	respKey, _ = strconv.Atoi(section["ORSRESPONSESHMKEY"])
	clientStoreKey, _ = strconv.Atoi(section["CLIENTSTORESHMKEY"])
	mdSize, _ = strconv.Atoi(section["MDSHMSIZE"])
	reqSize, _ = strconv.Atoi(section["ORSREQUESTSHMSIZE"])
	respSize, _ = strconv.Atoi(section["ORSRESPONSESHMSIZE"])
	return
}
