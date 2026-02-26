package com.quantlink.trader.api;

import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * OrderHistoryTracker 单元测试。
 * 对齐: tbsrc-golang/pkg/api/snapshot.go — OrderHistoryTracker
 */
class OrderHistoryTrackerTest {

    @Test
    void testNewOrdersAddedToHistory() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(10);

        List<DashboardSnapshot.OrderSnapshot> live = new ArrayList<>();
        live.add(makeOrder(1, "BUY", 100.0, 5, 0, "NEW_CONFIRM"));
        live.add(makeOrder(2, "SELL", 101.0, 3, 0, "NEW_CONFIRM"));

        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(live);
        assertEquals(2, result.size());
        // 最新在前
        assertEquals(2, result.get(0).orderID);
        assertEquals(1, result.get(1).orderID);
    }

    @Test
    void testRemovedOrdersMarkedAsTraded() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(10);

        // 第一次：2 个活跃订单
        List<DashboardSnapshot.OrderSnapshot> live1 = new ArrayList<>();
        live1.add(makeOrder(1, "BUY", 100.0, 5, 0, "NEW_CONFIRM"));
        live1.add(makeOrder(2, "SELL", 101.0, 3, 0, "NEW_CONFIRM"));
        tracker.update(live1);

        // 第二次：只有 order 2 还在（order 1 已从 OrdMap 移除 = 已成交）
        List<DashboardSnapshot.OrderSnapshot> live2 = new ArrayList<>();
        live2.add(makeOrder(2, "SELL", 101.0, 3, 0, "NEW_CONFIRM"));
        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(live2);

        assertEquals(2, result.size());
        // order 1 应该被标记为 TRADED
        DashboardSnapshot.OrderSnapshot order1 = result.stream()
                .filter(o -> o.orderID == 1).findFirst().orElseThrow();
        assertEquals("TRADED", order1.status);
    }

    @Test
    void testCancelConfirmNotOverwritten() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(10);

        // order 1 状态为 CANCEL_CONFIRM
        List<DashboardSnapshot.OrderSnapshot> live1 = new ArrayList<>();
        live1.add(makeOrder(1, "BUY", 100.0, 5, 0, "CANCEL_CONFIRM"));
        tracker.update(live1);

        // order 1 不在 liveOrders 中了，但 CANCEL_CONFIRM 不应被覆盖为 TRADED
        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(new ArrayList<>());

        DashboardSnapshot.OrderSnapshot order1 = result.stream()
                .filter(o -> o.orderID == 1).findFirst().orElseThrow();
        assertEquals("CANCEL_CONFIRM", order1.status);
    }

    @Test
    void testMaxSizeEnforced() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(3);

        // 添加 5 个订单
        for (int i = 1; i <= 5; i++) {
            List<DashboardSnapshot.OrderSnapshot> live = new ArrayList<>();
            live.add(makeOrder(i, "BUY", 100.0 + i, 1, 0, "NEW_CONFIRM"));
            tracker.update(live);
        }

        // 全部消失
        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(new ArrayList<>());
        // 最多保留 3 条
        assertEquals(3, result.size());
        // 最新的 3 个（3, 4, 5）
        assertEquals(5, result.get(0).orderID);
        assertEquals(4, result.get(1).orderID);
        assertEquals(3, result.get(2).orderID);
    }

    @Test
    void testUpdateExistingOrderStatus() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(10);

        // 第一次：NEW_CONFIRM
        List<DashboardSnapshot.OrderSnapshot> live1 = new ArrayList<>();
        live1.add(makeOrder(1, "BUY", 100.0, 5, 0, "NEW_CONFIRM"));
        tracker.update(live1);

        // 第二次：部分成交
        List<DashboardSnapshot.OrderSnapshot> live2 = new ArrayList<>();
        live2.add(makeOrder(1, "BUY", 100.0, 3, 2, "NEW_CONFIRM"));
        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(live2);

        assertEquals(1, result.size());
        assertEquals(3, result.get(0).openQty);
        assertEquals(2, result.get(0).doneQty);
    }

    @Test
    void testNullLiveOrders() {
        OrderHistoryTracker tracker = new OrderHistoryTracker(10);
        List<DashboardSnapshot.OrderSnapshot> result = tracker.update(null);
        assertNotNull(result);
        assertTrue(result.isEmpty());
    }

    private static DashboardSnapshot.OrderSnapshot makeOrder(
            int id, String side, double price, int openQty, int doneQty, String status) {
        DashboardSnapshot.OrderSnapshot o = new DashboardSnapshot.OrderSnapshot();
        o.orderID = id;
        o.side = side;
        o.price = price;
        o.openQty = openQty;
        o.doneQty = doneQty;
        o.status = status;
        o.ordType = "STANDARD";
        o.time = "";
        return o;
    }
}
