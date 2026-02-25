package com.quantlink.trader.core;

import com.quantlink.trader.shm.Types;
import org.junit.jupiter.api.Test;

import java.lang.foreign.Arena;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Instrument 行情数据模型测试。
 */
class InstrumentTest {

    @Test
    void test_fillOrderBook_readsBookCorrectly() {
        Instrument inst = new Instrument();

        // 构造 MarketUpdateNew
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        long base = Types.MU_DATA_OFFSET; // 96

        // 写入 bid/ask 数据
        Types.MDD_BID_PRICE_VH.set(md, base, 0L, 5499.0);
        Types.MDD_BID_QUANTITY_VH.set(md, base, 0L, 10);
        Types.MDD_BID_PRICE_VH.set(md, base, 1L, 5498.0);
        Types.MDD_BID_QUANTITY_VH.set(md, base, 1L, 20);

        Types.MDD_ASK_PRICE_VH.set(md, base, 0L, 5501.0);
        Types.MDD_ASK_QUANTITY_VH.set(md, base, 0L, 5);
        Types.MDD_ASK_PRICE_VH.set(md, base, 1L, 5502.0);
        Types.MDD_ASK_QUANTITY_VH.set(md, base, 1L, 15);

        Types.MDD_LAST_TRADED_PRICE_VH.set(md, base, 5500.0);

        inst.fillOrderBook(md);

        assertEquals(5499.0, inst.bidPx[0], 0.001);
        assertEquals(10.0, inst.bidQty[0], 0.001);
        assertEquals(5498.0, inst.bidPx[1], 0.001);
        assertEquals(20.0, inst.bidQty[1], 0.001);
        assertEquals(5501.0, inst.askPx[0], 0.001);
        assertEquals(5.0, inst.askQty[0], 0.001);
        assertEquals(5502.0, inst.askPx[1], 0.001);
        assertEquals(15.0, inst.askQty[1], 0.001);
        assertEquals(5500.0, inst.lastTradePx, 0.001);
    }

    @Test
    void test_getMidPrice() {
        Instrument inst = new Instrument();
        inst.bidPx[0] = 5499.0;
        inst.askPx[0] = 5501.0;
        assertEquals(5500.0, inst.getMidPrice(), 0.001);
    }

    @Test
    void test_getMswPrice() {
        Instrument inst = new Instrument();
        inst.bidPx[0] = 5499.0;
        inst.askPx[0] = 5501.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 5;

        // MSW = (5*5499 + 5501*10) / (5+10) = (27495 + 55010) / 15 = 82505/15 = 5500.333...
        assertEquals(5500.333, inst.getMswPrice(), 0.01);
    }

    @Test
    void test_getLtpPrice_withinSpread() {
        Instrument inst = new Instrument();
        inst.bidPx[0] = 5499.0;
        inst.askPx[0] = 5501.0;
        inst.lastTradePx = 5500.0;
        assertEquals(5500.0, inst.getLtpPrice(), 0.001);
    }

    @Test
    void test_getLtpPrice_belowBid() {
        Instrument inst = new Instrument();
        inst.bidPx[0] = 5499.0;
        inst.askPx[0] = 5501.0;
        inst.lastTradePx = 5490.0;
        assertEquals(5499.0, inst.getLtpPrice(), 0.001);
    }

    @Test
    void test_readSymbol() {
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        byte[] sym = "ag2603".getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(sym), 0, md, Types.MDH_SYMBOL_OFFSET, sym.length);

        assertEquals("ag2603", Instrument.readSymbol(md));
    }

    @Test
    void test_readSymbolID() {
        MemorySegment md = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        Types.MDH_SYMBOL_ID_VH.set(md, 0L, (short) 42);

        assertEquals(42, Instrument.readSymbolID(md));
    }

    @Test
    void test_reset() {
        Instrument inst = new Instrument();
        inst.bidPx[0] = 100;
        inst.askQty[5] = 200;
        inst.lastTradePx = 99;
        inst.totalTradedQty = 1000;

        inst.reset();

        assertEquals(0.0, inst.bidPx[0]);
        assertEquals(0.0, inst.askQty[5]);
        assertEquals(0.0, inst.lastTradePx);
        assertEquals(0.0, inst.totalTradedQty);
    }
}
