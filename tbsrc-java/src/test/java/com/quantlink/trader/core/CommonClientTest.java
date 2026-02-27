package com.quantlink.trader.core;

import com.quantlink.trader.shm.Types;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * CommonClient 回调分发测试。
 */
class CommonClientTest {

    @BeforeEach
    void setup() {
        ConfigParams.resetInstance();
        Watch.resetInstance();
        Watch.createInstance(0);
    }

    @AfterEach
    void cleanup() {
        Watch.resetInstance();
        ConfigParams.resetInstance();
    }

    /**
     * 测试 MD 按 symbolID 数组路由到 Instrument 并触发回调。
     * symbolID 由 md_shm_feeder 设置 (BuildSymbolIDMap, 按字母排序分配 0,1,2...)
     * Ref: CommonClient.cpp:418 — simIter = m_configParams->m_simConfigList[update->m_symbolID]
     */
    @Test
    void test_mdDispatchBySymbol() {
        ConfigParams params = ConfigParams.getInstance();
        CommonClient client = new CommonClient();

        // 创建 Instrument 和 SimConfig
        Instrument inst = new Instrument();
        inst.origBaseName = "ag2603";
        inst.symbol = "ag2603";
        inst.instrument = "ag2603"; // C++: m_instrument — 用于 isStratSymbol 判定

        SimConfig simCfg = new SimConfig();
        simCfg.instrument = inst;
        simCfg.instruMap.put("ag2603", inst);
        // C++: m_instruList[symbolID] — symbolID=0 for "ag2603" (single symbol)
        simCfg.instruList = new Instrument[]{inst};

        // C++: m_simConfigList[symbolID] — symbolID=0
        List<SimConfig> simList = new ArrayList<>();
        simList.add(simCfg);
        @SuppressWarnings("unchecked")
        List<SimConfig>[] scList = new List[]{simList};
        params.simConfigList = scList;
        params.simConfigMap.put("ag2603", simList);

        // 记录 MD 回调
        AtomicReference<MemorySegment> received = new AtomicReference<>();
        client.setMDCallback(received::set);

        // 构造 MarketUpdateNew — 写入 m_symbol + m_symbolID=0
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        byte[] symBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, md, Types.MDH_SYMBOL_OFFSET, symBytes.length);
        Types.MDH_SYMBOL_ID_VH.set(md, 0L, (short) 0);
        Types.MDH_TIMESTAMP_VH.set(md, 0L, 1000000L);
        Types.MDH_EXCH_TS_VH.set(md, 0L, 2000000L);

        long base = Types.MU_DATA_OFFSET;
        Types.MDD_BID_PRICE_VH.set(md, base, 0L, 5499.0);
        Types.MDD_BID_QUANTITY_VH.set(md, base, 0L, 10);
        Types.MDD_ASK_PRICE_VH.set(md, base, 0L, 5501.0);
        Types.MDD_ASK_QUANTITY_VH.set(md, base, 0L, 5);
        Types.MDD_LAST_TRADED_PRICE_VH.set(md, base, 5500.0);

        // 分发
        client.sendInfraMDUpdate(md);

        // 验证 Instrument 更新
        assertEquals(5499.0, inst.bidPx[0], 0.001);
        assertEquals(5501.0, inst.askPx[0], 0.001);
        assertEquals(5500.0, inst.lastTradePx, 0.001);
        assertEquals(1000000L, inst.lastLocalTime);
        assertEquals(2000000L, inst.lastExchTime);

        // 验证回调触发
        assertNotNull(received.get());
    }

    /**
     * 测试 MD 按 symbolID 数组路由 (C++ 原生路径)。
     * symbolID 由 md_shm_feeder 设置 (BuildSymbolIDMap, 按字母排序分配 0,1,2...)
     * Ref: CommonClient.cpp:418 — simIter = m_configParams->m_simConfigList[update->m_symbolID]
     */
    @Test
    void test_mdDispatchBySymbolID() {
        ConfigParams params = ConfigParams.getInstance();
        CommonClient client = new CommonClient();

        // 创建 2 个合约 — 按字母排序: ag2603(symbolID=0), ag2605(symbolID=1)
        Instrument inst1 = new Instrument();
        inst1.origBaseName = "ag2603";
        inst1.symbol = "ag2603";
        inst1.instrument = "ag2603";

        Instrument inst2 = new Instrument();
        inst2.origBaseName = "ag2605";
        inst2.symbol = "ag2605";
        inst2.instrument = "ag2605";

        SimConfig simCfg1 = new SimConfig();
        simCfg1.instrument = inst1;
        simCfg1.instruMap.put("ag2603", inst1);
        simCfg1.instruMap.put("ag2605", inst2);
        // instruList: symbolID 0 → inst1, symbolID 1 → inst2
        simCfg1.instruList = new Instrument[]{inst1, inst2};

        SimConfig simCfg2 = new SimConfig();
        simCfg2.instrument = inst2;
        simCfg2.instruMap.put("ag2603", inst1);
        simCfg2.instruMap.put("ag2605", inst2);
        simCfg2.instruList = new Instrument[]{inst1, inst2};

        // simConfigList: symbolID 0 → [simCfg1], symbolID 1 → [simCfg2]
        List<SimConfig> list1 = new ArrayList<>();
        list1.add(simCfg1);
        List<SimConfig> list2 = new ArrayList<>();
        list2.add(simCfg2);

        @SuppressWarnings("unchecked")
        List<SimConfig>[] scList = new List[]{list1, list2};
        params.simConfigList = scList;
        params.simConfigMap.put("ag2603", list1);
        params.simConfigMap.put("ag2605", list2);

        // 记录 MD 回调
        AtomicReference<MemorySegment> received = new AtomicReference<>();
        client.setMDCallback(received::set);

        // 构造 MarketUpdateNew — 写入 symbol="ag2605" + symbolID=1
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        byte[] symBytes = "ag2605".getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, md, Types.MDH_SYMBOL_OFFSET, symBytes.length);
        Types.MDH_SYMBOL_ID_VH.set(md, 0L, (short) 1); // symbolID = 1
        Types.MDH_TIMESTAMP_VH.set(md, 0L, 1000000L);
        Types.MDH_EXCH_TS_VH.set(md, 0L, 2000000L);

        long base = Types.MU_DATA_OFFSET;
        Types.MDD_BID_PRICE_VH.set(md, base, 0L, 4990.0);
        Types.MDD_BID_QUANTITY_VH.set(md, base, 0L, 10);
        Types.MDD_ASK_PRICE_VH.set(md, base, 0L, 4991.0);
        Types.MDD_ASK_QUANTITY_VH.set(md, base, 0L, 5);
        Types.MDD_LAST_TRADED_PRICE_VH.set(md, base, 4990.5);

        client.sendInfraMDUpdate(md);

        // 验证 inst2 更新 (通过 symbolID=1 路由)
        assertEquals(4990.0, inst2.bidPx[0], 0.001);
        assertEquals(4991.0, inst2.askPx[0], 0.001);
        assertEquals(4990.5, inst2.lastTradePx, 0.001);

        // 验证回调触发 (isStratSymbol: simCfg2.instrument="ag2605" 匹配 symbol="ag2605")
        assertNotNull(received.get());
    }

    /**
     * 测试未注册的 symbol 不触发回调。
     */
    @Test
    void test_mdDispatch_unknownSymbol_noCallback() {
        CommonClient client = new CommonClient();

        AtomicReference<MemorySegment> received = new AtomicReference<>();
        client.setMDCallback(received::set);

        // 写入未注册的 symbol
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        byte[] symBytes = "zz9999".getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, md, Types.MDH_SYMBOL_OFFSET, symBytes.length);

        client.sendInfraMDUpdate(md);

        assertNull(received.get());
    }

    /**
     * 测试 ORS 回调分发。
     */
    @Test
    void test_orsDispatch() {
        CommonClient client = new CommonClient();

        AtomicReference<MemorySegment> received = new AtomicReference<>();
        client.setORSCallback(received::set);

        MemorySegment resp = Arena.global().allocate(Types.RESPONSE_MSG_LAYOUT);
        Types.RESP_ORDER_ID_VH.set(resp, 0L, 1000001);
        Types.RESP_RESPONSE_TYPE_VH.set(resp, 0L, 4); // TRADE_CONFIRM

        client.sendInfraORSUpdate(resp);

        assertNotNull(received.get());
        assertEquals(1000001, (int) Types.RESP_ORDER_ID_VH.get(received.get(), 0L));
    }

    /**
     * 测试 ConfigParams 单例。
     */
    @Test
    void test_configParamsSingleton() {
        ConfigParams p1 = ConfigParams.getInstance();
        ConfigParams p2 = ConfigParams.getInstance();
        assertSame(p1, p2);

        ConfigParams.resetInstance();
        ConfigParams p3 = ConfigParams.getInstance();
        assertNotSame(p1, p3);
    }
}
