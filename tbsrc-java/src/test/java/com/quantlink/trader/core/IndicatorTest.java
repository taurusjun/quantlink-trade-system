package com.quantlink.trader.core;

import com.quantlink.trader.indicator.Dependant;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Indicator 基类 + Dependant 指标单元测试。
 * 验证 calculate()、getValue()、getDiffValue() 以及 Dependant 各价格类型。
 */
class IndicatorTest {

    private Instrument inst;

    @BeforeEach
    void setup() {
        inst = new Instrument();
        inst.tickSize = 1.0;
    }

    // =======================================================================
    //  Indicator.calculate() 测试
    // =======================================================================

    @Test
    void testCalculateDiffValue() {
        // 创建一个简单的 Indicator 子类用于测试
        Indicator ind = createTestIndicator();
        ind.value = 100.0;
        ind.isValid = true;

        ind.calculate();
        // diff = value - lastValue = 100 - 0 = 100
        assertEquals(100.0, ind.getDiffValue(), 1e-10);
        assertEquals(100.0, ind.lastValue, 1e-10);

        // 第二次: value=105, diff = 105 - 100 = 5
        ind.value = 105.0;
        ind.calculate();
        assertEquals(5.0, ind.getDiffValue(), 1e-10);
        assertEquals(105.0, ind.lastValue, 1e-10);
    }

    @Test
    void testCalculateInvalidSetsValueToZero() {
        // C++: if (!isValid) value = 0.0;
        Indicator ind = createTestIndicator();
        ind.value = 50.0;
        ind.isValid = false;

        ind.calculate();
        assertEquals(0.0, ind.value, 1e-10);
        assertEquals(0.0, ind.getDiffValue(), 1e-10);
    }

    @Test
    void testCalculateMultipleRounds() {
        Indicator ind = createTestIndicator();

        // Round 1: value=10
        ind.value = 10.0;
        ind.isValid = true;
        ind.calculate();
        assertEquals(10.0, ind.getDiffValue(), 1e-10);

        // Round 2: value=15
        ind.value = 15.0;
        ind.calculate();
        assertEquals(5.0, ind.getDiffValue(), 1e-10);

        // Round 3: value=12 (decrease)
        ind.value = 12.0;
        ind.calculate();
        assertEquals(-3.0, ind.getDiffValue(), 1e-10);
    }

    // =======================================================================
    //  Dependant 价格类型测试
    // =======================================================================

    @Test
    void testDependantMSWPrice() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MKTW_PX2");
        assertEquals(Dependant.MKTW_PX2, dep.getStyle());

        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // MSW = (askQty*bidPx + askPx*bidQty) / (askQty+bidQty)
        // = (20*100 + 102*10) / (20+10) = (2000+1020)/30 = 100.6667
        assertEquals(100.6667, dep.value, 0.001);
    }

    @Test
    void testDependantMIDPrice() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MID_PX2");
        assertEquals(Dependant.MID_PX2, dep.getStyle());

        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // MID = (100+102)/2 = 101
        assertEquals(101.0, dep.value, 1e-10);
    }

    @Test
    void testDependantMSWMIDPrice() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MKTMID_PX2");
        assertEquals(Dependant.MKTMID_PX2, dep.getStyle());

        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // askPx - bidPx = 2 > tickSize(1) + 0.0001 → use MID = 101
        assertEquals(101.0, dep.value, 1e-10);
    }

    @Test
    void testDependantMSWMIDPriceTightSpread() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 101.0; // spread = 1 tick
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MKTMID_PX2");
        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // spread = 1 <= tickSize(1) + 0.0001 → use MSW
        double expectedMSW = (20 * 100.0 + 101.0 * 10) / 30.0;
        assertEquals(expectedMSW, dep.value, 0.001);
    }

    @Test
    void testDependantWGTPrice() {
        // 3 档数据
        inst.bidPx[0] = 100.0; inst.bidQty[0] = 10;
        inst.bidPx[1] = 99.0;  inst.bidQty[1] = 20;
        inst.bidPx[2] = 98.0;  inst.bidQty[2] = 30;
        inst.askPx[0] = 101.0; inst.askQty[0] = 15;
        inst.askPx[1] = 102.0; inst.askQty[1] = 25;
        inst.askPx[2] = 103.0; inst.askQty[2] = 35;

        Dependant dep = new Dependant(inst, "WGT_PX");
        assertEquals(Dependant.WGT_PX, dep.getStyle());

        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // WGT price uses top 3 levels weighted
        assertTrue(dep.value > 99.0 && dep.value < 103.0);
    }

    @Test
    void testDependantLTPPrice() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;
        inst.lastTradePx = 101.5;

        Dependant dep = new Dependant(inst, "LTP_PX");
        assertEquals(Dependant.LTP_PX, dep.getStyle());

        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        // LTP clamped to [bid, ask]
        assertEquals(101.5, dep.value, 1e-10);
    }

    @Test
    void testDependantLTPClampedToBid() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;
        inst.lastTradePx = 99.0; // below bid

        Dependant dep = new Dependant(inst, "LTP_PX");
        dep.orderBookUpdate();
        assertEquals(100.0, dep.value, 1e-10);
    }

    // =======================================================================
    //  isValid 边界测试
    // =======================================================================

    @Test
    void testDependantInvalidWhenBidZero() {
        inst.bidPx[0] = 0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MKTW_PX2");
        dep.orderBookUpdate();
        assertFalse(dep.isValid);
    }

    @Test
    void testDependantInvalidWhenAskZero() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MID_PX2");
        dep.orderBookUpdate();
        assertFalse(dep.isValid);
    }

    @Test
    void testDependantReset() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MKTW_PX2");
        dep.orderBookUpdate();
        assertTrue(dep.isValid);
        assertTrue(dep.value > 0);

        dep.reset();
        assertFalse(dep.isValid);
        assertEquals(0.0, dep.value, 1e-10);
    }

    @Test
    void testDependantStratBookUpdate() {
        inst.bidPxStrat[0] = 200.0;
        inst.askPxStrat[0] = 204.0;
        inst.bidQtyStrat[0] = 5;
        inst.askQtyStrat[0] = 15;

        Dependant dep = new Dependant(inst, "MKTW_PX2");
        dep.orderBookStratUpdate();
        assertTrue(dep.isValid);
        // MSW from strat book: (15*200 + 204*5) / (15+5) = (3000+1020)/20 = 201
        assertEquals(201.0, dep.value, 0.001);
    }

    @Test
    void testDependantStratBookInvalid() {
        inst.bidPxStrat[0] = 0;
        inst.askPxStrat[0] = 204.0;

        Dependant dep = new Dependant(inst, "MKTW_PX2");
        dep.orderBookStratUpdate();
        assertFalse(dep.isValid);
    }

    @Test
    void testDependantQuoteUpdateCallsOrderBookUpdate() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MID_PX2");
        dep.quoteUpdate();
        assertTrue(dep.isValid);
        assertEquals(101.0, dep.value, 1e-10);
    }

    @Test
    void testDependantTickUpdateCallsOrderBookUpdate() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        Dependant dep = new Dependant(inst, "MID_PX2");
        dep.tickUpdate();
        assertTrue(dep.isValid);
        assertEquals(101.0, dep.value, 1e-10);
    }

    @Test
    void testDependantStyleMapping() {
        // MKTW_PX maps to MKTW_PX2
        assertEquals(Dependant.MKTW_PX2, new Dependant(inst, "MKTW_PX").getStyle());
        assertEquals(Dependant.MKTW_PX2, new Dependant(inst, "MKTW_PX2").getStyle());
        assertEquals(Dependant.MKTW_PX2, new Dependant(inst, "MKTW_RATIO").getStyle());

        // MID_PX maps to MID_PX2
        assertEquals(Dependant.MID_PX2, new Dependant(inst, "MID_PX").getStyle());
        assertEquals(Dependant.MID_PX2, new Dependant(inst, "MID_PX2").getStyle());
        assertEquals(Dependant.MID_PX2, new Dependant(inst, "MID_RATIO").getStyle());

        assertEquals(Dependant.MKTMID_PX2, new Dependant(inst, "MKTMID_PX2").getStyle());
        assertEquals(Dependant.WGT_PX, new Dependant(inst, "WGT_PX").getStyle());
        assertEquals(Dependant.LTP_PX, new Dependant(inst, "LTP_PX").getStyle());

        // Unknown defaults to MID
        assertEquals(Dependant.MID_PX2, new Dependant(inst, "UNKNOWN_PX").getStyle());
    }

    // =======================================================================
    //  IndElem 测试
    // =======================================================================

    @Test
    void testIndElemDefaults() {
        IndElem elem = new IndElem();
        assertEquals("", elem.baseName);
        assertEquals("", elem.type);
        assertEquals("", elem.indName);
        assertEquals(0, elem.coefficient);
        assertEquals(0, elem.index);
        assertEquals(0, elem.argCount);
        assertNull(elem.indicator);
        // argList should be initialized to empty strings
        for (String s : elem.argList) {
            assertEquals("", s);
        }
    }

    // =======================================================================
    //  Instrument 价格类型测试
    // =======================================================================

    @Test
    void testInstrumentSubscribeAndGetTBPriceType() {
        inst.bidPx[0] = 100.0;
        inst.askPx[0] = 102.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        inst.subscribeTBPriceType(Dependant.MKTW_PX2);
        double msw = inst.getTBPriceType(Dependant.MKTW_PX2);
        double expectedMSW = (20 * 100.0 + 102.0 * 10) / 30.0;
        assertEquals(expectedMSW, msw, 0.001);
    }

    @Test
    void testInstrumentGetTBStratPriceType() {
        inst.bidPxStrat[0] = 200.0;
        inst.askPxStrat[0] = 204.0;
        inst.bidQtyStrat[0] = 5;
        inst.askQtyStrat[0] = 15;

        inst.subscribeTBPriceType(Dependant.MKTW_PX2);
        double msw = inst.getTBStratPriceType(Dependant.MKTW_PX2);
        // MSW from strat: (15*200+204*5)/(15+5) = (3000+1020)/20 = 201.0
        assertEquals(201.0, msw, 0.001);
    }

    // =======================================================================
    //  Helper
    // =======================================================================

    private Indicator createTestIndicator() {
        return new Indicator() {
            @Override public void quoteUpdate() {}
            @Override public void tickUpdate() {}
            @Override public void reset() { value = 0; isValid = false; }
        };
    }
}
