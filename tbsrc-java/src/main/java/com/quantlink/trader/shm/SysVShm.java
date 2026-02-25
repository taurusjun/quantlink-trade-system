package com.quantlink.trader.shm;

import java.lang.foreign.*;
import java.lang.invoke.MethodHandle;

/**
 * SysV 共享内存封装 -- 使用 Java 22+ Panama FFI（java.lang.foreign.Linker）。
 * <p>
 * 迁移自: hftbase/Ipc/include/sharedmemory.h  (illuminati::ipc::SharedMemory)
 * 迁移自: hftbase/Ipc/include/shmallocator.h   (illuminati::ipc::ShmAllocator)
 * <p>
 * [C++差异] C++ 使用继承体系 SharedMemory → ShmAllocator&lt;T,Header&gt;,
 *           Java 扁平化为工具类 + ShmSegment 值对象，与 Go 迁移方案一致。
 * <p>
 * 通过 {@code Linker.nativeLinker().defaultLookup()} 查找 libc 中的
 * shmget/shmat/shmdt/shmctl 符号，跨 Linux/macOS 无需 syscall 编号。
 */
public final class SysVShm {

    // -----------------------------------------------------------------------
    //  IPC 常量 (与 C++ <sys/ipc.h> / <sys/shm.h> 一致)
    // -----------------------------------------------------------------------
    // C++: IPC_CREAT  defined in <sys/ipc.h>
    // Ref: hftbase/Ipc/include/sharedmemory.h:42  (flag = IPC_CREAT | 0666)
    private static final int IPC_CREAT = 0x200;   // 01000 octal = 0x200

    // C++: IPC_EXCL   defined in <sys/ipc.h>
    // Ref: hftbase/Ipc/include/sharedmemory.h:60  (int tempFlag = flag | IPC_EXCL)
    private static final int IPC_EXCL  = 0x400;   // 02000 octal = 0x400

    // C++: IPC_RMID   defined in <sys/ipc.h>
    // Ref: hftbase/Ipc/include/sharedmemory.h:37  (shmctl(m_shmid, IPC_RMID, 0))
    private static final int IPC_RMID  = 0;

    // 默认权限: 0666 (rw-rw-rw-)
    // C++: IPC_CREAT | 0666
    // Ref: hftbase/Ipc/include/sharedmemory.h:42
    private static final int DEFAULT_PERM = 0666;  // 0x1B6

    // -----------------------------------------------------------------------
    //  libc 函数句柄（延迟初始化，线程安全）
    // -----------------------------------------------------------------------
    private static final MethodHandle SHMGET;
    private static final MethodHandle SHMAT;
    private static final MethodHandle SHMDT;
    private static final MethodHandle SHMCTL;

    static {
        Linker linker = Linker.nativeLinker();
        SymbolLookup lookup = linker.defaultLookup();

        // int shmget(int key, size_t size, int shmflg)
        // C++: m_shmid = shmget(shmkey, m_shmsize, tempFlag);
        // Ref: hftbase/Ipc/include/sharedmemory.h:61
        SHMGET = linker.downcallHandle(
                lookup.find("shmget").orElseThrow(() ->
                        new UnsatisfiedLinkError("shmget not found in libc")),
                FunctionDescriptor.of(
                        ValueLayout.JAVA_INT,     // return: int shmid
                        ValueLayout.JAVA_INT,     // key
                        ValueLayout.JAVA_LONG,    // size (size_t → long)
                        ValueLayout.JAVA_INT      // shmflg
                )
        );

        // void* shmat(int shmid, const void* shmaddr, int shmflg)
        // C++: m_shmadr = shmat(m_shmid, NULL, 0)
        // Ref: hftbase/Ipc/include/sharedmemory.h:96
        SHMAT = linker.downcallHandle(
                lookup.find("shmat").orElseThrow(() ->
                        new UnsatisfiedLinkError("shmat not found in libc")),
                FunctionDescriptor.of(
                        ValueLayout.ADDRESS,      // return: void*
                        ValueLayout.JAVA_INT,     // shmid
                        ValueLayout.ADDRESS,      // shmaddr (NULL)
                        ValueLayout.JAVA_INT      // shmflg
                )
        );

        // int shmdt(const void* shmaddr)
        // C++: shmdt(m_shmadr)
        // Ref: hftbase/Ipc/include/sharedmemory.h:35
        SHMDT = linker.downcallHandle(
                lookup.find("shmdt").orElseThrow(() ->
                        new UnsatisfiedLinkError("shmdt not found in libc")),
                FunctionDescriptor.of(
                        ValueLayout.JAVA_INT,     // return: int
                        ValueLayout.ADDRESS       // shmaddr
                )
        );

        // int shmctl(int shmid, int cmd, struct shmid_ds* buf)
        // C++: shmctl(m_shmid, IPC_RMID, 0)
        // Ref: hftbase/Ipc/include/sharedmemory.h:37
        SHMCTL = linker.downcallHandle(
                lookup.find("shmctl").orElseThrow(() ->
                        new UnsatisfiedLinkError("shmctl not found in libc")),
                FunctionDescriptor.of(
                        ValueLayout.JAVA_INT,     // return: int
                        ValueLayout.JAVA_INT,     // shmid
                        ValueLayout.JAVA_INT,     // cmd
                        ValueLayout.ADDRESS       // buf (NULL for IPC_RMID)
                )
        );
    }

    private SysVShm() {
        // 工具类，禁止实例化
    }

    // =======================================================================
    //  ShmSegment — 已连接的共享内存段
    // =======================================================================

    /**
     * 已连接的 SysV 共享内存段。
     * <p>
     * 迁移自: hftbase/Ipc/include/sharedmemory.h  SharedMemory 的成员变量
     * <p>
     * 对应 C++ 成员:
     * <ul>
     *   <li>{@code m_shmid}  → {@link #shmid}</li>
     *   <li>{@code m_shmadr} → {@link #segment}</li>
     *   <li>{@code m_size}   → {@link #size}</li>
     * </ul>
     */
    public static final class ShmSegment {

        // C++: int32_t m_shmid;
        // Ref: hftbase/Ipc/include/sharedmemory.h:112
        private final int shmid;

        // C++: void* m_shmadr;  (shmat 返回的地址)
        // Ref: hftbase/Ipc/include/sharedmemory.h:111
        private final MemorySegment segment;

        // C++: size_t m_size;  (页对齐后的字节数)
        // Ref: hftbase/Ipc/include/sharedmemory.h:114
        private final long size;

        private ShmSegment(int shmid, MemorySegment segment, long size) {
            this.shmid = shmid;
            this.segment = segment;
            this.size = size;
        }

        /** 返回 SysV 共享内存 ID (shmget 返回值) */
        public int shmid() {
            return shmid;
        }

        /**
         * 返回 MemorySegment，可用于读写共享内存。
         * 边界已设置为 {@link #size()} 字节。
         */
        public MemorySegment segment() {
            return segment;
        }

        /** 返回页对齐后的共享内存大小（字节） */
        public long size() {
            return size;
        }

        /**
         * 分离共享内存段（不删除）。
         * <p>
         * 迁移自: hftbase/Ipc/include/sharedmemory.h:35
         * C++: shmdt(m_shmadr);
         *
         * @throws ShmException 如果 shmdt 调用失败
         */
        public void detach() {
            try {
                int ret = (int) SHMDT.invokeExact(segment);
                if (ret < 0) {
                    throw new ShmException("shmdt failed, return=" + ret);
                }
            } catch (ShmException e) {
                throw e;
            } catch (Throwable t) {
                throw new ShmException("shmdt invocation failed", t);
            }
        }

        /**
         * 标记共享内存段待删除（IPC_RMID），当所有进程 detach 后内核释放。
         * <p>
         * 迁移自: hftbase/Ipc/include/sharedmemory.h:36-37
         * C++: if ((m_shmflag & IPC_CREAT) == IPC_CREAT) shmctl(m_shmid, IPC_RMID, 0);
         * <p>
         * [C++差异] C++ 析构函数中仅创建者（flag 含 IPC_CREAT）执行 IPC_RMID；
         *           Java 改为显式调用 remove()，由调用方控制生命周期。
         *
         * @throws ShmException 如果 shmctl 调用失败
         */
        public void remove() {
            try {
                int ret = (int) SHMCTL.invokeExact(shmid, IPC_RMID, MemorySegment.NULL);
                if (ret < 0) {
                    throw new ShmException("shmctl(IPC_RMID) failed, shmid=" + shmid + " return=" + ret);
                }
            } catch (ShmException e) {
                throw e;
            } catch (Throwable t) {
                throw new ShmException("shmctl invocation failed", t);
            }
        }
    }

    // =======================================================================
    //  公开 API
    // =======================================================================

    /**
     * 连接到已存在的 SysV 共享内存段。
     * <p>
     * 迁移自: hftbase/Ipc/include/sharedmemory.h:82-94
     * C++: m_shmid = shmget(shmkey, m_shmsize, flag);  // 不含 IPC_CREAT
     *      m_shmadr = shmat(m_shmid, NULL, 0);
     *
     * @param key  SysV SHM key (例如 0x1001)
     * @param size 期望的共享内存大小（字节），会自动页对齐
     * @return 已连接的 ShmSegment
     * @throws ShmException 如果 shmget 或 shmat 失败
     */
    public static ShmSegment open(int key, long size) {
        // C++: size_t m_shmsize = size_in + sz - (size_in % sz);
        // Ref: hftbase/Ipc/include/sharedmemory.h:52
        long alignedSize = pageAlign(size);

        try {
            // C++: m_shmid = shmget(shmkey, m_shmsize, flag);
            // Ref: hftbase/Ipc/include/sharedmemory.h:84
            // open 模式: flag = 0666 (仅连接，不创建)
            int shmid = (int) SHMGET.invokeExact(key, alignedSize, DEFAULT_PERM);
            if (shmid < 0) {
                throw new ShmException(
                        "shmget failed: key=0x" + Integer.toHexString(key)
                                + " size=" + alignedSize + " shmid=" + shmid);
            }

            // C++: m_shmadr = shmat(m_shmid, NULL, 0)
            // Ref: hftbase/Ipc/include/sharedmemory.h:96
            MemorySegment rawAddr = (MemorySegment) SHMAT.invokeExact(shmid, MemorySegment.NULL, 0);

            // shmat 返回 (void*)-1 表示失败
            // C++: if ((m_shmadr = shmat(m_shmid, NULL, 0)) == (void *)-1)
            // Ref: hftbase/Ipc/include/sharedmemory.h:96
            if (rawAddr.address() == -1L) {
                throw new ShmException(
                        "shmat failed: shmid=" + shmid);
            }

            // 设置 MemorySegment 的边界为 alignedSize
            MemorySegment bounded = rawAddr.reinterpret(alignedSize);
            return new ShmSegment(shmid, bounded, alignedSize);

        } catch (ShmException e) {
            throw e;
        } catch (Throwable t) {
            throw new ShmException("SHM open invocation failed: key=0x" + Integer.toHexString(key), t);
        }
    }

    /**
     * 创建新的 SysV 共享内存段；如果同 key 已存在则回退为 {@link #open(int, long)}。
     * <p>
     * 迁移自: hftbase/Ipc/include/sharedmemory.h:57-107
     * <p>
     * C++ 逻辑:
     * <pre>
     *   // 1. 尝试 IPC_CREAT|IPC_EXCL 创建全新段
     *   int tempFlag = flag | IPC_EXCL;
     *   m_shmid = shmget(shmkey, m_shmsize, tempFlag);
     *   if (m_shmid < 0) {
     *       if (errno == EEXIST) {
     *           m_balreadyExisting = true;
     *       } else {
     *           throw ...;
     *       }
     *   }
     *   // 2. 如果已存在，用不含 IPC_EXCL 的 flag 重新获取
     *   if (m_shmid < 0) {
     *       m_shmid = shmget(shmkey, m_shmsize, flag);
     *   }
     *   // 3. shmat
     *   m_shmadr = shmat(m_shmid, NULL, 0);
     * </pre>
     *
     * @param key  SysV SHM key (例如 0x1001)
     * @param size 期望的共享内存大小（字节），会自动页对齐
     * @return 已连接的 ShmSegment
     * @throws ShmException 如果 shmget 或 shmat 失败
     */
    public static ShmSegment create(int key, long size) {
        // C++: size_t m_shmsize = size_in + sz - (size_in % sz);
        // Ref: hftbase/Ipc/include/sharedmemory.h:52
        long alignedSize = pageAlign(size);

        try {
            // C++: int tempFlag = flag | IPC_EXCL;
            // C++: m_shmid = shmget(shmkey, m_shmsize, tempFlag);
            // Ref: hftbase/Ipc/include/sharedmemory.h:60-61
            int shmid = (int) SHMGET.invokeExact(key, alignedSize, IPC_CREAT | IPC_EXCL | DEFAULT_PERM);

            if (shmid < 0) {
                // C++: if (errno == EEXIST) { m_balreadyExisting = true; }
                // Ref: hftbase/Ipc/include/sharedmemory.h:65-66
                //
                // [C++差异] C++ 检查 errno == EEXIST; Java 通过 shmget 返回 -1 检测，
                //           然后回退为不含 IPC_EXCL 的调用。如果回退也失败则抛出异常。
                //           由于 Panama FFI 无法直接读取 errno，我们采用与 Go 相同的
                //           回退策略：shmget 返回负值时尝试不带 IPC_EXCL 重新获取。

                // C++: m_shmid = shmget(shmkey, m_shmsize, flag);
                // Ref: hftbase/Ipc/include/sharedmemory.h:84
                shmid = (int) SHMGET.invokeExact(key, alignedSize, IPC_CREAT | DEFAULT_PERM);
                if (shmid < 0) {
                    throw new ShmException(
                            "shmget(create fallback) failed: key=0x" + Integer.toHexString(key)
                                    + " size=" + alignedSize + " shmid=" + shmid);
                }
            }

            // C++: m_shmadr = shmat(m_shmid, NULL, 0)
            // Ref: hftbase/Ipc/include/sharedmemory.h:96
            MemorySegment rawAddr = (MemorySegment) SHMAT.invokeExact(shmid, MemorySegment.NULL, 0);

            // C++: if ((m_shmadr = shmat(m_shmid, NULL, 0)) == (void *)-1)
            // Ref: hftbase/Ipc/include/sharedmemory.h:96
            if (rawAddr.address() == -1L) {
                throw new ShmException("shmat failed: shmid=" + shmid);
            }

            // 设置 MemorySegment 的边界为 alignedSize
            MemorySegment bounded = rawAddr.reinterpret(alignedSize);
            return new ShmSegment(shmid, bounded, alignedSize);

        } catch (ShmException e) {
            throw e;
        } catch (Throwable t) {
            throw new ShmException("SHM create invocation failed: key=0x" + Integer.toHexString(key), t);
        }
    }

    // =======================================================================
    //  内部工具
    // =======================================================================

    /**
     * 将 size 向上对齐到操作系统页大小（通常 4096）的整数倍。
     * <p>
     * 迁移自: hftbase/Ipc/include/sharedmemory.h:49-52
     * C++: long sz = sysconf(_SC_PAGESIZE);
     *      size_t m_shmsize = size_in + sz - (size_in % sz);
     * <p>
     * 注意: C++ 实现在 size_in 已经是页对齐时会多分配一页。
     * 例如 size_in=4096, sz=4096 → m_shmsize = 4096 + 4096 - 0 = 8192。
     * Java 实现保持与 C++ 完全一致的行为。
     *
     * @param size 原始大小
     * @return 页对齐后的大小
     */
    static long pageAlign(long size) {
        // C++: long sz = sysconf(_SC_PAGESIZE);
        // Ref: hftbase/Ipc/include/sharedmemory.h:49
        long pageSize = 4096L; // 标准 Linux/macOS 页大小

        // C++: size_t m_shmsize = size_in + sz - (size_in % sz);
        // Ref: hftbase/Ipc/include/sharedmemory.h:52
        return size + pageSize - (size % pageSize);
    }
}
