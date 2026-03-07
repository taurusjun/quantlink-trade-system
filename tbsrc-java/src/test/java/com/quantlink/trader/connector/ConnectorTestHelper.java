package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;

import java.lang.foreign.MemorySegment;
import java.util.*;

/**
 * Connector 测试辅助工具。
 * <p>
 * 在进程内创建 SHM 段，然后通过 Connector 正常构造函数 attach。
 * C++ 测试中 SHM 段由外部进程创建，Connector 通过正常构造函数连接。
 * Java 没有外部进程，因此在同一进程中 create + attach。
 * <p>
 * 注意: macOS shmseg 限制（默认 8）限制了进程同时附着的 SHM 段数。
 * 为应对此限制，createForTest 采用 create→detach→Connector attach 模式，
 * 数据注入直接通过 Connector 内部 shmMgr 已附着的队列写入（不额外占用名额）。
 * <p>
 * 生命周期: createForTest → startAsync → (use) → stop → destroy
 */
class ConnectorTestHelper {

    /**
     * 创建测试用 Connector，包含预创建的 SHM 段。
     * <p>
     * 流程:
     * 1. 为每个 ExchangeConfig 创建 SHM 段 (md, req, resp, clientStore)
     * 2. 立即 detach（保留 SHM 段 ID，释放附着名额）
     * 3. 使用 Connector 正常构造函数 attach 到这些 SHM 段
     * 4. 返回 TestConnectorBundle（通过 Connector.shmMgr 的队列引用进行数据注入）
     */
    static TestConnectorBundle createForTest(Connector.Config cfg,
                                             Connector.MDCallback mdCb,
                                             Connector.ORSCallback orsCb) {
        if (cfg.exchanges.isEmpty()) {
            throw new IllegalArgumentException("createForTest requires at least one ExchangeConfig");
        }

        // 记录每个交易所的 SHM 配置，用于 destroy 时清理
        List<ExchShmConfig> exchShmConfigs = new ArrayList<>();

        // Phase 1: 创建所有 SHM 段，初始化后立即 detach（释放附着名额给 Connector）
        for (int exchIdx = 0; exchIdx < cfg.exchanges.size(); exchIdx++) {
            Connector.ExchangeConfig exchCfg = cfg.exchanges.get(exchIdx);

            ExchShmConfig shmCfg = new ExchShmConfig();
            shmCfg.clientStoreShmKey = exchCfg.clientStoreShmKey;
            shmCfg.mdShmKey = exchCfg.mdShmKeys.get(0);
            shmCfg.mdShmSize = exchCfg.mdShmSizes.get(0);
            shmCfg.reqShmKey = exchCfg.reqShmKey;
            shmCfg.reqQueueSize = exchCfg.reqQueueSize;
            shmCfg.respShmKey = exchCfg.respShmKey;
            shmCfg.respQueueSize = exchCfg.respQueueSize;

            // 创建并立即 detach — SHM 段保留在系统中
            ClientStore cs = ClientStore.create(exchCfg.clientStoreShmKey, 1L);
            cs.close(); // detach only, no IPC_RMID

            MWMRQueue mdQ = MWMRQueue.create(exchCfg.mdShmKeys.get(0), exchCfg.mdShmSizes.get(0),
                    Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
            mdQ.close();

            MWMRQueue reqQ = MWMRQueue.create(exchCfg.reqShmKey, exchCfg.reqQueueSize,
                    Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
            reqQ.close();

            MWMRQueue respQ = MWMRQueue.create(exchCfg.respShmKey, exchCfg.respQueueSize,
                    Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);
            respQ.close();

            exchShmConfigs.add(shmCfg);
        }

        // Phase 2: Connector attach（占用 4 × N 个附着名额）
        Connector connector = new Connector(mdCb, orsCb, cfg);

        return new TestConnectorBundle(connector, exchShmConfigs);
    }

    /** 每个交易所的 SHM 配置，用于 destroy 时清理。 */
    static class ExchShmConfig {
        int clientStoreShmKey;
        int mdShmKey;
        int mdShmSize;
        int reqShmKey;
        int reqQueueSize;
        int respShmKey;
        int respQueueSize;
    }

    /**
     * 测试用 Bundle: Connector + SHM 配置引用。
     * <p>
     * 数据注入直接使用 Connector.shmMgr 内部已附着的 MWMRQueue（不额外占用 shmseg 名额）。
     * 轮询使用 Connector.startAsync()（利用 shmMgr 的内部轮询线程）。
     * <p>
     * exchIdx = 注册顺序（0, 1, 2...），与 shmMgr 的 mdQueues/respQueues 索引一致。
     */
    static class TestConnectorBundle {
        final Connector connector;
        private final List<ExchShmConfig> exchShmConfigs;

        TestConnectorBundle(Connector connector, List<ExchShmConfig> exchShmConfigs) {
            this.connector = connector;
            this.exchShmConfigs = exchShmConfigs;
        }

        // =================================================================
        // 多交易所方法 (exchIdx = 注册顺序)
        // =================================================================

        /**
         * 向指定交易所的行情队列写入数据（模拟 md_shm_feeder）。
         * 优先使用 Connector.shmMgr 内部已附着的队列。
         * shutdown 后队列被置 null，则临时 open → enqueue → close。
         */
        void enqueueMD(MemorySegment md, int exchIdx) {
            MWMRQueue q = connector.getShmMgr().getMdQueue(exchIdx);
            if (q != null) {
                q.enqueue(md);
            } else {
                ExchShmConfig cfg = exchShmConfigs.get(exchIdx);
                MWMRQueue tmp = MWMRQueue.open(cfg.mdShmKey, cfg.mdShmSize,
                        Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
                tmp.enqueue(md);
                tmp.close();
            }
        }

        /**
         * 向指定交易所的回报队列写入数据（模拟 counter_bridge）。
         * 优先使用 Connector.shmMgr 内部已附着的队列。
         * shutdown 后队列被置 null，则临时 open → enqueue → close。
         */
        void enqueueResponse(MemorySegment resp, int exchIdx) {
            MWMRQueue q = connector.getShmMgr().getRespQueue(exchIdx);
            if (q != null) {
                q.enqueue(resp);
            } else {
                ExchShmConfig cfg = exchShmConfigs.get(exchIdx);
                MWMRQueue tmp = MWMRQueue.open(cfg.respShmKey, cfg.respQueueSize,
                        Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);
                tmp.enqueue(resp);
                tmp.close();
            }
        }

        /** 获取指定交易所的 clientId。exchCode = MD 交易所代码 (如 CHINA_SHFE=57)。 */
        int getClientId(int exchCode) {
            return connector.getClientId(exchCode);
        }

        /**
         * 获取指定交易所的请求队列（用于验证订单路由）。
         * 直接返回 Connector.shmMgr 内部已附着的队列引用。
         */
        MWMRQueue getReqQueue(int exchIdx) {
            return connector.getShmMgr().getReqQueue(exchIdx);
        }

        // =================================================================
        // 向后兼容方法 (委托到 exchIdx=0)
        // =================================================================

        /** 向行情队列写入数据（模拟 md_shm_feeder）— 默认 exchIdx=0。 */
        void enqueueMD(MemorySegment md) {
            enqueueMD(md, 0);
        }

        /** 向回报队列写入数据（模拟 counter_bridge）— 默认 exchIdx=0。 */
        void enqueueResponse(MemorySegment resp) {
            enqueueResponse(resp, 0);
        }

        /** 获取 clientId — 默认第一个交易所。 */
        int getClientId() {
            return connector.getClientId();
        }

        // =================================================================
        // 生命周期
        // =================================================================

        /**
         * 启动 Connector 内部轮询线程。
         * 使用 shmMgr 的真实 MD+ORS 轮询线程，与 C++ Connector::StartAsync() 一致。
         */
        void startAsync() {
            connector.startAsync();
        }

        /** 停止 Connector 轮询线程。 */
        void stop() {
            connector.stop();
        }

        /** 停止轮询 + 分离所有 SHM 段（不删除）。 */
        void close() {
            connector.close();
        }

        /** 停止轮询 + 删除所有预创建的 SHM 段。 */
        void destroy() {
            connector.close();

            // connector.close() 已 detach 所有段，现在临时 open 以标记 IPC_RMID 删除
            for (ExchShmConfig cfg : exchShmConfigs) {
                destroyQueue(cfg.mdShmKey, cfg.mdShmSize,
                        Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
                destroyQueue(cfg.reqShmKey, cfg.reqQueueSize,
                        Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
                destroyQueue(cfg.respShmKey, cfg.respQueueSize,
                        Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);
                destroyClientStore(cfg.clientStoreShmKey);
            }
        }

        private static void destroyQueue(int key, int queueSize, long dataSize, long elemSize) {
            try {
                MWMRQueue q = MWMRQueue.open(key, queueSize, dataSize, elemSize);
                q.destroy();
            } catch (Exception e) {
                // 段可能已被删除
            }
        }

        private static void destroyClientStore(int key) {
            try {
                ClientStore cs = ClientStore.open(key);
                cs.destroy();
            } catch (Exception e) {
                // 段可能已被删除
            }
        }
    }
}
