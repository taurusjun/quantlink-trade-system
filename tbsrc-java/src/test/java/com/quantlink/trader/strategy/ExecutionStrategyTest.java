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
     * Fix 1: useArbStrat=true 时 checkSquareoff 不调用基类 handleSquareoff()。
     * 子 strat 的平仓由父级 PairwiseArbStrategy 统一管理。
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

        // 验证：useArbStrat=true 时不调用基类 handleSquareoff → 不发单
        assertEquals(ordersBefore, client.newOrderCount,
                "useArbStrat=true 时 checkSquareoff 不应发送订单");
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
     * Fix 2: handleSquareoff active=false 时不发送平仓订单。
     */
    @Test
    void test_handleSquareoff_activeFlase_noOrders() {
        strategy.netpos = 82;
        strategy.active = false;
        strategy.onFlat = true;
        strategy.onExit = true;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;

        int ordersBefore = client.newOrderCount;

        strategy.handleSquareoff();

        assertEquals(ordersBefore, client.newOrderCount,
                "active=false 时 handleSquareoff 不应发送任何订单");
    }

    /**
     * Fix 2 (负持仓): handleSquareoff active=false 时不发送买入平仓订单。
     */
    @Test
    void test_handleSquareoff_activeFalse_shortPos_noOrders() {
        strategy.netpos = -83;
        strategy.active = false;
        strategy.onFlat = true;
        strategy.onExit = true;
        instru.bidPx[0] = 5000;
        instru.askPx[0] = 5001;

        int ordersBefore = client.newOrderCount;

        strategy.handleSquareoff();

        assertEquals(ordersBefore, client.newOrderCount,
                "active=false 时 handleSquareoff 不应发送买入订单");
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
     * Fix 2 综合: 模拟事故场景 — CTP模式 active=false，有昨仓 82/-83。
     * checkSquareoff 触发 END TIME → onFlat=true → handleSquareoff() → 不应发单。
     */
    @Test
    void test_checkSquareoff_ctpMode_activeFalse_noOrders() {
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

        // 关键验证: active=false 时 handleSquareoff 不发单
        assertEquals(ordersBefore, client.newOrderCount,
                "CTP mode active=false 时 handleSquareoff 不应发送任何订单");
    }
}
