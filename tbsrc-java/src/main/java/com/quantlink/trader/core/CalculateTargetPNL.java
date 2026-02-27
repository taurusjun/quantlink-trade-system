package com.quantlink.trader.core;

import java.util.List;
import java.util.logging.Logger;

/**
 * 公允价计算引擎 — 基于指标差值计算 targetPrice 和各档 PNL。
 * 迁移自: tbsrc/main/TradeBotUtils.cpp:3749-4025 — class CalculatePNL
 *         tbsrc/main/include/TradeBotUtils.h:506 — class CalculatePNL
 *
 * 核心算法:
 * 1. depPrice = Dependant 指标值（当前市价）
 * 2. targetPrice = depPrice + Σ(coefficient_i × indicator_diff_i × tickSize)  [MKTW_PX2 模式]
 *    或 targetPrice += pxOffset × depPrice / 10000                           [MKTW_PX 模式]
 * 3. targetBidPNL[i] = (targetPrice - bidPx[i] - 交易成本) × priceMultiplier
 * 4. targetAskPNL[i] = (askPx[i] - targetPrice - 交易成本) × priceMultiplier
 */
public class CalculateTargetPNL {

    private static final Logger log = Logger.getLogger(CalculateTargetPNL.class.getName());

    // 价格模式常量
    // 迁移自: TradeBotUtils.h — enum PriceType
    public static final int MKTW_PX = 0;
    public static final int MKTW_PX2 = 1;
    public static final int RATIO = 2;
    public static final int VOL = 3;

    // 迁移自: CalculatePNL 成员变量 — TradeBotUtils.h:508-520
    private final SimConfig simConfig;
    private final List<IndElem> indicatorList;
    private final Instrument instru;
    private final int indCount;
    private final int priceType;
    private final double constOffset;

    // 迁移自: CalculatePNL::m_lasttargetpx, m_lastTS, m_indVal
    private double lastTargetPx = 0;
    private final double[] indVal;

    // 输出缓存（供外部读取，类似 C++ 的 m_depPrice, m_targetPrice）
    private double depPrice;
    private double targetPrice;

    /**
     * 构造 CalculateTargetPNL。
     * 迁移自: CalculatePNL::CalculatePNL(SimConfig *simConfig) — TradeBotUtils.cpp:3749-3776
     *
     * @param simConfig 策略配置（包含 indicatorList, instrument, thresholdSet 等）
     */
    public CalculateTargetPNL(SimConfig simConfig) {
        this.simConfig = simConfig;
        this.indicatorList = simConfig.indicatorList;
        this.instru = simConfig.instrument;
        this.indCount = indicatorList.size();
        this.indVal = new double[indCount];

        // C++: m_depIter = m_indicatorList->begin();
        // 确定价格模式 — 从第一个 IndElem 的 argList[1] 读取
        // Ref: TradeBotUtils.cpp:3764-3773
        IndElem depElem = indicatorList.get(0);
        String priceStyle = depElem.argList[1];

        if ("MKTW_PX".equals(priceStyle) || "MID_PX".equals(priceStyle)) {
            priceType = MKTW_PX;
        } else if ("MKTW_PX2".equals(priceStyle) || "MID_PX2".equals(priceStyle)
                || "MKTMID_PX2".equals(priceStyle) || "WGT_PX".equals(priceStyle)
                || "LTP_PX".equals(priceStyle)) {
            priceType = MKTW_PX2;
        } else if ("MKTW_RATIO".equals(priceStyle) || "MID_RATIO".equals(priceStyle)) {
            priceType = RATIO;
        } else {
            // 默认 MKTW_PX2
            priceType = MKTW_PX2;
        }

        // C++: m_CONST = simConfig->m_tholdSet.CONST;
        this.constOffset = simConfig.thresholdSet.CONST;
    }

    /**
     * 获取价格模式。
     */
    public int getPriceType() {
        return priceType;
    }

    /**
     * 计算目标价格和各档 PNL。
     * 迁移自: CalculatePNL::CalculateTargetPNL() — TradeBotUtils.cpp:3788-4025
     *
     * @param depPriceOut    输出: depPriceOut[0] = 当前市价（Dependant 指标值）
     * @param targetPriceOut 输出: targetPriceOut[0] = 目标价
     * @param targetBidPNL   输出: 各档买入 PNL
     * @param targetAskPNL   输出: 各档卖出 PNL
     * @return true 如果有正 PNL 的档位（即有利可图），或 CHECK_PNL=false
     */
    public boolean calculateTargetPNL(double[] depPriceOut, double[] targetPriceOut,
                                       double[] targetBidPNL, double[] targetAskPNL) {

        // C++: BidpxAtThisLevel, AskpxAtThisLevel 初始化
        // Ref: TradeBotUtils.cpp:3794-3803
        double bidPxBase, askPxBase;
        if (!simConfig.useStratBook) {
            bidPxBase = instru.bidPx[0] + instru.tickSize;
            askPxBase = instru.askPx[0] - instru.tickSize;
        } else {
            bidPxBase = instru.bidPxStrat[0] + instru.tickSize;
            askPxBase = instru.askPxStrat[0] - instru.tickSize;
        }

        // C++: depPrice = (*m_depIter)->m_indicator->value;
        // Ref: TradeBotUtils.cpp:3807
        IndElem depElem = indicatorList.get(0);
        this.depPrice = depElem.indicator.value;
        double target = this.depPrice;

        // C++: double pxOffset = m_lasttargetpx;
        double pxOffset = lastTargetPx;

        // C++: if (m_priceType == RATIO) targetPrice = 1;
        if (priceType == RATIO) {
            target = 1;
        }

        // ---- 非 VOL 路径: 遍历指标计算 pxOffset ----
        // Ref: TradeBotUtils.cpp:3816-3837
        if (priceType != VOL) {
            // C++: 遍历 m_simConfig->m_lastInstruMapIter->second->m_indList
            // Java: 遍历 simConfig.lastInstruMapInstrument.indList（如果有的话）
            // 或遍历全局 indicatorList
            Instrument lastInstru = simConfig.lastInstruMapInstrument;
            List<IndElem> indList = (lastInstru != null && lastInstru.indList != null)
                    ? lastInstru.indList : indicatorList;

            for (IndElem elem : indList) {
                if (!elem.indicator.isDep && elem.indicator.index == simConfig.index) {
                    if (priceType == RATIO) {
                        // C++: targetPrice *= ((*iter)->m_coefficient) * (*iter)->m_indicator->Value(status);
                        target *= elem.coefficient * elem.indicator.getValue();
                    }
                    if (priceType == MKTW_PX2 || priceType == MKTW_PX) {
                        // C++: (*iter)->m_indicator->Calculate();
                        elem.indicator.calculate();
                        // C++: value = (*iter)->m_coefficient * (*iter)->m_indicator->diffValue(status);
                        double val = elem.coefficient * elem.indicator.getDiffValue();
                        // C++: m_indVal[(*iter)->m_index] += value;
                        if (elem.index >= 0 && elem.index < indVal.length) {
                            indVal[elem.index] += val;
                        }
                        // C++: if (m_priceType == MKTW_PX2) value *= m_instru->m_tickSize;
                        if (priceType == MKTW_PX2) {
                            val *= instru.tickSize;
                        }
                        // C++: pxOffset += value;
                        pxOffset += val;
                    }
                }
            }
        }

        // ---- 根据价格模式计算最终 targetPrice ----
        // Ref: TradeBotUtils.cpp:3845-3855
        if (priceType == MKTW_PX) {
            // C++: targetPrice += (pxOffset * depPrice / 10000);
            target += (pxOffset * this.depPrice / 10000.0);
        } else if (priceType == MKTW_PX2) {
            // C++: targetPrice += pxOffset;
            target += pxOffset;
        }
        // [C++差异] VOL 模式（期权）和 RATIO 模式的最终 target 计算省略 — 中国期货不使用
        // 参见: TradeBotUtils.cpp:3856-3885

        // C++: targetPrice = targetPrice + m_CONST < 0 ? 0 : targetPrice;
        // Ref: TradeBotUtils.cpp:3887
        this.targetPrice = (target + constOffset < 0) ? 0 : target;

        // C++: m_lasttargetpx = pxOffset;
        lastTargetPx = pxOffset;

        // 输出
        depPriceOut[0] = this.depPrice;
        targetPriceOut[0] = this.targetPrice;

        // ---- 验证所有指标有效性 ----
        // Ref: TradeBotUtils.cpp:3903-3912
        for (IndElem elem : indicatorList) {
            if (!elem.indicator.isValid) {
                log.fine("Invalid Indicator Value: " + elem.baseName + " " + elem.indName
                        + " " + elem.argList[1] + " " + elem.indicator.getValue());
                return false;
            }
        }

        // ---- CHECK_PNL=false 时直接返回 true ----
        // Ref: TradeBotUtils.cpp:3969-3970
        if (!simConfig.thresholdSet.CHECK_PNL) {
            return true;
        }

        // ---- 计算各档 PNL ----
        // Ref: TradeBotUtils.cpp:3972-4024
        boolean retValBid = false, retValAsk = false;
        double bidPxAtLevel = bidPxBase;
        double askPxAtLevel = askPxBase;

        for (int i = 0; i < simConfig.thresholdSet.MAX_QUOTE_LEVEL; i++) {
            // C++: if (m_simConfig->m_bUseStratBook) { bidPxAtLevel = instru->bidPxStrat[i]; ... }
            // Ref: TradeBotUtils.cpp:3979-3988
            if (simConfig.useStratBook) {
                bidPxAtLevel = instru.bidPxStrat[i];
                askPxAtLevel = instru.askPxStrat[i];
            } else if (i > 0) {
                bidPxAtLevel = instru.bidPx[i];
                askPxAtLevel = instru.askPx[i];
            }

            if (retValBid) {
                // C++: targetBidPNL[i] = 1.0;
                targetBidPNL[i] = 1.0;
            } else {
                if (instru.perYield) {
                    // C++: targetBidPNL[i] = BondPrice(bid) - BondPrice(target) - costs
                    targetBidPNL[i] = (Instrument.bondPrice(bidPxAtLevel, instru.cDays)
                            - Instrument.bondPrice(this.targetPrice, instru.cDays))
                            - (simConfig.buyExchTx + simConfig.sellExchTx);
                } else {
                    // C++: targetBidPNL[i] = (targetPrice - BidPx - buyTx*BidPx - sellTx*targetPrice) * multiplier - contractCosts
                    // Ref: TradeBotUtils.cpp:3999
                    targetBidPNL[i] = (this.targetPrice - bidPxAtLevel
                            - simConfig.buyExchTx * bidPxAtLevel
                            - simConfig.sellExchTx * this.targetPrice)
                            * instru.priceMultiplier
                            - (simConfig.buyExchContractTx + simConfig.sellExchContractTx);
                }
                if (targetBidPNL[i] > 0.0) {
                    retValBid = true;
                }
            }

            if (retValAsk) {
                targetAskPNL[i] = 1.0;
            } else {
                if (instru.perYield) {
                    targetAskPNL[i] = (Instrument.bondPrice(this.targetPrice, instru.cDays)
                            - Instrument.bondPrice(askPxAtLevel, instru.cDays))
                            - (simConfig.buyExchTx + simConfig.sellExchTx);
                } else {
                    // C++: targetAskPNL[i] = (AskPx - targetPrice - buyTx*targetPrice - sellTx*AskPx) * multiplier - contractCosts
                    // Ref: TradeBotUtils.cpp:4013
                    targetAskPNL[i] = (askPxAtLevel - this.targetPrice
                            - simConfig.buyExchTx * this.targetPrice
                            - simConfig.sellExchTx * askPxAtLevel)
                            * instru.priceMultiplier
                            - (simConfig.buyExchContractTx + simConfig.sellExchContractTx);
                }
                if (targetAskPNL[i] > 0.0) {
                    retValAsk = true;
                }
            }
        }

        // C++: return (retValBid || retValAsk);
        return retValBid || retValAsk;
    }
}
