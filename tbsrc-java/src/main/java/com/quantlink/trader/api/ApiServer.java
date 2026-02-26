package com.quantlink.trader.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.javalin.Javalin;
import io.javalin.http.Context;
import io.javalin.websocket.WsContext;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicReference;
import java.util.logging.Logger;

/**
 * HTTP + WebSocket API 服务。
 * 对齐: tbsrc-golang/pkg/api/server.go — Server
 *       tbsrc-golang/pkg/api/handlers.go — 7 个 REST 端点
 *       tbsrc-golang/pkg/api/websocket.go — WebSocketHub
 *
 * Javalin 内嵌到 TraderMain 进程，端口 9201。
 */
public class ApiServer {

    private static final Logger logger = Logger.getLogger(ApiServer.class.getName());
    private static final ObjectMapper mapper = new ObjectMapper();

    private final int port;
    private Javalin app;

    // 对齐 Go: atomic.Pointer[DashboardSnapshot]
    private final AtomicReference<DashboardSnapshot> snapshot = new AtomicReference<>();

    // 对齐 Go: cmdChan chan Command (buffered 10)
    private final LinkedBlockingQueue<String> cmdQueue = new LinkedBlockingQueue<>(10);

    // 对齐 Go: WebSocketHub
    private final Set<WsContext> wsClients = ConcurrentHashMap.newKeySet();

    // 订单历史追踪器 — 对齐 Go: leg1History, leg2History
    private final OrderHistoryTracker leg1History = new OrderHistoryTracker(50);
    private final OrderHistoryTracker leg2History = new OrderHistoryTracker(50);

    // 心跳定时器
    private ScheduledExecutorService heartbeatExecutor;

    public ApiServer(int port) {
        this.port = port;
    }

    /**
     * 启动 HTTP + WebSocket 服务。
     * 对齐: tbsrc-golang/pkg/api/server.go:Start()
     */
    public void start() {
        app = Javalin.create(config -> {
            // 静态文件 — 对齐 Go: http.FileServer
            config.staticFiles.add("/web");
            // CORS — 对齐 Go: Access-Control-Allow-Origin: *
            config.bundledPlugins.enableCors(cors -> cors.addRule(rule -> rule.anyHost()));
        });

        // ---- REST 端点 (对齐 Go handlers.go) ----
        app.get("/api/v1/health", this::handleHealth);
        app.get("/api/v1/status", this::handleStatus);
        app.get("/api/v1/orders", this::handleOrders);
        app.post("/api/v1/strategy/activate", this::handleActivate);
        app.post("/api/v1/strategy/deactivate", this::handleDeactivate);
        app.post("/api/v1/strategy/squareoff", this::handleSquareoff);
        app.post("/api/v1/strategy/reload-thresholds", this::handleReloadThresholds);

        // ---- WebSocket (对齐 Go websocket.go) ----
        app.ws("/ws", ws -> {
            ws.onConnect(ctx -> {
                wsClients.add(ctx);
                logger.info("[WebSocket] Client connected, total: " + wsClients.size());
            });
            ws.onClose(ctx -> {
                wsClients.remove(ctx);
                logger.info("[WebSocket] Client disconnected, total: " + wsClients.size());
            });
            ws.onError(ctx -> {
                wsClients.remove(ctx);
                logger.warning("[WebSocket] Client error: " + ctx.error());
            });
            ws.onMessage(ctx -> {
                // 对齐 Go: pong 或其他客户端消息，忽略
            });
        });

        // 心跳 — 对齐 Go: 30 秒 ping
        heartbeatExecutor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "ws-heartbeat");
            t.setDaemon(true);
            return t;
        });
        heartbeatExecutor.scheduleAtFixedRate(this::sendPing, 30, 30, TimeUnit.SECONDS);

        app.start(port);
        logger.info("[API] Server starting on :" + port);
    }

    /**
     * 优雅关闭。
     * 对齐: tbsrc-golang/pkg/api/server.go:Stop()
     */
    public void stop() {
        if (heartbeatExecutor != null) {
            heartbeatExecutor.shutdownNow();
        }
        // 关闭所有 WebSocket 连接
        for (WsContext ctx : wsClients) {
            try { ctx.session.close(); } catch (Exception ignored) {}
        }
        wsClients.clear();
        if (app != null) {
            app.stop();
        }
        logger.info("[API] Server stopped");
    }

    /**
     * 原子更新快照并广播到 WebSocket。
     * 对齐: tbsrc-golang/pkg/api/server.go:UpdateSnapshot()
     * 使用 OrderHistoryTracker 合并订单历史。
     */
    public void updateSnapshot(DashboardSnapshot snap) {
        // 合并订单历史
        snap.leg1.orders = leg1History.update(snap.leg1.orders);
        snap.leg2.orders = leg2History.update(snap.leg2.orders);
        snapshot.set(snap);
        broadcast(snap);
    }

    /**
     * 返回命令队列，供 TraderMain 主线程消费。
     * 对齐: tbsrc-golang/pkg/api/server.go:CommandChan()
     */
    public LinkedBlockingQueue<String> commandQueue() {
        return cmdQueue;
    }

    // =======================================================================
    //  REST Handlers — 对齐 Go handlers.go
    // =======================================================================

    /** GET /api/v1/health — 对齐 Go handleHealth */
    private void handleHealth(Context ctx) {
        ctx.json(Map.of(
            "success", true,
            "message", "ok",
            "data", Map.of("ws_clients", wsClients.size())
        ));
    }

    /** GET /api/v1/status — 对齐 Go handleStatus */
    private void handleStatus(Context ctx) {
        DashboardSnapshot snap = snapshot.get();
        if (snap == null) {
            ctx.json(Map.of("success", true, "message", "no snapshot yet"));
            return;
        }
        ctx.json(Map.of("success", true, "data", snap));
    }

    /** GET /api/v1/orders — 对齐 Go handleOrders */
    private void handleOrders(Context ctx) {
        DashboardSnapshot snap = snapshot.get();
        if (snap == null) {
            ctx.json(Map.of(
                "success", true,
                "data", Map.of("leg1", List.of(), "leg2", List.of())
            ));
            return;
        }
        ctx.json(Map.of(
            "success", true,
            "data", Map.of(
                "leg1", snap.leg1.orders,
                "leg2", snap.leg2.orders
            )
        ));
    }

    /** POST /api/v1/strategy/activate — 对齐 Go handleActivate */
    private void handleActivate(Context ctx) {
        sendCommand(ctx, "activate");
    }

    /** POST /api/v1/strategy/deactivate — 对齐 Go handleDeactivate */
    private void handleDeactivate(Context ctx) {
        sendCommand(ctx, "deactivate");
    }

    /** POST /api/v1/strategy/squareoff — 对齐 Go handleSquareoff */
    private void handleSquareoff(Context ctx) {
        sendCommand(ctx, "squareoff");
    }

    /** POST /api/v1/strategy/reload-thresholds — 对齐 Go handleReloadThresholds */
    private void handleReloadThresholds(Context ctx) {
        sendCommand(ctx, "reload_thresholds");
    }

    /**
     * 发送命令到 cmdQueue — 对齐 Go 的 select/default 非阻塞写入。
     */
    private void sendCommand(Context ctx, String command) {
        boolean sent = cmdQueue.offer(command);
        if (sent) {
            ctx.json(Map.of("success", true, "message", command + " command sent"));
        } else {
            ctx.status(503).json(Map.of("success", false, "message", "command channel full"));
        }
    }

    // =======================================================================
    //  WebSocket 广播 — 对齐 Go websocket.go
    // =======================================================================

    /**
     * 向所有客户端广播快照。
     * 对齐: tbsrc-golang/pkg/api/websocket.go:Broadcast()
     */
    private void broadcast(DashboardSnapshot snap) {
        if (wsClients.isEmpty()) return;

        Map<String, Object> msg = Map.of(
            "type", "dashboard_update",
            "timestamp", Instant.now().toString(),
            "data", snap
        );

        String json;
        try {
            json = mapper.writeValueAsString(msg);
        } catch (Exception e) {
            logger.warning("[WebSocket] JSON serialize error: " + e.getMessage());
            return;
        }

        for (WsContext ctx : wsClients) {
            try {
                ctx.send(json);
            } catch (Exception e) {
                logger.warning("[WebSocket] Send error: " + e.getMessage());
                wsClients.remove(ctx);
            }
        }
    }

    /**
     * 发送心跳 ping。
     * 对齐: tbsrc-golang/pkg/api/websocket.go 30 秒心跳
     */
    private void sendPing() {
        if (wsClients.isEmpty()) return;

        Map<String, Object> msg = Map.of(
            "type", "ping",
            "timestamp", Instant.now().toString()
        );

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

    /** 当前 WebSocket 客户端数。 */
    public int clientCount() {
        return wsClients.size();
    }
}
