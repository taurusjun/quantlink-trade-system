package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * SysV MWMR SHM Connector -- 行情接收、订单发送、回报轮询。
 * <p>
 * 迁移自: hftbase/Connector/include/connector.h  (illuminati::Connector)
 * 迁移自: hftbase/Connector/src/connector.cpp
 * <p>
 * C++ 类核心职责:
 * <ul>
 *   <li>从 mdQueue 读取 MarketUpdateNew 并回调给策略 (HandleUpdates)</li>
 *   <li>将 RequestMsg 写入 reqQueue (SendNewOrder/SendModifyOrder/SendCancelOrder)</li>
 *   <li>从 respQueue 读取 ResponseMsg，按 clientId 过滤后回调 (HandleOrderResponse)</li>
 *   <li>通过 ClientStore 获取唯一 clientId，用于 OrderID 编码</li>
 * </ul>
 * <p>
 * [C++差异] C++ Connector 支持多种 InteractionMode (LIVE/SIMULATION/PAPERTRADING/PARALLELSIM)，
 *           Java 版本仅实现 LIVE 模式（SHM 直连），回测通过独立的 BacktestConnector 实现。
 * <p>
 * [C++差异] C++ 使用 exchCode 索引的 m_clientId[MAX_EXCHANGE_COUNT] 数组，
 *           Java 使用 Map&lt;Integer, Integer&gt; clientIdMap 按 exchCode 索引 clientId。
 *           默认 exchCode=0 用于中国期货。多交易所场景需为每个 exchCode 分配独立 clientId。
 * <p>
 * [C++差异] C++ m_OrderCount 是 uint32_t 非原子变量（单线程使用），
 *           Java 使用 AtomicInteger 以支持潜在的多线程发单场景。
 */
public class Connector {

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
    //  配置
    // =======================================================================

    /**
     * Connector 配置。
     * <p>
     * 迁移自: hftbase/Connector/include/connectorconfig.h  (ConnectorConfig)
     * <p>
     * [C++差异] C++ ConnectorConfig 包含大量字段（SimMode, ExchangeType, 多交易所配置等），
     *           Java 版本精简为 MWMR SHM key 和队列大小配置。
     */
    public static class Config {
        /** 行情 SHM key, 例如 0x1001 */
        public int mdShmKey;
        /** 行情队列容量 */
        public int mdQueueSize;
        /** 订单请求 SHM key, 例如 0x2001 */
        public int reqShmKey;
        /** 订单请求队列容量 */
        public int reqQueueSize;
        /** 订单回报 SHM key, 例如 0x3001 */
        public int respShmKey;
        /** 订单回报队列容量 */
        public int respQueueSize;
        /** ClientStore SHM key, 例如 0x4001 */
        public int clientStoreShmKey;
    }

    // =======================================================================
    //  实例字段
    // =======================================================================

    // C++: ShmMgr 内部的 mdQueue, requestQueue, responseQueue
    // Ref: hftbase/Connector/include/connector.h:390-394

    /** 行情队列 (读) -- C++: MultiWriterMultiReaderShmQueue<MarketUpdateNew> */
    private final MWMRQueue mdQueue;

    /** 订单请求队列 (写) -- C++: m_requestQueue[exchType] */
    private final MWMRQueue reqQueue;

    /** 订单回报队列 (读) -- C++: responseQueue in ShmMgr */
    private final MWMRQueue respQueue;

    /**
     * 客户端 ID 存储。
     * 迁移自: hftbase/Connector/include/connector.h:401 (m_shmMgr 内部)
     * C++: LocklessShmClientStore 通过 MultiClientStoreShmReader 访问
     */
    private final ClientStore clientStore;

    /**
     * 本客户端的唯一 ID，从 ClientStore 原子递增获取。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:380
     * C++: uint32_t m_clientId[illuminati::md::MAX_EXCHANGE_COUNT];
     * <p>
     * Java 使用 clientIdMap 按 exchCode 索引，defaultClientId 为默认值（exchCode=0）。
     */
    private final int defaultClientId;

    /**
     * 按 exchCode 索引的 clientId 映射。
     * 迁移自: C++ m_clientId[MAX_EXCHANGE_COUNT] 数组
     * Ref: hftbase/Connector/include/connector.h:380
     */
    private final Map<Integer, Integer> clientIdMap = new HashMap<>();

    /**
     * 订单计数器，用于生成唯一 OrderID。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:383
     * C++: uint32_t m_OrderCount;
     * <p>
     * [C++差异] C++ 使用 uint32_t (非原子)，Java 使用 AtomicInteger。
     */
    private final AtomicInteger orderCount = new AtomicInteger(0);

    /** 行情回调 -- C++: MDConnection m_mdcb (connector.h:376) */
    private final MDCallback mdCallback;

    /** 订单回报回调 -- C++: ORSConnection m_orscb (connector.h:377) */
    private final ORSCallback orsCallback;

    /**
     * 运行状态标志。
     * <p>
     * 迁移自: Connector::Stop() 中通过 signal 控制轮询线程退出
     * Ref: hftbase/Connector/src/connector.cpp:Stop()
     */
    private volatile boolean running;

    /** 行情轮询线程 -- C++: 在 StartAsync 中创建的 MD 线程 */
    private Thread pollMDThread;

    /** 回报轮询线程 -- C++: 在 StartAsync 中创建的 ORS 线程 */
    private Thread pollORSThread;

    // =======================================================================
    //  构造（私有）
    // =======================================================================

    /**
     * 私有构造函数。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:73
     * C++: Connector(MDConnection, ORSConnection, InteractionMode, ConnectorConfig *);
     */
    private Connector(MWMRQueue mdQueue, MWMRQueue reqQueue, MWMRQueue respQueue,
                      ClientStore clientStore, int clientId,
                      MDCallback mdCallback, ORSCallback orsCallback) {
        this.mdQueue = mdQueue;
        this.reqQueue = reqQueue;
        this.respQueue = respQueue;
        this.clientStore = clientStore;
        this.defaultClientId = clientId;
        // C++: m_clientId[exchCode] — 默认交易所 exchCode=0
        this.clientIdMap.put(0, clientId);
        this.mdCallback = mdCallback;
        this.orsCallback = orsCallback;
        this.running = false;
    }

    // =======================================================================
    //  工厂方法
    // =======================================================================

    /**
     * 连接模式 -- 连接到已有的 SHM 段（生产用）。
     * <p>
     * 迁移自: Connector 构造函数 + MultiClientStoreShmReader::init()
     * Ref: hftbase/Connector/src/connector.cpp:Connector()
     * <p>
     * 流程:
     * <ol>
     *   <li>MWMRQueue.open() 连接行情/请求/回报三个队列</li>
     *   <li>ClientStore.open() 连接客户端 ID 存储</li>
     *   <li>ClientStore.getClientIdAndIncrement() 原子获取唯一 clientId</li>
     * </ol>
     *
     * @param cfg  SHM 配置
     * @param mdCb 行情回调
     * @param orsCb 回报回调
     * @return 已连接的 Connector
     */
    public static Connector open(Config cfg, MDCallback mdCb, ORSCallback orsCb) {
        // C++: ShmMgr::init() -- 连接三个 MWMR 队列
        // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h

        // 行情队列: dataSize=816 (MarketUpdateNew), elemSize=824
        MWMRQueue mdQueue = MWMRQueue.open(cfg.mdShmKey, cfg.mdQueueSize,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);

        // 请求队列: dataSize=256 (RequestMsg), elemSize=320
        MWMRQueue reqQueue = MWMRQueue.open(cfg.reqShmKey, cfg.reqQueueSize,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);

        // 回报队列: dataSize=176 (ResponseMsg), elemSize=184
        MWMRQueue respQueue = MWMRQueue.open(cfg.respShmKey, cfg.respQueueSize,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // ClientStore: 原子递增获取 clientId
        // C++: m_clientId[exchCode] = m_shmMgr.getClientIdAndIncrement()
        // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h
        ClientStore clientStore = ClientStore.open(cfg.clientStoreShmKey);
        int clientId = (int) clientStore.getClientIdAndIncrement();

        return new Connector(mdQueue, reqQueue, respQueue, clientStore, clientId, mdCb, orsCb);
    }

    /**
     * 创建模式 -- 创建新的 SHM 段（测试用）。
     * <p>
     * 与 open() 相同逻辑，但使用 create() 创建新的 SHM 段。
     * 测试完毕后需调用 {@link #destroy()} 清理。
     *
     * @param cfg   SHM 配置
     * @param mdCb  行情回调
     * @param orsCb 回报回调
     * @return 新创建的 Connector
     */
    public static Connector createForTest(Config cfg, MDCallback mdCb, ORSCallback orsCb) {
        // 行情队列
        MWMRQueue mdQueue = MWMRQueue.create(cfg.mdShmKey, cfg.mdQueueSize,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);

        // 请求队列
        MWMRQueue reqQueue = MWMRQueue.create(cfg.reqShmKey, cfg.reqQueueSize,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);

        // 回报队列
        MWMRQueue respQueue = MWMRQueue.create(cfg.respShmKey, cfg.respQueueSize,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // ClientStore: 创建并初始化，初始 clientId=1
        // C++: LocklessShmClientStore::init(shmkey, 1, ...)
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:47-55
        ClientStore clientStore = ClientStore.create(cfg.clientStoreShmKey, 1L);
        int clientId = (int) clientStore.getClientIdAndIncrement();

        return new Connector(mdQueue, reqQueue, respQueue, clientStore, clientId, mdCb, orsCb);
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
     *
     * @param req RequestMsg MemorySegment (至少 256 bytes)
     * @return 分配的 OrderID
     */
    public int sendNewOrder(MemorySegment req) {
        // C++: stratReq.OrderID = GetUniqueOrderNumber(stratReq.Exchange_Type);
        // Ref: hftbase/Connector/include/connector.h:273
        int orderId = getUniqueOrderNumber();

        // C++: stratReq.Request_Type = illuminati::infra::NEWORDER;
        // Ref: hftbase/Connector/include/connector.h:274
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_NEWORDER);

        // C++: stratReq.OrderID = orderid;
        // Ref: hftbase/Connector/include/connector.h:273
        Types.REQ_ORDER_ID_VH.set(req, 0L, orderId);

        // C++: msg.TimeStamp = illuminati::ITime_ClockRT::GetCurrentTime();
        // Ref: hftbase/Connector/include/connector.h:202 (PushRequest)
        Types.REQ_TIMESTAMP_VH.set(req, 0L, System.nanoTime());

        // C++: m_requestQueue[msg.Exchange_Type]->enqueue(msg);
        // Ref: hftbase/Connector/include/connector.h:218
        reqQueue.enqueue(req);

        return orderId;
    }

    /**
     * 发送撤单请求。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:318-322
     * C++:
     * <pre>
     *   void SendCancelOrder(illuminati::infra::RequestMsg &amp;stratReq) {
     *       stratReq.Request_Type = illuminati::infra::CANCELORDER;
     *       PushRequest(stratReq);
     *   }
     * </pre>
     *
     * @param req RequestMsg MemorySegment，必须已设置 OrderID 为要撤销的订单 ID
     */
    public void sendCancelOrder(MemorySegment req) {
        // C++: stratReq.Request_Type = illuminati::infra::CANCELORDER;
        // Ref: hftbase/Connector/include/connector.h:320
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_CANCELORDER);

        // C++: msg.TimeStamp = illuminati::ITime_ClockRT::GetCurrentTime();
        // Ref: hftbase/Connector/include/connector.h:202 (PushRequest)
        Types.REQ_TIMESTAMP_VH.set(req, 0L, System.nanoTime());

        // C++: m_requestQueue[msg.Exchange_Type]->enqueue(msg);
        // Ref: hftbase/Connector/include/connector.h:218
        reqQueue.enqueue(req);
    }

    /**
     * 发送改单请求。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:299-303
     * C++:
     * <pre>
     *   void SendModifyOrder(illuminati::infra::RequestMsg &amp;stratReq) {
     *       stratReq.Request_Type = infra::MODIFYORDER;
     *       PushRequest(stratReq);
     *   }
     * </pre>
     *
     * @param req RequestMsg MemorySegment，必须已设置 OrderID 和新的 Price/Quantity
     */
    public void sendModifyOrder(MemorySegment req) {
        // C++: stratReq.Request_Type = infra::MODIFYORDER;
        // Ref: hftbase/Connector/include/connector.h:301
        Types.REQ_REQUEST_TYPE_VH.set(req, 0L, Constants.REQUEST_MODIFYORDER);

        // C++: msg.TimeStamp = illuminati::ITime_ClockRT::GetCurrentTime();
        // Ref: hftbase/Connector/include/connector.h:202 (PushRequest)
        Types.REQ_TIMESTAMP_VH.set(req, 0L, System.nanoTime());

        // C++: m_requestQueue[msg.Exchange_Type]->enqueue(msg);
        // Ref: hftbase/Connector/include/connector.h:218
        reqQueue.enqueue(req);
    }

    // =======================================================================
    //  轮询控制
    // =======================================================================

    /**
     * 启动行情和回报轮询线程。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:94
     * C++: void StartAsync(SimEngineType = TBTEngine);
     * <p>
     * C++ 实现中 StartAsync 创建两个 pthread:
     *   1. MD 轮询线程: 持续从 mdQueue 读取 MarketUpdateNew
     *   2. ORS 轮询线程: 持续从 respQueue 读取 ResponseMsg
     * Ref: hftbase/Connector/src/connector.cpp:StartAsync()
     */
    public void startAsync() {
        running = true;

        pollMDThread = new Thread(this::handleLiveMdUpdates, "connector-md-poll");
        pollMDThread.setDaemon(true);
        pollMDThread.start();

        pollORSThread = new Thread(this::handleOrderResponse, "connector-ors-poll");
        pollORSThread.setDaemon(true);
        pollORSThread.start();
    }

    /**
     * 停止轮询线程。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:98
     * C++: void Stop();
     * Ref: hftbase/Connector/src/connector.cpp:Stop()
     */
    public void stop() {
        running = false;
        if (pollMDThread != null) {
            try {
                pollMDThread.join(5000);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }
        if (pollORSThread != null) {
            try {
                pollORSThread.join(5000);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }
    }

    // =======================================================================
    //  轮询实现
    // =======================================================================

    /**
     * 行情轮询循环。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:HandleLiveMdUpdates()
     * C++ 逻辑: 无限循环从 mdQueue 出队 MarketUpdateNew，调用 HandleUpdates(update)
     * Ref: hftbase/Connector/include/connector.h:116
     */
    private void handleLiveMdUpdates() {
        // C++: MarketUpdateNew update; (栈上分配)
        // Java: 使用全局 Arena 分配（避免 GC 开销）
        MemorySegment buf = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        while (running) {
            if (mdQueue.dequeue(buf)) {
                // C++: m_mdcb(update);
                // Ref: hftbase/Connector/include/connector.h:156
                mdCallback.onMarketData(buf);
            } else {
                // C++: 忙等待（无显式 yield/sleep）
                Thread.onSpinWait();
            }
        }
    }

    /**
     * 回报轮询循环 + clientId 过滤。
     * <p>
     * 迁移自: hftbase/Connector/src/connector.cpp:HandleOrderResponse() (L820-857)
     * C++ 逻辑:
     * <pre>
     *   int32_t clientId = msg-&gt;OrderID / ORDERID_RANGE;
     *   if (m_responseFilterType == STRATEGY_FILTER) {
     *       for (int i = 0; i &lt; MAX_ORS_CLIENTS; i++) {
     *           if (m_all_clientIds[exchId][i] == clientId) {
     *               m_orscb(msg);
     *               break;
     *           }
     *       }
     *   }
     * </pre>
     * <p>
     * [C++差异] C++ 支持多种过滤模式 (STRATEGY_FILTER, TICKERS_ON_ONE_ACCOUNT_FILTER)，
     *           Java 使用 clientIdMap 中所有 clientId 匹配（STRATEGY_FILTER 模式）。
     */
    private void handleOrderResponse() {
        // C++: ResponseMsg msg; (栈上分配)
        MemorySegment buf = Arena.global().allocate(Types.RESPONSE_MSG_LAYOUT);
        while (running) {
            if (respQueue.dequeue(buf)) {
                // C++: int32_t clientId = msg->OrderID / ORDERID_RANGE;
                // Ref: hftbase/Connector/src/connector.cpp:822
                int orderID = (int) Types.RESP_ORDER_ID_VH.get(buf, 0L);
                int respClientId = orderID / Constants.ORDERID_RANGE;

                // C++: for (int i = 0; i < MAX_ORS_CLIENTS; i++) {
                //          if (m_all_clientIds[exchId][i] == clientId) { m_orscb(msg); break; }
                //      }
                // Ref: hftbase/Connector/src/connector.cpp:826-832
                if (clientIdMap.containsValue(respClientId)) {
                    // C++: m_orscb(msg);
                    orsCallback.onOrderResponse(buf);
                }
            } else {
                Thread.onSpinWait();
            }
        }
    }

    // =======================================================================
    //  测试辅助方法
    // =======================================================================

    /**
     * 向行情队列写入数据（测试用: 模拟 md_shm_feeder 写入行情）。
     *
     * @param md MarketUpdateNew MemorySegment
     */
    public void enqueueMD(MemorySegment md) {
        mdQueue.enqueue(md);
    }

    /**
     * 向回报队列写入数据（测试用: 模拟 counter_bridge 写入回报）。
     *
     * @param resp ResponseMsg MemorySegment
     */
    public void enqueueResponse(MemorySegment resp) {
        respQueue.enqueue(resp);
    }

    // =======================================================================
    //  生命周期
    // =======================================================================

    /**
     * 分离所有 SHM 段（不删除）。
     * <p>
     * 迁移自: Connector 析构函数中的 SHM detach
     * Ref: hftbase/Ipc/include/sharedmemory.h:35
     */
    public void close() {
        stop();
        mdQueue.close();
        reqQueue.close();
        respQueue.close();
        clientStore.close();
    }

    /**
     * 分离并删除所有 SHM 段（测试用: 清理 createForTest 创建的 SHM）。
     * <p>
     * 迁移自: SharedMemory 析构函数中的 shmdt + shmctl(IPC_RMID)
     * Ref: hftbase/Ipc/include/sharedmemory.h:35-37
     */
    public void destroy() {
        stop();
        mdQueue.destroy();
        reqQueue.destroy();
        respQueue.destroy();
        clientStore.destroy();
    }

    // =======================================================================
    //  Getter
    // =======================================================================

    /**
     * 获取默认 clientId (exchCode=0)。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:380
     * C++: uint32_t m_clientId[illuminati::md::MAX_EXCHANGE_COUNT]
     *
     * @return 默认 clientId
     */
    public int getClientId() {
        return defaultClientId;
    }

    /**
     * 获取指定交易所的 clientId。
     * 迁移自: C++ m_clientId[exchCode]
     *
     * @param exchCode 交易所代码
     * @return clientId，不存在则返回默认值
     */
    public int getClientId(int exchCode) {
        return clientIdMap.getOrDefault(exchCode, defaultClientId);
    }

    /**
     * 为指定交易所注册新的 clientId。
     * 迁移自: C++ m_clientId[exchCode] = m_shmMgr.getClientIdAndIncrement();
     * Ref: hftbase/Connector/src/connector.cpp 中多交易所初始化
     *
     * @param exchCode 交易所代码
     * @return 新分配的 clientId
     */
    public int addClientId(int exchCode) {
        int newClientId = (int) clientStore.getClientIdAndIncrement();
        clientIdMap.put(exchCode, newClientId);
        return newClientId;
    }

    // =======================================================================
    //  内部方法
    // =======================================================================

    /**
     * 生成唯一的 OrderID（默认 exchCode=0）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:362-372
     *
     * @return clientId * ORDERID_RANGE + seq
     */
    private int getUniqueOrderNumber() {
        return getUniqueOrderNumber(0);
    }

    /**
     * 生成唯一的 OrderID（指定 exchCode）。
     * <p>
     * 迁移自: hftbase/Connector/include/connector.h:362-372
     * C++:
     * <pre>
     *   uint32_t GetUniqueOrderNumber(unsigned char exchCode) {
     *       if (illumiati_likely_branch(m_OrderCount &lt; ORDERID_RANGE)) {
     *           return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
     *       } else {
     *           return GetOrderNumberWithNewClientId(exchCode);
     *       }
     *   }
     * </pre>
     * <p>
     * [C++差异] C++ 在 m_OrderCount 溢出 ORDERID_RANGE 时请求新的 clientId，
     *           Java 版本暂不处理溢出（AtomicInteger.getAndIncrement 会持续递增，
     *           在 1M 以内足够日内使用）。
     *
     * @param exchCode 交易所代码 (0=默认)
     * @return clientId * ORDERID_RANGE + seq
     */
    private int getUniqueOrderNumber(int exchCode) {
        // C++: return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
        // Ref: hftbase/Connector/include/connector.h:366
        int cid = clientIdMap.getOrDefault(exchCode, defaultClientId);
        return cid * Constants.ORDERID_RANGE + orderCount.getAndIncrement();
    }
}
