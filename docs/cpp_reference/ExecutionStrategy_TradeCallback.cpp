/**
 * C++ 原代码: ExecutionStrategy::TradeCallBack()
 * 来源: tbsrc/strategy/ExecutionStrategy.cpp
 *
 * Go 对应实现: golang/pkg/strategy/pairwise_arb_strategy.go
 *   - 函数: updateLeg1Position()
 *   - 函数: updateLeg2Position()
 *
 * 功能: 处理成交回报，更新持仓统计
 *   - 中国期货净持仓模型
 *   - 买入先平空再开多
 *   - 卖出先平多再开空
 */

void ExecutionStrategy::TradeCallBack(TradeInfo* trade) {
    int qty = trade->qty;
    double price = trade->price;

    if (trade->side == BUY) {
        // 买入逻辑
        m_buyTotalQty += qty;
        m_buyTotalValue += qty * price;

        // 检查是否有空头需要平仓
        if (m_netpos < 0) {
            // 平空
            int closedQty = std::min(qty, m_sellQty);
            m_sellQty -= closedQty;
            m_netpos += closedQty;
            qty -= closedQty;

            if (m_sellQty == 0) {
                m_sellAvgPrice = 0;
            }
        }

        // 开多
        if (qty > 0) {
            double totalCost = m_buyAvgPrice * m_buyQty;
            totalCost += price * qty;
            m_buyQty += qty;
            m_netpos += qty;
            if (m_buyQty > 0) {
                m_buyAvgPrice = totalCost / m_buyQty;
            }
        }
    } else {
        // 卖出逻辑
        m_sellTotalQty += qty;
        m_sellTotalValue += qty * price;

        // 检查是否有多头需要平仓
        if (m_netpos > 0) {
            // 平多
            int closedQty = std::min(qty, m_buyQty);
            m_buyQty -= closedQty;
            m_netpos -= closedQty;
            qty -= closedQty;

            if (m_buyQty == 0) {
                m_buyAvgPrice = 0;
            }
        }

        // 开空
        if (qty > 0) {
            double totalCost = m_sellAvgPrice * m_sellQty;
            totalCost += price * qty;
            m_sellQty += qty;
            m_netpos -= qty;
            if (m_sellQty > 0) {
                m_sellAvgPrice = totalCost / m_sellQty;
            }
        }
    }
}

/**
 * 变量说明:
 *
 * - m_netpos: 净持仓 (正=多头, 负=空头)
 * - m_buyQty: 多头持仓数量
 * - m_sellQty: 空头持仓数量
 * - m_buyAvgPrice: 多头平均成本
 * - m_sellAvgPrice: 空头平均成本
 * - m_buyTotalQty: 累计买入数量
 * - m_sellTotalQty: 累计卖出数量
 * - m_buyTotalValue: 累计买入金额
 * - m_sellTotalValue: 累计卖出金额
 *
 * 净持仓模型:
 * - 买入时: 先检查是否有空头，有则平仓；剩余部分开多
 * - 卖出时: 先检查是否有多头，有则平仓；剩余部分开空
 * - m_netpos = m_buyQty - m_sellQty
 */
