package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.ValueLayout;
import java.util.List;
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
 * 测试使用 ConnectorTestHelper.createForTest 预创建 SHM 段，
 * 然后 Connector 正常构造函数 attach（与 C++ 测试流程一致：外部创建 SHM → Connector attach）。
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

        Connector.ExchangeConfig exchCfg = new Connector.ExchangeConfig();
        exchCfg.exchangeName = "CHINA_SHFE";
        exchCfg.mdShmKeys.add(BASE_KEY + offset + 1);
        exchCfg.mdShmSizes.add(64);
        exchCfg.mdShmReadModes.add(Connector.MD_READ_ROUND_ROBIN);
        exchCfg.reqShmKey = BASE_KEY + offset + 2;
        exchCfg.reqQueueSize = 64;
        exchCfg.respShmKey = BASE_KEY + offset + 3;
        exchCfg.respQueueSize = 64;
        exchCfg.clientStoreShmKey = BASE_KEY + offset + 4;
        cfg.exchanges.add(exchCfg);

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

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg, md -> {}, resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            int clientId = bundle.getClientId();
            // ClientStore 初始值=1, getAndIncrement 返回 1
            assertEquals(1, clientId, "首个 clientId 应为 1");

            // 准备 RequestMsg — 必须设置 Exchange_Type 以路由到正确的请求队列
            MemorySegment req1 = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            MemorySegment req2 = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            MemorySegment req3 = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(req1, 0L, (byte) Constants.CHINA_SHFE);
            Types.REQ_EXCHANGE_TYPE_VH.set(req2, 0L, (byte) Constants.CHINA_SHFE);
            Types.REQ_EXCHANGE_TYPE_VH.set(req3, 0L, (byte) Constants.CHINA_SHFE);

            // 连续发 3 个新订单
            // C++: OrderID = clientId * ORDERID_RANGE + m_OrderCount++
            // Ref: hftbase/Connector/include/connector.h:366
            int orderId1 = bundle.connector.sendNewOrder(req1);
            int orderId2 = bundle.connector.sendNewOrder(req2);
            int orderId3 = bundle.connector.sendNewOrder(req3);

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
            assertTrue(ts1 > 0, "TimeStamp 应为正值");
        } finally {
            bundle.destroy();
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
        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg, md -> {}, resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            // 先发一个新订单获取 OrderID
            MemorySegment newReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(newReq, 0L, (byte) Constants.CHINA_SHFE);
            int orderId = bundle.connector.sendNewOrder(newReq);

            // 发送撤单
            MemorySegment cancelReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(cancelReq, 0L, (byte) Constants.CHINA_SHFE);
            Types.REQ_ORDER_ID_VH.set(cancelReq, 0L, orderId);
            bundle.connector.sendCancelOrder(cancelReq);

            // 验证 cancelReq 的 Request_Type
            assertEquals(Constants.REQUEST_CANCELORDER,
                    (int) Types.REQ_REQUEST_TYPE_VH.get(cancelReq, 0L),
                    "撤单请求的 Request_Type 应为 CANCELORDER (2)");

            // 验证 TimeStamp 已写入
            long cancelTs = (long) Types.REQ_TIMESTAMP_VH.get(cancelReq, 0L);
            assertTrue(cancelTs > 0, "撤单请求的 TimeStamp 应为正值");

            // 发送改单
            MemorySegment modifyReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(modifyReq, 0L, (byte) Constants.CHINA_SHFE);
            Types.REQ_ORDER_ID_VH.set(modifyReq, 0L, orderId);
            Types.REQ_PRICE_VH.set(modifyReq, 0L, 6800.0);
            bundle.connector.sendModifyOrder(modifyReq);

            // 验证 modifyReq 的 Request_Type
            assertEquals(Constants.REQUEST_MODIFYORDER,
                    (int) Types.REQ_REQUEST_TYPE_VH.get(modifyReq, 0L),
                    "改单请求的 Request_Type 应为 MODIFYORDER (1)");
        } finally {
            bundle.destroy();
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

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> {},
                        resp -> {
                            int oid = (int) Types.RESP_ORDER_ID_VH.get(resp, 0L);
                            receivedOrderId.set(oid);
                            callbackCount.incrementAndGet();
                            latch.countDown();
                        });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = bundle.getClientId();

            // 启动轮询线程
            bundle.startAsync();
            Thread.sleep(50);

            // 构造属于本 clientId 的回报
            int myOrderId = myClientId * Constants.ORDERID_RANGE + 1;
            MemorySegment myResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(myResp, 0L, myOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(myResp, 0L, Constants.RESP_TRADE_CONFIRM);

            // 构造不属于本 clientId 的回报 (clientId=5)
            int otherOrderId = 5 * Constants.ORDERID_RANGE + 1;
            MemorySegment otherResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(otherResp, 0L, otherOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(otherResp, 0L, Constants.RESP_TRADE_CONFIRM);

            // 先入队「不属于」本 clientId 的回报
            bundle.enqueueResponse(otherResp);

            // 再入队「属于」本 clientId 的回报
            bundle.enqueueResponse(myResp);

            // 等待回调触发
            assertTrue(latch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到回调");

            // 稍等以确保不会有额外的回调
            Thread.sleep(200);

            assertEquals(1, callbackCount.get(),
                    "应仅收到 1 条回报回调（属于 clientId=" + myClientId + " 的）");
            assertEquals(myOrderId, receivedOrderId.get(),
                    "收到的 OrderID 应为 " + myOrderId);
        } finally {
            bundle.destroy();
        }
    }

    // =====================================================================
    // 测试 4: 完整流程测试
    // 流程: enqueueMD -> mdCallback 触发 -> sendNewOrder -> enqueueResponse -> orsCallback 触发
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

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> {
                            double price = md.get(ValueLayout.JAVA_DOUBLE,
                                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET);
                            receivedMdPrice.set(price);
                            mdLatch.countDown();
                        },
                        resp -> {
                            int oid = (int) Types.RESP_ORDER_ID_VH.get(resp, 0L);
                            receivedOrderIdRef.set(oid);
                            orsLatch.countDown();
                        });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = bundle.getClientId();

            bundle.startAsync();
            Thread.sleep(50);

            // Step 1: 模拟 md_shm_feeder 写入行情
            MemorySegment md = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            md.set(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET, 6789.50);
            bundle.enqueueMD(md);

            assertTrue(mdLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到行情回调");
            assertEquals(6789.50, receivedMdPrice.get(), 1e-10,
                    "行情价格应为 6789.50");

            // Step 3: 发送新订单
            MemorySegment req = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(req, 0L, (byte) Constants.CHINA_SHFE);
            Types.REQ_PRICE_VH.set(req, 0L, 6789.50);
            Types.REQ_QUANTITY_VH.set(req, 0L, 1);
            int orderId = bundle.connector.sendNewOrder(req);
            sentOrderId.set(orderId);

            // Step 4: 模拟 counter_bridge 回写成交回报
            MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(resp, 0L, orderId);
            Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_TRADE_CONFIRM);
            Types.RESP_QUANTITY_VH.set(resp, 0L, 1);
            Types.RESP_PRICE_VH.set(resp, 0L, 6789.50);
            bundle.enqueueResponse(resp);

            assertTrue(orsLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到回报回调");
            assertEquals(sentOrderId.get(), receivedOrderIdRef.get(),
                    "回报中的 OrderID 应与发送的一致");
        } finally {
            bundle.destroy();
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

        // 创建 SHM 队列
        MWMRQueue mdQ1 = MWMRQueue.create(mdKey1, 16,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
        MWMRQueue reqQ1 = MWMRQueue.create(reqKey1, 16,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
        MWMRQueue respQ1 = MWMRQueue.create(respKey1, 16,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        int clientId1 = (int) cs.getClientIdAndIncrement();

        MWMRQueue mdQ2 = MWMRQueue.create(mdKey2, 16,
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
        MWMRQueue reqQ2 = MWMRQueue.create(reqKey2, 16,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
        MWMRQueue respQ2 = MWMRQueue.create(respKey2, 16,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        int clientId2 = (int) cs.getClientIdAndIncrement();

        try {
            assertEquals(1, clientId1, "第一个 clientId 应为 1");
            assertEquals(2, clientId2, "第二个 clientId 应为 2");

            assertNotEquals(clientId1 * Constants.ORDERID_RANGE,
                    clientId2 * Constants.ORDERID_RANGE,
                    "两个 Connector 的 OrderID 基础值不应重叠");
        } finally {
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
    // =====================================================================

    @Test
    @Order(6)
    void test_orsCallback_multipleResponses_allReceived() throws InterruptedException {
        Connector.Config cfg = newTestConfig();

        int expectedCount = 3;
        CountDownLatch latch = new CountDownLatch(expectedCount);
        AtomicInteger callbackCount = new AtomicInteger(0);

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> {},
                        resp -> {
                            callbackCount.incrementAndGet();
                            latch.countDown();
                        });

        try (Arena arena = Arena.ofConfined()) {
            int myClientId = bundle.getClientId();
            bundle.startAsync();
            Thread.sleep(50);

            for (int i = 0; i < expectedCount; i++) {
                MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
                int orderId = myClientId * Constants.ORDERID_RANGE + i;
                Types.RESP_ORDER_ID_VH.set(resp, 0L, orderId);
                Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_NEW_ORDER_CONFIRM);
                bundle.enqueueResponse(resp);
            }

            assertTrue(latch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到所有 " + expectedCount + " 条回调");
            assertEquals(expectedCount, callbackCount.get(),
                    "回调次数应为 " + expectedCount);
        } finally {
            bundle.destroy();
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

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> mdCount.incrementAndGet(),
                        resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            bundle.startAsync();
            Thread.sleep(50);

            // 入队一条行情
            MemorySegment md = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            bundle.enqueueMD(md);
            Thread.sleep(200);
            assertEquals(1, mdCount.get(), "应收到 1 条行情回调");

            // stop
            bundle.stop();
            Thread.sleep(100);

            // stop 后入队的行情不应触发回调
            int countBefore = mdCount.get();
            bundle.enqueueMD(md);
            Thread.sleep(200);
            assertEquals(countBefore, mdCount.get(),
                    "stop 后不应再收到行情回调");
        } finally {
            bundle.destroy();
        }
    }

    // =====================================================================
    // 多交易所辅助方法
    // =====================================================================

    /**
     * 创建含 CHINA_SHFE(57) + CHINA_CFFEX(58) 的双交易所配置。
     * 各交易所使用独立 SHM key。
     */
    private static Connector.Config newDualExchangeConfig() {
        int offset = testKeyOffset;
        testKeyOffset += 20; // 双交易所需要更多 key 间距

        Connector.Config cfg = new Connector.Config();

        // 交易所 0: CHINA_SHFE
        Connector.ExchangeConfig shfe = new Connector.ExchangeConfig();
        shfe.exchangeName = "CHINA_SHFE";
        shfe.mdShmKeys.add(BASE_KEY + offset + 1);
        shfe.mdShmSizes.add(64);
        shfe.mdShmReadModes.add(Connector.MD_READ_ROUND_ROBIN);
        shfe.reqShmKey = BASE_KEY + offset + 2;
        shfe.reqQueueSize = 64;
        shfe.respShmKey = BASE_KEY + offset + 3;
        shfe.respQueueSize = 64;
        shfe.clientStoreShmKey = BASE_KEY + offset + 4;
        cfg.exchanges.add(shfe);

        // 交易所 1: CHINA_CFFEX
        Connector.ExchangeConfig cffex = new Connector.ExchangeConfig();
        cffex.exchangeName = "CHINA_CFFEX";
        cffex.mdShmKeys.add(BASE_KEY + offset + 11);
        cffex.mdShmSizes.add(64);
        cffex.mdShmReadModes.add(Connector.MD_READ_ROUND_ROBIN);
        cffex.reqShmKey = BASE_KEY + offset + 12;
        cffex.reqQueueSize = 64;
        cffex.respShmKey = BASE_KEY + offset + 13;
        cffex.respQueueSize = 64;
        cffex.clientStoreShmKey = BASE_KEY + offset + 14;
        cfg.exchanges.add(cffex);

        return cfg;
    }

    // =====================================================================
    // 测试 8: 多交易所 clientId 独立分配
    // 验证: 各交易所独立 ClientStore → 各自 clientId=1
    //       未注册交易所返回 0
    //
    // C++: m_clientId[exchCode] 按交易所独立分配
    // Ref: hftbase/Connector/src/connector.cpp:94-106
    // =====================================================================

    @Test
    @Order(8)
    void test_multiExchange_clientId_allocation() {
        Connector.Config cfg = newDualExchangeConfig();

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg, md -> {}, resp -> {});

        try {
            // 各交易所独立 ClientStore，各自分配 clientId=1
            int shfeClientId = bundle.getClientId(Constants.CHINA_SHFE);
            int cffexClientId = bundle.getClientId(Constants.CHINA_CFFEX);

            assertEquals(1, shfeClientId,
                    "SHFE clientId 应为 1（独立 ClientStore）");
            assertEquals(1, cffexClientId,
                    "CFFEX clientId 应为 1（独立 ClientStore）");

            // 未注册交易所返回 0
            int unknownClientId = bundle.getClientId(Constants.CHINA_DCE);
            assertEquals(0, unknownClientId,
                    "未注册交易所 DCE 应返回 clientId=0");
        } finally {
            bundle.destroy();
        }
    }

    // =====================================================================
    // 测试 9: 多交易所订单路由
    // 验证: Exchange_Type=SHFE 的订单 → SHFE request queue
    //       Exchange_Type=CFFEX 的订单 → CFFEX request queue
    //
    // C++: m_requestQueue[msg.Exchange_Type]->enqueue(msg)
    // Ref: hftbase/Connector/include/connector.h:200-220
    // =====================================================================

    @Test
    @Order(9)
    void test_multiExchange_orderRouting() {
        Connector.Config cfg = newDualExchangeConfig();

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg, md -> {}, resp -> {});

        try (Arena arena = Arena.ofConfined()) {
            // 发送 SHFE 订单
            MemorySegment shfeReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(shfeReq, 0L, (byte) Constants.CHINA_SHFE);
            int shfeOrderId = bundle.connector.sendNewOrder(shfeReq);
            assertTrue(shfeOrderId > 0, "SHFE 订单 OrderID 应为正值");

            // 发送 CFFEX 订单
            MemorySegment cffexReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(cffexReq, 0L, (byte) Constants.CHINA_CFFEX);
            int cffexOrderId = bundle.connector.sendNewOrder(cffexReq);
            assertTrue(cffexOrderId > 0, "CFFEX 订单 OrderID 应为正值");

            // 验证: SHFE request queue 中有 1 条订单
            // 使用 Connector.shmMgr 内部已附着的队列（不额外占用 shmseg 名额）
            MWMRQueue shfeReqQueue = bundle.getReqQueue(0); // exchIdx=0 → SHFE
            MemorySegment dequeueBuf = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            assertTrue(shfeReqQueue.dequeue(dequeueBuf),
                    "SHFE request queue 应有 1 条订单");
            assertEquals(shfeOrderId,
                    (int) Types.REQ_ORDER_ID_VH.get(dequeueBuf, 0L),
                    "SHFE queue 中的 OrderID 应为 SHFE 订单");
            assertFalse(shfeReqQueue.dequeue(dequeueBuf),
                    "SHFE request queue 应只有 1 条订单");

            // 验证: CFFEX request queue 中有 1 条订单
            MWMRQueue cffexReqQueue = bundle.getReqQueue(1); // exchIdx=1 → CFFEX
            assertTrue(cffexReqQueue.dequeue(dequeueBuf),
                    "CFFEX request queue 应有 1 条订单");
            assertEquals(cffexOrderId,
                    (int) Types.REQ_ORDER_ID_VH.get(dequeueBuf, 0L),
                    "CFFEX queue 中的 OrderID 应为 CFFEX 订单");
            assertFalse(cffexReqQueue.dequeue(dequeueBuf),
                    "CFFEX request queue 应只有 1 条订单");
        } finally {
            bundle.destroy();
        }
    }

    // =====================================================================
    // 测试 10: 多交易所 ORS 回报隔离
    // 验证: SHFE 回报队列入队本 clientId 回报 → 回调触发
    //       CFFEX 回报队列入队本 clientId 回报 → 回调触发
    //       SHFE 回报队列入队 clientId=5 回报 → 回调不触发（被过滤）
    //       总回调数 = 2
    //
    // C++: exchId = m_response_queue_to_exchange_map[queueNum];
    //      m_all_clientIds[exchId][i] == clientId → m_orscb(msg)
    // Ref: hftbase/Connector/src/connector.cpp:819-838
    // =====================================================================

    @Test
    @Order(10)
    void test_multiExchange_orsResponseIsolation() throws InterruptedException {
        Connector.Config cfg = newDualExchangeConfig();

        AtomicInteger callbackCount = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(2);

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> {},
                        resp -> {
                            callbackCount.incrementAndGet();
                            latch.countDown();
                        });

        try (Arena arena = Arena.ofConfined()) {
            int shfeClientId = bundle.getClientId(Constants.CHINA_SHFE);
            int cffexClientId = bundle.getClientId(Constants.CHINA_CFFEX);

            bundle.startAsync();
            Thread.sleep(50);

            // SHFE 回报: 本 clientId → 应触发回调
            MemorySegment shfeResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            int shfeOrderId = shfeClientId * Constants.ORDERID_RANGE + 1;
            Types.RESP_ORDER_ID_VH.set(shfeResp, 0L, shfeOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(shfeResp, 0L, Constants.RESP_TRADE_CONFIRM);
            bundle.enqueueResponse(shfeResp, 0); // exchIdx=0 → SHFE

            // CFFEX 回报: 本 clientId → 应触发回调
            MemorySegment cffexResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            int cffexOrderId = cffexClientId * Constants.ORDERID_RANGE + 1;
            Types.RESP_ORDER_ID_VH.set(cffexResp, 0L, cffexOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(cffexResp, 0L, Constants.RESP_TRADE_CONFIRM);
            bundle.enqueueResponse(cffexResp, 1); // exchIdx=1 → CFFEX

            // SHFE 回报: 非本 clientId(=5) → 不应触发回调
            MemorySegment otherResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            int otherOrderId = 5 * Constants.ORDERID_RANGE + 1;
            Types.RESP_ORDER_ID_VH.set(otherResp, 0L, otherOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(otherResp, 0L, Constants.RESP_TRADE_CONFIRM);
            bundle.enqueueResponse(otherResp, 0); // exchIdx=0 → SHFE

            // 等待 2 条合法回报的回调
            assertTrue(latch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到 2 条回调");

            // 稍等以确保不会有额外的回调
            Thread.sleep(200);

            assertEquals(2, callbackCount.get(),
                    "应仅收到 2 条回调（SHFE 1 条 + CFFEX 1 条，clientId=5 被过滤）");
        } finally {
            bundle.destroy();
        }
    }

    // =====================================================================
    // 测试 11: 多交易所完整流程
    // 流程:
    //   MD 分别写入 SHFE/CFFEX 队列 → 两次 MD 回调
    //   分别在 SHFE/CFFEX 发单 → 模拟对应队列回报 → 两次 ORS 回调
    //   验证 OrderID 匹配
    // =====================================================================

    @Test
    @Order(11)
    void test_multiExchange_fullFlow() throws InterruptedException {
        Connector.Config cfg = newDualExchangeConfig();

        AtomicInteger mdCount = new AtomicInteger(0);
        CountDownLatch mdLatch = new CountDownLatch(2);
        CountDownLatch orsLatch = new CountDownLatch(2);
        List<Integer> receivedOrderIds = java.util.Collections.synchronizedList(new java.util.ArrayList<>());

        ConnectorTestHelper.TestConnectorBundle bundle =
                ConnectorTestHelper.createForTest(cfg,
                        md -> {
                            mdCount.incrementAndGet();
                            mdLatch.countDown();
                        },
                        resp -> {
                            int oid = (int) Types.RESP_ORDER_ID_VH.get(resp, 0L);
                            receivedOrderIds.add(oid);
                            orsLatch.countDown();
                        });

        try (Arena arena = Arena.ofConfined()) {
            bundle.startAsync();
            Thread.sleep(50);

            // Step 1: MD 写入 SHFE 和 CFFEX 队列
            MemorySegment shfeMd = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            shfeMd.set(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET, 6000.0);
            bundle.enqueueMD(shfeMd, 0);

            MemorySegment cffexMd = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            cffexMd.set(ValueLayout.JAVA_DOUBLE,
                    Types.MU_DATA_OFFSET + Types.MDD_NEW_PRICE_OFFSET, 4000.0);
            bundle.enqueueMD(cffexMd, 1);

            assertTrue(mdLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到 2 条行情回调");
            assertEquals(2, mdCount.get(), "应收到 2 条行情回调");

            // Step 2: 在 SHFE 发单
            MemorySegment shfeReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(shfeReq, 0L, (byte) Constants.CHINA_SHFE);
            int shfeOrderId = bundle.connector.sendNewOrder(shfeReq);

            // Step 3: 在 CFFEX 发单
            MemorySegment cffexReq = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            Types.REQ_EXCHANGE_TYPE_VH.set(cffexReq, 0L, (byte) Constants.CHINA_CFFEX);
            int cffexOrderId = bundle.connector.sendNewOrder(cffexReq);

            // Step 4: 模拟 SHFE 回报
            MemorySegment shfeResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(shfeResp, 0L, shfeOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(shfeResp, 0L, Constants.RESP_TRADE_CONFIRM);
            bundle.enqueueResponse(shfeResp, 0);

            // Step 5: 模拟 CFFEX 回报
            MemorySegment cffexResp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            Types.RESP_ORDER_ID_VH.set(cffexResp, 0L, cffexOrderId);
            Types.RESP_RESPONSE_TYPE_VH.set(cffexResp, 0L, Constants.RESP_TRADE_CONFIRM);
            bundle.enqueueResponse(cffexResp, 1);

            assertTrue(orsLatch.await(3, TimeUnit.SECONDS),
                    "应在 3 秒内收到 2 条回报回调");

            // 验证: 收到的 OrderID 包含 SHFE 和 CFFEX 的
            assertTrue(receivedOrderIds.contains(shfeOrderId),
                    "应收到 SHFE OrderID=" + shfeOrderId);
            assertTrue(receivedOrderIds.contains(cffexOrderId),
                    "应收到 CFFEX OrderID=" + cffexOrderId);
        } finally {
            bundle.destroy();
        }
    }
}
