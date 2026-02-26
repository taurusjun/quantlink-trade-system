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
    }

    @AfterEach
    void cleanup() {
        ConfigParams.resetInstance();
    }

    /**
     * 测试 MD 按 symbol 字符串路由到 Instrument 并触发回调。
     * 注意: md_shm_feeder 不设置 m_symbolID（memset 后为 0），
     *       因此按 m_symbol 字符串路由（与 Go 版本一致）。
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

        // 注册 symbol → SimConfig 映射
        List<SimConfig> simList = new ArrayList<>();
        simList.add(simCfg);
        params.simConfigMap.put("ag2603", simList);

        // 记录 MD 回调
        AtomicReference<MemorySegment> received = new AtomicReference<>();
        client.setMDCallback(received::set);

        // 构造 MarketUpdateNew — 写入 m_symbol 字符串
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        byte[] symBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, md, Types.MDH_SYMBOL_OFFSET, symBytes.length);
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
