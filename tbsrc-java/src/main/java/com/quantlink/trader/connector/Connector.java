package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;
import com.quantlink.trader.core.Instrument;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.logging.Logger;

/**
 * SysV MWMR SHM Connector -- 行情接收、订单发送、回报轮询。
 * <p>
 * 迁移自: hftbase/Connector/include/connector.h  (illuminati::Connector)
 * 迁移自: hftbase/Connector/src/connector.cpp
 * <p>
 * C++ 类核心职责:
 * <ul>
 *   <li>持有 ShmMgr m_shmMgr (MultiClientStoreShmReader) 管理所有 SHM 队列和线程</li>
 *   <li>通过 connectWithParam 将 HandleUpdates/HandleOrderResponse 注册为 ShmMgr 回调</li>
 *   <li>HandleUpdates: 按 m_interestedsymbols_for_md 过滤行情、覆写 symbolID 后回调策略</li>
 *   <li>HandleOrderResponse: 按 clientId/Account/Symbol 过滤回报后回调策略</li>
 *   <li>SendNewOrder/SendModifyOrder/SendCancelOrder: 写 RequestMsg 到 m_requestQueue</li>
 * </ul>
 * <p>
 * [C++差异] C++ Connector 支持多种 InteractionMode (LIVE/SIMULATION/PAPERTRADING/PARALLELSIM)，
 *           Java 版本仅实现 LIVE 模式（SHM 直连），回测通过独立的 BacktestConnector 实现。
 * <p>
 * [C++差异] C++ m_OrderCount 是 uint32_t 非原子变量（单线程使用），
 *           Java 使用 AtomicInteger 以支持潜在的多线程发单场景。
 */
public class Connector {

    private static final Logger log = Logger.getLogger(Connector.class.getName());

    // =======================================================================
    //  回调接口
    // =======================================================================

    /**
     * 行情回调。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:58
     * C++: typedef std::tr1::function&lt;void(illuminati::md::MarketUpdateNew *)&gt; MDConnection;
     */
    @FunctionalInterface
    public interface MDCallback {
        void onMarketData(MemorySegment marketUpdate);
    }

    /**
     * 订单回报回调。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:59
     * C++: typedef std::tr1::function&lt;void(illuminati::infra::ResponseMsg *)&gt; ORSConnection;
     */
    @FunctionalInterface
    public interface ORSCallback {
        void onOrderResponse(MemorySegment response);
    }

    // =======================================================================
    //  枚举 (对齐 C++ connectorconfig.h)
    // =======================================================================

    /**
     * 回报过滤类型。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h:37-41
     * C++: enum ResponseFilterType { STRATEGY_FILTER, TICKERS_ON_ONE_ACCOUNT_FILTER };
     */
    public static final int STRATEGY_FILTER = 0;
    public static final int TICKERS_ON_ONE_ACCOUNT_FILTER = 1;

    /**
     * 行情 SHM 读取模式。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h:43-47
     * C++: enum ReadMDShmMode { UNTIL_ENDPACKET, ROUND_ROBIN };
     */
    public static final int MD_READ_UNTIL_ENDPACKET = 0;
    public static final int MD_READ_ROUND_ROBIN = 1;

    // =======================================================================
    //  配置
    // =======================================================================

    /**
     * 单个交易所的 SHM 配置。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h EXCH_* MAP 条目
     */
    public static class ExchangeConfig {
        /** 交易所名称 (如 "CHINA_SHFE") */
        public String exchangeName;
        /** 行情 SHM key 列表 */
        public List<Integer> mdShmKeys = new ArrayList<>();
        /** 行情 SHM 队列大小列表 */
        public List<Integer> mdShmSizes = new ArrayList<>();
        /** 行情 SHM 读取模式列表 (MD_READ_ROUND_ROBIN / MD_READ_UNTIL_ENDPACKET) */
        public List<Integer> mdShmReadModes = new ArrayList<>();
        /** 订单请求 SHM key */
        public int reqShmKey;
        /** 订单请求队列容量 */
        public int reqQueueSize;
        /** 订单回报 SHM key */
        public int respShmKey;
        /** 订单回报队列容量 */
        public int respQueueSize;
        /** ClientStore SHM key */
        public int clientStoreShmKey;
    }

    /**
     * Connector 配置。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h (ConnectorConfig)
     * <p>
     * C++ ConnectorConfig 包含: INTERESTED_EXCHANGES → EXCH_*_MAP 多交易所映射、
     * RESPONSE_FILTER_TYPE、INTERESTED_ACCOUNT、INTERESTED_SYMBOLS_FOR_ORS、
     * ASYNC_MD_AND_RESPONSE 等字段。
     */
    /**
     * Connector 配置。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h (ConnectorConfig)
     * 字段与 C++ ConnectorConfig 一一对应。
     */
    public static class Config {
        /**
         * 关注的交易所配置列表。
         * 迁移自: C++ set<string> INTERESTED_EXCHANGES + EXCH_*_MAP
         * Ref: hftbase/Connector/include/connectorconfig.h:533, 520-527
         */
        public List<ExchangeConfig> exchanges = new ArrayList<>();

        /**
         * 关注的合约列表（行情过滤用）。
         * 迁移自: C++ set<string> INTERESTED_SYMBOLS
         * Ref: hftbase/Connector/include/connectorconfig.h:531
         * <p>
         * C++ 构造函数中: m_interestedsymbols_for_md.create_map(INTERESTED_SYMBOLS);
         * 按顺序自动分配 symbolID (0, 1, 2, ...)。
         */
        public Set<String> interestedSymbols = new LinkedHashSet<>();

        /**
         * ORS 合约过滤集（TICKERS_ON_ONE_ACCOUNT_FILTER 模式使用）。
         * 迁移自: C++ set<string> INTERESTED_SYMBOLS_FOR_ORS
         * Ref: hftbase/Connector/include/connectorconfig.h:532
         */
        public Set<String> interestedSymbolsForOrs = new HashSet<>();

        /**
         * 回报过滤模式。
         * 迁移自: C++ ResponseFilterType RESPONSE_FILTER_TYPE
         * Ref: hftbase/Connector/include/connectorconfig.h:546
         */
        public int responseFilterType = STRATEGY_FILTER;

        /**
         * TICKERS_ON_ONE_ACCOUNT_FILTER 模式的账户 ID。
         * 迁移自: C++ string INTERESTED_ACCOUNT
         * Ref: hftbase/Connector/include/connectorconfig.h:534
         */
        public String interestedAccount = "";

        /**
         * 是否异步分离 MD 和 Response 线程。
         * 迁移自: C++ bool ASYNC_MD_AND_RESPONSE
         * Ref: hftbase/Connector/include/connectorconfig.h:552
         */
        public boolean asyncMdAndResponse = false;

        /**
         * 是否启用请求回调。
         * 迁移自: C++ bool REQUEST_CALLBACK
         * Ref: hftbase/Connector/include/connectorconfig.h:551
         */
        public boolean requestCallback = false;
    }

    // =======================================================================
    //  实例字段
    // =======================================================================

    /**
     * SHM 管理器 -- 管理所有 SHM 队列注册、轮询线程、回调分发。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:401
     * C++: ShmMgr m_shmMgr;
     */
    private final MultiClientStoreShmReader shmMgr;

    /**
     * 多交易所请求队列映射 -- C++: m_requestQueue[MAX_EXCHANGE_COUNT]
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:390
     * C++: ShmMgr::ReqShmQ *m_requestQueue[illuminati::md::MAX_EXCHANGE_COUNT];
     * <p>
     * Key: exchId (unsigned char, 0-71), Value: MWMRQueue 引用。
     */
    private final Map<Integer, MWMRQueue> requestQueues = new HashMap<>();

    /**
     * 按 exchCode 索引的 clientId 映射。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:380
     * C++: uint32_t m_clientId[illuminati::md::MAX_EXCHANGE_COUNT];
     */
    private final Map<Integer, Integer> clientIdMap = new HashMap<>();

    /**
     * 策略关心的合约过滤表: symbol → 本地 symbolID。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:375
     * C++: ds::fixed_string_map<uint16_t> m_interestedsymbols_for_md;
     */
    private final Map<String, Short> interestedSymbolsForMd = new HashMap<>();

    /**
     * ORS 合约过滤集（TICKERS_ON_ONE_ACCOUNT_FILTER 模式使用）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:396
     * C++: HashSet m_interestedsymbols_for_ors;
     */
    private final Set<String> interestedSymbolsForOrs = new HashSet<>();

    /**
     * 按交易所维度存储的所有历史 clientId。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:381
     * C++: uint32_t m_all_clientIds[MAX_EXCHANGE_COUNT][MAX_ORS_CLIENTS];
     * <p>
     * key = exchId, value = 该交易所上注册过的所有 clientId。
     * 在 HandleOrderResponse STRATEGY_FILTER 分支中，C++ 用
     *   exchId = m_response_queue_to_exchange_map[queueNum]
     * 然后只检查 m_all_clientIds[exchId]，Java 必须对齐此行为。
     */
    private final Map<Integer, Set<Integer>> allClientIdsByExchange = new HashMap<>();

    /**
     * 回报队列索引 → 交易所 ID 映射。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:387
     * C++: unsigned char m_response_queue_to_exchange_map[illuminati::md::MAX_EXCHANGE_COUNT];
     */
    private final int[] responseQueueToExchangeMap = new int[Constants.MAX_EXCHANGE_COUNT];

    /**
     * 订单计数器，用于生成唯一 OrderID。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:383
     * C++: uint32_t m_OrderCount;
     * <p>
     * [C++差异] C++ 使用 uint32_t (非原子)，Java 使用 AtomicInteger。
     */
    private final AtomicInteger orderCount = new AtomicInteger(0);

    /**
     * 行情序列号。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:379
     * C++: int64_t m_mdSeqNum;
     */
    private long mdSeqNum = 0;

    /**
     * 回报过滤类型。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:385
     * C++: ResponseFilterType m_responseFilterType;
     */
    private int responseFilterType = STRATEGY_FILTER;

    /**
     * 是否为所有合约运行（无 symbol 过滤）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:404
     * C++: bool m_runForAllSymbols;
     */
    private boolean runForAllSymbols = false;

    /**
     * 是否启用实盘请求回调。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:405
     * C++: bool m_liveReqCb;
     */
    private boolean liveReqCb = false;

    /**
     * Connector 配置引用。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:400
     * C++: ConnectorConfig *m_cfg;
     */
    private final Config config;

    /** 行情回调 -- C++: MDConnection m_mdcb (connector.h:376) */
    private final MDCallback mdCallback;

    /** 订单回报回调 -- C++: ORSConnection m_orscb (connector.h:377) */
    private final ORSCallback orsCallback;

    // =======================================================================
    //  构造函数
    // =======================================================================

    /**
     * 构造函数 — 对齐 C++ Connector(MDConnection, ORSConnection, InteractionMode, ConnectorConfig *)。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:73
     * C++: Connector(MDConnection, ORSConnection, InteractionMode, ConnectorConfig *);
     * Ref: hftbase/Connector/src/connector.cpp:25-211
     * <p>
     * [C++差异] C++ 接收 InteractionMode 参数支持 LIVE/SIMULATION/REPLAY 等模式，
     *           Java 仅实现 LIVE 模式，因此省略 InteractionMode 参数。
     *
     * @param mdCb  行情回调 — C++: MDConnection
     * @param orsCb 回报回调 — C++: ORSConnection
     * @param cfg   配置     — C++: ConnectorConfig *
     */
    public Connector(MDCallback mdCb, ORSCallback orsCb, Config cfg) {
        this.mdCallback = mdCb;
        this.orsCallback = orsCb;
        this.config = cfg;

        // C++: m_cfg = new ConnectorConfig(); *m_cfg = *cfg;
        // Ref: hftbase/Connector/src/connector.cpp:27-28

        // C++: for (i=0; i<MAX_EXCHANGE_COUNT) { m_requestQueue[i]=NULL; m_clientId[i]=0;
        //          m_response_queue_to_exchange_map[i]=0;
        //          for (j=0; j<MAX_ORS_CLIENTS) m_all_clientIds[i][j]=DEFAULT; }
        // Ref: hftbase/Connector/src/connector.cpp:35-46
        Arrays.fill(responseQueueToExchangeMap, 0);

        // C++: m_interestedsymbols_for_md.create_map(m_cfg->INTERESTED_SYMBOLS);
        //      uint16_t symbolid = 0;
        //      for (auto i = INTERESTED_SYMBOLS.begin(); ...) val->val = symbolid++;
        // Ref: hftbase/Connector/src/connector.cpp:48-62
        {
            short symbolId = 0;
            for (String symbol : cfg.interestedSymbols) {
                interestedSymbolsForMd.put(symbol, symbolId++);
            }
        }

        // C++: for (auto i = INTERESTED_SYMBOLS_FOR_ORS.begin(); ...) m_interestedsymbols_for_ors.insert(*i);
        // Ref: hftbase/Connector/src/connector.cpp:64-69
        this.interestedSymbolsForOrs.addAll(cfg.interestedSymbolsForOrs);

        // C++: ShmMgr m_shmMgr; (栈上构造，MAXSIZE = MAX_ORS_CLIENTS)
        // Ref: hftbase/Connector/include/connector.h:39-43, 401
        this.shmMgr = new MultiClientStoreShmReader(
                Constants.MAX_ORS_CLIENTS,
                Types.MARKET_UPDATE_NEW_SIZE, Types.REQUEST_MSG_SIZE, Types.RESPONSE_MSG_SIZE,
                Types.QUEUE_ELEM_MD_SIZE, Types.QUEUE_ELEM_REQ_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // ---- LIVE 模式: 遍历 INTERESTED_EXCHANGES 注册 SHM 队列 ----
        // C++: if (m_interactionMode == LIVE) { ... }
        // Ref: hftbase/Connector/src/connector.cpp:71-107
        int exchCount = 0;
        for (ExchangeConfig exchCfg : cfg.exchanges) {
            int exchId = Constants.getExchangeIdFromName(exchCfg.exchangeName);

            // C++: m_shmMgr.initClientStore(clientStoreKey);
            // Ref: hftbase/Connector/src/connector.cpp:79
            shmMgr.initClientStore(exchCfg.clientStoreShmKey);

            // C++: for (i=0; i<mdmshmkeylist.size(); i++) {
            //          if (readmode == ROUND_ROBIN) registerMDClient(...)
            //          else if (readmode == UNTIL_ENDPACKET) registerMDWithEndPacketClient(...)
            //      }
            // Ref: hftbase/Connector/src/connector.cpp:84-90
            for (int i = 0; i < exchCfg.mdShmKeys.size(); i++) {
                int readMode = (i < exchCfg.mdShmReadModes.size())
                        ? exchCfg.mdShmReadModes.get(i) : MD_READ_ROUND_ROBIN;
                if (readMode == MD_READ_UNTIL_ENDPACKET) {
                    shmMgr.registerMDWithEndPacketClient(exchCfg.mdShmKeys.get(i), exchCfg.mdShmSizes.get(i));
                } else {
                    shmMgr.registerMDClient(exchCfg.mdShmKeys.get(i), exchCfg.mdShmSizes.get(i));
                }
            }

            // C++: m_shmMgr.registerResponseClient(...)
            // Ref: hftbase/Connector/src/connector.cpp:92
            shmMgr.registerResponseClient(exchCfg.respShmKey, exchCfg.respQueueSize);

            // C++: m_response_queue_to_exchange_map[exchCount++] = exchId;
            // Ref: hftbase/Connector/src/connector.cpp:93
            responseQueueToExchangeMap[exchCount++] = exchId;

            // C++: m_requestQueue[exchId] = m_shmMgr.registerRequestClient(shmreqkey, size, m_clientId[exchId], clientStoreKey);
            // Ref: hftbase/Connector/src/connector.cpp:94-97
            int clientId = shmMgr.registerRequestClient(
                    exchCfg.reqShmKey, exchCfg.reqQueueSize, exchCfg.clientStoreShmKey);
            int reqIdx = shmMgr.getReqClientCount() - 1;
            requestQueues.put(exchId, shmMgr.getReqQueue(reqIdx));

            // C++: m_clientId[exchId] = clientId (通过引用设置)
            clientIdMap.put(exchId, clientId);

            // C++: for (i=0; i<MAX_ORS_CLIENTS; i++) {
            //          if (m_all_clientIds[exchId][i] == DEFAULT) { m_all_clientIds[exchId][i] = m_clientId[exchId]; break; }
            //      }
            // Ref: hftbase/Connector/src/connector.cpp:99-106
            allClientIdsByExchange.computeIfAbsent(exchId, k -> new HashSet<>()).add(clientId);
        }

        // C++: connectWithParam(&m_shmMgr, this, &Connector::HandleOrderResponse, ...);
        // Ref: hftbase/Connector/src/connector.cpp:109-111
        shmMgr.setORSResponseCallback(this::handleOrderResponse);

        // C++: connectWithParam(&m_shmMgr, this, &Connector::HandleUpdates, ...);
        // Ref: hftbase/Connector/src/connector.cpp:112-113
        shmMgr.setMDCallback(this::handleUpdates);

        // C++: m_runForAllSymbols = (m_cfg->INTERESTED_SYMBOLS.size() == 0) ? true : false;
        // Ref: hftbase/Connector/src/connector.cpp:209
        this.runForAllSymbols = cfg.interestedSymbols.isEmpty();

        // C++: m_responseFilterType = m_cfg->RESPONSE_FILTER_TYPE;
        // Ref: hftbase/Connector/src/connector.cpp:210
        this.responseFilterType = cfg.responseFilterType;

        // C++: m_liveReqCb = m_cfg->REQUEST_CALLBACK; (仅 GUI 构造函数，connector.cpp:329)
        this.liveReqCb = cfg.requestCallback;
    }


    // =======================================================================
    //  发单接口
    // =======================================================================

    /**
     * 发送新订单请求。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:271-277
     * C++:
     * <pre>
     *   uint32_t SendNewOrder(illuminati::infra::RequestMsg &amp;stratReq) {
     *       stratReq.OrderID = GetUniqueOrderNumber(stratReq.Exchange_Type);
     *       stratReq.Request_Type = illuminati::infra::NEWORDER;
     *       PushRequest(stratReq);
     *       return stratReq.OrderID;
     *   }
     * </pre>
     */
    public int sendNewOrder(MemorySegment req) {
        // C++: stratReq.OrderID = GetUniqueOrderNumber(stratReq.Exchange_Type);
        // Ref: hftbase/Connector/include/connector.h:273
        int exchType = ((byte) Types.REQ_EXCHANGE_TYPE_VH.get(req, 0L)) & 0xFF;
        int orderId = getUniqueOrderNumber(exchType);
        if (orderId < 0) {
            log.severe("[sendNewOrder] OrderID 分配失败，拒绝发单");
            return -1;
        }

        // C++: stratReq.Request_Type = illuminati::infra::NEWORDER;
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_NEWORDER);

        // C++: stratReq.OrderID = orderid;
        Types.REQ_ORDER_ID_VH.set(req, 0L, orderId);

        // C++: PushRequest(stratReq);
        // Ref: hftbase/Connector/include/connector.h:200-220
        pushRequest(req);

        return orderId;
    }

    /**
     * 发送撤单请求。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:318-322
     */
    public void sendCancelOrder(MemorySegment req) {
        // C++: stratReq.Request_Type = illuminati::infra::CANCELORDER;
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_CANCELORDER);

        // C++: PushRequest(stratReq);
        pushRequest(req);
    }

    /**
     * 发送改单请求。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:299-303
     */
    public void sendModifyOrder(MemorySegment req) {
        // C++: stratReq.Request_Type = infra::MODIFYORDER;
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_MODIFYORDER);

        // C++: PushRequest(stratReq);
        pushRequest(req);
    }

    /**
     * 写请求到 SHM 队列。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:200-220
     * C++:
     * <pre>
     *   inline void PushRequest(RequestMsg &amp;msg) {
     *       msg.TimeStamp = ITime_ClockRT::GetCurrentTime();
     *       m_requestQueue[msg.Exchange_Type]-&gt;enqueue(msg);
     *   }
     * </pre>
     *
     * @param req RequestMsg MemorySegment
     */
    public void pushRequest(MemorySegment req) {
        // C++: msg.TimeStamp = illuminati::ITime_ClockRT::GetCurrentTime();
        // ITime_ClockRT 使用 CLOCK_REALTIME (wall clock epoch nanoseconds)
        // Ref: hftbase/CommonUtils/include/itimer.h:147-152
        Types.REQ_TIMESTAMP_VH.set(req, 0L, System.currentTimeMillis() * 1_000_000L);

        // C++: m_requestQueue[msg.Exchange_Type]->enqueue(msg);
        int exchType = ((byte) Types.REQ_EXCHANGE_TYPE_VH.get(req, 0L)) & 0xFF;
        MWMRQueue q = requestQueues.get(exchType);
        if (q == null) {
            log.severe("[PushRequest] no request queue for Exchange_Type=" + exchType);
            return;
        }
        q.enqueue(req);
    }

    // =======================================================================
    //  轮询控制
    // =======================================================================

    /**
     * 启动行情和回报轮询线程。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:594-676 — Connector::StartAsync()
     * C++ LIVE 模式:
     * <pre>
     *   if (!(m_cfg->ASYNC_MD_AND_RESPONSE))
     *       m_shmMgr.startMonitorAsyncMarketDataAndResponse();
     *   else {
     *       m_shmMgr.startMonitorAsyncMarketData();
     *       m_shmMgr.startMonitorAsyncORSResponse();
     *   }
     * </pre>
     * Ref: hftbase/Connector/src/connector.cpp:601-614
     * <p>
     * [C++差异-仅LIVE模式，省略 SimEngineType 参数]
     * C++ 签名: StartAsync(SimEngineType = TBTEngine)，Java 仅实现 LIVE 模式，省略参数。
     */
    public void startAsync() {
        // C++: if (!(m_cfg->ASYNC_MD_AND_RESPONSE))
        // Ref: hftbase/Connector/src/connector.cpp:601-614
        if (!config.asyncMdAndResponse) {
            // C++: m_shmMgr.startMonitorAsyncMarketDataAndResponse();
            shmMgr.startMonitorAsyncMarketDataAndResponse();
        } else {
            // C++: m_shmMgr.startMonitorAsyncMarketData();
            shmMgr.startMonitorAsyncMarketData();
            // C++: m_shmMgr.startMonitorAsyncORSResponse();
            shmMgr.startMonitorAsyncORSResponse();
        }

        // C++: if (m_interactionMode == GUI || m_liveReqCb) {
        //          m_shmMgr.startMonitorAsyncORSRequest();
        //      }
        // Ref: hftbase/Connector/src/connector.cpp:615-621
        if (liveReqCb) {
            shmMgr.startMonitorAsyncORSRequest();
        }
    }

    /**
     * 停止轮询线程。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:678-684 — Connector::Stop()
     */
    public void stop() {
        shmMgr.shutdown();
    }

    // =======================================================================
    //  D1: HandleORSRequests (C++ REQUEST_CALLBACK 模式回调)
    // =======================================================================

    /**
     * ORS 请求回调 — REQUEST_CALLBACK 模式下由 ShmMgr 调用。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:261
     * C++: void HandleORSRequests(RequestMsg *msg);
     * Ref: hftbase/Connector/src/connector.cpp:892-920
     * <p>
     * 当前业务不使用 REQUEST_CALLBACK 模式，方法体为空。
     * C++ 实现中此方法在 GUI 模式下被调用，用于实时显示请求队列状态。
     *
     * @param request RequestMsg MemorySegment
     */
    public void handleORSRequests(MemorySegment request) {
        // C++: 在 REQUEST_CALLBACK=true 或 GUI 模式下被 ShmMgr 回调
        // 当前 Java 仅实现 LIVE 模式，此回调为空实现
    }

    // =======================================================================
    //  D2: PushRequest 批量版
    // =======================================================================

    /**
     * 批量写请求到 SHM 队列。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:172-190
     * C++:
     * <pre>
     *   inline void PushRequest(unsigned char extype, Container&lt;RequestMsg&gt; &amp;reqlist) {
     *       for (auto &amp;req : reqlist) {
     *           req.TimeStamp = ITime_ClockRT::GetCurrentTime();
     *           m_requestQueue[extype]-&gt;enqueue(req);
     *       }
     *   }
     * </pre>
     *
     * @param exchType 交易所类型（unsigned char, C++: extype）
     * @param requests RequestMsg 列表
     */
    public void pushRequest(int exchType, List<MemorySegment> requests) {
        MWMRQueue q = requestQueues.get(exchType);
        if (q == null) {
            log.severe("[PushRequest-batch] no request queue for Exchange_Type=" + exchType);
            return;
        }
        for (MemorySegment req : requests) {
            // C++: req.TimeStamp = ITime_ClockRT::GetCurrentTime();
            Types.REQ_TIMESTAMP_VH.set(req, 0L, System.currentTimeMillis() * 1_000_000L);
            // C++: m_requestQueue[extype]->enqueue(req);
            q.enqueue(req);
        }
    }

    // =======================================================================
    //  C++ 同步轮询 / 信号处理方法
    // =======================================================================

    /**
     * 同步启动行情和回报轮询（阻塞当前线程）。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:520-592 — Connector::StartSync()
     * C++: void StartSync(SimEngineType = TBTEngine);
     * <p>
     * [C++差异-仅LIVE模式，省略 SimEngineType 参数]
     * C++ 签名接收 SimEngineType 参数，Java 仅实现 LIVE 模式，省略参数。
     * <p>
     * 当前 Java 策略使用 startAsync() 异步轮询，此方法为空实现保持 C++ 签名对齐。
     */
    public void startSync() {
        // C++: LIVE 模式下调用 m_shmMgr.startMonitorSyncMarketDataAndResponse()
        // Java 策略使用 startAsync() 异步模式，此处保留空实现
        log.warning("[Connector] startSync() 未实现，请使用 startAsync()");
    }

    /**
     * 阻塞 POSIX 信号。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:478-480 — Connector::BlockSignals()
     * C++: void BlockSignals();
     * <p>
     * [C++差异-语言适配] C++ 使用 POSIX sigprocmask 阻塞信号，
     * Java 信号处理在 TraderMain 中通过 sun.misc.Signal 实现，无需在 Connector 层阻塞。
     */
    public void blockSignals() {
        // C++: sigprocmask(SIG_BLOCK, &m_signalSet, NULL);
        // Java 信号处理不在 Connector 层实现
    }

    /**
     * 信号处理循环。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:686-746 — Connector::HandleSignals()
     * C++: void HandleSignals(SquareOffConnection sqoff, SimEngineType = TBTEngine);
     * <p>
     * [C++差异-语言适配] C++ 使用 POSIX sigwait 循环处理 SIGUSR1/SIGUSR2/SIGTSTP/SIGINT/SIGTERM，
     * Java 信号处理在 TraderMain.registerSignalHandlers() 中实现。
     * 保留方法签名以对齐 C++。
     */
    public void handleSignals() {
        // C++: sigwait 循环，Java 在 TraderMain 中实现
    }

    /**
     * 获取交易所品种列表。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:508-518 — Connector::getSymbolList()
     * C++: int32_t getSymbolList(string &exchange, string &date, set<string> &symbollist);
     * <p>
     * [C++差异-语言适配] C++ 通过 API 查询交易所品种，Java 通过配置文件获取品种列表，
     * 无需实时查询。保留方法签名以对齐 C++。
     *
     * @param exchange  交易所名称
     * @param date      日期
     * @param symbolList 输出参数: 品种集合
     * @return 品种数量（当前返回 0）
     */
    public int getSymbolList(String exchange, String date, Set<String> symbolList) {
        // C++ 通过 ORS API 查询交易所品种列表
        // Java 通过配置文件获取，此处保留空实现
        return 0;
    }

    // =======================================================================
    //  D10: C++ 公有方法空实现
    // =======================================================================

    /**
     * 设置合约缓存（仅对俄罗斯交易所有效）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:98
     * C++: void setInstrumentCache(InstrumentCache *cache);
     * <p>
     * 仅对 MICEX/FORTS 交易所有效，中国期货不使用。空实现保持 C++ 方法签名对齐。
     */
    public void setInstrumentCache() {
        // C++ 实现仅在俄罗斯交易所场景使用
    }

    /**
     * 检查是否在俄罗斯交易所运行。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:100
     * C++: bool ifRunningForRussianExchange(const ConnectorConfig &cfg);
     * Ref: hftbase/Connector/src/connector.cpp:482-505
     * <p>
     * [C++差异-语言适配] C++ 接收 ConnectorConfig& 参数，Java 版本直接使用内部 config 字段，
     * 因此省略参数。语义等价。
     * <p>
     * 仅对 MICEX/FORTS 交易所有效。中国期货场景始终返回 false。
     *
     * @return false (非俄罗斯交易所)
     */
    public boolean ifRunningForRussianExchange() {
        // C++ 实现: 遍历 INTERESTED_EXCHANGES 检查是否包含 MICEX/FORTS
        return false;
    }

    /**
     * 处理实盘行情更新。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:259
     * C++: void HandleLiveMdUpdates(MarketUpdateNew *update);
     * <p>
     * C++ header 中声明但无实现（死代码）。保留方法签名以对齐 C++。
     */
    public void handleLiveMdUpdates() {
        // C++ 中此方法在 header 中声明但无实现（死代码）
    }

    // =======================================================================
    //  回调实现
    // =======================================================================

    /**
     * symbol 过滤 + symbolID 重写 + 行情分发。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:749-782 — Connector::HandleUpdates()
     * C++:
     * <pre>
     *   void Connector::HandleUpdates(MarketUpdateNew *update) {
     *       if ((update->m_endPkt == 1) || m_interestedsymbols_for_md.size() == 0) {
     *           m_mdcb(update);
     *       } else {
     *           auto iter = m_interestedsymbols_for_md.find(update->m_symbol);
     *           if (iter != NULL) {
     *               update->m_symbolID = iter->val;
     *               m_mdcb(update);
     *           }
     *       }
     *   }
     * </pre>
     */
    // package-private: 测试代码从同包访问
    void handleUpdates(MemorySegment update) {
        // C++: if ((update->m_endPkt == 1) || m_interestedsymbols_for_md.size() == 0) {
        // Ref: hftbase/Connector/src/connector.cpp:768

        // 读取 endPkt 标志: MarketUpdateNew offset = MD_HEADER(96) + MDD_END_PKT_OFFSET(711) = 807
        byte endPkt = update.get(ValueLayout.JAVA_BYTE,
                Types.MD_HEADER_LAYOUT.byteSize() + Types.MDD_END_PKT_OFFSET);

        if (endPkt == 1 || interestedSymbolsForMd.isEmpty()) {
            mdCallback.onMarketData(update);
            return;
        }

        // C++: auto iter = m_interestedsymbols_for_md.find(update->m_symbol);
        String symbol = Instrument.readSymbol(update);
        Short localID = interestedSymbolsForMd.get(symbol);
        if (localID != null) {
            // C++: update->m_symbolID = iter->val;
            // Ref: connector.cpp:777
            Types.MDH_SYMBOL_ID_VH.set(update, 0L, localID.shortValue());
            // C++: m_mdcb(update);
            mdCallback.onMarketData(update);
        }
        // else: 未注册合约，静默丢弃 (与 C++ 一致)
    }

    /**
     * 回报过滤 + 分发。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:789-857 — Connector::HandleOrderResponse()
     * <p>
     * C++ LIVE 模式:
     * <pre>
     *   unsigned char exchId = m_response_queue_to_exchange_map[queueNum];
     *   int32_t clientId = msg->OrderID / ORDERID_RANGE;
     *
     *   if (m_responseFilterType == STRATEGY_FILTER) {
     *       for (int i = 0; i &lt; MAX_ORS_CLIENTS; i++) {
     *           if (m_all_clientIds[exchId][i] == clientId) { m_orscb(msg); break; }
     *           else if (m_all_clientIds[exchId][i] == DEFAULT_NOT_POSSIBLE_CLIENTID) { break; }
     *       }
     *   } else if (m_responseFilterType == TICKERS_ON_ONE_ACCOUNT_FILTER) {
     *       if (strcmp(m_cfg->INTERESTED_ACCOUNT, msg->AccountID) == 0) {
     *           if (m_interestedsymbols_for_ors.size() == 0
     *               || m_interestedsymbols_for_ors.find(msg->Symbol) != end()) {
     *               m_orscb(msg);
     *           }
     *       }
     *   }
     * </pre>
     * Ref: hftbase/Connector/src/connector.cpp:819-856
     */
    // package-private: 测试代码从同包访问
    void handleOrderResponse(MemorySegment data, int queueIndex) {
        if (responseFilterType == STRATEGY_FILTER) {
            // C++: unsigned char exchId = m_response_queue_to_exchange_map[queueNum];
            // Ref: hftbase/Connector/src/connector.cpp:821
            int exchId = responseQueueToExchangeMap[queueIndex];

            // C++: int32_t clientId = msg->OrderID / ORDERID_RANGE;
            // Ref: hftbase/Connector/src/connector.cpp:822
            int orderID = (int) Types.RESP_ORDER_ID_VH.get(data, 0L);
            int respClientId = orderID / Constants.ORDERID_RANGE;

            // C++: for (int i = 0; i < MAX_ORS_CLIENTS; i++) {
            //          if (m_all_clientIds[exchId][i] == clientId) { m_orscb(msg); break; }
            //          else if (m_all_clientIds[exchId][i] == DEFAULT_NOT_POSSIBLE_CLIENTID) { break; }
            //      }
            // Ref: hftbase/Connector/src/connector.cpp:826-838
            Set<Integer> exchClientIds = allClientIdsByExchange.get(exchId);
            if (exchClientIds != null && exchClientIds.contains(respClientId)) {
                orsCallback.onOrderResponse(data);
            }
        } else if (responseFilterType == TICKERS_ON_ONE_ACCOUNT_FILTER) {
            // C++: if (strcmp(m_cfg->INTERESTED_ACCOUNT.c_str(), msg->AccountID) == 0)
            // Ref: hftbase/Connector/src/connector.cpp:846
            String accountId = readAccountIdFromResponse(data);
            if (config.interestedAccount.equals(accountId)) {
                // C++: if (m_interestedsymbols_for_ors.size() == 0 || find(msg->Symbol) != end())
                // Ref: hftbase/Connector/src/connector.cpp:849-850
                if (interestedSymbolsForOrs.isEmpty()) {
                    orsCallback.onOrderResponse(data);
                } else {
                    String symbol = readSymbolFromResponse(data);
                    if (interestedSymbolsForOrs.contains(symbol)) {
                        orsCallback.onOrderResponse(data);
                    }
                }
            }
        }
    }


    // =======================================================================
    //  生命周期
    // =======================================================================

    /**
     * 停止轮询 + 分离所有 SHM 段（不删除）。
     * <p>
     * [C++差异-Java 资源管理] C++ 仅有 Stop() 和隐式析构函数 (~Connector)。
     * Java 无析构函数，close() 提供显式资源释放，语义等价于 C++ 析构函数中的 SHM 分离。
     */
    public void close() {
        shmMgr.shutdown();
    }

    // =======================================================================
    //  Getter
    // =======================================================================

    /**
     * 获取指定交易所的 clientId。
     * <p>
     * [C++差异-Java getter 惯例] C++ 直接访问 m_clientId[exchCode] 数组，
     * Java 通过 getter 方法暴露。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:380
     * C++: m_clientId[exchCode]
     */
    public int getClientId(int exchCode) {
        return clientIdMap.getOrDefault(exchCode, 0);
    }

    /**
     * 获取第一个交易所的 clientId（便利方法，单交易所场景常用）。
     * <p>
     * [C++差异-Java getter 惯例] C++ 直接访问 m_clientId 数组。
     * C++: m_clientId[first_exchId]
     */
    public int getClientId() {
        if (clientIdMap.isEmpty()) return 0;
        return clientIdMap.values().iterator().next();
    }

    /**
     * 获取 ShmMgr 引用。
     * <p>
     * [C++差异-Java getter 惯例] C++ 直接访问 m_shmMgr 成员，Java 通过 getter 暴露。
     * C++: m_shmMgr (connector.h:401)
     */
    public MultiClientStoreShmReader getShmMgr() {
        return shmMgr;
    }

    /**
     * 返回 interestedSymbolsForMd 映射。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:470-473
     * C++: std::map<std::string, uint16_t> Connector::getSymbolIDMap()
     */
    public Map<String, Short> getSymbolIDMap() {
        return Collections.unmodifiableMap(interestedSymbolsForMd);
    }

    /**
     * 获取指定交易所的请求队列。
     * <p>
     * [C++差异-Java getter 惯例] C++ 直接访问 m_requestQueue[exchId] 数组。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:390
     * C++: m_requestQueue[exchId]
     */
    public MWMRQueue getRequestQueue(int exchType) {
        return requestQueues.get(exchType);
    }

    /**
     * 设置为所有合约运行（无 symbol 过滤）。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:477-480
     * C++: void Connector::setToRunForAllSymbols() { m_runForAllSymbols = true; }
     */
    public void setToRunForAllSymbols() {
        this.runForAllSymbols = true;
    }

    // =======================================================================
    //  交易所特定方法
    // =======================================================================

    /**
     * 填充交易所特定的请求信息。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:1240-1274 — Connector::FillExchangeSpecificReqInfo()
     * C++:
     * <pre>
     *   void Connector::FillExchangeSpecificReqInfo(RequestMsg &amp;req) {
     *       switch (req.Exchange_Type) {
     *           case MICEX_FOND:
     *           case MICEX_CURR:
     *               if (req.Price == 0) req.OrdType = MARKET; else req.OrdType = LIMIT;
     *               req.PxType = PERUNIT;
     *               req.Duration = DAY;
     *               break;
     *           default:
     *               throw "bad exchange type in order request";
     *       }
     *   }
     * </pre>
     *
     * @param req RequestMsg MemorySegment
     */
    public void fillExchangeSpecificReqInfo(MemorySegment req) {
        int exchType = ((byte) Types.REQ_EXCHANGE_TYPE_VH.get(req, 0L)) & 0xFF;

        switch (exchType) {
            // C++: case MICEX_FOND / MICEX_CURR
            case Constants.MD_MICEX_FOND:
            case Constants.MD_MICEX_CURR: {
                double price = (double) Types.REQ_PRICE_VH.get(req, 0L);
                // C++: if (req.Price == 0) req.OrdType = MARKET; else req.OrdType = LIMIT;
                Types.REQ_ORD_TYPE_VH.set(req, 0L,
                        (price == 0.0) ? Constants.ORD_MARKET : Constants.ORD_LIMIT);
                // C++: req.PxType = PERUNIT;
                Types.REQ_PX_TYPE_VH.set(req, 0L, Constants.PX_PERUNIT);
                // C++: req.Duration = DAY;
                Types.REQ_DURATION_VH.set(req, 0L, Constants.DUR_DAY);
                break;
            }
            default:
                log.severe("[FillExchangeSpecificReqInfo] bad exchange type: " + exchType);
                throw new IllegalArgumentException("bad exchange type in order request: " + exchType);
        }
    }

    // =======================================================================
    //  内部方法
    // =======================================================================

    /**
     * 生成唯一的 OrderID（指定 exchCode）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:362-372
     * C++:
     * <pre>
     *   uint32_t GetUniqueOrderNumber(unsigned char exchCode) {
     *       if (m_OrderCount &lt; ORDERID_RANGE)
     *           return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
     *       else
     *           return GetOrderNumberWithNewClientId(exchCode);
     *   }
     * </pre>
     */
    private int getUniqueOrderNumber(int exchCode) {
        int seq = orderCount.getAndIncrement();
        if (seq >= Constants.ORDERID_RANGE) {
            return getOrderNumberWithNewClientId(exchCode);
        }
        int cid = clientIdMap.getOrDefault(exchCode, 0);
        return cid * Constants.ORDERID_RANGE + seq;
    }

    /**
     * OrderCount 溢出时: 重置计数器，为所有交易所注册新请求队列获取新 clientId。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:1152-1182 — Connector::GetOrderNumberWithNewClientId()
     * C++ (LIVE 模式):
     * <pre>
     *   m_OrderCount = 0;
     *   for (each exchange) {
     *       char exchId = getExchangeIdFromName(*it);
     *       m_requestQueue[exchId] = m_shmMgr.registerRequestClient(shmreqkey, size, m_clientId[exchId], clientStoreKey);
     *       m_all_clientIds[exchId][i] = m_clientId[exchId];
     *   }
     *   return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
     * </pre>
     */
    private int getOrderNumberWithNewClientId(int exchCode) {
        log.info("[Connector] GetOrderNumberWithNewClientId: orderCount 已达 ORDERID_RANGE="
                + Constants.ORDERID_RANGE + "，为所有交易所申请新 clientId");

        // C++: m_OrderCount = 0;
        // Ref: hftbase/Connector/src/connector.cpp:1160
        orderCount.set(0);

        // C++: 遍历所有 INTERESTED_EXCHANGES
        // Ref: hftbase/Connector/src/connector.cpp:1161-1182
        for (ExchangeConfig exchCfg : config.exchanges) {
            int exchId = Constants.getExchangeIdFromName(exchCfg.exchangeName);

            int newClientId = shmMgr.registerRequestClient(
                    exchCfg.reqShmKey, exchCfg.reqQueueSize, exchCfg.clientStoreShmKey);

            // C++: m_clientId[exchId] 被 registerRequestClient 通过引用更新
            clientIdMap.put(exchId, newClientId);

            // C++: m_all_clientIds[exchId][i] = m_clientId[exchId];
            allClientIdsByExchange.computeIfAbsent(exchId, k -> new HashSet<>()).add(newClientId);

            // 更新 requestQueues 引用
            int reqIdx = shmMgr.getReqClientCount() - 1;
            requestQueues.put(exchId, shmMgr.getReqQueue(reqIdx));

            log.info("[Connector] GetOrderNumberWithNewClientId: exchId=" + exchId
                    + " newClientId=" + newClientId);
        }

        // C++: return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
        // Ref: hftbase/Connector/src/connector.cpp:1215
        int seq = orderCount.getAndIncrement();
        int cid = clientIdMap.getOrDefault(exchCode, 0);
        return cid * Constants.ORDERID_RANGE + seq;
    }

    // =======================================================================
    //  Response 字段读取辅助
    // =======================================================================

    /**
     * 从 ResponseMsg 读取 AccountID 字符串。
     * <p>
     * [C++差异-语言适配] C++ 直接访问 msg->AccountID (char[]),
     * Java 需要封装为 String 提取方法。
     * <p>
     * C++: msg->AccountID — char[MAX_ACCNTID_LEN+1] at offset 91
     * Ref: hftbase/CommonUtils/include/orderresponse.h:149
     */
    private static String readAccountIdFromResponse(MemorySegment resp) {
        byte[] buf = new byte[Constants.ACCOUNT_ID_SIZE];
        MemorySegment.copy(resp, ValueLayout.JAVA_BYTE, Types.RESP_ACCOUNT_ID_OFFSET,
                buf, 0, buf.length);
        int len = 0;
        while (len < buf.length && buf[len] != 0) len++;
        return new String(buf, 0, len, StandardCharsets.US_ASCII);
    }

    /**
     * 从 ResponseMsg 读取 Symbol 字符串。
     * <p>
     * [C++差异-语言适配] C++ 直接访问 msg->Symbol (char[]),
     * Java 需要封装为 String 提取方法。
     * <p>
     * C++: msg->Symbol — char[MAX_SYMBOL_SIZE] at offset 41
     * Ref: hftbase/CommonUtils/include/orderresponse.h:453
     */
    private static String readSymbolFromResponse(MemorySegment resp) {
        byte[] buf = new byte[Constants.MAX_SYMBOL_SIZE];
        MemorySegment.copy(resp, ValueLayout.JAVA_BYTE, Types.RESP_SYMBOL_OFFSET,
                buf, 0, buf.length);
        int len = 0;
        while (len < buf.length && buf[len] != 0) len++;
        return new String(buf, 0, len, StandardCharsets.US_ASCII);
    }

}
