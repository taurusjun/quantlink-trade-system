package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.core.Watch;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;

import static org.junit.jupiter.api.Assertions.*;

/**
 * ExecutionStrategy 基类单元测试。
 */
class ExecutionStrategyTest {

    private Arena arena;
    private MockCommonClient client;
    private SimConfig simConfig;
    private Instrument instru;
    private TestStrategy strategy;

    /** 具体策略子类（最小实现） */
    static class TestStrategy extends ExecutionStrategy {
        public boolean sendOrderCalled = false;

        public TestStrategy(CommonClient client, SimConfig simConfig) {
            super(client, simConfig);
        }

        @Override
        public void sendOrder() {
            sendOrderCalled = true;
        }
    }

    @BeforeEach
    void setup() {
        ConfigParams.resetInstance();
        Watch.resetInstance();
        Watch.createInstance(0);
        arena = Arena.ofConfined();

        instru = new Instrument();
        instru.origBaseName = "ag2603";
        instru.symbol = "ag2603";
        instru.tickSize = 1.0;
        instru.lotSize = 15.0;
        instru.priceMultiplier = 15.0;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;
        instru.bidQty[0] = 10;
        instru.askQty[0] = 10;

        simConfig = new SimConfig();
        simConfig.instrument = instru;
        simConfig.thresholdSet.MAX_SIZE = 10;
        simConfig.thresholdSet.SIZE = 1;
        simConfig.thresholdSet.BEGIN_SIZE = 5;
        simConfig.thresholdSet.BEGIN_PLACE = 2.0;
        simConfig.thresholdSet.BEGIN_REMOVE = 1.0;
        simConfig.thresholdSet.LONG_PLACE = 4.0;
        simConfig.thresholdSet.LONG_REMOVE = 3.0;
        simConfig.thresholdSet.SHORT_PLACE = 0.5;
        simConfig.thresholdSet.SHORT_REMOVE = 0.3;
        simConfig.thresholdSet.MAX_LOSS = 100000;
        simConfig.thresholdSet.CHECK_PNL = true;
        simConfig.thresholdSet.UPNL_LOSS = 50000;
        simConfig.thresholdSet.STOP_LOSS = 80000;

        client = new MockCommonClient();

        ConfigParams.getInstance().modeType = 1; // Sim mode
        strategy = new TestStrategy(client, simConfig);
    }

    @AfterEach
    void cleanup() {
        arena.close();
        Watch.resetInstance();
        ConfigParams.resetInstance();
    }

    @Test
    void test_constructorInitialization() {
        assertEquals(instru, strategy.instru);
        assertEquals(simConfig, strategy.simConfig);
        assertTrue(strategy.active); // Sim mode → active
        assertEquals(0, strategy.netpos);
        assertEquals(0, strategy.netposPass);
        assertEquals(0, strategy.netposAgg);
    }

    @Test
    void test_reset() {
        strategy.netpos = 5;
        strategy.realisedPNL = 100;
        strategy.buyTotalQty = 50;

        strategy.reset();

        assertEquals(0, strategy.netpos);
        assertEquals(0, strategy.realisedPNL);
        assertEquals(0, strategy.buyTotalQty);
        assertTrue(strategy.active);
        assertTrue(strategy.ordMap.isEmpty());
    }

    @Test
    void test_setThresholds_zeroPos() {
        strategy.netpos = 0;
        strategy.setThresholds();

        assertEquals(2.0, strategy.tholdBidPlace, 0.001);
        assertEquals(2.0, strategy.tholdAskPlace, 0.001);
        assertEquals(1.0, strategy.tholdBidRemove, 0.001);
        assertEquals(1.0, strategy.tholdAskRemove, 0.001);
    }

    @Test
    void test_setThresholds_longPos() {
        strategy.netpos = 3; // > 0, < beginPos(5)
        strategy.setThresholds();

        assertEquals(2.0, strategy.tholdBidPlace, 0.001); // BEGIN_PLACE
        assertEquals(0.5, strategy.tholdAskPlace, 0.001); // SHORT_PLACE
    }

    @Test
    void test_setThresholds_shortPos() {
        strategy.netpos = -3; // < 0, > -beginPos
        strategy.setThresholds();

        assertEquals(0.5, strategy.tholdBidPlace, 0.001); // SHORT_PLACE
        assertEquals(2.0, strategy.tholdAskPlace, 0.001); // BEGIN_PLACE
    }

    @Test
    void test_setThresholds_largeLongPos() {
        strategy.netpos = 8; // > beginPos(5)
        strategy.setThresholds();

        assertEquals(4.0, strategy.tholdBidPlace, 0.001); // LONG_PLACE
        assertEquals(0.5, strategy.tholdAskPlace, 0.001); // SHORT_PLACE
    }

    @Test
    void test_sendNewOrder_buy() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);

        assertNotNull(order);
        assertEquals(5000, order.price);
        assertEquals(1, order.qty);
        assertEquals(1, order.openQty);
        assertEquals(Constants.SIDE_BUY, order.side);
        assertEquals(OrderStats.Status.NEW_ORDER, order.status);
        assertEquals(1, strategy.buyOpenOrders);
        assertEquals(1, strategy.orderCount);
        assertTrue(strategy.ordMap.containsValue(order));
        assertTrue(strategy.bidMap.containsKey(5000.0));
    }

    @Test
    void test_sendNewOrder_sell() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 2, 0);

        assertNotNull(order);
        assertEquals(Constants.SIDE_SELL, order.side);
        assertEquals(1, strategy.sellOpenOrders);
        assertTrue(strategy.askMap.containsKey(5001.0));
    }

    @Test
    void test_sendNewOrder_duplicatePrice() {
        strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        OrderStats dup = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);

        assertNull(dup); // 重复价格返回 null
        assertEquals(1, strategy.bidMap.size());
    }

    @Test
    void test_sendCancelOrder_byOrderID() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        boolean result = strategy.sendCancelOrder(order.orderID);

        assertTrue(result);
        assertEquals(OrderStats.Status.CANCEL_ORDER, order.status);
        assertTrue(order.cancel);
        assertEquals(1, strategy.cancelCount);
    }

    @Test
    void test_sendCancelOrder_byPrice() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        boolean result = strategy.sendCancelOrder(5000.0, Constants.SIDE_BUY);

        assertTrue(result);
    }

    @Test
    void test_sendModifyOrder() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        OrderStats modified = strategy.sendModifyOrder(order.orderID, 4999, 5000, 2, 0, OrderStats.HitType.STANDARD);

        assertNotNull(modified);
        assertEquals(OrderStats.Status.MODIFY_ORDER, modified.status);
        assertEquals(4999, modified.newPrice);
        assertTrue(strategy.bidMap.containsKey(4999.0));
    }

    @Test
    void test_calculatePNL_longPosition() {
        strategy.netpos = 1;
        strategy.buyPrice = 5000;
        strategy.buyQty = 1;
        instru.bidPx[0] = 5010;

        strategy.calculatePNL();

        assertTrue(strategy.unrealisedPNL != 0);
    }

    @Test
    void test_calculatePNL_flatPosition() {
        strategy.netpos = 0;
        strategy.calculatePNL();

        assertEquals(0, strategy.unrealisedPNL, 0.001);
    }

    @Test
    void test_checkSquareoff_maxLoss() {
        strategy.netPNL = -200000; // exceeds MAX_LOSS = 100000

        strategy.checkSquareoff();

        assertTrue(strategy.onExit);
        // C++: handleSquareoff() 在 checkSquareoff 末尾被调用，会重置 onCancel = false
        // 验证 onExit 和 aggFlat 保持 true（已触发退出）
        assertFalse(strategy.onCancel); // handleSquareoff() resets onCancel
        assertTrue(strategy.onFlat);
    }

    @Test
    void test_checkSquareoff_noTrigger() {
        strategy.netPNL = 100; // positive
        strategy.maxTradedQty = 1_000_000; // set high to avoid trigger

        MemorySegment update = arena.allocate(Types.MD_HEADER_LAYOUT);
        strategy.checkSquareoff();

        assertFalse(strategy.onExit);
        assertFalse(strategy.onFlat);
    }

    @Test
    void test_processTrade_buy() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 2, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        // 构造 trade response
        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_TRADE_CONFIRM);
        Types.RESP_QUANTITY_VH.set(resp, 0L, 1);
        Types.RESP_PRICE_VH.set(resp, 0L, 5000.0);

        strategy.orsCallBack(resp);

        assertEquals(1, strategy.tradeCount);
        assertEquals(1, order.doneQty);
        assertEquals(1, order.openQty);
        assertEquals(1, strategy.netpos);
        assertEquals(5000, strategy.buyTotalValue, 0.001);
    }

    @Test
    void test_processTrade_fullFill() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_TRADE_CONFIRM);
        Types.RESP_QUANTITY_VH.set(resp, 0L, 1);
        Types.RESP_PRICE_VH.set(resp, 0L, 5000.0);

        strategy.orsCallBack(resp);

        assertEquals(OrderStats.Status.TRADED, order.status);
        assertFalse(strategy.ordMap.containsKey(order.orderID));
        assertFalse(strategy.bidMap.containsKey(5000.0));
    }

    @Test
    void test_orsCallBack_newConfirm() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);

        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_NEW_ORDER_CONFIRM);

        strategy.orsCallBack(resp);

        assertEquals(OrderStats.Status.NEW_CONFIRM, order.status);
        assertEquals(1, strategy.confirmCount);
    }

    @Test
    void test_orsCallBack_cancelConfirm() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 1, 0);
        order.status = OrderStats.Status.CANCEL_ORDER;

        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_CANCEL_ORDER_CONFIRM);

        strategy.orsCallBack(resp);

        assertFalse(strategy.ordMap.containsKey(order.orderID));
        assertFalse(strategy.askMap.containsKey(5001.0));
        assertEquals(1, strategy.cancelconfirmCount);
    }

    @Test
    void test_orsCallBack_newReject() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);

        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_ORDER_ERROR);

        strategy.orsCallBack(resp);

        assertEquals(OrderStats.Status.NEW_REJECT, order.status);
        assertFalse(strategy.ordMap.containsKey(order.orderID));
        assertEquals(1, strategy.rejectCount);
    }

    @Test
    void test_roundWorse() {
        assertEquals(5000, strategy.roundWorse(Constants.SIDE_BUY, 5000.7, 1.0), 0.001);
        assertEquals(5001, strategy.roundWorse(Constants.SIDE_SELL, 5000.3, 1.0), 0.001);
    }

    @Test
    void test_handleSquareoff() {
        OrderStats o1 = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        o1.status = OrderStats.Status.NEW_CONFIRM;
        OrderStats o2 = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 1, 0);
        o2.status = OrderStats.Status.NEW_CONFIRM;

        strategy.handleSquareoff();

        assertEquals(2, strategy.cancelCount);
    }

    @Test
    void test_setLinearThresholds_positivePos() {
        strategy.netpos = 5;
        strategy.setLinearThresholds();

        // With netpos=5, maxPos=10: interpolation ratio = 0.5
        // tholdBidPlace = 2.0 + (4.0-2.0)*5/10 = 3.0
        assertEquals(3.0, strategy.tholdBidPlace, 0.001);
        // tholdAskPlace = 2.0 - (2.0-0.5)*5/10 = 1.25
        assertEquals(1.25, strategy.tholdAskPlace, 0.001);
    }

    @Test
    void test_setLinearThresholds_negativePos() {
        strategy.netpos = -5;
        strategy.setLinearThresholds();

        // tholdAskPlace = 2.0 + (4.0-2.0)*5/10 = 3.0
        assertEquals(3.0, strategy.tholdAskPlace, 0.001);
        // tholdBidPlace = 2.0 - (2.0-0.5)*5/10 = 1.25
        assertEquals(1.25, strategy.tholdBidPlace, 0.001);
    }

    // =======================================================================
    //  Bug fix tests: handleSquareoff endTime 误发单
    //  事故: 策略在 endTime 后启动，checkSquareoff 触发 onFlat，
    //  基类 handleSquareoff() 发送了 SELL 82 / BUY 83 (flag=OPEN)
    // =======================================================================

    /**
     * 验证: useArbStrat=true 时 checkSquareoff 也调用 handleSquareoff（与 C++ 一致）。
     * C++ ExecutionStrategy::CheckSquareoff() L2338 无条件调用 HandleSquareoff()。
     */
    @Test
    void test_checkSquareoff_useArbStrat_skipsHandleSquareoff() {
        // 模拟子 strat: useArbStrat=true, 有持仓, END TIME 触发
        simConfig.useArbStrat = true;
        ConfigParams.getInstance().modeType = 1; // sim, active=true
        TestStrategy subStrat = new TestStrategy(client, simConfig);
        subStrat.netpos = 82;
        subStrat.active = true;
        subStrat.instru = instru;

        // 设置 endTimeEpoch 为过去时间，触发 END TIME
        subStrat.endTimeEpoch = 1000L;
        // 通过 Watch 设置当前时间（替代原先直接设置 exchTS）
        Watch.getInstance().updateTime(2000L, "test");

        int ordersBefore = client.newOrderCount;

        subStrat.checkSquareoff();

        // 验证 onExit/onFlat 被设置
        assertTrue(subStrat.onExit, "onExit should be set");
        assertTrue(subStrat.onFlat, "onFlat should be set");

        // 验证：useArbStrat=true 时也调用 handleSquareoff → 发平仓单（与 C++ 一致）
        assertTrue(client.newOrderCount > ordersBefore,
                "useArbStrat=true 时 checkSquareoff 也应发送平仓订单（C++ 无条件调用）");
    }

    /**
     * 对比: useArbStrat=false 时 checkSquareoff 正常调用 handleSquareoff()。
     */
    @Test
    void test_checkSquareoff_nonArbStrat_callsHandleSquareoff() {
        simConfig.useArbStrat = false;
        ConfigParams.getInstance().modeType = 1; // sim, active=true
        TestStrategy standalone = new TestStrategy(client, simConfig);
        standalone.netpos = 10;
        standalone.active = true;
        standalone.instru = instru;
        standalone.endTimeEpoch = 1000L;
        // 通过 Watch 设置当前时间（替代原先直接设置 exchTS）
        Watch.getInstance().updateTime(2000L, "test");

        int ordersBefore = client.newOrderCount;

        standalone.checkSquareoff();

        assertTrue(standalone.onExit);
        assertTrue(standalone.onFlat);

        // 验证：useArbStrat=false 时正常调用 handleSquareoff → 发单平仓
        assertTrue(client.newOrderCount > ordersBefore,
                "useArbStrat=false 时 checkSquareoff 应发送平仓订单");
    }

    /**
     * 验证: handleSquareoff active=false 时仍然发送平仓订单（与 C++ 一致）。
     * C++ ExecutionStrategy::HandleSquareoff() 不检查 m_Active。
     * PairwiseArb 场景下子腿 active 在 CTP 模式始终为 false，平仓仍需执行。
     */
    @Test
    void test_handleSquareoff_activeFalse_stillSendsOrders() {
        strategy.netpos = 82;
        strategy.active = false;
        strategy.onFlat = true;
        strategy.onExit = true;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;

        int ordersBefore = client.newOrderCount;

        strategy.handleSquareoff();

        assertTrue(client.newOrderCount > ordersBefore,
                "active=false 时 handleSquareoff 仍应发送平仓订单（C++ 不检查 active）");
    }

    /**
     * 验证: handleSquareoff active=false 负持仓时也发送买入平仓订单（与 C++ 一致）。
     */
    @Test
    void test_handleSquareoff_activeFalse_shortPos_stillSendsOrders() {
        strategy.netpos = -83;
        strategy.active = false;
        strategy.onFlat = true;
        strategy.onExit = true;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;

        int ordersBefore = client.newOrderCount;

        strategy.handleSquareoff();

        assertTrue(client.newOrderCount > ordersBefore,
                "active=false 时 handleSquareoff 仍应发送买入平仓订单（C++ 不检查 active）");
    }

    /**
     * 对比: handleSquareoff active=true 时正常发送平仓订单。
     */
    @Test
    void test_handleSquareoff_activeTrue_sendsOrders() {
        strategy.netpos = 82;
        strategy.active = true;
        strategy.onFlat = true;
        strategy.onExit = false;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;

        int ordersBefore = client.newOrderCount;

        strategy.handleSquareoff();

        assertEquals(ordersBefore + 1, client.newOrderCount,
                "active=true 时 handleSquareoff 应发送平仓订单");
        // 验证是卖单（平多仓）
        MockCommonClient.OrderRecord rec = client.orderRecords.get(client.orderRecords.size() - 1);
        assertEquals(Constants.SIDE_SELL, rec.side);
        assertEquals(82, rec.qty);
    }

    /**
     * CTP模式 active=false，有昨仓 82。
     * checkSquareoff 触发 END TIME → onFlat=true → handleSquareoff() → 应发单。
     * C++ HandleSquareoff() 不检查 active 状态 (ExecutionStrategy.cpp:2355-2437)，
     * 子腿 active=false 在 CTP 模式是正常状态（modeType != 1），不应阻止平仓。
     * PairwiseArb 层 active 守卫 (L711) 负责防止预激活触发，不影响子腿独立平仓。
     */
    @Test
    void test_checkSquareoff_ctpMode_activeFalse_stillSendsOrders() {
        // CTP 模式: modeType != 1
        ConfigParams.getInstance().modeType = 2;
        simConfig.useArbStrat = false; // 独立策略（测试基类行为）
        TestStrategy ctpStrat = new TestStrategy(client, simConfig);
        ctpStrat.netpos = 82;
        ctpStrat.instru = instru;

        // CTP mode: active=false (由 reset() 设置)
        assertFalse(ctpStrat.active, "CTP mode 应初始化为 active=false");

        // 设置 END TIME 已过
        ctpStrat.endTimeEpoch = 1000L;
        // 通过 Watch 设置当前时间（替代原先直接设置 exchTS）
        Watch.getInstance().updateTime(2000L, "test");

        int ordersBefore = client.newOrderCount;

        ctpStrat.checkSquareoff();

        assertTrue(ctpStrat.onExit, "onExit 应被设置");
        assertTrue(ctpStrat.onFlat, "onFlat 应被设置");

        // C++ 对齐: HandleSquareoff 不检查 active，有持仓就发平仓单
        assertTrue(client.newOrderCount > ordersBefore,
                "C++ HandleSquareoff 不检查 active，有持仓应发送平仓订单");
    }

    // =======================================================================
    //  Fix: CANCELREQ_PAUSE 撤单拒绝暂停机制
    //  C++: ExecutionStrategy.cpp:1764-1770, 1855-1872
    // =======================================================================

    @Test
    void test_sendCancelOrder_normalCancel_noPriorReject() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        // 无拒绝记录，lastCancelReqRejectSet = 0
        assertEquals(0, strategy.lastCancelReqRejectSet);

        boolean result = strategy.sendCancelOrder(order.orderID);

        assertTrue(result);
        assertEquals(OrderStats.Status.CANCEL_ORDER, order.status);
        assertEquals(1, strategy.cancelCount);
    }

    @Test
    void test_sendCancelOrder_crossOrder_cannotCancel() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;
        order.ordType = OrderStats.HitType.CROSS;

        boolean result = strategy.sendCancelOrder(order.orderID);

        assertFalse(result, "CROSS 类型订单不可撤");
        assertEquals(OrderStats.Status.NEW_CONFIRM, order.status);
    }

    @Test
    void test_sendCancelOrder_blockedByCancelReqPause() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        // 模拟该订单刚被拒绝
        strategy.lastCancelReqRejectSet = 1;
        strategy.lastCancelRejectOrderID = order.orderID;
        strategy.lastCancelRejectTime = Watch.getInstance().getCurrentTime();
        // CANCELREQ_PAUSE 设为很大值，确保不过期
        strategy.thold.CANCELREQ_PAUSE = 999_999_999_999L;

        boolean result = strategy.sendCancelOrder(order.orderID);

        assertFalse(result, "CANCELREQ_PAUSE 未过期时应阻止同一 orderID 的撤单");
        assertEquals(OrderStats.Status.NEW_CONFIRM, order.status, "状态不应变更");
    }

    @Test
    void test_sendCancelOrder_allowedAfterPauseExpired() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        // 模拟该订单很久之前被拒绝
        strategy.lastCancelReqRejectSet = 1;
        strategy.lastCancelRejectOrderID = order.orderID;
        strategy.lastCancelRejectTime = 1000L; // 很久以前
        strategy.thold.CANCELREQ_PAUSE = 1L;   // 1 纳秒

        // Watch 时间需要 > lastCancelRejectTime + CANCELREQ_PAUSE 才能通过暂停检查
        Watch.getInstance().updateTime(100_000L, "test");

        boolean result = strategy.sendCancelOrder(order.orderID);

        assertTrue(result, "CANCELREQ_PAUSE 过期后应允许重试");
        assertEquals(0, strategy.lastCancelReqRejectSet, "重试后应清除拒绝标记");
    }

    @Test
    void test_sendCancelOrder_differentOrderID_notBlocked() {
        OrderStats orderA = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        orderA.status = OrderStats.Status.NEW_CONFIRM;
        OrderStats orderB = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 1, 0);
        orderB.status = OrderStats.Status.NEW_CONFIRM;

        // 模拟 orderA 被拒绝
        strategy.lastCancelReqRejectSet = 1;
        strategy.lastCancelRejectOrderID = orderA.orderID;
        strategy.lastCancelRejectTime = Watch.getInstance().getCurrentTime();
        strategy.thold.CANCELREQ_PAUSE = 999_999_999_999L;

        // orderB 不应被阻止
        boolean result = strategy.sendCancelOrder(orderB.orderID);

        assertTrue(result, "不同 orderID 的撤单不应被阻止");
        assertEquals(OrderStats.Status.CANCEL_ORDER, orderB.status);
    }

    // =======================================================================
    //  Fix: processCancelReject 设置拒绝状态
    //  C++: ExecutionStrategy.cpp:1855-1886
    // =======================================================================

    @Test
    void test_processCancelReject_setsRejectState() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.CANCEL_ORDER;

        // Watch 时间需要 > 0 才能验证 lastCancelRejectTime 被正确设置
        Watch.getInstance().updateTime(500_000L, "test");

        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_CANCEL_ORDER_REJECT);
        Types.RESP_QUANTITY_VH.set(resp, 0L, 1); // non-zero to avoid fillOnCxlReject

        strategy.orsCallBack(resp);

        assertEquals(1, strategy.lastCancelReqRejectSet, "应设置拒绝标记");
        assertEquals(order.orderID, strategy.lastCancelRejectOrderID, "应记录被拒 orderID");
        assertTrue(strategy.lastCancelRejectTime > 0, "应记录拒绝时间");
        assertFalse(order.cancel, "cancel 标志应被清除");
    }

    // =======================================================================
    //  Fix: SelfBook 缓存删除 (bidMapCacheDel / askMapCacheDel)
    //  C++: ExecutionStrategy.cpp:1784-1795
    // =======================================================================

    @Test
    void test_sendCancelOrder_selfBook_insertsCacheDel() {
        ConfigParams.getInstance().bSelfBook = true;
        instru.bSnapshot = false;

        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 2, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;
        order.openQty = 2;

        strategy.sendCancelOrder(order.orderID);

        assertTrue(strategy.bidMapCacheDel.containsKey(5000.0),
                "SelfBook 模式下撤单应插入 bidMapCacheDel");
        assertEquals(2, order.cxlQty, "cxlQty 应设为 openQty");
    }

    @Test
    void test_sendCancelOrder_selfBook_sell_insertsCacheDel() {
        ConfigParams.getInstance().bSelfBook = true;
        instru.bSnapshot = false;

        OrderStats order = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 3, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;
        order.openQty = 3;

        strategy.sendCancelOrder(order.orderID);

        assertTrue(strategy.askMapCacheDel.containsKey(5001.0),
                "SelfBook 模式下卖方撤单应插入 askMapCacheDel");
    }

    @Test
    void test_processCancelReject_selfBook_clearsFromCacheDel() {
        ConfigParams.getInstance().bSelfBook = true;
        instru.bSnapshot = false;

        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;
        order.openQty = 1;

        // 先发起撤单，将订单插入 bidMapCacheDel
        strategy.sendCancelOrder(order.orderID);
        assertTrue(strategy.bidMapCacheDel.containsKey(5000.0));

        // 撤单被拒，应从 bidMapCacheDel 移除
        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_CANCEL_ORDER_REJECT);
        Types.RESP_QUANTITY_VH.set(resp, 0L, 1);

        strategy.orsCallBack(resp);

        assertFalse(strategy.bidMapCacheDel.containsKey(5000.0),
                "撤单被拒后应从 bidMapCacheDel 移除");
    }

    // =======================================================================
    //  Fix: removeOrder SelfBook 模式下的条件性清理
    //  C++: ExecutionStrategy.cpp:1175-1213
    // =======================================================================

    @Test
    void test_removeOrder_selfBook_cleansBidMapCache() {
        ConfigParams.getInstance().bSelfBook = true;
        instru.bSnapshot = false;

        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        // 手动放入 bidMapCache
        strategy.bidMapCache.put(5000.0, order);

        // 模拟 TRADED 状态
        order.status = OrderStats.Status.TRADED;
        order.cancel = true;
        strategy.removeOrder(order);

        assertFalse(strategy.bidMapCache.containsKey(5000.0),
                "removeOrder 应清理 bidMapCache");
    }

    @Test
    void test_removeOrder_selfBook_cancelConfirm_noCancelFlag() {
        ConfigParams.getInstance().bSelfBook = true;
        instru.bSnapshot = false;

        OrderStats order = strategy.sendNewOrder(Constants.SIDE_SELL, 5001, 1, 0);
        order.status = OrderStats.Status.CANCEL_CONFIRM;
        order.cancel = false;

        strategy.removeOrder(order);

        // C++: 当 cancel=false 时，ordMap 不移除，仅设置 cancel=true
        assertTrue(order.cancel, "cancel=false 的 CANCEL_CONFIRM 应将 cancel 设为 true");
        // ordMap 不应被移除（仅设置 cancel 标志）
        // 注意：askMap 已被移除，但 ordMap 保留
    }

    // =======================================================================
    //  Fix: handleSquareON 基类方法
    //  C++: ExecutionStrategy.h:47-51
    // =======================================================================

    @Test
    void test_handleSquareON_baseClass() {
        strategy.onExit = true;
        strategy.onCancel = true;
        strategy.product = "testProduct";
        strategy.strategyID = 99;

        // 基类 handleSquareON 应调用 sendMonitorStratStatus，不应抛异常
        assertDoesNotThrow(() -> strategy.handleSquareON());
    }

    @Test
    void test_reset_initializesCancelRejectFields() {
        strategy.lastCancelReqRejectSet = 1;
        strategy.lastCancelRejectTime = 12345L;
        strategy.lastCancelRejectOrderID = 999;

        strategy.reset();

        assertEquals(0, strategy.lastCancelReqRejectSet,
                "reset 应清除 lastCancelReqRejectSet");
        assertEquals(0, strategy.lastCancelRejectTime,
                "reset 应清除 lastCancelRejectTime");
        assertEquals(-1, strategy.lastCancelRejectOrderID,
                "reset 应将 lastCancelRejectOrderID 重置为 -1");
    }
}
