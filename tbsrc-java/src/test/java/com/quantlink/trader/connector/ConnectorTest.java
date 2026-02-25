package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Connector 单元测试。
 * <p>
 * 迁移自: hftbase/Connector/include/connector.h (illuminati::Connector)
 * <p>
 * 测试使用 createForTest 模式（创建新 SHM），测试完毕后 destroy() 清理。
 * 使用随机高位 SHM key (0x7Fxx_xxxx) 避免与运行中系统冲突。
 * <p>
 * 测试覆盖:
 * <ol>
 *   <li>OrderID 生成: clientId * ORDERID_RANGE + seq</li>
 *   <li>ORS 过滤: 仅回调属于本 clientId 的回报</li>
 *   <li>完整流程: MD -> callback -> sendNewOrder -> response -> callback</li>
 * </ol>
 */
@EnabledOnOs({OS.LINUX, OS.MAC})
@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
class ConnectorTest {

    // 使用随机高位 key 避免与其他测试和生产环境冲突
    // 每个测试使用不同的基础 key 以避免测试间干扰
    private static final int BASE_KEY = 0x7FB0_0000;

    private static int testKeyOffset = 0;

    /**
     * 为每个测试生成一组唯一的 SHM key。
     * 每个测试需要 4 个 key（md, req, resp, clientStore）。
     */
    private static Connector.Config newTestConfig() {
        int offset = testKeyOffset;
        testKeyOffset += 10; // 留足间距

        Connector.Config cfg = new Connector.Config();
        cfg.mdShmKey = BASE_KEY + offset + 1;
        cfg.mdQueueSize = 64;
        cfg.reqShmKey = BASE_KEY + offset + 2;
        cfg.reqQueueSize = 64;
        cfg.respShmKey = BASE_KEY + offset + 3;
        cfg.respQueueSize = 64;
        cfg.clientStoreShmKey = BASE_KEY + offset + 4;
        return cfg;
    }

    // =====================================================================
    // 测试 1: OrderID 生成
    // 验证: clientId=1 (ClientStore 初始值=1, getAndIncrement 返回 1),
    //       连续发单 -> OrderID 依次为 1_000_000, 1_000_001, 1_000_002
    //
    // C++: return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
    // Ref: hftbase/Connector/include/connector.h:366
    // =====================================================================

    @Test
    @Order(1)
    void test_orderID_generation_sequential() {
        Connector.Config cfg = newTestConfig();

        // 使用 no-op 回调（此测试不关心回调）
        Connector connector = Connector.createForTest(cfg,
                md -> {}, resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            int clientId = connector.getClientId();
            // ClientStore 初始值=1, getAndIncrement 返回 1
            assertEquals(1, clientId, "首个 clientId 应为 1");

            // 准备 RequestMsg
            MemorySegment req1 = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            MemorySegment req2 = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            MemorySegment req3 = arena.allocate(Types.REQUEST_MSG_LAYOUT);

            // 连续发 3 个新订单
            // C++: OrderID = clientId * ORDERID_RANGE + m_OrderCount++
            // Ref: hftbase/Connector/include/connector.h:366
            int orderId1 = connector.sendNewOrder(req1);
            int orderId2 = connector.sendNewOrder(req2);
            int orderId3 = connector.sendNewOrder(req3);

            // 验证 OrderID 序列: clientId(1) * 1_000_000 + 0, 1, 2
            assertEquals(1_000_000, orderId1, "第 1 个 OrderID 应为 clientId*ORDERID_RANGE + 0");
            assertEquals(1_000_001, orderId2, "第 2 个 OrderID 应为 clientId*ORDERID_RANGE + 1");
            assertEquals(1_000_002, orderId3, "第 3 个 OrderID 应为 clientId*ORDERID_RANGE + 2");

            // 验证 RequestType 被正确设置为 NEWORDER
            // C++: stratReq.Request_Type = illuminati::infra::NEWORDER;
            // Ref: hftbase/Connector/include/connector.h:274
            assertEquals(Constants.REQUEST_NEWORDER,
                    (int) Types.REQ_REQUEST_TYPE_VH.get(req1, 0L),
                    "Request_Type 应为 NEWORDER (0)");

            // 验证 OrderID 写入到 RequestMsg 中
            assertEquals(orderId1, (int) Types.REQ_ORDER_ID_VH.get(req1, 0L),
                    "req1 的 OrderID 字段应与返回值一致");
            assertEquals(orderId2, (int) Types.REQ_ORDER_ID_VH.get(req2, 0L),
                    "req2 的 OrderID 字段应与返回值一致");
            assertEquals(orderId3, (int) Types.REQ_ORDER_ID_VH.get(req3, 0L),
                    "req3 的 OrderID 字段应与返回值一致");

            // 验证 TimeStamp 被写入 (非零)
            // C++: msg.TimeStamp = illuminati::ITime_ClockRT::GetCurrentTime();
            // Ref: hftbase/Connector/include/connector.h:202
            long ts1 = (long) Types.REQ_TIMESTAMP_VH.get(req1, 0L);
            assertTrue(ts1 > 0, "TimeStamp 应为正值 (System.nanoTime)");
        } finally {
            connector.destroy();
        }
    }

    // =====================================================================
    // 测试 2: sendCancelOrder 和 sendModifyOrder
    // 验证: RequestType 正确设置，TimeStamp 写入
    //
    // C++: SendCancelOrder -> Request_Type = CANCELORDER
    // Ref: hftbase/Connector/include/connector.h:320
    // C++: SendModifyOrder -> Request_Type = MODIFYORDER
    // Ref: hftbase/Connector/include/connector.h:301
    // =====================================================================

    @Test
    @Order(2)
    void test_sendCancelOrder_and_sendModifyOrder() {
        Connector.Config cfg = newTestConfig();
        Connector connector = Connector.createForTest(cfg,
                md -> {}, resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            // 先发一个新订单获取 OrderID
            MemorySegment newReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            int orderId = connector.sendNewOrder(newReq);

            // 发送撤单
            MemorySegment cancelReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_ORDER_ID_VH.set(cancelReq, 0L, orderId);
            connector.sendCancelOrder(cancelReq);

            // 验证 cancelReq 的 Request_Type
            assertEquals(Constants.REQUEST_CANCELORDER,
                    (int) Types.REQ_REQUEST_TYPE_VH.get(cancelReq, 0L),
                    "撤单请求的 Request_Type 应为 CANCELORDER (2)");

            // 验证 TimeStamp 已写入
            long cancelTs = (long) Types.REQ_TIMESTAMP_VH.get(cancelReq, 0L);
            assertTrue(cancelTs > 0, "撤单请求的 TimeStamp 应为正值");

            // 发送改单
            MemorySegment modifyReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_ORDER_ID_VH.set(modifyReq, 0L, orderId);
            Types.REQ_PRICE_VH.set(modifyReq, 0L, 6800.0);
            connector.sendModifyOrder(modifyReq);

            // 验证 modifyReq 的 Request_Type
            assertEquals(Constants.REQUEST_MODIFYORDER,
                    (int) Types.REQ_REQUEST_TYPE_VH.get(modifyReq, 0L),
                    "改单请求的 Request_Type 应为 MODIFYORDER (1)");
        } finally {
            connector.destroy();
        }
    }

    // =====================================================================
    // 测试 3: ORS 回报过滤
    // 验证: 入队两条回报，一条 OrderID 属于本 clientId，一条不属于
    //       只有属于本 clientId 的回报触发回调
    //
    // C++: int32_t clientId = msg->OrderID / ORDERID_RANGE;
    //      if (m_all_clientIds[exchId][i] == clientId) { m_orscb(msg); }
    // Ref: hftbase/Connector/src/connector.cpp:822-830
    // =====================================================================

    @Test
    @Order(3)
    void test_orsFilter_onlyCallbackForOwnClientId() throws InterruptedException {
        Connector.Config cfg = newTestConfig();

        AtomicInteger callbackCount = new AtomicInteger(0);
        AtomicReference<Integer> receivedOrderId = new AtomicReference<>();
        CountDownLatch latch = new CountDownLatch(1);

        Connector connector = Connector.createForTest(cfg,
                md -> {},  // no-op MD callback
                resp -> {
                    // ORS 回调: 记录收到的 OrderID
                    int oid = (int) Types.RESP_ORDER_ID_VH.get(resp, 0L);
                    receivedOrderId.set(oid);
                    callbackCount.incrementAndGet();
                    latch.countDown();
                });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = connector.getClientId();
            // myClientId 应为 1

            // 启动轮询线程
            connector.start();

            // 等待轮询线程启动
            Thread.sleep(50);

            // 构造属于本 clientId 的回报
            // C++: OrderID = clientId * ORDERID_RANGE + seq
            // Ref: hftbase/Connector/include/connector.h:366
            int myOrderId = myClientId * Constants.ORDERID_RANGE + 1; // e.g. 1_000_001
            MemorySegment myResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(myResp, 0L, myOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(myResp, 0L, Constants.RESP_TRADE_CONFIRM);

            // 构造不属于本 clientId 的回报 (clientId=5)
            int otherOrderId = 5 * Constants.ORDERID_RANGE + 1; // 5_000_001
            MemorySegment otherResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(otherResp, 0L, otherOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(otherResp, 0L, Constants.RESP_TRADE_CONFIRM);

            // 先入队「不属于」本 clientId 的回报
            connector.enqueueResponse(otherResp);

            // 再入队「属于」本 clientId 的回报
            connector.enqueueResponse(myResp);

            // 等待回调触发
            assertTrue(latch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到回调");

            // 稍等以确保不会有额外的回调
            Thread.sleep(200);

            // 验证: 只有一条回报触发了回调
            assertEquals(1, callbackCount.get(),
                    "应仅收到 1 条回报回调（属于 clientId=" + myClientId + " 的）");
            assertEquals(myOrderId, receivedOrderId.get(),
                    "收到的 OrderID 应为 " + myOrderId);
        } finally {
            connector.destroy();
        }
    }

    // =====================================================================
    // 测试 4: 完整流程测试
    // 流程: enqueueMD -> mdCallback 触发 -> sendNewOrder -> enqueueResponse -> orsCallback 触发
    //
    // 模拟完整交易链:
    //   1. md_shm_feeder 写入行情 (enqueueMD)
    //   2. 策略收到行情 (mdCallback)
    //   3. 策略发送订单 (sendNewOrder)
    //   4. counter_bridge 回写成交回报 (enqueueResponse)
    //   5. 策略收到回报 (orsCallback)
    // =====================================================================

    @Test
    @Order(4)
    void test_fullFlow_md_to_order_to_response() throws InterruptedException {
        Connector.Config cfg = newTestConfig();

        CountDownLatch mdLatch = new CountDownLatch(1);
        CountDownLatch orsLatch = new CountDownLatch(1);
        AtomicReference<Integer> sentOrderId = new AtomicReference<>();
        AtomicReference<Integer> receivedOrderIdRef = new AtomicReference<>();
        AtomicReference<Double> receivedMdPrice = new AtomicReference<>();

        // 创建 Connector: mdCallback 中发单，orsCallback 中记录
        Connector connector = Connector.createForTest(cfg,
                md -> {
                    // Step 2: 收到行情 -> 提取价格
                    double price = md.get(ValueLayout.JAVA_DOUBLE,
                            Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET);
                    receivedMdPrice.set(price);
                    mdLatch.countDown();
                },
                resp -> {
                    // Step 5: 收到回报 -> 记录 OrderID
                    int oid = (int) Types.RESP_ORDER_ID_VH.get(resp, 0L);
                    receivedOrderIdRef.set(oid);
                    orsLatch.countDown();
                });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = connector.getClientId();

            // 启动轮询
            connector.start();
            Thread.sleep(50);

            // Step 1: 模拟 md_shm_feeder 写入行情
            MemorySegment md = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            // 设置价格: m_newPrice (在 MDDataPart 中, 绝对 offset = 96 + 0 = 96)
            md.set(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET, 6789.50);
            connector.enqueueMD(md);

            // 等待 mdCallback
            assertTrue(mdLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到行情回调");
            assertEquals(6789.50, receivedMdPrice.get(), 1e-10,
                    "行情价格应为 6789.50");

            // Step 3: 发送新订单 (在主线程中模拟策略发单)
            MemorySegment req = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_PRICE_VH.set(req, 0L, 6789.50);
            Types.REQ_QUANTITY_VH.set(req, 0L, 1);
            int orderId = connector.sendNewOrder(req);
            sentOrderId.set(orderId);

            // Step 4: 模拟 counter_bridge 回写成交回报
            MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(resp, 0L, orderId);
            Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_TRADE_CONFIRM);
            Types.RESP_QUANTITY_VH.set(resp, 0L, 1);
            Types.RESP_PRICE_VH.set(resp, 0L, 6789.50);
            connector.enqueueResponse(resp);

            // Step 5: 等待 orsCallback
            assertTrue(orsLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到回报回调");
            assertEquals(sentOrderId.get(), receivedOrderIdRef.get(),
                    "回报中的 OrderID 应与发送的一致");
        } finally {
            connector.destroy();
        }
    }

    // =====================================================================
    // 测试 5: clientId 自增测试
    // 验证: 连续创建两个 Connector 使用同一 ClientStore SHM，
    //       clientId 应自增 (1, 2)
    //
    // C++: m_clientId[exchCode] = m_shmMgr.getClientIdAndIncrement()
    // Ref: hftbase/Ipc/include/locklessshmclientstore.h:77-80
    // =====================================================================

    @Test
    @Order(5)
    void test_clientId_increment_across_connectors() {
        // 这个测试需要手动管理 SHM，因为两个 Connector 共享 clientStore
        int offset = testKeyOffset;
        testKeyOffset += 10;

        int mdKey1 = BASE_KEY + offset + 1;
        int reqKey1 = BASE_KEY + offset + 2;
        int respKey1 = BASE_KEY + offset + 3;
        int csKey = BASE_KEY + offset + 4;
        int mdKey2 = BASE_KEY + offset + 5;
        int reqKey2 = BASE_KEY + offset + 6;
        int respKey2 = BASE_KEY + offset + 7;

        // 手动创建 ClientStore
        ClientStore cs = ClientStore.create(csKey, 1L);

        // 创建第一个 Connector 的队列
        MWMRQueue mdQ1 = MWMRQueue.create(mdKey1, 16,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
        MWMRQueue reqQ1 = MWMRQueue.create(reqKey1, 16,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
        MWMRQueue respQ1 = MWMRQueue.create(respKey1, 16,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // 第一次 getClientIdAndIncrement: 返回 1, store 变为 2
        int clientId1 = (int) cs.getClientIdAndIncrement();

        // 创建第二个 Connector 的队列
        MWMRQueue mdQ2 = MWMRQueue.create(mdKey2, 16,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
        MWMRQueue reqQ2 = MWMRQueue.create(reqKey2, 16,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
        MWMRQueue respQ2 = MWMRQueue.create(respKey2, 16,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // 第二次 getClientIdAndIncrement: 返回 2, store 变为 3
        int clientId2 = (int) cs.getClientIdAndIncrement();

        try {
            assertEquals(1, clientId1, "第一个 clientId 应为 1");
            assertEquals(2, clientId2, "第二个 clientId 应为 2");

            // 验证 OrderID 范围不重叠
            // clientId1=1 -> OrderID 范围: [1_000_000, 1_999_999]
            // clientId2=2 -> OrderID 范围: [2_000_000, 2_999_999]
            assertNotEquals(clientId1 * Constants.ORDERID_RANGE,
                    clientId2 * Constants.ORDERID_RANGE,
                    "两个 Connector 的 OrderID 基础值不应重叠");
        } finally {
            // 清理所有 SHM
            mdQ1.destroy();
            reqQ1.destroy();
            respQ1.destroy();
            mdQ2.destroy();
            reqQ2.destroy();
            respQ2.destroy();
            cs.destroy();
        }
    }

    // =====================================================================
    // 测试 6: 多次 ORS 回报 -- 本 clientId 的多条回报全部回调
    // 验证: 3 条属于本 clientId 的回报全部触发回调
    // =====================================================================

    @Test
    @Order(6)
    void test_orsCallback_multipleResponses_allReceived() throws InterruptedException {
        Connector.Config cfg = newTestConfig();

        int expectedCount = 3;
        CountDownLatch latch = new CountDownLatch(expectedCount);
        AtomicInteger callbackCount = new AtomicInteger(0);

        Connector connector = Connector.createForTest(cfg,
                md -> {},
                resp -> {
                    callbackCount.incrementAndGet();
                    latch.countDown();
                });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = connector.getClientId();
            connector.start();
            Thread.sleep(50);

            // 入队 3 条属于本 clientId 的回报
            for (int i = 0; i < expectedCount; i++) {
                MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
                int orderId = myClientId * Constants.ORDERID_RANGE + i;
                Types.RESP_ORDER_ID_VH.set(resp, 0L, orderId);
                Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_NEW_ORDER_CONFIRM);
                connector.enqueueResponse(resp);
            }

            assertTrue(latch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到所有 " + expectedCount + " 条回调");
            assertEquals(expectedCount, callbackCount.get(),
                    "回调次数应为 " + expectedCount);
        } finally {
            connector.destroy();
        }
    }

    // =====================================================================
    // 测试 7: start/stop 生命周期
    // 验证: stop 后轮询线程退出，不再触发回调
    // =====================================================================

    @Test
    @Order(7)
    void test_startStop_lifecycle() throws InterruptedException {
        Connector.Config cfg = newTestConfig();

        AtomicInteger mdCount = new AtomicInteger(0);

        Connector connector = Connector.createForTest(cfg,
                md -> mdCount.incrementAndGet(),
                resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            // start
            connector.start();
            Thread.sleep(50);

            // 入队一条行情
            MemorySegment md = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            connector.enqueueMD(md);
            Thread.sleep(200);
            assertEquals(1, mdCount.get(), "应收到 1 条行情回调");

            // stop
            connector.stop();
            Thread.sleep(100);

            // stop 后入队的行情不应触发回调
            int countBefore = mdCount.get();
            connector.enqueueMD(md);
            Thread.sleep(200);
            assertEquals(countBefore, mdCount.get(),
                    "stop 后不应再收到行情回调");
        } finally {
            connector.destroy();
        }
    }
}
