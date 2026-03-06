package com.quantlink.trader.config;

import com.quantlink.trader.core.ThresholdSet;

import java.io.IOException;
import java.util.Map;
import java.util.logging.Logger;

/**
 * 统一配置加载入口。
 * 迁移自: tbsrc/main/TradeBotUtils.cpp — ThresholdSet::AddThreshold() (L2661-L3079)
 *         tbsrc/main/include/TradeBotUtils.h — struct ThresholdSet (L237-L504)
 *
 * 从 C++ 遗留格式文件组合构建完整配置：
 * 1. controlFile → ControlConfig
 * 2. .cfg → CfgConfig (SHM keys)
 * 3. model .par.txt → ThresholdSet
 */
public class ConfigParser {

    private static final Logger logger = Logger.getLogger(ConfigParser.class.getName());

    /**
     * baseName → symbol 转换（含跨年推断）。
     * 迁移自: tbsrc/common/Instrument.cpp:FillChinaFields2() (L601-631)
     *
     * C++: int month = ((expiryDate / 100) % 100);   // 当前月份
     *      int year = (expiryDate / 10000) % 100;     // 当前年份
     *      if (m_month < month) { year = nextYear; }  // 合约月份 < 当前月份 → 下一年
     *      sprintf(m_instrument, "%s%.2d%.2d", m_symbol, year, m_month);
     *
     * 格式: <product>_F_<month>_<exchange>
     * month: 1-12 → 01-12
     *
     * @param baseName   C++ baseName (e.g. ag_F_3_SFE)
     * @param yearPrefix 年份后两位 (e.g. "26")，若为空则自动推断
     * @return symbol (e.g. ag2603)
     */
    public static String baseNameToSymbol(String baseName, String yearPrefix) {
        String[] parts = baseName.split("_");
        if (parts.length < 4 || !parts[1].equals("F")) {
            throw new IllegalArgumentException("baseName '" + baseName
                + "': 不是期货格式 (期望 <product>_F_<month>_<exchange>)");
        }
        String product = parts[0].toLowerCase();
        int contractMonth = Integer.parseInt(parts[2]);
        String exchange = parts[3];

        // C++: int year = (expiryDate / 10000) % 100
        int year = Integer.parseInt(yearPrefix);
        // C++: int month = ((expiryDate / 100) % 100)  — 当前日历月份
        int currentMonth = java.time.LocalDate.now().getMonthValue();

        // C++: if (m_month < month) { auto next_year = GetNextMonth(m_currDate, 12); year = ... }
        // 合约月份 < 当前月份 → 合约属于下一年
        if (contractMonth < currentMonth) {
            year = (year + 1) % 100;
        }

        // C++: if (!strcmp(m_exchange, "ZCE")) sprintf("%s%.1d%.2d", ...) — 郑商所用1位年份
        //      else sprintf("%s%.2d%.2d", ...)
        String month = String.format("%02d", contractMonth);
        if ("ZCE".equals(exchange) || "CZCE".equals(exchange)) {
            // C++: sprintf(m_instrument, "%s%.1d%.2d", m_symbol, year % 10, m_month)
            return product + (year % 10) + month;
        } else {
            // C++: sprintf(m_instrument, "%s%.2d%.2d", m_symbol, year, m_month)
            return product + String.format("%02d", year) + month;
        }
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
            case "bu" -> 1.0;
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
     * 迁移自: tbsrc/main/TradeBotUtils.cpp — ThresholdSet::AddThreshold() (L2661-L3079)
     *
     * 1:1 对齐 C++ if-else 链，包括:
     * - 时间单位转换 (PAUSE×1e6, SQROFF_TIME×1e9, CANCELREQ_PAUSE×1e6, AGG_COOL_OFF×1e9,
     *   WINDOW_DURATION×1e6, #LOOKAHEAD×1e6, STAT_DURATION_LONG×1e9, STAT_DURATION_SMALL×1e9)
     * - 副作用赋值 (SIZE→BEGIN_SIZE/BID_SIZE/ASK_SIZE, MAX_SIZE→SMS_RATIO/BID_MAX_SIZE/ASK_MAX_SIZE,
     *   CROSS→CLOSE_CROSS, MAX_CROSS→MAX_SHORT_CROSS/MAX_LONG_CROSS 等)
     * - 名称重映射 (DECAY→DECAY1, PRODUCT→productName, #LOOKAHEAD→LOOKBACK_TIME,
     *   #DEP_STD_DEV→HISTORICAL_STDDEV, #TRGT_STD_DEV→TARGET_STD_DEV, STDEV_*→STDDEV_*)
     * - 特殊布尔处理 (QUOTE_MAX_QTY==1→true, CLOSE_PNL==0→false, CHECK_PNL==0→false, NEWS_FLAT!=0→true)
     * - 未知 key: C++ 调用 exit(1)，Java 抛异常
     */
    public static void loadThresholds(ThresholdSet ts, Map<String, String> thresholds) {
        for (Map.Entry<String, String> entry : thresholds.entrySet()) {
            String name = entry.getKey();
            String value = entry.getValue();
            addThreshold(ts, name, value);
        }
    }

    /**
     * 单个阈值加载 — 1:1 对齐 C++ ThresholdSet::AddThreshold(char *name, char *value)
     * 迁移自: tbsrc/main/TradeBotUtils.cpp:2661-3079
     */
    private static void addThreshold(ThresholdSet ts, String name, String value) {
        // C++: if (!strcmp(name, "SIZE"))
        switch (name) {
            case "SIZE" -> {
                // C++: SIZE = atoi(value); BEGIN_SIZE = SIZE;
                // C++: BID_SIZE = BID_SIZE == 0 ? SIZE : BID_SIZE;
                // C++: ASK_SIZE = ASK_SIZE == 0 ? SIZE : ASK_SIZE;
                ts.SIZE = (int) Double.parseDouble(value);
                ts.BEGIN_SIZE = ts.SIZE;
                ts.BID_SIZE = ts.BID_SIZE == 0 ? ts.SIZE : ts.BID_SIZE;
                ts.ASK_SIZE = ts.ASK_SIZE == 0 ? ts.SIZE : ts.ASK_SIZE;
            }
            case "BEGIN_SIZE" -> ts.BEGIN_SIZE = (int) Double.parseDouble(value);
            case "TA_SIZE" -> ts.TA_SIZE = (int) Double.parseDouble(value);
            case "MAX_SIZE" -> {
                // C++: MAX_SIZE = atoi(value); SMS_RATIO = int(MAX_SIZE / SIZE);
                // C++: BID_MAX_SIZE = BID_MAX_SIZE == 0 ? MAX_SIZE : BID_MAX_SIZE;
                // C++: ASK_MAX_SIZE = ASK_MAX_SIZE == 0 ? MAX_SIZE : ASK_MAX_SIZE;
                ts.MAX_SIZE = (int) Double.parseDouble(value);
                ts.SMS_RATIO = ts.SIZE != 0 ? ts.MAX_SIZE / ts.SIZE : 0;
                logger.info("SMS_RATIO " + ts.SMS_RATIO);
                ts.BID_MAX_SIZE = ts.BID_MAX_SIZE == 0 ? ts.MAX_SIZE : ts.BID_MAX_SIZE;
                ts.ASK_MAX_SIZE = ts.ASK_MAX_SIZE == 0 ? ts.MAX_SIZE : ts.ASK_MAX_SIZE;
            }
            case "SWEEP_PLACE" -> ts.SWEEP_PLACE = (int) Double.parseDouble(value);
            case "SWEEP_CLOSE" -> ts.SWEEP_CLOSE = (int) Double.parseDouble(value);
            case "SWEEP_PLACE_LEVEL" -> ts.SWEEP_PLACE_LEVEL = (int) Double.parseDouble(value);
            case "SWEEP_CLOSE_LEVEL" -> ts.SWEEP_CLOSE_LEVEL = (int) Double.parseDouble(value);
            case "MAX_OS_ORDER" -> ts.MAX_OS_ORDER = (int) Double.parseDouble(value);

            // ---- 特殊布尔处理 ----
            // C++: if (atoi(value) == 1) QUOTE_MAX_QTY = true;
            case "QUOTE_MAX_QTY" -> {
                if ((int) Double.parseDouble(value) == 1) ts.QUOTE_MAX_QTY = true;
            }
            // C++: if (atoi(value) == 0) CLOSE_PNL = false;
            case "CLOSE_PNL" -> {
                if ((int) Double.parseDouble(value) == 0) ts.CLOSE_PNL = false;
            }
            // C++: if (atoi(value) == 0) CHECK_PNL = false;
            case "CHECK_PNL" -> {
                if ((int) Double.parseDouble(value) == 0) ts.CHECK_PNL = false;
            }
            // C++: if (atoi(value) != 0) NEWS_FLAT = true;
            case "NEWS_FLAT" -> {
                if ((int) Double.parseDouble(value) != 0) ts.NEWS_FLAT = true;
            }

            // ---- 副作用: USE_* 开关 ----
            // C++: NOTIONAL_SIZE = atoi(value); USE_NOTIONAL = true;
            case "NOTIONAL_SIZE" -> {
                ts.NOTIONAL_SIZE = (int) Double.parseDouble(value);
                ts.USE_NOTIONAL = true;
            }
            // C++: PERCENT_SIZE = atoi(value); USE_PERCENT = true;
            case "PERCENT_SIZE" -> {
                ts.PERCENT_SIZE = (int) Double.parseDouble(value);
                ts.USE_PERCENT = true;
            }
            case "PERCENT_LEVEL" -> ts.PERCENT_LEVEL = (int) Double.parseDouble(value);
            // C++: NOTIONAL_MAX_SIZE = atoi(value); SMS_RATIO = int(NOTIONAL_MAX_SIZE / NOTIONAL_SIZE);
            case "NOTIONAL_MAX_SIZE" -> {
                ts.NOTIONAL_MAX_SIZE = (int) Double.parseDouble(value);
                ts.SMS_RATIO = ts.NOTIONAL_SIZE != 0 ? ts.NOTIONAL_MAX_SIZE / ts.NOTIONAL_SIZE : 0;
                logger.info("SMS_RATIO " + ts.SMS_RATIO);
            }
            case "PCA_COEFF1" -> ts.PCA_COEFF1 = Double.parseDouble(value);
            case "PCA_COEFF2" -> ts.PCA_COEFF2 = Double.parseDouble(value);
            case "PCA_COEFF3" -> ts.PCA_COEFF3 = Double.parseDouble(value);
            case "SUPPORTING_ORDERS" -> ts.SUPPORTING_ORDERS = (int) Double.parseDouble(value);
            case "MAX_ORDERS" -> ts.MAX_ORDERS = (int) Double.parseDouble(value);
            case "TAILING_ORDERS" -> ts.TAILING_ORDERS = (int) Double.parseDouble(value);

            // ---- 阈值参数 ----
            case "BEGIN_PLACE" -> ts.BEGIN_PLACE = Double.parseDouble(value);
            case "BEGIN_REMOVE" -> ts.BEGIN_REMOVE = Double.parseDouble(value);
            case "LONG_PLACE" -> ts.LONG_PLACE = Double.parseDouble(value);
            case "LONG_REMOVE" -> ts.LONG_REMOVE = Double.parseDouble(value);
            case "SHORT_PLACE" -> ts.SHORT_PLACE = Double.parseDouble(value);
            case "SHORT_REMOVE" -> ts.SHORT_REMOVE = Double.parseDouble(value);
            case "LONG_INC" -> ts.LONG_INC = Double.parseDouble(value);

            // ---- 时间参数（单位转换） ----
            // C++: PAUSE = (int64_t)(atol(value) * 1000000);  — 秒→微秒
            case "PAUSE" -> ts.PAUSE = (long) (Long.parseLong(value) * 1_000_000L);
            // C++: SQROFF_TIME = (int64_t)(atol(value) * 1000000000);  — 秒→纳秒
            case "SQROFF_TIME" -> ts.SQROFF_TIME = (long) (Long.parseLong(value) * 1_000_000_000L);
            case "SQROFF_AGG" -> ts.SQROFF_AGG = (int) Double.parseDouble(value);
            // C++: CANCELREQ_PAUSE = atoi(value) * 1000000;  — 秒→微秒
            case "CANCELREQ_PAUSE" -> ts.CANCELREQ_PAUSE = (int) Double.parseDouble(value) * 1_000_000L;
            case "IMPROVE" -> ts.IMPROVE = Double.parseDouble(value);
            // C++: AGG_COOL_OFF = atof(value) * 1000000000;  — 秒→纳秒
            case "AGG_COOL_OFF" -> ts.AGG_COOL_OFF = (long) (Double.parseDouble(value) * 1_000_000_000.0);
            case "PLACE_SPREAD" -> ts.PLACE_SPREAD = Double.parseDouble(value);
            case "PIL_FACTOR" -> ts.PIL_FACTOR = Double.parseDouble(value);

            // ---- CROSS 副作用 ----
            // C++: CROSS = atof(value); if (!USE_CLOSE_CROSS) CLOSE_CROSS = CROSS;
            case "CROSS" -> {
                ts.CROSS = Double.parseDouble(value);
                if (!ts.USE_CLOSE_CROSS) {
                    ts.CLOSE_CROSS = ts.CROSS;
                }
            }
            // C++: CLOSE_CROSS = atof(value); USE_CLOSE_CROSS = true;
            case "CLOSE_CROSS" -> {
                ts.CLOSE_CROSS = Double.parseDouble(value);
                ts.USE_CLOSE_CROSS = true;
            }
            case "CROSS_TARGET" -> ts.CROSS_TARGET = (int) Double.parseDouble(value);
            case "CROSS_TICKS" -> ts.CROSS_TICKS = (int) Double.parseDouble(value);
            case "TARGET_DELTA" -> ts.TARGET_DELTA = Double.parseDouble(value);
            case "CLOSE_IMPROVE" -> ts.CLOSE_IMPROVE = Double.parseDouble(value);
            case "OPP_QTY" -> ts.OPP_QTY = Double.parseDouble(value);
            case "SUPP_TOLERANCE" -> ts.SUPP_TOLERANCE = (int) Double.parseDouble(value);
            case "UPNL_LOSS" -> ts.UPNL_LOSS = Double.parseDouble(value);
            case "STOP_LOSS" -> ts.STOP_LOSS = Double.parseDouble(value);
            case "MAX_LOSS" -> ts.MAX_LOSS = Double.parseDouble(value);
            case "PT_LOSS" -> ts.PT_LOSS = Double.parseDouble(value);

            // C++: MAX_PRICE = atof(value); USE_PRICE_LIMIT = true;
            case "MAX_PRICE" -> {
                ts.MAX_PRICE = Double.parseDouble(value);
                ts.USE_PRICE_LIMIT = true;
            }
            case "MIN_PRICE" -> ts.MIN_PRICE = Double.parseDouble(value);

            // C++: MAX_CROSS = atof(value); MAX_SHORT_CROSS = MAX_CROSS; MAX_LONG_CROSS = MAX_CROSS;
            case "MAX_CROSS" -> {
                ts.MAX_CROSS = (int) Double.parseDouble(value);
                ts.MAX_SHORT_CROSS = ts.MAX_CROSS;
                ts.MAX_LONG_CROSS = ts.MAX_CROSS;
            }

            // C++: AHEAD_PERCENT = atof(value); USE_AHEAD_PERCENT = true;
            case "AHEAD_PERCENT" -> {
                ts.AHEAD_PERCENT = Double.parseDouble(value);
                ts.USE_AHEAD_PERCENT = true;
            }
            // C++: USE_PASSIVE_THOLD = false;  — 无条件设为 false
            case "USE_PASSIVE_THOLD" -> ts.USE_PASSIVE_THOLD = false;
            // C++: USE_LINEAR_THOLD = atoi(value);  — C++ bool 类型，atoi 隐式转换
            case "USE_LINEAR_THOLD" -> ts.USE_LINEAR_THOLD = (int) Double.parseDouble(value) != 0;
            case "AHEAD_SIZE" -> ts.AHEAD_SIZE = Double.parseDouble(value);
            case "DELTA_HEDGE" -> ts.DELTA_HEDGE = Double.parseDouble(value);
            case "SPREAD_EWA" -> ts.SPREAD_EWA = Double.parseDouble(value);
            case "MAX_SHORT_CROSS" -> ts.MAX_SHORT_CROSS = (int) Double.parseDouble(value);
            case "MAX_LONG_CROSS" -> ts.MAX_LONG_CROSS = (int) Double.parseDouble(value);
            case "MAX_IMPROVE" -> ts.MAX_IMPROVE = (int) Double.parseDouble(value);
            case "QUOTE_SKEW" -> ts.QUOTE_SKEW = Double.parseDouble(value);
            case "PT_PROFIT" -> ts.PT_PROFIT = Double.parseDouble(value);

            // C++: WINDOW_DURATION = atof(value) * 1000000;  — 秒→微秒
            case "WINDOW_DURATION" -> ts.WINDOW_DURATION = (long) (Double.parseDouble(value) * 1_000_000.0);

            // ---- 名称重映射: DECAY → DECAY1 ----
            // C++: else if (!strcmp(name, "DECAY")) DECAY1 = atof(value);
            case "DECAY" -> ts.DECAY1 = Double.parseDouble(value);
            case "DECAY1" -> ts.DECAY1 = Double.parseDouble(value);
            case "DECAY2" -> ts.DECAY2 = Double.parseDouble(value);
            case "LOCAL_STD_TYPE" -> ts.LOCAL_STD_TYPE = (int) Double.parseDouble(value);
            case "LOCAL_DEVIATION_WEIGHTAGE" -> ts.LOCAL_DEVIATION_WEIGHTAGE = Double.parseDouble(value);
            case "BASE_DEVIATION_WEIGHTAGE" -> ts.BASE_DEVIATION_WEIGHTAGE = Double.parseDouble(value);
            case "STDCOMP_VER" -> ts.STDCOMP_VER = (int) Double.parseDouble(value);

            // ---- 名称重映射: STDEV_* → STDDEV_* ----
            // C++: else if (!strcmp(name, "STDEV_LP")) STDDEV_LP = atof(value);
            case "STDEV_LP" -> ts.STDDEV_LP = Double.parseDouble(value);
            case "STDEV_LR" -> ts.STDDEV_LR = Double.parseDouble(value);
            case "STDEV_BR" -> ts.STDDEV_BR = Double.parseDouble(value);
            case "STDEV_BP" -> ts.STDDEV_BP = Double.parseDouble(value);
            case "STDEV_SP" -> ts.STDDEV_SP = Double.parseDouble(value);
            case "STDEV_SR" -> ts.STDDEV_SR = Double.parseDouble(value);

            // ---- 名称重映射: #LOOKAHEAD → LOOKBACK_TIME ----
            // C++: else if (!strcmp(name, "#LOOKAHEAD")) LOOKBACK_TIME = atol(value) * 1000000;  — 秒→微秒
            case "#LOOKAHEAD" -> ts.LOOKBACK_TIME = Long.parseLong(value) * 1_000_000L;
            // C++: else if (!strcmp(name, "#DEP_STD_DEV")) HISTORICAL_STDDEV = atof(value);
            case "#DEP_STD_DEV" -> ts.HISTORICAL_STDDEV = Double.parseDouble(value);
            case "VOLATILITY_CONST" -> ts.VOLATILITY_CONST = Double.parseDouble(value);
            case "ARCH_COEFF" -> ts.ARCH_COEFF = Double.parseDouble(value);
            case "GARCH_COEFF" -> ts.GARCH_COEFF = Double.parseDouble(value);
            case "MODE_INSTRUMENT1" -> ts.MODE_INSTRUMENT1 = (int) Double.parseDouble(value);
            case "MODE_INSTRUMENT2" -> ts.MODE_INSTRUMENT2 = (int) Double.parseDouble(value);
            case "TARGET_PNL_MODE" -> ts.TARGET_PNL_MODE = (int) Double.parseDouble(value);
            case "CLOSE_SPREAD" -> ts.CLOSE_SPREAD = Double.parseDouble(value);
            case "BEGIN_PLACE_STDDEV" -> ts.BEGIN_PLACE_STDDEV = Double.parseDouble(value);
            case "LONG_PLACE_STDDEV" -> ts.LONG_PLACE_STDDEV = Double.parseDouble(value);
            case "BEGIN_REMOVE_STDDEV" -> ts.BEGIN_REMOVE_STDDEV = Double.parseDouble(value);
            case "LONG_REMOVE_STDDEV" -> ts.LONG_REMOVE_STDDEV = Double.parseDouble(value);
            case "SHORT_PLACE_STDDEV" -> ts.SHORT_PLACE_STDDEV = Double.parseDouble(value);
            case "SQR_OFF_STDEV" -> ts.SQR_OFF_STDEV = Double.parseDouble(value);
            case "SQUARE_OFF_TIME" -> ts.SQUARE_OFF_TIME = Long.parseLong(value);
            case "SKIP_TIME" -> ts.SKIP_TIME = Long.parseLong(value);
            case "DEBUG_STMTS" -> ts.DEBUG_STMTS = (int) Double.parseDouble(value);
            case "CLOSING_SPREAD_DEV" -> ts.CLOSING_SPREAD_DEV = Double.parseDouble(value);
            case "VOLATILITY_TIME" -> ts.VOLATILITY_TIME = (long) Double.parseDouble(value);
            case "VWAP_RATIO" -> ts.VWAP_RATIO = Double.parseDouble(value);
            case "VWAP_COUNT" -> ts.VWAP_COUNT = Double.parseDouble(value);
            case "VWAP_DEPTH" -> ts.VWAP_DEPTH = Double.parseDouble(value);
            case "BIDASK_RATIO" -> ts.BIDASK_RATIO = Double.parseDouble(value);
            case "CROSS_FLAG" -> ts.CROSS_FLAG = (int) Double.parseDouble(value);
            case "ENTRY_BASED_SIGNAL" -> ts.ENTRY_BASED_SIGNAL = (int) Double.parseDouble(value);
            case "MEAN_DURATION_WINDOW" -> ts.MEAN_DURATION_WINDOW = Long.parseLong(value);
            case "StdevSqrOff_FLAG" -> ts.StdevSqrOff_FLAG = (int) Double.parseDouble(value);
            case "TICKSIZE" -> ts.TICKSIZE = Double.parseDouble(value);
            case "LEADLAG_FLAG" -> ts.LEADLAG_FLAG = (int) Double.parseDouble(value);
            case "DEP_BASKET" -> ts.DEP_BASKET = (int) Double.parseDouble(value);
            case "MODEL_ALGO" -> ts.MODEL_ALGO = (int) Double.parseDouble(value);
            case "HIDDEN_NEURONS" -> ts.HIDDEN_NEURONS = (int) Double.parseDouble(value);
            case "INP_FEAT_LENGTH" -> ts.INP_FEAT_LENGTH = (int) Double.parseDouble(value);
            case "LAGS" -> ts.LAGS = (int) Double.parseDouble(value);
            case "TWO_SIDED_QUOTE" -> ts.TWO_SIDED_QUOTE = (int) Double.parseDouble(value);
            case "ERR_STDEV" -> ts.ERR_STDEV = Double.parseDouble(value);
            case "CONFIDENCE_INTERVAL_BEGIN" -> ts.CONFIDENCE_INTERVAL_BEGIN = Double.parseDouble(value);
            case "CONFIDENCE_INTERVAL_CLOSE" -> ts.CONFIDENCE_INTERVAL_CLOSE = Double.parseDouble(value);
            case "LEARNING_RATE" -> ts.LEARNING_RATE = Double.parseDouble(value);
            case "DYNAMIC_WEIGHTS" -> ts.DYNAMIC_WEIGHTS = (int) Double.parseDouble(value);
            case "CONTINUOUS_TARGET_COMPUTATION" -> ts.CONTINUOUS_TARGET_COMPUTATION = (int) Double.parseDouble(value);
            case "DYNAMIC_DEVIATION_COMPUTATION" -> ts.DYNAMIC_DEVIATION_COMPUTATION = (int) Double.parseDouble(value);
            case "STDEV_IMPROVE" -> ts.STDEV_IMPROVE = Double.parseDouble(value);
            case "STDEV_CROSS" -> ts.STDEV_CROSS = Double.parseDouble(value);
            case "theta1_file" -> ts.theta1_file = value;
            case "theta2_file" -> ts.theta2_file = value;
            case "min_max_file" -> ts.min_max_file = value;
            case "ar_bask0_file" -> ts.ar_bask0_file = value;
            case "ar_bask1_file" -> ts.ar_bask1_file = value;
            case "cov_mat_file" -> ts.cov_mat_file = value;
            case "MAX_QUOTE_LEVEL" -> ts.MAX_QUOTE_LEVEL = (int) Double.parseDouble(value);
            case "SPREAD_COVER" -> ts.SPREAD_COVER = (int) Double.parseDouble(value);
            case "IMMEDIATE_POS_CLOSE" -> ts.IMMEDIATE_POS_CLOSE = (int) Double.parseDouble(value);
            case "DELTA" -> ts.DELTA = (int) Double.parseDouble(value);
            case "MAX_DELTA" -> ts.MAX_DELTA = (int) Double.parseDouble(value);
            // C++: else if (!strcmp(name, "#TRGT_STD_DEV")) TARGET_STD_DEV = atof(value);
            case "#TRGT_STD_DEV" -> ts.TARGET_STD_DEV = Double.parseDouble(value);
            case "PRICE_COOLOFF" -> ts.PRICE_COOLOFF = (int) Double.parseDouble(value);
            case "QUOTE_SIGNAL" -> ts.QUOTE_SIGNAL = (int) Double.parseDouble(value);
            // C++: STAT_DURATION_LONG = atof(value) * 1000000000;  — 秒→纳秒
            case "STAT_DURATION_LONG" -> ts.STAT_DURATION_LONG = (long) (Double.parseDouble(value) * 1_000_000_000.0);
            // C++: STAT_DURATION_SMALL = atof(value) * 1000000000;  — 秒→纳秒
            case "STAT_DURATION_SMALL" -> ts.STAT_DURATION_SMALL = (long) (Double.parseDouble(value) * 1_000_000_000.0);
            case "BEGIN_PLACE_HIGH" -> ts.BEGIN_PLACE_HIGH = Double.parseDouble(value);
            case "LONG_PLACE_HIGH" -> ts.LONG_PLACE_HIGH = Double.parseDouble(value);
            case "STAT_TRADE_THRESH" -> ts.STAT_TRADE_THRESH = Double.parseDouble(value);
            case "STAT_DECAY" -> ts.STAT_DECAY = (int) Double.parseDouble(value);
            case "PRICE_RATIO" -> ts.PRICE_RATIO = Double.parseDouble(value);
            case "HEDGE_RATIO" -> ts.HEDGE_RATIO = Double.parseDouble(value);
            case "HEDGE_THRES" -> ts.HEDGE_THRES = Double.parseDouble(value);
            case "HEDGE_SIZE_RATIO" -> ts.HEDGE_SIZE_RATIO = Double.parseDouble(value);
            case "ALPHA" -> ts.ALPHA = Double.parseDouble(value);
            case "MAX_DELTA_VALUE" -> ts.MAX_DELTA_VALUE = Double.parseDouble(value);
            case "MIN_DELTA_VALUE" -> ts.MIN_DELTA_VALUE = Double.parseDouble(value);
            case "MAX_DELTA_CHANGE" -> ts.MAX_DELTA_CHANGE = Double.parseDouble(value);
            // C++: else if (!strcmp(name, "PRODUCT")) product_name = string(value);
            case "PRODUCT" -> ts.productName = value;
            case "AVG_SPREAD_AWAY" -> ts.AVG_SPREAD_AWAY = (int) Double.parseDouble(value);
            case "SLOP" -> ts.SLOP = (int) Double.parseDouble(value);
            case "CONST" -> ts.CONST = Double.parseDouble(value);
            case "BID_SIZE" -> ts.BID_SIZE = (int) Double.parseDouble(value);
            case "BID_MAX_SIZE" -> ts.BID_MAX_SIZE = (int) Double.parseDouble(value);
            case "ASK_SIZE" -> ts.ASK_SIZE = (int) Double.parseDouble(value);
            case "ASK_MAX_SIZE" -> ts.ASK_MAX_SIZE = (int) Double.parseDouble(value);
            case "TVAR_KEY" -> ts.TVAR_KEY = (int) Double.parseDouble(value);
            case "TCACHE_KEY" -> ts.TCACHE_KEY = (int) Double.parseDouble(value);
            case "UNDERLYING_UPPER_BOND" -> ts.UNDERLYING_UPPER_BOND = Double.parseDouble(value);
            case "UNDERLYING_LOWER_BOND" -> ts.UNDERLYING_LOWER_BOND = Double.parseDouble(value);

            // C++: TBLOG << "Unknown Threshold Type" << endl; TBLOG << name << endl; exit(1);
            default -> {
                logger.severe("Unknown Threshold Type: " + name);
                throw new IllegalArgumentException("Unknown Threshold Type: " + name);
            }
        }
        // C++: TBLOG << name << " " << value << endl;
        logger.info(name + " " + value);
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
