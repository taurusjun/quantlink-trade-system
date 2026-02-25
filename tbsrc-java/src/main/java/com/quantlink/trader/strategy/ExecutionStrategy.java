package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.lang.foreign.MemorySegment;
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

        reset();
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

    // =======================================================================
    //  辅助方法
    // =======================================================================

    /**
     * RoundWorse — 向不利方向取整。
     * 迁移自: ExecutionStrategy::RoundWorse()
     * Ref: ExecutionStrategy.cpp:2343-2353
     */
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
