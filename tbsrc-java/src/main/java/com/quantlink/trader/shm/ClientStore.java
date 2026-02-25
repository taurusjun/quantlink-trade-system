package com.quantlink.trader.shm;

import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.lang.invoke.VarHandle;

/**
 * 无锁共享内存客户端 ID 存储 -- 使用 Java 22+ Panama FFI。
 * <p>
 * 迁移自: hftbase/Ipc/include/locklessshmclientstore.h
 *         -- illuminati::ipc::LocklessShmClientStore&lt;uint64_t&gt;
 * <p>
 * SHM 内存布局 (16 bytes):
 * <pre>
 *   offset 0:  atomic&lt;uint64_t&gt; data           (8 bytes) -- 当前客户端 ID（原子递增）
 *   offset 8:  uint64_t         firstClientId   (8 bytes) -- 初始客户端 ID（只读）
 * </pre>
 * <p>
 * C++ 类继承链:
 *   LocklessShmClientStore&lt;uint64_t&gt; : SharedMemory
 * <p>
 * [C++差异] C++ 通过继承 SharedMemory 完成 SHM 创建/连接，Java 使用组合 SysVShm.ShmSegment。
 * <p>
 * [C++差异] C++ 模板参数 IntType = uint64_t；Java 中固定使用 long（等价于 int64/uint64）。
 */
public final class ClientStore {

    // -----------------------------------------------------------------------
    //  常量
    // -----------------------------------------------------------------------

    // C++: sizeof(ClientData) = 16
    // C++: struct ClientData {
    //          std::atomic<IntType> data __attribute__((aligned(sizeof(IntType))));
    //          IntType firstCliendId __attribute__((aligned(sizeof(IntType))));
    //      };
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:22-25
    private static final long CLIENT_DATA_SIZE = Types.CLIENT_DATA_SIZE; // 16

    // C++: atomic<uint64_t> data 在 offset 0
    private static final long DATA_OFFSET = 0;

    // C++: uint64_t firstCliendId 在 offset 8
    // (注意: C++ 原代码中 "firstCliendId" 是拼写错误 "Client" -> "Cliend"，保留原样)
    private static final long FIRST_CLIENT_ID_OFFSET = 8;

    // -----------------------------------------------------------------------
    //  VarHandle for atomic operations on data field (offset 0 in SHM)
    // -----------------------------------------------------------------------

    // C++: std::atomic<IntType> data  在 SHM 的 offset 0
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:23
    private static final VarHandle DATA_VH =
            ValueLayout.JAVA_LONG.varHandle();

    // -----------------------------------------------------------------------
    //  实例字段
    // -----------------------------------------------------------------------

    // C++: SharedMemory 基类中的 m_shmid, m_shmadr
    // Ref: hftbase/Ipc/include/sharedmemory.h:111-112
    private final SysVShm.ShmSegment seg;

    // C++: m_shmadr (SHM 起始地址对应的 MemorySegment)
    private final MemorySegment mem;

    // -----------------------------------------------------------------------
    //  构造（私有）
    // -----------------------------------------------------------------------

    private ClientStore(SysVShm.ShmSegment seg) {
        this.seg = seg;
        this.mem = seg.segment();
    }

    // -----------------------------------------------------------------------
    //  工厂方法
    // -----------------------------------------------------------------------

    /**
     * 连接到已存在的 ClientStore（生产用）。
     * <p>
     * 迁移自: LocklessShmClientStore::init() 的已存在路径
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:43-63
     * <p>
     * C++:
     * <pre>
     *   size_t m_shmsize = sizeof(ClientData);
     *   bool balreadyExisting = SharedMemory::init(shmkey, 1, m_shmsize, flag);
     *   m_data = (ClientData *)(m_shmadr);
     *   // 已存在时不初始化
     *   std::cout &lt;&lt; "Store already existing, attaching to it: "
     *             &lt;&lt; m_data-&gt;data.load(std::memory_order_relaxed) &lt;&lt; "\n";
     * </pre>
     *
     * @param shmKey SysV SHM key (例如 0x4001)
     * @return 已连接的 ClientStore
     */
    public static ClientStore open(int shmKey) {
        // C++: size_t m_shmsize = sizeof(ClientData);  // = 16
        // C++: SharedMemory::init(shmkey, 1, m_shmsize, flag);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:47-48
        SysVShm.ShmSegment seg = SysVShm.open(shmKey, CLIENT_DATA_SIZE);
        return new ClientStore(seg);
    }

    /**
     * 创建新的 ClientStore（测试用 / 首次初始化）。
     * <p>
     * 迁移自: LocklessShmClientStore::init() 的新建路径
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:43-63
     * <p>
     * C++:
     * <pre>
     *   if (!balreadyExisting &amp;&amp; ((flag &amp; IPC_CREAT) == IPC_CREAT)) {
     *       std::cout &lt;&lt; "Initializing Client Store Values \n";
     *       m_data-&gt;data.store(initialValue);
     *       m_data-&gt;firstCliendId = initialValue;
     *   }
     * </pre>
     *
     * @param shmKey       SysV SHM key (例如 0x4001)
     * @param initialValue 初始客户端 ID 值
     * @return 新创建的 ClientStore
     */
    public static ClientStore create(int shmKey, long initialValue) {
        // C++: size_t m_shmsize = sizeof(ClientData);  // = 16
        // C++: SharedMemory::init(shmkey, 1, m_shmsize, flag);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:47-48
        SysVShm.ShmSegment seg = SysVShm.create(shmKey, CLIENT_DATA_SIZE);

        // C++: m_data->data.store(initialValue);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:54
        // memory_order: C++ 默认 store 为 seq_cst，Java 使用 setRelease（足够）
        DATA_VH.setRelease(seg.segment(), DATA_OFFSET, initialValue);

        // C++: m_data->firstCliendId = initialValue;
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:55
        // 普通写入（非原子），仅在初始化时设置一次
        seg.segment().set(ValueLayout.JAVA_LONG, FIRST_CLIENT_ID_OFFSET, initialValue);

        return new ClientStore(seg);
    }

    // -----------------------------------------------------------------------
    //  核心操作
    // -----------------------------------------------------------------------

    /**
     * 读取当前客户端 ID（不递增）。
     * <p>
     * 迁移自: LocklessShmClientStore::getClientId()
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:67-69
     * <p>
     * C++: return m_data-&gt;data.load(std::memory_order_acquire);
     *
     * @return 当前客户端 ID 值
     */
    public long getClientId() {
        // C++: return m_data->data.load(std::memory_order_acquire);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:68
        return (long) DATA_VH.getAcquire(mem, DATA_OFFSET);
    }

    /**
     * 设置客户端 ID。
     * <p>
     * 迁移自: LocklessShmClientStore::setClientId(IntType value)
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:71-74
     * <p>
     * C++: m_data-&gt;data.store(value, std::memory_order_release);
     *
     * @param value 新的客户端 ID 值
     */
    public void setClientId(long value) {
        // C++: m_data->data.store(value, std::memory_order_release);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:73
        DATA_VH.setRelease(mem, DATA_OFFSET, value);
    }

    /**
     * 原子递增并返回递增前的客户端 ID（fetch_add 语义）。
     * <p>
     * 迁移自: LocklessShmClientStore::getClientIdAndIncrement()
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:77-80
     * <p>
     * C++: return m_data-&gt;data.fetch_add(1, std::memory_order_acq_rel);
     *
     * @return 递增前的客户端 ID 值
     */
    public long getClientIdAndIncrement() {
        // C++: return m_data->data.fetch_add(1, std::memory_order_acq_rel);
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:80
        return (long) DATA_VH.getAndAdd(mem, DATA_OFFSET, 1L);
    }

    /**
     * 读取初始客户端 ID（创建时设置的值）。
     * <p>
     * 迁移自: LocklessShmClientStore::getFirstClientIdValue()
     * Ref: hftbase/Ipc/include/locklessshmclientstore.h:82-85
     * <p>
     * C++: return m_data-&gt;firstCliendId;
     *
     * @return 初始客户端 ID 值
     */
    public long getFirstClientIdValue() {
        // C++: return m_data->firstCliendId;
        // Ref: hftbase/Ipc/include/locklessshmclientstore.h:84
        // 普通读取（非原子），firstCliendId 只在初始化时写入一次
        return mem.get(ValueLayout.JAVA_LONG, FIRST_CLIENT_ID_OFFSET);
    }

    // -----------------------------------------------------------------------
    //  生命周期
    // -----------------------------------------------------------------------

    /**
     * 分离共享内存（不删除）。
     * <p>
     * 迁移自: SharedMemory::~SharedMemory() 中的 shmdt(m_shmadr)
     * Ref: hftbase/Ipc/include/sharedmemory.h:35
     */
    public void close() {
        seg.detach();
    }

    /**
     * 分离并标记删除共享内存段。
     * <p>
     * 迁移自: SharedMemory::~SharedMemory() 中的 shmdt + shmctl(IPC_RMID)
     * Ref: hftbase/Ipc/include/sharedmemory.h:35-37
     */
    public void destroy() {
        seg.detach();
        seg.remove();
    }
}
