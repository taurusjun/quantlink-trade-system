package com.quantlink.trader.core;

/**
 * 策略阈值参数集。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h — struct ThresholdSet (line 237-504)
 *
 * 所有默认值与 C++ ThresholdSet() 构造函数完全一致。
 */
public class ThresholdSet {

    // ---- 开关参数 ----
    // 迁移自: TradeBotUtils.h:241-251
    public boolean USE_NOTIONAL = false;
    public boolean USE_PERCENT = false;
    public boolean USE_PRICE_LIMIT = false;
    public boolean USE_AHEAD_PERCENT = false;
    public boolean USE_CLOSE_CROSS = false;
    public boolean USE_PASSIVE_THOLD = false;
    public boolean USE_LINEAR_THOLD = false; // C++: int USE_LINEAR_THOLD = 0
    public boolean QUOTE_MAX_QTY = false;
    public boolean CLOSE_PNL = true;
    public boolean CHECK_PNL = true;
    public boolean NEWS_FLAT = false;

    // ---- 仓位参数 ----
    // 迁移自: TradeBotUtils.h:336-345
    public int SIZE;
    public int TA_SIZE;
    public int BEGIN_SIZE;
    public int MAX_SIZE;
    public int PERCENT_SIZE;
    public int PERCENT_LEVEL = 1;
    public int NOTIONAL_SIZE;
    public int NOTIONAL_MAX_SIZE;
    public int SMS_RATIO;

    // ---- 买卖方向参数 ----
    // 迁移自: TradeBotUtils.h:496-503
    public int BID_SIZE = 0;
    public int BID_MAX_SIZE = 0;
    public int ASK_SIZE = 0;
    public int ASK_MAX_SIZE = 0;

    // ---- 风控参数 ----
    // 迁移自: TradeBotUtils.h:252-260
    public double OPP_QTY = 1_000_000_000;
    public int SUPP_TOLERANCE = 1;
    public double AHEAD_PERCENT = 100;
    public double AHEAD_SIZE = 1_000_000_000_000.0;
    public int SZAHEAD_NOCXL = 1_000_000;
    public int BOOKSZ_NOCXL = 1_000_000;
    public int AGGFLAT_BOOKSIZE = 0;
    public double AGGFLAT_BOOKFRAC = 0;
    public int MAX_OS_ORDER = 5;

    // ---- PNL/止损参数 ----
    // 迁移自: TradeBotUtils.h:261-267
    public double UPNL_LOSS = 10_000_000_000.0;
    public double STOP_LOSS = 10_000_000_000.0;
    public double MAX_LOSS = 100_000_000_000.0;
    public double PT_LOSS = 1_000_000;
    public double PT_PROFIT = 1_000_000;
    public double LONG_INC = 0;
    public double MAX_PRICE = 1_000_000_000_000.0;
    public double MIN_PRICE = -1000;

    // ---- 阈值参数 ----
    // 迁移自: TradeBotUtils.h:365-370
    public double BEGIN_PLACE;
    public double BEGIN_REMOVE;
    public double LONG_PLACE;
    public double LONG_REMOVE;
    public double SHORT_PLACE;
    public double SHORT_REMOVE;

    // ---- StdDev 阈值 ----
    // 迁移自: TradeBotUtils.h:419-424
    public double BEGIN_PLACE_STDDEV;
    public double LONG_PLACE_STDDEV;
    public double BEGIN_REMOVE_STDDEV;
    public double LONG_REMOVE_STDDEV;
    public double SHORT_PLACE_STDDEV;
    public double SQR_OFF_STDEV;

    // ---- VWAP ----
    // 迁移自: TradeBotUtils.h:272-275
    public double VWAP_RATIO = 1;
    public double VWAP_COUNT = 100;
    public double VWAP_DEPTH = 10;
    public double BIDASK_RATIO = 1;

    // ---- EWA / Spread ----
    // 迁移自: TradeBotUtils.h:276-284
    public double SPREAD_EWA = 0.6;
    public double CLOSE_CROSS = 100_000_000_000.0;
    public double CROSS = 1_000_000_000;
    public int CROSS_TARGET = 0;
    public int CROSS_TICKS = 0;
    public double IMPROVE = 1_000_000_000;
    public long AGG_COOL_OFF = 0;
    public double PLACE_SPREAD = 0.0;
    public double PIL_FACTOR = 0.0;

    // ---- CROSS 限制 ----
    // 迁移自: TradeBotUtils.h:285-291
    public int MAX_CROSS = 1_000_000_000;
    public int MAX_LONG_CROSS = 1_000_000_000;
    public int MAX_SHORT_CROSS = 1_000_000_000;
    public int MAX_QUOTE_SPREAD = 1_000_000_000;
    public double CLOSE_IMPROVE = -1;
    public double QUOTE_SKEW = 0;
    public double DELTA_HEDGE = 100_000;

    // ---- 时间参数 ----
    // 迁移自: TradeBotUtils.h:292-294
    public long PAUSE = 0;
    public long CANCELREQ_PAUSE = 0;
    public int MAX_QUOTE_LEVEL = 3;

    // ---- Sweep ----
    // 迁移自: TradeBotUtils.h:346-350
    public int SWEEP_PLACE;
    public int SWEEP_CLOSE;
    public int SWEEP_PLACE_LEVEL = 0;
    public int SWEEP_CLOSE_LEVEL = 0;
    public int SUPPORTING_ORDERS = 0;
    public int MAX_ORDERS = 0;
    public int TAILING_ORDERS = 0;

    // ---- Delta ----
    // 迁移自: TradeBotUtils.h:358-360
    public int DELTA;
    public int MAX_DELTA;

    // ---- PCA 系数 ----
    // 迁移自: TradeBotUtils.h:361-363
    public double PCA_COEFF1 = 0;
    public double PCA_COEFF2 = 0;
    public double PCA_COEFF3 = 0;
    public int QUOTE_SIGNAL = 0;

    // ---- 平仓时间 ----
    // 迁移自: TradeBotUtils.h:305-307
    public long SQROFF_TIME = 0;
    public int SQROFF_AGG = 0;
    public double TARGET_DELTA = 0;

    // ---- 统计参数 ----
    // 迁移自: TradeBotUtils.h:308-316
    public long STAT_DURATION_SMALL = 0;
    public long STAT_DURATION_LONG = 1;
    public double STAT_TRADE_THRESH = 0;
    public int STAT_DECAY = 5;
    public double MAX_DELTA_VALUE = 1;
    public double MIN_DELTA_VALUE = -1;
    public double MAX_DELTA_CHANGE = 2;

    // ---- 追单参数（pqr 20240902） ----
    // 迁移自: TradeBotUtils.h:490-493
    public int AVG_SPREAD_AWAY = 20;
    public int SLOP = 20;

    // ---- SPD hedge ----
    // 迁移自: TradeBotUtils.h:494-495
    public double CONST = 0.0;

    // ---- 其他 ----
    // 迁移自: TradeBotUtils.h 其余参数
    public int MIN_EXTR_IND = 0;
    public double TARGET_STD_DEV = 0.0;
    public int PRICE_COOLOFF = 0;
    public double ALPHA;
    public double HEDGE_RATIO;
    public double HEDGE_THRES;
    public double HEDGE_SIZE_RATIO;
    public double PRICE_RATIO;
    public double BEGIN_PLACE_HIGH;
    public double LONG_PLACE_HIGH;
    public int TWO_SIDED_QUOTE;
    public double TICKSIZE;
    public double DECAY;
    public double DECAY1;
    public double DECAY2;
    public long WINDOW_DURATION;
    public long LOOKBACK_TIME;
    public double HISTORICAL_STDDEV;
    public double TRGT_STDDEV;
    public double VOLATILITY_CONST;
    public long VOLATILITY_TIME;
    public double ARCH_COEFF;
    public double GARCH_COEFF;
    public int DEBUG_STMTS;
    public double CLOSE_SPREAD;
    public double CLOSING_SPREAD_DEV;
    public long SQUARE_OFF_TIME;
    public long SKIP_TIME;
    public int MAX_IMPROVE;
    public int CROSS_FLAG;
    public int ENTRY_BASED_SIGNAL;
    public long MEAN_DURATION_WINDOW;
    public int LOCAL_STD_TYPE;
    public double LOCAL_DEVIATION_WEIGHTAGE;
    public double BASE_DEVIATION_WEIGHTAGE;
    public int STDCOMP_VER;
    public double STDDEV_LP;
    public double STDDEV_LR;
    public double STDDEV_BP;
    public double STDDEV_BR;
    public double STDDEV_SP;
    public double STDDEV_SR;
    public double ERR_STDEV;
    public double LEARNING_RATE;
    public double STDEV_IMPROVE;
    public double STDEV_CROSS;
    public int SPREAD_COVER;
    public int IMMEDIATE_POS_CLOSE;

    // 迁移自: TradeBotUtils.h:500-503
    public int TVAR_KEY = -1;
    public int TCACHE_KEY = -1;
    public double UNDERLYING_UPPER_BOND = -1;
    public double UNDERLYING_LOWER_BOND = -1;

    // 迁移自: TradeBotUtils.h:474 — product_name
    public String productName = "";

    // ---- 以下字段来自 C++ ThresholdSet 完整定义 ----
    // 迁移自: TradeBotUtils.h:430-460

    // 模型/算法参数
    public int MODE_INSTRUMENT1;
    public int MODE_INSTRUMENT2;
    public int TARGET_PNL_MODE;
    public int StdevSqrOff_FLAG;
    public int LEADLAG_FLAG;
    public int DEP_BASKET;
    public int MODEL_ALGO;
    public int INP_FEAT_LENGTH;
    public int HIDDEN_NEURONS;
    public int LAGS;
    public int DYNAMIC_WEIGHTS;
    public double CONFIDENCE_INTERVAL_BEGIN;
    public double CONFIDENCE_INTERVAL_CLOSE;
    public int CONTINUOUS_TARGET_COMPUTATION;
    public int DYNAMIC_DEVIATION_COMPUTATION;

    // 模型文件路径
    // 迁移自: TradeBotUtils.h:467-472
    public String theta1_file = "";
    public String theta2_file = "";
    public String min_max_file = "";
    public String ar_bask0_file = "";
    public String ar_bask1_file = "";
    public String cov_mat_file = "";
}
