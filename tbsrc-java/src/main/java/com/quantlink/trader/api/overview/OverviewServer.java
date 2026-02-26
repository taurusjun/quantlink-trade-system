package com.quantlink.trader.api.overview;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.quantlink.trader.api.DashboardSnapshot;
import io.javalin.Javalin;
import io.javalin.http.Context;
import io.javalin.websocket.WsContext;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import java.util.logging.Logger;

/**
 * 聚合服务 — Overview 综合监控页面的后端。
 *
 * 独立运行在端口 8080:
 * - 作为 WebSocket 客户端连接各 trader /ws（StrategyConnector）
 * - 聚合策略列表、持仓、挂单、成交数据（OverviewSnapshot）
 * - 每次收到 trader 推送即转发聚合数据给前端 WebSocket
 * - REST API 转发控制命令到对应 trader
 */
public class OverviewServer {

    private static final Logger logger = Logger.getLogger(OverviewServer.class.getName());
    private static final ObjectMapper mapper = new ObjectMapper();

    private static final String COUNTER_BRIDGE_URL = "http://localhost:8082/account";

    private final int port;
    private Javalin app;
    private StrategyConnector connector;
    private HttpClient httpClient;

    // 前端 WebSocket 客户端
    private final Set<WsContext> wsClients = ConcurrentHashMap.newKeySet();

    // 心跳定时器
    private ScheduledExecutorService heartbeatExecutor;

    // 资金查询定时器 + 缓存
    private ScheduledExecutorService accountQueryExecutor;
    private volatile List<OverviewSnapshot.AccountRow> cachedAccounts = List.of();

    public OverviewServer(int port) {
        this.port = port;
    }

    /**
     * 启动 OverviewServer。
     */
    public void start() {
        httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .build();

        // 启动 StrategyConnector
        connector = new StrategyConnector();
        connector.setOnSnapshotReceived(this::onTraderSnapshot);
        connector.start();

        // 启动 Javalin
        app = Javalin.create(config -> {
            config.staticFiles.add("/web-overview");
            config.bundledPlugins.enableCors(cors -> cors.addRule(rule -> rule.anyHost()));
        });

        // ---- REST 端点 ----
        app.get("/api/v1/overview", this::handleOverview);
        app.get("/api/v1/positions", this::handlePositions);
        app.get("/api/v1/all-orders", this::handleAllOrders);
        app.get("/api/v1/all-fills", this::handleAllFills);
        app.post("/api/v1/command/{port}/{action}", this::handleCommand);
        app.post("/api/v1/stop-all", this::handleStopAll);

        // ---- 前端 WebSocket ----
        app.ws("/ws", ws -> {
            ws.onConnect(ctx -> {
                wsClients.add(ctx);
                logger.info("[OverviewWS] Client connected, total: " + wsClients.size());
                // 立即发送当前聚合数据
                sendCurrentSnapshot(ctx);
            });
            ws.onClose(ctx -> {
                wsClients.remove(ctx);
                logger.info("[OverviewWS] Client disconnected, total: " + wsClients.size());
            });
            ws.onError(ctx -> {
                wsClients.remove(ctx);
            });
            ws.onMessage(ctx -> {
                // 忽略客户端消息
            });
        });

        // 心跳 30 秒 ping
        heartbeatExecutor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "overview-heartbeat");
            t.setDaemon(true);
            return t;
        });
        heartbeatExecutor.scheduleAtFixedRate(this::sendPing, 30, 30, TimeUnit.SECONDS);

        // 资金查询 — 每 10 秒从 counter_bridge 查询
        accountQueryExecutor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "account-query");
            t.setDaemon(true);
            return t;
        });
        accountQueryExecutor.scheduleAtFixedRate(this::queryCounterBridgeAccount, 3, 10, TimeUnit.SECONDS);

        app.start(port);
        logger.info("[OverviewServer] 已启动 (port " + port + ")");
    }

    /**
     * 停止 OverviewServer。
     */
    public void stop() {
        if (accountQueryExecutor != null) accountQueryExecutor.shutdownNow();
        if (heartbeatExecutor != null) heartbeatExecutor.shutdownNow();
        if (connector != null) connector.stop();
        for (WsContext ctx : wsClients) {
            try { ctx.session.close(); } catch (Exception ignored) {}
        }
        wsClients.clear();
        if (app != null) app.stop();
        logger.info("[OverviewServer] 已停止");
    }

    // =======================================================================
    //  推送驱动: 收到 trader 快照后聚合并转发
    // =======================================================================

    private void onTraderSnapshot(int port, DashboardSnapshot snap) {
        // 每次收到任一 trader 推送 → 重新聚合 → 推送给前端
        OverviewSnapshot overview = OverviewSnapshot.aggregate(
                connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
        broadcastOverview(overview);
    }

    private void broadcastOverview(OverviewSnapshot overview) {
        if (wsClients.isEmpty()) return;

        Map<String, Object> msg = Map.of(
                "type", "overview_update",
                "timestamp", Instant.now().toString(),
                "data", overview
        );

        String json;
        try {
            json = mapper.writeValueAsString(msg);
        } catch (Exception e) {
            logger.warning("[OverviewWS] JSON 序列化失败: " + e.getMessage());
            return;
        }

        for (WsContext ctx : wsClients) {
            try {
                ctx.send(json);
            } catch (Exception e) {
                wsClients.remove(ctx);
            }
        }
    }

    private void sendCurrentSnapshot(WsContext ctx) {
        try {
            OverviewSnapshot overview = OverviewSnapshot.aggregate(
                    connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
            Map<String, Object> msg = Map.of(
                    "type", "overview_update",
                    "timestamp", Instant.now().toString(),
                    "data", overview
            );
            ctx.send(mapper.writeValueAsString(msg));
        } catch (Exception e) {
            logger.warning("[OverviewWS] 发送初始数据失败: " + e.getMessage());
        }
    }

    // =======================================================================
    //  REST 端点
    // =======================================================================

    /** GET /api/v1/overview — 完整聚合快照 */
    private void handleOverview(Context ctx) {
        OverviewSnapshot overview = OverviewSnapshot.aggregate(
                connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
        ctx.json(Map.of("success", true, "data", overview));
    }

    /** GET /api/v1/positions — 全局持仓 */
    private void handlePositions(Context ctx) {
        OverviewSnapshot overview = OverviewSnapshot.aggregate(
                connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
        ctx.json(Map.of("success", true, "data", overview.positions));
    }

    /** GET /api/v1/all-orders — 全局挂单 */
    private void handleAllOrders(Context ctx) {
        OverviewSnapshot overview = OverviewSnapshot.aggregate(
                connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
        ctx.json(Map.of("success", true, "data", overview.orders));
    }

    /** GET /api/v1/all-fills — 全局成交 */
    private void handleAllFills(Context ctx) {
        OverviewSnapshot overview = OverviewSnapshot.aggregate(
                connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
        ctx.json(Map.of("success", true, "data", overview.fills));
    }

    /**
     * POST /api/v1/command/{port}/{action} — 转发控制命令到对应 trader。
     * 示例: POST /api/v1/command/9201/activate
     */
    private void handleCommand(Context ctx) {
        String portStr = ctx.pathParam("port");
        String action = ctx.pathParam("action");
        int targetPort;
        try {
            targetPort = Integer.parseInt(portStr);
        } catch (NumberFormatException e) {
            ctx.status(400).json(Map.of("success", false, "message", "invalid port"));
            return;
        }

        // 映射 action → trader REST 端点
        String traderPath = switch (action) {
            case "activate" -> "/api/v1/strategy/activate";
            case "deactivate" -> "/api/v1/strategy/deactivate";
            case "squareoff" -> "/api/v1/strategy/squareoff";
            case "reload_thresholds", "reload-thresholds" -> "/api/v1/strategy/reload-thresholds";
            default -> null;
        };

        if (traderPath == null) {
            ctx.status(400).json(Map.of("success", false, "message", "unknown action: " + action));
            return;
        }

        forwardCommand(ctx, targetPort, traderPath);
    }

    /**
     * POST /api/v1/stop-all — 向所有已连接 trader 发送 deactivate + squareoff。
     */
    private void handleStopAll(Context ctx) {
        List<Integer> ports = connector.getConnectedPorts();
        int success = 0;
        int failed = 0;

        for (int p : ports) {
            try {
                forwardToTrader(p, "/api/v1/strategy/deactivate");
                forwardToTrader(p, "/api/v1/strategy/squareoff");
                success++;
            } catch (Exception e) {
                failed++;
                logger.warning("[OverviewServer] stopAll 失败 port " + p + ": " + e.getMessage());
            }
        }

        ctx.json(Map.of(
                "success", true,
                "message", String.format("stopAll: %d 成功, %d 失败", success, failed)
        ));
    }

    // =======================================================================
    //  命令转发
    // =======================================================================

    private void forwardCommand(Context ctx, int targetPort, String path) {
        try {
            String response = forwardToTrader(targetPort, path);
            ctx.json(Map.of("success", true, "message", "forwarded to " + targetPort, "response", response));
        } catch (Exception e) {
            ctx.status(502).json(Map.of("success", false, "message", "forward failed: " + e.getMessage()));
        }
    }

    private String forwardToTrader(int targetPort, String path) throws Exception {
        URI uri = URI.create("http://localhost:" + targetPort + path);
        HttpRequest request = HttpRequest.newBuilder()
                .uri(uri)
                .POST(HttpRequest.BodyPublishers.noBody())
                .timeout(Duration.ofSeconds(5))
                .build();
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        return response.body();
    }

    // =======================================================================
    //  心跳
    // =======================================================================

    private void sendPing() {
        if (wsClients.isEmpty()) return;
        Map<String, Object> msg = Map.of("type", "ping", "timestamp", Instant.now().toString());
        String json;
        try {
            json = mapper.writeValueAsString(msg);
        } catch (Exception e) {
            return;
        }
        for (WsContext ctx : wsClients) {
            try {
                ctx.send(json);
            } catch (Exception e) {
                wsClients.remove(ctx);
            }
        }
    }

    // =======================================================================
    //  资金查询 — 定时从 counter_bridge HTTP 获取
    // =======================================================================

    private void queryCounterBridgeAccount() {
        try {
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(COUNTER_BRIDGE_URL))
                    .GET()
                    .timeout(Duration.ofSeconds(3))
                    .build();
            HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() != 200) {
                return;
            }

            JsonNode root = mapper.readTree(response.body());
            if (!root.has("success") || !root.get("success").asBoolean()) {
                return;
            }

            OverviewSnapshot.AccountRow row = new OverviewSnapshot.AccountRow();
            row.broker = root.has("broker") ? root.get("broker").asText() : "";
            row.accountId = root.has("account_id") ? root.get("account_id").asText() : "";
            row.totalAsset = root.has("balance") ? root.get("balance").asDouble() : 0;
            row.availCash = root.has("available") ? root.get("available").asDouble() : 0;
            row.margin = root.has("margin") ? root.get("margin").asDouble() : 0;
            row.closeProfit = root.has("close_profit") ? root.get("close_profit").asDouble() : 0;
            row.positionProfit = root.has("position_profit") ? root.get("position_profit").asDouble() : 0;
            row.commission = root.has("commission") ? root.get("commission").asDouble() : 0;
            row.riskPercent = row.totalAsset > 0 ? (row.margin / row.totalAsset * 100.0) : 0;

            cachedAccounts = List.of(row);

            // 资金更新后也推送给前端
            OverviewSnapshot overview = OverviewSnapshot.aggregate(
                    connector.getSnapshots(), connector.getStatuses(), cachedAccounts);
            broadcastOverview(overview);

        } catch (java.net.ConnectException e) {
            // counter_bridge 未启动，静默忽略
        } catch (Exception e) {
            logger.fine("[AccountQuery] 查询失败: " + e.getMessage());
        }
    }

    // =======================================================================
    //  独立启动入口
    // =======================================================================

    /**
     * 独立 main 入口 — 可作为单独进程运行。
     * 用法: java -cp trader.jar:lib/* com.quantlink.trader.api.overview.OverviewServer
     */
    public static void main(String[] args) {
        System.setProperty("java.util.logging.SimpleFormatter.format",
                "%1$tF %1$tT.%1$tL %4$s %5$s%6$s%n");

        int port = 8080;
        if (args.length > 0) {
            try {
                port = Integer.parseInt(args[0]);
            } catch (NumberFormatException e) {
                System.err.println("Usage: OverviewServer [port]");
                System.exit(1);
            }
        }

        OverviewServer server = new OverviewServer(port);
        server.start();

        // 关闭钩子
        Runtime.getRuntime().addShutdownHook(new Thread(server::stop));

        logger.info("[OverviewServer] 独立模式运行中，端口 " + port);
        logger.info("[OverviewServer] 连接 trader 端口 9201-9210");

        // 阻塞等待
        try {
            Thread.currentThread().join();
        } catch (InterruptedException e) {
            server.stop();
        }
    }
}
