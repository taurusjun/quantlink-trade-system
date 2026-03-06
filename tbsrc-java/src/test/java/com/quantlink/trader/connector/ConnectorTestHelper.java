package com.quantlink.trader.connector;

import com.quantlink.trader.shm.*;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;

/**
 * Connector 测试辅助工具。
 * <p>
 * 在进程内创建 SHM 段，然后通过 Connector 正常构造函数 attach。
 * C++ 测试中 SHM 段由外部进程创建，Connector 通过正常构造函数连接。
 * Java 没有外部进程，因此在同一进程中 create + attach。
 * <p>
 * 生命周期: createForTest → startAsync → (use) → stop → destroy
 */
class ConnectorTestHelper {

    /**
     * 创建测试用 Connector，包含预创建的 SHM 段。
     * <p>
     * 流程:
     * 1. 为每个 ExchangeConfig 创建 SHM 段 (md, req, resp, clientStore)
     * 2. 使用 Connector 正常构造函数 attach 到这些 SHM 段
     * 3. 返回 TestConnectorBundle（Connector + 额外队列引用 + 清理方法）
     */
    static TestConnectorBundle createForTest(Connector.Config cfg,
                                             Connector.MDCallback mdCb,
                                             Connector.ORSCallback orsCb) {
        if (cfg.exchanges.isEmpty()) {
            throw new IllegalArgumentException("createForTest requires at least one ExchangeConfig");
        }

        Connector.ExchangeConfig exchCfg = cfg.exchanges.get(0);

        // 预创建 SHM 段（模拟外部进程创建 SHM）
        ClientStore clientStore = ClientStore.create(exchCfg.clientStoreShmKey, 1L);

        MWMRQueue mdQueue = MWMRQueue.create(exchCfg.mdShmKeys.get(0), exchCfg.mdShmSizes.get(0),
                Types.MARKET_UPDATE_NEW_SIZE, Types.QUEUE_ELEM_MD_SIZE);
        MWMRQueue reqQueue = MWMRQueue.create(exchCfg.reqShmKey, exchCfg.reqQueueSize,
                Types.REQUEST_MSG_SIZE, Types.QUEUE_ELEM_REQ_SIZE);
        MWMRQueue respQueue = MWMRQueue.create(exchCfg.respShmKey, exchCfg.respQueueSize,
                Types.RESPONSE_MSG_SIZE, Types.QUEUE_ELEM_RESP_SIZE);

        // 使用正常构造函数 — Connector 内部的 shmMgr 会 shmget attach 到已创建的段
        Connector connector = new Connector(mdCb, orsCb, cfg);

        return new TestConnectorBundle(connector, mdQueue, reqQueue, respQueue, clientStore);
    }

    /**
     * 测试用 Bundle: Connector + 预创建的 SHM 队列引用。
     * <p>
     * 测试通过 enqueueMD/enqueueResponse 模拟外部进程写入数据，
     * 通过 destroy() 清理 SHM 段。
     */
    static class TestConnectorBundle {
        final Connector connector;
        private final MWMRQueue testMdQueue;
        private final MWMRQueue testReqQueue;
        private final MWMRQueue testRespQueue;
        private final ClientStore testClientStore;
        private volatile boolean polling;
        private Thread mdPollThread;
        private Thread respPollThread;

        TestConnectorBundle(Connector connector,
                            MWMRQueue mdQueue, MWMRQueue reqQueue,
                            MWMRQueue respQueue, ClientStore clientStore) {
            this.connector = connector;
            this.testMdQueue = mdQueue;
            this.testReqQueue = reqQueue;
            this.testRespQueue = respQueue;
            this.testClientStore = clientStore;
        }

        /** 向行情队列写入数据（模拟 md_shm_feeder）。 */
        void enqueueMD(MemorySegment md) {
            testMdQueue.enqueue(md);
        }

        /** 向回报队列写入数据（模拟 counter_bridge）。 */
        void enqueueResponse(MemorySegment resp) {
            testRespQueue.enqueue(resp);
        }

        /**
         * 启动测试轮询线程。
         * <p>
         * 使用独立线程轮询预创建的 MWMRQueue，dequeue 后调用 Connector 的
         * package-private handleUpdates/handleOrderResponse（与 ShmMgr 轮询等价）。
         */
        void startAsync() {
            polling = true;

            MemorySegment mdBuf = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            mdPollThread = new Thread(() -> {
                while (polling) {
                    if (testMdQueue.dequeue(mdBuf)) {
                        connector.handleUpdates(mdBuf);
                    } else {
                        Thread.onSpinWait();
                    }
                }
            }, "test-connector-md-poll");
            mdPollThread.setDaemon(true);
            mdPollThread.start();

            MemorySegment respBuf = Arena.global().allocate(Types.RESPONSE_MSG_LAYOUT);
            respPollThread = new Thread(() -> {
                while (polling) {
                    if (testRespQueue.dequeue(respBuf)) {
                        connector.handleOrderResponse(respBuf, 0);
                    } else {
                        Thread.onSpinWait();
                    }
                }
            }, "test-connector-ors-poll");
            respPollThread.setDaemon(true);
            respPollThread.start();
        }

        /** 停止轮询线程。 */
        void stop() {
            polling = false;
            joinThread(mdPollThread);
            joinThread(respPollThread);
        }

        /** 获取 clientId。 */
        int getClientId() {
            return connector.getClientId();
        }

        /** 停止轮询 + 分离所有 SHM 段（不删除）。 */
        void close() {
            stop();
            connector.close();
        }

        /** 停止轮询 + 删除所有预创建的 SHM 段。 */
        void destroy() {
            stop();
            connector.close();
            testMdQueue.destroy();
            testReqQueue.destroy();
            testRespQueue.destroy();
            testClientStore.destroy();
        }

        private static void joinThread(Thread t) {
            if (t != null) {
                try { t.join(5000); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
            }
        }
    }
}
