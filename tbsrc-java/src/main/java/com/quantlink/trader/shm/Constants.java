package com.quantlink.trader.shm;

/**
 * C++ 枚举和常量的 Java 映射。
 * 使用 public static final int 而非 Java enum，以便与 C++ int 值直接对比。
 *
 * C++ 原代码:
 *   hftbase/CommonUtils/include/orderresponse.h  — 所有 ORS 枚举和结构体常量
 *   hftbase/CommonUtils/include/marketupdateNew.h — 行情常量和交易所代码
 *   hftbase/CommonUtils/include/constants.h       — ORDERID_RANGE
 */
public final class Constants {

    private Constants() {} // 不可实例化

    // =====================================================================
    // 大小常量
    // C++: hftbase/CommonUtils/include/marketupdateNew.h
    // =====================================================================

    /** C++: const int32_t INTEREST_LEVELS = 20; (marketupdateNew.h:21) */
    public static final int INTEREST_LEVELS = 20;

    /** C++: const int32_t MAX_SYMBOL_SIZE = 50; (marketupdateNew.h:22) */
    public static final int MAX_SYMBOL_SIZE = 50;

    // C++: hftbase/CommonUtils/include/orderresponse.h
    /** C++: const int32_t MAX_ORS_CLIENTS = 250; (orderresponse.h:17) */
    public static final int MAX_ORS_CLIENTS = 250;

    /** C++: const int32_t MAX_ACCNTID_LEN = 10; (orderresponse.h:18) — AccountID[MAX_ACCNTID_LEN+1] */
    public static final int MAX_ACCNT_ID_LEN = 10;

    /** AccountID 数组大小 = MAX_ACCNTID_LEN + 1 (含 null terminator) */
    public static final int ACCOUNT_ID_SIZE = MAX_ACCNT_ID_LEN + 1; // 11

    /** C++: const int32_t MAX_INSTRNAME_SIZE = 32; (orderresponse.h:21) */
    public static final int MAX_INSTR_NAME_SZ = 32;

    /** C++: const int32_t MAX_TRADE_ID_SIZE = 21; (orderresponse.h:22) */
    public static final int MAX_TRADE_ID_SIZE = 21;

    /** C++: const int32_t MAX_PRODUCT_SIZE = 32; (orderresponse.h:23) */
    public static final int MAX_PRODUCT_SIZE = 32;

    // C++: hftbase/CommonUtils/include/constants.h
    /** C++: static const int32_t ORDERID_RANGE = 1000000; (constants.h:14) */
    public static final int ORDERID_RANGE = 1_000_000;

    /**
     * C++: static const int32_t DEFAULT_NOT_POSSIBLE_CLIENTID = 99999999; (constants.h:17)
     * 用于 m_all_clientIds 数组的默认填充值，表示未分配的 slot。
     */
    public static final int DEFAULT_NOT_POSSIBLE_CLIENTID = 99999999;

    // =====================================================================
    // Fill Indicators
    // C++: orderresponse.h:25-27
    // =====================================================================
    public static final int NO_FILL_INDICATOR = 0;
    public static final int PARTIAL_FILL_INDICATOR = 1;
    public static final int FULL_FILL_INDICATOR = 2;

    // =====================================================================
    // RequestType 枚举
    // C++: enum RequestType { NEWORDER, MODIFYORDER, ... } (orderresponse.h:59-70)
    // =====================================================================
    public static final int REQUEST_NEWORDER = 0;
    public static final int REQUEST_MODIFYORDER = 1;
    public static final int REQUEST_CANCELORDER = 2;
    public static final int REQUEST_ORDERSTATUS = 3;
    public static final int REQUEST_SESSIONMSG = 4;
    public static final int REQUEST_HEARTBEAT = 5;
    public static final int REQUEST_OPTEXEC = 6;         // 期权行权指令
    public static final int REQUEST_OPTEXEC_CANCEL = 7;  // 期权撤销行权指令
    public static final int REQUEST_TYPE_NUM = 8;         // C++: RequestTypeNum

    // =====================================================================
    // ResponseType 枚举
    // C++: enum ResponseType { NEW_ORDER_CONFIRM, ... } (orderresponse.h:379-401)
    // =====================================================================
    public static final int RESP_NEW_ORDER_CONFIRM = 0;
    public static final int RESP_NEW_ORDER_FREEZE = 1;
    public static final int RESP_MODIFY_ORDER_CONFIRM = 2;
    public static final int RESP_CANCEL_ORDER_CONFIRM = 3;
    public static final int RESP_TRADE_CONFIRM = 4;
    public static final int RESP_ORDER_ERROR = 5;
    public static final int RESP_MODIFY_ORDER_REJECT = 6;
    public static final int RESP_CANCEL_ORDER_REJECT = 7;
    public static final int RESP_ORS_REJECT = 8;
    public static final int RESP_RMS_REJECT = 9;
    public static final int RESP_SIM_REJECT = 10;
    public static final int RESP_BUSINESS_REJECT = 11;
    public static final int RESP_MODIFY_ORDER_PENDING = 12;
    public static final int RESP_CANCEL_ORDER_PENDING = 13;
    public static final int RESP_ORDERS_PER_DAY_LIMIT_REJECT = 14;
    public static final int RESP_ORDERS_PER_DAY_LIMIT_WARNING = 15;
    public static final int RESP_ORDER_EXPIRED = 16;
    public static final int RESP_STOP_LOSS_WARNING = 17;
    public static final int RESP_NULL_RESPONSE = 18;
    public static final int RESPONSE_TYPE_NUM = 19;       // C++: ResponseTypeNum

    // =====================================================================
    // OrderType 枚举
    // C++: enum OrderType { LIMIT=1, MARKET=2, ... } (orderresponse.h:47-55)
    // 注意: C++ 中从 1 开始，没有 0
    // =====================================================================
    public static final int ORD_LIMIT = 1;
    public static final int ORD_MARKET = 2;
    public static final int ORD_WEIGHTAVG = 3;            // C++: WEIGHTAVG (aka STOP in user spec — actually WEIGHTAVG)
    public static final int ORD_CONDITIONAL_LIMIT_PRICE = 4; // C++: CONDITIONAL_LIMIT_PRICE (aka STOPLIMIT)
    public static final int ORD_BEST_PRICE = 5;
    public static final int ORD_TYPE_NUM = 6;              // C++: OrderTypeNum

    // =====================================================================
    // OrderDuration 枚举
    // C++: enum OrderDuration { DAY, IOC, FOK, COUNTER, FAK } (orderresponse.h:74-82)
    // =====================================================================
    public static final int DUR_DAY = 0;
    public static final int DUR_IOC = 1;
    public static final int DUR_FOK = 2;
    public static final int DUR_COUNTER = 3;  // C++: For forts (removed after auction end)
    public static final int DUR_FAK = 4;
    public static final int DUR_NUM = 5;       // C++: OrderDurationNum

    // =====================================================================
    // PriceType 枚举
    // C++: enum PriceType { PERCENTAGE=1, PERUNIT=2, YIELD=9 } (orderresponse.h:86-92)
    // =====================================================================
    public static final int PX_PERCENTAGE = 1;
    public static final int PX_PERUNIT = 2;
    public static final int PX_YIELD = 9;

    // =====================================================================
    // InstrumentType 枚举
    // C++: enum InstrumentType { STK, FUT, OPT, XXX } (orderresponse.h:96-103)
    // =====================================================================
    public static final int INSTR_STK = 0;
    public static final int INSTR_FUT = 1;
    public static final int INSTR_OPT = 2;
    public static final int INSTR_XXX = 3;
    public static final int INSTR_TYPE_NUM = 4; // C++: InstrumentTypeNum

    // =====================================================================
    // PositionDirection 枚举
    // C++: enum PositionDirection { OPEN=10, CLOSE, CLOSE_INTRADAY, POS_ERROR }
    //      (orderresponse.h:117-124)
    // =====================================================================
    public static final int POS_OPEN = 10;
    public static final int POS_CLOSE = 11;
    public static final int POS_CLOSE_INTRADAY = 12;  // C++: CLOSE_INTRADAY (平今)
    public static final int POS_ERROR = 13;

    // =====================================================================
    // SubResponseType 枚举
    // C++: enum SubResponseType (orderresponse.h:409-415)
    // =====================================================================
    public static final int SUB_NULL_RESPONSE_MIDDLE = 0;
    public static final int SUB_ORDER_REJECT_MIDDLE = 1;
    public static final int SUB_MODIFY_REJECT_MIDDLE = 2;
    public static final int SUB_CANCEL_ORDER_REJECT_MIDDLE = 3;

    // =====================================================================
    // OpenCloseType 枚举 (enum class : char)
    // C++: enum class OpenCloseType : char (orderresponse.h:417-423)
    // =====================================================================
    public static final byte OC_NULL_TYPE = 0;
    public static final byte OC_OPEN = 1;
    public static final byte OC_CLOSE = 2;
    public static final byte OC_CLOSE_TODAY = 3;

    // =====================================================================
    // TsExchangeID 枚举 (enum class : char)
    // C++: enum class TsExchangeID : char (orderresponse.h:425-434)
    // =====================================================================
    public static final byte TS_NULL_EXCHANGE = 0;
    public static final byte TS_SHFE = 1;
    public static final byte TS_INE = 2;
    public static final byte TS_CZCE = 3;
    public static final byte TS_DCE = 4;
    public static final byte TS_CFFEX = 5;
    public static final byte TS_GFEX = 6;

    // =====================================================================
    // ExchangeType 枚举 (enum, int values 0-12)
    // C++: enum ExchangeType { NSE_FO, NSE_CM, ... } (orderresponse.h:29-45)
    // =====================================================================
    public static final int ET_NSE_FO = 0;
    public static final int ET_NSE_CM = 1;
    public static final int ET_NSE_CDS = 2;
    public static final int ET_MICEX_FOND = 3;
    public static final int ET_MICEX_CURR = 4;
    public static final int ET_MCX = 5;
    public static final int ET_CME = 6;       // C++: eCME
    public static final int ET_LME = 7;       // C++: eLME
    public static final int ET_NYSE = 8;      // C++: eNYSE
    public static final int ET_ARCA = 9;      // C++: eARCA
    public static final int ET_NOT_NSE = 10;
    public static final int ET_REQUEST_MSG_EXCHG = 11;
    public static final int ET_RESPONSE_MSG_EXCHG = 12;
    public static final int EXCHANGE_TYPE_NUM = 13;  // C++: ExchangeTypeNum

    // =====================================================================
    // MD 交易所代码 (unsigned char, 行情 m_exchangeName 字段)
    // C++: hftbase/CommonUtils/include/marketupdateNew.h:24-98
    // =====================================================================
    public static final byte MD_EXCHANGE_UNKNOWN = 0;
    public static final byte MD_NSE_FO = 1;
    public static final byte MD_NSE_CM = 2;
    public static final byte MD_MICEX_FOND = 3;
    public static final byte MD_MICEX_CURR = 4;
    public static final byte MD_MICEX_FO = 5;
    public static final byte MD_FORTS_F = 6;         // Deprecated
    public static final byte MD_FORTS_O = 7;         // Deprecated
    public static final byte MD_MCX_FUTCOM = 8;
    public static final byte MD_NSE_CDS = 9;
    public static final byte MD_BSE = 10;
    public static final byte MD_MCX_SX = 11;
    public static final byte MD_REUTERS_GLOBAL = 12;
    public static final byte MD_LSE_IOB = 13;
    public static final byte MD_BRAZIL = 14;
    public static final byte MD_FLEXTRADE = 15;
    public static final byte MD_DGCX = 16;
    public static final byte MD_RDM = 17;
    public static final byte MD_PUMA_FXFU = 18;
    public static final byte MD_PUMA_FXEQOPT = 19;
    public static final byte MD_PUMA_IRFU = 20;
    public static final byte MD_PUMA_IROPT = 21;
    public static final byte MD_PUMA_CMFU = 22;
    public static final byte MD_PUMA_CMOPT = 23;
    public static final byte MD_PUMA_EQFU = 24;
    public static final byte MD_PUMA_SPFU = 25;
    public static final byte MD_PUMA_FXSPOT = 26;
    public static final byte MD_PUMA_EQIX = 27;
    public static final byte MD_PLAZA_FUT = 28;
    public static final byte MD_KOSPI_FNO = 29;
    public static final byte MD_KOSPI_EQUITY = 30;
    public static final byte MD_PUMA_EQ_MAT = 31;
    public static final byte MD_PUMA_EQ_OPT_MAT = 32;
    public static final byte MD_PUMA_EQ_OIL = 33;
    public static final byte MD_PUMA_EQ_OPT_OIL = 34;
    public static final byte MD_PUMA_EQ_FIN = 35;
    public static final byte MD_PUMA_EQ_OPT_FIN = 36;
    public static final byte MD_PUMA_EQ_GD = 37;
    public static final byte MD_PUMA_EQ_OPT_GD = 38;
    public static final byte MD_PUMA_EQ_OTC = 39;
    public static final byte MD_PUMA_EQ_OPT_IBOV = 40;
    public static final byte MD_PUMA_EQ = 41;
    public static final byte MD_PUMA_FU = 42;
    public static final byte MD_NYSE = 43;
    public static final byte MD_ARCA = 44;
    public static final byte MD_PACF = 45;
    public static final byte MD_NQEX = 46;
    public static final byte MD_NQNM = 47;
    public static final byte MD_NASDAQ = 48;
    public static final byte MD_IB_GLOBAL = 49;
    public static final byte MD_CHINA = 50;
    public static final byte MD_BATS = 51;
    public static final byte MD_INET = 52;
    public static final byte MD_CBSX = 53;
    public static final byte MD_CME = 54;
    public static final byte MD_LME = 55;
    public static final byte MD_FASTMD = 56;
    // 中国期货交易所代码
    public static final byte CHINA_SHFE = 57;   // 上海期货交易所
    public static final byte CHINA_CFFEX = 58;  // 中国金融期货交易所
    public static final byte CHINA_ZCE = 59;    // 郑州商品交易所
    public static final byte CHINA_DCE = 60;    // 大连商品交易所
    public static final byte CHINA_GFEX = 61;   // 广州期货交易所
    public static final byte MD_SGX = 62;
    public static final byte MD_OSE = 63;
    public static final byte MD_MCASTNSE_FO = 64;
    public static final byte MD_MCASTNSE_CDS = 65;
    public static final byte MD_MCASTNSE_CM = 66;
    public static final byte MD_ESPEED_TB = 67;
    public static final byte MD_ICEUS_BRN = 68;
    public static final byte MD_KOSPI_CUR = 69;
    public static final byte MD_CHINA_SH = 70;  // 上海证券交易所
    public static final byte MD_CHINA_SZ = 71;  // 深圳证券交易所
    /** C++: const size_t MAX_EXCHANGE_COUNT = 72; (marketupdateNew.h:98) */
    public static final int MAX_EXCHANGE_COUNT = 72;

    // 特殊消息类型标记
    /** C++: const unsigned char REQUEST_MSG = 100; (marketupdateNew.h:100) */
    public static final byte MD_REQUEST_MSG = 100;
    /** C++: const unsigned char RESPONSE_MSG = 101; (marketupdateNew.h:101) */
    public static final byte MD_RESPONSE_MSG = 101;

    // =====================================================================
    // MD Feed Type (m_feedType)
    // C++: marketupdateNew.h:103-106
    // =====================================================================
    public static final byte FEED_TBT = (byte) 'X';
    public static final byte FEED_SNAPSHOT = (byte) 'W';
    public static final byte FEED_AUCTION = (byte) 'A';

    // =====================================================================
    // MD Side (m_side)
    // C++: marketupdateNew.h:108-112
    // =====================================================================
    public static final byte SIDE_BUY = (byte) 'B';
    public static final byte SIDE_SELL = (byte) 'S';
    public static final byte SIDE_SHORT_SELL = (byte) 'A';
    public static final byte SIDE_NONE = (byte) 'N';

    // =====================================================================
    // MD Update Type (m_updateType)
    // C++: marketupdateNew.h:114-129
    // =====================================================================
    public static final byte MDUPDTYPE_ADD = (byte) 'A';
    public static final byte MDUPDTYPE_MODIFY = (byte) 'M';
    public static final byte MDUPDTYPE_DELETE = (byte) 'D';
    public static final byte MDUPDTYPE_INTERNAL_DELETE = (byte) 'V';
    public static final byte MDUPDTYPE_ORDER_ENTRY = (byte) 'O';
    public static final byte MDUPDTYPE_TRADE = (byte) 'X';
    public static final byte MDUPDTYPE_TRADE_INFO = (byte) 'I';
    public static final byte MDUPDTYPE_TRADE_IMPLIED = (byte) 'T';
    public static final byte MDUPDTYPE_NONE = (byte) 'N';
    public static final byte MDUPDTYPE_IMBALANCE = (byte) 'B';
    public static final byte MDUPDTYPE_MOD_ERR = (byte) 'J';
    public static final byte MDUPDTYPE_BOOKRESET = (byte) 'R';
    public static final byte MDUPDTYPE_DELETEFROM = (byte) 'e';
    public static final byte MDUPDTYPE_DELETETHRU = (byte) 'f';
    public static final byte MDUPDTYPE_OVERLAY = (byte) 'g';

    // =====================================================================
    // MD Update Level
    // C++: marketupdateNew.h:132
    // =====================================================================
    public static final byte MDUPDLEVEL_NONE = -1;

    // =====================================================================
    // ResponseType 字符串描述（用于日志输出）
    // C++: const std::string ResponseTypeStr[] (orderresponse.h:403-407)
    // =====================================================================
    private static final String[] RESPONSE_TYPE_STR = {
        "NEW_ORDER_CONFIRM", "NEW_ORDER_FREEZE", "MODIFY_ORDER_CONFIRM",
        "CANCEL_ORDER_CONFIRM", "TRADE_CONFIRM", "ORDER_ERROR",
        "MODIFY_ORDER_REJECT", "CANCEL_ORDER_REJECT",
        "ORS_REJECT", "RMS_REJECT", "SIM_REJECT", "BUSINESS_REJECT",
        "MODIFY_ORDER_PENDING", "CANCEL_ORDER_PENDING",
        "ORDERS_PER_DAY_LIMIT_REJECT", "ORDERS_PER_DAY_LIMIT_WARNING",
        "ORDER_EXPIRED", "STOP_LOSS_WARNING", "NULL_RESPONSE"
    };

    /**
     * 获取 ResponseType 的字符串描述。
     * C++: ResponseTypeStr[type] (orderresponse.h:403)
     */
    public static String responseTypeStr(int type) {
        if (type >= 0 && type < RESPONSE_TYPE_STR.length) {
            return RESPONSE_TYPE_STR[type];
        }
        return "UNKNOWN";
    }

    // =====================================================================
    // RequestType 字符串描述
    // C++: const std::string RequestTypeStr[] (orderresponse.h:72)
    // =====================================================================
    private static final String[] REQUEST_TYPE_STR = {
        "NEWORDER", "MODIFYORDER", "CANCELORDER", "ORDERSTATUS",
        "SESSIONMSG", "HEARTBEAT", "OPTEXEC", "OPTEXEC_CANCEL"
    };

    /**
     * 获取 RequestType 的字符串描述。
     * C++: RequestTypeStr[type] (orderresponse.h:72)
     */
    public static String requestTypeStr(int type) {
        if (type >= 0 && type < REQUEST_TYPE_STR.length) {
            return REQUEST_TYPE_STR[type];
        }
        return "UNKNOWN";
    }

    // =====================================================================
    // OrderType 字符串描述
    // C++: const std::string OrderTypeStr[] (orderresponse.h:57)
    // =====================================================================
    private static final String[] ORDER_TYPE_STR = {
        "", "LIMIT", "MARKET", "WEIGHTAVG", "CONDLIMITPRICE", "BESTPRICE"
    };

    /**
     * 获取 OrderType 的字符串描述。
     * C++: OrderTypeStr[type] (orderresponse.h:57)
     */
    public static String orderTypeStr(int type) {
        if (type >= 0 && type < ORDER_TYPE_STR.length) {
            return ORDER_TYPE_STR[type];
        }
        return "UNKNOWN";
    }

    // =====================================================================
    // OrderDuration 字符串描述
    // C++: const std::string OrderDurationStr[] (orderresponse.h:84)
    // =====================================================================
    private static final String[] ORDER_DURATION_STR = {
        "DAY", "IOC", "FOK", "COUNTER", "FAK"
    };

    /**
     * 获取 OrderDuration 的字符串描述。
     * C++: OrderDurationStr[duration] (orderresponse.h:84)
     */
    public static String orderDurationStr(int duration) {
        if (duration >= 0 && duration < ORDER_DURATION_STR.length) {
            return ORDER_DURATION_STR[duration];
        }
        return "UNKNOWN";
    }

    // =====================================================================
    // 交易所名称 → ID 映射
    // C++: hftbase/CommonUtils/include/marketupdateNew.h:881-980
    // =====================================================================

    /**
     * 交易所名称 → 交易所 ID 映射。
     * <p>
     * 迁移自: hftbase/CommonUtils/include/marketupdateNew.h:881-980
     * C++: static char getExchangeIdFromName(const std::string &amp;token)
     * <p>
     * 注意: 此方法在 C++ 中属于 MarketUpdateNew，不属于 Connector。
     */
    public static int getExchangeIdFromName(String name) {
        return switch (name) {
            case "EXCHANGE_UNKNOWN" -> MD_EXCHANGE_UNKNOWN;
            case "NSE_FO"          -> MD_NSE_FO;
            case "NSE_CM"          -> MD_NSE_CM;
            case "MICEX_FOND"      -> MD_MICEX_FOND;
            case "MICEX_CURR"      -> MD_MICEX_CURR;
            case "MICEX_FO"        -> MD_MICEX_FO;
            case "MCX_FUTCOM"      -> MD_MCX_FUTCOM;
            case "NSE_CDS"         -> MD_NSE_CDS;
            case "BSE"             -> MD_BSE;
            case "NYSE"            -> MD_NYSE;
            case "ARCA"            -> MD_ARCA;
            case "CME"             -> MD_CME;
            case "LME"             -> MD_LME;
            case "SGX"             -> MD_SGX;
            case "CHINA_SHFE"      -> CHINA_SHFE;
            case "CHINA_CFFEX"     -> CHINA_CFFEX;
            case "CHINA_ZCE"       -> CHINA_ZCE;
            case "CHINA_DCE"       -> CHINA_DCE;
            case "CHINA_GFEX"      -> CHINA_GFEX;
            case "CHINA"           -> MD_CHINA;
            case "CHINA_SH"        -> MD_CHINA_SH;
            case "CHINA_SZ"        -> MD_CHINA_SZ;
            default -> {
                java.util.logging.Logger.getLogger(Constants.class.getName())
                    .warning("[getExchangeIdFromName] unknown exchange: " + name + ", returning 0");
                yield 0;
            }
        };
    }

    // =====================================================================
    // PositionDirection 字符串描述
    // C++: const std::string PositionDirectionStr[] (orderresponse.h:126-127)
    // =====================================================================
    private static final String[] POSITION_DIRECTION_STR = {
        "", "", "", "", "", "", "", "", "", "",
        "OPEN", "CLOSE", "CLOSE_INTRADAY", "POS_ERROR"
    };

    /**
     * 获取 PositionDirection 的字符串描述。
     * C++: PositionDirectionStr[dir] (orderresponse.h:126)
     */
    public static String positionDirectionStr(int dir) {
        if (dir >= 0 && dir < POSITION_DIRECTION_STR.length) {
            return POSITION_DIRECTION_STR[dir];
        }
        return "UNKNOWN";
    }
}
