package com.quantlink.trader.shm;

import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.lang.invoke.VarHandle;

/**
 * MWMR (Multi-Writer Multi-Reader) 共享内存队列 -- 使用 Java 22+ Panama FFI。
 * <p>
 * 迁移自: hftbase/Ipc/include/multiwritermultireadershmqueue.h
 *         -- illuminati::ds::MultiWriterMultiReaderShmQueue&lt;T&gt;
 * <p>
 * SHM 内存布局:
 * <pre>
 *   [MWMRHeader (8 bytes)]  -- atomic&lt;int64_t&gt; head
 *   [QueueElem[0]]          -- { T data; uint64_t seqNo; }
 *   [QueueElem[1]]          -- ...
 *   ...
 *   [QueueElem[size-1]]
 * </pre>
 * <p>
 * C++ 类继承链:
 *   MultiWriterMultiReaderShmQueue&lt;T&gt; : ShmAllocator&lt;QueueElem&lt;T&gt;, MultiWriterMultiReaderShmHeader&gt; : SharedMemory
 * <p>
 * [C++差异] C++ 使用模板参数化元素类型 T (MultiWriterMultiReaderShmQueue&lt;T&gt;)，
 *           编译期自动推导 sizeof(T) 和 sizeof(QueueElem&lt;T&gt;)。
 *           Java 泛型存在类型擦除，无法获取结构体内存布局大小，
 *           因此改为 dataSize/elemSize 参数 + 原始 MemorySegment，由调用者传入正确的结构体大小。
 *           Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h — template&lt;typename T&gt;
 * <p>
 * [C++差异] C++ 通过继承 ShmAllocator 完成 SHM 创建/连接，Java 使用组合 SysVShm.ShmSegment。
 */
public final class MWMRQueue {

    // -----------------------------------------------------------------------
    //  常量
    // -----------------------------------------------------------------------

    // C++: sizeof(MultiWriterMultiReaderShmHeader) = 8
    // C++: struct MultiWriterMultiReaderShmHeader { std::atomic<int64_t> head; }
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:15-23
    private static final long HEADER_SIZE = Types.MWMR_HEADER_SIZE; // 8

    // -----------------------------------------------------------------------
    //  VarHandle for atomic operations on head field (offset 0 in SHM)
    // -----------------------------------------------------------------------

    // C++: std::atomic<int64_t> head  在 SHM 的 offset 0
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:17
    //
    // 使用 JAVA_LONG 的 VarHandle 进行原子操作。
    // head 在 SHM offset 0，天然 8 字节对齐。
    private static final VarHandle HEAD_VH =
            ValueLayout.JAVA_LONG.varHandle();

    // C++: uint64_t seqNo  在 QueueElem 的 data 之后
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:29
    private static final VarHandle SEQ_VH =
            ValueLayout.JAVA_LONG.varHandle();

    // -----------------------------------------------------------------------
    //  实例字段
    // -----------------------------------------------------------------------

    // C++: ShmAllocator 基类中的 m_shmid, m_shmadr
    // Ref: hftbase/Ipc/include/sharedmemory.h:111-112
    private final SysVShm.ShmSegment seg;

    // C++: m_shmadr (SHM 起始地址对应的 MemorySegment)
    private final MemorySegment mem;

    // C++: sizeof(QueueElem<T>) -- 包含 data + seqNo + padding
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:26-30
    private final long elemSize;

    // C++: sizeof(T) -- 纯数据大小，不含 seqNo
    private final long dataSize;

    // C++: ShmStore::m_size -- 队列容量（已确保为 2 的幂）
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:88
    private final long size;

    // C++: (ShmStore::m_size - 1) -- 用于环形索引的位掩码
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:123
    private final long mask;

    // C++: std::atomic<int64_t> tail -- 本地读指针，非 SHM 存储
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:42
    // [C++差异] C++ 中 tail 是 std::atomic<int64_t>，因为 dequeuePtrBlock 支持多线程消费；
    //           Java 中当前仅有单个 Connector 轮询线程消费，因此使用普通 long。
    //           如需多线程消费，需改为 AtomicLong 并对齐 C++ 的 CAS 自旋逻辑。
    //           Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:42
    private long localTail;

    // C++: QueueElem 中 seqNo 相对于 QueueElem 起始的偏移
    // C++: struct QueueElem { T data; uint64_t seqNo; }
    // seqNo offset = sizeof(T) = dataSize
    // 对于 RequestMsg (aligned(64)): seqNo at offset 256 within QueueElem, elemSize=320
    // 对于 MarketUpdateNew: seqNo at offset 816 within QueueElem, elemSize=824
    // 对于 ResponseMsg: seqNo at offset 176 within QueueElem, elemSize=184
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:28-29
    private final long seqNoOffsetInElem;

    // -----------------------------------------------------------------------
    //  构造（私有）
    // -----------------------------------------------------------------------

    private MWMRQueue(SysVShm.ShmSegment seg, long size, long dataSize, long elemSize, long initialTail) {
        this.seg = seg;
        this.mem = seg.segment();
        this.elemSize = elemSize;
        this.dataSize = dataSize;
        this.size = size;
        this.mask = size - 1;
        this.localTail = initialTail;

        // C++: QueueElem<T> = { T data; uint64_t seqNo; }
        // seqNo 紧跟在 data(sizeof(T)) 之后，但 QueueElem 的总大小可能因 T 的 alignment
        // 而大于 sizeof(T) + 8。例如 RequestMsg(aligned(64)): sizeof(T)=256, sizeof(QueueElem)=320。
        // seqNo 的偏移始终是 sizeof(T)（即 dataSize），不是 elemSize - 8。
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:28-29
        //   struct QueueElem { T data; uint64_t seqNo; }
        //   编译器将 seqNo 放在 offsetof(QueueElem, seqNo) = sizeof(T)
        this.seqNoOffsetInElem = dataSize;
    }

    // -----------------------------------------------------------------------
    //  工厂方法
    // -----------------------------------------------------------------------

    /**
     * 连接到已存在的 MWMR 队列（生产用: reader 连接到 writer 已创建的 SHM）。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::init() 的 non-IPC_CREAT 路径
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:84-106
     * <p>
     * C++: tail = ShmStore::header-&gt;head.load(std::memory_order_relaxed);
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:99
     * （未定义 _TAIL_Q_START 时）
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列元素个数（会自动调整到 2 的幂）
     * @param dataSize  sizeof(T) -- 纯数据大小
     * @param elemSize  sizeof(QueueElem&lt;T&gt;) -- 含 seqNo + alignment padding
     * @return 已连接的 MWMRQueue
     */
    public static MWMRQueue open(int shmKey, int queueSize, long dataSize, long elemSize) {
        // C++: shmsize = getMinHighestPowOf2(shmsize);
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:88
        long actualSize = nextPowerOf2(queueSize);

        // C++: ShmStore::init(shmkey, shmsize, flag);
        // 总 SHM 大小 = sizeof(Header) + actualSize * sizeof(QueueElem<T>)
        // Ref: hftbase/Ipc/include/shmallocator.h:52
        long totalShmSize = HEADER_SIZE + actualSize * elemSize;
        SysVShm.ShmSegment seg = SysVShm.open(shmKey, totalShmSize);

        // C++: tail = ShmStore::header->head.load(std::memory_order_relaxed);
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:99
        long currentHead = (long) HEAD_VH.getAcquire(seg.segment(), 0L);

        return new MWMRQueue(seg, actualSize, dataSize, elemSize, currentHead);
    }

    /**
     * 创建新的 MWMR 队列（测试用 / 首次初始化）。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::init() 的 IPC_CREAT 路径
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:84-106
     * <p>
     * C++: MultiWriterMultiReaderShmHeader() { head.store(1); }
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:19-22
     * <p>
     * C++: tail = 1;  (当定义 _TAIL_Q_START 时)
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:97
     *
     * @param shmKey    SysV SHM key
     * @param queueSize 队列元素个数（会自动调整到 2 的幂）
     * @param dataSize  sizeof(T) -- 纯数据大小
     * @param elemSize  sizeof(QueueElem&lt;T&gt;) -- 含 seqNo + alignment padding
     * @return 新创建的 MWMRQueue
     */
    public static MWMRQueue create(int shmKey, int queueSize, long dataSize, long elemSize) {
        // C++: shmsize = getMinHighestPowOf2(shmsize);
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:88
        long actualSize = nextPowerOf2(queueSize);

        // C++: ShmStore::init(shmkey, shmsize, flag);
        // Ref: hftbase/Ipc/include/shmallocator.h:52
        long totalShmSize = HEADER_SIZE + actualSize * elemSize;
        SysVShm.ShmSegment seg = SysVShm.create(shmKey, totalShmSize);

        // C++: MultiWriterMultiReaderShmHeader() { head.store(1); }
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:19-22
        // 初始化 head = 1（create 模式下新段需要初始化）
        HEAD_VH.setRelease(seg.segment(), 0L, 1L);

        // C++: tail = 1;
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:69 (构造函数)
        // 以及 L97 (#ifdef _TAIL_Q_START 路径)
        // create 模式下 tail 从 1 开始，与 head 初始值一致
        return new MWMRQueue(seg, actualSize, dataSize, elemSize, 1L);
    }

    // -----------------------------------------------------------------------
    //  核心操作
    // -----------------------------------------------------------------------

    /**
     * 入队: 将 value 写入队列。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::enqueue(const T &amp;value)
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:118-133
     * <p>
     * C++ 原代码:
     * <pre>
     *   int64_t myHead = ShmStore::header-&gt;head.fetch_add(1, std::memory_order_acq_rel);
     *   QueueElem&lt;T&gt; *slot = ShmStore::m_updates + (myHead &amp; (ShmStore::m_size - 1));
     *   memcpy(&amp;(slot-&gt;data), &amp;value, sizeof(T));
     *   asm volatile("" : : : "memory");   // compiler barrier
     *   slot-&gt;seqNo = myHead;
     * </pre>
     *
     * @param value MemorySegment，大小应 &gt;= dataSize
     */
    public void enqueue(MemorySegment value) {
        // C++: int64_t myHead = ShmStore::header->head.fetch_add(1, std::memory_order_acq_rel);
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:121
        long myHead = (long) HEAD_VH.getAndAdd(mem, 0L, 1L);

        // C++: QueueElem<T> *slot = ShmStore::m_updates + (myHead & (ShmStore::m_size - 1));
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:123
        long slotOffset = HEADER_SIZE + (myHead & mask) * elemSize;

        // C++: memcpy(&(slot->data), &value, sizeof(T));
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:124
        MemorySegment.copy(value, 0, mem, slotOffset, dataSize);

        // C++: asm volatile("" : : : "memory");  // compiler barrier
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:125
        // Java 等价: storeStoreFence 确保 data 写入在 seqNo 写入之前对其他线程可见
        VarHandle.storeStoreFence();

        // C++: slot->seqNo = myHead;
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:126
        // [C++差异] C++ 中这是一个 plain store（非 atomic），紧跟 compiler barrier。
        //           Java 中使用 setRelease 确保 reader 以 acquire 语义读到正确的 data。
        SEQ_VH.setRelease(mem, slotOffset + seqNoOffsetInElem, myHead);
    }

    /**
     * 出队: 从队列读取一条数据到 out。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::dequeuePtr(T *data)
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:204-211
     * <p>
     * C++ 原代码:
     * <pre>
     *   QueueElem&lt;T&gt; *value = ShmStore::m_updates + (tail.load(relaxed) &amp; (ShmStore::m_size - 1));
     *   memcpy(data, &amp;(value-&gt;data), sizeof(T));
     *   tail.store(value-&gt;seqNo + 1, relaxed);
     * </pre>
     * <p>
     * 注意: C++ dequeuePtr 不检查 isEmpty，调用者必须先调用 isEmpty() 判断。
     * Java 版本合并了 isEmpty 检查以提供更安全的 API。
     *
     * @param out 接收数据的 MemorySegment，大小应 &gt;= dataSize
     * @return true 如果成功出队，false 如果队列为空
     */
    public boolean dequeue(MemorySegment out) {
        // C++: QueueElem<T> *value = ShmStore::m_updates + (tail & (ShmStore::m_size - 1));
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:207
        long slotOffset = HEADER_SIZE + (localTail & mask) * elemSize;

        // C++: (slot)->seqNo < tail  (isEmpty check)
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:248
        // 使用 getAcquire 读取 seqNo，确保后续 data 读取看到正确值
        long seqNo = (long) SEQ_VH.getAcquire(mem, slotOffset + seqNoOffsetInElem);
        if (seqNo < localTail) {
            return false;
        }

        // C++: memcpy(data, &(value->data), sizeof(T));
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:208
        MemorySegment.copy(mem, slotOffset, out, 0, dataSize);

        // C++: tail.store(value->seqNo + 1, std::memory_order_relaxed);
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:209
        localTail = seqNo + 1;

        return true;
    }

    /**
     * 判断队列是否为空（从本地读指针视角）。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::isEmpty()
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:245-249
     * <p>
     * C++ 原代码:
     * <pre>
     *   return (ShmStore::m_updates + (tail &amp; (ShmStore::m_size - 1)))-&gt;seqNo &lt; tail;
     * </pre>
     *
     * @return true 如果队列为空（没有新数据可读）
     */
    public boolean isEmpty() {
        // C++: (ShmStore::m_updates + (tail & (ShmStore::m_size - 1)))->seqNo < tail
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:248
        long slotOffset = HEADER_SIZE + (localTail & mask) * elemSize;
        long seqNo = (long) SEQ_VH.getAcquire(mem, slotOffset + seqNoOffsetInElem);
        return seqNo < localTail;
    }

    /**
     * 获取当前 head 值（SHM 中的原子值）。
     * 用于诊断和监控。
     *
     * @return 当前 head 值
     */
    public long getHead() {
        // C++: ShmStore::header->head.load(std::memory_order_relaxed)
        // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:99
        return (long) HEAD_VH.getAcquire(mem, 0L);
    }

    /**
     * 获取本地 tail 值（读指针）。
     *
     * @return 当前 localTail 值
     */
    public long getLocalTail() {
        return localTail;
    }

    /**
     * 获取队列容量（2 的幂）。
     *
     * @return 队列大小
     */
    public long getSize() {
        return size;
    }

    /**
     * 获取数据大小 sizeof(T)。
     *
     * @return 数据大小（字节）
     */
    public long getDataSize() {
        return dataSize;
    }

    /**
     * 获取队列元素大小 sizeof(QueueElem&lt;T&gt;)。
     *
     * @return 元素大小（字节）
     */
    public long getElemSize() {
        return elemSize;
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

    // -----------------------------------------------------------------------
    //  内部工具
    // -----------------------------------------------------------------------

    /**
     * 计算 &gt;= value 的最小 2 的幂。
     * <p>
     * 迁移自: MultiWriterMultiReaderShmQueue::getMinHighestPowOf2(int64_t value)
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:46-59
     * <p>
     * C++ 原代码:
     * <pre>
     *   if (!value &amp; (!(value &amp; (value - 1)))) return value;
     *   int64_t result = 1;
     *   while (value) { value = value &gt;&gt; 1; result = result &lt;&lt; 1; }
     *   return result;
     * </pre>
     * <p>
     * 注意: C++ 原实现存在运算符优先级 bug（!value 先于 &amp; 求值），
     * 导致已经是 2 的幂的值被翻倍 (例如 getMinHighestPowOf2(4) = 8)。
     * Java 实现修正为标准语义（4 → 4, 8 → 8），与实际 SHM 队列大小匹配。
     * Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:165-172
     */
    static long nextPowerOf2(long value) {
        if (value <= 0) {
            return 1;
        }
        // C++: if (value & (value - 1) == 0) -- 检查是否已经是 2 的幂
        if ((value & (value - 1)) == 0) {
            return value;
        }
        // 找到 >= value 的最小 2 的幂
        long result = 1;
        while (result < value) {
            result <<= 1;
        }
        return result;
    }
}
