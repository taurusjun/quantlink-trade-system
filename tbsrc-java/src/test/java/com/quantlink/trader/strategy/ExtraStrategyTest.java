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
 * ExtraStrategy 单元测试 — Instrument 参数化订单方法。
 */
class ExtraStrategyTest {

    private MockCommonClient client;
    private SimConfig simConfig;
    private Instrument instru;
    private Instrument instruSec;
    private ExtraStrategy strategy;

    @BeforeEach
    void setup() {
        ConfigParams.resetInstance();
        Watch.resetInstance();
        Watch.createInstance(0);

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

        instruSec = new Instrument();
        instruSec.origBaseName = "ag2605";
        instruSec.symbol = "ag2605";
        instruSec.tickSize = 1.0;
        instruSec.lotSize = 15.0;
        instruSec.priceMultiplier = 15.0;
        instruSec.bidPx[0] = 5010;
        instruSec.askPx[0] = 5011;
        instruSec.bidQty[0] = 10;
        instruSec.askQty[0] = 10;

        simConfig = new SimConfig();
        simConfig.instrument = instru;
        simConfig.instrumentSec = instruSec;
        simConfig.thresholdSet.SIZE = 1;
        simConfig.thresholdSet.MAX_SIZE = 10;
        simConfig.thresholdSet.BEGIN_SIZE = 5;
        simConfig.thresholdSet.BID_SIZE = 2;
        simConfig.thresholdSet.ASK_SIZE = 2;

        client = new MockCommonClient();

        ConfigParams.getInstance().modeType = 1;
        strategy = new ExtraStrategy(client, simConfig);
    }

    @AfterEach
    void cleanup() {
        Watch.resetInstance();
        ConfigParams.resetInstance();
    }

    @Test
    void test_sendOrder_isEmpty() {
        // ExtraStrategy.sendOrder() 应为空实现
        strategy.sendOrder();
        // 不抛异常即为通过
    }

    @Test
    void test_sendBidOrder_withInstrument() {
        strategy.tholdSize = 1;
        strategy.sendBidOrder(instruSec, 0, 5010, OrderStats.HitType.STANDARD);

        // 订单应在 instru 被恢复后存在于 ordMap
        assertEquals(1, strategy.ordMap.size());
        assertEquals(1, strategy.buyOpenOrders);
        // instru 应已恢复
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendAskOrder_withInstrument() {
        strategy.tholdSize = 1;
        strategy.sendAskOrder(instruSec, 0, 5011, OrderStats.HitType.STANDARD);

        assertEquals(1, strategy.ordMap.size());
        assertEquals(1, strategy.sellOpenOrders);
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendBidOrder_withQuantity() {
        strategy.sendBidOrder(instruSec, 0, 5010, OrderStats.HitType.STANDARD, 3);

        assertEquals(1, strategy.ordMap.size());
        OrderStats order = strategy.ordMap.values().iterator().next();
        assertEquals(3, order.qty);
    }

    @Test
    void test_sendBidOrder_zeroPrice() {
        strategy.tholdSize = 1;
        strategy.sendBidOrder(instruSec, 0, 0, OrderStats.HitType.STANDARD);

        // 不应发单
        assertEquals(0, strategy.ordMap.size());
    }

    @Test
    void test_sendBidOrder2_returnsTrue() {
        strategy.tholdBidSize = 1;
        boolean result = strategy.sendBidOrder2(instruSec, 0, 5010, OrderStats.HitType.STANDARD, 0);

        assertTrue(result);
        assertEquals(1, strategy.ordMap.size());
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendBidOrder2_zeroPriceReturnsFalse() {
        strategy.tholdBidSize = 1;
        boolean result = strategy.sendBidOrder2(instruSec, 0, 0, OrderStats.HitType.STANDARD, 0);

        assertFalse(result);
        assertEquals(0, strategy.ordMap.size());
    }

    @Test
    void test_sendAskOrder2_returnsTrue() {
        strategy.tholdAskSize = 1;
        boolean result = strategy.sendAskOrder2(instruSec, 0, 5011, OrderStats.HitType.STANDARD, 0);

        assertTrue(result);
        assertEquals(1, strategy.ordMap.size());
    }

    @Test
    void test_sendNewOrder_withInstrument() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5010, 1, 0, instruSec, OrderStats.HitType.STANDARD);

        assertNotNull(order);
        assertEquals(5010, order.price);
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendModifyOrder_withInstrument() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5010, 1, 0, instruSec, OrderStats.HitType.STANDARD);
        order.status = OrderStats.Status.NEW_CONFIRM;

        OrderStats modified = strategy.sendModifyOrder(instruSec, order.orderID, 5009, 5010, 2, 0, OrderStats.HitType.STANDARD);

        assertNotNull(modified);
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendCancelOrder_withInstrument_byOrderID() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5010, 1, 0, instruSec, OrderStats.HitType.STANDARD);
        order.status = OrderStats.Status.NEW_CONFIRM;

        boolean result = strategy.sendCancelOrder(instruSec, order.orderID);

        assertTrue(result);
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_sendCancelOrder_withInstrument_byPriceSide() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5010, 1, 0, instruSec, OrderStats.HitType.STANDARD);
        order.status = OrderStats.Status.NEW_CONFIRM;

        boolean result = strategy.sendCancelOrder(instruSec, 5010, Constants.SIDE_BUY);

        assertTrue(result);
        assertEquals(instru, strategy.instru);
    }

    @Test
    void test_handleSquareoff_withInstrument() {
        OrderStats order = strategy.sendNewOrder(Constants.SIDE_BUY, 5010, 1, 0, instruSec, OrderStats.HitType.STANDARD);
        order.status = OrderStats.Status.NEW_CONFIRM;

        strategy.handleSquareoff(instruSec);

        assertEquals(instru, strategy.instru);
        assertEquals(1, strategy.cancelCount);
    }

    @Test
    void test_instruRestored_afterException() {
        // 验证 instru 在操作后总是被恢复
        strategy.tholdSize = 1;
        strategy.sendBidOrder(instruSec, 0, 5010, OrderStats.HitType.STANDARD);
        assertEquals(instru, strategy.instru);

        strategy.sendAskOrder(instruSec, 0, 5011, OrderStats.HitType.STANDARD);
        assertEquals(instru, strategy.instru);
    }
}
