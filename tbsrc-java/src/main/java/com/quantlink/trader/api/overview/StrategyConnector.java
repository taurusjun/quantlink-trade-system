package com.quantlink.trader.api.overview;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.quantlink.trader.api.DashboardSnapshot;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.WebSocket;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.*;
import java.util.function.BiConsumer;
import java.util.logging.Logger;

/**
 * WebSocket 客户端连接各 trader 的 /ws 端点，实时接收 DashboardSnapshot 推送。
 *
 * 扫描端口 9201-9210，每个端口一个 WebSocket 连接。
 * 断线自动重连（每 5 秒重试未连接端口）。
 */
public class StrategyConnector {

    private static final Logger logger = Logger.getLogger(StrategyConnector.class.getName());
    private static final ObjectMapper mapper = new ObjectMapper();

    private static final int PORT_START = 9201;
    private static final int PORT_END = 9210;
    private static final int RECONNECT_INTERVAL_SEC = 5;

    // 端口 → WebSocket 连接
    private final Map<Integer, WebSocket> connections = new ConcurrentHashMap<>();
    // 端口 → 最新快照
    private final Map<Integer, DashboardSnapshot> snapshots = new ConcurrentHashMap<>();
    // 端口 → 连接状态
    private final Map<Integer, ConnectionStatus> statuses = new ConcurrentHashMap<>();

    private final HttpClient httpClient;
    private ScheduledExecutorService reconnectExecutor;
    private volatile boolean running;

    // 收到快照时的回调: (port, snapshot) → 触发聚合
    private BiConsumer<Integer, DashboardSnapshot> onSnapshotReceived;

    public enum ConnectionStatus {
        CONNECTED,    // 运行中（绿色）
        DISCONNECTED, // 未连接（黄色）
        NO_PROCESS    // 无进程（灰色）
    }

    public StrategyConnector() {
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(3))
                .build();
    }

    /**
     * 设置快照接收回调。
     * 每次从 trader 收到推送后调用，用于触发聚合和转发。
     */
    public void setOnSnapshotReceived(BiConsumer<Integer, DashboardSnapshot> callback) {
        this.onSnapshotReceived = callback;
    }

    /**
     * 启动连接器：立即尝试连接所有端口，并启动重连定时器。
     */
    public void start() {
        running = true;
        // 初始化所有端口状态
        for (int port = PORT_START; port <= PORT_END; port++) {
            statuses.put(port, ConnectionStatus.NO_PROCESS);
        }

        // 立即尝试连接
        connectAll();

        // 重连定时器
        reconnectExecutor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "strategy-reconnector");
            t.setDaemon(true);
            return t;
        });
        reconnectExecutor.scheduleAtFixedRate(this::reconnectDisconnected,
                RECONNECT_INTERVAL_SEC, RECONNECT_INTERVAL_SEC, TimeUnit.SECONDS);

        logger.info("[StrategyConnector] 已启动，扫描端口 " + PORT_START + "-" + PORT_END);
    }

    /**
     * 停止连接器：关闭所有 WebSocket 连接。
     */
    public void stop() {
        running = false;
        if (reconnectExecutor != null) {
            reconnectExecutor.shutdownNow();
        }
        for (var entry : connections.entrySet()) {
            try {
                entry.getValue().sendClose(WebSocket.NORMAL_CLOSURE, "shutdown")
                        .orTimeout(2, TimeUnit.SECONDS);
            } catch (Exception ignored) {}
        }
        connections.clear();
        snapshots.clear();
        logger.info("[StrategyConnector] 已停止");
    }

    /**
     * 获取所有端口的最新快照。
     */
    public Map<Integer, DashboardSnapshot> getSnapshots() {
        return new ConcurrentHashMap<>(snapshots);
    }

    /**
     * 获取所有端口的连接状态。
     */
    public Map<Integer, ConnectionStatus> getStatuses() {
        return new ConcurrentHashMap<>(statuses);
    }

    /**
     * 获取已连接的端口列表。
     */
    public java.util.List<Integer> getConnectedPorts() {
        java.util.List<Integer> ports = new java.util.ArrayList<>();
        for (var entry : statuses.entrySet()) {
            if (entry.getValue() == ConnectionStatus.CONNECTED) {
                ports.add(entry.getKey());
            }
        }
        return ports;
    }

    // ---- 内部方法 ----

    private void connectAll() {
        for (int port = PORT_START; port <= PORT_END; port++) {
            if (!connections.containsKey(port)) {
                connectToPort(port);
            }
        }
    }

    private void reconnectDisconnected() {
        if (!running) return;
        for (int port = PORT_START; port <= PORT_END; port++) {
            ConnectionStatus status = statuses.get(port);
            if (status != ConnectionStatus.CONNECTED && !connections.containsKey(port)) {
                connectToPort(port);
            }
        }
    }

    private void connectToPort(int port) {
        if (!running) return;
        URI uri = URI.create("ws://localhost:" + port + "/ws");
        try {
            httpClient.newWebSocketBuilder()
                    .connectTimeout(Duration.ofSeconds(3))
                    .buildAsync(uri, new TraderWebSocketListener(port))
                    .whenComplete((ws, error) -> {
                        if (error != null) {
                            // 连接失败，标记为无进程
                            statuses.put(port, ConnectionStatus.NO_PROCESS);
                        }
                    });
        } catch (Exception e) {
            statuses.put(port, ConnectionStatus.NO_PROCESS);
        }
    }

    /**
     * WebSocket 监听器 — 处理每个 trader 端口的消息。
     */
    private class TraderWebSocketListener implements WebSocket.Listener {
        private final int port;
        private final StringBuilder buffer = new StringBuilder();

        TraderWebSocketListener(int port) {
            this.port = port;
        }

        @Override
        public void onOpen(WebSocket webSocket) {
            connections.put(port, webSocket);
            statuses.put(port, ConnectionStatus.CONNECTED);
            logger.info("[StrategyConnector] 已连接 port " + port);
            webSocket.request(1);
        }

        @Override
        public CompletionStage<?> onText(WebSocket webSocket, CharSequence data, boolean last) {
            buffer.append(data);
            if (last) {
                processMessage(buffer.toString());
                buffer.setLength(0);
            }
            webSocket.request(1);
            return null;
        }

        @Override
        public CompletionStage<?> onClose(WebSocket webSocket, int statusCode, String reason) {
            connections.remove(port);
            statuses.put(port, ConnectionStatus.DISCONNECTED);
            logger.info("[StrategyConnector] 断开 port " + port + " (code=" + statusCode + ")");
            return null;
        }

        @Override
        public void onError(WebSocket webSocket, Throwable error) {
            connections.remove(port);
            statuses.put(port, ConnectionStatus.DISCONNECTED);
            logger.warning("[StrategyConnector] 错误 port " + port + ": " + error.getMessage());
        }

        private void processMessage(String json) {
            try {
                JsonNode root = mapper.readTree(json);
                String type = root.has("type") ? root.get("type").asText() : "";

                if ("dashboard_update".equals(type) && root.has("data")) {
                    DashboardSnapshot snap = mapper.treeToValue(root.get("data"), DashboardSnapshot.class);
                    snapshots.put(port, snap);

                    if (onSnapshotReceived != null) {
                        onSnapshotReceived.accept(port, snap);
                    }
                }
                // ping 消息忽略
            } catch (Exception e) {
                logger.warning("[StrategyConnector] 解析消息失败 port " + port + ": " + e.getMessage());
            }
        }
    }
}
