package com.quantlink.trader.api;

import com.quantlink.trader.strategy.PairwiseArbStrategy;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * 每秒从 strategy 采集快照并推送到 ApiServer。
 * 对齐: tbsrc-golang/cmd/trader/main.go 中 ticker 每秒调用 CollectSnapshot + UpdateSnapshot
 */
public class SnapshotCollector {

    private static final Logger logger = Logger.getLogger(SnapshotCollector.class.getName());

    private final PairwiseArbStrategy strategy;
    private final ApiServer apiServer;
    private ScheduledExecutorService executor;

    public SnapshotCollector(PairwiseArbStrategy strategy, ApiServer apiServer) {
        this.strategy = strategy;
        this.apiServer = apiServer;
    }

    /**
     * 启动定时采集（每秒一次）。
     */
    public void start() {
        executor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "snapshot-collector");
            t.setDaemon(true);
            return t;
        });

        executor.scheduleAtFixedRate(() -> {
            try {
                DashboardSnapshot snap = DashboardSnapshot.collect(strategy);
                apiServer.updateSnapshot(snap);
            } catch (Exception e) {
                logger.warning("[SnapshotCollector] 采集异常: " + e.getMessage());
            }
        }, 1, 1, TimeUnit.SECONDS);

        logger.info("[SnapshotCollector] 已启动，每秒采集快照");
    }

    /**
     * 停止采集。
     */
    public void stop() {
        if (executor != null) {
            executor.shutdownNow();
            logger.info("[SnapshotCollector] 已停止");
        }
    }
}
