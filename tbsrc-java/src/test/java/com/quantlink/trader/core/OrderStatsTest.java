package com.quantlink.trader.core;

import com.quantlink.trader.shm.Constants;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * OrderStats 订单状态测试。
 */
class OrderStatsTest {

    @Test
    void test_defaultValues() {
        OrderStats os = new OrderStats();
        assertEquals(OrderStats.Status.INIT, os.status);
        assertEquals(OrderStats.HitType.STANDARD, os.ordType);
        assertEquals(Constants.SIDE_BUY, os.side);
        assertFalse(os.active);
        assertEquals(0, os.orderID);
        assertEquals(0, os.openQty);
    }

    @Test
    void test_statusTransitions() {
        OrderStats os = new OrderStats();
        os.status = OrderStats.Status.NEW_ORDER;
        assertEquals(OrderStats.Status.NEW_ORDER, os.status);
        os.status = OrderStats.Status.NEW_CONFIRM;
        assertEquals(OrderStats.Status.NEW_CONFIRM, os.status);
        os.status = OrderStats.Status.TRADED;
        assertEquals(OrderStats.Status.TRADED, os.status);
    }

    @Test
    void test_hitTypeValues() {
        assertEquals(5, OrderStats.HitType.values().length);
        assertEquals(OrderStats.HitType.STANDARD, OrderStats.HitType.values()[0]);
        assertEquals(OrderStats.HitType.IMPROVE, OrderStats.HitType.values()[1]);
        assertEquals(OrderStats.HitType.CROSS, OrderStats.HitType.values()[2]);
        assertEquals(OrderStats.HitType.DETECT, OrderStats.HitType.values()[3]);
        assertEquals(OrderStats.HitType.MATCH, OrderStats.HitType.values()[4]);
    }

    @Test
    void test_statusValues() {
        assertEquals(11, OrderStats.Status.values().length);
        assertEquals(OrderStats.Status.NEW_ORDER, OrderStats.Status.values()[0]);
        assertEquals(OrderStats.Status.TRADED, OrderStats.Status.values()[9]);
        assertEquals(OrderStats.Status.INIT, OrderStats.Status.values()[10]);
    }

    @Test
    void test_orderLifecycle() {
        OrderStats os = new OrderStats();
        os.orderID = 1000001;
        os.price = 5500.0;
        os.qty = 5;
        os.openQty = 5;
        os.side = Constants.SIDE_BUY;
        os.ordType = OrderStats.HitType.STANDARD;

        // New order
        os.status = OrderStats.Status.NEW_ORDER;

        // Confirmed
        os.status = OrderStats.Status.NEW_CONFIRM;

        // Partial trade
        os.openQty -= 3;
        os.doneQty += 3;
        assertEquals(2, os.openQty);
        assertEquals(3, os.doneQty);

        // Full trade
        os.openQty -= 2;
        os.doneQty += 2;
        os.status = OrderStats.Status.TRADED;
        assertEquals(0, os.openQty);
        assertEquals(5, os.doneQty);
    }
}
