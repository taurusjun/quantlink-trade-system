package com.quantlink.trader.strategy;

import com.quantlink.trader.core.CommonClient;
import com.quantlink.trader.core.ConfigParams;

import java.lang.foreign.MemorySegment;

/**
 * 测试用 CommonClient — 不需要真实 Connector/SHM。
 * 重写发单方法，记录调用并返回模拟 OrderID。
 */
class MockCommonClient extends CommonClient {

    int nextOrderID = 1001;
    int newOrderCount = 0;
    int modifyOrderCount = 0;
    int cancelOrderCount = 0;

    @Override
    public int sendNewOrder(int strategyID, String symbol, int side, double price,
                            int qty, int posDirection, Object strategy) {
        newOrderCount++;
        int orderId = nextOrderID++;
        ConfigParams.getInstance().orderIDStrategyMap.put(orderId, strategy);
        return orderId;
    }

    @Override
    public void sendModifyOrder(int strategyID, String symbol, int side, double price,
                                int qty, int orderID, int posDirection, Object strategy) {
        modifyOrderCount++;
    }

    @Override
    public void sendCancelOrder(int strategyID, String symbol, int side,
                                int orderID, Object strategy) {
        cancelOrderCount++;
    }
}
