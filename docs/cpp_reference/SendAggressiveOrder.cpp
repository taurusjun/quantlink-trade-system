/**
 * C++ 原代码: PairwiseArbStrategy::SendAggressiveOrder()
 * 来源: tbsrc/strategy/PairwiseArbStrategy.cpp
 *
 * Go 对应实现: golang/pkg/strategy/pairwise_arb_strategy.go
 *   - 函数: sendAggressiveOrder()
 *   - 函数: calculateExposure()
 *
 * 功能: 检测敞口并主动发送追单
 *   - 敞口计算: leg1Position + leg2Position (应为0)
 *   - 流控: 同方向追单间隔 > 500ms
 *   - 价格递进: 前3次每次 1 tick，第4次跳跃 SLOP ticks
 *   - 失败保护: 超过阈值触发策略停止
 */

void PairwiseArbStrategy::SendAggressiveOrder() {
    auto pending_netpos_agg2 = CalcPendingNetposAgg();

    // 计算总敞口
    auto exposure = m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg + pending_netpos_agg2;

    // 多头敞口 -> 主动卖对冲
    if (exposure > 0) {
        if (m_secondStrat->last_agg_side != SELL || now_ts - m_secondStrat->last_agg_time > 500) {
            // 首次追单或间隔超过 500ms: 重置计数，使用市价
            m_agg_repeat = 1;
            m_secondStrat->SendAskOrder2(m_secondinstru, NEWORDER, 0,
                m_secondinstru->bidPx[0], CROSS, qty);
            m_secondStrat->last_agg_side = SELL;
            m_secondStrat->last_agg_time = now_ts;
        } else {
            // 追单逻辑
            if (m_agg_repeat > 3) {
                // 超过 3 次追单失败，报警并停止策略
                HandleSquareoff();
            } else {
                // 价格递进: 前3次每次降价1个tick，第4次跳跃SLOP个tick
                double agg_price;
                if (m_agg_repeat < 3) {
                    agg_price = m_secondinstru->bidPx[0] - m_secondStrat->m_instru->m_tickSize * m_agg_repeat;
                } else {
                    agg_price = m_secondinstru->bidPx[0] - m_secondStrat->m_instru->m_tickSize * m_secondStrat->m_thold->SLOP;
                }

                m_secondStrat->SendAskOrder2(m_secondinstru, NEWORDER, 0, agg_price, CROSS, qty);
                m_agg_repeat++;
            }
        }
    }
    // 空头敞口 -> 主动买对冲
    else if (exposure < 0) {
        if (m_secondStrat->last_agg_side != BUY || now_ts - m_secondStrat->last_agg_time > 500) {
            // 首次追单或间隔超过 500ms
            m_agg_repeat = 1;
            m_secondStrat->SendBidOrder2(m_secondinstru, NEWORDER, 0,
                m_secondinstru->askPx[0], CROSS, qty);
            m_secondStrat->last_agg_side = BUY;
            m_secondStrat->last_agg_time = now_ts;
        } else {
            if (m_agg_repeat > 3) {
                HandleSquareoff();
            } else {
                double agg_price;
                if (m_agg_repeat < 3) {
                    agg_price = m_secondinstru->askPx[0] + m_secondStrat->m_instru->m_tickSize * m_agg_repeat;
                } else {
                    agg_price = m_secondinstru->askPx[0] + m_secondStrat->m_instru->m_tickSize * m_secondStrat->m_thold->SLOP;
                }

                m_secondStrat->SendBidOrder2(m_secondinstru, NEWORDER, 0, agg_price, CROSS, qty);
                m_agg_repeat++;
            }
        }
    }
}

/**
 * 参数说明:
 *
 * - m_netpos_pass: 被动成交持仓 (leg1)
 * - m_netpos_agg: 主动成交持仓 (leg2)
 * - last_agg_side: 上次追单方向 (BUY/SELL)
 * - last_agg_time: 上次追单时间戳 (ms)
 * - m_agg_repeat: 追单次数计数
 * - SLOP: 跳跃 tick 数 (配置参数)
 * - m_tickSize: 最小价格变动单位
 *
 * 流程:
 * 1. 计算敞口 = leg1持仓 + leg2持仓
 * 2. 如果敞口为0，不需要追单
 * 3. 如果方向变化或间隔 > 500ms，重置计数
 * 4. 追单价格递进:
 *    - 第1次: bid/ask
 *    - 第2次: bid/ask ± 1 tick
 *    - 第3次: bid/ask ± 2 ticks
 *    - 第4次: bid/ask ± SLOP ticks
 * 5. 超过3次追单失败，触发 HandleSquareoff()
 */
