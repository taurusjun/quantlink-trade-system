package com.quantlink.trader.shm;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.AfterEach;

import java.lang.foreign.*;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * 跨进程互操作验证 — Java 进程间 SHM 读写一致性测试。
 *
 * 注意: 与 C++ md_shm_feeder / counter_bridge 的完整跨进程测试
 * 需要在 Linux 环境下运行（macOS shmmax 限制导致大队列创建失败）。
 * 本测试使用小规模队列验证 Java SHM 层的正确性。
 *
 * 迁移自: Task 10 跨进程互操作验证
 */
class CrossProcessTest {

    private static final int TEST_MD_KEY = 0xF001;
    private static final int TEST_REQ_KEY = 0xF002;
    private static final int TEST_RESP_KEY = 0xF003;
    private MWMRQueue mdQueue;
    private MWMRQueue reqQueue;
    private MWMRQueue respQueue;

    @AfterEach
    void cleanup() {
        if (mdQueue != null) { try { mdQueue.destroy(); } catch (Exception e) {} }
        if (reqQueue != null) { try { reqQueue.destroy(); } catch (Exception e) {} }
        if (respQueue != null) { try { respQueue.destroy(); } catch (Exception e) {} }
    }

    /**
     * 10.1 模拟: Java 读取 MarketUpdateNew
     * 验证写入和读取的 symbol、price 等关键字段一致
     */
    @Test
    void test_readMarketUpdateNew_fieldsConsistent() {
        // 创建行情队列（小规模）
        mdQueue = MWMRQueue.create(TEST_MD_KEY, 16,
                Types.MARKET_UPDATE_NEW_LAYOUT.byteSize(),
                Types.QUEUE_ELEM_MD_SIZE);

        // 模拟 C++ md_shm_feeder 写入
        MemorySegment mdIn = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);

        // 写入 symbol (MDHeaderPart offset 40, char[48])
        String symbol = "ag2603";
        byte[] symbolBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symbolBytes), 0, mdIn, Types.MDH_SYMBOL_OFFSET, symbolBytes.length);

        // 写入 lastTradedPrice (MDDataPart, base=MU_DATA_OFFSET=96)
        long mdDataBase = Types.MU_DATA_OFFSET; // 96
        Types.MDD_LAST_TRADED_PRICE_VH.set(mdIn, mdDataBase, 5500.0);

        // 写入 bidUpdates[0].price
        Types.MDD_BID_PRICE_VH.set(mdIn, mdDataBase, 0L, 5499.0);
        Types.MDD_BID_QUANTITY_VH.set(mdIn, mdDataBase, 0L, 10);

        // 写入 askUpdates[0].price
        Types.MDD_ASK_PRICE_VH.set(mdIn, mdDataBase, 0L, 5501.0);
        Types.MDD_ASK_QUANTITY_VH.set(mdIn, mdDataBase, 0L, 5);

        mdQueue.enqueue(mdIn);

        // Java 端读取
        MemorySegment mdOut = Arena.global().allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
        assertTrue(mdQueue.dequeue(mdOut));

        // 验证 symbol
        byte[] readSymbol = new byte[48];
        MemorySegment.copy(mdOut, Types.MDH_SYMBOL_OFFSET, MemorySegment.ofArray(readSymbol), 0, 48);
        String readSymbolStr = new String(readSymbol, StandardCharsets.US_ASCII).trim().replace("\0", "");
        assertEquals("ag2603", readSymbolStr);

        // 验证 lastTradedPrice
        double ltp = (double) Types.MDD_LAST_TRADED_PRICE_VH.get(mdOut, mdDataBase);
        assertEquals(5500.0, ltp, 0.001);

        // 验证 bid/ask
        double bidPrice = (double) Types.MDD_BID_PRICE_VH.get(mdOut, mdDataBase, 0L);
        assertEquals(5499.0, bidPrice, 0.001);
        int bidQty = (int) Types.MDD_BID_QUANTITY_VH.get(mdOut, mdDataBase, 0L);
        assertEquals(10, bidQty);

        double askPrice = (double) Types.MDD_ASK_PRICE_VH.get(mdOut, mdDataBase, 0L);
        assertEquals(5501.0, askPrice, 0.001);
        int askQty = (int) Types.MDD_ASK_QUANTITY_VH.get(mdOut, mdDataBase, 0L);
        assertEquals(5, askQty);
    }

    /**
     * 10.2 模拟: Java 写入 RequestMsg
     * 验证 OrderID、RequestType、symbol、price 等字段正确
     */
    @Test
    void test_writeRequestMsg_fieldsCorrect() {
        reqQueue = MWMRQueue.create(TEST_REQ_KEY, 16,
                Types.REQUEST_MSG_LAYOUT.byteSize(),
                Types.QUEUE_ELEM_REQ_SIZE);

        // Java 构造 RequestMsg
        MemorySegment reqIn = Arena.global().allocate(Types.REQUEST_MSG_LAYOUT);

        // 设置字段
        Types.REQ_REQUEST_TYPE_VH.set(reqIn, 0L, Constants.REQUEST_NEWORDER);
        Types.REQ_ORD_TYPE_VH.set(reqIn, 0L, Constants.ORD_LIMIT);
        Types.REQ_ORDER_ID_VH.set(reqIn, 0L, 3_000_001);
        Types.REQ_QUANTITY_VH.set(reqIn, 0L, 5);
        Types.REQ_PRICE_VH.set(reqIn, 0L, 5500.0);
        Types.REQ_STRATEGY_ID_VH.set(reqIn, 0L, 92201);
        Types.REQ_POS_DIRECTION_VH.set(reqIn, 0L, Constants.POS_OPEN);

        // 设置 symbol (in ContractDescription, offset 32)
        String symbol = "ag2603";
        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, reqIn,
                Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symBytes.length);

        reqQueue.enqueue(reqIn);

        // 读取并验证
        MemorySegment reqOut = Arena.global().allocate(Types.REQUEST_MSG_LAYOUT);
        assertTrue(reqQueue.dequeue(reqOut));

        assertEquals(Constants.REQUEST_NEWORDER, (int) Types.REQ_REQUEST_TYPE_VH.get(reqOut, 0L));
        assertEquals(Constants.ORD_LIMIT, (int) Types.REQ_ORD_TYPE_VH.get(reqOut, 0L));
        assertEquals(3_000_001, (int) Types.REQ_ORDER_ID_VH.get(reqOut, 0L));
        assertEquals(5, (int) Types.REQ_QUANTITY_VH.get(reqOut, 0L));
        assertEquals(5500.0, (double) Types.REQ_PRICE_VH.get(reqOut, 0L), 0.001);
        assertEquals(92201, (int) Types.REQ_STRATEGY_ID_VH.get(reqOut, 0L));
        assertEquals(Constants.POS_OPEN, (int) Types.REQ_POS_DIRECTION_VH.get(reqOut, 0L));

        byte[] readSym = new byte[50];
        MemorySegment.copy(reqOut, Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET,
                MemorySegment.ofArray(readSym), 0, 50);
        assertEquals("ag2603", new String(readSym, StandardCharsets.US_ASCII).trim().replace("\0", ""));
    }

    /**
     * 10.3 模拟: Java 读取 ResponseMsg
     * 验证 OrderID、responseType、symbol、price 等字段正确
     */
    @Test
    void test_readResponseMsg_fieldsCorrect() {
        respQueue = MWMRQueue.create(TEST_RESP_KEY, 16,
                Types.RESPONSE_MSG_LAYOUT.byteSize(),
                Types.QUEUE_ELEM_RESP_SIZE);

        // 模拟 C++ counter_bridge 写入回报
        MemorySegment respIn = Arena.global().allocate(Types.RESPONSE_MSG_LAYOUT);

        Types.RESP_RESPONSE_TYPE_VH.set(respIn, 0L, Constants.RESP_TRADE_CONFIRM);
        Types.RESP_ORDER_ID_VH.set(respIn, 0L, 3_000_001);
        Types.RESP_QUANTITY_VH.set(respIn, 0L, 5);
        Types.RESP_PRICE_VH.set(respIn, 0L, 5500.0);
        Types.RESP_STRATEGY_ID_VH.set(respIn, 0L, 92201);

        // 设置 symbol (offset 41)
        String symbol = "ag2603";
        byte[] symBytes = symbol.getBytes(StandardCharsets.US_ASCII);
        MemorySegment.copy(MemorySegment.ofArray(symBytes), 0, respIn, Types.RESP_SYMBOL_OFFSET, symBytes.length);

        respQueue.enqueue(respIn);

        // Java 端读取
        MemorySegment respOut = Arena.global().allocate(Types.RESPONSE_MSG_LAYOUT);
        assertTrue(respQueue.dequeue(respOut));

        assertEquals(Constants.RESP_TRADE_CONFIRM, (int) Types.RESP_RESPONSE_TYPE_VH.get(respOut, 0L));
        assertEquals(3_000_001, (int) Types.RESP_ORDER_ID_VH.get(respOut, 0L));
        assertEquals(5, (int) Types.RESP_QUANTITY_VH.get(respOut, 0L));
        assertEquals(5500.0, (double) Types.RESP_PRICE_VH.get(respOut, 0L), 0.001);
        assertEquals(92201, (int) Types.RESP_STRATEGY_ID_VH.get(respOut, 0L));

        byte[] readSym = new byte[50];
        MemorySegment.copy(respOut, Types.RESP_SYMBOL_OFFSET, MemorySegment.ofArray(readSym), 0, 50);
        assertEquals("ag2603", new String(readSym, StandardCharsets.US_ASCII).trim().replace("\0", ""));
    }
}
