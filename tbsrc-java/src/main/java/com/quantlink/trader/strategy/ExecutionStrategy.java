package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.lang.foreign.MemorySegment;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.util.*;
import java.util.logging.Logger;

/**
 * 执行策略基类。
 * 迁移自: tbsrc/Strategies/include/ExecutionStrategy.h (line 22-309)
 *         tbsrc/Strategies/ExecutionStrategy.cpp (2506 lines)
 *
 * C++ class ExecutionStrategy — 核心方法:
 *   构造函数(CommonClient*, SimConfig*)
 *   Reset(), SetThresholds(), SetLinearThresholds()
 *   ORSCallBack(ResponseMsg*), MDCallBack(MarketUpdateNew*)
 *   SendNewOrder(), SendModifyOrder(), SendCancelOrder()
 *   ProcessTrade(), CalculatePNL(), CheckSquareoff(), HandleSquareoff()
 */
public abstract class ExecutionStrategy {

    private static final Logger log = Logger.getLogger(ExecutionStrategy.class.getName());

    // C++: #define MAX_POS_SIZE 10000000
    // C++: #define MAX_ORDERS   5000000000
    // C++: #define REJECT_LIMIT 200
    protected static final int MAX_POS_SIZE = 10_000_000;
    protected static final long MAX_ORDERS = 5_000_000_000L;
    protected static final int REJECT_LIMIT = 200;

    // ---- 引用 ----
    // 迁移自: ExecutionStrategy.h:248-255
    public CommonClient client;          // C++: m_client
    public Instrument instru;            // C++: m_instru
    public Instrument instruSec;         // C++: m_instru_sec
    public Instrument instruThird;       // C++: m_instru_third
    public ThresholdSet thold;           // C++: m_thold
    public SimConfig simConfig;          // C++: m_simConfig
    public ConfigParams configParams;    // C++: m_configParams
    public int strategyID;               // C++: m_strategyID

    // ---- 位置字段 ----
    // 迁移自: ExecutionStrategy.h:111-114
    public int netpos;                   // C++: m_netpos
    public int netposPass;               // C++: m_netpos_pass
    public int netposPassYtd;            // C++: m_netpos_pass_ytd
    public int netposAgg;                // C++: m_netpos_agg

    // ---- PNL 字段 ----
    // 迁移自: ExecutionStrategy.h:160-165
    public double realisedPNL;           // C++: m_realisedPNL
    public double unrealisedPNL;         // C++: m_unrealisedPNL
    public double netPNL;                // C++: m_netPNL
    public double grossPNL;              // C++: m_grossPNL
    public double maxPNL;                // C++: m_maxPNL
    public double drawdown;              // C++: m_drawdown

    // ---- 量/值字段 ----
    // 迁移自: ExecutionStrategy.h:138-158
    public double buyQty;
    public double sellQty;
    public double buyTotalQty;
    public double sellTotalQty;
    public double buyOpenQty;
    public double sellOpenQty;
    public double buyTotalValue;
    public double sellTotalValue;
    public double buyValue;
    public double sellValue;
    public double transTotalValue;
    public double transValue;
    public double buyAvgPrice;
    public double sellAvgPrice;
    public double buyPrice;
    public double sellPrice;

    // ---- 订单统计 ----
    // 迁移自: ExecutionStrategy.h:123-137
    public int buyOpenOrders;
    public int sellOpenOrders;
    public int tradeCount;
    public int orderCount;
    public int cancelCount;
    public int confirmCount;
    public int rejectCount;
    public int improveCount;
    public int crossCount;
    public int cancelconfirmCount;

    // ---- 阈值字段 ----
    // 迁移自: ExecutionStrategy.h:186-199
    public double tholdBidPlace;
    public double tholdBidRemove;
    public double tholdAskPlace;
    public double tholdAskRemove;
    public int tholdMaxPos;
    public int tholdBeginPos;
    public int tholdInc;
    public int tholdSize;
    public int smsRatio;
    // 控制买卖方向的单笔报单量和最大仓位
    // 迁移自: ExecutionStrategy.h:196-199
    public int tholdBidSize;
    public int tholdBidMaxPos;
    public int tholdAskSize;
    public int tholdAskMaxPos;

    // ---- 状态标志 ----
    // 迁移自: ExecutionStrategy.h:90-101
    public boolean onMaxPx;
    public boolean onNewsFlat;
    public boolean onStopLoss;
    public boolean onExit;
    public boolean onCancel;
    public boolean onTimeSqOff;
    public boolean onFlat;
    public boolean aggFlat;
    public boolean active;
    public boolean callSquareOff;

    // ---- 时间戳 ----
    public long exchTS;
    public long localTS;
    public long maxOrderCount;
    public long maxPosSize;
    public double maxTradedQty;
    public long endTime;
    public long endTimeEpoch;
    public long endTimeAgg;
    public long endTimeAggEpoch;
    public long lastFlatTS;
    public long lastTradeTime;
    public long lastOrderTime;

    // ---- 交易费 ----
    public double buyExchTx;
    public double sellExchTx;
    public double buyExchContractTx;
    public double sellExchContractTx;

    // ---- 其他 ----
    public double targetPrice;
    public double currPrice;
    public double ltp;
    public double avgQty;
    public boolean lastTradeSide;
    public boolean lastTrade;
    public double lastTradePx;
    public double currAvgPrice;
    public int level;

    // ---- 追单 ----
    // 迁移自: ExecutionStrategy.h:289-294
    public double buyAggCount;
    public double sellAggCount;
    public double buyAggOrder;
    public double sellAggOrder;
    public long lastAggTime;
    public byte lastAggSide;

    // ---- 期权策略标志 ----
    // 迁移自: ExecutionStrategy.h — m_optionStrategy, m_useNewsHandler
    public boolean optionStrategy;        // C++: m_optionStrategy — 是否为期权策略
    public boolean useNewsHandler;        // C++: m_useNewsHandler — 是否使用新闻处理器

    // ---- Delta 风控字段 ----
    // 迁移自: ExecutionStrategy.h — 用于 CheckSquareoff 中的 delta 滚动平均
    public double currAvgDelta;           // C++: m_currAvgDelta — 当前窗口平均 delta
    public double tmpAvgDelta;            // C++: tmpAvgDelta — 累积中的平均 delta
    public int deltaCount;                // C++: m_deltaCount — delta 采样计数
    public long lastDeltaTS;              // C++: m_lastDeltaTS — 上次 delta 更新时间戳

    // ---- 价格限制风控字段 ----
    // 迁移自: ExecutionStrategy.h — 用于 CheckSquareoff 中的 USE_PRICE_LIMIT
    public double tmpAvgTargetPrice;      // C++: tmpAvgTargetPrice — 累积中的平均目标价
    public int priceCount;                // C++: m_priceCount — 价格采样计数
    public long lastPxTS;                 // C++: m_lastPxTS — 上次价格更新时间戳

    // ---- PNL 变化检测 ----
    // 迁移自: ExecutionStrategy.h — m_bestbid_lastpnl, m_bestask_lastpnl
    public double bestbidLastpnl;         // C++: m_bestbid_lastpnl
    public double bestaskLastpnl;         // C++: m_bestask_lastpnl

    // ---- 风控最大手数 ----
    // 迁移自: ExecutionStrategy.h — m_rmsQty
    public int rmsQty;                    // C++: m_rmsQty — RMS 最大平仓手数

    // ---- 订单/价格 Map ----
    // 迁移自: ExecutionStrategy.h:257-264
    // C++: OrderMap = map<uint32_t, OrderStats*>
    // C++: PriceMap = map<double, OrderStats*>
    public final Map<Integer, OrderStats> ordMap = new LinkedHashMap<>();

    // ---- 订单历史环形缓冲区（Java 新增，C++ 无对应） ----
    // 模拟器成交极快（~150ms），订单在快照采集间隔（1s）内完成整个生命周期，
    // 导致 ordMap 中永远捕捉不到。此缓冲区在订单事件（创建/成交/撤单）时直接记录，
    // 供 DashboardSnapshot 采集，确保 Overview 页的 Orders/Fills/SpreadTrades 表有数据。
    public static final int ORDER_HISTORY_CAPACITY = 100;
    public final ArrayDeque<OrderHistoryEntry> orderHistory = new ArrayDeque<>();

    /** 订单历史条目 */
    public static class OrderHistoryEntry {
        public int orderID;
        public byte side;
        public double price;
        public int openQty;
        public int doneQty;
        public String status;  // NEW, TRADED, CANCEL_CONFIRM, NEW_REJECT
        public String ordType; // QUOTE, SUPPORTING, AGGRESSIVE 等
        public long timestampNanos;

        public OrderHistoryEntry(OrderStats ord, String status) {
            this.orderID = ord.orderID;
            this.side = ord.side;
            this.price = ord.price;
            this.openQty = ord.openQty;
            this.doneQty = ord.doneQty;
            this.status = status;
            this.ordType = ord.ordType != null ? ord.ordType.name() : "UNKNOWN";
            this.timestampNanos = System.nanoTime();
        }
    }

    /** 记录订单事件到历史缓冲区 */
    protected void recordOrderEvent(OrderStats ord, String status) {
        if (orderHistory.size() >= ORDER_HISTORY_CAPACITY) {
            orderHistory.pollFirst();
        }
        orderHistory.addLast(new OrderHistoryEntry(ord, status));
    }
    public final Map<Double, OrderStats> bidMap = new TreeMap<>();
    public final Map<Double, OrderStats> askMap = new TreeMap<>();

    // ---- Self-book Cache Map ----
    // 迁移自: ExecutionStrategy.h — m_bidMapCache, m_askMapCache
    // C++: PriceMap m_bidMapCache, m_askMapCache — used for self-book tracking
    public final Map<Double, OrderStats> bidMapCache = new TreeMap<>();
    public final Map<Double, OrderStats> askMapCache = new TreeMap<>();

    // ---- 统计字段 ----
    // 迁移自: ExecutionStrategy.h — instruAvgTradeQty, volumeEwa, SET_HIGH, prevTradeQty
    // C++: deque<uint64_t> StatTrTimeQ; deque<double> StatTradeQtyQ;
    public double instruAvgTradeQty;
    public double volumeEwa;
    public int SET_HIGH;
    public double prevTradeQty;
    public final Deque<Long> statTrTimeQ = new ArrayDeque<>();
    public final Deque<Double> statTradeQtyQ = new ArrayDeque<>();

    // ---- PNL 目标 ----
    public double[] targetBidPNL;
    public double[] targetAskPNL;

    // ---- 产品标识 ----
    public String product = "";

    /**
     * 构造函数。
     * 迁移自: ExecutionStrategy::ExecutionStrategy(CommonClient*, SimConfig*)
     * Ref: ExecutionStrategy.cpp:20-132
     */
    public ExecutionStrategy(CommonClient client, SimConfig simConfig) {
        this.client = client;
        this.configParams = ConfigParams.getInstance();
        this.simConfig = simConfig;
        this.instru = simConfig.instrument;
        if (simConfig.useArbStrat) {
            this.instruSec = simConfig.instrumentSec;
        }
        this.strategyID = configParams.strategyID;
        this.thold = simConfig.thresholdSet;
        this.maxOrderCount = MAX_ORDERS;
        this.maxPosSize = MAX_POS_SIZE;

        // C++: m_buyExchTx = m_simConfig->m_buyExchTx
        this.buyExchTx = simConfig.buyExchTx;
        this.sellExchTx = simConfig.sellExchTx;
        this.buyExchContractTx = simConfig.buyExchContractTx;
        this.sellExchContractTx = simConfig.sellExchContractTx;

        // 初始化时间限制 — 与 C++ ExecutionStrategy 构造函数对齐
        // C++: m_endTime = simConfig->m_dateConfig.m_endTime - 180000;  (3 分钟前)
        // C++: m_endTimeEpoch = GetNanoSecsFromEpoch(currDate, 0) + m_endTime * 1000000;
        // C++: m_endTimeAgg = simConfig->m_dateConfig.m_endTime - 60000; (1 分钟前)
        // C++: m_endTimeAggEpoch = GetNanoSecsFromEpoch(currDate, 0) + m_endTimeAgg * 1000000;
        // Ref: ExecutionStrategy.cpp:43-46
        initEndTimeEpochs(simConfig.endTime);

        reset();
    }

    /**
     * 初始化结束时间的 epoch 纳秒值。
     * 迁移自: ExecutionStrategy::ExecutionStrategy() — 时间初始化部分
     * Ref: ExecutionStrategy.cpp:43-46, TradeBotUtils.cpp:2586
     *
     * C++ 流程:
     *   1. endTime 从 controlFile 读取，格式 "HHMM"，如 "1500"
     *   2. dateConfig.m_endTime = ((HHMM/100)*3600 + (HHMM%100)*60) * 1000  → 毫秒自午夜
     *   3. m_endTime = dateConfig.m_endTime - 180000  (结束前 3 分钟)
     *   4. m_endTimeEpoch = GetNanoSecsFromEpoch(date, 0) + m_endTime * 1000000
     *   5. m_endTimeAgg = dateConfig.m_endTime - 60000  (结束前 1 分钟)
     *   6. m_endTimeAggEpoch = GetNanoSecsFromEpoch(date, 0) + m_endTimeAgg * 1000000
     *
     * @param endTimeStr HHMM 格式字符串，如 "1500"
     */
    private void initEndTimeEpochs(String endTimeStr) {
        if (endTimeStr == null || endTimeStr.isEmpty()) {
            // 未配置 endTime — 设为 Long.MAX_VALUE 表示不触发时间限制
            endTimeEpoch = Long.MAX_VALUE;
            endTimeAggEpoch = Long.MAX_VALUE;
            log.info("[时间限制] endTime 未配置，时间限制已禁用");
            return;
        }
        try {
            // C++: dateConfig.m_endTime = atoi(controlConfig.m_endTime)  → HHMM int
            int hhmm = Integer.parseInt(endTimeStr.trim());
            int h = hhmm / 100;
            int m = hhmm % 100;

            // C++: dateConfig.m_endTime = ((h*3600) + (m*60)) * 1000  → 毫秒自午夜
            long endTimeMs = ((long) h * 3600 + (long) m * 60) * 1000;

            // C++: m_endTime = dateConfig.m_endTime - 180000  (3 分钟前)
            endTime = endTimeMs - 180_000;
            // C++: m_endTimeAgg = dateConfig.m_endTime - 60000  (1 分钟前)
            endTimeAgg = endTimeMs - 60_000;

            // C++: GetNanoSecsFromEpoch(currDate, 0) → 当天 00:00:00 UTC 的纳秒
            // Java 等价: 当天午夜的 epoch 纳秒
            // [C++差异] C++ 使用 mktime-timezone 转换为 UTC epoch，
            // Java 使用 ZoneId.of("Asia/Shanghai") 因为 SHFE 交易时间为北京时间
            long midnightNanos = LocalDate.now()
                .atStartOfDay(ZoneId.of("Asia/Shanghai"))
                .toInstant()
                .toEpochMilli() * 1_000_000L;

            // C++: m_endTimeEpoch = GetNanoSecsFromEpoch(date, 0) + (uint64_t)(m_endTime) * 1000000
            endTimeEpoch = midnightNanos + endTime * 1_000_000L;
            // C++: m_endTimeAggEpoch = GetNanoSecsFromEpoch(date, 0) + (uint64_t)(m_endTimeAgg) * 1000000
            endTimeAggEpoch = midnightNanos + endTimeAgg * 1_000_000L;

            log.info(String.format("[时间限制] endTime=%s → endTimeEpoch=%d endTimeAggEpoch=%d (agg=%02d:%02d exit=%02d:%02d)",
                endTimeStr, endTimeEpoch, endTimeAggEpoch,
                (int)(endTimeAgg / 1000 / 3600), (int)((endTimeAgg / 1000 % 3600) / 60),
                (int)(endTime / 1000 / 3600), (int)((endTime / 1000 % 3600) / 60)));
        } catch (NumberFormatException e) {
            // 格式错误 — 禁用时间限制
            endTimeEpoch = Long.MAX_VALUE;
            endTimeAggEpoch = Long.MAX_VALUE;
            log.warning("[时间限制] endTime 格式错误: " + endTimeStr + "，时间限制已禁用");
        }
    }

    // ---- 监控时间戳 ----
    // 迁移自: ExecutionStrategy.h — m_lastStsTS
    public long lastStsTS;

    // =======================================================================
    //  监控上报 — SendMonitor* / SendAlert
    //  [C++差异] C++ 使用 SHM MemLog (mlog->enqueue) 推送监控数据，
    //  Java 使用日志输出，因为 Web API (Javalin) 已提供实时监控功能。
    // =======================================================================

    /**
     * 上报策略持仓。
     * 迁移自: ExecutionStrategy::SendMonitorStratPos(...)
     * Ref: ExecutionStrategy.cpp:133-161
     */
    public void sendMonitorStratPos(String prd, int id, String syb, double bpx, double spx,
            double btpx, double stpx, double bqty, double sqty, double btqty, double stqty, double netposVal) {
        // C++: mlog->enqueue(S) where S.type = MemLogType::POSITION
        // [C++差异] Java 使用日志输出替代 SHM MemLog
        log.info(String.format("[Monitor:POSITION] product=%s id=%d symbol=%s netpos=%.0f buyPx=%.2f sellPx=%.2f buyQty=%.0f sellQty=%.0f",
                prd, id, syb, netposVal, bpx, spx, bqty, sqty));
    }

    /**
     * 上报策略详情。
     * 迁移自: ExecutionStrategy::SendMonitorStratDetail(...)
     * Ref: ExecutionStrategy.cpp:163-195
     */
    public void sendMonitorStratDetail(String prd, String key, String val, String myinfo, int id, int type, boolean isalert) {
        // C++: mlog->enqueue(S) where S.type = MemLogType::STRATEGY_DETAIL
        if (key != null && !key.isEmpty() && val != null && !val.isEmpty() && myinfo != null && !myinfo.isEmpty()) {
            log.info(String.format("[Monitor:DETAIL] product=%s id=%d key=%s val=%s info=%s alert=%b",
                    prd, id, key, val, myinfo, isalert));
        } else {
            log.fine("[Monitor:DETAIL] enqueue failed, blank fields");
        }
    }

    /**
     * 上报策略 PNL。
     * 迁移自: ExecutionStrategy::SendMonitorStratPNL(...)
     * Ref: ExecutionStrategy.cpp:196-219
     */
    public void sendMonitorStratPNL(String prd, int id, double ur, double r, double g, double f, double n) {
        // C++: mlog->enqueue(S) where S.type = MemLogType::PNL
        log.info(String.format("[Monitor:PNL] product=%s id=%d unrealised=%.2f realised=%.2f gross=%.2f fee=%.2f net=%.2f",
                prd, id, ur, r, g, f, n));
    }

    /**
     * 上报策略状态。
     * 迁移自: ExecutionStrategy::SendMonitorStratStatus(...)
     * Ref: ExecutionStrategy.cpp:221-243
     */
    public void sendMonitorStratStatus(String prd, int id, boolean onexit, boolean oncancel, boolean onflat, boolean activeVal) {
        // C++: mlog->enqueue(S) where S.type = MemLogType::STRATEGY_STATUS
        log.info(String.format("[Monitor:STATUS] product=%s id=%d active=%b onExit=%b onCancel=%b onFlat=%b",
                prd, id, activeVal, onexit, oncancel, onflat));
    }

    /**
     * 上报撤单统计。
     * 迁移自: ExecutionStrategy::SendMonitorStratCancelSts(...)
     * Ref: ExecutionStrategy.cpp:244-264
     */
    public void sendMonitorStratCancelSts(String prd, String instruName, int id, int ordcnt, int cancelcnt) {
        // C++: mlog->enqueue(S) where S.type = MemLogType::CANCEL_STS
        log.info(String.format("[Monitor:CANCEL_STS] product=%s symbol=%s id=%d orderCount=%d cancelCount=%d",
                prd, instruName, id, ordcnt, cancelcnt));
    }

    /**
     * 发送告警。
     * 迁移自: ExecutionStrategy::SendAlert(const char *msg, const char *reason)
     * Ref: ExecutionStrategy.cpp:1156-1173
     *
     * [C++差异] C++ 在 Live 模式下原有 email 告警（已注释），目前仅日志 + 推送监控。
     * Java 使用 WARNING 级别日志 + 调用 sendMonitorStratDetail/sendMonitorStratStatus。
     */
    public void sendAlert(String msg, String reason) {
        // C++: TBLOG << "ALERT! Limit got hit." << msg << " Reason: " << reason << endl;
        log.warning("ALERT! Limit got hit. " + msg + " Reason: " + reason
                + " Symbol: " + instru.origBaseName);

        // C++: SendMonitorStratDetail(...) + SendMonitorStratStatus(...)
        String info = instru.currDate + " ALERT! Limit got hit. Symbol: " + instru.origBaseName + " " + msg + " Reason: " + reason;
        sendMonitorStratDetail(product, "m_netpos", String.valueOf(netpos), info, strategyID, 1, true);
        sendMonitorStratStatus(product, strategyID, onExit, onCancel, onFlat, active);
    }

    /**
     * 重置所有状态到初始值。
     * 迁移自: ExecutionStrategy::Reset()
     * Ref: ExecutionStrategy.cpp:276-396
     */
    public void reset() {
        cancelconfirmCount = 0;
        netpos = 0;
        netposPass = 0;
        netposPassYtd = 0;
        netposAgg = 0;
        buyQty = 0; sellQty = 0;
        buyTotalQty = 0; sellTotalQty = 0;
        buyOpenQty = 0; sellOpenQty = 0;
        buyValue = 0; sellValue = 0;
        transValue = 0;
        buyTotalValue = 0; sellTotalValue = 0;
        transTotalValue = 0;
        buyPrice = 0; sellPrice = 0;
        buyAvgPrice = 0; sellAvgPrice = 0;
        realisedPNL = 0; unrealisedPNL = 0;
        netPNL = 0; grossPNL = 0;
        maxPNL = 0; drawdown = 0;
        ltp = 0; targetPrice = 0; currPrice = 0;
        onExit = false; onCancel = false;
        onFlat = false; aggFlat = false;
        onNewsFlat = false; onStopLoss = false;
        buyOpenOrders = 0; sellOpenOrders = 0;
        lastFlatTS = 0;
        improveCount = 0; crossCount = 0;
        tradeCount = 0; rejectCount = 0;
        orderCount = 0; cancelCount = 0;
        confirmCount = 0;
        exchTS = 0; localTS = 0;
        currAvgPrice = 0;
        avgQty = 0;
        lastTradeSide = false;
        lastTrade = false;
        lastTradePx = 0;
        lastTradeTime = 0;

        buyAggCount = 0; sellAggCount = 0;
        buyAggOrder = 0; sellAggOrder = 0;
        lastAggTime = 0; lastAggSide = 0;

        tholdBidSize = 0; tholdBidMaxPos = 0;
        tholdAskSize = 0; tholdAskMaxPos = 0;

        // C++: m_Active = m_configParams->m_modeType == ModeType_Sim ? true : false
        active = (configParams.modeType == 1);

        // C++: m_maxTradedQty = m_instru->m_sendInLots ? m_maxPosSize : m_maxPosSize * m_instru->m_lotSize
        // Ref: ExecutionStrategy.cpp — Reset()
        if (instru != null) {
            maxTradedQty = instru.sendInLots ? maxPosSize : maxPosSize * instru.lotSize;
        }

        // C++: m_optionStrategy = m_configParams->m_optionStrategy
        // Ref: ExecutionStrategy.cpp — Reset()
        optionStrategy = (configParams.optionStrategy != 0);

        ordMap.clear();
        bidMap.clear();
        askMap.clear();
    }

    // =======================================================================
    //  纯虚方法
    // =======================================================================

    /**
     * 发单逻辑 — 子类必须实现。
     * 迁移自: ExecutionStrategy::SendOrder() = 0 (pure virtual)
     */
    public abstract void sendOrder();

    // =======================================================================
    //  SetTargetValue — 主入口
    // =======================================================================

    /**
     * 设置目标价格/PNL 并触发 SendOrder。
     * 迁移自: ExecutionStrategy::SetTargetValue(double &currPrice, double &targetPrice, double *targetBidPNL, double *targetAskPNL)
     * Ref: ExecutionStrategy.cpp:422-482
     *
     * @param currPriceVal  当前价格
     * @param targetPriceVal 目标价格
     * @param targetBidPNLArr  各档 Bid PNL 数组
     * @param targetAskPNLArr  各档 Ask PNL 数组
     */
    public void setTargetValue(double currPriceVal, double targetPriceVal, double[] targetBidPNLArr, double[] targetAskPNLArr) {
        // C++: m_currPrice = currPrice;
        this.currPrice = currPriceVal;
        // C++: m_targetPrice = targetPrice;
        this.targetPrice = targetPriceVal;
        // C++: m_targetBidPNL = targetBidPNL;
        this.targetBidPNL = targetBidPNLArr;
        // C++: m_targetAskPNL = targetAskPNL;
        this.targetAskPNL = targetAskPNLArr;

        // C++: if (m_configParams->m_modeType == ModeType_Sim) m_localTS = Watch::GetUniqueInstance()->GetCurrentTime();
        // Ref: ExecutionStrategy.cpp:432
        if (configParams.modeType == 1) { // ModeType_Sim = 1
            localTS = Watch.getInstance().getCurrentTime();
        }

        // C++: if ((m_rejectCount > REJECT_LIMIT - 100) && m_onFlat == false && m_Active)
        if ((rejectCount > REJECT_LIMIT - 100) && !onFlat && active) {
            // C++: SendAlert("Strategy squared off due to reject limit", " MAX REJECT LIMIT got hit");
            log.warning("[" + product + "] REJECT Limit approaching (" + rejectCount + "), cancelling orders and square off...");
        }

        // C++: 监控状态上报 — 每 120 秒
        // C++: auto curr_time = Watch::GetUniqueInstance()->GetCurrentTime();
        // Ref: ExecutionStrategy.cpp:438-450
        long currTime = Watch.getInstance().getCurrentTime();
        // C++: uint64_t gap = 1000000000; if (mlog && curr_time - m_lastStsTS > gap * 120)
        long gap = 1_000_000_000L;
        if (currTime - lastStsTS > gap * 120) {
            // C++: SendMonitorStratStatus(m_product, m_strategyID, m_onExit, m_onCancel, m_onFlat, m_Active);
            sendMonitorStratStatus(product, strategyID, onExit, onCancel, onFlat, active);
            lastStsTS = currTime;
            // C++: if (!m_simConfig->m_bUseArbStrat) { ... }
            // arb 策略（PairwiseArbStrategy）在自己的 MDCallBack 中上报，不在基类重复上报
            if (!simConfig.useArbStrat) {
                // C++: SendMonitorStratPos(m_product, m_strategyID, m_instru->m_instrument, ...)
                sendMonitorStratPos(product, strategyID, instru.instrument,
                        buyPrice, sellPrice, buyAvgPrice, sellAvgPrice,
                        buyQty, sellQty, buyTotalQty, sellTotalQty, netpos);
                // C++: SendMonitorStratPNL(m_product, m_strategyID, ...)
                sendMonitorStratPNL(product, strategyID, unrealisedPNL, realisedPNL,
                        grossPNL, transTotalValue, netPNL);
                // C++: SendMonitorStratCancelSts(m_product, m_instru->m_instrument, m_strategyID, ...)
                sendMonitorStratCancelSts(product, instru.instrument, strategyID,
                        orderCount, cancelconfirmCount);
            }
        }

        // C++: if (!m_onFlat && m_Active) { ... SendOrder(); }
        // Ref: ExecutionStrategy.cpp:454-472
        if (!onFlat && active) {
            // C++: if (m_optionStrategy) { ... }
            // Ref: ExecutionStrategy.cpp:456-467
            if (optionStrategy) {
                // C++: if (!strcmp(m_instru->m_origbaseName, m_configParams->m_underlyingSimConfig->m_instru->m_origbaseName))
                // [C++差异] Java 中 underlyingSimConfig 未迁移，使用 configParams.updateSymbol 替代。
                // C++ 原始逻辑: 如果当前合约是 underlying，不 return (允许交易 futures)
                // C++ 原始逻辑: 如果非 parityMode 且非 underlying 且合约不匹配 updateSymbol，则 return
                // Ref: ExecutionStrategy.cpp:458-466
                // [C++差异] OptionManager.parityMode 和 configParams.underlying 标志未迁移,
                // 保留逻辑结构; 当 OptionManager 可用时启用此分支
                // if (!OptionManager.parityMode && !configParams.underlying
                //         && !configParams.updateSymbol.equals(instru.instrument)) {
                //     return;
                // }
            }

            // C++: if (((bCrossBook || bCrossBook2) && ((bCrossBookEnd && !lastInstruMapIter->crossUpdate) || !bCrossBookEnd))
            //      || !(bCrossBook || bCrossBook2)) { SendOrder(); }
            // Ref: ExecutionStrategy.cpp:469-472
            if (((configParams.bCrossBook || configParams.bCrossBook2)
                    && ((configParams.bCrossBookEnd && !simConfig.lastCrossUpdate()) || !configParams.bCrossBookEnd))
                    || !(configParams.bCrossBook || configParams.bCrossBook2)) {
                sendOrder();
            }
        }

        // C++: if (m_rejectCount > REJECT_LIMIT) { m_onCancel = true; m_onFlat = true; m_Active = false; }
        if (rejectCount > REJECT_LIMIT) {
            log.severe("REJECT LIMIT REACHED!!! Strategy Stop!");
            onCancel = true;
            onFlat = true;
            active = false;
        }
    }

    // =======================================================================
    //  阈值设置
    // =======================================================================

    /**
     * 基于持仓的阶梯阈值设置。
     * 迁移自: ExecutionStrategy::SetThresholds()
     * Ref: ExecutionStrategy.cpp:595-689
     */
    public void setThresholds() {
        // C++: m_tholdBidPlace = -1; m_tholdBidRemove = -1; ...
        tholdBidPlace = -1;
        tholdBidRemove = -1;
        tholdAskPlace = -1;
        tholdAskRemove = -1;
        tholdMaxPos = 0;
        tholdBeginPos = 0;
        tholdSize = 0;

        // C++: if (m_thold->USE_NOTIONAL) { ... } else { ... }
        if (thold.USE_NOTIONAL) {
            double mktPx = (instru.bidPx[0] + instru.askPx[0]) / 2;
            double contractVal = mktPx * instru.lotSize;
            tholdMaxPos = (int) ((thold.NOTIONAL_MAX_SIZE * instru.priceFactor / contractVal) * instru.lotSize);
            tholdSize = (int) ((thold.NOTIONAL_SIZE * instru.priceFactor / contractVal) * instru.lotSize);
            smsRatio = (int) (thold.NOTIONAL_MAX_SIZE / thold.NOTIONAL_SIZE);
        } else {
            // C++: if (m_instru->m_sendInLots)
            // Java: 直接使用 lots 模式（中国期货按手）
            tholdMaxPos = (int) thold.MAX_SIZE;
            tholdBeginPos = (int) thold.BEGIN_SIZE;
            tholdSize = (int) thold.SIZE;
            smsRatio = (int) (thold.MAX_SIZE / thold.SIZE);
        }

        // C++: SetThresholds() 阶梯逻辑 — 含 SET_HIGH 分支
        // Ref: ExecutionStrategy.cpp:635-689
        if (netpos == 0) {
            // C++: if (SET_HIGH == 1) { BEGIN_PLACE_HIGH } else { BEGIN_PLACE }
            if (SET_HIGH == 1) {
                tholdBidPlace = thold.BEGIN_PLACE_HIGH;
                tholdAskPlace = thold.BEGIN_PLACE_HIGH;
            } else {
                tholdBidPlace = thold.BEGIN_PLACE;
                tholdAskPlace = thold.BEGIN_PLACE;
            }
            tholdBidRemove = thold.BEGIN_REMOVE;
            tholdAskRemove = thold.BEGIN_REMOVE;
        } else if (netpos >= 0 && netpos < tholdBeginPos) {
            // C++: if (SET_HIGH == 1) m_tholdBidPlace = BEGIN_PLACE_HIGH; else m_tholdBidPlace = BEGIN_PLACE;
            if (SET_HIGH == 1) {
                tholdBidPlace = thold.BEGIN_PLACE_HIGH;
            } else {
                tholdBidPlace = thold.BEGIN_PLACE;
            }
            tholdBidRemove = thold.BEGIN_REMOVE;
            tholdAskPlace = thold.SHORT_PLACE;
            tholdAskRemove = thold.SHORT_REMOVE;
        } else if (netpos <= 0 && netpos > -1 * tholdBeginPos) {
            // C++: if (SET_HIGH == 1) m_tholdBidPlace = BEGIN_PLACE_HIGH; else m_tholdAskPlace = BEGIN_PLACE;
            // 注意: C++ 中 SET_HIGH==1 时设置的 tholdBidPlace 会被后面 SHORT_PLACE 覆盖（C++ 原代码行为）
            if (SET_HIGH == 1) {
                tholdBidPlace = thold.BEGIN_PLACE_HIGH;
            } else {
                tholdAskPlace = thold.BEGIN_PLACE;
            }
            tholdAskRemove = thold.BEGIN_REMOVE;
            tholdBidPlace = thold.SHORT_PLACE;
            tholdBidRemove = thold.SHORT_REMOVE;
        } else if (netpos > 0) {
            // C++: if (SET_HIGH == 1) m_tholdBidPlace = LONG_PLACE_HIGH; else m_tholdBidPlace = LONG_PLACE;
            if (SET_HIGH == 1) {
                tholdBidPlace = thold.LONG_PLACE_HIGH;
            } else {
                tholdBidPlace = thold.LONG_PLACE;
            }
            tholdBidRemove = thold.LONG_REMOVE;
            tholdAskPlace = thold.SHORT_PLACE;
            tholdAskRemove = thold.SHORT_REMOVE;
        } else if (netpos < 0) {
            // C++: if (SET_HIGH == 1) m_tholdAskPlace = LONG_PLACE_HIGH; else m_tholdAskPlace = LONG_PLACE;
            if (SET_HIGH == 1) {
                tholdAskPlace = thold.LONG_PLACE_HIGH;
            } else {
                tholdAskPlace = thold.LONG_PLACE;
            }
            tholdAskRemove = thold.LONG_REMOVE;
            tholdBidPlace = thold.SHORT_PLACE;
            tholdBidRemove = thold.SHORT_REMOVE;
        }
    }

    /**
     * 线性插值阈值设置。
     * 迁移自: ExecutionStrategy::SetLinearThresholds()
     * Ref: ExecutionStrategy.cpp:500-593
     */
    public void setLinearThresholds() {
        tholdBidPlace = -1;
        tholdBidRemove = -1;
        tholdAskPlace = -1;
        tholdAskRemove = -1;
        tholdMaxPos = 0;
        tholdBeginPos = 0;
        tholdSize = 0;

        if (thold.USE_NOTIONAL) {
            double mktPx = (instru.bidPx[0] + instru.askPx[0]) / 2;
            double contractVal = mktPx * instru.lotSize;
            tholdMaxPos = (int) ((thold.NOTIONAL_MAX_SIZE * instru.priceFactor / contractVal) * instru.lotSize);
            tholdSize = (int) ((thold.NOTIONAL_SIZE * instru.priceFactor / contractVal) * instru.lotSize);
            smsRatio = (int) (thold.NOTIONAL_MAX_SIZE / thold.NOTIONAL_SIZE);
        } else {
            tholdMaxPos = (int) thold.MAX_SIZE;
            tholdBeginPos = (int) thold.BEGIN_SIZE;
            tholdSize = (int) thold.SIZE;
            smsRatio = (int) (thold.MAX_SIZE / thold.SIZE);
        }

        // C++: 线性插值
        if (netpos == 0) {
            tholdBidPlace = thold.BEGIN_PLACE;
            tholdAskPlace = thold.BEGIN_PLACE;
            tholdBidRemove = thold.BEGIN_REMOVE;
            tholdAskRemove = thold.BEGIN_REMOVE;
        } else if (netpos >= 0) {
            // C++: m_tholdBidPlace = m_thold->BEGIN_PLACE + ((LONG_PLACE - BEGIN_PLACE) * m_netpos) / m_tholdMaxPos
            tholdBidPlace = thold.BEGIN_PLACE + ((thold.LONG_PLACE - thold.BEGIN_PLACE) * netpos) / tholdMaxPos;
            tholdBidRemove = thold.BEGIN_REMOVE + ((thold.LONG_REMOVE - thold.BEGIN_REMOVE) * netpos) / tholdMaxPos;
            tholdAskPlace = thold.BEGIN_PLACE - ((thold.BEGIN_PLACE - thold.SHORT_PLACE) * netpos) / tholdMaxPos;
            tholdAskRemove = thold.BEGIN_REMOVE - ((thold.BEGIN_REMOVE - thold.SHORT_REMOVE) * netpos) / tholdMaxPos;
        } else {
            // netpos < 0
            tholdAskPlace = thold.BEGIN_PLACE + ((thold.LONG_PLACE - thold.BEGIN_PLACE) * -1 * netpos) / tholdMaxPos;
            tholdAskRemove = thold.BEGIN_REMOVE + ((thold.LONG_REMOVE - thold.BEGIN_REMOVE) * -1 * netpos) / tholdMaxPos;
            tholdBidPlace = thold.BEGIN_PLACE - ((thold.BEGIN_PLACE - thold.SHORT_PLACE) * -1 * netpos) / tholdMaxPos;
            tholdBidRemove = thold.BEGIN_REMOVE - ((thold.BEGIN_REMOVE - thold.SHORT_REMOVE) * -1 * netpos) / tholdMaxPos;
        }
    }

    // =======================================================================
    //  ORS 回调
    // =======================================================================

    /**
     * 订单响应回调。
     * 迁移自: ExecutionStrategy::ORSCallBack(ResponseMsg*)
     * Ref: ExecutionStrategy.cpp:951-1154
     */
    public void orsCallBack(MemorySegment response) {
        int orderID = (int) Types.RESP_ORDER_ID_VH.get(response, 0L);
        int responseType = (int) Types.RESP_RESPONSE_TYPE_VH.get(response, 0L);

        OrderStats order = ordMap.get(orderID);
        if (order == null) {
            log.warning("Response OrderID not found: " + orderID);
            return;
        }

        // C++: switch (response->Response_Type)
        switch (responseType) {
            case Constants.RESP_NEW_ORDER_CONFIRM:
                // C++: iter->second->m_status = NEW_CONFIRM
                order.status = OrderStats.Status.NEW_CONFIRM;
                rejectCount = 0;
                confirmCount++;
                break;

            case Constants.RESP_MODIFY_ORDER_CONFIRM:
                order.status = OrderStats.Status.MODIFY_CONFIRM;
                processModifyConfirm(response, order);
                rejectCount = 0;
                confirmCount++;
                break;

            case Constants.RESP_CANCEL_ORDER_CONFIRM:
                processCancelConfirm(response, order);
                rejectCount = 0;
                confirmCount++;
                cancelconfirmCount++;
                break;

            case Constants.RESP_TRADE_CONFIRM:
                if (order.status != OrderStats.Status.MODIFY_ORDER && order.status != OrderStats.Status.CANCEL_ORDER)
                    order.status = OrderStats.Status.NEW_CONFIRM;
                processTrade(response, order);
                rejectCount = 0;
                break;

            case Constants.RESP_NEW_ORDER_FREEZE:
            case Constants.RESP_ORDER_ERROR:
                order.status = OrderStats.Status.NEW_REJECT;
                processNewReject(response, order);
                rejectCount++;
                break;

            case Constants.RESP_MODIFY_ORDER_REJECT:
                if (order.status != OrderStats.Status.TRADED)
                    order.status = OrderStats.Status.MODIFY_REJECT;
                processModifyReject(response, order);
                rejectCount++;
                break;

            case Constants.RESP_CANCEL_ORDER_REJECT:
                if (order.status != OrderStats.Status.TRADED) {
                    order.status = OrderStats.Status.NEW_CONFIRM;
                    processCancelReject(response, order);
                }
                rejectCount++;
                break;

            default:
                break;
        }
    }

    // =======================================================================
    //  MD 回调
    // =======================================================================

    /**
     * 行情回调 — 基类实现。
     * 迁移自: ExecutionStrategy::MDCallBack(MarketUpdateNew*)
     * Ref: ExecutionStrategy.cpp:774-819
     */
    public void mdCallBack(MemorySegment update) {
        // C++: if (up->m_updateType == MDUPDTYPE_TRADE || up->m_updateType == MDUPDTYPE_TRADE_IMPLIED)
        //          m_ltp = up->m_newPrice;
        // Ref: ExecutionStrategy.cpp:776-777
        long mdDataBase = Types.MU_DATA_OFFSET;
        byte updateType = (byte) Types.MDD_UPDATE_TYPE_VH.get(update, mdDataBase);
        if (updateType == Constants.MDUPDTYPE_TRADE || updateType == Constants.MDUPDTYPE_TRADE_IMPLIED) {
            ltp = (double) Types.MDD_NEW_PRICE_VH.get(update, mdDataBase);
        }

        // C++: m_exchTS = up->m_timestamp;
        // Ref: ExecutionStrategy.cpp:779
        // [C++差异] C++ 在此处赋值 m_exchTS，Java 已迁移为全局 Watch 时钟。
        // Watch 在 CommonClient.sendINDUpdate() 中统一更新，此处不再单独读取。
        // 保留 exchTS 字段赋值以兼容可能的遗留引用:
        exchTS = Watch.getInstance().getCurrentTime();

        // C++: if ((m_bestbid_lastpnl != m_instru->bidPx[0]) || (m_bestask_lastpnl != m_instru->askPx[0]))
        //          { CalculatePNL(); m_bestbid_lastpnl = m_instru->bidPx[0]; m_bestask_lastpnl = m_instru->askPx[0]; }
        // Ref: ExecutionStrategy.cpp:790-795
        // [C++差异] C++ 启动后立即 activate 且行情很快到齐，不会在 bidPx=0 时算 PNL。
        // Java 需要等手动激活，期间行情已到但可能只有一腿有价格，bidPx=0 会导致
        // unrealisedPNL 巨大 → MAX LOSS 立即触发。加入 bidPx>0 守卫。
        if (instru.bidPx[0] > 0 && instru.askPx[0] > 0) {
            if (bestbidLastpnl != instru.bidPx[0] || bestaskLastpnl != instru.askPx[0]) {
                calculatePNL();
                bestbidLastpnl = instru.bidPx[0];
                bestaskLastpnl = instru.askPx[0];
            }

            // C++: CheckSquareoff(up);
            // Ref: ExecutionStrategy.cpp:797
            checkSquareoff();
        }

        // C++: if (m_thold->USE_AHEAD_PERCENT) SetQuantAhead(up);
        // Ref: ExecutionStrategy.cpp:799-800
        if (thold.USE_AHEAD_PERCENT) {
            setQuantAhead(update);
        }
    }

    /**
     * 检查各种平仓条件（完整版）。
     * 迁移自: ExecutionStrategy::CheckSquareoff(MarketUpdateNew*)
     * Ref: ExecutionStrategy.cpp:2150-2341
     *
     * C++ 检查条件:
     * 1. endTimeAgg — 激进平仓时间
     * 2. endTime / MAX_LOSS / maxOrderCount / maxTradedQty — 退出条件
     * 3. Option delta 范围检查（滚动平均 delta）
     * 4. USE_PRICE_LIMIT — 滚动平均价格 MIN_PRICE/MAX_PRICE 范围检查
     * 5. UPNL_LOSS / STOP_LOSS — 触发后阈值翻倍
     * 6. NEWS_FLAT — 新闻平仓
     * 7. Flat 恢复逻辑 — 15分钟 StopLoss 冷却、价格范围回归、新闻退出 Flat
     * 8. HandleSquareoff() — 当 onFlat 时执行平仓
     */
    protected void checkSquareoff() {
        // === 1. 激进平仓时间 ===
        // C++: if (Watch::GetUniqueInstance()->GetCurrentTime() >= m_endTimeAggEpoch && !m_aggFlat)
        // Ref: ExecutionStrategy.cpp:2152-2161
        long watchTime = Watch.getInstance().getCurrentTime();
        if (watchTime >= endTimeAggEpoch && !aggFlat) {
            aggFlat = true;
            onExit = true;
            onCancel = true;
            onFlat = true;
            log.warning(instru.currDate + " Exchange Time Limit reached. Aggressive flat got hit!! "
                    + watchTime + " " + endTimeAggEpoch + " Symbol: " + instru.origBaseName);
        }

        // === 2. 退出条件 ===
        // C++: if (((GetCurrentTime() >= m_endTimeEpoch) || m_netPNL < m_thold->MAX_LOSS * -1
        //      || m_orderCount >= m_maxOrderCount || m_buyTotalQty >= m_maxTradedQty
        //      || m_sellTotalQty >= m_maxTradedQty) && !m_onExit)
        // Ref: ExecutionStrategy.cpp:2163-2199
        if (!onExit) {
            String limitReason = null;
            if (watchTime >= endTimeEpoch) {
                limitReason = "END TIME limit got hit!!";
            }
            if (netPNL < thold.MAX_LOSS * -1) {
                limitReason = "MAX LOSS limit got hit!!";
            }
            if (orderCount >= maxOrderCount) {
                limitReason = "MAX ORDERS limit got hit!!";
            }
            if (buyTotalQty >= maxTradedQty || sellTotalQty >= maxTradedQty) {
                limitReason = "MAX TRADED limit got hit!!";
            }

            if (limitReason != null) {
                onExit = true;
                onCancel = true;
                onFlat = true;
                log.warning(watchTime + "  " + endTimeEpoch);
                log.warning(instru.currDate + " Limit reached. Square off is Called. Strategy Exiting.."
                        + " Order Count:" + orderCount + " Buy Qty: " + buyTotalQty
                        + " Sell Qty: " + sellTotalQty + " NetPNL: " + netPNL / 100
                        + " Limit Reason: " + limitReason + " Symbol: " + instru.origBaseName);

                if (watchTime < endTimeEpoch) {
                    sendAlert("Strategy squared off due to limit hit", limitReason);
                }
            }
        }

        // === 3. Option delta 范围检查 ===
        // C++: if (m_optionStrategy && ...) { delta 滚动平均 + minDelta/maxDelta 检查 }
        // Ref: ExecutionStrategy.cpp:2201-2237
        boolean deltaTooLow = false;
        if (optionStrategy) {
            // C++: double delta = abs(OptionManager::GetInstance()->GetOption(m_instru->m_instrument)->GetDelta());
            // [C++差异] Java 未迁移 OptionManager，此处 delta 逻辑保留结构但不执行实际 delta 获取
            // 当 optionStrategy=true 且有 delta 数据时，以下逻辑生效
            double delta = 0; // 由 OptionManager 提供
            double minDelta = 0; // 由 OptionManager 提供
            double maxDelta = 0; // 由 OptionManager 提供

            if (delta != 0 && !Double.isNaN(delta)) {
                // C++: if (m_exchTS - m_lastDeltaTS > 50000000000) // 50 secs (注释写 20 secs，实际 50s)
                if (watchTime - lastDeltaTS > 50_000_000_000L) {
                    currAvgDelta = tmpAvgDelta;
                    lastDeltaTS = watchTime;
                    deltaCount = 1;
                    tmpAvgDelta = delta;
                } else {
                    tmpAvgDelta = (tmpAvgDelta * deltaCount + delta) / (deltaCount + 1);
                    deltaCount++;
                }

                // C++: if (((abs(m_currAvgDelta) < minDelta || abs(m_currAvgDelta) > maxDelta) && (m_currAvgDelta != 0)) && !m_onFlat)
                if (((Math.abs(currAvgDelta) < minDelta || Math.abs(currAvgDelta) > maxDelta) && (currAvgDelta != 0)) && !onFlat) {
                    onCancel = true;
                    onFlat = true;
                    deltaTooLow = true;
                    log.warning(instru.currDate + " DELTA limit reached 1. " + instru.origBaseName
                            + " Squaring off due to min/max delta: " + minDelta + " " + delta + " " + maxDelta);
                }
                // C++: if (((abs(m_currAvgDelta) > minDelta && abs(m_currAvgDelta) < maxDelta) && (m_currAvgDelta != 0)) && m_onFlat && !m_onExit)
                if (((Math.abs(currAvgDelta) > minDelta && Math.abs(currAvgDelta) < maxDelta) && (currAvgDelta != 0)) && onFlat && !onExit) {
                    onCancel = false;
                    onFlat = false;
                    deltaTooLow = false;
                    log.warning(instru.currDate + " DELTA limit reached 2. " + instru.origBaseName
                            + " Squaring off due to min/max delta: " + minDelta + " " + delta + " " + maxDelta);
                }
            }
        }

        // === 4. USE_PRICE_LIMIT ===
        // C++: if (m_thold->USE_PRICE_LIMIT) { ... }
        // Ref: ExecutionStrategy.cpp:2239-2291
        if (thold.USE_PRICE_LIMIT) {
            if (targetPrice > 0) {
                // C++: if (m_exchTS - m_lastPxTS > 50000000000 && !m_onExit) // 50 secs
                if (watchTime - lastPxTS > 50_000_000_000L && !onExit) {
                    currAvgPrice = tmpAvgTargetPrice;
                    lastPxTS = watchTime;
                    priceCount = 1;
                    tmpAvgTargetPrice = targetPrice;
                } else {
                    tmpAvgTargetPrice = (tmpAvgTargetPrice * priceCount + targetPrice) / (priceCount + 1);
                    priceCount++;
                }

                // C++: if ((m_currAvgPrice < m_thold->MIN_PRICE || m_currAvgPrice > m_thold->MAX_PRICE || deltaTooLow || !isModelVolValid) && !m_onFlat)
                if ((currAvgPrice < thold.MIN_PRICE || currAvgPrice > thold.MAX_PRICE || deltaTooLow) && !onFlat) {
                    if (currAvgPrice == 0) {
                        // C++: if (tmpAvgTargetPrice < m_thold->MIN_PRICE || tmpAvgTargetPrice > m_thold->MAX_PRICE)
                        if (tmpAvgTargetPrice < thold.MIN_PRICE || tmpAvgTargetPrice > thold.MAX_PRICE) {
                            onCancel = true;
                            onFlat = true;
                            log.warning(instru.currDate + " PRICE limit reached."
                                    + " Min Price: " + thold.MIN_PRICE + " Price: " + currAvgPrice
                                    + " Max Price: " + thold.MAX_PRICE + " Square off is Called. Strategy Exiting..."
                                    + " Symbol: " + instru.origBaseName);
                        }
                    } else {
                        onCancel = true;
                        onFlat = true;
                        log.warning(instru.currDate + " PRICE limit reached."
                                + " Min Price: " + thold.MIN_PRICE + " Price: " + currAvgPrice
                                + " Max Price: " + thold.MAX_PRICE + " Square off is Called. Strategy Exiting..."
                                + " Symbol: " + instru.origBaseName);
                    }
                }
                // C++: if ((m_currAvgPrice > MIN_PRICE && m_currAvgPrice < MAX_PRICE && !deltaTooLow) && (m_currAvgPrice != 0) && m_onFlat && !m_onExit)
                if ((currAvgPrice > thold.MIN_PRICE && currAvgPrice < thold.MAX_PRICE && !deltaTooLow)
                        && (currAvgPrice != 0) && onFlat && !onExit) {
                    onCancel = false;
                    onFlat = false;
                    log.warning(instru.currDate + " PRICE bound reached."
                            + " Min Price: " + thold.MIN_PRICE + " Price: " + currAvgPrice
                            + " Max Price: " + thold.MAX_PRICE + " Strategy Starting..."
                            + " Symbol: " + instru.origBaseName);
                }
            }
        }

        // === 5. UPNL_LOSS / STOP_LOSS（阈值翻倍） ===
        // C++: if ((m_unrealisedPNL < m_thold->UPNL_LOSS * -1 || m_netPNL < m_thold->STOP_LOSS * -1) && !m_onCancel && !m_onFlat)
        // Ref: ExecutionStrategy.cpp:2293-2320
        if ((unrealisedPNL < thold.UPNL_LOSS * -1 || netPNL < thold.STOP_LOSS * -1) && !onCancel && !onFlat) {
            String limitReason;
            if (unrealisedPNL < thold.UPNL_LOSS * -1) {
                limitReason = "UPNL LOSS limit got hit!!";
            } else {
                limitReason = "STOP LOSS limit got hit!!";
            }

            onStopLoss = true;
            onCancel = true;
            onFlat = true;
            lastFlatTS = watchTime;

            // C++: m_thold->UPNL_LOSS += m_thold->UPNL_LOSS; — 阈值翻倍
            if (unrealisedPNL < thold.UPNL_LOSS * -1) {
                thold.UPNL_LOSS += thold.UPNL_LOSS;
            }
            // C++: m_thold->STOP_LOSS += m_thold->STOP_LOSS; — 阈值翻倍
            if (netPNL < thold.STOP_LOSS * -1) {
                thold.STOP_LOSS += thold.STOP_LOSS;
            }

            sendAlert("Strategy paused due to limit hit", limitReason);
            log.warning(instru.currDate + " Limit reached. Square off is Called. Strategy Paused. Reason: "
                    + limitReason + " Symbol: " + instru.origBaseName);
        }

        // === 6. NEWS_FLAT ===
        // C++: if (!m_onCancel && !m_onFlat && m_useNewsHandler && m_thold->NEWS_FLAT && news_handler->getFlat())
        // Ref: ExecutionStrategy.cpp:2322-2328
        // [C++差异] Java 中 news_handler 未迁移，保留结构但不执行
        // 当 useNewsHandler=true 时，需要外部注入 news handler 实例
        if (!onCancel && !onFlat && useNewsHandler && thold.NEWS_FLAT) {
            // news_handler->getFlat() 的等价检查
            // 当 Java 版 NewsHandler 可用时启用此分支
            // if (newsHandler != null && newsHandler.getFlat()) {
            //     onNewsFlat = true;
            //     onCancel = true;
            //     onFlat = true;
            //     log.warning(instru.currDate + " News Handler: Get Flat");
            // }
        }

        // === 7. Flat 恢复逻辑 ===
        // C++: m_onStopLoss = (m_onFlat) ? m_onStopLoss : false;
        // Ref: ExecutionStrategy.cpp:2330-2341
        onStopLoss = onFlat && onStopLoss;

        if (onFlat) {
            // C++: if (m_exchTS - m_lastFlatTS > 900000000000 && !m_onExit && m_onStopLoss) // 15 mins
            if (watchTime - lastFlatTS > 900_000_000_000L && !onExit && onStopLoss) {
                onFlat = false;
                onStopLoss = false;
                log.warning(instru.currDate + " STOPLOSS time limit reached. Strategy Restarted..");
            }

            // C++: if (!m_optionStrategy && m_thold->USE_PRICE_LIMIT) { ... m_exchTS - m_lastFlatTS > 60000000000 ... } // 1 min
            if (!optionStrategy && thold.USE_PRICE_LIMIT) {
                if (currAvgPrice > thold.MIN_PRICE && currAvgPrice < thold.MAX_PRICE
                        && (currAvgPrice != 0) && !onExit && watchTime - lastFlatTS > 60_000_000_000L) {
                    onFlat = false;
                    log.warning(instru.currDate + " Back in Price Ranges Strategy Restarted..");
                }
            }

            // C++: if (m_useNewsHandler && m_onNewsFlat && !news_handler->getFlat()) { ... }
            if (useNewsHandler && onNewsFlat) {
                // 当 Java 版 NewsHandler 可用时启用此分支
                // if (newsHandler != null && !newsHandler.getFlat()) {
                //     onFlat = false;
                //     onCancel = false;
                //     onNewsFlat = false;
                //     log.warning(instru.currDate + " News Handler: Quit Flat");
                // }
            }

            // === 8. 执行平仓 ===
            // C++: HandleSquareoff();
            // Ref: ExecutionStrategy.cpp:2341
            // [C++差异] 当 useArbStrat=true 时，本 strat 是 PairwiseArbStrategy 的子 strat
            // (firstStrat/secondStrat)。平仓由父级 PairwiseArbStrategy.handleSquareoff() 统一管理
            // （只撤单+设标志，不发新单）。如果在子 strat 级别调用基类 handleSquareoff()，
            // 会绕过父级控制直接发送 SendNewOrder 平仓单，导致：
            // 1. active=false 时仍然发单
            // 2. flag=POS_OPEN 而非 POS_CLOSE（对冲持仓应平仓）
            // 因此子 strat 只设标志不发单，平仓操作由父级处理。
            if (!simConfig.useArbStrat) {
                handleSquareoff();
            }
        }
    }

    /**
     * 队列位置跟踪 — 根据行情更新调整 quantAhead/quantBehind。
     * 迁移自: ExecutionStrategy::SetQuantAhead(MarketUpdateNew *update)
     * Ref: ExecutionStrategy.cpp:691-757
     *
     * @param update MarketUpdateNew MemorySegment
     */
    public void setQuantAhead(MemorySegment update) {
        long mdDataBase = Types.MU_DATA_OFFSET;

        // C++: update->m_side, update->m_newPrice
        byte side = (byte) Types.MDD_SIDE_VH.get(update, mdDataBase);
        double newPrice = (double) Types.MDD_NEW_PRICE_VH.get(update, mdDataBase);
        byte updateType = (byte) Types.MDD_UPDATE_TYPE_VH.get(update, mdDataBase);
        int newQuant = (int) Types.MDD_NEW_QUANT_VH.get(update, mdDataBase);
        int oldQuant = (int) Types.MDD_OLD_QUANT_VH.get(update, mdDataBase);

        // C++: if (update->m_side == SIDE_BUY) { iter = m_bidMap.find(update->m_newPrice); }
        OrderStats ordStats;
        if (side == Constants.SIDE_BUY) {
            ordStats = bidMap.get(newPrice);
        } else {
            ordStats = askMap.get(newPrice);
        }
        // C++: if (iter == m_bidMap.end()) return;
        if (ordStats == null) return;

        // C++: if (update->m_updateType == MDUPDTYPE_TRADE || update->m_updateType == MDUPDTYPE_TRADE_IMPLIED)
        if (updateType == Constants.MDUPDTYPE_TRADE || updateType == Constants.MDUPDTYPE_TRADE_IMPLIED) {
            // C++: ordStats->m_quantAhead -= update->m_newQuant;
            ordStats.quantAhead -= newQuant;
        }

        // C++: if (update->m_updateType == MDUPDTYPE_DELETE || update->m_updateType == MDUPDTYPE_MODIFY)
        if (updateType == Constants.MDUPDTYPE_DELETE || updateType == Constants.MDUPDTYPE_MODIFY) {
            int diffQty = 0;
            if (updateType == Constants.MDUPDTYPE_DELETE) {
                // C++: diffQty = update->m_newQuant;
                diffQty = newQuant;
            } else {
                // C++: diffQty = update->m_oldQuant - update->m_newQuant;
                diffQty = oldQuant - newQuant;
            }

            // C++: if (diffQty > 0)
            if (diffQty > 0) {
                // C++: if (diffQty <= ordStats->m_quantAhead && diffQty > ordStats->m_quantBehind)
                if (diffQty <= ordStats.quantAhead && diffQty > ordStats.quantBehind) {
                    ordStats.quantAhead -= diffQty;
                } else if (diffQty > ordStats.quantAhead && diffQty <= ordStats.quantBehind) {
                    // C++: ordStats->m_quantBehind -= diffQty;
                    ordStats.quantBehind -= diffQty;
                } else {
                    // C++: int32_t behindQty = ((double)ordStats->m_quantBehind / (ordStats->m_quantAhead + ordStats->m_quantBehind)) * diffQty;
                    int behindQty = (int) (((double) ordStats.quantBehind / (ordStats.quantAhead + ordStats.quantBehind)) * diffQty);
                    ordStats.quantBehind -= behindQty;
                    ordStats.quantAhead -= (diffQty - behindQty);
                }
            } else {
                // C++: ordStats->m_quantBehind -= diffQty;
                ordStats.quantBehind -= diffQty;
            }
        }

        // C++: if (update->m_updateType == MDUPDTYPE_ADD)
        if (updateType == Constants.MDUPDTYPE_ADD) {
            // C++: ordStats->m_quantBehind += update->m_newQuant;
            ordStats.quantBehind += newQuant;
        }
    }

    // =======================================================================
    //  订单管理
    // =======================================================================

    /**
     * 发送新订单。
     * 迁移自: ExecutionStrategy::SendNewOrder(TransactionType, double, int32_t, int32_t, TypeOfOrder, OrderHitType)
     * Ref: ExecutionStrategy.cpp:1522-1603
     *
     * @return 新创建的 OrderStats，或 null（重复价格）
     */
    public OrderStats sendNewOrder(byte side, double price, int qty, int orderLevel) {
        return sendNewOrder(side, price, qty, orderLevel, OrderStats.TypeOfOrder.QUOTE, OrderStats.HitType.STANDARD);
    }

    public OrderStats sendNewOrder(byte side, double price, int qty, int orderLevel, OrderStats.HitType ordtype) {
        return sendNewOrder(side, price, qty, orderLevel, OrderStats.TypeOfOrder.QUOTE, ordtype);
    }

    /**
     * 发送新订单（完整参数版本）。
     * 迁移自: ExecutionStrategy::SendNewOrder(TransactionType side, double price, int32_t qty,
     *          int32_t level, TypeOfOrder typeOfOrder, OrderHitType ordtype)
     * Ref: ExecutionStrategy.cpp:1060-1110
     *
     * @param side        买卖方向
     * @param price       价格
     * @param qty         数量
     * @param orderLevel  订单层级
     * @param typeOfOrder 订单类型 (QUOTE/PHEDGE/AHEDGE)
     * @param ordtype     命中类型 (STANDARD/IMPROVE/CROSS/DETECT/MATCH)
     * @return 新创建的 OrderStats，或 null（重复价格）
     */
    public OrderStats sendNewOrder(byte side, double price, int qty, int orderLevel,
                                    OrderStats.TypeOfOrder typeOfOrder, OrderStats.HitType ordtype) {
        Map<Double, OrderStats> priceMap;
        if (side == Constants.SIDE_BUY) {
            // C++: if (m_bidMap.find(price) != m_bidMap.end()) return m_ordMap.end()
            if (bidMap.containsKey(price)) return null;
            priceMap = bidMap;
            buyOpenOrders++;
            buyOpenQty += qty;
        } else {
            if (askMap.containsKey(price)) return null;
            priceMap = askMap;
            sellOpenOrders++;
            sellOpenQty += qty;
        }

        this.level = orderLevel;

        // C++: uint32_t orderID = m_client->SendNewOrder(...)
        int orderID = client.sendNewOrder(strategyID, instru.symbol, side, price, qty,
                Constants.POS_OPEN, this);

        // C++: OrderStats *ordStats = new OrderStats()
        OrderStats ordStats = new OrderStats();
        ordStats.active = false;
        ordStats.isNew = true;
        ordStats.cancel = false;
        ordStats.modifyWait = false;
        ordStats.modify = 0;
        ordStats.status = OrderStats.Status.NEW_ORDER;
        ordStats.price = price;
        ordStats.side = side;
        ordStats.orderID = orderID;
        ordStats.qty = qty;
        ordStats.openQty = qty;
        ordStats.doneQty = 0;
        ordStats.quantBehind = 0;
        ordStats.ordType = ordtype;
        ordStats.typeOfOrder = typeOfOrder;

        // C++: m_quantAhead = (side==BUY) ? (bidPx[level]==price ? bidQty[level] : 0) : ...
        if (side == Constants.SIDE_BUY) {
            ordStats.quantAhead = (instru.bidPx[orderLevel] == price) ? instru.bidQty[orderLevel] : 0;
        } else {
            ordStats.quantAhead = (instru.askPx[orderLevel] == price) ? instru.askQty[orderLevel] : 0;
        }

        ordMap.put(orderID, ordStats);
        priceMap.put(price, ordStats);
        configParams.orderIDStrategyMap.put(orderID, this);

        // 记录新建订单事件
        recordOrderEvent(ordStats, "NEW");

        orderCount++;
        return ordStats;
    }

    /**
     * 修改订单。
     * 迁移自: ExecutionStrategy::SendModifyOrder(...)
     * Ref: ExecutionStrategy.cpp:1650-1711
     */
    public OrderStats sendModifyOrder(int orderID, double price, double oldPx, int qty, int orderLevel, OrderStats.HitType ordtype) {
        OrderStats order = ordMap.get(orderID);
        if (order == null) return null;

        if (order.side == Constants.SIDE_BUY) {
            if (bidMap.containsKey(price) || order.status == OrderStats.Status.MODIFY_ORDER) return null;
        } else {
            if (askMap.containsKey(price) || order.status == OrderStats.Status.MODIFY_ORDER) return null;
        }

        order.status = OrderStats.Status.MODIFY_ORDER;
        order.newPrice = price;
        order.newQty = qty;
        order.ordType = ordtype;

        if (order.side == Constants.SIDE_BUY) {
            bidMap.put(price, order);
            buyOpenQty += (qty - order.qty);
        } else {
            askMap.put(price, order);
            sellOpenQty += (qty - order.qty);
        }

        this.level = orderLevel;
        client.sendModifyOrder(strategyID, instru.symbol, order.side, price, qty, orderID,
                Constants.POS_OPEN, this);

        if (order.modify == 0) {
            order.oldPrice = order.price;
            order.oldQty = order.openQty;
        }
        order.modify++;
        order.modifyWait = true;

        return order;
    }

    /**
     * 按 orderID 撤单。
     * 迁移自: ExecutionStrategy::SendCancelOrder(uint32_t orderID)
     * Ref: ExecutionStrategy.cpp:1767-1801
     */
    public boolean sendCancelOrder(int orderID) {
        OrderStats order = ordMap.get(orderID);
        if (order == null) return false;

        if (order.status == OrderStats.Status.NEW_CONFIRM ||
            order.status == OrderStats.Status.MODIFY_CONFIRM ||
            order.status == OrderStats.Status.MODIFY_REJECT) {
            order.status = OrderStats.Status.CANCEL_ORDER;
            order.cancel = true;
            cancelCount++;
            client.sendCancelOrder(strategyID, instru.symbol, order.side, orderID, this);
            return true;
        }
        return false;
    }

    /**
     * 按价格+方向撤单。
     * 迁移自: ExecutionStrategy::SendCancelOrder(double price, TransactionType side)
     * Ref: ExecutionStrategy.cpp:1713-1765
     */
    public boolean sendCancelOrder(double price, byte side) {
        Map<Double, OrderStats> priceMap = (side == Constants.SIDE_BUY) ? bidMap : askMap;
        OrderStats order = priceMap.get(price);
        if (order != null) {
            if (order.status == OrderStats.Status.NEW_CONFIRM ||
                order.status == OrderStats.Status.MODIFY_CONFIRM ||
                order.status == OrderStats.Status.MODIFY_REJECT) {
                return sendCancelOrder(order.orderID);
            }
        }
        return false;
    }

    // =======================================================================
    //  价格计算 — GetBidPrice / GetAskPrice
    // =======================================================================

    /**
     * 计算买单价格（含 cross/improve 逻辑）。
     * 迁移自: ExecutionStrategy::GetBidPrice(double &price, OrderHitType &ordType, int32_t &level)
     * Ref: ExecutionStrategy.cpp:1225-1309
     *
     * [C++差异] C++ 使用引用参数返回 price/ordType/level，Java 使用 double[1] 包装器。
     *
     * @param priceRef  priceRef[0] = 输出买单价格
     * @param ordTypeRef ordTypeRef[0] = 输出订单类型 (STANDARD/CROSS/IMPROVE)
     * @param levelRef  levelRef[0] = 输入/输出 level
     */
    public void getBidPrice(double[] priceRef, OrderStats.HitType[] ordTypeRef, int[] levelRef) {
        int level = levelRef[0];
        // C++: ordType = STANDARD;
        ordTypeRef[0] = OrderStats.HitType.STANDARD;
        double bidPx, askPx;

        // C++: if (!m_simConfig->m_bUseStratBook)
        if (!simConfig.useStratBook) {
            // C++: price = m_instru->bidPx[level];
            priceRef[0] = instru.bidPx[level];
            // C++: bidPx = m_instru->bidPx[0]; askPx = m_instru->askPx[0];
            bidPx = instru.bidPx[0];
            askPx = instru.askPx[0];
        } else {
            // C++: price = m_instru->bidPxStrat[level];
            priceRef[0] = instru.bidPxStrat[level];
            bidPx = instru.bidPxStrat[0];
            askPx = instru.askPxStrat[0];
        }

        // C++: if (m_configParams->m_bUseInvisibleBook)
        if (configParams.bUseInvisibleBook) {
            // C++: price = bidPx - level * m_instru->m_tickSize;
            priceRef[0] = bidPx - level * instru.tickSize;
        }

        // C++: if (level == 0) — 第0层判断是否可以高于买一价或者卖一价
        if (level == 0) {
            // C++: double bidImprove = m_targetPrice - bidPx - m_instru->m_tickSize;
            double bidImprove = targetPrice - bidPx - instru.tickSize;
            // C++: double bidCross = m_targetPrice - askPx;
            double bidCross = targetPrice - askPx;
            // C++: double spread = (askPx - bidPx) / m_instru->m_tickSize;
            double spread = (askPx - bidPx) / instru.tickSize;

            double bidCrossPNL = 0, bidImprovePNL = 0, bidCloseImprovePNL = 0;
            int bidTicksToImprove = 0;

            // C++: if (((m_netpos < 0) && (bidCross > m_tholdBidPlace + m_thold->CLOSE_CROSS) && (spread <= m_thold->MAX_SHORT_CROSS))
            //      || ((m_netpos >= 0) && (bidCross > m_tholdBidPlace + m_thold->CROSS) && (spread <= m_thold->MAX_LONG_CROSS)))
            if (((netpos < 0) && (bidCross > tholdBidPlace + thold.CLOSE_CROSS) && (spread <= thold.MAX_SHORT_CROSS))
                    || ((netpos >= 0) && (bidCross > tholdBidPlace + thold.CROSS) && (spread <= thold.MAX_LONG_CROSS))) {
                // C++: bidCrossPNL = CalculatePNL(askPx, m_targetPrice);
                bidCrossPNL = calculatePNL(askPx, targetPrice);
                if (bidCrossPNL > 0) {
                    // C++: if (m_thold->CROSS_TICKS > 0)
                    if (thold.CROSS_TICKS > 0) {
                        // C++: price = m_instru->askPx[level] + m_thold->CROSS_TICKS * m_instru->m_tickSize;
                        priceRef[0] = instru.askPx[level] + thold.CROSS_TICKS * instru.tickSize;
                    } else if (thold.CROSS_TARGET == 1) {
                        // C++: price = ((int)(m_targetPrice / m_instru->m_tickSize)) * m_instru->m_tickSize;
                        priceRef[0] = ((int) (targetPrice / instru.tickSize)) * instru.tickSize;
                    } else {
                        // C++: price = askPx;
                        priceRef[0] = askPx;
                    }
                    // C++: ordType = CROSS;
                    ordTypeRef[0] = OrderStats.HitType.CROSS;
                }
            } else {
                // C++: if ((m_netpos < 0) && (m_thold->CLOSE_IMPROVE > 0) && (m_thold->CLOSE_IMPROVE <= 1))
                if ((netpos < 0) && (thold.CLOSE_IMPROVE > 0) && (thold.CLOSE_IMPROVE <= 1)) {
                    // C++: bidTicksToImprove = int((((askPx - bidPx) / m_instru->m_tickSize) * m_thold->CLOSE_IMPROVE) / 2);
                    bidTicksToImprove = (int) ((((askPx - bidPx) / instru.tickSize) * thold.CLOSE_IMPROVE) / 2);
                    if (bidTicksToImprove >= 1) {
                        // C++: if (m_thold->CLOSE_PNL)
                        if (thold.CLOSE_PNL) {
                            // C++: bidCloseImprovePNL = CalculatePNL((bidPx + (bidTicksToImprove * m_instru->m_tickSize)), m_targetPrice);
                            bidCloseImprovePNL = calculatePNL(bidPx + (bidTicksToImprove * instru.tickSize), targetPrice);
                        } else {
                            bidCloseImprovePNL = 1;
                        }
                    }
                }

                // C++: if ((m_netpos < 0) && (bidCloseImprovePNL > 0))
                if ((netpos < 0) && (bidCloseImprovePNL > 0)) {
                    // C++: PriceMapIter iter = m_bidMap.find(price);
                    OrderStats existing = bidMap.get(priceRef[0]);
                    // C++: if ((iter != m_bidMap.end() && iter->second->m_quantAhead > 0) || iter == m_bidMap.end())
                    if ((existing != null && existing.quantAhead > 0) || existing == null) {
                        // C++: price = bidPx + (bidTicksToImprove * m_instru->m_tickSize);
                        priceRef[0] = bidPx + (bidTicksToImprove * instru.tickSize);
                        // C++: ordType = IMPROVE;
                        ordTypeRef[0] = OrderStats.HitType.IMPROVE;
                    }
                } else if ((bidImprove > tholdBidPlace + thold.IMPROVE)) {
                    // C++: bidImprovePNL = CalculatePNL(bidPx + m_instru->m_tickSize, m_targetPrice);
                    bidImprovePNL = calculatePNL(bidPx + instru.tickSize, targetPrice);
                    if (bidImprovePNL > 0) {
                        // C++: PriceMapIter iter = m_bidMap.find(price);
                        OrderStats existing = bidMap.get(priceRef[0]);
                        // C++: if (iter != m_bidMap.end() && iter->second->m_quantAhead > 0)
                        if (existing != null && existing.quantAhead > 0) {
                            // C++: price = bidPx + m_instru->m_tickSize;
                            priceRef[0] = bidPx + instru.tickSize;
                            ordTypeRef[0] = OrderStats.HitType.IMPROVE;
                        }
                    }
                }
            }
        }
    }

    /**
     * 计算卖单价格（含 cross/improve 逻辑）。
     * 迁移自: ExecutionStrategy::GetAskPrice(double &price, OrderHitType &ordType, int32_t &level)
     * Ref: ExecutionStrategy.cpp:1357-1440
     *
     * @param priceRef  priceRef[0] = 输出卖单价格
     * @param ordTypeRef ordTypeRef[0] = 输出订单类型 (STANDARD/CROSS/IMPROVE)
     * @param levelRef  levelRef[0] = 输入/输出 level
     */
    public void getAskPrice(double[] priceRef, OrderStats.HitType[] ordTypeRef, int[] levelRef) {
        int level = levelRef[0];
        // C++: ordType = STANDARD;
        ordTypeRef[0] = OrderStats.HitType.STANDARD;
        double bidPx, askPx;

        // C++: if (!m_simConfig->m_bUseStratBook)
        if (!simConfig.useStratBook) {
            // C++: price = m_instru->askPx[level];
            priceRef[0] = instru.askPx[level];
            bidPx = instru.bidPx[0];
            askPx = instru.askPx[0];
        } else {
            // C++: price = m_instru->askPxStrat[level];
            priceRef[0] = instru.askPxStrat[level];
            bidPx = instru.bidPxStrat[0];
            askPx = instru.askPxStrat[0];
        }

        // C++: if (m_configParams->m_bUseInvisibleBook)
        if (configParams.bUseInvisibleBook) {
            // C++: price = askPx + level * m_instru->m_tickSize;
            priceRef[0] = askPx + level * instru.tickSize;
        }

        // C++: if (level == 0)
        if (level == 0) {
            // C++: double askImprove = askPx - m_instru->m_tickSize - m_targetPrice;
            double askImprove = askPx - instru.tickSize - targetPrice;
            // C++: double askCross = bidPx - m_targetPrice;
            double askCross = bidPx - targetPrice;
            // C++: double spread = (askPx - bidPx) / m_instru->m_tickSize;
            double spread = (askPx - bidPx) / instru.tickSize;

            double askCrossPNL = 0, askImprovePNL = 0, askCloseImprovePNL = 0;
            int askTicksToImprove = 0;

            // C++: if (((m_netpos > 0) && (askCross > m_tholdAskPlace + m_thold->CLOSE_CROSS) && (spread <= m_thold->MAX_SHORT_CROSS))
            //      || ((m_netpos <= 0) && (askCross > m_tholdAskPlace + m_thold->CROSS) && (spread <= m_thold->MAX_LONG_CROSS)))
            if (((netpos > 0) && (askCross > tholdAskPlace + thold.CLOSE_CROSS) && (spread <= thold.MAX_SHORT_CROSS))
                    || ((netpos <= 0) && (askCross > tholdAskPlace + thold.CROSS) && (spread <= thold.MAX_LONG_CROSS))) {
                // C++: askCrossPNL = CalculatePNL(m_targetPrice, bidPx);
                askCrossPNL = calculatePNL(targetPrice, bidPx);
                if (askCrossPNL > 0) {
                    if (thold.CROSS_TICKS > 0) {
                        // C++: price = m_instru->bidPx[level] - m_thold->CROSS_TICKS * m_instru->m_tickSize;
                        priceRef[0] = instru.bidPx[level] - thold.CROSS_TICKS * instru.tickSize;
                    } else if (thold.CROSS_TARGET == 1) {
                        // C++: price = ceil(m_targetPrice / m_instru->m_tickSize) * m_instru->m_tickSize;
                        priceRef[0] = Math.ceil(targetPrice / instru.tickSize) * instru.tickSize;
                    } else {
                        // C++: price = bidPx;
                        priceRef[0] = bidPx;
                    }
                    ordTypeRef[0] = OrderStats.HitType.CROSS;
                }
            } else {
                // C++: if ((m_netpos > 0) && (m_thold->CLOSE_IMPROVE > 0) && (m_thold->CLOSE_IMPROVE <= 1))
                if ((netpos > 0) && (thold.CLOSE_IMPROVE > 0) && (thold.CLOSE_IMPROVE <= 1)) {
                    // C++: askTicksToImprove = int((((askPx - bidPx) / m_instru->m_tickSize) * m_thold->CLOSE_IMPROVE) / 2);
                    askTicksToImprove = (int) ((((askPx - bidPx) / instru.tickSize) * thold.CLOSE_IMPROVE) / 2);
                    if (askTicksToImprove >= 1) {
                        if (thold.CLOSE_PNL) {
                            // C++: askCloseImprovePNL = CalculatePNL(m_targetPrice, askPx - (askTicksToImprove * m_instru->m_tickSize));
                            askCloseImprovePNL = calculatePNL(targetPrice, askPx - (askTicksToImprove * instru.tickSize));
                        } else {
                            askCloseImprovePNL = 1;
                        }
                    }
                }

                // C++: if ((m_netpos > 0) && (askCloseImprovePNL > 0))
                if ((netpos > 0) && (askCloseImprovePNL > 0)) {
                    OrderStats existing = askMap.get(priceRef[0]);
                    // C++: if ((iter != m_askMap.end() && iter->second->m_quantAhead > 0) || iter == m_askMap.end())
                    if ((existing != null && existing.quantAhead > 0) || existing == null) {
                        // C++: price = askPx - (askTicksToImprove * m_instru->m_tickSize);
                        priceRef[0] = askPx - (askTicksToImprove * instru.tickSize);
                        ordTypeRef[0] = OrderStats.HitType.IMPROVE;
                    }
                } else if ((askImprove > tholdAskPlace + thold.IMPROVE)) {
                    // C++: askImprovePNL = CalculatePNL(m_targetPrice, askPx - m_instru->m_tickSize);
                    askImprovePNL = calculatePNL(targetPrice, askPx - instru.tickSize);
                    if (askImprovePNL > 0) {
                        OrderStats existing = askMap.get(priceRef[0]);
                        if (existing != null && existing.quantAhead > 0) {
                            // C++: price = askPx - m_instru->m_tickSize;
                            priceRef[0] = askPx - instru.tickSize;
                            ordTypeRef[0] = OrderStats.HitType.IMPROVE;
                        }
                    }
                }
            }
        }
    }

    // =======================================================================
    //  高层发单 — SendBidOrder / SendAskOrder
    // =======================================================================

    /**
     * 发送买单（含量计算 + 价格计算 + 新/改单分发）。
     * 迁移自: ExecutionStrategy::SendBidOrder(RequestType reqType, int32_t level, double price, OrderHitType ordType, uint32_t ordID, double oldPx)
     * Ref: ExecutionStrategy.cpp:1311-1355
     *
     * @param reqType  请求类型: Constants.REQUEST_NEWORDER 或 Constants.REQUEST_MODIFYORDER
     * @param level    档位
     * @param price    指定价格（0 = 自动计算）
     * @param ordType  订单类型
     * @param ordID    改单时的原订单 ID
     * @param oldPx    改单时的原价格
     */
    public void sendBidOrder(int reqType, int level, double price, OrderStats.HitType ordType, int ordID, double oldPx) {
        // C++: int32_t qty = m_tholdSize, avgQty = 0;
        int qty = tholdSize;
        int avgQtyVal = 0;

        // C++: if (m_thold->USE_PERCENT == true)
        if (thold.USE_PERCENT) {
            // C++: int32_t lotSize = m_instru->m_sendInLots ? 1 : m_instru->m_lotSize;
            int lotSize = instru.sendInLots ? 1 : (int) instru.lotSize;
            // C++: avgQty = ((int32_t)(((m_avgQty / lotSize) * m_thold->PERCENT_SIZE) / 100) + 1) * lotSize;
            avgQtyVal = ((int) (((avgQty / lotSize) * thold.PERCENT_SIZE) / 100) + 1) * lotSize;
            // C++: qty = qty > avgQty ? avgQty : qty;
            qty = Math.min(qty, avgQtyVal);
            // C++: m_tholdMaxPos = m_smsRatio * qty;
            tholdMaxPos = smsRatio * qty;
        }

        // C++: if (m_netpos > 0)
        if (netpos > 0) {
            // C++: qty = m_netpos < qty ? qty - m_netpos : qty;
            qty = netpos < qty ? qty - netpos : qty;
            // C++: qty = m_tholdMaxPos - m_netpos < qty ? m_tholdMaxPos - m_netpos : qty;
            qty = Math.min(qty, tholdMaxPos - netpos);
        }

        // C++: if (m_netpos < 0)
        if (netpos < 0) {
            // C++: if (m_thold->QUOTE_MAX_QTY) qty = -1 * m_netpos;
            if (thold.QUOTE_MAX_QTY) {
                qty = -1 * netpos;
            } else {
                // C++: qty = -1 * m_netpos > qty ? qty : -1 * m_netpos;
                qty = Math.min(qty, -1 * netpos);
            }
        }

        // C++: if (qty > 0)
        if (qty > 0) {
            // C++: if (price == 0) GetBidPrice(price, ordType, level);
            if (price == 0) {
                double[] priceRef = {price};
                OrderStats.HitType[] ordTypeRef = {ordType};
                int[] levelRef = {level};
                getBidPrice(priceRef, ordTypeRef, levelRef);
                price = priceRef[0];
                ordType = ordTypeRef[0];
                level = levelRef[0];
            }

            // C++: if (price == 0) { TBLOG << "No Bid Order being send as price is set to 0\n"; return; }
            if (price == 0) {
                log.fine("No Bid Order being sent as price is set to 0");
                return;
            }

            // C++: SendCancelOrder(price, SELL);
            sendCancelOrder(price, Constants.SIDE_SELL);

            // C++: if (reqType == NEWORDER) SendNewOrder(BUY, price, qty, level, QUOTE, ordType);
            if (reqType == Constants.REQUEST_NEWORDER) {
                sendNewOrder(Constants.SIDE_BUY, price, qty, level, ordType);
            } else {
                // C++: SendModifyOrder(ordID, price, oldPx, qty, level, QUOTE, ordType);
                sendModifyOrder(ordID, price, oldPx, qty, level, ordType);
            }
        }
    }

    /**
     * 发送卖单（含量计算 + 价格计算 + 新/改单分发）。
     * 迁移自: ExecutionStrategy::SendAskOrder(RequestType reqType, int32_t level, double price, OrderHitType ordType, uint32_t ordID, double oldPx)
     * Ref: ExecutionStrategy.cpp:1442-1485
     *
     * @param reqType  请求类型
     * @param level    档位
     * @param price    指定价格（0 = 自动计算）
     * @param ordType  订单类型
     * @param ordID    改单时的原订单 ID
     * @param oldPx    改单时的原价格
     */
    public void sendAskOrder(int reqType, int level, double price, OrderStats.HitType ordType, int ordID, double oldPx) {
        // C++: int32_t qty = m_tholdSize, avgQty = 0;
        int qty = tholdSize;
        int avgQtyVal = 0;

        // C++: if (m_thold->USE_PERCENT == true)
        if (thold.USE_PERCENT) {
            int lotSize = instru.sendInLots ? 1 : (int) instru.lotSize;
            // C++: avgQty = ((int32_t)(((m_avgQty / lotSize) * m_thold->PERCENT_SIZE) / 100) + 1) * lotSize;
            avgQtyVal = ((int) (((avgQty / lotSize) * thold.PERCENT_SIZE) / 100) + 1) * lotSize;
            qty = Math.min(qty, avgQtyVal);
            // C++: m_tholdMaxPos = m_smsRatio * qty;
            tholdMaxPos = smsRatio * qty;
        }

        // C++: if (m_netpos < 0)
        if (netpos < 0) {
            // C++: qty = m_netpos > -1 * qty ? qty + m_netpos : qty;
            qty = netpos > -1 * qty ? qty + netpos : qty;
            // C++: qty = m_tholdMaxPos - m_netpos * -1 < qty ? m_tholdMaxPos - m_netpos * -1 : qty;
            qty = Math.min(qty, tholdMaxPos - (-1 * netpos));
        }

        // C++: if (m_netpos > 0)
        if (netpos > 0) {
            if (thold.QUOTE_MAX_QTY) {
                // C++: qty = m_netpos;
                qty = netpos;
            } else {
                // C++: qty = m_netpos > qty ? qty : m_netpos;
                qty = Math.min(qty, netpos);
            }
        }

        // C++: if (qty > 0)
        if (qty > 0) {
            // C++: if (price == 0) GetAskPrice(price, ordType, level);
            if (price == 0) {
                double[] priceRef = {price};
                OrderStats.HitType[] ordTypeRef = {ordType};
                int[] levelRef = {level};
                getAskPrice(priceRef, ordTypeRef, levelRef);
                price = priceRef[0];
                ordType = ordTypeRef[0];
                level = levelRef[0];
            }

            if (price == 0) {
                log.fine("No Ask Order being sent as price is set to 0");
                return;
            }

            // C++: SendCancelOrder(price, BUY);
            sendCancelOrder(price, Constants.SIDE_BUY);

            if (reqType == Constants.REQUEST_NEWORDER) {
                // C++: SendNewOrder(SELL, price, qty, level, QUOTE, ordType);
                sendNewOrder(Constants.SIDE_SELL, price, qty, level, ordType);
            } else {
                sendModifyOrder(ordID, price, oldPx, qty, level, ordType);
            }
        }
    }

    // =======================================================================
    //  订单回调处理
    // =======================================================================

    /**
     * 成交处理。
     * 迁移自: ExecutionStrategy::ProcessTrade(ResponseMsg*, OrderMapIter)
     * Ref: ExecutionStrategy.cpp:1983-2122
     */
    protected void processTrade(MemorySegment response, OrderStats order) {
        int tradeQty = (int) Types.RESP_QUANTITY_VH.get(response, 0L);
        double tradePrice = (double) Types.RESP_PRICE_VH.get(response, 0L);

        order.openQty -= tradeQty;
        order.doneQty += tradeQty;

        lastTrade = true;
        lastTradePx = tradePrice;

        // C++: 更新买卖量/值
        if (order.side == Constants.SIDE_BUY) {
            lastTradeSide = true;
            buyTotalValue += tradePrice * tradeQty;
            buyValue += tradePrice * tradeQty;
            buyTotalQty += tradeQty;
            buyAvgPrice = buyTotalValue / buyTotalQty;
            buyQty += tradeQty;
            buyPrice = buyValue / buyQty;
            buyOpenQty -= tradeQty;
        } else {
            lastTradeSide = false;
            sellTotalValue += tradePrice * tradeQty;
            sellValue += tradePrice * tradeQty;
            sellTotalQty += tradeQty;
            sellAvgPrice = sellTotalValue / sellTotalQty;
            sellQty += tradeQty;
            sellPrice = sellValue / sellQty;
            sellOpenQty -= tradeQty;
        }

        // C++: 更新 netposPass / netposAgg
        if (order.ordType == OrderStats.HitType.IMPROVE) {
            improveCount++;
        } else if (order.ordType == OrderStats.HitType.CROSS) {
            crossCount++;
            if (order.side == Constants.SIDE_BUY) netposAgg += tradeQty; else netposAgg -= tradeQty;
        } else if (order.ordType == OrderStats.HitType.STANDARD) {
            if (order.side == Constants.SIDE_BUY) netposPass += tradeQty; else netposPass -= tradeQty;
        } else if (order.ordType == OrderStats.HitType.MATCH) {
            if (order.side == Constants.SIDE_BUY) netposAgg += tradeQty; else netposAgg -= tradeQty;
        }

        tradeCount++;
        netpos = (int)(buyTotalQty - sellTotalQty);

        // C++: 手续费计算
        transValue = (buyExchTx * buyValue + sellExchTx * sellValue) * instru.priceMultiplier
                   + (buyExchContractTx * buyQty + sellExchContractTx * sellQty);

        // C++: netpos == 0 时结算
        if (netpos == 0) {
            transTotalValue = (buyExchTx * buyTotalValue + sellExchTx * sellTotalValue) * instru.priceMultiplier
                            + (buyExchContractTx * buyTotalQty + sellExchContractTx * sellTotalQty);
            realisedPNL = (sellTotalValue - buyTotalValue) * instru.priceMultiplier;
            buyValue = 0; buyQty = 0; buyPrice = 0;
            sellValue = 0; sellQty = 0; sellPrice = 0;
            transValue = 0;
        }

        calculatePNL();

        // C++: 全部成交时移除订单
        if (order.openQty == 0) {
            if (order.status == OrderStats.Status.MODIFY_ORDER) {
                processModifyReject(response, order);
            }
            order.status = OrderStats.Status.TRADED;
            // 记录成交事件（在 removeOrder 之前，此时 order 数据完整）
            recordOrderEvent(order, "TRADED");
            removeOrder(order);
        }

        onTradeUpdate();
    }

    /**
     * 成交后回调钩子（子类可覆盖）。
     * 迁移自: ExecutionStrategy::OnTradeUpdate() {} (default no-op)
     */
    protected void onTradeUpdate() {}

    /**
     * 自成交检测处理 — 当成交价超出 bid/ask 价差时重建策略订单簿。
     * 迁移自: ExecutionStrategy::ProcessSelfTrade(ResponseMsg *response)
     * Ref: ExecutionStrategy.cpp:835-949
     *
     * 逻辑:
     * 1. 成交价 < bidPx[0] 或 > askPx[0] 时触发
     * 2. 首次触发时复制市场订单簿到策略订单簿 (bidPxStrat/askPxStrat)
     * 3. 根据成交价方向重建订单簿：
     *    - 成交价 < bidPxStrat[0]: 移除高于成交价的 bid 档位，填充剩余档位
     *    - 成交价 > askPxStrat[0]: 移除低于成交价的 ask 档位，填充剩余档位
     * 4. 设置 useStratBook=true，后续价格计算使用策略订单簿
     *
     * @param tradePrice 成交价格（来自 ResponseMsg.Price）
     */
    public void processSelfTrade(double tradePrice) {
        // C++: if (response->Price < m_instru->bidPx[0] || response->Price > m_instru->askPx[0])
        if (tradePrice < instru.bidPx[0] || tradePrice > instru.askPx[0]) {
            log.info("TRADE: " + tradePrice + " " + instru.bidPx[0] + " " + instru.askPx[0]
                    + " " + instru.validBids + " " + instru.validAsks);

            double fillprice = 0;

            // C++: if (!m_simConfig->m_bUseStratBook) — 首次触发时复制订单簿
            if (!simConfig.useStratBook) {
                // C++: memcpy(m_instru->bidPxStrat, m_instru->bidPx, ...)
                System.arraycopy(instru.bidPx, 0, instru.bidPxStrat, 0, instru.validBids);
                System.arraycopy(instru.bidQty, 0, instru.bidQtyStrat, 0, instru.validBids);
                System.arraycopy(instru.bidOrderCount, 0, instru.bidOrderCountStrat, 0, instru.validBids);
                System.arraycopy(instru.askPx, 0, instru.askPxStrat, 0, instru.validAsks);
                System.arraycopy(instru.askQty, 0, instru.askQtyStrat, 0, instru.validAsks);
                System.arraycopy(instru.askOrderCount, 0, instru.askOrderCountStrat, 0, instru.validAsks);
            }

            // C++: m_simConfig->m_bUseStratBook = true;
            simConfig.useStratBook = true;

            int i;

            // C++: if (response->Price < m_instru->bidPxStrat[0])
            if (tradePrice < instru.bidPxStrat[0]) {
                // C++: for (i = 0; i < m_validBids && response->Price < bidPxStrat[i]; i++) {}
                for (i = 0; i < instru.validBids && tradePrice < instru.bidPxStrat[i]; i++) {
                }

                // C++: if (i != 0) { memmove(...) }
                if (i != 0) {
                    System.arraycopy(instru.bidQtyStrat, i, instru.bidQtyStrat, 0, instru.validBids - i);
                    System.arraycopy(instru.bidPxStrat, i, instru.bidPxStrat, 0, instru.validBids - i);
                    System.arraycopy(instru.bidOrderCountStrat, i, instru.bidOrderCountStrat, 0, instru.validBids - i);
                }

                // C++: if (m_validBids == i) — 所有 bid 都高于成交价
                if (instru.validBids == i) {
                    instru.bidPxStrat[0] = tradePrice;
                    instru.bidQtyStrat[0] = 1;
                    instru.bidOrderCountStrat[0] = 1;
                    fillprice = tradePrice + instru.tickSize;
                } else {
                    // C++: fillprice = bidPxStrat[m_validBids - i - 1];
                    fillprice = instru.bidPxStrat[instru.validBids - i - 1];
                }

                // C++: for (int j = m_validBids - i; j < m_level; j++)
                for (int j = instru.validBids - i; j < instru.level; j++) {
                    fillprice -= instru.tickSize;
                    instru.bidPxStrat[j] = fillprice;
                    instru.bidQtyStrat[j] = 1;
                    instru.bidOrderCountStrat[j] = 1;
                }
                // C++: m_validBids = m_level;
                instru.validBids = instru.level;

                // C++: memmove(askQtyStrat + 1, askQtyStrat, ...) — 右移 ask
                System.arraycopy(instru.askQtyStrat, 0, instru.askQtyStrat, 1, instru.validAsks - 1);
                System.arraycopy(instru.askPxStrat, 0, instru.askPxStrat, 1, instru.validAsks - 1);
                System.arraycopy(instru.askOrderCountStrat, 0, instru.askOrderCountStrat, 1, instru.validAsks - 1);

                // C++: askPxStrat[0] = response->Price + m_tickSize;
                instru.askPxStrat[0] = tradePrice + instru.tickSize;
                instru.askQtyStrat[0] = 1;
                instru.askOrderCountStrat[0] = 1;
            }

            // C++: if (response->Price > m_instru->askPxStrat[0])
            if (tradePrice > instru.askPxStrat[0]) {
                // C++: for (i = 0; i < m_validAsks && response->Price > askPxStrat[i]; i++) {}
                for (i = 0; i < instru.validAsks && tradePrice > instru.askPxStrat[i]; i++) {
                }

                if (i != 0) {
                    System.arraycopy(instru.askQtyStrat, i, instru.askQtyStrat, 0, instru.validAsks - i);
                    System.arraycopy(instru.askPxStrat, i, instru.askPxStrat, 0, instru.validAsks - i);
                    System.arraycopy(instru.askOrderCountStrat, i, instru.askOrderCountStrat, 0, instru.validAsks - i);
                }

                if (instru.validAsks == i) {
                    instru.askPxStrat[0] = tradePrice;
                    instru.askQtyStrat[0] = 1;
                    instru.askOrderCountStrat[0] = 1;
                    fillprice = tradePrice - instru.tickSize;
                } else {
                    fillprice = instru.askPxStrat[instru.validAsks - i - 1];
                }

                for (int j = instru.validAsks - i; j < instru.level; j++) {
                    fillprice += instru.tickSize;
                    instru.askPxStrat[j] = fillprice;
                    instru.askQtyStrat[j] = 1;
                    instru.askOrderCountStrat[j] = 1;
                }
                instru.validAsks = instru.level;

                // C++: memmove(bidQtyStrat + 1, bidQtyStrat, ...) — 右移 bid
                System.arraycopy(instru.bidQtyStrat, 0, instru.bidQtyStrat, 1, instru.validBids - 1);
                System.arraycopy(instru.bidPxStrat, 0, instru.bidPxStrat, 1, instru.validBids - 1);
                System.arraycopy(instru.bidOrderCountStrat, 0, instru.bidOrderCountStrat, 1, instru.validBids - 1);
                instru.bidPxStrat[0] = tradePrice - instru.tickSize;
                instru.bidQtyStrat[0] = 1;
                instru.bidOrderCountStrat[0] = 1;
            }

            // C++: m_instru->CalculateStratPrices();
            // [C++差异] CalculateStratPrices 和 Indicator 回调未迁移（依赖指标系统）
            // C++: m_simConfig->m_indicatorList.begin()->m_indicator->OrderBookStratUpdate(...)
            // C++: m_simConfig->m_calculatePNL->CalculateTargetPNL(...)
            // C++: SetTargetValue(m_currPrice, m_targetPrice, m_targetBidPNL, m_targetAskPNL);

            onTradeUpdate();

            // C++: if (retVal && !m_onFlat && m_Active) SendOrder();
            if (!onFlat && active) {
                sendOrder();
            }
        }
    }

    /**
     * 移除订单。
     * 迁移自: ExecutionStrategy::RemoveOrder(OrderMapIter&)
     * Ref: ExecutionStrategy.cpp:1175-1213
     */
    protected void removeOrder(OrderStats order) {
        if (order.side == Constants.SIDE_BUY) {
            bidMap.remove(order.price);
            buyOpenOrders--;
        } else {
            askMap.remove(order.price);
            sellOpenOrders--;
        }
        ordMap.remove(order.orderID);
    }

    /** 新单被拒。 Ref: ExecutionStrategy.cpp:1803-1829 */
    protected void processNewReject(MemorySegment response, OrderStats order) {
        recordOrderEvent(order, "NEW_REJECT");
        removeOrder(order);
    }

    /** 改单被拒。 Ref: ExecutionStrategy.cpp:1831-1853 */
    protected void processModifyReject(MemorySegment response, OrderStats order) {
        // C++: 如果有新价格映射，移除
        if (order.newPrice != 0) {
            if (order.side == Constants.SIDE_BUY) {
                bidMap.remove(order.newPrice);
            } else {
                askMap.remove(order.newPrice);
            }
        }
        order.modifyWait = false;
    }

    /** 撤单被拒。 Ref: ExecutionStrategy.cpp:1855-1886 */
    protected void processCancelReject(MemorySegment response, OrderStats order) {
        order.cancel = false;
    }

    /** 改单确认。 Ref: ExecutionStrategy.cpp:1888-1910 */
    protected void processModifyConfirm(MemorySegment response, OrderStats order) {
        double newPrice = (double) Types.RESP_PRICE_VH.get(response, 0L);
        int newQty = (int) Types.RESP_QUANTITY_VH.get(response, 0L);

        // C++: 移除旧价格映射
        if (order.side == Constants.SIDE_BUY) {
            bidMap.remove(order.price);
        } else {
            askMap.remove(order.price);
        }

        order.price = newPrice;
        order.qty = newQty;
        order.openQty = newQty - order.doneQty;
        order.modifyWait = false;
        order.modify = 0;
    }

    /** 撤单确认。 Ref: ExecutionStrategy.cpp:1912-1981 */
    protected void processCancelConfirm(MemorySegment response, OrderStats order) {
        order.status = OrderStats.Status.CANCEL_CONFIRM;
        recordOrderEvent(order, "CANCEL_CONFIRM");
        removeOrder(order);
    }

    // =======================================================================
    //  PNL 计算
    // =======================================================================

    /**
     * 计算未实现PNL、总PNL、净PNL和回撤。
     * 迁移自: ExecutionStrategy::CalculatePNL()
     * Ref: ExecutionStrategy.cpp:2124-2148
     */
    public void calculatePNL() {
        // C++: m_unrealisedPNL = netpos>0 ? netpos*((bidPx[0]-buyPrice-bidPx[0]*sellExchTx)*priceMultiplier - sellExchContractTx) : ...
        // [C++差异] C++ 中 netpos 来自昨仓时 buyPrice/sellPrice=0，PNL 公式会产生巨大虚假值。
        // C++ 生产中未触发是因为 MAX_LOSS 阈值较大或行情快速到齐。
        // Java 需要防护：当 netpos!=0 但无今日交易(buyQty==0 && sellQty==0)时，
        // 使用当前市价作为成本基准，使 unrealisedPNL 接近 0。
        double effectiveBuyPrice = buyPrice;
        double effectiveSellPrice = sellPrice;
        if (netpos != 0 && buyQty == 0 && sellQty == 0) {
            // 昨仓 netpos，今日无交易，用当前价作为基准避免虚假 PNL
            if (netpos > 0 && buyPrice == 0) {
                effectiveBuyPrice = instru.bidPx[0];
            } else if (netpos < 0 && sellPrice == 0) {
                effectiveSellPrice = instru.askPx[0];
            }
        }
        if (netpos > 0) {
            unrealisedPNL = netpos * ((instru.bidPx[0] - effectiveBuyPrice - instru.bidPx[0] * sellExchTx) * instru.priceMultiplier - sellExchContractTx);
        } else if (netpos < 0) {
            unrealisedPNL = -1 * netpos * ((effectiveSellPrice - instru.askPx[0] - instru.askPx[0] * buyExchTx) * instru.priceMultiplier - buyExchContractTx);
        } else {
            unrealisedPNL = 0;
        }

        double qty = netpos > 0 ? sellQty : buyQty;
        unrealisedPNL += (qty * (effectiveSellPrice - effectiveBuyPrice) * instru.priceMultiplier);
        unrealisedPNL -= transValue;

        grossPNL = realisedPNL + unrealisedPNL;
        netPNL = grossPNL - transTotalValue;

        if (netPNL > maxPNL) maxPNL = netPNL;
        drawdown = netPNL - maxPNL;
    }

    /**
     * 计算两价格间的 PNL（双参数重载）。
     * 迁移自: ExecutionStrategy::CalculatePNL(double buyprice, double sellprice)
     * Ref: ExecutionStrategy.cpp:1215-1223
     *
     * @param buyprice 买入价
     * @param sellprice 卖出价
     * @return PNL 值
     */
    public double calculatePNL(double buyprice, double sellprice) {
        double pnl = 0;
        // C++: if (m_instru->m_perYield)
        //          PNL = (BondPrice(buyprice, m_instru->m_cDays) - BondPrice(sellprice, m_instru->m_cDays)) - (m_buyExchTx + m_sellExchTx);
        //      else
        //          PNL = (sellprice - buyprice - m_buyExchTx * buyprice - m_sellExchTx * sellprice) * m_instru->m_priceMultiplier - (m_buyExchContractTx + m_sellExchContractTx);
        // Ref: ExecutionStrategy.cpp:1215-1223
        if (instru.perYield) {
            // C++: PNL = (BondPrice(buyprice, m_instru->m_cDays) - BondPrice(sellprice, m_instru->m_cDays)) - (m_buyExchTx + m_sellExchTx);
            pnl = (Instrument.bondPrice(buyprice, instru.cDays) - Instrument.bondPrice(sellprice, instru.cDays))
                    - (buyExchTx + sellExchTx);
        } else {
            // C++: PNL = (sellprice - buyprice - m_buyExchTx * buyprice - m_sellExchTx * sellprice) * m_instru->m_priceMultiplier - (m_buyExchContractTx + m_sellExchContractTx);
            pnl = (sellprice - buyprice - buyExchTx * buyprice - sellExchTx * sellprice) * instru.priceMultiplier
                    - (buyExchContractTx + sellExchContractTx);
        }
        return pnl;
    }

    // =======================================================================
    //  风控
    // =======================================================================

    /**
     * 执行平仓。
     * 迁移自: ExecutionStrategy::HandleSquareoff()
     * Ref: ExecutionStrategy.cpp:2355-2437
     *
     * 逻辑:
     * 1. netpos==0 且 onExit 且无挂单 → 停止策略
     * 2. aggFlat 时用对手价穿越(bid-tick/ask+tick)，否则用被动价
     * 3. DI1F/DI1N 薄订单簿检测 → 即使非 aggFlat 也用激进价
     * 4. 撤销价格不利的挂单（含 optionStrategy 逻辑）
     * 5. onCancel = false
     * 6. qty = abs(netpos), 受 rmsQty 上限限制
     * 7. 无挂单时发送平仓订单（aggFlat 用 CROSS 类型）
     */
    public void handleSquareoff() {
        // C++: if (m_netpos == 0 && m_onExit && m_askMap.size() == 0 && m_bidMap.size() == 0)
        // Ref: ExecutionStrategy.cpp:2361-2370
        if (netpos == 0 && onExit && askMap.isEmpty() && bidMap.isEmpty()) {
            if (onExit && active) {
                log.warning(instru.currDate + " Positions Closed. Strategy Exiting.."
                        + " Symbol: " + instru.origBaseName);
                active = false;
            }
        }

        // C++: double sellprice = m_aggFlat == true ? m_instru->bidPx[0] - m_instru->m_tickSize : m_instru->askPx[0];
        // C++: double buyprice = m_aggFlat == true ? m_instru->askPx[0] + m_instru->m_tickSize : m_instru->bidPx[0];
        // Ref: ExecutionStrategy.cpp:2372-2373
        double sellprice = aggFlat ? instru.bidPx[0] - instru.tickSize : instru.askPx[0];
        double buyprice = aggFlat ? instru.askPx[0] + instru.tickSize : instru.bidPx[0];

        // C++: Go aggressive if the book is thin (DI1F/DI1N)
        // Ref: ExecutionStrategy.cpp:2375-2383
        if (!aggFlat && (instru.symbol.startsWith("DI1F") || instru.symbol.startsWith("DI1N"))) {
            if ((instru.bidQty[0] < thold.AGGFLAT_BOOKSIZE
                    && instru.bidQty[0] / instru.askQty[0] < thold.AGGFLAT_BOOKFRAC)
                    || (int) (instru.askPx[0] - instru.bidPx[0] + 0.1 * instru.tickSize) > 0) {
                sellprice = instru.askPx[0] - instru.tickSize;
            }
            if ((instru.askQty[0] < thold.AGGFLAT_BOOKSIZE
                    && instru.askQty[0] / instru.bidQty[0] < thold.AGGFLAT_BOOKFRAC)
                    || (int) (instru.askPx[0] - instru.bidPx[0] + 0.1 * instru.tickSize) > 0) {
                buyprice = instru.bidPx[0] + instru.tickSize;
            }
        }

        // C++: sellprice = sellprice <= 0 ? m_instru->bidPx[0] : sellprice;
        // C++: buyprice = buyprice <= 0 ? m_instru->askPx[0] : buyprice;
        // Ref: ExecutionStrategy.cpp:2384-2385
        sellprice = sellprice <= 0 ? instru.bidPx[0] : sellprice;
        buyprice = buyprice <= 0 ? instru.askPx[0] : buyprice;

        // C++: 撤销价格不利的挂单
        // Ref: ExecutionStrategy.cpp:2389-2406
        if (!askMap.isEmpty() || !bidMap.isEmpty()) {
            // C++: for (PriceMapIter iter = m_askMap.begin(); ...)
            for (OrderStats order : new ArrayList<>(askMap.values())) {
                // C++: if (m_onCancel || (!m_optionStrategy && sellprice < iter->second->m_price)
                //      || (m_optionStrategy && sellprice != iter->second->m_price) || m_netpos == 0)
                if (onCancel
                        || (!optionStrategy && sellprice < order.price)
                        || (optionStrategy && sellprice != order.price)
                        || netpos == 0) {
                    sendCancelOrder(order.orderID);
                }
            }

            // C++: for (PriceMapIter iter = m_bidMap.begin(); ...)
            for (OrderStats order : new ArrayList<>(bidMap.values())) {
                // C++: if (m_onCancel || (!m_optionStrategy && buyprice > iter->second->m_price)
                //      || (m_optionStrategy && buyprice != iter->second->m_price) || m_netpos == 0)
                if (onCancel
                        || (!optionStrategy && buyprice > order.price)
                        || (optionStrategy && buyprice != order.price)
                        || netpos == 0) {
                    sendCancelOrder(order.orderID);
                }
            }
        }

        // C++: m_onCancel = false;
        // Ref: ExecutionStrategy.cpp:2408
        onCancel = false;

        // C++: int32_t qty = m_netpos > 0 ? m_netpos : -1 * m_netpos;
        // C++: qty = ((m_rmsQty != 0) && (qty > m_rmsQty)) ? m_rmsQty : qty;
        // Ref: ExecutionStrategy.cpp:2417-2418
        int qty = netpos > 0 ? netpos : -netpos;
        qty = (rmsQty != 0 && qty > rmsQty) ? rmsQty : qty;

        // C++: if (m_askMap.size() == 0 && m_bidMap.size() == 0)
        // Ref: ExecutionStrategy.cpp:2420-2436
        // [C++差异] 防御性守卫：active=false 时禁止发送平仓订单。
        // C++ 中此路径在 PairwiseArb 场景下也会执行（通过 CheckSquareoff → HandleSquareoff 链），
        // 但 C++ 不会在 endTime 之后启动策略，所以此 bug 不会在 C++ 中触发。
        // Java 中策略可能在 endTime 之后启动（等待手动激活），必须阻止未激活时发单。
        if (askMap.isEmpty() && bidMap.isEmpty() && active) {
            if (netpos > 0) {
                // C++: if (m_aggFlat) SendNewOrder(SELL, sellprice, qty, 0, QUOTE, CROSS);
                //      else SendNewOrder(SELL, sellprice, qty, 0);
                if (aggFlat) {
                    sendNewOrder(Constants.SIDE_SELL, sellprice, qty, 0, OrderStats.TypeOfOrder.QUOTE, OrderStats.HitType.CROSS);
                } else {
                    sendNewOrder(Constants.SIDE_SELL, sellprice, qty, 0);
                }
            } else if (netpos < 0) {
                // C++: if (m_aggFlat) SendNewOrder(BUY, buyprice, qty, 0, QUOTE, CROSS);
                //      else SendNewOrder(BUY, buyprice, qty, 0);
                if (aggFlat) {
                    sendNewOrder(Constants.SIDE_BUY, buyprice, qty, 0, OrderStats.TypeOfOrder.QUOTE, OrderStats.HitType.CROSS);
                } else {
                    sendNewOrder(Constants.SIDE_BUY, buyprice, qty, 0);
                }
            }
        } else if (askMap.isEmpty() && bidMap.isEmpty() && !active && netpos != 0) {
            log.warning("[GUARD] handleSquareoff: active=false, 跳过发送平仓订单."
                    + " netpos=" + netpos + " symbol=" + (instru != null ? instru.origBaseName : "null"));
        }
    }

    /**
     * 时间限制平仓 — 到达时间限制后激进平仓。
     * 迁移自: ExecutionStrategy::HandleTimeLimitSquareoff()
     * Ref: ExecutionStrategy.cpp:2442-2506
     *
     * 逻辑：
     * 1. netpos==0 直接返回
     * 2. SQROFF_AGG!=0 时用对手价（激进平仓）
     * 3. 先取消价格不利的挂单
     * 4. 无挂单时发送新的平仓订单
     */
    public void handleTimeLimitSquareoff() {
        // C++: if (m_netpos == 0) return;
        if (netpos == 0) {
            log.info("[" + instru.origBaseName + "] Positions Closed due to last traded time");
            return;
        }

        // C++: double sellprice = m_instru->askPx[0]; double buyprice = m_instru->bidPx[0];
        double sellprice = instru.askPx[0];
        double buyprice = instru.bidPx[0];

        // C++: if (m_thold->SQROFF_AGG != 0) { sellprice = bidPx[0]; buyprice = askPx[0]; }
        if (thold.SQROFF_AGG != 0) {
            sellprice = instru.bidPx[0];
            buyprice = instru.askPx[0];
        }

        // C++: if (m_askMap.size() != 0 || m_bidMap.size() != 0) — 取消价格不利的挂单
        if (!askMap.isEmpty() || !bidMap.isEmpty()) {
            // C++: for (PriceMapIter iter = m_askMap.begin(); ...)
            for (OrderStats order : new ArrayList<>(askMap.values())) {
                // C++: if (sellprice < iter->second->m_price || m_netpos == 0)
                if (sellprice < order.price || netpos == 0) {
                    sendCancelOrder(order.orderID);
                }
            }

            // C++: for (PriceMapIter iter = m_bidMap.begin(); ...)
            for (OrderStats order : new ArrayList<>(bidMap.values())) {
                // C++: if (buyprice > iter->second->m_price || m_netpos == 0)
                if (buyprice > order.price || netpos == 0) {
                    sendCancelOrder(order.orderID);
                }
            }
        }

        // C++: int32_t qty = 0;
        int qty = 0;
        // C++: if (m_netpos > 0) { qty = m_netpos > m_tholdBeginPos ? m_tholdBeginPos : m_netpos; }
        if (netpos > 0) {
            qty = Math.min(netpos, tholdBeginPos);
        } else {
            // C++: qty = -m_netpos > m_tholdBeginPos ? m_tholdBeginPos : -m_netpos;
            qty = Math.min(-netpos, tholdBeginPos);
        }

        // C++: if (m_askMap.size() == 0 && m_bidMap.size() == 0)
        if (askMap.isEmpty() && bidMap.isEmpty()) {
            if (netpos > 0) {
                // C++: SendNewOrder(SELL, sellprice, qty, 0);
                sendNewOrder(Constants.SIDE_SELL, sellprice, qty, 0);
            } else if (netpos < 0) {
                // C++: SendNewOrder(BUY, buyprice, qty, 0);
                sendNewOrder(Constants.SIDE_BUY, buyprice, qty, 0);
            }
        }
    }

    // =======================================================================
    //  辅助方法
    // =======================================================================

    /**
     * RoundWorse — 向不利方向取整。
     * 迁移自: ExecutionStrategy::RoundWorse()
     * Ref: ExecutionStrategy.cpp:2343-2353
     */
    /**
     * 填充请求消息固定字段。
     * 迁移自: ExecutionStrategy::fillFixedFields(Instrument *instrument)
     * Ref: ExecutionStrategy.cpp:1487-1520
     *
     * [C++差异] C++ 直接操作 RequestMsg 结构体的内存字段（Token, AccountID, Symbol 等），
     * Java 版本中 CommonClient.sendNewOrder() 已封装了请求字段构建逻辑，
     * 因此此方法仅更新 instru 引用。原 C++ 中此方法在构造函数中被调用（已注释掉）。
     *
     * @param instrument 要设置的合约
     */
    public void fillFixedFields(Instrument instrument) {
        // C++: memset(&m_reqMsg, '\0', sizeof(m_reqMsg));
        // C++: m_instru = instrument;
        // Ref: ExecutionStrategy.cpp:1487-1520
        this.instru = instrument;

        // C++: 以下字段在 Java 中由 CommonClient.sendNewOrder() 内部处理：
        //   m_reqMsg.Token = m_instru->m_token;
        //   m_reqMsg.AccountID = m_account;
        //   m_reqMsg.Contract_Description.InstrumentName = m_instruType;
        //   m_reqMsg.Contract_Description.Symbol = m_instru->m_symbol;
        //   m_reqMsg.Contract_Description.OptionType = GetOptionType(m_instru->m_callPutFlag);
        //   m_reqMsg.Contract_Description.ExpiryDate = m_instru->m_expiryDate;
        //   m_reqMsg.Contract_Description.StrikePrice = m_instru->m_strike;
        //   m_reqMsg.OrdType = LIMIT;
        //   m_reqMsg.PxType = PERUNIT;

        // C++: if (!strcmp(m_instru->m_exchange, "CME")) { ... }
        // Ref: ExecutionStrategy.cpp:1513-1519
        // CME 特殊处理: InstrumentName = securitygroup, Future 时 OptionType = "X"
        // [C++差异] Java 中 RequestMsg 字段由 CommonClient.sendNewOrder() 构建，
        // CME 的 securitygroup/productType 字段已添加到 Instrument 类中。
        // 当发送 CME 订单时，CommonClient 需读取 instru.securityGroup 和 instru.productType
        // 来正确设置 Contract_Description 字段。
    }

    public double roundWorse(byte side, double price, double tick) {
        if (side == Constants.SIDE_BUY) {
            return Math.floor(price / tick) * tick;
        } else {
            return Math.ceil(price / tick) * tick;
        }
    }

    // =======================================================================
    //  OptionType 枚举 + GetOptionType
    // =======================================================================

    /**
     * 期权类型枚举。
     * 迁移自: tbsrc/common/include/CommonDefs.h — enum OptionType { CALL, PUT, NILL }
     */
    public enum OptionType {
        CALL, PUT, NILL
    }

    /**
     * 将期权类型字符转换为枚举。
     * 迁移自: ExecutionStrategy::GetOptionType(char optionType)
     * Ref: ExecutionStrategy.cpp:484-498
     *
     * @param optionType 'C' for CALL, 'P' for PUT
     * @return OptionType 枚举值
     */
    public OptionType getOptionType(char optionType) {
        // C++: OptionType optType = NILL;
        // C++: switch (optionType) { case 'C': optType = CALL; break; case 'P': optType = PUT; break; }
        return switch (optionType) {
            case 'C' -> OptionType.CALL;
            case 'P' -> OptionType.PUT;
            default -> OptionType.NILL;
        };
    }

    // =======================================================================
    //  GetInstrumentStats — 成交量 EWA 统计
    // =======================================================================

    /**
     * 计算合约成交量统计 — 滑动窗口 EWA (指数加权平均)。
     * 迁移自: ExecutionStrategy::GetInstrumentStats()
     * Ref: ExecutionStrategy.cpp:398-420
     *
     * 逻辑:
     * 1. 记录当前时刻和成交量增量到滑动窗口队列
     * 2. 清除过期数据（超过 STAT_DURATION_SMALL 窗口期）
     * 3. 计算 EWA: stat_multiplier * instruAvgTradeQty + (1-stat_multiplier) * volume_ewa
     * 4. 根据 EWA 与 STAT_TRADE_THRESH 比较设置 SET_HIGH 标志
     */
    public void getInstrumentStats() {
        // C++: StatTrTimeQ.push_back(Watch::GetUniqueInstance()->GetCurrentTime());
        long currTime = Watch.getInstance().getCurrentTime();
        statTrTimeQ.addLast(currTime);

        // C++: double trade_diff = (m_instru->totalTradedQty - prev_tradeQty) / m_instru->m_lotSize;
        double tradeDiff = (instru.totalTradedQty - prevTradeQty) / instru.lotSize;
        // C++: prev_tradeQty = m_instru->totalTradedQty;
        prevTradeQty = instru.totalTradedQty;

        // C++: if (StatTradeQtyQ.size() > 0) instruAvgTradeQty += trade_diff; else instruAvgTradeQty = trade_diff;
        if (!statTradeQtyQ.isEmpty()) {
            instruAvgTradeQty += tradeDiff;
        } else {
            instruAvgTradeQty = tradeDiff;
        }
        statTradeQtyQ.addLast(tradeDiff);

        // C++: while (StatTrTimeQ.size() > 0 && StatTrTimeQ.front() <= currTime - m_thold->STAT_DURATION_SMALL)
        while (!statTrTimeQ.isEmpty() && statTrTimeQ.peekFirst() <= currTime - thold.STAT_DURATION_SMALL) {
            // C++: instruAvgTradeQty -= StatTradeQtyQ.front();
            instruAvgTradeQty -= statTradeQtyQ.pollFirst();
            statTrTimeQ.pollFirst();
        }

        // C++: double stat_multiplier = 2.0 / (1 + 2.8854 * 5);
        double statMultiplier = 2.0 / (1 + 2.8854 * 5);
        // C++: volume_ewa = stat_multiplier * instruAvgTradeQty + (1 - stat_multiplier) * volume_ewa;
        volumeEwa = statMultiplier * instruAvgTradeQty + (1 - statMultiplier) * volumeEwa;

        // C++: if (volume_ewa > m_thold->STAT_TRADE_THRESH) SET_HIGH = 0; else SET_HIGH = 1;
        if (volumeEwa > thold.STAT_TRADE_THRESH) {
            SET_HIGH = 0;
        } else {
            SET_HIGH = 1;
        }
    }

    // =======================================================================
    //  AddtoCache — Self-book 缓存管理
    // =======================================================================

    /**
     * 将订单添加到 self-book 价格缓存。
     * 迁移自: ExecutionStrategy::AddtoCache(OrderMapIter &iter, double &price)
     * Ref: ExecutionStrategy.cpp:821-834
     *
     * @param order 订单统计
     * @param price 订单价格
     */
    public void addToCache(OrderStats order, double price) {
        // C++: if (iter->second->m_side == BUY) priceMapCache = &m_bidMapCache; else priceMapCache = &m_askMapCache;
        if (order.side == Constants.SIDE_BUY) {
            bidMapCache.put(price, order);
        } else {
            askMapCache.put(price, order);
        }
    }

    // =======================================================================
    //  Dump 调试方法
    // =======================================================================

    /**
     * 打印内部订单簿（我方挂单）。
     * 迁移自: ExecutionStrategy::DumpOurBook()
     * Ref: ExecutionStrategy.cpp:1605-1624
     */
    public void dumpOurBook() {
        // C++: TBLOG << __PRETTY_FUNCTION__ << " OUR BID orders : " << endl;
        StringBuilder sb = new StringBuilder();
        sb.append("DumpOurBook OUR BID orders:\n");
        int bidctr = 0;
        for (Map.Entry<Double, OrderStats> e : bidMap.entrySet()) {
            bidctr++;
            OrderStats o = e.getValue();
            // C++: iter->second->m_orderID \t m_price \t m_Qty \t m_openQty \t m_status \t m_typeOfOrder
            sb.append(String.format("  %d\t%.4f\t%d\t%d\t%s\n",
                    o.orderID, o.price, o.qty, o.openQty, o.ordType));
        }
        sb.append("DumpOurBook OUR ASK orders:\n");
        int askctr = 0;
        for (Map.Entry<Double, OrderStats> e : askMap.entrySet()) {
            askctr++;
            OrderStats o = e.getValue();
            sb.append(String.format("  %d\t%.4f\t%d\t%d\t%s\n",
                    o.orderID, o.price, o.qty, o.openQty, o.ordType));
        }
        sb.append("BIDCTR: ").append(bidctr).append(" ASKCTR: ").append(askctr);
        log.info(sb.toString());
    }

    /**
     * 打印指标值。
     * 迁移自: ExecutionStrategy::DumpIndicators()
     * Ref: ExecutionStrategy.cpp:1626-1634
     *
     * [C++差异] C++ 遍历 m_simConfig->m_indicatorList 打印每个指标的
     * coefficient * indicator->Value(status) * tickSize。
     * Java 版本打印 targetPrice 和 currPrice。指标列表遍历依赖 Indicator 模块（独立迁移范围）。
     */
    public void dumpIndicators() {
        // C++: TBLOG << m_targetPrice << "\t" << m_currPrice << "\t";
        // C++: for (iter : m_indicatorList) TBLOG << (*iter)->m_coefficient * (*iter)->m_indicator->Value(status) * m_instru->m_tickSize;
        StringBuilder sb = new StringBuilder();
        sb.append("DumpIndicators: targetPrice=").append(targetPrice)
          .append(" currPrice=").append(currPrice);
        // [C++差异-用户确认] 指标列表遍历省略 — Java Indicator 系统为独立模块，
        // 不在当前 ExecutionStrategy 迁移范围内。当 Indicator 模块迁移后，
        // 在此处遍历 indicatorList 打印 coefficient * Value(status) * tickSize。
        // 参见 ExecutionStrategy.cpp:1626-1634
        log.info(sb.toString());
    }

    /**
     * 打印市场订单簿。
     * 迁移自: ExecutionStrategy::DumpMktBook()
     * Ref: ExecutionStrategy.cpp:1636-1641
     */
    public void dumpMktBook() {
        // C++: for (i = 0; i < m_instru->m_level; ++i)
        //   TBLOG << bidOrderCount[i] << bidQty[i] << bidPx[i] << " X " << askPx[i] << askQty[i] << askOrderCount[i]
        StringBuilder sb = new StringBuilder("DumpMktBook:\n");
        int levels = instru.level > 0 ? instru.level : instru.bookDepth;
        for (int i = 0; i < levels; i++) {
            sb.append(String.format("  %5.0f %8.0f %10.4f X %-10.4f %-8.0f %-5.0f\n",
                    instru.bidOrderCount[i], instru.bidQty[i], instru.bidPx[i],
                    instru.askPx[i], instru.askQty[i], instru.askOrderCount[i]));
        }
        log.info(sb.toString());
    }

    /**
     * 打印策略订单簿。
     * 迁移自: ExecutionStrategy::DumpStratBook()
     * Ref: ExecutionStrategy.cpp:1643-1648
     */
    public void dumpStratBook() {
        // C++: for (i = 0; i < m_instru->m_level; ++i)
        //   TBLOG << bidOrderCountStrat[i] << bidQtyStrat[i] << bidPxStrat[i] << " X " << askPxStrat[i] << askQtyStrat[i] << askOrderCountStrat[i]
        StringBuilder sb = new StringBuilder("DumpStratBook:\n");
        int levels = instru.level > 0 ? instru.level : instru.bookDepth;
        for (int i = 0; i < levels; i++) {
            sb.append(String.format("  %5.0f %8.0f %10.4f X %-10.4f %-8.0f %-5.0f\n",
                    instru.bidOrderCountStrat[i], instru.bidQtyStrat[i], instru.bidPxStrat[i],
                    instru.askPxStrat[i], instru.askQtyStrat[i], instru.askOrderCountStrat[i]));
        }
        log.info(sb.toString());
    }

    @Override
    public String toString() {
        return String.format("Strategy[id=%d, instru=%s, netpos=%d, pnl=%.2f, active=%s]",
                strategyID, instru != null ? instru.origBaseName : "null", netpos, netPNL, active);
    }
}
