package com.quantlink.trader.api;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.*;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.ArrayList;

import static org.junit.jupiter.api.Assertions.*;

/**
 * ApiServer REST 端点测试。
 * 验证 7 个 REST 端点返回正确的 JSON 格式。
 */
class ApiServerTest {

    private static ApiServer server;
    private static final int TEST_PORT = 19201; // 避免与正常实例冲突
    private static final ObjectMapper mapper = new ObjectMapper();
    private static final HttpClient http = HttpClient.newHttpClient();

    @BeforeAll
    static void startServer() {
        server = new ApiServer(TEST_PORT);
        server.start();
    }

    @AfterAll
    static void stopServer() {
        server.stop();
    }

    @Test
    void testHealthEndpoint() throws Exception {
        JsonNode resp = getJson("/api/v1/health");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("ok", resp.get("message").asText());
        assertTrue(resp.has("data"));
        assertTrue(resp.get("data").has("ws_clients"));
    }

    @Test
    void testStatusNoSnapshot() throws Exception {
        JsonNode resp = getJson("/api/v1/status");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("no snapshot yet", resp.get("message").asText());
    }

    @Test
    void testStatusWithSnapshot() throws Exception {
        DashboardSnapshot snap = new DashboardSnapshot();
        snap.timestamp = "2026-02-26T10:00:00Z";
        snap.strategyID = 92201;
        snap.active = true;
        snap.leg1.orders = new ArrayList<>();
        snap.leg2.orders = new ArrayList<>();
        server.updateSnapshot(snap);

        JsonNode resp = getJson("/api/v1/status");
        assertTrue(resp.get("success").asBoolean());
        JsonNode data = resp.get("data");
        assertNotNull(data);
        assertEquals(92201, data.get("strategy_id").asInt());
        assertTrue(data.get("active").asBoolean());
    }

    @Test
    void testOrdersNoSnapshot() throws Exception {
        // Reset snapshot
        ApiServer freshServer = new ApiServer(TEST_PORT + 1);
        freshServer.start();
        try {
            JsonNode resp = getJson(TEST_PORT + 1, "/api/v1/orders");
            assertTrue(resp.get("success").asBoolean());
            JsonNode data = resp.get("data");
            assertEquals(0, data.get("leg1").size());
            assertEquals(0, data.get("leg2").size());
        } finally {
            freshServer.stop();
        }
    }

    @Test
    void testActivateCommand() throws Exception {
        JsonNode resp = postJson("/api/v1/strategy/activate");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("activate command sent", resp.get("message").asText());

        // 验证命令进入了队列
        String cmd = server.commandQueue().poll();
        assertEquals("activate", cmd);
    }

    @Test
    void testDeactivateCommand() throws Exception {
        JsonNode resp = postJson("/api/v1/strategy/deactivate");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("deactivate command sent", resp.get("message").asText());

        String cmd = server.commandQueue().poll();
        assertEquals("deactivate", cmd);
    }

    @Test
    void testSquareoffCommand() throws Exception {
        JsonNode resp = postJson("/api/v1/strategy/squareoff");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("squareoff command sent", resp.get("message").asText());

        String cmd = server.commandQueue().poll();
        assertEquals("squareoff", cmd);
    }

    @Test
    void testReloadThresholdsCommand() throws Exception {
        JsonNode resp = postJson("/api/v1/strategy/reload-thresholds");
        assertTrue(resp.get("success").asBoolean());
        assertEquals("reload_thresholds command sent", resp.get("message").asText());

        String cmd = server.commandQueue().poll();
        assertEquals("reload_thresholds", cmd);
    }

    // ---- helpers ----

    private JsonNode getJson(String path) throws Exception {
        return getJson(TEST_PORT, path);
    }

    private JsonNode getJson(int port, String path) throws Exception {
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create("http://localhost:" + port + path))
                .GET().build();
        HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
        assertEquals(200, resp.statusCode());
        return mapper.readTree(resp.body());
    }

    private JsonNode postJson(String path) throws Exception {
        HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create("http://localhost:" + TEST_PORT + path))
                .POST(HttpRequest.BodyPublishers.noBody()).build();
        HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
        return mapper.readTree(resp.body());
    }
}
