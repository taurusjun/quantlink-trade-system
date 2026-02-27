package com.quantlink.trader.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.javalin.Javalin;
import io.javalin.http.Context;
import io.javalin.websocket.WsContext;
import org.eclipse.jetty.server.AbstractConnector;
import org.eclipse.jetty.websocket.api.WriteCallback;

import java.time.Duration;
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
            // WebSocket: 禁用 idle timeout，由应用层 30s ping 保活
            config.jetty.modifyWebSocketServletFactory(factory -> {
                factory.setIdleTimeout(Duration.ZERO);
                factory.setMaxTextMessageSize(256 * 1024);  // 256KB
            });
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
                // 设置 idle timeout: 0=无限，依赖应用层 30s ping 保活
                Duration beforeTimeout = ctx.session.getIdleTimeout();
                ctx.session.setIdleTimeout(Duration.ofSeconds(0));
                Duration afterTimeout = ctx.session.getIdleTimeout();
                wsClients.add(ctx);
                logger.info("[WebSocket] Client connected from " + ctx.session.getRemoteAddress()
                        + ", total: " + wsClients.size()
                        + ", idleTimeout: " + beforeTimeout.toMillis() + "ms → " + afterTimeout.toMillis() + "ms");
            });
            ws.onClose(ctx -> {
                wsClients.remove(ctx);
                logger.info("[WebSocket] Client disconnected from " + ctx.session.getRemoteAddress()
                        + " code=" + ctx.status() + " reason=" + ctx.reason()
                        + ", total: " + wsClients.size()
                        + " [thread=" + Thread.currentThread().getName() + "]");
            });
            ws.onError(ctx -> {
                wsClients.remove(ctx);
                Throwable err = ctx.error();
                String errInfo = err != null
                        ? err.getClass().getName() + ": " + err.getMessage()
                          + (err.getCause() != null ? " caused by " + err.getCause().getClass().getName() + ": " + err.getCause().getMessage() : "")
                        : "null";
                logger.warning("[WebSocket] Client error from " + ctx.session.getRemoteAddress()
                        + ": " + errInfo);
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

        // Jetty ServerConnector 默认 idle timeout = 30s，会断开 WebSocket 底层 TCP 连接（code=1001）。
        // 必须在 app.start() 之后设置（start 前 connectors 尚未创建）。
        try {
            for (var connector : app.jettyServer().server().getConnectors()) {
                if (connector instanceof AbstractConnector ac) {
                    logger.info("[API] Connector idle timeout: " + ac.getIdleTimeout() + "ms → 0 (infinite)");
                    ac.setIdleTimeout(0);
                }
            }
        } catch (Exception e) {
            logger.warning("[API] 设置 connector idle timeout 失败: " + e.getMessage());
        }

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
     *
     * 使用 Jetty 的 sendString(msg, WriteCallback.NOOP) 异步发送，避免阻塞 SnapshotCollector 线程。
     * Javalin 的 ctx.send(String) 内部调用 RemoteEndpoint.sendString() 是同步阻塞的，
     * 如果某个客户端慢会阻塞所有后续客户端的发送，导致快照数据延迟更新。
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

        // 使用 sendString(msg, WriteCallback.NOOP) 异步发送 — 不阻塞当前线程
        for (WsContext ctx : wsClients) {
            try {
                if (ctx.session.isOpen()) {
                    ctx.session.getRemote().sendString(json, WriteCallback.NOOP);
                }
            } catch (Exception e) {
                logger.fine("[WebSocket] send failed: " + e.getClass().getSimpleName());
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
                if (ctx.session.isOpen()) {
                    ctx.session.getRemote().sendString(json, WriteCallback.NOOP);
                }
            } catch (Exception ignored) {
            }
        }
    }

    /** 当前 WebSocket 客户端数。 */
    public int clientCount() {
        return wsClients.size();
    }
}
