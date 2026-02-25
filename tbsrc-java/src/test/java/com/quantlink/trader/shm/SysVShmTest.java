package com.quantlink.trader.shm;

import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;

import java.lang.foreign.ValueLayout;

import static org.junit.jupiter.api.Assertions.*;

/**
 * SysVShm 单元测试。
 * <p>
 * 测试需要运行在 Linux 或 macOS 上（SysV SHM 可用的环境）。
 * 使用高位 key (0x7F_xxxx) 避免与运行中的 QuantLink 系统冲突。
 * <p>
 * 测试用例对照 C++ 行为:
 * - create → write → detach → open → read → verify → remove
 * - create 已存在的 key 回退为 open（EEXIST fallback）
 * - 页对齐验证（与 C++ sysconf(_SC_PAGESIZE) 对齐逻辑一致）
 */
@EnabledOnOs({OS.LINUX, OS.MAC})
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class SysVShmTest {

    // 使用高位 key 避免与正在运行的 QuantLink 系统冲突
    // QuantLink 使用 0x1001, 0x2001, 0x3001, 0x4001
    private static final int TEST_KEY = 0x7F_FF01;

    /**
     * 测试 1: 完整生命周期
     *   create → 写入 int → detach → open → 读取 int → 验证一致 → remove
     * <p>
     * 对照 C++ SharedMemory::init() 的完整流程:
     *   shmget(IPC_CREAT|IPC_EXCL) → shmat → 写数据 → shmdt → shmget(0666) → shmat → 读数据
     * Ref: hftbase/Ipc/include/sharedmemory.h:57-107
     */
    @Test
    @Order(1)
    void test_createWriteDetachOpenRead_fullLifecycle() {
        int testValue = 0xDEAD_BEEF;
        long size = 4096;

        // --- Phase 1: create + write ---
        SysVShm.ShmSegment seg1 = SysVShm.create(TEST_KEY, size);
        assertNotNull(seg1, "create 应返回非 null ShmSegment");
        assertTrue(seg1.shmid() >= 0, "shmid 应为非负数, 实际=" + seg1.shmid());
        assertTrue(seg1.size() >= size, "对齐后 size 应 >= 请求 size");

        // 写入一个 int 到共享内存偏移 0
        seg1.segment().set(ValueLayout.JAVA_INT, 0, testValue);

        // 验证写入后可立即读取
        int readBack = seg1.segment().get(ValueLayout.JAVA_INT, 0);
        assertEquals(testValue, readBack, "写入后立即读取应一致");

        // --- Phase 2: detach ---
        assertDoesNotThrow(seg1::detach, "detach 不应抛异常");

        // --- Phase 3: open + read ---
        SysVShm.ShmSegment seg2 = SysVShm.open(TEST_KEY, size);
        assertNotNull(seg2, "open 应返回非 null ShmSegment");
        assertEquals(seg1.shmid(), seg2.shmid(), "同 key 的 shmid 应相同");

        int readFromOpen = seg2.segment().get(ValueLayout.JAVA_INT, 0);
        assertEquals(testValue, readFromOpen,
                "open 后读取的值应与 create 时写入的一致: expected=0x"
                        + Integer.toHexString(testValue) + " actual=0x" + Integer.toHexString(readFromOpen));

        // --- Phase 4: cleanup ---
        seg2.detach();
        seg2.remove();
    }

    /**
     * 测试 2: create 已存在的 key 应回退为 open（EEXIST fallback）
     * <p>
     * 对照 C++ SharedMemory::init():
     *   m_shmid = shmget(shmkey, m_shmsize, tempFlag);  // tempFlag 含 IPC_EXCL
     *   if (m_shmid < 0) {
     *       if (errno == EEXIST) { m_balreadyExisting = true; }
     *   }
     *   if (m_shmid < 0) {
     *       m_shmid = shmget(shmkey, m_shmsize, flag);  // 回退不含 IPC_EXCL
     *   }
     * Ref: hftbase/Ipc/include/sharedmemory.h:60-94
     */
    @Test
    @Order(2)
    void test_createExistingKey_fallbackToOpen() {
        int key = 0x7F_FF02;
        long size = 4096;
        int sentinel = 42;

        // 第一次 create
        SysVShm.ShmSegment seg1 = SysVShm.create(key, size);
        assertNotNull(seg1);
        seg1.segment().set(ValueLayout.JAVA_INT, 0, sentinel);

        try {
            // 第二次 create 相同 key — 应回退为 open，不抛异常
            SysVShm.ShmSegment seg2 = assertDoesNotThrow(
                    () -> SysVShm.create(key, size),
                    "对已存在 key 调用 create 不应抛异常（应回退为 open）");

            assertNotNull(seg2);
            assertEquals(seg1.shmid(), seg2.shmid(),
                    "回退后 shmid 应与第一次 create 相同");

            // 验证数据完整性 — 第二次 create 应能读到第一次写入的值
            int readBack = seg2.segment().get(ValueLayout.JAVA_INT, 0);
            assertEquals(sentinel, readBack,
                    "回退 open 后应能读取到第一次 create 写入的数据");

            seg2.detach();
        } finally {
            seg1.detach();
            seg1.remove();
        }
    }

    /**
     * 测试 3: 页对齐验证
     * <p>
     * 对照 C++ SharedMemory::init():
     *   long sz = sysconf(_SC_PAGESIZE);
     *   size_t m_shmsize = size_in + sz - (size_in % sz);
     * Ref: hftbase/Ipc/include/sharedmemory.h:49-52
     * <p>
     * C++ 实现: 当 size_in 已经是页对齐时，结果是 size_in + pageSize
     * 例如: pageAlign(4096) = 4096 + 4096 - 0 = 8192
     */
    @Test
    @Order(3)
    void test_pageAlign_variousSizes() {
        // C++: size_in + sz - (size_in % sz)  where sz = 4096

        // 1字节 → 4096 + 4096 - (4096 % 4096 对 1 字节: 1 + 4096 - 1) = 4096
        // 实际: 1 + 4096 - (1 % 4096) = 1 + 4096 - 1 = 4096
        assertEquals(4096L, SysVShm.pageAlign(1),
                "pageAlign(1) 应为 4096");

        // 100 字节 → 100 + 4096 - 100 = 4096
        assertEquals(4096L, SysVShm.pageAlign(100),
                "pageAlign(100) 应为 4096");

        // 4095 字节 → 4095 + 4096 - 4095 = 4096
        assertEquals(4096L, SysVShm.pageAlign(4095),
                "pageAlign(4095) 应为 4096");

        // 4096 字节 (已对齐) → C++ 行为: 4096 + 4096 - 0 = 8192
        // 注意: C++ 实现在已对齐时多分配一页，Java 保持一致
        assertEquals(8192L, SysVShm.pageAlign(4096),
                "pageAlign(4096) 应为 8192 (C++ 行为: 已对齐时多分配一页)");

        // 4097 字节 → 4097 + 4096 - 1 = 8192
        assertEquals(8192L, SysVShm.pageAlign(4097),
                "pageAlign(4097) 应为 8192");

        // 8192 字节 (已对齐) → 8192 + 4096 - 0 = 12288
        assertEquals(12288L, SysVShm.pageAlign(8192),
                "pageAlign(8192) 应为 12288 (C++ 行为)");

        // 大 size: 1MB → 1048576 + 4096 - 0 = 1052672
        assertEquals(1052672L, SysVShm.pageAlign(1048576),
                "pageAlign(1MB) 应为 1MB + 4096");
    }

    /**
     * 测试 4: open 不存在的 key 应抛 ShmException
     */
    @Test
    @Order(4)
    void test_openNonExistentKey_throwsShmException() {
        // 使用一个几乎不可能存在的 key
        int nonExistentKey = 0x7F_FFFF;

        assertThrows(ShmException.class,
                () -> SysVShm.open(nonExistentKey, 4096),
                "open 不存在的 key 应抛出 ShmException");
    }

    /**
     * 测试 5: 写入多种数据类型并验证
     * <p>
     * 验证 MemorySegment 的 reinterpret(size) 正确设置了边界，
     * 能支持多偏移量的读写。
     */
    @Test
    @Order(5)
    void test_multipleDataTypes_readWriteConsistency() {
        int key = 0x7F_FF03;
        long size = 4096;

        SysVShm.ShmSegment seg = SysVShm.create(key, size);
        try {
            // 写入 int 到 offset 0
            seg.segment().set(ValueLayout.JAVA_INT, 0, 12345);
            // 写入 long 到 offset 8 (8字节对齐)
            seg.segment().set(ValueLayout.JAVA_LONG, 8, 9876543210L);
            // 写入 double 到 offset 16
            seg.segment().set(ValueLayout.JAVA_DOUBLE, 16, 3.14159265358979);
            // 写入 short 到 offset 24
            seg.segment().set(ValueLayout.JAVA_SHORT, 24, (short) -32768);

            // 读取并验证
            assertEquals(12345, seg.segment().get(ValueLayout.JAVA_INT, 0));
            assertEquals(9876543210L, seg.segment().get(ValueLayout.JAVA_LONG, 8));
            assertEquals(3.14159265358979, seg.segment().get(ValueLayout.JAVA_DOUBLE, 16), 1e-15);
            assertEquals((short) -32768, seg.segment().get(ValueLayout.JAVA_SHORT, 24));
        } finally {
            seg.detach();
            seg.remove();
        }
    }
}
