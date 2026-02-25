package com.quantlink.trader.shm;

import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;

import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * ClientStore 单元测试。
 * <p>
 * 迁移自: hftbase/Ipc/include/locklessshmclientstore.h
 *         -- illuminati::ipc::LocklessShmClientStore&lt;uint64_t&gt;
 * <p>
 * 测试需要运行在 Linux 或 macOS 上（SysV SHM 可用的环境）。
 * 使用高位 key (0x7F_xxxx) 避免与运行中的 QuantLink 系统冲突。
 * <p>
 * 对照 C++ 行为:
 * - create(key, initialValue) 初始化 data 和 firstCliendId
 * - getClientId() 返回当前 data (acquire)
 * - getClientIdAndIncrement() 返回旧值并原子 +1 (acq_rel)
 * - getFirstClientIdValue() 返回初始值 (plain read)
 */
@EnabledOnOs({OS.LINUX, OS.MAC})
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class ClientStoreTest {

    // 使用高位 key 避免冲突
    private static final int TEST_KEY_BASIC = 0x7F_B001;
    private static final int TEST_KEY_CONCURRENT = 0x7F_B002;
    private static final int TEST_KEY_FIRST_ID = 0x7F_B003;
    private static final int TEST_KEY_SET = 0x7F_B004;

    // =====================================================================
    // 测试 1: create -> getClientId -> getClientIdAndIncrement -> verify
    // 对照 C++ LocklessShmClientStore 完整生命周期
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:43-85
    // =====================================================================

    @Test
    @Order(1)
    void test_createGetIncrementLifecycle() {
        long initialValue = 100L;

        ClientStore store = ClientStore.create(TEST_KEY_BASIC, initialValue);
        try {
            // C++: m_data->data.store(initialValue);
            // Ref: hftbase/Ipc/include/locklessshmclientstore.h:54
            assertEquals(initialValue, store.getClientId(),
                    "初始 getClientId 应返回 initialValue");

            // C++: m_data->firstCliendId = initialValue;
            // Ref: hftbase/Ipc/include/locklessshmclientstore.h:55
            assertEquals(initialValue, store.getFirstClientIdValue(),
                    "getFirstClientIdValue 应返回 initialValue");

            // C++: return m_data->data.fetch_add(1, std::memory_order_acq_rel);
            // Ref: hftbase/Ipc/include/locklessshmclientstore.h:80
            long old1 = store.getClientIdAndIncrement();
            assertEquals(100L, old1, "第 1 次 getClientIdAndIncrement 应返回 100");
            assertEquals(101L, store.getClientId(), "递增后 getClientId 应返回 101");

            long old2 = store.getClientIdAndIncrement();
            assertEquals(101L, old2, "第 2 次 getClientIdAndIncrement 应返回 101");
            assertEquals(102L, store.getClientId(), "再次递增后 getClientId 应返回 102");

            long old3 = store.getClientIdAndIncrement();
            assertEquals(102L, old3, "第 3 次 getClientIdAndIncrement 应返回 102");
            assertEquals(103L, store.getClientId(), "三次递增后 getClientId 应返回 103");

            // firstClientIdValue 不应变化
            assertEquals(initialValue, store.getFirstClientIdValue(),
                    "递增不应影响 firstClientIdValue");
        } finally {
            store.destroy();
        }
    }

    // =====================================================================
    // 测试 2: 多线程并发 getClientIdAndIncrement 唯一性
    // C++ 核心特性: atomic fetch_add 保证多线程下每次返回不同值
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:77-80
    // =====================================================================

    @Test
    @Order(2)
    void test_concurrentGetClientIdAndIncrement_uniqueness() throws InterruptedException {
        long initialValue = 1000L;
        int numThreads = 8;
        int incrementsPerThread = 500;
        int totalIncrements = numThreads * incrementsPerThread;

        ClientStore store = ClientStore.create(TEST_KEY_CONCURRENT, initialValue);
        try {
            CountDownLatch startLatch = new CountDownLatch(1);
            CountDownLatch doneLatch = new CountDownLatch(numThreads);
            Set<Long> allIds = ConcurrentHashMap.newKeySet();
            AtomicInteger errors = new AtomicInteger(0);

            for (int t = 0; t < numThreads; t++) {
                Thread thread = new Thread(() -> {
                    try {
                        startLatch.await();
                        for (int i = 0; i < incrementsPerThread; i++) {
                            long id = store.getClientIdAndIncrement();
                            if (!allIds.add(id)) {
                                errors.incrementAndGet(); // 重复 ID
                            }
                        }
                    } catch (Exception e) {
                        errors.incrementAndGet();
                    } finally {
                        doneLatch.countDown();
                    }
                });
                thread.setDaemon(true);
                thread.start();
            }

            startLatch.countDown();
            doneLatch.await();

            assertEquals(0, errors.get(), "不应有错误或重复 ID");
            assertEquals(totalIncrements, allIds.size(),
                    "应有 " + totalIncrements + " 个不同的 ID");

            // 验证最终值
            // C++: 初始值 1000 + 4000 次 fetch_add(1) = 5000
            assertEquals(initialValue + totalIncrements, store.getClientId(),
                    "最终 clientId 应为 initialValue + totalIncrements");

            // 验证所有 ID 在 [1000, 1000+totalIncrements) 范围内
            for (long id : allIds) {
                assertTrue(id >= initialValue && id < initialValue + totalIncrements,
                        "ID " + id + " 应在 [" + initialValue + ", " + (initialValue + totalIncrements) + ") 范围内");
            }

            // firstClientIdValue 不应变化
            assertEquals(initialValue, store.getFirstClientIdValue(),
                    "并发递增不应影响 firstClientIdValue");
        } finally {
            store.destroy();
        }
    }

    // =====================================================================
    // 测试 3: getFirstClientIdValue 验证
    // C++: m_data->firstCliendId = initialValue; (初始化时设置)
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:55
    // =====================================================================

    @Test
    @Order(3)
    void test_getFirstClientIdValue_immutable() {
        long initialValue = 42L;

        ClientStore store = ClientStore.create(TEST_KEY_FIRST_ID, initialValue);
        try {
            assertEquals(42L, store.getFirstClientIdValue(),
                    "firstClientIdValue 应为初始值 42");

            // 多次递增
            for (int i = 0; i < 100; i++) {
                store.getClientIdAndIncrement();
            }

            // firstClientIdValue 应保持不变
            assertEquals(42L, store.getFirstClientIdValue(),
                    "100 次递增后 firstClientIdValue 仍应为 42");
            assertEquals(142L, store.getClientId(),
                    "100 次递增后 clientId 应为 142");
        } finally {
            store.destroy();
        }
    }

    // =====================================================================
    // 测试 4: setClientId 验证
    // C++: m_data->data.store(value, std::memory_order_release);
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:73
    // =====================================================================

    @Test
    @Order(4)
    void test_setClientId() {
        long initialValue = 0L;

        ClientStore store = ClientStore.create(TEST_KEY_SET, initialValue);
        try {
            assertEquals(0L, store.getClientId(), "初始值应为 0");

            store.setClientId(999L);
            assertEquals(999L, store.getClientId(), "setClientId(999) 后应返回 999");

            long old = store.getClientIdAndIncrement();
            assertEquals(999L, old, "getClientIdAndIncrement 应返回 999");
            assertEquals(1000L, store.getClientId(), "递增后应为 1000");

            // firstClientIdValue 不受 setClientId 影响
            assertEquals(0L, store.getFirstClientIdValue(),
                    "setClientId 不应影响 firstClientIdValue");
        } finally {
            store.destroy();
        }
    }

    // =====================================================================
    // 测试 5: 初始值为 0 的 ClientStore
    // C++: LocklessShmClientStore::init(shmkey, flag, 0)  (默认 initialValue=0)
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:43
    // =====================================================================

    @Test
    @Order(5)
    void test_zeroInitialValue() {
        int key = 0x7F_B005;
        ClientStore store = ClientStore.create(key, 0L);
        try {
            assertEquals(0L, store.getClientId(), "初始值为 0");
            assertEquals(0L, store.getFirstClientIdValue(), "firstClientIdValue 应为 0");

            long old = store.getClientIdAndIncrement();
            assertEquals(0L, old, "fetch_add 应返回旧值 0");
            assertEquals(1L, store.getClientId(), "递增后应为 1");
        } finally {
            store.destroy();
        }
    }
}
