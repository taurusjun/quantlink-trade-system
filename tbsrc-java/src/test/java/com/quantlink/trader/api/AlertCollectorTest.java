package com.quantlink.trader.api;

import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class AlertCollectorTest {

    @Test
    void testAddAndGetAll() {
        AlertCollector collector = new AlertCollector();
        assertEquals(0, collector.count());

        collector.add(new AlertEvent(AlertEvent.LEVEL_WARNING, AlertEvent.TYPE_UPNL_LOSS,
                "UPNL LOSS triggered", "ag2606", 92201));
        assertEquals(1, collector.count());

        List<AlertEvent> all = collector.getAll();
        assertEquals(1, all.size());
        assertEquals(AlertEvent.TYPE_UPNL_LOSS, all.get(0).type);
        assertEquals(AlertEvent.LEVEL_WARNING, all.get(0).level);
        assertEquals("ag2606", all.get(0).symbol);
        assertEquals(92201, all.get(0).strategyId);
    }

    @Test
    void testMultipleAlerts() {
        AlertCollector collector = new AlertCollector();

        collector.add(new AlertEvent(AlertEvent.LEVEL_WARNING, AlertEvent.TYPE_UPNL_LOSS,
                "msg1", "ag2606", 92201));
        collector.add(new AlertEvent(AlertEvent.LEVEL_CRITICAL, AlertEvent.TYPE_MAX_LOSS,
                "msg2", "ag2608", 92201));
        collector.add(new AlertEvent(AlertEvent.LEVEL_CRITICAL, AlertEvent.TYPE_AVG_SPREAD_AWAY,
                "msg3", "ag2606", 92201));

        assertEquals(3, collector.count());
        List<AlertEvent> all = collector.getAll();
        assertEquals(3, all.size());
        // 按时间升序（先进先出）
        assertEquals(AlertEvent.TYPE_UPNL_LOSS, all.get(0).type);
        assertEquals(AlertEvent.TYPE_MAX_LOSS, all.get(1).type);
        assertEquals(AlertEvent.TYPE_AVG_SPREAD_AWAY, all.get(2).type);
    }

    @Test
    void testOverflowEviction() {
        AlertCollector collector = new AlertCollector();

        // 添加 110 条，超过容量 100
        for (int i = 0; i < 110; i++) {
            collector.add(new AlertEvent(AlertEvent.LEVEL_WARNING, "TYPE_" + i,
                    "msg-" + i, "ag2606", 92201));
        }

        // 容量不超过 100
        assertEquals(100, collector.count());

        List<AlertEvent> all = collector.getAll();
        assertEquals(100, all.size());

        // 最旧的 10 条被淘汰，剩余 type 从 TYPE_10 到 TYPE_109
        assertEquals("TYPE_10", all.get(0).type);
        assertEquals("TYPE_109", all.get(99).type);
    }

    @Test
    void testTimestampAutoSet() {
        long before = System.currentTimeMillis();
        AlertEvent event = new AlertEvent(AlertEvent.LEVEL_CRITICAL, AlertEvent.TYPE_END_TIME,
                "End time", "ag2606", 92201);
        long after = System.currentTimeMillis();

        assertTrue(event.timestamp >= before);
        assertTrue(event.timestamp <= after);
    }
}
