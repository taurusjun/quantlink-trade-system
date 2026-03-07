package com.quantlink.trader.shm;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.util.HashMap;
import java.util.Map;
import java.util.logging.Logger;

/**
 * 多队列 SHM 管理器 -- 行情/请求/回报队列注册、轮询、线程管理。
 * <p>
 * 迁移自: hftbase/Ipc/include/multiclientstoreshmreader.h
 *         -- illuminati::ipc::MultiClientStoreShmReader&lt;MD, REQ, RESP, MAXSIZE&gt;
 * <p>
 * C++ 在 Connector 中实例化:
 * <pre>
 *   typedef MultiClientStoreShmReader&lt;MarketUpdateNew, RequestMsg, ResponseMsg, MAX_ORS_CLIENTS&gt; ShmMgr;
 *   ShmMgr m_shmMgr;
 * </pre>
 * Ref: hftbase/Connector/include/connector.h:39-43
 * <p>
 * [C++差异] C++ 是模板类 (MD/REQ/RESP/MAXSIZE)，Java 中 MD/REQ/RESP 都是 MemorySegment
 *           (Panama FFI)，MAXSIZE 作为构造参数传入。
 * <p>
 * [C++差异] C++ 使用 DEFINE_SIGNAL/EMIT_PARAM 宏（信号-槽模式）分发回调，
 *           Java 使用函数式接口（MDCallback/ORSRequestCallback/ORSResponseCallback）。
 */
public class MultiClientStoreShmReader {

    private static final Logger log = Logger.getLogger(MultiClientStoreShmReader.class.getName());

    // =======================================================================
    //  回调接口 -- 对齐 C++ DEFINE_SIGNAL 宏
    // =======================================================================

    /**
     * 行情回调。
     * 迁移自: DEFINE_SIGNAL(MarketUpdateAvailable, ...)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:251
     */
    @FunctionalInterface
    public interface MDCallback {
        void onMarketUpdate(MemorySegment data);
    }

    /**
     * 订单请求回调。
     * 迁移自: DEFINE_SIGNAL(..., ORSRequestAvailable, ...)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:251
     */
    @FunctionalInterface
    public interface ORSRequestCallback {
        void onRequest(MemorySegment data);
    }

    /**
     * 订单回报回调。
     * 迁移自: DEFINE_SIGNAL(..., ORSResponseAvailable, ...)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:251
     * <p>
     * C++: EMIT_PARAM(ORSResponseAvailable, &amp;data, i) — 第二个参数是队列索引
     */
    @FunctionalInterface
    public interface ORSResponseCallback {
        void onResponse(MemorySegment data, int queueIndex);
    }

    // =======================================================================
    //  MDEndPktClient 内部类 -- 对齐 C++ MDWithEndPacketClient
    // =======================================================================

    /**
     * 带 endPkt 状态的行情客户端。
     * <p>
     * 迁移自: hftbase/Ipc/include/mdclients.h — MDWithEndPacketClient&lt;MDQueue, MD&gt;
     * <p>
     * C++ 字段:
     * <pre>
     *   bool last_packet_was_endpacket;
     *   MD data;                         // 预取的数据缓存
     *   bool contains_new_data;          // 是否已预取数据
     * </pre>
     */
    public static class MDEndPktClient {
        // C++: MDClient::mdqueue_
        // Ref: hftbase/Ipc/include/mdclients.h:17
        final MWMRQueue queue;

        // C++: bool last_packet_was_endpacket (line 65)
        boolean lastPacketWasEndpacket = false;

        // C++: MD data (line 67) — 预取的数据缓存
        final MemorySegment data;

        // C++: bool contains_new_data (line 69)
        boolean containsNewData = false;

        /**
         * 迁移自: MDWithEndPacketClient(MDQueue *mdqueue)
         * Ref: hftbase/Ipc/include/mdclients.h:46-47
         */
        MDEndPktClient(MWMRQueue queue, long dataSize) {
            this.queue = queue;
            // 分配预取缓存 — 对应 C++ 的 MD data 成员变量（栈分配结构体）
            this.data = Arena.ofAuto().allocate(dataSize);
        }

        /**
         * 迁移自: MDClient::isEmpty()
         * Ref: hftbase/Ipc/include/mdclients.h:30-33
         */
        boolean isEmpty() {
            return queue.isEmpty();
        }

        /**
         * 迁移自: MDClient::dequeue()
         * Ref: hftbase/Ipc/include/mdclients.h:25-28
         */
        boolean dequeue(MemorySegment out) {
            return queue.dequeue(out);
        }

        /**
         * 尝试从队列预取数据。
         * <p>
         * 迁移自: MDWithEndPacketClient::fetch_data_if_possible_from_queue()
         * Ref: hftbase/Ipc/include/mdclients.h:51-62
         * <p>
         * C++:
         * <pre>
         *   if (!(this-&gt;mdqueue_)-&gt;isEmpty()) {
         *       data = this-&gt;dequeue();
         *       contains_new_data = true;
         *   } else {
         *       contains_new_data = false;
         *   }
         * </pre>
         */
        void fetchDataIfPossibleFromQueue() {
            // C++: if (!(this->mdqueue_)->isEmpty()) { data = this->dequeue(); contains_new_data = true; }
            if (!queue.isEmpty()) {
                if (queue.dequeue(data)) {
                    containsNewData = true;
                } else {
                    containsNewData = false;
                }
            } else {
                // C++: contains_new_data = false;
                containsNewData = false;
            }
        }

        /**
         * 获取预取数据中的 timestamp。
         * 对应 C++: m_mdWithEndPacketClients[i]-&gt;data.m_timestamp
         * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:139
         */
        long getTimestamp() {
            // C++: data.m_timestamp — offset 8 in MarketUpdateNew
            return (long) Types.MDH_TIMESTAMP_VH.get(data, 0L);
        }

        /**
         * 获取预取数据中的 endPkt 标志。
         * 对应 C++: data.m_endPkt
         * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:169
         */
        byte getEndPkt() {
            // C++: data.m_endPkt — offset 711 in MdData (从 data 部分起算)
            // MarketUpdateNew = MdHeader(96) + MdData(720) = 816
            // endPkt 在 MdData offset 711，即 MarketUpdateNew offset 96+711=807
            // 但 dequeue 输出的是完整 MarketUpdateNew (816 bytes)
            // MDD_END_PKT_OFFSET = 711 是相对 MdData 的偏移
            // 在完整 MarketUpdateNew 中 = MD_HEADER_SIZE(96) + 711 = 807
            return data.get(java.lang.foreign.ValueLayout.JAVA_BYTE,
                    Types.MD_HEADER_LAYOUT.byteSize() + Types.MDD_END_PKT_OFFSET);
        }
    }

    // =======================================================================
    //  实例字段 -- 对齐 C++ private 成员
    // =======================================================================

    /** 最大客户端数。C++: template param MAXSIZE */
    private final int maxSize;

    /** 行情数据大小 (bytes)。对应 C++ sizeof(MD) = sizeof(MarketUpdateNew) */
    private final long mdDataSize;

    /** 请求消息大小 (bytes)。对应 C++ sizeof(REQ) = sizeof(RequestMsg) */
    private final long reqDataSize;

    /** 回报消息大小 (bytes)。对应 C++ sizeof(RESP) = sizeof(ResponseMsg) */
    private final long respDataSize;

    /** 行情队列 elem 大小 (含 seqNo padding) */
    private final long mdElemSize;

    /** 请求队列 elem 大小 */
    private final long reqElemSize;

    /** 回报队列 elem 大小 */
    private final long respElemSize;

    // C++: MDClient<MdShmQ, MD> *m_mdClients[MAXSIZE ? MAXSIZE : 1];
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:65
    private final MWMRQueue[] mdQueues;

    // C++: MDWithEndPacketClient<MdShmQ, MD> *m_mdWithEndPacketClients[MAXSIZE ? MAXSIZE : 1];
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:66
    private final MDEndPktClient[] mdEndPktClients;

    // C++: ReqShmQ *m_reqClients[MAXSIZE ? MAXSIZE : 1];
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:67
    private final MWMRQueue[] reqQueues;

    // C++: RespShmQ *m_respClients[MAXSIZE ? MAXSIZE : 1];
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:68
    private final MWMRQueue[] respQueues;

    // C++: bool m_isRequestQInitialized;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:69
    private boolean isRequestQInitialized = false;

    // C++: size_t m_defaultClientStoreKey;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:71
    private int defaultClientStoreKey;

    // C++: uint32_t m_mdClientCount;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:73
    private int mdClientCount = 0;

    // C++: uint32_t m_mdWithEndPacketClientCount;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:74
    private int mdEndPktClientCount = 0;

    // C++: uint32_t m_reqClientCount;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:75
    private int reqClientCount = 0;

    // C++: uint32_t m_respClientCount;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:76
    private int respClientCount = 0;

    // C++: std::thread m_threadHandler;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:79
    private Thread threadHandler;

    // C++: std::thread m_mdThread;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:81
    private Thread mdThread;

    // C++: std::thread m_orsRequestThread;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:82
    private Thread orsRequestThread;

    // C++: std::thread m_orsResponseThread;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:83
    private Thread orsResponseThread;

    // C++: bool m_active;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:90
    private volatile boolean active = true;

    // C++: bool m_mdActive;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:91
    private volatile boolean mdActive = true;

    // C++: bool m_orsRequestActive;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:92
    private volatile boolean orsRequestActive = true;

    // C++: bool m_orsResponseActive;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:93
    private volatile boolean orsResponseActive = true;

    // C++: std::map<size_t, LocklessShmClientStore<uint64_t> *> clientStores;
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:95
    private final Map<Integer, ClientStore> clientStores = new HashMap<>();

    // ---- 预分配 dequeue 缓冲区 ----
    // [C++差异] C++ 在栈上分配 MD/REQ/RESP 临时变量，Java 预分配 MemorySegment 避免热循环中重复 allocate
    private final MemorySegment mdBuf;
    private final MemorySegment reqBuf;
    private final MemorySegment respBuf;

    // ---- 回调 ----
    // C++: DEFINE_SIGNAL(MarketUpdateAvailable, ORSRequestAvailable, ORSResponseAvailable, ...)
    // Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:251
    private MDCallback mdCallback;
    private ORSRequestCallback orsRequestCallback;
    private ORSResponseCallback orsResponseCallback;

    // =======================================================================
    //  构造函数
    // =======================================================================

    /**
     * 迁移自: MultiClientStoreShmReader()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:254-260
     * <p>
     * C++:
     * <pre>
     *   MultiClientStoreShmReader()
     *       : m_isRequestQInitialized(false), m_mdClientCount(0),
     *         m_mdWithEndPacketClientCount(0), m_reqClientCount(0),
     *         m_respClientCount(0), m_active(true), m_mdActive(true),
     *         m_orsRequestActive(true), m_orsResponseActive(true) {}
     * </pre>
     *
     * @param maxSize     最大客户端数 (C++: template MAXSIZE)
     * @param mdDataSize  行情消息体大小 (C++: sizeof(MD))
     * @param reqDataSize 请求消息体大小 (C++: sizeof(REQ))
     * @param respDataSize 回报消息体大小 (C++: sizeof(RESP))
     * @param mdElemSize  行情队列元素大小 (含 seqNo padding)
     * @param reqElemSize 请求队列元素大小
     * @param respElemSize 回报队列元素大小
     */
    public MultiClientStoreShmReader(int maxSize,
                                      long mdDataSize, long reqDataSize, long respDataSize,
                                      long mdElemSize, long reqElemSize, long respElemSize) {
        this.maxSize = maxSize;
        this.mdDataSize = mdDataSize;
        this.reqDataSize = reqDataSize;
        this.respDataSize = respDataSize;
        this.mdElemSize = mdElemSize;
        this.reqElemSize = reqElemSize;
        this.respElemSize = respElemSize;

        // C++: MDClient *m_mdClients[MAXSIZE ? MAXSIZE : 1];
        int arraySize = maxSize > 0 ? maxSize : 1;
        this.mdQueues = new MWMRQueue[arraySize];
        this.mdEndPktClients = new MDEndPktClient[arraySize];
        this.reqQueues = new MWMRQueue[arraySize];
        this.respQueues = new MWMRQueue[arraySize];

        // 预分配 dequeue 缓冲区 — 对应 C++ 栈上临时变量 (MD data / REQ data / RESP data)
        this.mdBuf = Arena.ofAuto().allocate(mdDataSize);
        this.reqBuf = Arena.ofAuto().allocate(reqDataSize);
        this.respBuf = Arena.ofAuto().allocate(respDataSize);
    }

    // =======================================================================
    //  回调注册 -- 对齐 C++ connectWithParam / DEFINE_SIGNAL
    // =======================================================================

    /**
     * 注册行情回调。
     * 对齐 C++ connectWithParam(&amp;m_shmMgr, this, &amp;Connector::HandleUpdates, ShmMgr::MarketUpdateAvailable, ...)
     * Ref: hftbase/Connector/src/connector.cpp:308-309
     */
    public void setMDCallback(MDCallback callback) {
        this.mdCallback = callback;
    }

    /**
     * 注册请求回调。
     * 对齐 C++ connectWithParam(..., ORSRequestAvailable, ...)
     */
    public void setORSRequestCallback(ORSRequestCallback callback) {
        this.orsRequestCallback = callback;
    }

    /**
     * 注册回报回调。
     * 对齐 C++ connectWithParam(..., ORSResponseAvailable, ...)
     * Ref: hftbase/Connector/src/connector.cpp:310-312
     */
    public void setORSResponseCallback(ORSResponseCallback callback) {
        this.orsResponseCallback = callback;
    }

    // =======================================================================
    //  initClientStore -- 对齐 C++ initClientStore()
    // =======================================================================

    /**
     * 初始化 ClientStore。
     * <p>
     * 迁移自: MultiClientStoreShmReader::initClientStore(size_t clientStoreKey)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:265-289
     * <p>
     * C++:
     * <pre>
     *   auto it = clientStores.find(clientStoreKey);
     *   if (it == clientStores.end()) {
     *       LocklessShmClientStore&lt;uint64_t&gt; *clientStore = new LocklessShmClientStore&lt;uint64_t&gt;();
     *       clientStore-&gt;init(clientStoreKey, 0666);
     *       clientStores.insert(...);
     *   } else {
     *       (it-&gt;second)-&gt;init(clientStoreKey, 0666);
     *   }
     *   m_defaultClientStoreKey = clientStoreKey;
     * </pre>
     *
     * @param clientStoreKey SysV SHM key (例如 0x4001)
     */
    public void initClientStore(int clientStoreKey) {
        // C++: auto it = clientStores.find(clientStoreKey);
        ClientStore existing = clientStores.get(clientStoreKey);
        if (existing == null) {
            // C++: clientStore->init(clientStoreKey, 0666);
            // C++: clientStores.insert(...)
            ClientStore cs = ClientStore.open(clientStoreKey);
            clientStores.put(clientStoreKey, cs);
        } else {
            // C++: (it->second)->init(clientStoreKey, 0666);  — reinit
            existing.close();
            clientStores.put(clientStoreKey, ClientStore.open(clientStoreKey));
        }
        // C++: m_defaultClientStoreKey = clientStoreKey;
        this.defaultClientStoreKey = clientStoreKey;
    }

    // =======================================================================
    //  registerMDClient -- 对齐 C++ registerMDClient()
    // =======================================================================

    /**
     * 注册行情队列 (ROUND_ROBIN 模式)。
     * <p>
     * 迁移自: MultiClientStoreShmReader::registerMDClient(size_t shmMdKey, uint64_t shmSize)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:294-304
     * <p>
     * C++:
     * <pre>
     *   if (m_mdClientCount == MAXSIZE - 1) throw ...;
     *   return m_mdClients[m_mdClientCount++] = new MDClient&lt;MdShmQ, MD&gt;(new MdShmQ(shmMdKey, shmSize, 0666));
     * </pre>
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列容量 (slot 数)
     * @return 注册的 MWMRQueue
     */
    public MWMRQueue registerMDClient(int shmKey, int queueSize) {
        // C++: if (m_mdClientCount == MAXSIZE - 1) throw ...;
        if (mdClientCount == maxSize - 1) {
            throw new RuntimeException("MD clients have exceeded the maximum limit, which is " + maxSize);
        }
        // C++: m_mdClients[m_mdClientCount++] = new MDClient(new MdShmQ(shmMdKey, shmSize, 0666));
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, mdDataSize, mdElemSize);
        mdQueues[mdClientCount++] = queue;
        return queue;
    }

    // =======================================================================
    //  registerMDWithEndPacketClient -- 对齐 C++ registerMDWithEndPacketClient()
    // =======================================================================

    /**
     * 注册行情队列 (UNTIL_ENDPACKET 模式)。
     * <p>
     * 迁移自: MultiClientStoreShmReader::registerMDWithEndPacketClient(size_t shmMdKey, uint64_t shmSize)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:309-319
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列容量
     * @return 注册的 MDEndPktClient
     */
    public MDEndPktClient registerMDWithEndPacketClient(int shmKey, int queueSize) {
        // C++: if (m_mdWithEndPacketClientCount == MAXSIZE - 1) throw ...;
        if (mdEndPktClientCount == maxSize - 1) {
            throw new RuntimeException("MD clients have exceeded the maximum limit, which is " + maxSize);
        }
        // C++: m_mdWithEndPacketClients[m_mdWithEndPacketClientCount++] = new MDWithEndPacketClient(new MdShmQ(...));
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, mdDataSize, mdElemSize);
        MDEndPktClient client = new MDEndPktClient(queue, mdDataSize);
        mdEndPktClients[mdEndPktClientCount++] = client;
        return client;
    }

    // =======================================================================
    //  registerResponseClient -- 对齐 C++ registerResponseClient()
    // =======================================================================

    /**
     * 注册回报队列。
     * <p>
     * 迁移自: MultiClientStoreShmReader::registerResponseClient(size_t shmResponseKey, uint64_t shmSize)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:324-333
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列容量
     * @return 注册的 MWMRQueue
     */
    public MWMRQueue registerResponseClient(int shmKey, int queueSize) {
        // C++: if (m_respClientCount == MAXSIZE - 1) throw ...;
        if (respClientCount == maxSize - 1) {
            throw new RuntimeException("Response clients have exceeded the maximum limit, which is " + maxSize);
        }
        // C++: m_respClients[m_respClientCount++] = new RespShmQ(shmResponseKey, shmSize, 0666);
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, respDataSize, respElemSize);
        respQueues[respClientCount++] = queue;
        return queue;
    }

    // =======================================================================
    //  registerRequestClient -- 对齐 C++ registerRequestClient()
    // =======================================================================

    /**
     * 注册请求队列并分配 clientId。
     * <p>
     * 迁移自: MultiClientStoreShmReader::registerRequestClient(size_t shmRequestKey, uint64_t shmSize,
     *         uint32_t &amp;clientId, size_t clientStoreKey)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:339-368
     * <p>
     * C++:
     * <pre>
     *   if (clientStoreKey == -1) clientStoreKey = m_defaultClientStoreKey;
     *   if (m_reqClientCount == MAXSIZE - 1) throw ...;
     *   int32_t totalShmQueues = clientStores[clientStoreKey]-&gt;getClientId() - clientStores[clientStoreKey]-&gt;getFirstClientIdValue();
     *   if (totalShmQueues == MAXSIZE - 1) throw ...;
     *   clientId = clientStores[clientStoreKey]-&gt;getClientIdAndIncrement();
     *   return m_reqClients[m_reqClientCount++] = new ReqShmQ(shmRequestKey, shmSize, 0666);
     * </pre>
     *
     * @param shmKey         SysV SHM key
     * @param queueSize      队列容量
     * @param clientStoreKey ClientStore SHM key (-1 使用默认)
     * @return 分配的 clientId
     */
    public int registerRequestClient(int shmKey, int queueSize, int clientStoreKey) {
        // C++: if (clientStoreKey == -1) clientStoreKey = m_defaultClientStoreKey;
        if (clientStoreKey == -1) {
            clientStoreKey = defaultClientStoreKey;
        }

        // C++: if (m_reqClientCount == MAXSIZE - 1) throw ...;
        if (reqClientCount == maxSize - 1) {
            throw new RuntimeException("Request clients have exceeded the maximum limit, which is " + maxSize);
        }

        // C++: int32_t totalShmQueues = clientStores[clientStoreKey]->getClientId()
        //          - clientStores[clientStoreKey]->getFirstClientIdValue();
        ClientStore cs = clientStores.get(clientStoreKey);
        if (cs == null) {
            throw new RuntimeException("ClientStore not initialized for key: 0x" + Integer.toHexString(clientStoreKey));
        }
        long totalShmQueues = cs.getClientId() - cs.getFirstClientIdValue();
        // C++: if (totalShmQueues == MAXSIZE - 1) throw ...;
        if (totalShmQueues == maxSize - 1) {
            throw new RuntimeException("Clients have exceeded client store limit, which is " + maxSize);
        }

        // C++: clientId = clientStores[clientStoreKey]->getClientIdAndIncrement();
        int clientId = (int) cs.getClientIdAndIncrement();

        // C++: m_reqClients[m_reqClientCount++] = new ReqShmQ(shmRequestKey, shmSize, 0666);
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, reqDataSize, reqElemSize);
        reqQueues[reqClientCount++] = queue;

        return clientId;
    }

    /**
     * 注册请求队列（使用默认 ClientStore）。
     * 对齐 C++ 默认参数: clientStoreKey = -1
     */
    public int registerRequestClient(int shmKey, int queueSize) {
        return registerRequestClient(shmKey, queueSize, -1);
    }

    // =======================================================================
    //  getRequestClient -- 对齐 C++ getRequestClient()
    // =======================================================================

    /**
     * 获取请求队列（不分配 clientId）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::getRequestClient(size_t shmRequestKey, uint64_t shmSize)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:370-379
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列容量
     * @return 注册的 MWMRQueue
     */
    public MWMRQueue getRequestClient(int shmKey, int queueSize) {
        // C++: if (m_reqClientCount == MAXSIZE - 1) throw ...;
        if (reqClientCount == maxSize - 1) {
            throw new RuntimeException("Request clients have exceeded the maximum limit, which is " + maxSize);
        }
        // C++: m_reqClients[m_reqClientCount++] = new ReqShmQ(shmRequestKey, shmSize, 0666);
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, reqDataSize, reqElemSize);
        reqQueues[reqClientCount++] = queue;
        return queue;
    }

    // =======================================================================
    //  getReqMsgQueueForClient -- 对齐 C++ getReqMsgQueueForClient()
    // =======================================================================

    /**
     * 按 clientId 获取请求队列。
     * <p>
     * 迁移自: MultiClientStoreShmReader::getReqMsgQueueForClient(size_t shmRequestKey, uint64_t shmSize, uint32_t clientId)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:445-455
     *
     * @param shmKey   SysV SHM key
     * @param queueSize 队列容量
     * @param clientId 客户端 ID
     * @return MWMRQueue 或 null (clientId 越界)
     */
    public MWMRQueue getReqMsgQueueForClient(int shmKey, int queueSize, int clientId) {
        // C++: if (clientId >= MAXSIZE) return nullptr;
        if (clientId >= maxSize) {
            return null;
        }
        // C++: return m_reqClients[clientId] = new ReqShmQ(shmRequestKey, shmSize, 0666);
        MWMRQueue queue = MWMRQueue.open(shmKey, queueSize, reqDataSize, reqElemSize);
        reqQueues[clientId] = queue;
        return queue;
    }

    // =======================================================================
    //  getMaxClientId -- 对齐 C++ getMaxClientId()
    // =======================================================================

    /**
     * 获取当前最大 clientId。
     * <p>
     * 迁移自: MultiClientStoreShmReader::getMaxClientId(size_t clientStoreKey)
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:457-466
     *
     * @param clientStoreKey ClientStore SHM key (-1 使用默认)
     * @return 当前 clientId 值
     */
    public long getMaxClientId(int clientStoreKey) {
        // C++: if (clientStoreKey == -1) clientStoreKey = m_defaultClientStoreKey;
        if (clientStoreKey == -1) {
            clientStoreKey = defaultClientStoreKey;
        }
        // C++: return clientStores[clientStoreKey]->getClientId();
        return clientStores.get(clientStoreKey).getClientId();
    }

    /**
     * 获取当前最大 clientId（使用默认 ClientStore）。
     */
    public long getMaxClientId() {
        return getMaxClientId(-1);
    }

    // =======================================================================
    //  totalSHMRequestQueues -- 对齐 C++ totalSHMRequestQueues()
    // =======================================================================

    /**
     * 获取请求队列总数。
     * <p>
     * 迁移自: MultiClientStoreShmReader::totalSHMRequestQueues()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:579-588
     *
     * @return 所有 ClientStore 的队列总数
     */
    public long totalSHMRequestQueues() {
        // C++: for (auto it = clientStores.begin(); ...) count += getClientId() - getFirstClientIdValue();
        long count = 0;
        for (ClientStore cs : clientStores.values()) {
            count += cs.getClientId() - cs.getFirstClientIdValue();
        }
        return count;
    }

    // =======================================================================
    //  轮询方法 -- 对齐 C++ loopMD / loopRequest / loopResponse
    // =======================================================================

    /**
     * 轮询行情队列 (ROUND_ROBIN 模式)。
     * <p>
     * 迁移自: MultiClientStoreShmReader::loopMD()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:100-123
     * <p>
     * C++:
     * <pre>
     *   for (int i = 0; i &lt; m_mdClientCount; i++) {
     *       if (!m_mdClients[i]-&gt;isEmpty()) {
     *           MD data = m_mdClients[i]-&gt;dequeue();
     *           EMIT_PARAM(MarketUpdateAvailable, &amp;data)
     *       }
     *   }
     * </pre>
     */
    private void loopMD() {
        for (int i = 0; i < mdClientCount; i++) {
            // C++: if (!m_mdClients[i]->isEmpty())
            if (!mdQueues[i].isEmpty()) {
                // C++: MD data = m_mdClients[i]->dequeue();
                if (mdQueues[i].dequeue(mdBuf)) {
                    // C++: EMIT_PARAM(MarketUpdateAvailable, &data)
                    if (mdCallback != null) {
                        mdCallback.onMarketUpdate(mdBuf);
                    }
                }
            }
        }
    }

    /**
     * 选择 timestamp 最小的 endPkt 客户端。
     * <p>
     * 迁移自: MultiClientStoreShmReader::getBestEndPacketClientAsPerTimestampSequencing()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:125-147
     * <p>
     * C++:
     * <pre>
     *   int minclient = -1;
     *   uint64_t mintimestamp = numeric_limits&lt;uint64_t&gt;::max();
     *   for (int i = 0; i &lt; m_mdWithEndPacketClientCount; i++) {
     *       if (!m_mdWithEndPacketClients[i]-&gt;contains_new_data)
     *           m_mdWithEndPacketClients[i]-&gt;fetch_data_if_possible_from_queue();
     *       if (m_mdWithEndPacketClients[i]-&gt;contains_new_data
     *           &amp;&amp; m_mdWithEndPacketClients[i]-&gt;data.m_timestamp &lt; mintimestamp) {
     *           mintimestamp = m_mdWithEndPacketClients[i]-&gt;data.m_timestamp;
     *           minclient = i;
     *       }
     *   }
     *   return minclient;
     * </pre>
     *
     * @return 最佳客户端索引，-1 表示所有队列为空
     */
    private int getBestEndPacketClientAsPerTimestampSequencing() {
        int minClient = -1;
        long minTimestamp = Long.MAX_VALUE;
        for (int i = 0; i < mdEndPktClientCount; i++) {
            // C++: if (!m_mdWithEndPacketClients[i]->contains_new_data)
            if (!mdEndPktClients[i].containsNewData) {
                // C++: m_mdWithEndPacketClients[i]->fetch_data_if_possible_from_queue();
                mdEndPktClients[i].fetchDataIfPossibleFromQueue();
            }
            // C++: if (contains_new_data && data.m_timestamp < mintimestamp)
            if (mdEndPktClients[i].containsNewData && mdEndPktClients[i].getTimestamp() < minTimestamp) {
                minTimestamp = mdEndPktClients[i].getTimestamp();
                minClient = i;
            }
        }
        // C++: return minclient; // -1 if all queues are empty
        return minClient;
    }

    /**
     * 轮询行情队列 (UNTIL_ENDPACKET 模式)。
     * <p>
     * 迁移自: MultiClientStoreShmReader::loopMD_until_endpacket()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:149-203
     */
    private void loopMDUntilEndpacket() {
        for (int j = 0; j < mdEndPktClientCount; j++) {
            // C++: int i = getBestEndPacketClientAsPerTimestampSequencing();
            int i = getBestEndPacketClientAsPerTimestampSequencing();
            if (i == -1) {
                // C++: // This would mean all queues are empty
                continue;
            }

            MDEndPktClient client = mdEndPktClients[i];

            // C++: if ((data.m_endPkt == 1) && m_mdWithEndPacketClients[i]->last_packet_was_endpacket)
            if (client.getEndPkt() == 1 && client.lastPacketWasEndpacket) {
                continue;
            }

            // C++: EMIT_PARAM(MarketUpdateAvailable, &data)
            if (mdCallback != null) {
                mdCallback.onMarketUpdate(client.data);
            }
            // C++: m_mdWithEndPacketClients[i]->last_packet_was_endpacket = (data.m_endPkt == 1);
            client.lastPacketWasEndpacket = (client.getEndPkt() == 1);

            // C++: while (data.m_endPkt == 0) { ... }
            while (client.getEndPkt() == 0) {
                // C++: while (m_mdWithEndPacketClients[i]->isEmpty()) { /* Wait */ }
                while (client.isEmpty()) {
                    Thread.onSpinWait();
                }

                // C++: data = m_mdWithEndPacketClients[i]->dequeue();
                client.dequeue(client.data);

                // C++: EMIT_PARAM(MarketUpdateAvailable, &data)
                if (mdCallback != null) {
                    mdCallback.onMarketUpdate(client.data);
                }
            }

            // C++: m_mdWithEndPacketClients[i]->contains_new_data = false;
            client.containsNewData = false;
            // C++: m_mdWithEndPacketClients[i]->last_packet_was_endpacket = (data.m_endPkt == 1);
            client.lastPacketWasEndpacket = (client.getEndPkt() == 1);
        }
    }

    /**
     * 轮询请求队列。
     * <p>
     * 迁移自: MultiClientStoreShmReader::loopRequest()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:205-236
     */
    private void loopRequest() {
        for (int i = 0; i < reqClientCount; i++) {
            // C++: if (!m_reqClients[i]->isEmpty())
            if (!reqQueues[i].isEmpty()) {
                // C++: REQ data = m_reqClients[i]->dequeue();
                if (reqQueues[i].dequeue(reqBuf)) {
                    // C++: EMIT_PARAM(ORSRequestAvailable, &data)
                    if (orsRequestCallback != null) {
                        orsRequestCallback.onRequest(reqBuf);
                    }
                }
            }
        }
    }

    /**
     * 轮询回报队列。
     * <p>
     * 迁移自: MultiClientStoreShmReader::loopResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:238-248
     * <p>
     * C++:
     * <pre>
     *   for (int i = 0; i &lt; m_respClientCount; i++) {
     *       if (!m_respClients[i]-&gt;isEmpty()) {
     *           RESP data = m_respClients[i]-&gt;dequeue();
     *           EMIT_PARAM(ORSResponseAvailable, &amp;data, i)
     *       }
     *   }
     * </pre>
     */
    private void loopResponse() {
        for (int i = 0; i < respClientCount; i++) {
            // C++: if (!m_respClients[i]->isEmpty())
            if (!respQueues[i].isEmpty()) {
                // C++: RESP data = m_respClients[i]->dequeue();
                if (respQueues[i].dequeue(respBuf)) {
                    // C++: EMIT_PARAM(ORSResponseAvailable, &data, i)
                    if (orsResponseCallback != null) {
                        orsResponseCallback.onResponse(respBuf, i);
                    }
                }
            }
        }
    }

    // =======================================================================
    //  组合轮询 -- 对齐 C++ startMonitorAll / startMonitorAsyncAll
    // =======================================================================

    /**
     * 组合轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAll()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:482-493
     */
    public void startMonitorAll() {
        active = true;
        // C++: while (m_active) { loopMD(); loopMD_until_endpacket(); loopRequest(); loopResponse(); }
        while (active) {
            loopMD();
            loopMDUntilEndpacket();
            loopRequest();
            loopResponse();
        }
    }

    /**
     * 组合轮询（异步线程）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAsyncAll()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:471-474
     */
    public void startMonitorAsyncAll() {
        // C++: m_threadHandler = std::thread(&MultiClientStoreShmReader::startMonitorAll, this);
        threadHandler = new Thread(this::startMonitorAll, "ShmMgr-All");
        threadHandler.start();
    }

    /**
     * 等待组合轮询线程结束。
     * <p>
     * 迁移自: MultiClientStoreShmReader::waitForCompletionAll()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:475-481
     */
    public void waitForCompletionAll() {
        // C++: if (m_threadHandler.joinable()) m_threadHandler.join();
        if (threadHandler != null) {
            try { threadHandler.join(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
        }
    }

    // =======================================================================
    //  行情轮询 -- 对齐 C++ startMonitorMarketData / startMonitorAsyncMarketData
    // =======================================================================

    /**
     * 仅行情轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorMarketData()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:514-525
     */
    public void startMonitorMarketData() {
        mdActive = true;
        // C++: while (m_mdActive) { loopMD(); loopMD_until_endpacket(); }
        while (mdActive) {
            loopMD();
            loopMDUntilEndpacket();
        }
    }

    /**
     * 仅行情轮询（异步线程）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAsyncMarketData()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:498-501
     */
    public void startMonitorAsyncMarketData() {
        // C++: m_mdThread = std::thread(&MultiClientStoreShmReader::startMonitorMarketData, this);
        mdThread = new Thread(this::startMonitorMarketData, "ShmMgr-MD");
        mdThread.start();
    }

    /**
     * 等待行情线程结束。
     * <p>
     * 迁移自: MultiClientStoreShmReader::waitForCompletionMarketData()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:507-513
     */
    public void waitForCompletionMarketData() {
        // C++: if (m_mdThread.joinable()) m_mdThread.join();
        if (mdThread != null) {
            try { mdThread.join(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
        }
    }

    // =======================================================================
    //  行情+回报轮询 -- 对齐 C++ startMonitorMarketDataAndResponse
    // =======================================================================

    /**
     * 行情+回报轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorMarketDataAndResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:527-548
     */
    public void startMonitorMarketDataAndResponse() {
        mdActive = true;
        orsResponseActive = true;
        // C++: while (m_mdActive || m_orsResponseActive)
        while (mdActive || orsResponseActive) {
            // C++: if (illuminati_unlikely_branch(m_mdActive))
            if (mdActive) {
                loopMD();
                loopMDUntilEndpacket();
            }
            // C++: if (illuminati_unlikely_branch(m_orsResponseActive))
            if (orsResponseActive) {
                loopResponse();
            }
        }
    }

    /**
     * 行情+回报轮询（异步线程）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAsyncMarketDataAndResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:503-506
     */
    public void startMonitorAsyncMarketDataAndResponse() {
        // C++: m_mdThread = std::thread(&MultiClientStoreShmReader::startMonitorMarketDataAndResponse, this);
        mdThread = new Thread(this::startMonitorMarketDataAndResponse, "ShmMgr-MD-Resp");
        mdThread.start();
    }

    // =======================================================================
    //  行情+回报+请求轮询
    // =======================================================================

    /**
     * 行情+回报+请求轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorMarketDataAndResponseAndRequest()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:550-577
     */
    public void startMonitorMarketDataAndResponseAndRequest() {
        mdActive = true;
        orsResponseActive = true;
        orsRequestActive = true;
        // C++: while (m_mdActive || m_orsResponseActive || m_orsRequestActive)
        while (mdActive || orsResponseActive || orsRequestActive) {
            if (mdActive) {
                loopMD();
                loopMDUntilEndpacket();
            }
            if (orsResponseActive) {
                loopResponse();
            }
            if (orsRequestActive) {
                loopRequest();
            }
        }
    }

    // =======================================================================
    //  请求轮询 -- 对齐 C++ startMonitorORSRequest
    // =======================================================================

    /**
     * 请求轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorORSRequest()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:606-617
     */
    public void startMonitorORSRequest() {
        orsRequestActive = true;
        // C++: while (m_orsRequestActive) { loopRequest(); }
        while (orsRequestActive) {
            loopRequest();
        }
    }

    /**
     * 请求轮询（异步线程）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAsyncORSRequest()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:593-597
     * <p>
     * C++: 实际调用 startMonitorORSRequestHighPerf
     */
    public void startMonitorAsyncORSRequest() {
        // C++: m_orsRequestThread = std::thread(&MultiClientStoreShmReader::startMonitorORSRequestHighPerf, this);
        orsRequestThread = new Thread(this::startMonitorORSRequestHighPerf, "ShmMgr-Req");
        orsRequestThread.start();
    }

    /**
     * 等待请求线程结束。
     * <p>
     * 迁移自: MultiClientStoreShmReader::waitForCompletionORSRequest()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:599-605
     */
    public void waitForCompletionORSRequest() {
        if (orsRequestThread != null) {
            try { orsRequestThread.join(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
        }
    }

    /**
     * 高性能请求轮询 -- 轮询所有请求队列。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorORSRequestHighPerf()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:382-438
     * <p>
     * C++:
     * <pre>
     *   setpriority(PRIO_PROCESS, 0, -20);
     *   while (totalSHMRequestQueues() == 0 &amp;&amp; m_orsRequestActive) { }
     *   int i = 0;
     *   while (m_orsRequestActive) {
     *       if (!m_reqClients[i]-&gt;isEmpty()) {
     *           m_reqClients[i]-&gt;dequeuePtr(&amp;data2);
     *           EMIT_PARAM(ORSRequestAvailable, &amp;data2);
     *       }
     *       int32_t totalShmQueues = m_reqClientCount;  // USE_MWMRQ_REQSHM
     *       i = (i + 1) % (totalShmQueues);
     *   }
     * </pre>
     * <p>
     * [C++差异] C++ setpriority(PRIO_PROCESS, 0, -20) 提升线程优先级，
     * Java 使用 Thread.MAX_PRIORITY 近似。
     */
    public void startMonitorORSRequestHighPerf() {
        // C++: setpriority(PRIO_PROCESS, 0, -20);
        // [C++差异] Java 无法直接设置 nice 值，使用 Thread.MAX_PRIORITY 近似
        Thread.currentThread().setPriority(Thread.MAX_PRIORITY);

        orsRequestActive = true;

        // C++: while (totalSHMRequestQueues() == 0 && m_orsRequestActive) { }
        while (totalSHMRequestQueues() == 0 && orsRequestActive) {
            Thread.onSpinWait();
        }

        int i = 0;

        // C++: while (m_orsRequestActive)
        while (orsRequestActive) {
            // C++: if (!m_reqClients[i]->isEmpty())
            if (!reqQueues[i].isEmpty()) {
                // C++: m_reqClients[i]->dequeuePtr(&data2);
                if (reqQueues[i].dequeue(reqBuf)) {
                    // C++: EMIT_PARAM(ORSRequestAvailable, &data2);
                    if (orsRequestCallback != null) {
                        orsRequestCallback.onRequest(reqBuf);
                    }
                }
            }
            // C++: int32_t totalShmQueues = m_reqClientCount;  (USE_MWMRQ_REQSHM)
            int totalShmQueues = reqClientCount;
            // C++: i = (i + 1) % (totalShmQueues);
            i = (i + 1) % totalShmQueues;
        }
    }

    // =======================================================================
    //  回报轮询 -- 对齐 C++ startMonitorORSResponse
    // =======================================================================

    /**
     * 回报轮询（阻塞）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorORSResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:651-661
     */
    public void startMonitorORSResponse() {
        orsResponseActive = true;
        // C++: while (m_orsResponseActive) { loopResponse(); }
        while (orsResponseActive) {
            loopResponse();
        }
    }

    /**
     * 回报轮询（异步线程）。
     * <p>
     * 迁移自: MultiClientStoreShmReader::startMonitorAsyncORSResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:640-643
     */
    public void startMonitorAsyncORSResponse() {
        // C++: m_orsResponseThread = std::thread(&MultiClientStoreShmReader::startMonitorORSResponse, this);
        orsResponseThread = new Thread(this::startMonitorORSResponse, "ShmMgr-Resp");
        orsResponseThread.start();
    }

    /**
     * 等待回报线程结束。
     * <p>
     * 迁移自: MultiClientStoreShmReader::waitForCompletionORSResponse()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:644-650
     */
    public void waitForCompletionORSResponse() {
        if (orsResponseThread != null) {
            try { orsResponseThread.join(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
        }
    }

    // =======================================================================
    //  停止和清理 -- 对齐 C++ stopMonitor / shutdown
    // =======================================================================

    /**
     * 停止所有轮询。
     * <p>
     * 迁移自: MultiClientStoreShmReader::stopMonitor()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:717-723
     * <p>
     * C++:
     * <pre>
     *   m_active = false;
     *   m_mdActive = false;
     *   m_orsRequestActive = false;
     *   m_orsResponseActive = false;
     * </pre>
     */
    public void stopMonitor() {
        active = false;
        mdActive = false;
        orsRequestActive = false;
        orsResponseActive = false;
    }

    /**
     * 停止所有轮询线程并清理资源。
     * <p>
     * 迁移自: MultiClientStoreShmReader::shutdown()
     * Ref: hftbase/Ipc/include/multiclientstoreshmreader.h:666-712
     * <p>
     * C++:
     * <pre>
     *   stopMonitor();
     *   m_threadHandler.join();
     *   m_mdThread.join();
     *   m_orsRequestThread.join();
     *   m_orsResponseThread.join();
     *   for (i...) delete m_mdClients[i];
     *   for (i...) delete m_mdWithEndPacketClients[i];
     *   for (i...) delete m_respClients[i];
     *   for (i...) delete m_reqClients[i];
     *   m_mdClientCount = 0; m_mdWithEndPacketClientCount = 0;
     *   m_respClientCount = 0; m_reqClientCount = 0;
     * </pre>
     */
    public void shutdown() {
        // C++: stopMonitor();
        stopMonitor();

        // C++: m_threadHandler.join(); m_mdThread.join(); m_orsRequestThread.join(); m_orsResponseThread.join();
        joinThread(threadHandler, "threadHandler");
        joinThread(mdThread, "mdThread");
        joinThread(orsRequestThread, "orsRequestThread");
        joinThread(orsResponseThread, "orsResponseThread");

        // C++: for (i...) delete m_mdClients[i]; etc.
        for (int i = 0; i < mdClientCount; i++) {
            if (mdQueues[i] != null) { mdQueues[i].close(); mdQueues[i] = null; }
        }
        for (int i = 0; i < mdEndPktClientCount; i++) {
            if (mdEndPktClients[i] != null) { mdEndPktClients[i].queue.close(); mdEndPktClients[i] = null; }
        }
        for (int i = 0; i < respClientCount; i++) {
            if (respQueues[i] != null) { respQueues[i].close(); respQueues[i] = null; }
        }
        for (int i = 0; i < reqClientCount; i++) {
            if (reqQueues[i] != null) { reqQueues[i].close(); reqQueues[i] = null; }
        }

        // C++: ~MultiClientStoreShmReader() 中 delete clientStores 中的所有条目
        // [C++差异-Java 资源管理] C++ 依赖析构函数释放 clientStores，
        // Java 需要在 shutdown() 中显式 close 以释放 SHM 附着。
        for (ClientStore cs : clientStores.values()) {
            cs.close();
        }
        clientStores.clear();

        // C++: m_mdClientCount = 0; ...
        mdClientCount = 0;
        mdEndPktClientCount = 0;
        respClientCount = 0;
        reqClientCount = 0;
    }

    private void joinThread(Thread t, String name) {
        if (t != null && t.isAlive()) {
            try {
                log.info("waiting for " + name + " to complete");
                t.join();
                log.info(name + " done");
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }
    }

    // =======================================================================
    //  Getter — 供 Connector 等外部类访问
    // =======================================================================

    /** 获取已注册的行情队列数。 */
    public int getMdClientCount() { return mdClientCount; }

    /** 获取已注册的 endPkt 行情客户端数。 */
    public int getMdEndPktClientCount() { return mdEndPktClientCount; }

    /** 获取已注册的请求队列数。 */
    public int getReqClientCount() { return reqClientCount; }

    /** 获取已注册的回报队列数。 */
    public int getRespClientCount() { return respClientCount; }

    /** 获取指定行情队列。 */
    public MWMRQueue getMdQueue(int index) { return mdQueues[index]; }

    /** 获取指定请求队列。 */
    public MWMRQueue getReqQueue(int index) { return reqQueues[index]; }

    /** 获取指定回报队列。 */
    public MWMRQueue getRespQueue(int index) { return respQueues[index]; }

    /** 获取 ClientStore 映射。 */
    public Map<Integer, ClientStore> getClientStores() { return clientStores; }
}
