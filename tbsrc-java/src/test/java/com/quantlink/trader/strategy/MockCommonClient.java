package com.quantlink.trader.strategy;

import com.quantlink.trader.core.CommonClient;
import com.quantlink.trader.core.ConfigParams;

import java.lang.foreign.MemorySegment;
import java.util.ArrayList;
import java.util.List;

/**
 * 测试用 CommonClient — 不需要真实 Connector/SHM。
 * 重写发单方法，记录调用并返回模拟 OrderID。
 */
class MockCommonClient extends CommonClient {

    int nextOrderID = 1001;
    int newOrderCount = 0;
    int modifyOrderCount = 0;
    int cancelOrderCount = 0;

    /** 记录每笔 sendNewOrder 调用的详情 */
    static class OrderRecord {
        int strategyID;
        String symbol;
        int side;
        double price;
        int qty;
        int posDirection;
        int orderID;

        OrderRecord(int strategyID, String symbol, int side, double price,
                     int qty, int posDirection, int orderID) {
            this.strategyID = strategyID;
            this.symbol = symbol;
            this.side = side;
            this.price = price;
            this.qty = qty;
            this.posDirection = posDirection;
            this.orderID = orderID;
        }
    }

    List<OrderRecord> orderRecords = new ArrayList<>();

    @Override
    public int sendNewOrder(int strategyID, String symbol, int side, double price,
                            int qty, int posDirection, Object strategy) {
        newOrderCount++;
        int orderId = nextOrderID++;
        orderRecords.add(new OrderRecord(strategyID, symbol, side, price, qty, posDirection, orderId));
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
