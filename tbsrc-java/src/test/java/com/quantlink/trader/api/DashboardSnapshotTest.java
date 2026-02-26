package com.quantlink.trader.api;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;

import static org.junit.jupiter.api.Assertions.*;

/**
 * DashboardSnapshot JSON 序列化测试。
 * 验证 Jackson @JsonProperty 注解生成正确的 snake_case JSON。
 */
class DashboardSnapshotTest {

    private static final ObjectMapper mapper = new ObjectMapper();

    @Test
    void testSnapshotJsonKeys() throws Exception {
        DashboardSnapshot snap = new DashboardSnapshot();
        snap.timestamp = "2026-02-26T10:00:00Z";
        snap.strategyID = 92201;
        snap.active = true;
        snap.exposure = 5;

        snap.spread.current = 1.5;
        snap.spread.avgSpread = 1.2;
        snap.spread.avgOri = 1.0;
        snap.spread.tValue = 2.5;
        snap.spread.deviation = 0.3;
        snap.spread.isValid = true;
        snap.spread.alpha = 0.01;

        snap.leg1.symbol = "ag2603";
        snap.leg1.bidPx = 5600.0;
        snap.leg1.askPx = 5601.0;
        snap.leg1.netpos = 3;
        snap.leg1.realisedPNL = 1000.0;
        snap.leg1.tholdBidPlace = 1.5;
        snap.leg1.onExit = false;
        snap.leg1.orders = new ArrayList<>();

        snap.leg2.symbol = "ag2605";
        snap.leg2.orders = new ArrayList<>();

        String json = mapper.writeValueAsString(snap);
        JsonNode root = mapper.readTree(json);

        // 顶级字段
        assertEquals("2026-02-26T10:00:00Z", root.get("timestamp").asText());
        assertEquals(92201, root.get("strategy_id").asInt());
        assertTrue(root.get("active").asBoolean());
        assertEquals(5, root.get("exposure").asInt());

        // 价差字段 snake_case
        JsonNode spread = root.get("spread");
        assertNotNull(spread);
        assertEquals(1.5, spread.get("current").asDouble(), 1e-9);
        assertEquals(1.2, spread.get("avg_spread").asDouble(), 1e-9);
        assertEquals(1.0, spread.get("avg_ori").asDouble(), 1e-9);
        assertEquals(2.5, spread.get("t_value").asDouble(), 1e-9);
        assertEquals(0.3, spread.get("deviation").asDouble(), 1e-9);
        assertTrue(spread.get("is_valid").asBoolean());
        assertEquals(0.01, spread.get("alpha").asDouble(), 1e-9);

        // Leg1 字段 snake_case
        JsonNode leg1 = root.get("leg1");
        assertNotNull(leg1);
        assertEquals("ag2603", leg1.get("symbol").asText());
        assertEquals(5600.0, leg1.get("bid_px").asDouble(), 1e-9);
        assertEquals(5601.0, leg1.get("ask_px").asDouble(), 1e-9);
        assertEquals(3, leg1.get("netpos").asInt());
        assertEquals(1000.0, leg1.get("realised_pnl").asDouble(), 1e-9);
        assertEquals(1.5, leg1.get("thold_bid_place").asDouble(), 1e-9);
        assertFalse(leg1.get("on_exit").asBoolean());
    }

    @Test
    void testOrderSnapshotJsonKeys() throws Exception {
        DashboardSnapshot.OrderSnapshot os = new DashboardSnapshot.OrderSnapshot();
        os.orderID = 42;
        os.side = "BUY";
        os.price = 5600.0;
        os.openQty = 3;
        os.doneQty = 2;
        os.status = "NEW_CONFIRM";
        os.ordType = "STANDARD";
        os.time = "14:30:00";

        String json = mapper.writeValueAsString(os);
        JsonNode root = mapper.readTree(json);

        assertEquals(42, root.get("order_id").asInt());
        assertEquals("BUY", root.get("side").asText());
        assertEquals(5600.0, root.get("price").asDouble(), 1e-9);
        assertEquals(3, root.get("open_qty").asInt());
        assertEquals(2, root.get("done_qty").asInt());
        assertEquals("NEW_CONFIRM", root.get("status").asText());
        assertEquals("STANDARD", root.get("ord_type").asText());
        assertEquals("14:30:00", root.get("time").asText());
    }
}
