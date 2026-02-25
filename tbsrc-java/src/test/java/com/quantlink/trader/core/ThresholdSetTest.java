package com.quantlink.trader.core;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * ThresholdSet 默认值测试。
 * 验证与 C++ ThresholdSet() 构造函数的默认值一致。
 * Ref: tbsrc/main/include/TradeBotUtils.h:239-320
 */
class ThresholdSetTest {

    @Test
    void test_defaultBooleans() {
        ThresholdSet ts = new ThresholdSet();
        assertFalse(ts.USE_NOTIONAL);
        assertFalse(ts.USE_PERCENT);
        assertFalse(ts.USE_PRICE_LIMIT);
        assertFalse(ts.USE_AHEAD_PERCENT);
        assertFalse(ts.USE_CLOSE_CROSS);
        assertFalse(ts.USE_PASSIVE_THOLD);
        assertFalse(ts.USE_LINEAR_THOLD);
        assertFalse(ts.QUOTE_MAX_QTY);
        assertTrue(ts.CLOSE_PNL);    // C++: CLOSE_PNL = true
        assertTrue(ts.CHECK_PNL);    // C++: CHECK_PNL = true
        assertFalse(ts.NEWS_FLAT);
    }

    @Test
    void test_defaultNumericValues() {
        ThresholdSet ts = new ThresholdSet();

        // C++: OPP_QTY = 1000000000
        assertEquals(1_000_000_000, ts.OPP_QTY, 0.001);
        assertEquals(1, ts.SUPP_TOLERANCE);
        assertEquals(100, ts.AHEAD_PERCENT, 0.001);
        assertEquals(1_000_000_000_000.0, ts.AHEAD_SIZE, 0.001);
        assertEquals(1_000_000, ts.SZAHEAD_NOCXL);
        assertEquals(1_000_000, ts.BOOKSZ_NOCXL);
        assertEquals(0, ts.AGGFLAT_BOOKSIZE);
        assertEquals(0, ts.AGGFLAT_BOOKFRAC, 0.001);

        // C++: MAX_OS_ORDER = 5
        assertEquals(5, ts.MAX_OS_ORDER);

        // C++: UPNL_LOSS = 10000000000
        assertEquals(10_000_000_000.0, ts.UPNL_LOSS, 0.001);
        assertEquals(10_000_000_000.0, ts.STOP_LOSS, 0.001);
        assertEquals(100_000_000_000.0, ts.MAX_LOSS, 0.001);
        assertEquals(1_000_000, ts.PT_LOSS, 0.001);
        assertEquals(1_000_000, ts.PT_PROFIT, 0.001);
    }

    @Test
    void test_defaultSpreadAndEwa() {
        ThresholdSet ts = new ThresholdSet();

        // C++: SPREAD_EWA = 0.6
        assertEquals(0.6, ts.SPREAD_EWA, 0.001);
        assertEquals(100_000_000_000.0, ts.CLOSE_CROSS, 0.001);
        assertEquals(1_000_000_000, ts.CROSS, 0.001);
        assertEquals(0, ts.CROSS_TARGET);
        assertEquals(0, ts.CROSS_TICKS);
        assertEquals(1_000_000_000, ts.IMPROVE, 0.001);
        assertEquals(0, ts.AGG_COOL_OFF);
        assertEquals(0.0, ts.PLACE_SPREAD, 0.001);
        assertEquals(0.0, ts.PIL_FACTOR, 0.001);
    }

    @Test
    void test_defaultCrossLimits() {
        ThresholdSet ts = new ThresholdSet();
        assertEquals(1_000_000_000, ts.MAX_CROSS);
        assertEquals(1_000_000_000, ts.MAX_LONG_CROSS);
        assertEquals(1_000_000_000, ts.MAX_SHORT_CROSS);
        assertEquals(1_000_000_000, ts.MAX_QUOTE_SPREAD);
        assertEquals(-1, ts.CLOSE_IMPROVE, 0.001);
        assertEquals(0, ts.QUOTE_SKEW, 0.001);
        assertEquals(100_000, ts.DELTA_HEDGE, 0.001);
    }

    @Test
    void test_defaultTimeAndLevel() {
        ThresholdSet ts = new ThresholdSet();
        assertEquals(0, ts.PAUSE);
        assertEquals(0, ts.CANCELREQ_PAUSE);
        assertEquals(3, ts.MAX_QUOTE_LEVEL);
    }

    @Test
    void test_defaultChineseSpecificParams() {
        ThresholdSet ts = new ThresholdSet();
        // pqr 20240902
        assertEquals(20, ts.AVG_SPREAD_AWAY);
        assertEquals(20, ts.SLOP);
        assertEquals(0.0, ts.CONST, 0.001);
    }

    @Test
    void test_defaultStatParams() {
        ThresholdSet ts = new ThresholdSet();
        assertEquals(0, ts.STAT_DURATION_SMALL);
        assertEquals(1, ts.STAT_DURATION_LONG);
        assertEquals(0, ts.STAT_TRADE_THRESH, 0.001);
        assertEquals(5, ts.STAT_DECAY);
        assertEquals(1, ts.MAX_DELTA_VALUE, 0.001);
        assertEquals(-1, ts.MIN_DELTA_VALUE, 0.001);
        assertEquals(2, ts.MAX_DELTA_CHANGE, 0.001);
    }

    @Test
    void test_defaultTvarTcache() {
        ThresholdSet ts = new ThresholdSet();
        assertEquals(-1, ts.TVAR_KEY);
        assertEquals(-1, ts.TCACHE_KEY);
        assertEquals(-1, ts.UNDERLYING_UPPER_BOND, 0.001);
        assertEquals(-1, ts.UNDERLYING_LOWER_BOND, 0.001);
    }
}
