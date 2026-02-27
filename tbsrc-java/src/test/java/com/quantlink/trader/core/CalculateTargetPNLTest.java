package com.quantlink.trader.core;

import com.quantlink.trader.indicator.Dependant;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * CalculateTargetPNL 单元测试。
 * 验证 MKTW_PX2/MKTW_PX 价格模式、PNL 计算、CHECK_PNL 逻辑、指标无效返回 false。
 */
class CalculateTargetPNLTest {

    private Instrument inst;
    private SimConfig simConfig;

    @BeforeEach
    void setup() {
        inst = new Instrument();
        inst.tickSize = 1.0;
        inst.priceMultiplier = 15.0; // ag 品种
        inst.bidPx[0] = 5000.0;
        inst.askPx[0] = 5002.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 20;

        simConfig = new SimConfig();
        simConfig.instrument = inst;
        simConfig.useStratBook = false;
        simConfig.buyExchTx = 0.0;
        simConfig.sellExchTx = 0.0;
        simConfig.buyExchContractTx = 0.0;
        simConfig.sellExchContractTx = 0.0;
        simConfig.index = 0;
        simConfig.lastInstruMapInstrument = inst;
    }

    // =======================================================================
    //  MKTW_PX2 模式测试
    // =======================================================================

    @Test
    void testMKTWPX2BasicCalculation() {
        // 创建 Dependant + 一个非 dep 指标
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        // 非 dep 指标: coefficient=0.5, diffValue will be 0 (fresh start)
        IndElem nonDep = new IndElem();
        nonDep.baseName = "ind1";
        nonDep.indName = "BookDelta";
        nonDep.argList[1] = "MKTW_PX2";
        nonDep.coefficient = 0.5;
        nonDep.index = 0;
        nonDep.indicator = createSimpleIndicator(0, false, 0);
        indList.add(nonDep);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        assertEquals(CalculateTargetPNL.MKTW_PX2, calc.getPriceType());

        // 更新 Dependant 值
        indList.get(0).indicator.isValid = true;
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] depPriceOut = new double[1];
        double[] targetPriceOut = new double[1];
        double[] targetBidPNL = new double[5];
        double[] targetAskPNL = new double[5];

        boolean result = calc.calculateTargetPNL(depPriceOut, targetPriceOut, targetBidPNL, targetAskPNL);
        assertTrue(result); // CHECK_PNL=false → always true

        // depPrice should be the MSW price
        assertTrue(depPriceOut[0] > 0);
        assertTrue(targetPriceOut[0] > 0);
    }

    @Test
    void testMKTWPX2TargetPriceWithIndicatorDiff() {
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        // 非 dep 指标: coefficient=1.0, will produce a diffValue
        Indicator nonDepInd = createSimpleIndicator(0, false, 0);
        nonDepInd.value = 10.0;
        nonDepInd.isValid = true;

        IndElem nonDep = new IndElem();
        nonDep.baseName = "ind1";
        nonDep.indName = "Signal";
        nonDep.argList[1] = "MKTW_PX2";
        nonDep.coefficient = 2.0;
        nonDep.index = 0;
        nonDep.indicator = nonDepInd;
        indList.add(nonDep);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);

        // Update Dependant
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] depPriceOut = new double[1];
        double[] targetPriceOut = new double[1];
        double[] targetBidPNL = new double[5];
        double[] targetAskPNL = new double[5];

        calc.calculateTargetPNL(depPriceOut, targetPriceOut, targetBidPNL, targetAskPNL);

        // After calculate(): diffValue = value(10) - lastValue(0) = 10
        // pxOffset += coefficient(2.0) * diffValue(10) = 20
        // In MKTW_PX2: val *= tickSize(1) → val = 20
        // targetPrice = depPrice + pxOffset = depPrice + 20
        assertEquals(depPriceOut[0] + 20.0, targetPriceOut[0], 0.01);
    }

    // =======================================================================
    //  MKTW_PX 模式测试
    // =======================================================================

    @Test
    void testMKTWPXBasicCalculation() {
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX"));

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        assertEquals(CalculateTargetPNL.MKTW_PX, calc.getPriceType());

        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] depPriceOut = new double[1];
        double[] targetPriceOut = new double[1];
        double[] targetBidPNL = new double[5];
        double[] targetAskPNL = new double[5];

        boolean result = calc.calculateTargetPNL(depPriceOut, targetPriceOut, targetBidPNL, targetAskPNL);
        assertTrue(result);

        // With no non-dep indicators, pxOffset=0 → target = depPrice + (0 * depPrice / 10000) = depPrice
        assertEquals(depPriceOut[0], targetPriceOut[0], 0.01);
    }

    @Test
    void testMKTWPXWithBasisPointOffset() {
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MID_PX"));

        // Non-dep indicator with value that produces pxOffset
        Indicator nonDepInd = createSimpleIndicator(0, false, 0);
        nonDepInd.value = 100.0; // Will produce diffValue=100 on first calculate()
        nonDepInd.isValid = true;

        IndElem nonDep = new IndElem();
        nonDep.baseName = "ind1";
        nonDep.indName = "Signal";
        nonDep.argList[1] = "MID_PX";
        nonDep.coefficient = 1.0;
        nonDep.index = 0;
        nonDep.indicator = nonDepInd;
        indList.add(nonDep);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        assertEquals(CalculateTargetPNL.MKTW_PX, calc.getPriceType());

        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] depPriceOut = new double[1];
        double[] targetPriceOut = new double[1];
        double[] targetBidPNL = new double[5];
        double[] targetAskPNL = new double[5];

        calc.calculateTargetPNL(depPriceOut, targetPriceOut, targetBidPNL, targetAskPNL);

        // MKTW_PX: targetPrice = depPrice + (pxOffset * depPrice / 10000)
        // pxOffset = coefficient(1) * diffValue(100) = 100 (no tickSize multiplication in PX mode)
        // targetPrice = depPrice + (100 * depPrice / 10000) = depPrice * (1 + 0.01)
        double expectedTarget = depPriceOut[0] + (100.0 * depPriceOut[0] / 10000.0);
        assertEquals(expectedTarget, targetPriceOut[0], 0.01);
    }

    // =======================================================================
    //  CHECK_PNL 测试
    // =======================================================================

    @Test
    void testCheckPNLFalseAlwaysReturnsTrue() {
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));
        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CHECK_PNL = false;
        simConfig.thresholdSet.CONST = 0.0;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        assertTrue(calc.calculateTargetPNL(dp, tp, bidPNL, askPNL));
    }

    @Test
    void testCheckPNLTrueWithProfitableLevel() {
        // Set up a scenario where target price > bid price (profitable bid)
        inst.bidPx[0] = 5000.0;
        inst.askPx[0] = 5010.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 10;

        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        // Create indicator that pushes target up significantly
        Indicator pushUp = createSimpleIndicator(0, false, 0);
        pushUp.value = 50.0; // Will create pxOffset = 50 * tickSize = 50
        pushUp.isValid = true;
        IndElem nonDep = new IndElem();
        nonDep.coefficient = 1.0;
        nonDep.index = 0;
        nonDep.argList[1] = "MKTW_PX2";
        nonDep.indicator = pushUp;
        indList.add(nonDep);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CHECK_PNL = true;
        simConfig.thresholdSet.MAX_QUOTE_LEVEL = 1;
        simConfig.thresholdSet.CONST = 0.0;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        boolean result = calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        // target should be pushed up by +50, making bid PNL positive
        assertTrue(result);
        assertTrue(bidPNL[0] > 0);
    }

    @Test
    void testCheckPNLTrueNoProfitReturnsFalse() {
        // target ~= depPrice, costs will make PNL negative
        inst.bidPx[0] = 5000.0;
        inst.askPx[0] = 5001.0;
        inst.bidQty[0] = 10;
        inst.askQty[0] = 10;

        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CHECK_PNL = true;
        simConfig.thresholdSet.MAX_QUOTE_LEVEL = 1;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.buyExchContractTx = 10.0; // Large contract costs
        simConfig.sellExchContractTx = 10.0;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        boolean result = calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        // With high costs and no pxOffset, both bid and ask PNL should be negative
        assertFalse(result);
    }

    // =======================================================================
    //  指标无效返回 false 测试
    // =======================================================================

    @Test
    void testInvalidIndicatorReturnsFalse() {
        List<IndElem> indList = new ArrayList<>();
        IndElem depElem = createDepElem(inst, "MKTW_PX2");
        // Set dep indicator to invalid
        depElem.indicator.isValid = false;
        indList.add(depElem);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = true;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        boolean result = calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        assertFalse(result); // Invalid indicator → return false
    }

    @Test
    void testInvalidNonDepIndicatorReturnsFalse() {
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        // Set Dependant to valid
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        // Add invalid non-dep indicator
        Indicator invalidInd = createSimpleIndicator(0, false, 0);
        invalidInd.isValid = false; // explicitly invalid
        IndElem nonDep = new IndElem();
        nonDep.baseName = "bad_ind";
        nonDep.indName = "BadSignal";
        nonDep.argList[1] = "MKTW_PX2";
        nonDep.coefficient = 1.0;
        nonDep.index = 0;
        nonDep.indicator = invalidInd;
        indList.add(nonDep);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = true;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        boolean result = calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        assertFalse(result); // Non-dep indicator invalid → return false
    }

    // =======================================================================
    //  CONST offset 测试
    // =======================================================================

    @Test
    void testCONSTNegativeProtection() {
        // C++: this.targetPrice = (target + constOffset < 0) ? 0 : target;
        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = -10000.0; // Very large negative CONST
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        // target + CONST < 0 → targetPrice = 0
        assertEquals(0.0, tp[0], 1e-10);
    }

    // =======================================================================
    //  RATIO 模式测试
    // =======================================================================

    @Test
    void testRATIOPriceType() {
        List<IndElem> indList = new ArrayList<>();
        IndElem depElem = createDepElem(inst, "MKTW_RATIO");
        indList.add(depElem);

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = false;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        assertEquals(CalculateTargetPNL.RATIO, calc.getPriceType());
    }

    // =======================================================================
    //  StratBook 路径测试
    // =======================================================================

    @Test
    void testUseStratBookPath() {
        inst.bidPxStrat[0] = 4990.0;
        inst.askPxStrat[0] = 4992.0;
        inst.bidQtyStrat[0] = 5;
        inst.askQtyStrat[0] = 15;

        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.useStratBook = true;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = true;
        simConfig.thresholdSet.MAX_QUOTE_LEVEL = 1;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        // Should use strat book for bid/ask base prices
        assertTrue(tp[0] > 0);
    }

    // =======================================================================
    //  perYield (bond) 路径测试
    // =======================================================================

    @Test
    void testPerYieldBondPricePNL() {
        inst.perYield = true;
        inst.cDays = 252;

        List<IndElem> indList = new ArrayList<>();
        indList.add(createDepElem(inst, "MKTW_PX2"));

        simConfig.indicatorList = indList;
        inst.indList = indList;
        simConfig.thresholdSet.CONST = 0.0;
        simConfig.thresholdSet.CHECK_PNL = true;
        simConfig.thresholdSet.MAX_QUOTE_LEVEL = 1;
        simConfig.buyExchTx = 0.001;
        simConfig.sellExchTx = 0.001;

        CalculateTargetPNL calc = new CalculateTargetPNL(simConfig);
        Dependant dep = (Dependant) indList.get(0).indicator;
        dep.orderBookUpdate();

        double[] dp = new double[1], tp = new double[1];
        double[] bidPNL = new double[5], askPNL = new double[5];

        calc.calculateTargetPNL(dp, tp, bidPNL, askPNL);
        // Just verify it doesn't crash and produces some values
        assertNotNull(bidPNL);
        assertNotNull(askPNL);
    }

    // =======================================================================
    //  Helpers
    // =======================================================================

    private IndElem createDepElem(Instrument instrument, String style) {
        IndElem elem = new IndElem();
        elem.baseName = instrument.origBaseName;
        elem.indName = "Dependant";
        elem.argList[1] = style;
        elem.coefficient = 1.0;
        elem.index = 0;
        elem.indicator = new Dependant(instrument, style);
        return elem;
    }

    private Indicator createSimpleIndicator(double index, boolean isDep, int strategyIndex) {
        Indicator ind = new Indicator() {
            @Override public void quoteUpdate() {}
            @Override public void tickUpdate() {}
            @Override public void reset() { value = 0; isValid = false; }
        };
        ind.index = index;
        ind.isDep = isDep;
        ind.strategyIndex = strategyIndex;
        ind.isValid = true;
        return ind;
    }
}
