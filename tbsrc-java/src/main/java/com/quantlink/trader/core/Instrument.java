package com.quantlink.trader.core;

import com.quantlink.trader.shm.Types;

import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.util.List;

/**
 * 20 档订单簿行情数据模型。
 * 迁移自: tbsrc/common/include/Instrument.h
 *
 * [C++差异] C++ Instrument 包含交易所特定的 FillOrderBook 方法（CME/ICE/KRX/China 等），
 * Java 版本仅实现通用 FillOrderBook（从 MarketUpdateNew 读取 20 档），
 * 交易所特定逻辑通过 C++ 网关层（md_shm_feeder）在写入 SHM 前已完成规范化。
 * Ref: tbsrc/common/include/Instrument.h (456 lines)
 */
public class Instrument {

    // ---- 合约属性 ----
    // 迁移自: Instrument.h — m_origbaseName, m_symbol, m_exchange, m_symbolID
    public String origBaseName = "";
    public String symbol = "";
    public String exchange = "";
    public int symbolID;

    // 迁移自: Instrument.h — m_tickSize, m_lotSize, m_priceFactor, m_contractFactor, m_priceMultiplier
    public double tickSize = 1.0;
    public double lotSize = 1.0;
    public double priceFactor = 1.0;
    public double contractFactor = 1.0;
    public double priceMultiplier = 1.0;
    public double multipleSize = 1.0;
    public double divisorSize = 1.0;

    // 迁移自: Instrument.h — m_sendInLots, m_perContract, m_perYield
    public boolean sendInLots = false;
    public boolean perContract = false;
    public boolean perYield = false;
    public int level = 0; // 迁移自: m_level — 当前关注档位

    // ---- 20 档订单簿 ----
    // 迁移自: Instrument.h — bidPx[20], askPx[20], bidQty[20], askQty[20]
    public final double[] bidPx = new double[20];
    public final double[] askPx = new double[20];
    public final double[] bidQty = new double[20];
    public final double[] askQty = new double[20];
    public final double[] bidOrderCount = new double[20];
    public final double[] askOrderCount = new double[20];
    public int bookDepth = 20;

    // ---- StratBook 档位 ----
    // 迁移自: Instrument.h — bidPxStrat[20], askPxStrat[20], bidQtyStrat[20], askQtyStrat[20]
    // C++ 用于 UseStratBook 模式下的策略内部订单簿
    public final double[] bidPxStrat = new double[20];
    public final double[] askPxStrat = new double[20];
    public final double[] bidQtyStrat = new double[20];
    public final double[] askQtyStrat = new double[20];
    public final double[] bidOrderCountStrat = new double[20];
    public final double[] askOrderCountStrat = new double[20];

    // ---- 有效档位数 ----
    // 迁移自: Instrument.h — m_validBids, m_validAsks
    public int validBids = 20;
    public int validAsks = 20;

    // ---- 成交数据 ----
    // 迁移自: Instrument.h — lastTradePx, lastTradeqty, totalTradedQty 等
    public double lastTradePx;
    public double lastTradeQty;
    public double totalTradedQty;
    public double totalTradedValue;
    public double prevTotalTradedQty;
    public double prevTotalTradedValue;
    public double initTotalTradedQty;
    public double initTotalTradedValue;
    public double avgPrice;
    public double totalL1Event;

    // ---- 时间戳 ----
    // 迁移自: Instrument.h — lastLocalTime, lastExchTime
    public long lastLocalTime;
    public long lastExchTime;
    public long lastRptSeqNum;

    // ---- 日期 ----
    public String currDate = "";

    // ---- Bond 相关字段 ----
    // 迁移自: Instrument.h — m_cDays, m_tDays, m_yield
    // C++: int32_t m_cDays, m_tDays; double m_yield;
    public int cDays;       // 剩余交易日（用于 BondPrice 计算）
    public int tDays;       // 总交易日
    public double yield;    // 收益率

    // ---- 合约全名 ----
    // 迁移自: Instrument.h — m_instrument (char[48])
    // C++: 与 m_symbol 不同，m_instrument 是完整的合约标识（含交易所前缀等）
    public String instrument = "";

    // ---- CME/期权相关字段 ----
    // 迁移自: Instrument.h — m_token, m_expiryDate, m_strike, m_callPutFlag, m_securitygroup, m_product
    public int token;                    // C++: m_token — 合约 token (NSE 等使用)
    public int expiryDate;               // C++: m_expiryDate — 到期日 (YYYYMMDD)
    public double strike;                // C++: m_strike — 行权价
    public double strikeSpread;          // C++: m_strikeSpread — 行权价差
    public char callPutFlag;             // C++: m_callPutFlag — 'C'=CALL, 'P'=PUT, 其他=NILL
    public String securityGroup = "";    // C++: m_securitygroup[100] — CME 安全组
    public int productType;              // C++: m_product (ProductType enum) — 0=Future, 1=Option 等

    // ---- 行情状态标志 ----
    // 迁移自: Instrument.h — m_crossUpdate, m_firstTrade, m_ignoreImpliedTrades, m_bUseTradeInfo
    public boolean crossUpdate;           // C++: m_crossUpdate — 是否为 cross book 更新
    public boolean firstTrade;            // C++: m_firstTrade — 是否已收到首笔成交
    public boolean ignoreImpliedTrades;   // C++: m_ignoreImpliedTrades — 是否过滤隐含成交
    public boolean bUseTradeInfo;         // C++: m_bUseTradeInfo — 是否使用 TRADE_INFO 类型更新
    public boolean useSmartBook;          // C++: m_useSmartBook — SmartMD 模式
    public boolean updateIndicators;      // C++: m_updateIndicators — 是否更新指标
    public boolean smartTrade;            // C++: m_smartTrade — SmartMD 交易标志

    // ---- 指标列表 ----
    // 迁移自: Instrument.h — IndList* m_indList
    // 每个合约持有自己的指标列表引用，由 CommonClient::Update() 遍历
    public List<IndElem> indList;

    // ---- 价格缓存与按需计算标志 ----
    // 迁移自: Instrument.h:63-77 — 价格缓存变量和 is_needed_ 标志
    // C++: bool is_needed_MSWMIDPrice_, is_needed_MSWPrice_, is_needed_MOWPrice_,
    //      is_needed_MIDPrice_, is_needed_SINPrice_, is_needed_WGTPrice_, is_needed_LTPPrice_
    private boolean isNeededMSWMIDPrice;
    private boolean isNeededMSWPrice;
    private boolean isNeededMOWPrice;
    private boolean isNeededMIDPrice;
    private boolean isNeededSINPrice;
    private boolean isNeededWGTPrice;
    private boolean isNeededLTPPrice;

    // C++: double MSWMIDPrice_, MSWPrice_, MOWPrice_, MIDPrice_, SINPrice_, WGTPrice_, LTPPrice_, mkt_tilt
    private double cachedMSWMIDPrice;
    private double cachedMSWPrice;
    private double cachedMOWPrice;
    private double cachedMIDPrice;
    private double cachedSINPrice;
    private double cachedWGTPrice;
    private double cachedLTPPrice;
    private double mktTilt;

    /**
     * 从 MarketUpdateNew MemorySegment 填充 20 档订单簿。
     * 迁移自: Instrument::FillOrderBook(MarketUpdateNew*)
     *
     * @param mdUpdate MarketUpdateNew MemorySegment (816 bytes)
     */
    public void fillOrderBook(MemorySegment mdUpdate) {
        long mdDataBase = Types.MU_DATA_OFFSET; // 96

        // C++: m_updateIndicators = true  (Instrument.cpp:2177)
        updateIndicators = true;

        // C++: CopyOrderBook(update) — 遍历 bidUpdates[20] / askUpdates[20]
        // Ref: Instrument.cpp:2032-2091
        // C++: m_validBids = update->m_validBids
        int vBids = ((byte) Types.MDD_VALID_BIDS_VH.get(mdUpdate, mdDataBase)) & 0xFF;
        int vAsks = ((byte) Types.MDD_VALID_ASKS_VH.get(mdUpdate, mdDataBase)) & 0xFF;

        // C++: for (int i = 0; i < update->m_validBids; i++) { bidPx[i]=...; bidQty[i]=...; bidOrderCount[i]=...; }
        for (int i = 0; i < vBids && i < 20; i++) {
            bidPx[i] = (double) Types.MDD_BID_PRICE_VH.get(mdUpdate, mdDataBase, (long) i);
            bidQty[i] = (int) Types.MDD_BID_QUANTITY_VH.get(mdUpdate, mdDataBase, (long) i);
            bidOrderCount[i] = (int) Types.MDD_BID_ORDER_COUNT_VH.get(mdUpdate, mdDataBase, (long) i);
        }
        // C++: if (update->m_validBids < m_level + 1) { 清零后续档位 }
        for (int i = vBids; i < level + 1 && i < 20; i++) {
            bidPx[i] = 0;
            bidQty[i] = 0;
            bidOrderCount[i] = 0;
        }
        // C++: m_validBids = update->m_validBids
        validBids = vBids;

        // C++: for (int i = 0; i < update->m_validAsks; i++) { askPx[i]=...; askQty[i]=...; askOrderCount[i]=...; }
        for (int i = 0; i < vAsks && i < 20; i++) {
            askPx[i] = (double) Types.MDD_ASK_PRICE_VH.get(mdUpdate, mdDataBase, (long) i);
            askQty[i] = (int) Types.MDD_ASK_QUANTITY_VH.get(mdUpdate, mdDataBase, (long) i);
            askOrderCount[i] = (int) Types.MDD_ASK_ORDER_COUNT_VH.get(mdUpdate, mdDataBase, (long) i);
        }
        // C++: if (update->m_validAsks < m_level + 1) { 清零后续档位 }
        for (int i = vAsks; i < level + 1 && i < 20; i++) {
            askPx[i] = 0;
            askQty[i] = 0;
            askOrderCount[i] = 0;
        }
        // C++: m_validAsks = update->m_validAsks
        validAsks = vAsks;

        // C++: lastTradePx = update->m_newPrice; lastTradeqty = update->m_newQuant;
        // Ref: Instrument.cpp:2199-2202
        // [C++差异] C++ 仅在 TRADE/TRADE_IMPLIED/TRADE_INFO 类型时更新 lastTradePx/lastTradeqty,
        // Java 的 md_shm_feeder 使用 MDUPDTYPE_ORDER_ENTRY，lastTradedPrice 由 feeder 填充
        lastTradePx = (double) Types.MDD_LAST_TRADED_PRICE_VH.get(mdUpdate, mdDataBase);
        // C++: lastTradeqty = update->m_newQuant
        lastTradeQty = (int) Types.MDD_NEW_QUANT_VH.get(mdUpdate, mdDataBase);

        // C++: CalculatePrices()  (Instrument.cpp:2232)
        calculatePrices();
    }

    /**
     * 读取 MarketUpdateNew 中的 symbol 字段。
     * 迁移自: update->m_symbol (MDHeaderPart offset 40, char[48])
     */
    public static String readSymbol(MemorySegment mdUpdate) {
        byte[] buf = new byte[48];
        MemorySegment.copy(mdUpdate, Types.MDH_SYMBOL_OFFSET, MemorySegment.ofArray(buf), 0, 48);
        return new String(buf, StandardCharsets.US_ASCII).trim().replace("\0", "");
    }

    /**
     * 读取 MarketUpdateNew 中的 symbolID 字段。
     * 迁移自: update->m_symbolID (MDHeaderPart)
     */
    public static int readSymbolID(MemorySegment mdUpdate) {
        // MDH_SYMBOL_ID_VH is JAVA_SHORT (uint16_t m_symbolID)
        return Short.toUnsignedInt((short) Types.MDH_SYMBOL_ID_VH.get(mdUpdate, 0L));
    }

    // =======================================================================
    //  价格类型订阅 / 获取 — 按需计算架构
    // =======================================================================
    // 迁移自: Instrument.h:169-232 — SubscribeTBPriceType, GetTBPriceType, GetTBStratPriceType

    /**
     * 订阅价格类型 — 标记该价格需要在每次行情更新时计算。
     * 迁移自: Instrument::SubscribeTBPriceType(TBPriceType) — Instrument.h:169-196
     *
     * @param priceType 价格类型常量（使用 Dependant.MKTW_PX2 等）
     */
    public void subscribeTBPriceType(int priceType) {
        // C++: switch (t_price) { case MKTMID_PX2: is_needed_MSWMIDPrice_ = true; break; ... }
        switch (priceType) {
            case 2 -> isNeededMSWMIDPrice = true;  // MKTMID_PX2
            case 0 -> isNeededMSWPrice = true;     // MKTW_PX2
            case 1 -> isNeededMIDPrice = true;     // MID_PX2
            case 3 -> isNeededWGTPrice = true;     // WGT_PX
            case 4 -> isNeededLTPPrice = true;     // LTP_PX
        }
    }

    /**
     * 计算所有已订阅的价格。
     * 迁移自: Instrument::CalculatePrices() — Instrument.h:143-159
     */
    public void calculatePrices() {
        // C++: if (is_needed_MSWMIDPrice_) calculate_MSWMIDPrice();
        if (isNeededMSWMIDPrice) cachedMSWMIDPrice = calculateMSWMIDPrice();
        // C++: if (is_needed_MSWPrice_) calculate_MSWPrice();
        if (isNeededMSWPrice) cachedMSWPrice = calculateMSWPrice();
        // C++: if (is_needed_MOWPrice_) calculate_MOWPrice();
        if (isNeededMOWPrice) cachedMOWPrice = calculateMOWPrice();
        // C++: if (is_needed_MIDPrice_ || is_needed_SINPrice_) calculate_MIDPrice();
        if (isNeededMIDPrice || isNeededSINPrice) cachedMIDPrice = calculateMIDPrice();
        // C++: if (is_needed_SINPrice_) calculate_SINPrice();
        if (isNeededSINPrice) cachedSINPrice = calculateSINPrice();
        // C++: if (is_needed_WGTPrice_) calculate_WGTPrice();
        if (isNeededWGTPrice) cachedWGTPrice = calculateWGTPrice();
        // C++: if (is_needed_LTPPrice_) calculate_LTPPrice();
        if (isNeededLTPPrice) cachedLTPPrice = calculateLTPPrice();
    }

    /**
     * 计算策略订单簿的价格。
     * 迁移自: Instrument::CalculateStratPrices() — Instrument.h:161-167
     */
    public void calculateStratPrices() {
        // C++: if (is_needed_MSWPrice_) calculate_StratMSWPrice();
        if (isNeededMSWPrice) cachedMSWPrice = calculateStratMSWPrice();
        // C++: if (is_needed_MIDPrice_ || is_needed_SINPrice_) calculate_StratMIDPrice();
        if (isNeededMIDPrice || isNeededSINPrice) cachedMIDPrice = calculateStratMIDPrice();
    }

    /**
     * 获取指定价格类型的值（从普通 book 计算）。
     * 迁移自: Instrument::GetTBPriceType(TBPriceType) — Instrument.h:204-232
     *
     * @param priceType 价格类型常量
     * @return 计算后的价格
     */
    public double getTBPriceType(int priceType) {
        // C++: CalculatePrices(); — 已在 FillOrderBook 后由 CommonClient 调用
        // 此处读取缓存值
        calculatePrices();
        return switch (priceType) {
            case 2 -> cachedMSWMIDPrice;    // MKTMID_PX2
            case 0 -> cachedMSWPrice;       // MKTW_PX2
            case 1 -> cachedMIDPrice;       // MID_PX2
            case 3 -> cachedWGTPrice;       // WGT_PX
            case 4 -> cachedLTPPrice;       // LTP_PX
            default -> 0.0;
        };
    }

    /**
     * 获取指定价格类型的值（从策略 book 计算）。
     * 迁移自: Instrument::GetTBStratPriceType(TBPriceType) — Instrument.h:198-202
     * C++: CalculateStratPrices(); return GetTBPriceType(t_price);
     */
    public double getTBStratPriceType(int priceType) {
        calculateStratPrices();
        return switch (priceType) {
            case 2 -> cachedMSWMIDPrice;    // MKTMID_PX2
            case 0 -> cachedMSWPrice;       // MKTW_PX2
            case 1 -> cachedMIDPrice;       // MID_PX2
            case 3 -> cachedWGTPrice;       // WGT_PX
            case 4 -> cachedLTPPrice;       // LTP_PX
            default -> 0.0;
        };
    }

    // ---- 价格计算方法 ----
    // 迁移自: Instrument.h — calculate_MIDPrice(), calculate_MSWPrice(), calculate_LTPPrice()

    /**
     * 中间价。
     * C++: MIDPrice_ = (bidPx[0] + askPx[0]) / 2.0
     */
    public double calculateMIDPrice() {
        return (bidPx[0] + askPx[0]) / 2.0;
    }

    /**
     * 量加权中间价 (Market Size Weighted)。
     * C++: MSWPrice_ = (askQty[0]*bidPx[0] + askPx[0]*bidQty[0]) / (askQty[0]+bidQty[0])
     */
    public double calculateMSWPrice() {
        double totalQty = askQty[0] + bidQty[0];
        if (totalQty <= 0) return calculateMIDPrice();
        return (askQty[0] * bidPx[0] + askPx[0] * bidQty[0]) / totalQty;
    }

    /**
     * LTP 价格（约束在 bid-ask 范围内）。
     * C++: if (lastTradePx != 0) LTPPrice_ = clamp(lastTradePx, bidPx[0], askPx[0])
     */
    public double calculateLTPPrice() {
        if (lastTradePx == 0) return calculateMIDPrice();
        if (lastTradePx < bidPx[0]) return bidPx[0];
        if (lastTradePx > askPx[0]) return askPx[0];
        return lastTradePx;
    }

    /**
     * MSW-MID 混合价格。
     * C++: calculate_MSWMIDPrice — 当价差 > tickSize 时用 MID，否则用 MSW
     */
    public double calculateMSWMIDPrice() {
        double msw = calculateMSWPrice();
        if (askPx[0] - bidPx[0] > tickSize + 0.0001) {
            return calculateMIDPrice();
        }
        return msw;
    }

    /**
     * 量加权中间价（从策略订单簿）。
     * 迁移自: Instrument::calculate_StratMSWPrice() — Instrument.h:104-107
     * C++: MSWPrice_ = (askQtyStrat[0]*bidPxStrat[0] + askPxStrat[0]*bidQtyStrat[0]) / (askQtyStrat[0]+bidQtyStrat[0])
     */
    public double calculateStratMSWPrice() {
        double totalQty = askQtyStrat[0] + bidQtyStrat[0];
        if (totalQty <= 0) return calculateStratMIDPrice();
        return (askQtyStrat[0] * bidPxStrat[0] + askPxStrat[0] * bidQtyStrat[0]) / totalQty;
    }

    /**
     * 中间价（从策略订单簿）。
     * 迁移自: Instrument::calculate_StratMIDPrice() — Instrument.h:109-112
     * C++: MIDPrice_ = (bidPxStrat[0] + askPxStrat[0]) / 2.0
     */
    public double calculateStratMIDPrice() {
        return (bidPxStrat[0] + askPxStrat[0]) / 2.0;
    }

    /**
     * MOW 价格（订单数量加权）。
     * 迁移自: Instrument::calculate_MOWPrice() — Instrument.h:114-117
     * C++: MOWPrice_ = (askOrderCount[0]*bidPx[0] + askPx[0]*bidOrderCount[0]) / (askOrderCount[0]+bidOrderCount[0])
     */
    public double calculateMOWPrice() {
        double totalCount = askOrderCount[0] + bidOrderCount[0];
        if (totalCount <= 0) return calculateMIDPrice();
        return (askOrderCount[0] * bidPx[0] + askPx[0] * bidOrderCount[0]) / totalCount;
    }

    /**
     * SIN 价格（正弦倾斜模型）。
     * 迁移自: Instrument::calculate_SINPrice() — Instrument.h:119-123
     * C++: mkt_tilt = (bidQty[0]-askQty[0])/(bidQty[0]+askQty[0]);
     *      SINPrice_ = MIDPrice_ + ((askPx[0]-bidPx[0])*(mkt_tilt^3))/2.0
     */
    public double calculateSINPrice() {
        double totalQty = bidQty[0] + askQty[0];
        if (totalQty <= 0) return cachedMIDPrice;
        mktTilt = (bidQty[0] - askQty[0]) / totalQty;
        return cachedMIDPrice + ((askPx[0] - bidPx[0]) * (mktTilt * mktTilt * mktTilt)) / 2.0;
    }

    /**
     * WGT 价格（3 档加权）。
     * 迁移自: Instrument::calculate_WGTPrice() — Instrument.h:125-140
     */
    public double calculateWGTPrice() {
        double bidSum = 0, bidQuant = 0, askSum = 0, askQuant = 0;
        for (int i = 0; i < 3; i++) {
            bidSum += bidPx[i] * bidQty[i];
            bidQuant += bidQty[i];
            askSum += askPx[i] * askQty[i];
            askQuant += askQty[i];
        }
        if (bidQuant <= 0 || askQuant <= 0) return calculateMIDPrice();
        return (((bidSum / bidQuant) * askQuant) + ((askSum / askQuant) * bidQuant)) / (bidQuant + askQuant);
    }

    /**
     * 重置订单簿和交易数据。
     * 迁移自: Instrument::Reset()
     */
    public void reset() {
        java.util.Arrays.fill(bidPx, 0.0);
        java.util.Arrays.fill(askPx, 0.0);
        java.util.Arrays.fill(bidQty, 0.0);
        java.util.Arrays.fill(askQty, 0.0);
        java.util.Arrays.fill(bidOrderCount, 0.0);
        java.util.Arrays.fill(askOrderCount, 0.0);
        lastTradePx = 0;
        lastTradeQty = 0;
        totalTradedQty = 0;
        totalTradedValue = 0;
        totalL1Event = 0;
        lastLocalTime = 0;
        lastExchTime = 0;
    }

    // =======================================================================
    //  价格工具方法
    // =======================================================================

    /**
     * 零息债券现值折扣因子。
     * 迁移自: TradeBotUtils.cpp:3744-3747
     * C++: double BondPrice(double rate, int32_t days) { return 1 / (pow(rate/100 + 1, (double)days/252)); }
     *
     * @param rate 年化利率（百分比形式，如 3.5 表示 3.5%）
     * @param days 剩余交易日
     * @return 折扣因子
     */
    public static double bondPrice(double rate, int days) {
        // C++: return 1 / (pow(rate / 100 + 1, (double)days / 252));
        return 1.0 / Math.pow(rate / 100.0 + 1.0, (double) days / 252.0);
    }

    /**
     * 年化利率转复合收益。
     * 迁移自: TradeBotUtils.cpp:3739-3742
     * C++: double TransPrice(double rate, int32_t days) { return (pow(rate/100 + 1, (double)days/252) - 1); }
     *
     * @param rate 年化利率（百分比形式）
     * @param days 交易日数
     * @return 复合收益率
     */
    public static double transPrice(double rate, int days) {
        // C++: return (pow(rate / 100 + 1, (double)days / 252) - 1);
        return Math.pow(rate / 100.0 + 1.0, (double) days / 252.0) - 1.0;
    }

    @Override
    public String toString() {
        return String.format("Instrument[%s, bid=%.2f×%.0f, ask=%.2f×%.0f, ltp=%.2f]",
                origBaseName, bidPx[0], bidQty[0], askPx[0], askQty[0], lastTradePx);
    }
}
