package com.quantlink.trader.core;

import com.quantlink.trader.connector.Connector;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.function.Consumer;
import java.util.logging.Logger;

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

    private static final Logger log = Logger.getLogger(CommonClient.class.getName());

    // ---- 回调 ----
    // 迁移自: CommonClient.h:108-111
    // C++: MDcb m_MDCallBack; INDcb m_INDCallBack; ORScb m_ORSCallBack; AUCcb m_AuctionCallBack;
    private Consumer<MemorySegment> mdCallback;
    private Consumer<SimConfig> indCallback;
    private Consumer<MemorySegment> orsCallback;
    private Consumer<MemorySegment> auctionCallback;

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

    // 迁移自: CommonClient.h:131 — bool sendAllUpdates
    // C++: 当 sendAllUpdates=true 时，即使 significantUpdate=false 也发送 MDCallBack
    private boolean sendAllUpdates = false;

    // 迁移自: CommonClient.cpp:404 — int totalTrades (局部静态计数)
    private int totalTrades = 0;

    // 迁移自: CommonClient.h:113 — bool m_bMDUpdate
    private boolean bMDUpdate = false;

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
     * Ref: CommonClient.cpp:145 — m_MDCallBack = mdcb
     */
    public void setMDCallback(Consumer<MemorySegment> callback) {
        this.mdCallback = callback;
    }

    /**
     * 注册 Indicator 回调。
     * 迁移自: CommonClient::Initialize() 中的 INDcb 参数
     * Ref: CommonClient.cpp:147 — m_INDCallBack = indcb
     *
     * C++: typedef void (*INDcb)(IndicatorList*) — 接收 IndicatorList 参数
     * Java: Consumer<SimConfig> — 传入当前 simConfig 上下文
     * Ref: CommonClient.h:72,109
     */
    public void setINDCallback(Consumer<SimConfig> callback) {
        this.indCallback = callback;
    }

    /**
     * 注册回报回调。
     * 迁移自: CommonClient::Initialize() 中的 ORScb 参数
     * Ref: CommonClient.cpp:146 — m_ORSCallBack = orscb
     */
    public void setORSCallback(Consumer<MemorySegment> callback) {
        this.orsCallback = callback;
    }

    /**
     * 注册集合竞价回调。
     * 迁移自: CommonClient.h:74 — AUCcb m_AuctionCallBack
     * Ref: CommonClient.cpp:455 — m_AuctionCallBack(update)
     */
    public void setAuctionCallback(Consumer<MemorySegment> callback) {
        this.auctionCallback = callback;
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
     * Ref: CommonClient.cpp:401-769 (370 行)
     *
     * C++ 调用顺序 (关键):
     * 1. symbol 路由，找到 SimConfig 列表                    [CommonClient.cpp:418,428]
     * 2. 遍历 simConfigList，设置 configParams.simConfig     [CommonClient.cpp:431,434]
     * 3. 首次迭代执行: trade 过滤 + FillOrderBook + significantUpdate 判定
     *    [CommonClient.cpp:460-680]
     * 4. 每次迭代执行 per-simConfig 回调分发:
     *    a. Update(iter, tick) — Indicator 更新              [CommonClient.cpp:731]
     *    b. m_INDCallBack(&indicatorList)                    [CommonClient.cpp:752]
     *    c. m_MDCallBack(update) — 仅当 isStratSymbol       [CommonClient.cpp:761]
     *
     * [C++差异] OptionManager/DeltaStrategy/VOLTHREAD/Profiler 块未迁移，
     *           因 Java 系统仅用于中国期货，不涉及期权定价。
     *           相关代码用 optionStrategy 条件跳过，如未来需要迁移，
     *           参见 CommonClient.cpp:587-718。
     * [C++差异] CommonBook/SelfBook (m_bCommonBook/m_bSelfBook) 未迁移，
     *           中国期货场景不使用 common book 或 self-book 过滤。
     *           参见 CommonClient.cpp:536-555。
     * [C++差异] SMARTMD 条件编译块未迁移 (m_smartTrade/endPkt==2 提前返回)，
     *           中国期货 md_shm_feeder 不产生 SMARTMD 标记。
     *           参见 CommonClient.cpp:492-497。
     *
     * @param mdUpdate MarketUpdateNew MemorySegment
     */
    private void sendINDUpdate(MemorySegment mdUpdate) {
        // C++: bool significantUpdate = true; m_isActive = false; m_isOptionCalled = false;
        // Ref: CommonClient.cpp:403-405
        boolean significantUpdate = true;
        active = false;
        // C++: strcpy(m_configParams->m_updateSymbol, update->m_symbol)
        // Ref: CommonClient.cpp:409
        String symbol = Instrument.readSymbol(mdUpdate);
        configParams.updateSymbol = symbol;
        // C++: m_configParams->m_underlying = false
        // Ref: CommonClient.cpp:410
        configParams.underlying = false;

        // ---- Watch 时钟更新 ----
        // 迁移自: CommonClient.cpp:412-415
        // C++: if (!m_bUseExchTS)
        //          Watch::GetUniqueInstance()->UpdateTime(update->m_timestamp, symbol);
        //      else
        //          Watch::GetUniqueInstance()->UpdateTime(update->m_exchTS * 1000000, symbol);
        if (Watch.getInstance() != null) {
            long mdTimestamp = (long) Types.MDH_TIMESTAMP_VH.get(mdUpdate, 0L);
            long mdExchTS = (long) Types.MDH_EXCH_TS_VH.get(mdUpdate, 0L);
            if (!configParams.useExchTS) {
                Watch.getInstance().updateTime(mdTimestamp, symbol);
            } else {
                Watch.getInstance().updateTime(mdExchTS * 1_000_000, symbol);
            }
        }

        // 读取 MDDataPart 关键字段
        long mdDataBase = Types.MU_DATA_OFFSET; // 96
        byte feedType = (byte) Types.MDD_FEED_TYPE_VH.get(mdUpdate, mdDataBase);
        byte updateType = (byte) Types.MDD_UPDATE_TYPE_VH.get(mdUpdate, mdDataBase);
        byte endPkt = (byte) Types.MDD_END_PKT_VH.get(mdUpdate, mdDataBase);
        byte updateLevel = (byte) Types.MDD_UPDATE_LEVEL_VH.get(mdUpdate, mdDataBase);
        int newQuant = (int) Types.MDD_NEW_QUANT_VH.get(mdUpdate, mdDataBase);
        long rptSeqnum = (long) Types.MDH_RPT_SEQNUM_VH.get(mdUpdate, 0L);

        // C++: SimConfigMapIter simIter = m_configParams->m_simConfigList[update->m_symbolID]
        // Ref: CommonClient.cpp:418,428
        // symbolID 由 md_shm_feeder 设置 (BuildSymbolIDMap, 按字母排序分配 0,1,2...)
        short symbolID = (short) Types.MDH_SYMBOL_ID_VH.get(mdUpdate, 0L);
        if (configParams.simConfigList == null
                || symbolID < 0 || symbolID >= configParams.simConfigList.length) {
            return; // symbolID 未注册
        }
        List<SimConfig> simConfigList = configParams.simConfigList[symbolID];
        if (simConfigList == null) {
            return; // symbolID 未注册
        }

        // C++: bool indUpdate = false;
        // Ref: CommonClient.cpp:430 — 确保行情处理（FillOrderBook 等）只执行一次
        boolean indUpdate = false;
        // C++: Instrument *instru = NULL;
        Instrument instru = null;

        // C++: for (SimConfigListIter listIter = simIter->second->begin(); ...)
        // Ref: CommonClient.cpp:431
        for (SimConfig simCfg : simConfigList) {
            // C++: m_configParams->m_simConfig = (*listIter)
            // Ref: CommonClient.cpp:434
            configParams.simConfig = simCfg;

            // C++: InstruMapIter iter = m_configParams->m_simConfig->m_instruList[update->m_symbolID]
            // Ref: CommonClient.cpp:437
            if (simCfg.instruList == null
                    || symbolID >= simCfg.instruList.length) {
                continue;
            }
            Instrument iterInstru = simCfg.instruList[symbolID];
            if (iterInstru == null) continue;

            // ================================================================
            //  交易时间检查
            // ================================================================
            // C++: if (!m_configParams->m_simConfig->m_dateConfig.m_simActive)
            // Ref: CommonClient.cpp:445-446
            if (!simCfg.simActive) {
                log.fine("Error: Not trading time for " + symbol);
                continue;
            }

            // ================================================================
            //  集合竞价处理
            // ================================================================
            // C++: if (update->m_feedType == FEED_AUCTION) { m_AuctionCallBack(update); }
            // Ref: CommonClient.cpp:450-453
            if (feedType == Constants.FEED_AUCTION) {
                if (auctionCallback != null) {
                    auctionCallback.accept(mdUpdate);
                }
                continue;
            }

            // C++: m_isActive = true
            // Ref: CommonClient.cpp:456
            active = true;

            // ================================================================
            //  行情处理（仅首次迭代）
            // ================================================================
            // C++: if (!indUpdate) { indUpdate = true; ... }
            // Ref: CommonClient.cpp:458-680
            if (!indUpdate) {
                indUpdate = true;
                instru = iterInstru;

                // ---- firstTrade 标记 ----
                // C++: if (!instru->m_crossUpdate && (update->m_updateType == MDUPDTYPE_TRADE_INFO || == MDUPDTYPE_TRADE))
                //          instru->m_firstTrade = true;
                // Ref: CommonClient.cpp:466-467
                if (!instru.crossUpdate
                        && (updateType == Constants.MDUPDTYPE_TRADE_INFO || updateType == Constants.MDUPDTYPE_TRADE)) {
                    instru.firstTrade = true;
                }

                // ---- 隐含成交过滤 ----
                // C++: if ((update->m_updateType == MDUPDTYPE_TRADE_IMPLIED) && instru->m_ignoreImpliedTrades
                //          && (instru->lastrptseqnum == update->m_rptseqnum)) return;
                // Ref: CommonClient.cpp:469-470
                if (updateType == Constants.MDUPDTYPE_TRADE_IMPLIED
                        && instru.ignoreImpliedTrades
                        && instru.lastRptSeqNum == rptSeqnum) {
                    return;
                }

                // ---- CrossBook 过滤 ----
                // C++: if (m_configParams->m_bCrossBook || m_configParams->m_bCrossBook2)
                //          if (OnCrossBook(update, instru, this)) return;
                // Ref: CommonClient.cpp:472-476
                // [C++差异] OnCrossBook 未迁移 — 中国期货场景不使用 CrossBook。
                // 如果启用了 crossBook 配置，此处应添加 OnCrossBook 逻辑。
                // 参见 CommonClient.cpp:472-476

                // ---- 成交量累加 ----
                // C++: if (feedType != FEED_SNAPSHOT && (updateType == TRADE_INFO || TRADE))
                //          instru->totalTradedQty += update->m_newQuant; totalTrades++;
                //          if (!instru->m_firstTrade) return;
                // Ref: CommonClient.cpp:478-486
                if (feedType != Constants.FEED_SNAPSHOT
                        && (updateType == Constants.MDUPDTYPE_TRADE_INFO || updateType == Constants.MDUPDTYPE_TRADE)) {
                    instru.totalTradedQty += newQuant;
                    totalTrades++;
                    if (!instru.firstTrade) {
                        return;
                    }
                }

                // ---- SMARTMD 过滤 ----
                // C++: #ifdef SMARTMD
                //          if (instru->m_smartTrade && update->m_endPkt == 2) instru->m_smartTrade = false;
                //          if (instru->m_smartTrade || update->m_endPkt == 2) return;
                // Ref: CommonClient.cpp:492-497
                // [C++差异] SMARTMD 是条件编译，中国期货 md_shm_feeder 不产生 SMARTMD 标记。
                // 保留逻辑以支持未来扩展:
                if (instru.smartTrade && endPkt == 2) {
                    instru.smartTrade = false;
                }
                if (instru.smartTrade || endPkt == 2) {
                    return;
                }

                // ---- 时间戳更新 ----
                // C++: instru->lastLocalTime = update->m_timestamp; instru->lastExchTime = update->m_exchTS;
                //      instru->lastrptseqnum = update->m_rptseqnum;
                // Ref: CommonClient.cpp:499-501
                instru.lastLocalTime = (long) Types.MDH_TIMESTAMP_VH.get(mdUpdate, 0L);
                instru.lastExchTime = (long) Types.MDH_EXCH_TS_VH.get(mdUpdate, 0L);
                instru.lastRptSeqNum = rptSeqnum;

                // ---- Snapshot 处理 ----
                // C++: if (update->m_feedType == FEED_SNAPSHOT) ProcessSnapShot(update, iter);
                // Ref: CommonClient.cpp:503
                // [C++差异] ProcessSnapShot 未单独迁移 — FillOrderBook 已处理快照数据

                // ---- UseTradeInfo 过滤 ----
                // C++: if (!instru->m_bUseTradeInfo && update->m_updateType == MDUPDTYPE_TRADE_INFO) return;
                // Ref: CommonClient.cpp:505
                if (!instru.bUseTradeInfo && updateType == Constants.MDUPDTYPE_TRADE_INFO) {
                    return;
                }

                // ---- L1 Event 计数 ----
                // C++: instru->lastTick.FillTick(update, instru);
                //      if (instru->lastTick.tickLevel == 1) instru->totalL1Event += 1;
                // Ref: CommonClient.cpp:507-509
                // [C++差异] Tick 对象未迁移。tickLevel 判定用 updateLevel 字段代替。
                if (updateLevel == 1) {
                    instru.totalL1Event += 1;
                }

                // ---- FillOrderBook ----
                // C++: instru->FillOrderBook(update)
                // Ref: CommonClient.cpp:520
                instru.fillOrderBook(mdUpdate);

                // ---- CommonBook 过滤 ----
                // C++: if (m_configParams->m_bCommonBook) { ... if (instruCBIter->second->MDCallBack(update)) return; }
                // Ref: CommonClient.cpp:522-531
                // [C++差异] CommonBook 未迁移 — 中国期货场景不使用 common book

                // ---- SelfBook 过滤 ----
                // C++: if (m_configParams->m_bSelfBook) { ... if (RemoveSelfBook(...)) return; }
                // Ref: CommonClient.cpp:533-542
                // [C++差异] SelfBook 未迁移 — 中国期货场景不使用 self-book 过滤

                // ---- significantUpdate 判定 ----
                // C++: if (instru->lastTick.tickType == INVALID
                //          || (m_configParams->m_optionStrategy && instru->lastTick.tickLevel > 1))
                //          return;
                // C++: if (instru->lastTick.tickLevel > instru->m_level)
                //          significantUpdate = false;
                // Ref: CommonClient.cpp:548-552
                // [C++差异] Tick.tickType 未迁移。使用 bid/ask 有效性代替 INVALID 检查:
                //           如果 bid[0]=0 && ask[0]=0，视为 INVALID tick。
                //           tickLevel 用 MDDataPart.updateLevel 字段。
                if (instru.bidPx[0] == 0 && instru.askPx[0] == 0) {
                    return; // C++: tickType == INVALID
                }
                if (configParams.optionStrategy > 0 && updateLevel > 1) {
                    return;
                }
                if (updateLevel > instru.level) {
                    significantUpdate = false;
                }

                // C++: if (!significantUpdate && !sendAllUpdates) return;
                // Ref: CommonClient.cpp:554
                if (!significantUpdate && !sendAllUpdates) {
                    return;
                }

                // ---- Combined Instrument 处理 ----
                // C++: if (ConfigParams::GetInstance()->m_bUseCombined && significantUpdate) { ... }
                // Ref: CommonClient.cpp:556-583
                // [C++差异] Combined instrument 未迁移 — 中国期货配对套利使用独立合约，
                //           不使用 CME 组合合约模式。参见 CommonClient.cpp:556-583。

                // ---- Delta Strategy 处理 ----
                // C++: if (m_configParams->m_deltaStrategy) { ... }
                // Ref: CommonClient.cpp:585-609
                // [C++差异] DeltaStrategy 未迁移 — 中国期货场景不使用期权 delta 对冲

                // ---- Option Strategy 处理 ----
                // C++: if (m_configParams->m_optionStrategy) { ... } (130 行)
                // Ref: CommonClient.cpp:611-718
                // [C++差异] OptionStrategy/OptionManager/VOLTHREAD 未迁移 —
                //           中国期货场景不涉及期权定价、vol 更新。
                //           如未来需要迁移，参见 CommonClient.cpp:611-718。

            } // end if (!indUpdate)

            // ================================================================
            //  Per-SimConfig 回调分发
            // ================================================================
            // C++: if (!instru->m_useSmartBook || (instru->m_useSmartBook && instru->m_updateIndicators))
            // Ref: CommonClient.cpp:720
            if (instru != null
                    && (!instru.useSmartBook || (instru.useSmartBook && instru.updateIndicators))) {

                // ---- Indicator 更新 + INDCallBack ----
                if (significantUpdate) {
                    // C++: Update(iter, tick)
                    // Ref: CommonClient.cpp:731
                    update(instru, updateType);

                    // C++: 当 optionStrategy && underlying 时:
                    //      ConfigParams::GetInstance()->m_simConfig->m_lastInstruMapIter = iter
                    // Ref: CommonClient.cpp:735
                    // 非 option 模式下也设置 lastInstruMapIter
                    simCfg.lastInstruMapInstrument = instru;
                }

                // ---- isStratSymbol 判定 ----
                // C++: if (!strcmp(m_configParams->m_simConfig->m_instru->m_instrument, update->m_symbol))
                // Ref: CommonClient.cpp:739-744
                boolean isStratSymbol = false;
                if (simCfg.instrument != null && symbol.equals(simCfg.instrument.instrument)) {
                    isStratSymbol = true;
                    // C++: if (m_configParams->m_simConfig->m_bUseStratBook)
                    //          m_configParams->m_simConfig->m_bUseStratBook = false;
                    // Ref: CommonClient.cpp:742-744
                    if (simCfg.bUseStratBookRuntime) {
                        simCfg.bUseStratBookRuntime = false;
                    }
                }

                // C++: if (significantUpdate && !ConfigParams::GetInstance()->m_bUseEndPkt
                //          && !(underlying && (m_Mode == ModeType_Sim || m_Mode == ModeType_Live)))
                // Ref: CommonClient.cpp:746-754
                if (significantUpdate && !configParams.useEndPkt) {
                    // C++: m_INDCallBack(&m_configParams->m_simConfig->m_indicatorList)
                    // Ref: CommonClient.cpp:752
                    if (indCallback != null) {
                        indCallback.accept(simCfg);
                    }
                    bMDUpdate = true;
                }

                // C++: if (isStratSymbol && (significantUpdate || sendAllUpdates))
                //          m_MDCallBack(update);
                // Ref: CommonClient.cpp:761
                if (isStratSymbol && (significantUpdate || sendAllUpdates)) {
                    if (mdCallback != null) {
                        mdCallback.accept(mdUpdate);
                    }
                }
            }
        } // end for simConfigList
    }

    // =======================================================================
    //  Indicator 更新
    // =======================================================================

    /**
     * 遍历合约的指标列表，根据行情类型调用 quoteUpdate 或 tickUpdate。
     * 迁移自: CommonClient::Update(InstruMapIter iter, Tick *tick)
     * Ref: CommonClient.cpp:830-846
     *
     * C++: if (tick->tickType == BIDQUOTE || tick->tickType == ASKQUOTE)
     *          for (elem : iter->second->m_indList) elem->m_indicator->QuoteUpdate(tick);
     *      else if (tick->tickType == BIDTRADE || tick->tickType == ASKTRADE)
     *          for (elem : iter->second->m_indList) elem->m_indicator->TickUpdate(tick);
     *
     * @param instru    当前合约
     * @param updateType MDDataPart.updateType — 区分报价/成交
     */
    private void update(Instrument instru, byte updateType) {
        if (instru.indList == null) return;

        // C++: tickType 由 Tick::FillTick 从 updateType 推导
        // Ref: Tick.cpp — TRADE/TRADE_INFO/TRADE_IMPLIED → BIDTRADE/ASKTRADE
        //                  其余 (ADD/MODIFY/DELETE/OVERLAY 等) → BIDQUOTE/ASKQUOTE
        // Ref: CommonClient.cpp:833-844
        boolean isTrade = (updateType == Constants.MDUPDTYPE_TRADE
                || updateType == Constants.MDUPDTYPE_TRADE_INFO
                || updateType == Constants.MDUPDTYPE_TRADE_IMPLIED);

        if (isTrade) {
            // C++: for (elem : indList) elem->m_indicator->TickUpdate(tick);
            for (IndElem elem : instru.indList) {
                elem.indicator.tickUpdate();
            }
        } else {
            // C++: for (elem : indList) elem->m_indicator->QuoteUpdate(tick);
            for (IndElem elem : instru.indList) {
                elem.indicator.quoteUpdate();
            }
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
