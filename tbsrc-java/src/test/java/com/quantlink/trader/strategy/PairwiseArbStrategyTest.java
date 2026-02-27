package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.core.Watch;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.PrintWriter;
import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * PairwiseArbStrategy 单元测试。
 */
class PairwiseArbStrategyTest {

    private MockCommonClient client;
    private SimConfig simConfig;
    private Instrument instru1;
    private Instrument instru2;
    private PairwiseArbStrategy strategy;

    @BeforeEach
    void setup() {
        ConfigParams.resetInstance();
        Watch.resetInstance();
        Watch.createInstance(0);

        instru1 = new Instrument();
        instru1.origBaseName = "ag2603";
        instru1.symbol = "ag2603";
        instru1.tickSize = 1.0;
        instru1.lotSize = 15.0;
        instru1.priceMultiplier = 15.0;
        instru1.sendInLots = true;
        instru1.bidPx[0] = 5000;
        instru1.askPx[0] = 5001;
        instru1.bidQty[0] = 10;
        instru1.askQty[0] = 10;

        instru2 = new Instrument();
        instru2.origBaseName = "ag2605";
        instru2.symbol = "ag2605";
        instru2.tickSize = 1.0;
        instru2.lotSize = 15.0;
        instru2.priceMultiplier = 15.0;
        instru2.sendInLots = true;
        instru2.bidPx[0] = 4990;
        instru2.askPx[0] = 4991;
        instru2.bidQty[0] = 10;
        instru2.askQty[0] = 10;

        simConfig = new SimConfig();
        simConfig.instrument = instru1;
        simConfig.instrumentSec = instru2;
        simConfig.useArbStrat = true;

        ThresholdSet ts = simConfig.thresholdSet;
        ts.SIZE = 1;
        ts.MAX_SIZE = 10;
        ts.BEGIN_SIZE = 5;
        ts.BID_SIZE = 1;
        ts.BID_MAX_SIZE = 10;
        ts.ASK_SIZE = 1;
        ts.ASK_MAX_SIZE = 10;
        ts.BEGIN_PLACE = 5.0;
        ts.BEGIN_REMOVE = 3.0;
        ts.LONG_PLACE = 8.0;
        ts.LONG_REMOVE = 6.0;
        ts.SHORT_PLACE = 2.0;
        ts.SHORT_REMOVE = 1.0;
        ts.MAX_LOSS = 100000;
        ts.CHECK_PNL = true;
        ts.MAX_QUOTE_LEVEL = 1;
        ts.SUPPORTING_ORDERS = 3;
        ts.ALPHA = 0.01;
        ts.AVG_SPREAD_AWAY = 40;
        ts.SLOP = 20;

        client = new MockCommonClient();

        ConfigParams.getInstance().modeType = 1;
        strategy = new PairwiseArbStrategy(client, simConfig, null);
    }

    @AfterEach
    void cleanup() {
        Watch.resetInstance();
        ConfigParams.resetInstance();
    }

    @Test
    void test_constructorInitialization() {
        assertNotNull(strategy.firstStrat);
        assertNotNull(strategy.secondStrat);
        assertEquals(instru1, strategy.firstStrat.instru);
        assertEquals(instru2, strategy.secondStrat.instru);
        assertSame(strategy.ordMap1, strategy.firstStrat.ordMap);
        assertSame(strategy.ordMap2, strategy.secondStrat.ordMap);
        assertFalse(strategy.firstStrat.callSquareOff);
        assertFalse(strategy.secondStrat.callSquareOff);
        assertEquals(5, strategy.firstStrat.targetBidPNL.length);
    }

    @Test
    void test_loadMatrix2(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("daily_init.92201");
        try (PrintWriter pw = new PrintWriter(Files.newBufferedWriter(file))) {
            pw.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            pw.println("92201 0 10.5 ag2603 ag2605 3 -3");
        }

        Map<Integer, Map<String, String>> mx = strategy.loadMatrix2(file.toString());

        assertTrue(mx.containsKey(92201));
        assertEquals("10.5", mx.get(92201).get("avgPx"));
        assertEquals("3", mx.get(92201).get("ytd1"));
        assertEquals("-3", mx.get(92201).get("ytd2"));
    }

    @Test
    void test_dailyInit_viaConstructor(@TempDir Path tempDir) throws Exception {
        ConfigParams.getInstance().strategyID = 92201;

        Path file = tempDir.resolve("daily_init.92201");
        try (PrintWriter pw = new PrintWriter(Files.newBufferedWriter(file))) {
            pw.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            pw.println("92201 0 10.5 ag2603 ag2605 3 -3");
        }

        // C++ 在构造函数中调用 LoadMatrix2
        PairwiseArbStrategy strat = new PairwiseArbStrategy(client, simConfig, file.toString());
        strat.strategyID = 92201;

        assertEquals(10.5, strat.avgSpreadRatio_ori, 0.001);
        assertEquals(10.5, strat.avgSpreadRatio, 0.001);
        assertEquals(3, strat.firstStrat.netposPassYtd);
        assertEquals(3, strat.firstStrat.netpos);
        assertEquals(3, strat.firstStrat.netposPass);
        assertEquals(-3, strat.secondStrat.netpos);
        assertEquals(-3, strat.secondStrat.netposAgg);
    }

    @Test
    void test_saveMatrix2(@TempDir Path tempDir) throws Exception {
        strategy.strategyID = 92201;
        strategy.avgSpreadRatio_ori = 10.5;
        strategy.firstStrat.netposPass = 3;
        strategy.secondStrat.netposAgg = -3;

        Path file = tempDir.resolve("daily_init.92201");
        strategy.saveMatrix2(file.toString());

        String content = Files.readString(file);
        assertTrue(content.contains("StrategyID"));
        assertTrue(content.contains("92201"));
        assertTrue(content.contains("10.5"));
    }

    @Test
    void test_setThresholds_zeroPos() {
        strategy.firstinstru = instru1;
        strategy.secondinstru = instru2;
        strategy.thold_first = simConfig.thresholdSet;
        strategy.thold_second = simConfig.thresholdSet;
        strategy.firstStrat.netposPass = 0;

        strategy.setThresholds();

        assertEquals(5.0, strategy.firstStrat.tholdBidPlace, 0.001);
        assertEquals(5.0, strategy.firstStrat.tholdAskPlace, 0.001);
        assertEquals(3.0, strategy.firstStrat.tholdBidRemove, 0.001);
        assertEquals(3.0, strategy.firstStrat.tholdAskRemove, 0.001);
    }

    @Test
    void test_setThresholds_positivePos() {
        strategy.firstinstru = instru1;
        strategy.secondinstru = instru2;
        strategy.thold_first = simConfig.thresholdSet;
        strategy.thold_second = simConfig.thresholdSet;
        strategy.firstStrat.tholdMaxPos = 10;
        strategy.firstStrat.netposPass = 5;

        strategy.setThresholds();

        // longPlaceDiff = 8-5=3, netpos=5, maxPos=10 → offset=1.5
        // tholdBidPlace = 5.0 + 1.5 = 6.5
        assertEquals(6.5, strategy.firstStrat.tholdBidPlace, 0.001);
        // shortPlaceDiff = 5-2=3 → offset=1.5
        // tholdAskPlace = 5.0 - 1.5 = 3.5
        assertEquals(3.5, strategy.firstStrat.tholdAskPlace, 0.001);
    }

    @Test
    void test_setThresholds_negativePos() {
        strategy.firstinstru = instru1;
        strategy.secondinstru = instru2;
        strategy.thold_first = simConfig.thresholdSet;
        strategy.thold_second = simConfig.thresholdSet;
        strategy.firstStrat.tholdMaxPos = 10;
        strategy.firstStrat.netposPass = -5;

        strategy.setThresholds();

        // shortPlaceDiff=3, netpos=-5, maxPos=10 → offset=-1.5
        // tholdBidPlace = 5.0 + 3 * (-5)/10 = 3.5
        assertEquals(3.5, strategy.firstStrat.tholdBidPlace, 0.001);
        // longPlaceDiff=3, netpos=-5 → offset=-(-1.5)
        // tholdAskPlace = 5.0 - 3 * (-5)/10 = 6.5
        assertEquals(6.5, strategy.firstStrat.tholdAskPlace, 0.001);
    }

    @Test
    void test_calcPendingNetposAgg() {
        // 添加 CROSS 买单到 ordMap2
        OrderStats buy = new OrderStats();
        buy.side = Constants.SIDE_BUY;
        buy.ordType = OrderStats.HitType.CROSS;
        buy.openQty = 3;
        buy.orderID = 101;
        strategy.ordMap2.put(101, buy);

        // 添加 CROSS 卖单
        OrderStats sell = new OrderStats();
        sell.side = Constants.SIDE_SELL;
        sell.ordType = OrderStats.HitType.CROSS;
        sell.openQty = 2;
        sell.orderID = 102;
        strategy.ordMap2.put(102, sell);

        // 添加 STANDARD 单（不计入）
        OrderStats std = new OrderStats();
        std.side = Constants.SIDE_BUY;
        std.ordType = OrderStats.HitType.STANDARD;
        std.openQty = 5;
        std.orderID = 103;
        strategy.ordMap2.put(103, std);

        int pending = strategy.calcPendingNetposAgg();

        // 3 (buy) - 2 (sell) = 1
        assertEquals(1, pending);
    }

    @Test
    void test_orsCallBack_routesToFirstStrat() {
        // 在 firstStrat 上创建订单
        strategy.firstStrat.tholdSize = 1;
        OrderStats order = strategy.firstStrat.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);

        Arena arena = Arena.ofConfined();
        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_NEW_ORDER_CONFIRM);

        strategy.orsCallBack(resp);

        assertEquals(OrderStats.Status.NEW_CONFIRM, order.status);
        arena.close();
    }

    @Test
    void test_orsCallBack_routesToSecondStrat() {
        // 在 secondStrat 上创建订单
        strategy.secondStrat.tholdSize = 1;
        OrderStats order = strategy.secondStrat.sendNewOrder(Constants.SIDE_SELL, 4990, 1, 0);

        Arena arena = Arena.ofConfined();
        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_NEW_ORDER_CONFIRM);

        strategy.orsCallBack(resp);

        assertEquals(OrderStats.Status.NEW_CONFIRM, order.status);
        arena.close();
    }

    @Test
    void test_orsCallBack_tradeResetsAggRepeat() {
        strategy.agg_repeat = 3;
        strategy.firstStrat.tholdSize = 1;
        OrderStats order = strategy.firstStrat.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        order.status = OrderStats.Status.NEW_CONFIRM;

        Arena arena = Arena.ofConfined();
        MemorySegment resp = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, order.orderID);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, Constants.RESP_TRADE_CONFIRM);
        Types.RESP_QUANTITY_VH.set(resp, 0L, 1);
        Types.RESP_PRICE_VH.set(resp, 0L, 5000.0);

        strategy.orsCallBack(resp);

        assertEquals(1, strategy.agg_repeat); // reset to 1
        arena.close();
    }

    @Test
    void test_handleSquareoff() {
        // 添加订单到两腿
        strategy.firstStrat.tholdSize = 1;
        OrderStats o1 = strategy.firstStrat.sendNewOrder(Constants.SIDE_BUY, 5000, 1, 0);
        o1.status = OrderStats.Status.NEW_CONFIRM;

        strategy.secondStrat.tholdSize = 1;
        OrderStats o2 = strategy.secondStrat.sendNewOrder(Constants.SIDE_SELL, 4990, 1, 0);
        o2.status = OrderStats.Status.NEW_CONFIRM;

        strategy.handleSquareoff();

        assertTrue(strategy.firstStrat.onExit);
        assertTrue(strategy.firstStrat.onFlat);
        assertTrue(strategy.secondStrat.onExit);
        assertTrue(strategy.secondStrat.onFlat);
        assertFalse(strategy.active);
    }

    @Test
    void test_dailyInit_missingStrategy(@TempDir Path tempDir) throws Exception {
        ConfigParams.getInstance().strategyID = 99999; // not in file

        Path file = tempDir.resolve("daily_init.99999");
        try (PrintWriter pw = new PrintWriter(Files.newBufferedWriter(file))) {
            pw.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            pw.println("92201 0 10.5 ag2603 ag2605 3 -3");
        }

        // C++ 构造函数中如果找不到 strategyID 对应行，会抛异常
        assertThrows(RuntimeException.class, () ->
            new PairwiseArbStrategy(client, simConfig, file.toString()));
    }

    @Test
    void test_dailyInit_sameInstrumentNames(@TempDir Path tempDir) throws Exception {
        ConfigParams.getInstance().strategyID = 92201;
        // Make both instruments have same name
        instru1.origBaseName = "ag2603";
        instru2.origBaseName = "ag2603";

        Path file = tempDir.resolve("daily_init.92201");
        try (PrintWriter pw = new PrintWriter(Files.newBufferedWriter(file))) {
            pw.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            pw.println("92201 0 10.5 ag2603 ag2603 3 -3");
        }

        // C++ 构造函数中如果两腿 origBaseName 相同，会抛异常
        assertThrows(RuntimeException.class, () ->
            new PairwiseArbStrategy(client, simConfig, file.toString()));
    }

    @Test
    void test_toString() {
        String s = strategy.toString();
        assertTrue(s.contains("PairwiseArbStrategy"));
    }

    // =======================================================================
    //  Bug fix tests: handleSquareoff endTime 误发单
    //  事故重现: 策略在 endTime 后启动 (active=false)，
    //  END TIME 立即触发，发出 SELL 82 ag2603 / BUY 83 ag2605 (flag=OPEN)
    // =======================================================================

    /** 构造包含 exchTS 和 symbol 的 MarketUpdateNew MemorySegment */
    private MemorySegment buildMarketUpdate(Arena arena, long exchTS, String symbol) {
        MemorySegment update = arena.allocate(Types.MARKET_UPDATE_NEW_SIZE);
        // header.exchTS
        Types.MDH_EXCH_TS_VH.set(update, 0L, exchTS);
        // header.symbol (offset 40, 48 bytes)
        if (symbol != null) {
            byte[] symBytes = symbol.getBytes(java.nio.charset.StandardCharsets.US_ASCII);
            MemorySegment symSlice = update.asSlice(Types.MDH_SYMBOL_OFFSET, 48);
            for (int i = 0; i < Math.min(symBytes.length, 48); i++) {
                symSlice.set(java.lang.foreign.ValueLayout.JAVA_BYTE, i, symBytes[i]);
            }
        }
        return update;
    }

    /**
     * Fix 3: active=false 时 mdCallBack 不触发 endTime 检查。
     * 模拟事故场景: 昨仓 82/-83, endTime 已过, active=false.
     */
    @Test
    void test_mdCallBack_activeFalse_endTimePassed_noOrders() {
        Arena arena = Arena.ofConfined();
        try {
            // 设置昨仓
            strategy.firstStrat.netpos = 82;
            strategy.firstStrat.netposPass = 82;
            strategy.secondStrat.netpos = -83;
            strategy.secondStrat.netposAgg = -83;

            // CTP 模式: active=false
            strategy.active = false;

            // 设置 endTime 为过去 (exchTS > endTimeEpoch)
            long pastEndTime = 1_000_000_000_000L;
            strategy.endTimeEpoch = pastEndTime;
            strategy.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            strategy.firstStrat.endTimeEpoch = pastEndTime;
            strategy.firstStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            strategy.secondStrat.endTimeEpoch = pastEndTime;
            strategy.secondStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;

            // 设置行情价格有效
            instru1.bidPx[0] = 5000;
            instru1.askPx[0] = 5001;
            instru2.bidPx[0] = 4990;
            instru2.askPx[0] = 4991;

            int ordersBefore = client.newOrderCount;

            // 发送行情 (exchTS 晚于 endTime)
            long nowTS = pastEndTime + 60_000_000_000L; // endTime 过后 60 秒
            // 设置 Watch 全局时钟
            Watch.getInstance().updateTime(nowTS, "test");
            MemorySegment update = buildMarketUpdate(arena, nowTS, "ag2603");
            strategy.mdCallBack(update);

            // 关键验证: active=false 时不触发 endTime → 不调用 handleSquareoff → 不发单
            assertFalse(strategy.onExit, "active=false 时不应触发 endTime → onExit 应保持 false");
            assertFalse(strategy.onFlat, "active=false 时不应触发 endTime → onFlat 应保持 false");
            assertEquals(ordersBefore, client.newOrderCount,
                    "active=false + endTime 过后，不应发送任何订单（事故重现验证）");
        } finally {
            arena.close();
        }
    }

    /**
     * 回归测试: active=true 时 mdCallBack 正常触发 endTime squareoff。
     * Java 现在与 C++ 一致，使用全局 Watch::GetCurrentTime()。
     * 测试中需通过 Watch.getInstance().updateTime() 设置时钟。
     */
    @Test
    void test_mdCallBack_activeTrue_endTimePassed_triggersSquareoff() {
        Arena arena = Arena.ofConfined();
        try {
            strategy.firstStrat.netpos = 0;
            strategy.firstStrat.netposPass = 0;
            strategy.secondStrat.netpos = 0;
            strategy.secondStrat.netposAgg = 0;

            // active=true (sim mode)
            strategy.active = true;

            long pastEndTime = 1_000_000_000_000L;
            long nowTS = pastEndTime + 60_000_000_000L;
            strategy.endTimeEpoch = pastEndTime;
            strategy.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            strategy.firstStrat.endTimeEpoch = pastEndTime;
            strategy.firstStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            strategy.secondStrat.endTimeEpoch = pastEndTime;
            strategy.secondStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;

            instru1.bidPx[0] = 5000;
            instru1.askPx[0] = 5001;
            instru2.bidPx[0] = 4990;
            instru2.askPx[0] = 4991;

            // 设置 Watch 全局时钟（替代原先由 MemorySegment 中的 exchTS 字段驱动）
            Watch.getInstance().updateTime(nowTS, "test");

            MemorySegment update = buildMarketUpdate(arena, nowTS, "ag2603");
            strategy.mdCallBack(update);

            // 验证: active=true 时 endTime 正常触发
            assertTrue(strategy.onExit, "active=true 时 endTime 应触发 onExit");
            assertTrue(strategy.onFlat, "active=true 时 endTime 应触发 onFlat");
            assertFalse(strategy.active, "handleSquareoff 应设置 active=false");
        } finally {
            arena.close();
        }
    }

    /**
     * 综合事故重现: 昨仓 82/-83 + endTime 过后 + active=false + 子 strat 也不发单。
     * 验证三层守卫全部生效。
     */
    @Test
    void test_fullAccidentScenario_noOrdersSent(@TempDir Path tempDir) throws Exception {
        Arena arena = Arena.ofConfined();
        try {
            // 使用 CTP 模式创建策略
            ConfigParams.resetInstance();
            Watch.resetInstance();
            Watch.createInstance(0);
            ConfigParams.getInstance().modeType = 2; // CTP mode
            ConfigParams.getInstance().strategyID = 92201;

            Path file = tempDir.resolve("daily_init.92201");
            try (PrintWriter pw = new PrintWriter(Files.newBufferedWriter(file))) {
                pw.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
                pw.println("92201 0 360.0 ag_F_3_SFE ag_F_5_SFE 82 -83");
            }
            instru1.origBaseName = "ag_F_3_SFE";
            instru2.origBaseName = "ag_F_5_SFE";

            MockCommonClient testClient = new MockCommonClient();
            PairwiseArbStrategy testStrat = new PairwiseArbStrategy(testClient, simConfig, file.toString());
            testStrat.strategyID = 92201;

            // 验证初始状态: CTP mode → active=false, 昨仓已加载
            assertFalse(testStrat.active, "CTP mode 策略应初始化为 active=false");
            assertEquals(82, testStrat.firstStrat.netpos);
            assertEquals(82, testStrat.firstStrat.netposPass);
            assertEquals(-83, testStrat.secondStrat.netpos);
            assertEquals(-83, testStrat.secondStrat.netposAgg);

            // 设置 endTime 为过去
            long pastEndTime = 1_000_000_000_000L;
            testStrat.endTimeEpoch = pastEndTime;
            testStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            testStrat.firstStrat.endTimeEpoch = pastEndTime;
            testStrat.firstStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;
            testStrat.secondStrat.endTimeEpoch = pastEndTime;
            testStrat.secondStrat.endTimeAggEpoch = pastEndTime + 120_000_000_000L;

            // 设置行情
            instru1.bidPx[0] = 23288;
            instru1.askPx[0] = 23290;
            instru2.bidPx[0] = 22828;
            instru2.askPx[0] = 22830;

            int ordersBefore = testClient.newOrderCount;

            // 模拟第一次行情到达 (endTime 过后 60 秒)
            long nowTS = pastEndTime + 60_000_000_000L;
            // 设置 Watch 全局时钟
            Watch.getInstance().updateTime(nowTS, "test");
            MemorySegment update = buildMarketUpdate(arena, nowTS, "ag_F_3_SFE");
            testStrat.mdCallBack(update);

            // ===== 核心验证 =====
            // 事故中: 此处发出了 SELL 82 ag2603 + BUY 83 ag2605
            // 修复后: 三层守卫阻止所有订单
            assertEquals(ordersBefore, testClient.newOrderCount,
                    "事故重现: 昨仓82/-83 + endTime过后 + active=false，不应发送任何订单！"
                    + " (实际发送了 " + (testClient.newOrderCount - ordersBefore) + " 笔)");
            assertTrue(testClient.orderRecords.isEmpty(),
                    "不应有任何订单记录");

            // 验证策略状态: active=false 时 endTime 检查被跳过
            assertFalse(testStrat.onExit,
                    "active=false 时 PairwiseArb 层 endTime 不应触发 onExit");
        } finally {
            arena.close();
            ConfigParams.resetInstance();
        }
    }

    /**
     * 验证 handleSquareoff 中 sendNewOrder 使用 POS_OPEN flag 的问题。
     * 当 active=true 时基类 handleSquareoff 发送的订单 flag 应该是什么？
     * 记录当前行为以便后续修复 flag 问题。
     */
    @Test
    void test_handleSquareoff_orderFlag_isPosOpen() {
        // 当 active=true 且有持仓时，handleSquareoff 发送平仓订单
        // 当前 C++ 原代码和 Java 都使用 POS_OPEN (sendNewOrder 硬编码)
        // counter_bridge 的 SetCombOffsetFlag 会自动推断开平方向
        // 此测试记录当前行为
        strategy.firstStrat.netpos = 10;
        strategy.firstStrat.active = true;
        strategy.firstStrat.onFlat = true;
        strategy.firstStrat.onExit = false;
        strategy.firstStrat.instru = instru1;
        instru1.bidPx[0] = 5000;
        instru1.askPx[0] = 5001;

        strategy.firstStrat.handleSquareoff();

        assertFalse(client.orderRecords.isEmpty(), "应发送平仓订单");
        MockCommonClient.OrderRecord rec = client.orderRecords.get(client.orderRecords.size() - 1);
        assertEquals(Constants.POS_OPEN, rec.posDirection,
                "当前 sendNewOrder 硬编码使用 POS_OPEN（与 C++ 一致，由 counter_bridge 自动推断开平）");
    }
}
