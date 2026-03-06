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

    // ---- Exchange Type ----
    // 迁移自: CommonClient.h:122 — char m_exchangeType
    // C++: CommonClient.cpp:850-901 — 从 simConfig.m_controlConfig.m_exchange 映射
    private byte exchangeType = 0;

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

    /**
     * 设置交易所类型。
     * 迁移自: CommonClient.cpp:850-901 — m_exchangeType 赋值
     * C++: m_exchangeType 用于 FillReqInfo() 中设置 m_reqMsg.Exchange_Type
     *
     * @param exchangeType 交易所类型字节值（如 Constants.CHINA_SHFE=57）
     */
    public void setExchangeType(byte exchangeType) {
        this.exchangeType = exchangeType;
        log.info("[CommonClient] exchangeType set to " + (exchangeType & 0xFF));
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
     * 1. endPkt==1 处理: 若 bUseEndPkt 且有待处理更新，触发 INDCallBack 并返回
     * 2. 定期检查行情时效性 (CheckLastUpdate)
     * 3. 调用 SendINDUpdate 进行 symbolID 路由
     *
     * @param mdUpdate MarketUpdateNew MemorySegment
     */
    public void sendInfraMDUpdate(MemorySegment mdUpdate) {
        long mdDataBase = Types.MU_DATA_OFFSET;

        // ---- endPkt==1 处理 ----
        // C++: if (update->m_endPkt == 1) {
        //          if (m_bMDUpdate == true) {
        //              m_bMDUpdate = false;
        //              if (ConfigParams::GetInstance()->m_bUseEndPkt && client->m_dateConfig->m_simActive == true) {
        //                  m_INDCallBack(&ConfigParams::GetInstance()->m_simConfig->m_indicatorList);
        //              }
        //          }
        //          return;
        //      }
        // Ref: CommonClient.cpp:342-357
        byte endPkt = (byte) Types.MDD_END_PKT_VH.get(mdUpdate, mdDataBase);
        if (endPkt == 1) {
            if (bMDUpdate) {
                bMDUpdate = false;
                if (configParams.useEndPkt && configParams.simConfig != null) {
                    if (indCallback != null) {
                        indCallback.accept(configParams.simConfig);
                    }
                }
            }
            return;
        }

        // ---- 行情时效性检查 ----
        // C++: if (update->m_timestamp - m_lastUpdate > ConfigParams::GetInstance()->m_updateInterval
        //          && m_Mode == ModeType_Live) {
        //          m_lastUpdate = update->m_timestamp;
        //          CheckLastUpdate();
        //      }
        // Ref: CommonClient.cpp:359-363
        long mdTimestamp = (long) Types.MDH_TIMESTAMP_VH.get(mdUpdate, 0L);
        if (mdTimestamp - lastUpdate > configParams.updateInterval
                && configParams.modeType == 2) { // ModeType_Live = 2
            lastUpdate = mdTimestamp;
            checkLastUpdate();
        }

        // C++: SendINDUpdate(update)
        // Ref: CommonClient.cpp:365
        sendINDUpdate(mdUpdate);
    }

    /**
     * 检查行情是否过期（stale market data 检测）。
     * 迁移自: CommonClient::CheckLastUpdate()
     * Ref: CommonClient.cpp:378-399
     *
     * C++ 逻辑:
     * 对所有 simConfig 的所有 instrument，检查 lastLocalTime 是否超过 updateInterval。
     * 如果在交易时段内某合约行情超时，触发策略紧急退出 (onExit+onCancel+onFlat)。
     *
     * [C++差异] C++ 调用 system("mailx ...") 发送邮件告警。
     * Java 使用日志 SEVERE 告警替代。
     */
    private void checkLastUpdate() {
        // C++: for (int i = 0; i < ConfigParams::GetInstance()->m_strategyCount; i++)
        // Ref: CommonClient.cpp:380
        if (simConfigs == null) return;
        for (SimConfig simCfg : simConfigs) {
            // C++: for (InstruMapIter iter = m_simConfig[i].m_instruMap.begin(); ...)
            // Ref: CommonClient.cpp:382
            for (Instrument instru : simCfg.instruMap.values()) {
                // C++: if (((int64_t)(m_lastUpdate - iter->second->m_instrument->lastLocalTime) > m_updateInterval)
                //          && (iter->second->m_instrument->lastLocalTime > 0)
                //          && (Watch::GetUniqueInstance()->GetCurrentTime() < m_simConfig[i].m_dateConfig.m_endTimeEpoch))
                // Ref: CommonClient.cpp:384
                long timeSinceUpdate = lastUpdate - instru.lastLocalTime;
                if (timeSinceUpdate > configParams.updateInterval
                        && instru.lastLocalTime > 0
                        && (simCfg.executionStrategy != null)) {
                    // C++: system(cmd)  — 发送邮件告警
                    // [C++差异] Java 使用 SEVERE 日志替代 mailx 邮件
                    log.severe("ALERT! Strategy exited!!! , Reason: MARKET DATA is not valid!! Symbol: "
                            + instru.origBaseName
                            + " Last Update:" + instru.lastLocalTime
                            + " Current Update:" + lastUpdate);

                    // C++: m_simConfig[i].m_execStrategy->m_onExit = true;
                    //      m_simConfig[i].m_execStrategy->m_onCancel = true;
                    //      m_simConfig[i].m_execStrategy->m_onFlat = true;
                    // Ref: CommonClient.cpp:392-394
                    if (simCfg.executionStrategy instanceof com.quantlink.trader.strategy.ExecutionStrategy es) {
                        es.onExit = true;
                        es.onCancel = true;
                        es.onFlat = true;
                    }
                }
            }
        }
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

            // ---- 全局 DateConfig UpdateActive ----
            // C++: m_dateConfig->UpdateActive(Watch::GetUniqueInstance()->GetCurrentTime())
            // Ref: CommonClient.cpp:416
            // [C++差异] C++ 有独立的全局 m_dateConfig (CommonClient.h:125)。
            // Java 中全局 dateConfig 对应 simConfigs[0] (若存在)。
            if (simConfigs != null && simConfigs.length > 0) {
                simConfigs[0].updateActive(Watch.getInstance().getCurrentTime());
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

            // C++: m_configParams->m_simConfig->m_dateConfig.UpdateActive(Watch::GetUniqueInstance()->GetCurrentTime())
            // Ref: CommonClient.cpp:435
            if (Watch.getInstance() != null) {
                simCfg.updateActive(Watch.getInstance().getCurrentTime());
            }

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
                // C++ Tick::FillTick (Tick.cpp:121-122) INVALID 判定:
                //   if (validBids == 0 || validAsks == 0 || askQty[0] == 0 || bidQty[0] == 0)
                //       tickType = INVALID;
                // [C++差异] Tick 对象未迁移。使用 FillOrderBook 后的 bid/ask 数据代替:
                //           bidQty[0]==0 或 askQty[0]==0 视为 INVALID tick，
                //           与 C++ Tick::FillTick 的 INVALID 判定对齐。
                //           tickLevel 用 MDDataPart.updateLevel 字段。
                if (instru.bidQty[0] == 0 || instru.askQty[0] == 0) {
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

    /**
     * 请求队列回调 — 将发出的订单通知给 CommonBook 进行 self-book 过滤。
     * 迁移自: CommonClient.cpp:256-274 — SendInfraReqUpdate(RequestMsg*)
     *
     * C++: void SendInfraReqUpdate(RequestMsg *request) {
     *          if (m_configParams->m_bCommonBook && m_dateConfig->m_simActive) {
     *              if (request->Quantity < 0) { request->Quantity *= -1; request->OrderID += 1000000; }
     *              auto iter = m_configParams->m_instruCBMap.find(request->Contract_Description.Symbol);
     *              if (iter != end) iter->second->RequestCallBack(request);
     *          }
     *      }
     *
     * [C++差异] 此方法仅在 m_bCommonBook=true 时执行实际逻辑。
     * CommonBook 已明确排除在中国期货场景之外 (CommonClient.java:282)，
     * 因此当前为空壳实现。后续如迁移 CommonBook 功能，需补齐内部逻辑。
     */
    public void sendInfraReqUpdate(MemorySegment request) {
        // C++: if (m_configParams->m_bCommonBook && m_dateConfig->m_simActive) { ... }
        // 中国期货场景 bCommonBook=false，不执行
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
        return sendNewOrder(strategyID, symbol, side, price, qty, posDirection,
                strategy, 0, 0, 0, "", "", "", OrderHitType.STANDARD);
    }

    /**
     * 发送新订单（完整参数版本）。
     * 迁移自: CommonClient::SendNewOrder(uint32_t, const char*, OptionType, const char*,
     *          TransactionType, double, int32_t, int32_t, int32_t, const char*, int32_t,
     *          OrderHitType, ExecutionStrategy*)
     * Ref: CommonClient.cpp:918-989
     *
     * @param token       合约 token (C++: m_instru->m_token)
     * @param expiryDate  到期日 (C++: m_instru->m_expiryDate)
     * @param strikePrice 行权价 (C++: m_instru->m_strike)
     * @param account     账户ID (C++: m_account)
     * @param instruType  合约类型 (C++: m_instruType, 6 chars)
     * @param product     产品名 (C++: execStrategy->m_product)
     * @param ordHitType  命中类型 (C++: OrderHitType — STANDARD/CROSS)
     */
    public int sendNewOrder(int strategyID, String symbol, int side, double price,
                            int qty, int posDirection, Object strategy,
                            int token, int expiryDate, int strikePrice,
                            String account, String instruType, String product,
                            OrderHitType ordHitType) {
        // 清零请求缓冲
        reqMsg.fill((byte) 0);

        // C++: m_reqMsg.Request_Type = NEWORDER  (CommonClient.cpp:920)
        Types.REQ_REQUEST_TYPE_VH.set(reqMsg, 0L, Constants.REQUEST_NEWORDER);

        // C++: m_reqMsg.Token = Token  (CommonClient.cpp:922)
        Types.REQ_TOKEN_VH.set(reqMsg, 0L, token);

        // C++: m_reqMsg.Transaction_Type = ConvertSide(side)  (CommonClient.cpp:923)
        Types.REQ_TRANSACTION_TYPE_VH.set(reqMsg, 0L, (byte) side);

        // C++: m_reqMsg.Price = price  (CommonClient.cpp:924)
        Types.REQ_PRICE_VH.set(reqMsg, 0L, price);

        // C++: m_reqMsg.Quantity = qty  (CommonClient.cpp:925)
        Types.REQ_QUANTITY_VH.set(reqMsg, 0L, qty);

        // C++: m_reqMsg.QuantityFilled = 0  (CommonClient.cpp:926)
        Types.REQ_QUANTITY_FILLED_VH.set(reqMsg, 0L, 0);

        // C++: m_reqMsg.DisclosedQnty = m_reqMsg.Quantity  (CommonClient.cpp:927)
        Types.REQ_DISCLOSED_QNTY_VH.set(reqMsg, 0L, qty);

        // C++: memcpy(m_reqMsg.Product, execStrategy->m_product, 32)  (CommonClient.cpp:929)
        if (product != null && !product.isEmpty()) {
            byte[] productBytes = product.getBytes(StandardCharsets.US_ASCII);
            int len = Math.min(productBytes.length, Constants.MAX_PRODUCT_SIZE);
            reqMsg.asSlice(Types.REQ_PRODUCT_OFFSET, len)
                  .copyFrom(MemorySegment.ofArray(productBytes).asSlice(0, len));
        }

        // C++: m_reqMsg.StrategyID = execStrategy->m_strategyID  (CommonClient.cpp:930)
        Types.REQ_STRATEGY_ID_VH.set(reqMsg, 0L, strategyID);

        // C++: memcpy(m_reqMsg.AccountID, Account, strlen(Account))  (CommonClient.cpp:932-933)
        if (account != null && !account.isEmpty()) {
            byte[] accBytes = account.getBytes(StandardCharsets.US_ASCII);
            int len = Math.min(accBytes.length, Constants.ACCOUNT_ID_SIZE);
            reqMsg.asSlice(Types.REQ_ACCOUNT_ID_OFFSET, len)
                  .copyFrom(MemorySegment.ofArray(accBytes).asSlice(0, len));
        }

        // C++: memcpy(m_reqMsg.Contract_Description.InstrumentName, instType, 6)  (CommonClient.cpp:937-938)
        if (instruType != null && !instruType.isEmpty()) {
            byte[] instTypeBytes = instruType.getBytes(StandardCharsets.US_ASCII);
            int len = Math.min(instTypeBytes.length, 6);
            reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_INSTRUMENT_NAME_OFFSET, len)
                  .copyFrom(MemorySegment.ofArray(instTypeBytes).asSlice(0, len));
        }

        // C++: memcpy(m_reqMsg.Contract_Description.Symbol, ticker, strlen(ticker))  (CommonClient.cpp:940)
        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symBytes.length)
              .copyFrom(MemorySegment.ofArray(symBytes));

        // C++: Contract_Description.OptionType = "XX"  (CommonClient.cpp:946)
        // [C++差异] Java 版本默认写 "XX"（期货），期权场景由调用方处理
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_OPTION_TYPE_OFFSET, 2)
              .copyFrom(MemorySegment.ofArray(new byte[]{'X', 'X'}));

        // C++: m_reqMsg.Contract_Description.CALevel = 0  (CommonClient.cpp:961)
        // CD_CA_LEVEL_VH 是 JAVA_SHORT
        reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_CA_LEVEL_OFFSET, 2)
              .copyFrom(MemorySegment.ofArray(new byte[]{0, 0}));

        // C++: m_reqMsg.Contract_Description.ExpiryDate = Exp  (CommonClient.cpp:962)
        MemorySegment cdSlice = reqMsg.asSlice(Types.REQ_CONTRACT_DESC_OFFSET, Types.CONTRACT_DESC_SIZE);
        Types.CD_EXPIRY_DATE_VH.set(cdSlice, 0L, expiryDate);

        // C++: m_reqMsg.Contract_Description.StrikePrice = sp  (CommonClient.cpp:963)
        Types.CD_STRIKE_PRICE_VH.set(cdSlice, 0L, strikePrice);

        // C++: m_reqMsg.TimeStamp = getcurtime()  (CommonClient.cpp:964)
        Types.REQ_TIMESTAMP_VH.set(reqMsg, 0L, System.nanoTime());

        // C++: if (ordHitType == CROSS) m_reqMsg.Duration = FAK; else m_reqMsg.Duration = DAY;
        // Ref: CommonClient.cpp:966-969
        if (ordHitType == OrderHitType.CROSS) {
            Types.REQ_DURATION_VH.set(reqMsg, 0L, Constants.DUR_FAK);
        } else {
            Types.REQ_DURATION_VH.set(reqMsg, 0L, Constants.DUR_DAY);
        }

        // C++: FillReqInfo()  (CommonClient.cpp:971)
        // C++: m_reqMsg.OrdType = LIMIT; m_reqMsg.PxType = PERUNIT; m_reqMsg.Exchange_Type = m_exchangeType;
        Types.REQ_ORD_TYPE_VH.set(reqMsg, 0L, Constants.ORD_LIMIT);
        Types.REQ_PX_TYPE_VH.set(reqMsg, 0L, Constants.PX_PERUNIT);
        Types.REQ_EXCHANGE_TYPE_VH.set(reqMsg, 0L, exchangeType);

        // C++: CFFEX override  (CommonClient.cpp:973-974)
        // if (symbol starts with IF/IC/IM/IH) Exchange_Type = CHINA_CFFEX
        if (symbol.startsWith("IF") || symbol.startsWith("IC")
                || symbol.startsWith("IM") || symbol.startsWith("IH")) {
            Types.REQ_EXCHANGE_TYPE_VH.set(reqMsg, 0L, Constants.CHINA_CFFEX);
        }

        // C++: PosDirection
        Types.REQ_POS_DIRECTION_VH.set(reqMsg, 0L, posDirection);

        // 委托 Connector 发送
        // C++: OrderID = m_connector->SendNewOrder(m_reqMsg)  (CommonClient.cpp:976)
        int orderID = connector.sendNewOrder(reqMsg);

        // 注册到 orderID→strategy 映射
        // C++: m_configParams->m_orderIDStrategyMap[orderID] = strategy
        configParams.orderIDStrategyMap.put(orderID, strategy);

        log.info(String.format("CommonClient SendNewOrder, OrderID: %d, product: %s, StrategyID: %d, Symbol: %s, Quantity: %d",
                orderID, product, strategyID, symbol, qty));

        return orderID;
    }

    /**
     * 命中类型枚举（用于 Duration FAK/DAY 判断）。
     * 迁移自: hftbase CommonUtils — enum OrderHitType
     */
    public enum OrderHitType {
        STANDARD, IMPROVE, CROSS, DETECT, MATCH
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

        // C++: m_reqMsg.Exchange_Type = m_exchangeType (FillReqInfo, CommonClient.cpp:1117)
        Types.REQ_EXCHANGE_TYPE_VH.set(reqMsg, 0L, exchangeType);

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

        // C++: m_reqMsg.Exchange_Type = m_exchangeType (FillReqInfo, CommonClient.cpp:1117)
        Types.REQ_EXCHANGE_TYPE_VH.set(reqMsg, 0L, exchangeType);

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
