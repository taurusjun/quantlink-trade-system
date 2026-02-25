package com.quantlink.trader.config;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;

/**
 * .cfg INI 文件解析结果。
 * 迁移自: hftbase Configfile (illuminati::Configfile)
 * C++: illuminati::Configfile::LoadCfg()
 *
 * 格式:
 *   KEY = VALUE (全局)
 *   [SECTION]
 *   KEY = VALUE (section 内)
 */
public class CfgConfig {
    public String product;
    public String exchanges;
    public final Map<String, String> globalKeys = new HashMap<>();
    public final Map<String, Map<String, String>> sections = new HashMap<>();

    /**
     * 解析 .cfg INI 文件。
     * C++: illuminati::Configfile::LoadCfg()
     */
    public static CfgConfig parse(String path) throws IOException {
        CfgConfig cfg = new CfgConfig();
        String currentSection = null;

        try (BufferedReader reader = Files.newBufferedReader(Path.of(path))) {
            String line;
            while ((line = reader.readLine()) != null) {
                line = line.trim();
                if (line.isEmpty() || line.startsWith("#") || line.startsWith(";")) {
                    continue;
                }

                // [SECTION] header
                if (line.startsWith("[") && line.endsWith("]")) {
                    currentSection = line.substring(1, line.length() - 1);
                    sections(cfg, currentSection);
                    continue;
                }

                // KEY = VALUE
                int eqIdx = line.indexOf('=');
                if (eqIdx < 0) continue;
                String key = line.substring(0, eqIdx).trim();
                String value = line.substring(eqIdx + 1).trim();

                if (currentSection == null) {
                    cfg.globalKeys.put(key, value);
                } else {
                    sections(cfg, currentSection).put(key, value);
                }
            }
        }

        cfg.product = cfg.globalKeys.getOrDefault("PRODUCT", "");
        cfg.exchanges = cfg.globalKeys.getOrDefault("EXCHANGES", "");
        return cfg;
    }

    private static Map<String, String> sections(CfgConfig cfg, String section) {
        return cfg.sections.computeIfAbsent(section, k -> new HashMap<>());
    }

    /**
     * 获取交易所 SHM 配置。
     * C++: 从 [CHINA_SHFE] section 读取 MDSHMKEY 等
     */
    public int[] getExchangeShmConfig(String exchange) {
        if (exchange == null || exchange.isEmpty()) {
            exchange = this.exchanges;
        }
        Map<String, String> section = sections.get(exchange);
        if (section == null) {
            throw new IllegalArgumentException("cfgFile: section [" + exchange + "] 不存在");
        }
        return new int[]{
            parseInt(section, "MDSHMKEY"),
            parseInt(section, "ORSREQUESTSHMKEY"),
            parseInt(section, "ORSRESPONSESHMKEY"),
            parseInt(section, "CLIENTSTORESHMKEY"),
            parseInt(section, "MDSHMSIZE"),
            parseInt(section, "ORSREQUESTSHMSIZE"),
            parseInt(section, "ORSRESPONSESHMSIZE"),
        };
    }

    private static int parseInt(Map<String, String> map, String key) {
        String v = map.get(key);
        return v == null ? 0 : Integer.parseInt(v.trim());
    }
}
