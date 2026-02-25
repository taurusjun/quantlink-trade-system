package com.quantlink.trader.shm;

import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * MWMRQueue 单元测试。
 * <p>
 * 迁移自: hftbase/Ipc/include/multiwritermultireadershmqueue.h
 * <p>
 * 测试需要运行在 Linux 或 macOS 上（SysV SHM 可用的环境）。
 * 使用高位 key (0x7F_xxxx) 避免与运行中的 QuantLink 系统冲突。
 * <p>
 * C++ 对应测试数据:
 * - MarketUpdateNew: dataSize=816, elemSize=824  (Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE)
 * - RequestMsg:      dataSize=256, elemSize=320  (Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE)
 * - ResponseMsg:     dataSize=176, elemSize=184  (Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE)
 */
@EnabledOnOs({OS.LINUX, OS.MAC})
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class MWMRQueueTest {

    // 使用高位 key 避免冲突 (QuantLink 使用 0x1001, 0x2001, 0x3001, 0x4001)
    private static final int TEST_KEY_MD = 0x7F_A001;
    private static final int TEST_KEY_REQ = 0x7F_A002;
    private static final int TEST_KEY_CONCURRENT = 0x7F_A003;
    private static final int TEST_KEY_WRAPAROUND = 0x7F_A004;
    private static final int TEST_KEY_RESP = 0x7F_A005;

    // C++: 队列大小
    private static final int QUEUE_SIZE = 1024;

    // =====================================================================
    // 测试 1: 单线程 MarketUpdateNew 入队/出队
    // 验证: 使用 MarketUpdateNew 大小 (dataSize=816, elemSize=824) 的基本入队出队
    // =====================================================================

    @Test
    @Order(1)
    void test_singleThread_enqueueDequeue_marketUpdateNew() {
        long dataSize = Types.MARKET_UPDATE_NEW_SIZE;   // 816
        long elemSize = Types.QUEUE_ELEM_MD_SIZE;       // 824

        MWMRQueue queue = MWMRQueue.create(TEST_KEY_MD, QUEUE_SIZE, dataSize, elemSize);
        try (Arena arena = Arena.ofConfined()) {
            // 验证初始状态
            // C++: head 初始化为 1
            // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:21
            assertEquals(1, queue.getHead(), "head 应初始化为 1");
            assertEquals(1, queue.getLocalTail(), "localTail 应初始化为 1");
            assertTrue(queue.isEmpty(), "新队列应为空");

            // 准备测试数据: 模拟 MarketUpdateNew
            MemorySegment data = arena.allocate(dataSize);
            // 写入 MDHeaderPart.exchTS (offset 0) = 1234567890
            data.set(ValueLayout.JAVA_LONG, Types.MDH_EXCH_TS_OFFSET, 1234567890L);
            // 写入 MDDataPart.newPrice (offset 96) = 6789.50
            data.set(ValueLayout.JAVA_DOUBLE, Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET, 6789.50);
            // 写入 MDDataPart.lastTradedPrice (offset 96+16) = 6790.00
            data.set(ValueLayout.JAVA_DOUBLE, Types.MU_DATA_OFFSET + Types.MDD_LAST_TRADED_PRICE_OFFSET, 6790.00);

            // 入队
            queue.enqueue(data);

            // 验证 head 前进
            assertEquals(2, queue.getHead(), "入队后 head 应为 2");
            assertFalse(queue.isEmpty(), "入队后队列不应为空");

            // 出队
            MemorySegment out = arena.allocate(dataSize);
            assertTrue(queue.dequeue(out), "出队应成功");

            // 验证数据
            assertEquals(1234567890L, out.get(ValueLayout.JAVA_LONG, Types.MDH_EXCH_TS_OFFSET),
                    "exchTS 应与入队值一致");
            assertEquals(6789.50, out.get(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET), 1e-10,
                    "newPrice 应与入队值一致");
            assertEquals(6790.00, out.get(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_LAST_TRADED_PRICE_OFFSET), 1e-10,
                    "lastTradedPrice 应与入队值一致");

            // 出队后队列应为空
            assertTrue(queue.isEmpty(), "出队后队列应为空");
            assertFalse(queue.dequeue(out), "空队列出队应返回 false");
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 2: 单线程 RequestMsg 入队/出队 (elemSize=320, dataSize=256)
    // 验证: RequestMsg 的 __attribute__((aligned(64))) padding 处理
    // C++: sizeof(QueueElem<RequestMsg>) = 320 (不是 264)
    // =====================================================================

    @Test
    @Order(2)
    void test_singleThread_enqueueDequeue_requestMsg() {
        long dataSize = Types.REQUEST_MSG_SIZE;          // 256
        long elemSize = Types.QUEUE_ELEM_REQ_SIZE;       // 320

        MWMRQueue queue = MWMRQueue.create(TEST_KEY_REQ, QUEUE_SIZE, dataSize, elemSize);
        try (Arena arena = Arena.ofConfined()) {
            // 准备测试数据: 模拟 RequestMsg
            MemorySegment data = arena.allocate(dataSize);
            // 写入 RequestType (offset 96) = NEWORDER (0)
            data.set(ValueLayout.JAVA_INT, Types.REQ_REQUEST_TYPE_OFFSET, Constants.REQUEST_NEWORDER);
            // 写入 OrderID (offset 116) = 92001001
            data.set(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET, 92001001);
            // 写入 Price (offset 136) = 6800.0
            data.set(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET, 6800.0);
            // 写入 Quantity (offset 124) = 5
            data.set(ValueLayout.JAVA_INT, Types.REQ_QUANTITY_OFFSET, 5);
            // 写入 StrategyID (offset 220) = 92201
            data.set(ValueLayout.JAVA_INT, Types.REQ_STRATEGY_ID_OFFSET, 92201);

            // 入队 3 条消息
            queue.enqueue(data);
            data.set(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET, 92001002);
            data.set(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET, 6801.0);
            queue.enqueue(data);
            data.set(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET, 92001003);
            data.set(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET, 6802.0);
            queue.enqueue(data);

            assertEquals(4, queue.getHead(), "3 次入队后 head 应为 4");

            // 出队并验证顺序
            MemorySegment out = arena.allocate(dataSize);

            assertTrue(queue.dequeue(out), "第 1 次出队应成功");
            assertEquals(92001001, out.get(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET));
            assertEquals(6800.0, out.get(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET), 1e-10);

            assertTrue(queue.dequeue(out), "第 2 次出队应成功");
            assertEquals(92001002, out.get(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET));
            assertEquals(6801.0, out.get(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET), 1e-10);

            assertTrue(queue.dequeue(out), "第 3 次出队应成功");
            assertEquals(92001003, out.get(ValueLayout.JAVA_INT, Types.REQ_ORDER_ID_OFFSET));
            assertEquals(6802.0, out.get(ValueLayout.JAVA_DOUBLE, Types.REQ_PRICE_OFFSET), 1e-10);

            assertFalse(queue.dequeue(out), "队列应为空");
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 3: 多线程并发入队 + 单线程出队，验证无数据丢失
    // C++ MWMR 的核心特性: 多个 writer 并发 fetch_add 安全入队
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:121
    // =====================================================================

    @Test
    @Order(3)
    void test_concurrentEnqueue_singleDequeue_noDataLoss() throws InterruptedException {
        long dataSize = Types.RESPONSE_MSG_SIZE;         // 176
        long elemSize = Types.QUEUE_ELEM_RESP_SIZE;      // 184

        int numWriters = 4;
        int msgsPerWriter = 250;
        int totalMsgs = numWriters * msgsPerWriter;

        MWMRQueue queue = MWMRQueue.create(TEST_KEY_CONCURRENT, QUEUE_SIZE, dataSize, elemSize);
        try (Arena arena = Arena.ofShared()) {
            CountDownLatch startLatch = new CountDownLatch(1);
            CountDownLatch doneLatch = new CountDownLatch(numWriters);
            AtomicInteger errors = new AtomicInteger(0);

            // 启动多个 writer 线程
            for (int w = 0; w < numWriters; w++) {
                final int writerId = w;
                Thread writerThread = new Thread(() -> {
                    try {
                        startLatch.await();
                        for (int i = 0; i < msgsPerWriter; i++) {
                            MemorySegment msg = arena.allocate(dataSize);
                            // 用 OrderID 编码 writerId 和序号: writerId * 1000 + i
                            int orderId = writerId * 1000 + i;
                            msg.set(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET, orderId);
                            // 用 Quantity 字段写入校验值
                            msg.set(ValueLayout.JAVA_INT, Types.RESP_QUANTITY_OFFSET, orderId * 7);
                            queue.enqueue(msg);
                        }
                    } catch (Exception e) {
                        errors.incrementAndGet();
                    } finally {
                        doneLatch.countDown();
                    }
                });
                writerThread.setDaemon(true);
                writerThread.start();
            }

            // 释放所有 writer 线程
            startLatch.countDown();
            doneLatch.await();

            assertEquals(0, errors.get(), "writer 线程不应有错误");

            // 单线程出队，收集所有 OrderID
            Set<Integer> receivedIds = ConcurrentHashMap.newKeySet();
            MemorySegment out = arena.allocate(dataSize);
            int dequeueCount = 0;

            // 尝试出队所有消息（可能需要多轮，因为 seqNo 写入有延迟）
            int maxRetries = totalMsgs * 100;
            int retries = 0;
            while (dequeueCount < totalMsgs && retries < maxRetries) {
                if (queue.dequeue(out)) {
                    int orderId = out.get(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET);
                    int quantity = out.get(ValueLayout.JAVA_INT, Types.RESP_QUANTITY_OFFSET);
                    // 验证数据完整性
                    assertEquals(orderId * 7, quantity,
                            "OrderID=" + orderId + " 的 Quantity 校验失败");
                    receivedIds.add(orderId);
                    dequeueCount++;
                } else {
                    retries++;
                    Thread.onSpinWait();
                }
            }

            assertEquals(totalMsgs, dequeueCount,
                    "应收到 " + totalMsgs + " 条消息，实际收到 " + dequeueCount);
            assertEquals(totalMsgs, receivedIds.size(),
                    "应有 " + totalMsgs + " 个不同的 OrderID");

            // 验证每个 writer 的所有消息都到达
            for (int w = 0; w < numWriters; w++) {
                for (int i = 0; i < msgsPerWriter; i++) {
                    int expectedId = w * 1000 + i;
                    assertTrue(receivedIds.contains(expectedId),
                            "缺少 writer=" + w + " msg=" + i + " (OrderID=" + expectedId + ")");
                }
            }
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 4: 空队列出队返回 false
    // C++: isEmpty() 检查 seqNo < tail
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:248
    // =====================================================================

    @Test
    @Order(4)
    void test_emptyQueue_dequeueFalse() {
        long dataSize = Types.MARKET_UPDATE_NEW_SIZE;
        long elemSize = Types.QUEUE_ELEM_MD_SIZE;
        int key = 0x7F_A006;

        MWMRQueue queue = MWMRQueue.create(key, 64, dataSize, elemSize);
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment out = arena.allocate(dataSize);

            // 新创建的队列应为空
            assertTrue(queue.isEmpty(), "新队列应为空");
            assertFalse(queue.dequeue(out), "空队列出队应返回 false");

            // 入队一条再出队，之后再尝试出队
            MemorySegment data = arena.allocate(dataSize);
            data.set(ValueLayout.JAVA_LONG, 0, 42L);
            queue.enqueue(data);
            assertFalse(queue.isEmpty(), "入队后不应为空");
            assertTrue(queue.dequeue(out), "有数据时出队应成功");
            assertTrue(queue.isEmpty(), "出队后应为空");
            assertFalse(queue.dequeue(out), "再次出队应返回 false");
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 5: 队列环绕测试 -- 写满一圈后继续写入
    // 验证: head & mask 实现环形缓冲区
    // C++: QueueElem<T> *slot = ShmStore::m_updates + (myHead & (ShmStore::m_size - 1));
    // Ref: hftbase/Ipc/include/multiwritermultireadershmqueue.h:123
    // =====================================================================

    @Test
    @Order(5)
    void test_queueWraparound_fullCycleAndBeyond() {
        long dataSize = Types.RESPONSE_MSG_SIZE;   // 176 (小消息，便于测试)
        long elemSize = Types.QUEUE_ELEM_RESP_SIZE; // 184
        int smallQueueSize = 16;  // 小队列，易于触发环绕

        MWMRQueue queue = MWMRQueue.create(TEST_KEY_WRAPAROUND, smallQueueSize, dataSize, elemSize);
        try (Arena arena = Arena.ofConfined()) {
            // 验证队列大小 (16 已经是 2 的幂)
            assertEquals(16, queue.getSize(), "队列大小应为 16");

            MemorySegment data = arena.allocate(dataSize);
            MemorySegment out = arena.allocate(dataSize);

            // 写入 32 条消息（2 倍队列大小），每条写完后立即读出
            for (int i = 0; i < 32; i++) {
                data.set(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET, 10000 + i);
                data.set(ValueLayout.JAVA_DOUBLE, Types.RESP_PRICE_OFFSET, 5000.0 + i);
                queue.enqueue(data);

                assertTrue(queue.dequeue(out),
                        "第 " + i + " 次出队应成功（环绕测试）");
                assertEquals(10000 + i, out.get(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET),
                        "第 " + i + " 次 OrderID 应正确");
                assertEquals(5000.0 + i, out.get(ValueLayout.JAVA_DOUBLE, Types.RESP_PRICE_OFFSET), 1e-10,
                        "第 " + i + " 次 Price 应正确");
            }

            // head 应该从 1 前进到 33 (1 + 32)
            assertEquals(33, queue.getHead(), "32 次入队后 head 应为 33");
            // localTail 也应该前进到 33
            assertEquals(33, queue.getLocalTail(), "32 次出队后 localTail 应为 33");
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 6: ResponseMsg 入队/出队验证
    // 验证: ResponseMsg (dataSize=176, elemSize=184) 的字段正确性
    // =====================================================================

    @Test
    @Order(6)
    void test_singleThread_enqueueDequeue_responseMsg() {
        long dataSize = Types.RESPONSE_MSG_SIZE;         // 176
        long elemSize = Types.QUEUE_ELEM_RESP_SIZE;      // 184

        MWMRQueue queue = MWMRQueue.create(TEST_KEY_RESP, QUEUE_SIZE, dataSize, elemSize);
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment data = arena.allocate(dataSize);

            // 写入 ResponseMsg 字段
            data.set(ValueLayout.JAVA_INT, Types.RESP_RESPONSE_TYPE_OFFSET, Constants.RESP_TRADE_CONFIRM);
            data.set(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET, 92001042);
            data.set(ValueLayout.JAVA_INT, Types.RESP_QUANTITY_OFFSET, 3);
            data.set(ValueLayout.JAVA_DOUBLE, Types.RESP_PRICE_OFFSET, 6850.0);
            data.set(ValueLayout.JAVA_LONG, Types.RESP_TIMESTAMP_OFFSET, System.nanoTime());
            data.set(ValueLayout.JAVA_INT, Types.RESP_STRATEGY_ID_OFFSET, 92201);

            queue.enqueue(data);

            MemorySegment out = arena.allocate(dataSize);
            assertTrue(queue.dequeue(out), "出队应成功");

            assertEquals(Constants.RESP_TRADE_CONFIRM,
                    out.get(ValueLayout.JAVA_INT, Types.RESP_RESPONSE_TYPE_OFFSET),
                    "ResponseType 应为 TRADE_CONFIRM");
            assertEquals(92001042, out.get(ValueLayout.JAVA_INT, Types.RESP_ORDER_ID_OFFSET),
                    "OrderID 应正确");
            assertEquals(3, out.get(ValueLayout.JAVA_INT, Types.RESP_QUANTITY_OFFSET),
                    "Quantity 应正确");
            assertEquals(6850.0, out.get(ValueLayout.JAVA_DOUBLE, Types.RESP_PRICE_OFFSET), 1e-10,
                    "Price 应正确");
            assertEquals(92201, out.get(ValueLayout.JAVA_INT, Types.RESP_STRATEGY_ID_OFFSET),
                    "StrategyID 应正确");
        } finally {
            queue.destroy();
        }
    }

    // =====================================================================
    // 测试 7: nextPowerOf2 验证
    // 对照 Go 实现: tbsrc-golang/pkg/shm/mwmr_queue_test.go:187-210
    // =====================================================================

    @Test
    @Order(7)
    void test_nextPowerOf2() {
        // 与 Go 测试数据一致
        // Ref: tbsrc-golang/pkg/shm/mwmr_queue_test.go:188-203
        assertEquals(1, MWMRQueue.nextPowerOf2(0), "nextPowerOf2(0)");
        assertEquals(1, MWMRQueue.nextPowerOf2(1), "nextPowerOf2(1)");
        assertEquals(2, MWMRQueue.nextPowerOf2(2), "nextPowerOf2(2)");
        assertEquals(4, MWMRQueue.nextPowerOf2(3), "nextPowerOf2(3)");
        assertEquals(4, MWMRQueue.nextPowerOf2(4), "nextPowerOf2(4)");
        assertEquals(8, MWMRQueue.nextPowerOf2(5), "nextPowerOf2(5)");
        assertEquals(8, MWMRQueue.nextPowerOf2(7), "nextPowerOf2(7)");
        assertEquals(8, MWMRQueue.nextPowerOf2(8), "nextPowerOf2(8)");
        assertEquals(16, MWMRQueue.nextPowerOf2(9), "nextPowerOf2(9)");
        assertEquals(128, MWMRQueue.nextPowerOf2(100), "nextPowerOf2(100)");
        assertEquals(1024, MWMRQueue.nextPowerOf2(1024), "nextPowerOf2(1024)");
        assertEquals(2048, MWMRQueue.nextPowerOf2(1025), "nextPowerOf2(1025)");

        // 额外测试: 负值
        assertEquals(1, MWMRQueue.nextPowerOf2(-1), "nextPowerOf2(-1)");
        assertEquals(1, MWMRQueue.nextPowerOf2(-100), "nextPowerOf2(-100)");
    }

    // =====================================================================
    // 测试 8: 队列属性验证
    // =====================================================================

    @Test
    @Order(8)
    void test_queueProperties() {
        int key = 0x7F_A007;
        long dataSize = Types.REQUEST_MSG_SIZE;          // 256
        long elemSize = Types.QUEUE_ELEM_REQ_SIZE;       // 320

        MWMRQueue queue = MWMRQueue.create(key, 100, dataSize, elemSize);
        try {
            // 100 -> nextPowerOf2 -> 128
            assertEquals(128, queue.getSize(), "100 应向上取整到 128");
            assertEquals(dataSize, queue.getDataSize(), "dataSize 应为 256");
            assertEquals(elemSize, queue.getElemSize(), "elemSize 应为 320");
        } finally {
            queue.destroy();
        }
    }
}
