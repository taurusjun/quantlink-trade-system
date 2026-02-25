package com.quantlink.trader.core;

import com.quantlink.trader.connector.Connector;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.function.Consumer;

/**
 * MD/ORS 回调分发中枢。
 * 迁移自: tbsrc/main/include/CommonClient.h (line 78-132)
 *         tbsrc/main/CommonClient.cpp
 *
 * 核心职责:
 * 1. MD 分发 — 按 symbolID 路由行情到对应策略 (SendINDUpdate)
 * 2. ORS 分发 — 回报回调路由 (SendInfraORSUpdate)
 * 3. 发单封装 — SendNewOrder/SendModifyOrder/SendCancelOrder 委托给 Connector
 */
public class CommonClient {

    // ---- 回调 ----
    // 迁移自: CommonClient.h:108-111
    // C++: MDcb m_MDCallBack; ORScb m_ORSCallBack;
    private Consumer<MemorySegment> mdCallback;
    private Consumer<MemorySegment> orsCallback;

    // ---- Connector ----
    // 迁移自: CommonClient.h:105 — illuminati::Connector* m_connector
    private Connector connector;

    // ---- ConfigParams ----
    // 迁移自: CommonClient.h:130 — ConfigParams* m_configParams
    private final ConfigParams configParams;

    // ---- SimConfig 数组 ----
    // 迁移自: CommonClient.h:124 — SimConfig* m_simConfig (数组)
    private SimConfig[] simConfigs;

    // ---- 状态标志 ----
    // 迁移自: CommonClient.h:112-113
    private boolean active = false;
    private long lastUpdate = 0;

    // ---- 请求消息缓冲 ----
    // 迁移自: CommonClient.h:123 — RequestMsg m_reqMsg
    private final MemorySegment reqMsg;
    private final Arena arena;

    public CommonClient() {
        this.configParams = ConfigParams.getInstance();
        this.arena = Arena.ofAuto();
        this.reqMsg = arena.allocate(Types.REQUEST_MSG_LAYOUT);
    }

    // =======================================================================
    //  回调注册
    // =======================================================================

    /**
     * 注册行情回调。
     * 迁移自: CommonClient::Initialize() 中的 MDcb 参数
     */
    public void setMDCallback(Consumer<MemorySegment> callback) {
        this.mdCallback = callback;
    }

    /**
     * 注册回报回调。
     * 迁移自: CommonClient::Initialize() 中的 ORScb 参数
     */
    public void setORSCallback(Consumer<MemorySegment> callback) {
        this.orsCallback = callback;
    }

    /**
     * 设置 Connector。
     * 迁移自: CommonClient::SetConnector(Connector*)
     * Ref: CommonClient.h:101
     */
    public void setConnector(Connector connector) {
        this.connector = connector;
    }

    /**
     * 设置 SimConfig 数组。
     * 迁移自: CommonClient::Initialize() 中的 SimConfig* 参数
     */
    public void setSimConfigs(SimConfig[] configs) {
        this.simConfigs = configs;
    }

    // =======================================================================
    //  行情分发 (MD Dispatch)
    // =======================================================================

    /**
     * 行情分发入口 — 接收 MarketUpdateNew 并路由到对应策略。
     * 迁移自: CommonClient::SendInfraMDUpdate(MarketUpdateNew*)
     * Ref: CommonClient.cpp:321-376
     *
     * 流程:
     * 1. 调用 SendINDUpdate 进行 symbolID 路由
     *
     * @param mdUpdate MarketUpdateNew MemorySegment
     */
    public void sendInfraMDUpdate(MemorySegment mdUpdate) {
        // C++: SendINDUpdate(update)
        // Ref: CommonClient.cpp:365
        sendINDUpdate(mdUpdate);
    }

    /**
     * 按 symbolID 分发行情到策略。
     * 迁移自: CommonClient::SendINDUpdate(MarketUpdateNew*)
     * Ref: CommonClient.cpp:401-769
     *
     * 核心逻辑:
     * 1. 从 update 读取 symbolID
     * 2. 通过 configParams.simConfigMap[symbolID] 查找 SimConfigList
     * 3. 遍历 list，找到对应 Instrument，更新订单簿
     * 4. 调用 MDCallback 路由到策略
     *
     * @param mdUpdate MarketUpdateNew MemorySegment
     */
    private void sendINDUpdate(MemorySegment mdUpdate) {
        // C++: SimConfigMapIter simIter = m_configParams->m_simConfigList[update->m_symbolID]
        // Ref: CommonClient.cpp:418
        int symbolID = Instrument.readSymbolID(mdUpdate);

        List<SimConfig> simConfigList = configParams.simConfigMap.get(symbolID);
        if (simConfigList == null) {
            return; // symbolID 未注册
        }

        // C++: for (SimConfigListIter listIter = simIter->second->begin(); ...)
        // Ref: CommonClient.cpp:431
        for (SimConfig simCfg : simConfigList) {
            Instrument instru = simCfg.instruMap.get(symbolID);
            if (instru == null) continue;

            // C++: instru->lastLocalTime = update->m_timestamp
            // Ref: CommonClient.cpp:499
            instru.lastLocalTime = (long) Types.MDH_TIMESTAMP_VH.get(mdUpdate, 0L);
            instru.lastExchTime = (long) Types.MDH_EXCH_TS_VH.get(mdUpdate, 0L);

            // C++: instru->FillOrderBook(update)
            // Ref: CommonClient.cpp:520
            instru.fillOrderBook(mdUpdate);
        }

        // C++: m_MDCallBack(update)
        // 在 main 中，MDCallBack 路由到 configParams->m_simConfig->m_execStrategy->MDCallBack(up)
        if (mdCallback != null) {
            active = true;
            mdCallback.accept(mdUpdate);
        }
    }

    // =======================================================================
    //  ORS 分发 (Order Response Dispatch)
    // =======================================================================

    /**
     * 回报分发入口。
     * 迁移自: CommonClient::SendInfraORSUpdate(ResponseMsg*)
     * Ref: CommonClient.cpp:277-319
     *
     * @param response ResponseMsg MemorySegment
     */
    public void sendInfraORSUpdate(MemorySegment response) {
        // C++: m_ORSCallBack(response)
        // Ref: CommonClient.cpp:298
        if (orsCallback != null) {
            orsCallback.accept(response);
        }
    }

    // =======================================================================
    //  发单接口
    // =======================================================================

    /**
     * 发送新订单。
     * 迁移自: CommonClient::SendNewOrder(...)
     * Ref: CommonClient.h:87
     *
     * C++ 签名: uint32_t SendNewOrder(uint32_t strategyID, const char* symbol,
     *            OptionType, const char* exchange, TransactionType, double price,
     *            int32_t qty, int32_t level, int32_t group, const char* account,
     *            int32_t posDirection, OrderHitType, ExecutionStrategy*)
     *
     * @param strategyID   策略 ID
     * @param symbol       合约符号
     * @param side         买卖方向 (Constants.TRANS_BUY/SELL)
     * @param price        价格
     * @param qty          数量
     * @param posDirection 开平方向 (Constants.POS_OPEN/CLOSE等)
     * @param strategy     策略实例引用（用于订单映射）
     * @return 分配的 OrderID
     */
    public int sendNewOrder(int strategyID, String symbol, int side, double price,
                            int qty, int posDirection, Object strategy) {
        // 清零请求缓冲
        reqMsg.fill((byte) 0);

        // 填充字段
        // C++: m_reqMsg.Request_Type = NEWORDER
        Types.REQ_REQUEST_TYPE_VH.set(reqMsg, 0L, Constants.REQUEST_NEWORDER);
        Types.REQ_ORD_TYPE_VH.set(reqMsg, 0L, Constants.ORD_LIMIT);
        Types.REQ_STRATEGY_ID_VH.set(reqMsg, 0L, strategyID);
        Types.REQ_QUANTITY_VH.set(reqMsg, 0L, qty);
        Types.REQ_PRICE_VH.set(reqMsg, 0L, price);
        Types.REQ_POS_DIRECTION_VH.set(reqMsg, 0L, posDirection);

        // C++: Transaction_Type (offset 163)
        Types.REQ_TRANSACTION_TYPE_VH.set(reqMsg, 0L, (byte) side);

        // 写入 symbol
        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symBytes.length)
              .copyFrom(MemorySegment.ofArray(symBytes));

        // 委托 Connector 发送
        int orderID = connector.sendNewOrder(reqMsg);

        // 注册到 orderID→strategy 映射
        // C++: m_configParams->m_orderIDStrategyMap[orderID] = strategy
        configParams.orderIDStrategyMap.put(orderID, strategy);

        return orderID;
    }

    /**
     * 发送修改订单。
     * 迁移自: CommonClient::SendModifyOrder(...)
     * Ref: CommonClient.h:88
     */
    public void sendModifyOrder(int strategyID, String symbol, int side, double price,
                                int qty, int orderID, int posDirection, Object strategy) {
        reqMsg.fill((byte) 0);

        Types.REQ_REQUEST_TYPE_VH.set(reqMsg, 0L, Constants.REQUEST_MODIFYORDER);
        Types.REQ_ORD_TYPE_VH.set(reqMsg, 0L, Constants.ORD_LIMIT);
        Types.REQ_STRATEGY_ID_VH.set(reqMsg, 0L, strategyID);
        Types.REQ_ORDER_ID_VH.set(reqMsg, 0L, orderID);
        Types.REQ_QUANTITY_VH.set(reqMsg, 0L, qty);
        Types.REQ_PRICE_VH.set(reqMsg, 0L, price);
        Types.REQ_POS_DIRECTION_VH.set(reqMsg, 0L, posDirection);
        Types.REQ_TRANSACTION_TYPE_VH.set(reqMsg, 0L, (byte) side);

        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symBytes.length)
              .copyFrom(MemorySegment.ofArray(symBytes));

        connector.sendModifyOrder(reqMsg);
    }

    /**
     * 发送撤单。
     * 迁移自: CommonClient::SendCancelOrder(...)
     * Ref: CommonClient.h:89
     */
    public void sendCancelOrder(int strategyID, String symbol, int side,
                                int orderID, Object strategy) {
        reqMsg.fill((byte) 0);

        Types.REQ_REQUEST_TYPE_VH.set(reqMsg, 0L, Constants.REQUEST_CANCELORDER);
        Types.REQ_STRATEGY_ID_VH.set(reqMsg, 0L, strategyID);
        Types.REQ_ORDER_ID_VH.set(reqMsg, 0L, orderID);
        Types.REQ_TRANSACTION_TYPE_VH.set(reqMsg, 0L, (byte) side);

        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symBytes.length)
              .copyFrom(MemorySegment.ofArray(symBytes));

        connector.sendCancelOrder(reqMsg);
    }

    // =======================================================================
    //  Getters
    // =======================================================================

    public Connector getConnector() { return connector; }
    public boolean isActive() { return active; }
    public ConfigParams getConfigParams() { return configParams; }
}
