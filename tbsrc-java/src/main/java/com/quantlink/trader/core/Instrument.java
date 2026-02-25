package com.quantlink.trader.core;

import com.quantlink.trader.shm.Types;

import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;

/**
 * 20 档订单簿行情数据模型。
 * 迁移自: tbsrc/common/include/Instrument.h
 *
 * 保留中国期货核心字段，省略 CME/ICE/KRX 等交易所特定逻辑。
 */
public class Instrument {

    // ---- 合约属性 ----
    // 迁移自: Instrument.h — m_origbaseName, m_symbol, m_exchange, m_symbolID
    public String origBaseName = "";
    public String symbol = "";
    public String exchange = "";
    public int symbolID;

    // 迁移自: Instrument.h — m_tickSize, m_lotSize, m_priceFactor, m_contractFactor, m_priceMultiplier
    public double tickSize = 1.0;
    public double lotSize = 1.0;
    public double priceFactor = 1.0;
    public double contractFactor = 1.0;
    public double priceMultiplier = 1.0;
    public double multipleSize = 1.0;
    public double divisorSize = 1.0;

    // 迁移自: Instrument.h — m_sendInLots, m_perContract, m_perYield
    public boolean sendInLots = false;
    public boolean perContract = false;
    public boolean perYield = false;
    public int level = 0; // 迁移自: m_level — 当前关注档位

    // ---- 20 档订单簿 ----
    // 迁移自: Instrument.h — bidPx[20], askPx[20], bidQty[20], askQty[20]
    public final double[] bidPx = new double[20];
    public final double[] askPx = new double[20];
    public final double[] bidQty = new double[20];
    public final double[] askQty = new double[20];
    public final double[] bidOrderCount = new double[20];
    public final double[] askOrderCount = new double[20];
    public int bookDepth = 20;

    // ---- 成交数据 ----
    // 迁移自: Instrument.h — lastTradePx, lastTradeqty, totalTradedQty 等
    public double lastTradePx;
    public double lastTradeQty;
    public double totalTradedQty;
    public double totalTradedValue;
    public double prevTotalTradedQty;
    public double prevTotalTradedValue;
    public double initTotalTradedQty;
    public double initTotalTradedValue;
    public double avgPrice;
    public double totalL1Event;

    // ---- 时间戳 ----
    // 迁移自: Instrument.h — lastLocalTime, lastExchTime
    public long lastLocalTime;
    public long lastExchTime;
    public long lastRptSeqNum;

    // ---- 日期 ----
    public String currDate = "";

    /**
     * 从 MarketUpdateNew MemorySegment 填充 20 档订单簿。
     * 迁移自: Instrument::FillOrderBook(MarketUpdateNew*)
     *
     * @param mdUpdate MarketUpdateNew MemorySegment (816 bytes)
     */
    public void fillOrderBook(MemorySegment mdUpdate) {
        long mdDataBase = Types.MU_DATA_OFFSET; // 96
        // C++: instru->FillOrderBook(update) — 遍历 bidUpdates[20] / askUpdates[20]
        for (long i = 0; i < 20; i++) {
            bidPx[Math.toIntExact(i)] = (double) Types.MDD_BID_PRICE_VH.get(mdUpdate, mdDataBase, i);
            bidQty[Math.toIntExact(i)] = (int) Types.MDD_BID_QUANTITY_VH.get(mdUpdate, mdDataBase, i);
            askPx[Math.toIntExact(i)] = (double) Types.MDD_ASK_PRICE_VH.get(mdUpdate, mdDataBase, i);
            askQty[Math.toIntExact(i)] = (int) Types.MDD_ASK_QUANTITY_VH.get(mdUpdate, mdDataBase, i);
        }

        // 读取 LTP
        // C++: update->m_lastTradedPrice (MDDataPart)
        lastTradePx = (double) Types.MDD_LAST_TRADED_PRICE_VH.get(mdUpdate, mdDataBase);
    }

    /**
     * 读取 MarketUpdateNew 中的 symbol 字段。
     * 迁移自: update->m_symbol (MDHeaderPart offset 40, char[48])
     */
    public static String readSymbol(MemorySegment mdUpdate) {
        byte[] buf = new byte[48];
        MemorySegment.copy(mdUpdate, Types.MDH_SYMBOL_OFFSET, MemorySegment.ofArray(buf), 0, 48);
        return new String(buf, StandardCharsets.US_ASCII).trim().replace("\0", "");
    }

    /**
     * 读取 MarketUpdateNew 中的 symbolID 字段。
     * 迁移自: update->m_symbolID (MDHeaderPart)
     */
    public static int readSymbolID(MemorySegment mdUpdate) {
        // MDH_SYMBOL_ID_VH is JAVA_SHORT (uint16_t m_symbolID)
        return Short.toUnsignedInt((short) Types.MDH_SYMBOL_ID_VH.get(mdUpdate, 0L));
    }

    // ---- 价格计算方法 ----
    // 迁移自: Instrument.h — calculate_MIDPrice(), calculate_MSWPrice(), calculate_LTPPrice()

    /**
     * 中间价。
     * C++: MIDPrice_ = (bidPx[0] + askPx[0]) / 2.0
     */
    public double calculateMIDPrice() {
        return (bidPx[0] + askPx[0]) / 2.0;
    }

    /**
     * 量加权中间价 (Market Size Weighted)。
     * C++: MSWPrice_ = (askQty[0]*bidPx[0] + askPx[0]*bidQty[0]) / (askQty[0]+bidQty[0])
     */
    public double calculateMSWPrice() {
        double totalQty = askQty[0] + bidQty[0];
        if (totalQty <= 0) return calculateMIDPrice();
        return (askQty[0] * bidPx[0] + askPx[0] * bidQty[0]) / totalQty;
    }

    /**
     * LTP 价格（约束在 bid-ask 范围内）。
     * C++: if (lastTradePx != 0) LTPPrice_ = clamp(lastTradePx, bidPx[0], askPx[0])
     */
    public double calculateLTPPrice() {
        if (lastTradePx == 0) return calculateMIDPrice();
        if (lastTradePx < bidPx[0]) return bidPx[0];
        if (lastTradePx > askPx[0]) return askPx[0];
        return lastTradePx;
    }

    /**
     * MSW-MID 混合价格。
     * C++: calculate_MSWMIDPrice — 当价差 > tickSize 时用 MID，否则用 MSW
     */
    public double calculateMSWMIDPrice() {
        double msw = calculateMSWPrice();
        if (askPx[0] - bidPx[0] > tickSize + 0.0001) {
            return calculateMIDPrice();
        }
        return msw;
    }

    /**
     * 重置订单簿和交易数据。
     * 迁移自: Instrument::Reset()
     */
    public void reset() {
        java.util.Arrays.fill(bidPx, 0.0);
        java.util.Arrays.fill(askPx, 0.0);
        java.util.Arrays.fill(bidQty, 0.0);
        java.util.Arrays.fill(askQty, 0.0);
        java.util.Arrays.fill(bidOrderCount, 0.0);
        java.util.Arrays.fill(askOrderCount, 0.0);
        lastTradePx = 0;
        lastTradeQty = 0;
        totalTradedQty = 0;
        totalTradedValue = 0;
        totalL1Event = 0;
        lastLocalTime = 0;
        lastExchTime = 0;
    }

    @Override
    public String toString() {
        return String.format("Instrument[%s, bid=%.2f×%.0f, ask=%.2f×%.0f, ltp=%.2f]",
                origBaseName, bidPx[0], bidQty[0], askPx[0], askQty[0], lastTradePx);
    }
}
