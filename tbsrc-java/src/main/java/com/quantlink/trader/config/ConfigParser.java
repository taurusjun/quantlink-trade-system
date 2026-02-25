package com.quantlink.trader.config;

import com.quantlink.trader.core.ThresholdSet;

import java.io.IOException;
import java.lang.reflect.Field;
import java.util.Map;
import java.util.logging.Logger;

/**
 * 统一配置加载入口。
 * 迁移自: tbsrc-golang/pkg/config/build_config.go — BuildFromCppFiles()
 *
 * 从 C++ 遗留格式文件组合构建完整配置：
 * 1. controlFile → ControlConfig
 * 2. .cfg → CfgConfig (SHM keys)
 * 3. model .par.txt → ThresholdSet
 */
public class ConfigParser {

    private static final Logger logger = Logger.getLogger(ConfigParser.class.getName());

    /**
     * baseName → symbol 转换。
     * C++: ag_F_3_SFE → ag2603 (product + yearPrefix + month)
     *
     * 格式: <product>_F_<month>_<exchange>
     * month: 1-12 → 01-12
     *
     * @param baseName   C++ baseName (e.g. ag_F_3_SFE)
     * @param yearPrefix 年份后两位 (e.g. "26")
     * @return symbol (e.g. ag2603)
     */
    public static String baseNameToSymbol(String baseName, String yearPrefix) {
        String[] parts = baseName.split("_");
        if (parts.length < 4 || !parts[1].equals("F")) {
            throw new IllegalArgumentException("baseName '" + baseName
                + "': 不是期货格式 (期望 <product>_F_<month>_<exchange>)");
        }
        String product = parts[0].toLowerCase();
        String month = parts[2];
        if (month.length() == 1) {
            month = "0" + month;
        }
        return product + yearPrefix + month;
    }

    /**
     * 从 controlFile exchange 转为标准交易所名。
     * C++: SFE → SHFE
     */
    public static String exchangeToName(String exchange) {
        return switch (exchange.toUpperCase()) {
            case "SFE" -> "SHFE";
            case "ZCE", "CZCE" -> "ZCE";
            case "DCE" -> "DCE";
            case "CFFEX" -> "CFFEX";
            case "GFEX" -> "GFEX";
            default -> exchange;
        };
    }

    /**
     * 从 symbol 提取产品代码。
     * e.g. ag2603 → ag, au2604 → au
     */
    public static String extractProduct(String symbol) {
        for (int i = 0; i < symbol.length(); i++) {
            if (Character.isDigit(symbol.charAt(i))) {
                return symbol.substring(0, i);
            }
        }
        return symbol;
    }

    /**
     * 根据产品代码获取 tickSize。
     * 迁移自: tbsrc-golang/pkg/config/build_config.go — buildDefaultInstrumentConfig()
     */
    public static double getTickSize(String product) {
        return switch (product) {
            case "ag" -> 1.0;
            case "au" -> 0.02;
            case "al", "zn", "ss" -> 5.0;
            case "cu" -> 10.0;
            case "rb" -> 1.0;
            case "bu" -> 2.0;
            case "sc" -> 0.1;
            default -> 1.0;
        };
    }

    /**
     * 根据产品代码获取 lotSize (合约乘数)。
     * 迁移自: tbsrc-golang/pkg/config/build_config.go — buildDefaultInstrumentConfig()
     */
    public static double getLotSize(String product) {
        return switch (product) {
            case "ag" -> 15.0;
            case "au", "sc" -> 1000.0;
            case "al", "cu", "zn", "ss" -> 5.0;
            case "rb", "bu" -> 10.0;
            default -> 1.0;
        };
    }

    /**
     * 将 model file 的 UPPER_CASE 阈值加载到 ThresholdSet。
     * 迁移自: tbsrc-golang/pkg/config/model_file.go — LoadThresholdSet()
     *
     * 使用反射按字段名直接赋值（ThresholdSet 字段名即为 C++ UPPER_CASE key）。
     */
    public static void loadThresholds(ThresholdSet ts, Map<String, String> thresholds) {
        for (Map.Entry<String, String> entry : thresholds.entrySet()) {
            String key = entry.getKey();
            String value = entry.getValue();
            try {
                Field field = ThresholdSet.class.getField(key);
                Class<?> type = field.getType();
                if (type == double.class) {
                    field.setDouble(ts, Double.parseDouble(value));
                } else if (type == int.class) {
                    field.setInt(ts, (int) Double.parseDouble(value));
                } else if (type == long.class) {
                    field.setLong(ts, (long) Double.parseDouble(value));
                } else if (type == boolean.class) {
                    field.setBoolean(ts, Double.parseDouble(value) != 0);
                }
            } catch (NoSuchFieldException e) {
                // 未知 key，跳过
                logger.fine("阈值字段未找到: " + key);
            } catch (Exception e) {
                logger.warning("阈值加载失败: " + key + "=" + value + " (" + e.getMessage() + ")");
            }
        }
    }

    /**
     * 加载 StrategyConfig.cfg 中的 ACCOUNT 字段。
     */
    public static String loadAccount(String configDir) {
        try {
            CfgConfig stratCfg = CfgConfig.loadCfg(configDir + "/StrategyConfig.cfg");
            return stratCfg.globalKeys.getOrDefault("ACCOUNT", "");
        } catch (IOException e) {
            return "";
        }
    }
}
