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
    public int netpos_pass;              // C++: m_netpos_pass
    public int netpos_pass_ytd;          // C++: m_netpos_pass_ytd
    public int netpos_agg;               // C++: m_netpos_agg

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
    public long last_agg_time;
    public byte last_agg_side;

    // ---- 订单/价格 Map ----
    // 迁移自: ExecutionStrategy.h:257-264
    // C++: OrderMap = map<uint32_t, OrderStats*>
    // C++: PriceMap = map<double, OrderStats*>
    public final Map<Integer, OrderStats> ordMap = new LinkedHashMap<>();
    public final Map<Double, OrderStats> bidMap = new TreeMap<>();
    public final Map<Double, OrderStats> askMap = new TreeMap<>();

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
        netpos_pass = 0;
        netpos_pass_ytd = 0;
        netpos_agg = 0;
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
        last_agg_time = 0; last_agg_side = 0;

        tholdBidSize = 0; tholdBidMaxPos = 0;
        tholdAskSize = 0; tholdAskMaxPos = 0;

        // C++: m_Active = m_configParams->m_modeType == ModeType_Sim ? true : false
        active = (configParams.modeType == 1);

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
        if (configParams.modeType == 1) { // ModeType_Sim = 1
            localTS = System.nanoTime();
        }

        // C++: if ((m_rejectCount > REJECT_LIMIT - 100) && m_onFlat == false && m_Active)
        if ((rejectCount > REJECT_LIMIT - 100) && !onFlat && active) {
            // C++: SendAlert("Strategy squared off due to reject limit", " MAX REJECT LIMIT got hit");
            log.warning("[" + product + "] REJECT Limit approaching (" + rejectCount + "), cancelling orders and square off...");
        }

        // C++: 监控状态上报 — 每 120 秒
        // C++: auto curr_time = Watch::GetUniqueInstance()->GetCurrentTime();
        long currTime = exchTS != 0 ? exchTS : System.nanoTime();
        // C++: uint64_t gap = 1000000000; if (mlog && curr_time - m_lastStsTS > gap * 120)
        long gap = 1_000_000_000L;
        if (currTime - lastStsTS > gap * 120) {
            // C++: SendMonitorStratStatus(...) — 监控上报省略（未迁移）
            lastStsTS = currTime;
        }

        // C++: if (!m_onFlat && m_Active) { ... SendOrder(); }
        if (!onFlat && active) {
            // C++: optionStrategy 相关逻辑省略（中国期货不使用）

            // C++: if (((bCrossBook || bCrossBook2) && ...) || !(bCrossBook || bCrossBook2)) { SendOrder(); }
            // [C++差异] CrossBook 条件简化：当前中国期货场景 bCrossBook/bCrossBook2 均为 false，直接调用 SendOrder
            if ((!configParams.bCrossBook && !configParams.bCrossBook2) ||
                    ((configParams.bCrossBook || configParams.bCrossBook2))) {
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

        // C++: SetThresholds() 阶梯逻辑
        if (netpos == 0) {
            tholdBidPlace = thold.BEGIN_PLACE;
            tholdAskPlace = thold.BEGIN_PLACE;
            tholdBidRemove = thold.BEGIN_REMOVE;
            tholdAskRemove = thold.BEGIN_REMOVE;
        } else if (netpos > 0 && netpos < tholdBeginPos) {
            tholdBidPlace = thold.BEGIN_PLACE;
            tholdBidRemove = thold.BEGIN_REMOVE;
            tholdAskPlace = thold.SHORT_PLACE;
            tholdAskRemove = thold.SHORT_REMOVE;
        } else if (netpos < 0 && netpos > -1 * tholdBeginPos) {
            tholdAskPlace = thold.BEGIN_PLACE;
            tholdAskRemove = thold.BEGIN_REMOVE;
            tholdBidPlace = thold.SHORT_PLACE;
            tholdBidRemove = thold.SHORT_REMOVE;
        } else if (netpos > 0) {
            tholdBidPlace = thold.LONG_PLACE;
            tholdBidRemove = thold.LONG_REMOVE;
            tholdAskPlace = thold.SHORT_PLACE;
            tholdAskRemove = thold.SHORT_REMOVE;
        } else if (netpos < 0) {
            tholdAskPlace = thold.LONG_PLACE;
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
        exchTS = (long) Types.MDH_EXCH_TS_VH.get(update, 0L);
        calculatePNL();
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
        return sendNewOrder(side, price, qty, orderLevel, OrderStats.HitType.STANDARD);
    }

    public OrderStats sendNewOrder(byte side, double price, int qty, int orderLevel, OrderStats.HitType ordtype) {
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
        ordStats.modifyCount = 0;
        ordStats.status = OrderStats.Status.NEW_ORDER;
        ordStats.price = price;
        ordStats.side = side;
        ordStats.orderID = orderID;
        ordStats.qty = qty;
        ordStats.openQty = qty;
        ordStats.doneQty = 0;
        ordStats.quantBehind = 0;
        ordStats.hitType = ordtype;

        // C++: m_quantAhead = (side==BUY) ? (bidPx[level]==price ? bidQty[level] : 0) : ...
        if (side == Constants.SIDE_BUY) {
            ordStats.quantAhead = (instru.bidPx[orderLevel] == price) ? instru.bidQty[orderLevel] : 0;
        } else {
            ordStats.quantAhead = (instru.askPx[orderLevel] == price) ? instru.askQty[orderLevel] : 0;
        }

        ordMap.put(orderID, ordStats);
        priceMap.put(price, ordStats);
        configParams.orderIDStrategyMap.put(orderID, this);

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
        order.hitType = ordtype;

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

        if (order.modifyCount == 0) {
            order.oldPrice = order.price;
            order.oldQty = order.openQty;
        }
        order.modifyCount++;
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

        // C++: 更新 netpos_pass / netpos_agg
        if (order.hitType == OrderStats.HitType.IMPROVE) {
            improveCount++;
        } else if (order.hitType == OrderStats.HitType.CROSS) {
            crossCount++;
            if (order.side == Constants.SIDE_BUY) netpos_agg += tradeQty; else netpos_agg -= tradeQty;
        } else if (order.hitType == OrderStats.HitType.STANDARD) {
            if (order.side == Constants.SIDE_BUY) netpos_pass += tradeQty; else netpos_pass -= tradeQty;
        } else if (order.hitType == OrderStats.HitType.MATCH) {
            if (order.side == Constants.SIDE_BUY) netpos_agg += tradeQty; else netpos_agg -= tradeQty;
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
        order.modifyCount = 0;
    }

    /** 撤单确认。 Ref: ExecutionStrategy.cpp:1912-1981 */
    protected void processCancelConfirm(MemorySegment response, OrderStats order) {
        order.status = OrderStats.Status.CANCEL_CONFIRM;
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
        if (netpos > 0) {
            unrealisedPNL = netpos * ((instru.bidPx[0] - buyPrice - instru.bidPx[0] * sellExchTx) * instru.priceMultiplier - sellExchContractTx);
        } else if (netpos < 0) {
            unrealisedPNL = -1 * netpos * ((sellPrice - instru.askPx[0] - instru.askPx[0] * buyExchTx) * instru.priceMultiplier - buyExchContractTx);
        } else {
            unrealisedPNL = 0;
        }

        double qty = netpos > 0 ? sellQty : buyQty;
        unrealisedPNL += (qty * (sellPrice - buyPrice) * instru.priceMultiplier);
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
        // C++: if (m_instru->m_perYield) ... Bond 逻辑省略（中国期货不使用 perYield）
        if (instru.perYield) {
            // [C++差异] BondPrice() 未迁移，perYield=false 时不会进入此分支
            pnl = 0;
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
     * 检查是否需要平仓。
     * 迁移自: ExecutionStrategy::CheckSquareoff(MarketUpdateNew*)
     * Ref: ExecutionStrategy.cpp:2150-2341 (简化版，不含期权/delta/新闻处理)
     */
    public void checkSquareoff(MemorySegment update) {
        // C++: 时间限制、最大亏损、最大订单数、最大成交量
        if (!onExit) {
            boolean shouldExit = false;
            if (netPNL < thold.MAX_LOSS * -1) shouldExit = true;
            if (orderCount >= maxOrderCount) shouldExit = true;
            if (buyTotalQty >= maxTradedQty || sellTotalQty >= maxTradedQty) shouldExit = true;

            if (shouldExit) {
                onExit = true;
                onCancel = true;
                onFlat = true;
                log.warning("[" + instru.origBaseName + "] Squareoff triggered: netPNL=" + netPNL + " orderCount=" + orderCount);
            }
        }

        // C++: UPNL LOSS check
        if (!onFlat && thold.CHECK_PNL && unrealisedPNL < -1 * thold.UPNL_LOSS) {
            onCancel = true;
            onFlat = true;
            log.warning("[" + instru.origBaseName + "] UPNL loss triggered: " + unrealisedPNL);
        }

        // C++: STOP LOSS check
        if (!onFlat && thold.CHECK_PNL && netPNL < -1 * thold.STOP_LOSS) {
            onExit = true;
            onCancel = true;
            onFlat = true;
            log.warning("[" + instru.origBaseName + "] Stop loss triggered: " + netPNL);
        }
    }

    /**
     * 执行平仓。
     * 迁移自: ExecutionStrategy::HandleSquareoff()
     * Ref: ExecutionStrategy.cpp:2355-2437
     */
    public void handleSquareoff() {
        // C++: 撤销所有挂单
        List<Integer> cancelList = new ArrayList<>(ordMap.keySet());
        for (int ordID : cancelList) {
            sendCancelOrder(ordID);
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
        // C++: m_instru = instrument;
        this.instru = instrument;
        // C++: 以下字段在 Java 中由 CommonClient.sendNewOrder() 内部处理：
        //   m_reqMsg.Token = m_instru->m_token;
        //   m_reqMsg.AccountID = m_account;
        //   m_reqMsg.Contract_Description.InstrumentName = m_instruType;
        //   m_reqMsg.Contract_Description.Symbol = m_instru->m_symbol;
        //   m_reqMsg.Contract_Description.OptionType = ...;
        //   m_reqMsg.Contract_Description.ExpiryDate = m_instru->m_expiryDate;
        //   m_reqMsg.Contract_Description.StrikePrice = m_instru->m_strike;
        //   m_reqMsg.OrdType = LIMIT;
        //   m_reqMsg.PxType = PERUNIT;
        // CME 特殊处理省略（中国期货不使用）
    }

    public double roundWorse(byte side, double price, double tick) {
        if (side == Constants.SIDE_BUY) {
            return Math.floor(price / tick) * tick;
        } else {
            return Math.ceil(price / tick) * tick;
        }
    }

    @Override
    public String toString() {
        return String.format("Strategy[id=%d, instru=%s, netpos=%d, pnl=%.2f, active=%s]",
                strategyID, instru != null ? instru.origBaseName : "null", netpos, netPNL, active);
    }
}
