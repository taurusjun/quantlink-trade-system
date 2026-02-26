package com.quantlink.trader.api;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.ThreadLocalRandom;

/**
 * Demo 模式数据生成器。
 * 生成逼真的 AG 配对套利快照数据，用于在无实盘/模拟网关时验证 Dashboard 页面显示。
 *
 * 典型数据:
 * - AG 合约价差交易，行情价格 5800-6000 范围
 * - 持仓/PNL/订单统计有合理值
 * - 每次调用 generate() 微调价格模拟行情变动
 */
public class DemoDataGenerator {

    private static final String LEG1_SYMBOL = "ag2603";
    private static final String LEG2_SYMBOL = "ag2605";
    private static final String EXCHANGE = "SFE";
    private static final int STRATEGY_ID = 92201;

    // 行情状态（每次 generate 微调）
    private double leg1Mid = 5920.0;
    private double leg2Mid = 5960.0;
    private double avgSpread = 40.0;
    private double avgSpreadOri = 39.5;

    // 累计统计
    private int leg1TradeCount = 12;
    private int leg2TradeCount = 8;
    private int leg1OrderCount = 28;
    private int leg2OrderCount = 18;
    private double leg1RealisedPNL = 1250.0;
    private double leg2RealisedPNL = -380.0;
    private int tick = 0;

    /**
     * 生成一个完整的 DashboardSnapshot。
     * 每次调用会微调价格和统计数据，模拟实时行情变动。
     */
    public DashboardSnapshot generate() {
        tick++;
        ThreadLocalRandom rng = ThreadLocalRandom.current();

        // 微调行情
        leg1Mid += rng.nextDouble(-2.0, 2.0);
        leg2Mid += rng.nextDouble(-2.0, 2.0);
        leg1Mid = Math.max(5800, Math.min(6000, leg1Mid));
        leg2Mid = Math.max(5840, Math.min(6040, leg2Mid));

        double spread = leg2Mid - leg1Mid;
        avgSpread = avgSpread * 0.99 + spread * 0.01;

        // 偶尔增加成交
        if (tick % 5 == 0) {
            leg1TradeCount++;
            leg1OrderCount += 2;
            leg1RealisedPNL += rng.nextDouble(-200, 300);
        }
        if (tick % 7 == 0) {
            leg2TradeCount++;
            leg2OrderCount += 2;
            leg2RealisedPNL += rng.nextDouble(-200, 200);
        }

        DashboardSnapshot snap = new DashboardSnapshot();
        snap.timestamp = Instant.now().toString();
        snap.strategyID = STRATEGY_ID;
        snap.active = true;
        snap.exposure = 2;
        snap.modelFile = "./models/model.ag2603.ag2605.par.txt.92201";
        snap.strategyType = "TB_PAIR_STRAT";
        snap.controlFile = "./controls/ag_92201.ctrl";

        // 价差
        snap.spread.current = spread;
        snap.spread.avgSpread = avgSpread;
        snap.spread.avgOri = avgSpreadOri;
        snap.spread.tValue = 0;
        snap.spread.deviation = spread - avgSpread;
        snap.spread.isValid = true;
        snap.spread.alpha = 0.0050;

        // Leg1
        snap.leg1 = buildLeg(LEG1_SYMBOL, leg1Mid, 2, 1, rng,
                leg1TradeCount, leg1OrderCount, leg1RealisedPNL,
                0.45, -0.30, 0.55, -0.20, 10, 1);
        // Leg2
        snap.leg2 = buildLeg(LEG2_SYMBOL, leg2Mid, -2, -1, rng,
                leg2TradeCount, leg2OrderCount, leg2RealisedPNL,
                0.50, -0.25, 0.50, -0.25, 10, 1);

        // 模拟订单列表
        snap.leg1.orders = buildDemoOrders(LEG1_SYMBOL, leg1Mid, rng);
        snap.leg2.orders = buildDemoOrders(LEG2_SYMBOL, leg2Mid, rng);

        return snap;
    }

    private DashboardSnapshot.LegSnapshot buildLeg(
            String symbol, double mid, int netpos, int netposPass,
            ThreadLocalRandom rng,
            int tradeCount, int orderCount, double realisedPNL,
            double bidPlace, double bidRemove, double askPlace, double askRemove,
            int maxPos, int size) {

        DashboardSnapshot.LegSnapshot ls = new DashboardSnapshot.LegSnapshot();
        ls.symbol = symbol;
        ls.exchange = EXCHANGE;

        // 行情
        double tickSize = 1.0;
        ls.bidPx = mid - tickSize;
        ls.askPx = mid + tickSize;
        ls.midPx = mid;
        ls.bidQty = rng.nextInt(5, 50);
        ls.askQty = rng.nextInt(5, 50);
        ls.lastTradePx = mid + rng.nextDouble(-1, 1);

        // 持仓
        ls.netpos = netpos;
        ls.netposPass = netposPass;
        ls.netposAgg = netpos - netposPass;

        // PNL
        ls.realisedPNL = realisedPNL;
        ls.unrealisedPNL = netpos * rng.nextDouble(-100, 100);
        ls.netPNL = ls.realisedPNL + ls.unrealisedPNL;
        ls.grossPNL = ls.realisedPNL + Math.abs(ls.unrealisedPNL);
        ls.maxPNL = Math.max(ls.netPNL, realisedPNL + 500);
        ls.drawdown = Math.min(0, ls.netPNL - ls.maxPNL);

        // 统计
        ls.tradeCount = tradeCount;
        ls.orderCount = orderCount;
        ls.rejectCount = 1;
        ls.cancelCount = orderCount - tradeCount - 1;
        ls.buyTotalQty = tradeCount / 2 + 1;
        ls.sellTotalQty = tradeCount / 2;

        // 阈值
        ls.tholdBidPlace = bidPlace;
        ls.tholdBidRemove = bidRemove;
        ls.tholdAskPlace = askPlace;
        ls.tholdAskRemove = askRemove;
        ls.tholdMaxPos = maxPos;
        ls.tholdSize = size;

        // 挂单
        ls.buyOpenOrders = rng.nextInt(0, 3);
        ls.sellOpenOrders = rng.nextInt(0, 3);
        ls.buyOpenQty = ls.buyOpenOrders;
        ls.sellOpenQty = ls.sellOpenOrders;

        // 状态标志
        ls.onExit = false;
        ls.onFlat = false;
        ls.onStopLoss = false;

        return ls;
    }

    private List<DashboardSnapshot.OrderSnapshot> buildDemoOrders(
            String symbol, double mid, ThreadLocalRandom rng) {
        List<DashboardSnapshot.OrderSnapshot> orders = new ArrayList<>();
        String[] sides = {"BUY", "SELL"};
        String[] statuses = {"NEW_CONFIRM", "TRADED", "CANCEL_CONFIRM", "NEW_CONFIRM"};
        String[] types = {"PASSIVE", "AGGRESSIVE", "PASSIVE", "PASSIVE"};

        int count = rng.nextInt(2, 5);
        for (int i = 0; i < count; i++) {
            DashboardSnapshot.OrderSnapshot os = new DashboardSnapshot.OrderSnapshot();
            os.orderID = 92201_000 + tick * 10 + i;
            os.side = sides[i % 2];
            os.price = mid + (i % 2 == 0 ? -1 : 1) * rng.nextDouble(0, 3);
            int totalQty = rng.nextInt(1, 4);
            String status = statuses[i % statuses.length];
            if ("TRADED".equals(status)) {
                os.doneQty = totalQty;
                os.openQty = 0;
            } else if ("CANCEL_CONFIRM".equals(status)) {
                os.doneQty = 0;
                os.openQty = 0;
            } else {
                os.doneQty = 0;
                os.openQty = totalQty;
            }
            os.status = status;
            os.ordType = types[i % types.length];
            os.time = String.format("%02d:%02d:%02d",
                    9 + rng.nextInt(0, 6), rng.nextInt(0, 60), rng.nextInt(0, 60));
            orders.add(os);
        }
        return orders;
    }
}
