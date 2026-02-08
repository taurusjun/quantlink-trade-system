/**
 * C++ 原代码: PairwiseArbStrategy::SetThresholds()
 * 来源: tbsrc/strategy/PairwiseArbStrategy.cpp
 *
 * Go 对应实现: golang/pkg/strategy/pairwise_arb_strategy.go
 *   - 函数: setDynamicThresholds()
 *
 * 功能: 根据当前持仓动态调整入场阈值
 *   - 持仓越多，同方向开仓阈值越严格
 *   - 反方向开仓阈值放宽
 */

void PairwiseArbStrategy::SetThresholds() {
    // 计算阈值差值
    auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
    auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;

    if (m_firstStrat->m_netpos_pass == 0) {
        // 无持仓：使用初始阈值
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE;
    } else if (m_firstStrat->m_netpos_pass > 0) {
        // 多头持仓：
        //   - 买入阈值变严（做多更难）
        //   - 卖出阈值放宽（做空更易）
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE
            + long_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE
            - short_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
    } else {
        // 空头持仓 (m_netpos_pass < 0)：
        //   - 卖出阈值变严（做空更难）
        //   - 买入阈值放宽（做多更易）
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE
            + short_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE
            - long_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
    }
}

/**
 * 计算示例:
 *
 * 配置: BEGIN_PLACE=2.0, LONG_PLACE=3.5, SHORT_PLACE=0.5, maxPos=100
 *
 * long_place_diff = 3.5 - 2.0 = 1.5
 * short_place_diff = 2.0 - 0.5 = 1.5
 *
 * Case 1: netpos = 0 (空仓)
 *   tholdBid = 2.0
 *   tholdAsk = 2.0
 *
 * Case 2: netpos = 100 (满仓多头, posRatio = 1.0)
 *   tholdBid = 2.0 + 1.5 * 1.0 = 3.5
 *   tholdAsk = 2.0 - 1.5 * 1.0 = 0.5
 *
 * Case 3: netpos = -100 (满仓空头, posRatio = -1.0)
 *   tholdBid = 2.0 + 1.5 * (-1.0) = 0.5
 *   tholdAsk = 2.0 - 1.5 * (-1.0) = 3.5
 *
 * Case 4: netpos = 50 (半仓多头, posRatio = 0.5)
 *   tholdBid = 2.0 + 1.5 * 0.5 = 2.75
 *   tholdAsk = 2.0 - 1.5 * 0.5 = 1.25
 */
