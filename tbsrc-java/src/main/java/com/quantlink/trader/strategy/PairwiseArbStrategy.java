package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.io.*;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import java.util.logging.Logger;

/**
 * 双腿配对套利策略。
 * 迁移自: tbsrc/Strategies/include/PairwiseArbStrategy.h (line 14-76)
 *         tbsrc/Strategies/PairwiseArbStrategy.cpp (947 lines)
 *
 * C++ class PairwiseArbStrategy : public ExecutionStrategy
 * 第一腿(firstStrat)被动挂单，第二腿(secondStrat)对冲。
 */
public class PairwiseArbStrategy extends ExecutionStrategy {

    private static final Logger log = Logger.getLogger(PairwiseArbStrategy.class.getName());

    // ---- 双腿策略 ----
    // 迁移自: PairwiseArbStrategy.h:63-64
    public ExtraStrategy firstStrat;
    public ExtraStrategy secondStrat;

    // ---- 合约引用 ----
    // 迁移自: PairwiseArbStrategy.h:39-40
    public Instrument firstinstru;
    public Instrument secondinstru;

    // ---- 阈值引用（每次 sendOrder 刷新） ----
    // 迁移自: PairwiseArbStrategy.h:65-66
    public ThresholdSet thold_first;
    public ThresholdSet thold_second;

    // ---- 价格 Map 快照（每次 sendOrder 刷新） ----
    // 迁移自: PairwiseArbStrategy.h:67-70
    public Map<Double, OrderStats> bidMap1;
    public Map<Double, OrderStats> bidMap2;
    public Map<Double, OrderStats> askMap1;
    public Map<Double, OrderStats> askMap2;

    // ---- OrderMap 引用 ----
    // 迁移自: PairwiseArbStrategy.h:71-72 — OrderMap *m_ordMap1, *m_ordMap2
    public Map<Integer, OrderStats> ordMap1;
    public Map<Integer, OrderStats> ordMap2;

    // ---- 价差字段 ----
    // 迁移自: PairwiseArbStrategy.h:49-53
    public double avgSpreadRatio_ori;
    public double avgSpreadRatio;
    public double currSpreadRatio;
    public double currSpreadRatio_prev;
    public double expectedRatio;

    // ---- 最佳行情 ----
    // 迁移自: PairwiseArbStrategy.h:44-47
    public double i1_bestBid;
    public double i1_bestAsk;
    public double i2_bestBid;
    public double i2_bestAsk;

    // ---- 其他字段 ----
    // 迁移自: PairwiseArbStrategy.h:41-43, 54-62
    public double maxloss_limit;
    public double count;
    public double curr_time_val;
    public double last_time_val;
    public double tValue;
    public boolean is_valid_mkdata = true;
    public int netpos_agg1;
    public int netpos_agg2;
    public int agg_repeat = 1;  // C++: m_agg_repeat{1}
    public double second_ordIDstart;

    /**
     * 构造函数。
     * 迁移自: PairwiseArbStrategy::PairwiseArbStrategy(CommonClient*, SimConfig*)
     * Ref: PairwiseArbStrategy.cpp:7-84
     *
     * @param client    CommonClient
     * @param simConfig SimConfig
     * @param dailyInitPath daily_init 文件路径（C++ 在构造函数中调用 LoadMatrix2）
     */
    public PairwiseArbStrategy(CommonClient client, SimConfig simConfig, String dailyInitPath) {
        super(client, simConfig);

        // C++: m_firstStrat = new ExtraStrategy(client, simConfig)
        // C++: m_secondStrat = new ExtraStrategy(client, simConfig)
        firstStrat = new ExtraStrategy(client, simConfig);
        secondStrat = new ExtraStrategy(client, simConfig);

        // C++: m_secondStrat->m_instru = m_secondStrat->m_instru_sec
        secondStrat.instru = secondStrat.instruSec;

        // C++: m_ordMap1 = &m_firstStrat->m_ordMap
        // C++: m_ordMap2 = &m_secondStrat->m_ordMap
        ordMap1 = firstStrat.ordMap;
        ordMap2 = secondStrat.ordMap;

        log.info("PairwiseArbStrategy strategyID:" + strategyID
                + ",firstStrat:" + firstStrat.strategyID
                + ",secondStrat:" + secondStrat.strategyID);

        // C++: m_firstStrat->callSquareOff = false; m_secondStrat->callSquareOff = false
        firstStrat.callSquareOff = false;
        secondStrat.callSquareOff = false;

        // C++: m_firstStrat->m_targetBidPNL = new double[5]{1,1,1,1,1}
        firstStrat.targetBidPNL = new double[]{1, 1, 1, 1, 1};
        firstStrat.targetAskPNL = new double[]{1, 1, 1, 1, 1};
        secondStrat.targetBidPNL = new double[]{1, 1, 1, 1, 1};
        secondStrat.targetAskPNL = new double[]{1, 1, 1, 1, 1};

        // C++: m_firstinstru = m_firstStrat->m_instru; m_secondinstru = m_secondStrat->m_instru
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        expectedRatio = 0;
        currSpreadRatio = 0;
        currSpreadRatio_prev = 0;
        count = 100000;
        curr_time_val = 0;
        last_time_val = 0;
        second_ordIDstart = 10;

        // ---- LoadMatrix2 + 昨仓初始化 ----
        // C++: PairwiseArbStrategy.cpp:18-62 — 构造函数中直接调用 LoadMatrix2
        if (dailyInitPath != null && !dailyInitPath.isEmpty()) {
            loadDailyInitData(dailyInitPath);
        }
    }

    /**
     * 加载 daily_init 数据。
     * 从构造函数调用，匹配 C++ PairwiseArbStrategy 构造函数中的 LoadMatrix2 逻辑。
     */
    private void loadDailyInitData(String dailyInitPath) {
        Map<Integer, Map<String, String>> mx = loadMatrix2(dailyInitPath);

        if (!mx.containsKey(strategyID)) {
            throw new RuntimeException("daily_init ERROR! Missing strategyID " + strategyID);
        }

        String name1 = firstStrat.instru.origBaseName;
        String name2 = secondStrat.instru.origBaseName;
        if (name1.equals(name2)) {
            throw new RuntimeException("daily_init ERROR! origBaseName1:" + name1
                    + " origBaseName2:" + name2);
        }

        Map<String, String> row = mx.get(strategyID);
        avgSpreadRatio_ori = Double.parseDouble(row.getOrDefault("avgPx", "0"));
        avgSpreadRatio = avgSpreadRatio_ori;

        int netpos_ytd1 = Integer.parseInt(row.getOrDefault("ytd1", "0"));
        int netpos_2day1 = Integer.parseInt(row.getOrDefault("2day", "0"));
        int netpos_agg2_val = Integer.parseInt(row.getOrDefault("ytd2", "0"));

        // C++: m_firstStrat->m_netpos_pass_ytd = netpos_ytd1
        firstStrat.netpos_pass_ytd = netpos_ytd1;
        firstStrat.netpos = netpos_ytd1 + netpos_2day1;
        firstStrat.netpos_pass = netpos_ytd1 + netpos_2day1;
        secondStrat.netpos = netpos_agg2_val;
        secondStrat.netpos_agg = netpos_agg2_val;

        log.info("avgSpreadRatio_ori:" + avgSpreadRatio_ori
                + " origBaseName1:" + name1 + " netpos_ytd1:" + netpos_ytd1
                + " netpos_2day1:" + netpos_2day1 + " netpos1:" + firstStrat.netpos
                + " origBaseName2:" + name2 + " netpos_agg2:" + netpos_agg2_val);
    }

    // =======================================================================
    //  LoadMatrix2 / SaveMatrix2
    // =======================================================================

    /**
     * 加载 daily_init 格式文件。
     * 迁移自: PairwiseArbStrategy::LoadMatrix2(string filepath)
     * Ref: PairwiseArbStrategy.cpp:112-144
     *
     * 文件格式: 首行为 header，后续行以空格分隔。
     * 第一列为 strategyID，后续列按 header 名映射为 string value。
     */
    public Map<Integer, Map<String, String>> loadMatrix2(String filepath) {
        Map<Integer, Map<String, String>> mx = new LinkedHashMap<>();
        try (BufferedReader reader = new BufferedReader(new FileReader(filepath))) {
            String headerLine = reader.readLine();
            if (headerLine == null) return mx;
            String[] headers = headerLine.trim().split("\\s+");

            String line;
            while ((line = reader.readLine()) != null) {
                line = line.trim();
                if (line.isEmpty()) continue;
                String[] tokens = line.split("\\s+");
                int sid = Integer.parseInt(tokens[0]);
                Map<String, String> rowMap = new LinkedHashMap<>();
                for (int i = 1; i < Math.min(tokens.length, headers.length); i++) {
                    rowMap.put(headers[i], tokens[i]);
                }
                mx.put(sid, rowMap);
            }
        } catch (IOException e) {
            log.severe("Failed to load daily_init from " + filepath + ": " + e.getMessage());
        }
        return mx;
    }

    /**
     * 保存 daily_init 文件。
     * 迁移自: PairwiseArbStrategy::SaveMatrix2(string filepath)
     * Ref: PairwiseArbStrategy.cpp:653-686
     */
    public void saveMatrix2(String filepath) {
        try (PrintWriter out = new PrintWriter(new FileWriter(filepath))) {
            // C++: const string Head = "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 "
            out.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            out.println(strategyID + " 0 " + avgSpreadRatio_ori
                    + " " + firstStrat.instru.origBaseName
                    + " " + secondStrat.instru.origBaseName
                    + " " + firstStrat.netpos_pass
                    + " " + secondStrat.netpos_agg);
        } catch (IOException e) {
            log.severe("Failed to save daily_init to " + filepath + ": " + e.getMessage());
        }
    }

    // =======================================================================
    //  SendOrder — 被动挂单 + 对冲逻辑
    // =======================================================================

    /**
     * 发单逻辑 — 第一腿被动挂单 + 第二腿对冲。
     * 迁移自: PairwiseArbStrategy::SendOrder()
     * Ref: PairwiseArbStrategy.cpp:146-385
     */
    @Override
    public void sendOrder() {
        OrderStats.HitType ordType = OrderStats.HitType.STANDARD;

        // C++: 刷新合约/价格/阈值引用
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        bidMap1 = firstStrat.bidMap;
        bidMap2 = secondStrat.bidMap;
        askMap1 = firstStrat.askMap;
        askMap2 = secondStrat.askMap;

        thold_first = firstStrat.thold;
        thold_second = secondStrat.thold;

        i1_bestBid = firstinstru.bidPx[0];
        i1_bestAsk = firstinstru.askPx[0];
        i2_bestBid = secondinstru.bidPx[0];
        i2_bestAsk = secondinstru.askPx[0];
        maxloss_limit = thold_first.MAX_LOSS;

        setThresholds();

        // C++: 撤销 firstStrat 中 CROSS/MATCH 类型的订单
        for (OrderStats order : new ArrayList<>(ordMap1.values())) {
            if (order.hitType == OrderStats.HitType.CROSS || order.hitType == OrderStats.HitType.MATCH) {
                firstStrat.sendCancelOrder(firstinstru, order.orderID);
            }
        }
        // C++: 撤销 secondStrat 中 CROSS/MATCH 类型的订单
        for (OrderStats order : new ArrayList<>(ordMap2.values())) {
            if (order.hitType == OrderStats.HitType.CROSS || order.hitType == OrderStats.HitType.MATCH) {
                secondStrat.sendCancelOrder(secondinstru, order.orderID);
            }
        }

        // C++: 检查阈值有效
        if (firstStrat.tholdBidPlace != -1 && firstStrat.tholdBidRemove != -1
                && firstStrat.tholdAskPlace != -1 && firstStrat.tholdAskRemove != -1) {

            // C++: 撤销价差超出范围的买单
            for (OrderStats order : new ArrayList<>(bidMap1.values())) {
                double longSpread = order.price - secondinstru.bidPx[0];
                if ((longSpread > avgSpreadRatio - firstStrat.tholdBidRemove)
                        && (order.status == OrderStats.Status.NEW_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_REJECT)) {
                    firstStrat.sendCancelOrder(firstinstru, order.orderID);
                }
            }
            // C++: 撤销价差超出范围的卖单
            for (OrderStats order : new ArrayList<>(askMap1.values())) {
                double shortSpread = order.price - secondinstru.askPx[0];
                if ((shortSpread < avgSpreadRatio + firstStrat.tholdAskRemove)
                        && (order.status == OrderStats.Status.NEW_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_REJECT)) {
                    firstStrat.sendCancelOrder(firstinstru, order.orderID);
                }
            }

            // C++: 行情无效检查
            if (firstinstru.bidPx[0] == 0 || firstinstru.askPx[0] == 0
                    || secondinstru.bidPx[0] == 0 || secondinstru.askPx[0] == 0) {
                return;
            }

            // C++: 第一腿多层被动挂单
            // Ref: PairwiseArbStrategy.cpp:235-346
            for (int level = 0; level < thold_first.MAX_QUOTE_LEVEL; level++) {
                double longSpreadRatio1 = firstinstru.bidPx[level] - secondinstru.bidPx[0];
                double shortSpreadRatio1 = firstinstru.askPx[level] - secondinstru.askPx[0];

                // 卖单逻辑
                if (shortSpreadRatio1 > avgSpreadRatio + firstStrat.tholdAskPlace) {
                    double passive_sellprice1 = firstinstru.askPx[level];
                    passive_sellprice1 = getAskPriceFirst(passive_sellprice1, level);

                    if (firstStrat.netpos_pass * -1 < firstStrat.tholdAskMaxPos) {
                        if (firstStrat.sellOpenOrders > firstStrat.thold.SUPPORTING_ORDERS
                                || firstStrat.sellOpenQty + -1 * firstStrat.netpos_pass >= firstStrat.tholdAskMaxPos) {
                            // 找最远卖单，如果新价格更优则撤销最远单
                            double askHigh1 = findHighestConfirmedAskPrice(askMap1);
                            if (!askMap1.containsKey(passive_sellprice1) && passive_sellprice1 < askHigh1) {
                                if (askHigh1 != 0) {
                                    OrderStats farOrder = askMap1.get(askHigh1);
                                    if (farOrder != null) {
                                        firstStrat.sendCancelOrder(firstinstru, farOrder.orderID);
                                    }
                                }
                            }
                        } else {
                            firstStrat.sendAskOrder2(firstinstru, level, passive_sellprice1, ordType, 0);
                        }
                    } else {
                        // 超过最大卖仓，撤销所有卖单
                        for (OrderStats order : new ArrayList<>(askMap1.values())) {
                            firstStrat.sendCancelOrder(firstinstru, order.orderID);
                        }
                    }
                }

                // 买单逻辑
                if (longSpreadRatio1 < avgSpreadRatio - firstStrat.tholdBidPlace) {
                    double passive_buyprice1 = firstinstru.bidPx[level];
                    passive_buyprice1 = getBidPriceFirst(passive_buyprice1, level);

                    if (firstStrat.netpos_pass < firstStrat.tholdBidMaxPos) {
                        if (firstStrat.buyOpenOrders > firstStrat.thold.SUPPORTING_ORDERS
                                || firstStrat.buyOpenQty + firstStrat.netpos_pass >= firstStrat.tholdBidMaxPos) {
                            double bidLow1 = findLowestConfirmedBidPrice(bidMap1);
                            if (!bidMap1.containsKey(passive_buyprice1) && passive_buyprice1 > bidLow1) {
                                if (bidLow1 != 0) {
                                    OrderStats farOrder = bidMap1.get(bidLow1);
                                    if (farOrder != null) {
                                        firstStrat.sendCancelOrder(firstinstru, farOrder.orderID);
                                    }
                                }
                            }
                        } else {
                            firstStrat.sendBidOrder2(firstinstru, level, passive_buyprice1, ordType, 0);
                        }
                    } else {
                        for (OrderStats order : new ArrayList<>(bidMap1.values())) {
                            firstStrat.sendCancelOrder(firstinstru, order.orderID);
                        }
                    }
                }
            }

            // C++: 对冲检查
            // Ref: PairwiseArbStrategy.cpp:348-375
            int pending_netpos_agg2 = calcPendingNetposAgg();
            long now_ts = System.nanoTime() / 1000; // microseconds

            int netExposure = firstStrat.netpos_pass + secondStrat.netpos_agg + pending_netpos_agg2;

            if (netExposure > 0
                    && secondStrat.sellAggOrder <= firstStrat.thold.SUPPORTING_ORDERS
                    && (secondStrat.last_agg_side != Constants.SIDE_SELL
                    || (secondStrat.last_agg_side == Constants.SIDE_SELL
                    && now_ts / 1000 - secondStrat.last_agg_time > 100))) {
                // 价差净多头，第二腿卖出对冲
                secondStrat.sendAskOrder2(secondinstru, 0,
                        secondinstru.bidPx[0] - secondStrat.instru.tickSize,
                        OrderStats.HitType.CROSS, netExposure);
                secondStrat.sellAggOrder++;
                secondStrat.last_agg_time = now_ts / 1000;
                secondStrat.last_agg_side = Constants.SIDE_SELL;
            } else if (netExposure < 0
                    && secondStrat.buyAggOrder <= firstStrat.thold.SUPPORTING_ORDERS
                    && (secondStrat.last_agg_side != Constants.SIDE_BUY
                    || (secondStrat.last_agg_side == Constants.SIDE_BUY
                    && now_ts / 1000 - secondStrat.last_agg_time > 100))) {
                // 价差净空头，第二腿买入对冲
                secondStrat.sendBidOrder2(secondinstru, 0,
                        secondinstru.askPx[0] + secondStrat.instru.tickSize,
                        OrderStats.HitType.CROSS, -netExposure);
                secondStrat.buyAggOrder++;
                secondStrat.last_agg_time = now_ts / 1000;
                secondStrat.last_agg_side = Constants.SIDE_BUY;
            }
        }
    }

    // =======================================================================
    //  SendAggressiveOrder — 主动追单
    // =======================================================================

    /**
     * 主动追单 — 在敞口持续时重复追单。
     * 迁移自: PairwiseArbStrategy::SendAggressiveOrder()
     * Ref: PairwiseArbStrategy.cpp:701-800
     */
    public void sendAggressiveOrder() {
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        int pending_netpos_agg2 = calcPendingNetposAgg();
        long now_ts = System.nanoTime() / 1000; // microseconds
        int netExposure = firstStrat.netpos_pass + secondStrat.netpos_agg + pending_netpos_agg2;

        if (netExposure > 0 && secondStrat.sellAggOrder <= secondStrat.thold.SUPPORTING_ORDERS) {
            if (secondStrat.last_agg_side != Constants.SIDE_SELL
                    || (secondStrat.last_agg_side == Constants.SIDE_SELL
                    && now_ts / 1000 - secondStrat.last_agg_time > 500)) {
                // 首次或超过500ms，按市场行情发单
                secondStrat.sendAskOrder2(secondinstru, 0,
                        secondinstru.bidPx[0], OrderStats.HitType.CROSS, netExposure);
                secondStrat.sellAggOrder++;
                secondStrat.last_agg_time = now_ts / 1000;
                secondStrat.last_agg_side = Constants.SIDE_SELL;
            } else {
                if (agg_repeat > 3) {
                    log.warning("Reach max agg_repeat, deactive Strategy");
                    handleSquareoff();
                } else {
                    // C++: agg_repeat < 3 ? bidPx[0] - tickSize * agg_repeat : bidPx[0] - tickSize * SLOP
                    double aggPrice = agg_repeat < 3
                            ? secondinstru.bidPx[0] - secondStrat.instru.tickSize * agg_repeat
                            : secondinstru.bidPx[0] - secondStrat.instru.tickSize * secondStrat.thold.SLOP;
                    boolean ret = secondStrat.sendAskOrder2(secondinstru, 0, aggPrice,
                            OrderStats.HitType.CROSS, netExposure);
                    if (ret) {
                        agg_repeat++;
                        secondStrat.sellAggOrder++;
                        secondStrat.last_agg_time = now_ts / 1000;
                        secondStrat.last_agg_side = Constants.SIDE_SELL;
                    }
                }
            }
        } else if (netExposure < 0 && secondStrat.buyAggOrder <= secondStrat.thold.SUPPORTING_ORDERS) {
            if (secondStrat.last_agg_side != Constants.SIDE_BUY
                    || (secondStrat.last_agg_side == Constants.SIDE_BUY
                    && now_ts / 1000 - secondStrat.last_agg_time > 500)) {
                secondStrat.sendBidOrder2(secondinstru, 0,
                        secondinstru.askPx[0], OrderStats.HitType.CROSS, -netExposure);
                secondStrat.buyAggOrder++;
                secondStrat.last_agg_time = now_ts / 1000;
                secondStrat.last_agg_side = Constants.SIDE_BUY;
            } else {
                if (agg_repeat > 3) {
                    log.warning("Reach max agg_repeat, deactive Strategy");
                    handleSquareoff();
                } else {
                    double aggPrice = agg_repeat < 3
                            ? secondinstru.askPx[0] + secondStrat.instru.tickSize * agg_repeat
                            : secondinstru.askPx[0] + secondStrat.instru.tickSize * secondStrat.thold.SLOP;
                    boolean ret = secondStrat.sendBidOrder2(secondinstru, 0, aggPrice,
                            OrderStats.HitType.CROSS, -netExposure);
                    if (ret) {
                        agg_repeat++;
                        secondStrat.buyAggOrder++;
                        secondStrat.last_agg_time = now_ts / 1000;
                        secondStrat.last_agg_side = Constants.SIDE_BUY;
                    }
                }
            }
        }
    }

    // =======================================================================
    //  SetThresholds — 覆盖基类
    // =======================================================================

    /**
     * 设置双腿阈值（线性插值）。
     * 迁移自: PairwiseArbStrategy::SetThresholds()
     * Ref: PairwiseArbStrategy.cpp:902-947
     */
    @Override
    public void setThresholds() {
        // C++: if (m_firstinstru->m_sendInLots && m_secondinstru->m_sendInLots)
        if (firstinstru.sendInLots && secondinstru.sendInLots) {
            firstStrat.tholdMaxPos = Math.max(thold_first.BID_MAX_SIZE, thold_first.ASK_MAX_SIZE);
            firstStrat.tholdBeginPos = thold_first.BEGIN_SIZE;
            firstStrat.tholdSize = thold_first.SIZE;

            firstStrat.tholdBidSize = thold.BID_SIZE;
            firstStrat.tholdBidMaxPos = thold.BID_MAX_SIZE;
            firstStrat.tholdAskSize = thold.ASK_SIZE;
            firstStrat.tholdAskMaxPos = thold.ASK_MAX_SIZE;
        } else {
            firstStrat.tholdMaxPos = (int) (thold_first.MAX_SIZE * firstinstru.lotSize);
            firstStrat.tholdBeginPos = (int) (thold_first.BEGIN_SIZE * firstinstru.lotSize);
            firstStrat.tholdSize = (int) (thold_first.SIZE * firstinstru.lotSize);
        }

        // C++: 线性插值阈值
        double longPlaceDiff = thold_first.LONG_PLACE - thold_first.BEGIN_PLACE;
        double shortPlaceDiff = thold_first.BEGIN_PLACE - thold_first.SHORT_PLACE;
        double longRemoveDiff = thold_first.LONG_REMOVE - thold_first.BEGIN_REMOVE;
        double shortRemoveDiff = thold_first.BEGIN_REMOVE - thold_first.SHORT_REMOVE;

        if (firstStrat.netpos_pass == 0) {
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE;
        } else if (firstStrat.netpos_pass > 0) {
            // C++: m_tholdBidPlace = BEGIN_PLACE + longPlaceDiff * netpos_pass / tholdMaxPos
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE + longPlaceDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE + longRemoveDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE - shortPlaceDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE - shortRemoveDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
        } else {
            // netpos_pass < 0
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE + shortPlaceDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE + shortRemoveDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE - longPlaceDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE - longRemoveDiff * firstStrat.netpos_pass / firstStrat.tholdMaxPos;
        }
    }

    // =======================================================================
    //  ORS 回调
    // =======================================================================

    /**
     * 路由回报到 firstStrat 或 secondStrat。
     * 迁移自: PairwiseArbStrategy::ORSCallBack(ResponseMsg*)
     * Ref: PairwiseArbStrategy.cpp:428-477
     */
    @Override
    public void orsCallBack(MemorySegment response) {
        int orderID = (int) Types.RESP_ORDER_ID_VH.get(response, 0L);
        int responseType = (int) Types.RESP_RESPONSE_TYPE_VH.get(response, 0L);

        if (ordMap1.containsKey(orderID)) {
            // 第一腿回报
            firstStrat.orsCallBack(response);
            if (responseType == Constants.RESP_TRADE_CONFIRM) {
                agg_repeat = 1;
            }
        } else if (ordMap2.containsKey(orderID)) {
            // 第二腿回报 — 先处理 aggOrder 计数再调用基类
            handleAggOrder(response, ordMap2.get(orderID), secondStrat);
            secondStrat.orsCallBack(response);
            if (responseType == Constants.RESP_TRADE_CONFIRM) {
                agg_repeat = 1;
            }
        }

        // C++: m_rejectCount = m_firstStrat->m_rejectCount + m_secondStrat->m_rejectCount
        rejectCount = firstStrat.rejectCount + secondStrat.rejectCount;

        if (active) {
            sendAggressiveOrder();
        }
    }

    // =======================================================================
    //  MD 回调
    // =======================================================================

    /**
     * 行情回调 — 转发到两腿 + 计算价差。
     * 迁移自: PairwiseArbStrategy::MDCallBack(MarketUpdateNew*)
     * Ref: PairwiseArbStrategy.cpp:479-569
     */
    @Override
    public void mdCallBack(MemorySegment update) {
        // C++: m_onFlat = m_firstStrat->m_onFlat && m_secondStrat->m_onFlat
        onFlat = firstStrat.onFlat && secondStrat.onFlat;

        // C++: 双腿合计PNL 检查 max_loss
        if (firstStrat.netPNL + secondStrat.netPNL < -1 * maxloss_limit) {
            firstStrat.callSquareOff = true;
            secondStrat.callSquareOff = true;
        }

        // C++: 转发到两腿
        firstStrat.mdCallBack(update);
        secondStrat.mdCallBack(update);

        // C++: 计算当前价差
        if (firstStrat.instru.bidPx[0] <= 0 || firstStrat.instru.askPx[0] <= 0
                || secondStrat.instru.bidPx[0] <= 0 && secondStrat.instru.askPx[0] <= 0) {
            // currSpreadRatio 不变
        } else {
            currSpreadRatio = ((firstStrat.instru.bidPx[0] + firstStrat.instru.askPx[0]) / 2)
                    - ((secondStrat.instru.bidPx[0] + secondStrat.instru.askPx[0]) / 2);
            expectedRatio = currSpreadRatio;
        }

        // C++: AVG_SPREAD_AWAY 检查
        if (Math.abs(currSpreadRatio - avgSpreadRatio)
                > firstStrat.instru.tickSize * firstStrat.thold.AVG_SPREAD_AWAY) {
            is_valid_mkdata = false;
            log.warning("Error avgSpreadRatio, Exit Strategy. currSpread:" + currSpreadRatio
                    + " avgSpread:" + avgSpreadRatio + " AVG_SPREAD_AWAY:" + firstStrat.thold.AVG_SPREAD_AWAY);
            if (active) {
                handleSquareoff();
            }
            return;
        }

        // C++: 收到第一腿行情时更新 avgSpreadRatio_ori (EWA)
        // Ref: PairwiseArbStrategy.cpp:519-523
        // 读取行情中的 symbol 进行比对
        byte[] symBytes = new byte[32];
        MemorySegment symSeg = update.asSlice(Types.MDH_SYMBOL_OFFSET, 32);
        symSeg.asByteBuffer().get(symBytes);
        String symbol = new String(symBytes, StandardCharsets.US_ASCII).trim().replace("\0", "");

        if (symbol.equals(firstStrat.instru.symbol)) {
            avgSpreadRatio_ori = (1 - firstStrat.thold.ALPHA) * avgSpreadRatio_ori
                    + firstStrat.thold.ALPHA * currSpreadRatio;
            avgSpreadRatio = avgSpreadRatio_ori + tValue;
        }
        currSpreadRatio_prev = currSpreadRatio;

        is_valid_mkdata = true;

        // C++: 时间限制检查 — endTimeAggEpoch / endTimeEpoch
        long currentTime = System.nanoTime();
        if (currentTime >= endTimeAggEpoch && !aggFlat) {
            aggFlat = true;
            onExit = true;
            onCancel = true;
            onFlat = true;
            log.warning("Exchange Time Limit reached. Aggressive flat!");
            handleSquareoff();
        }

        if (currentTime >= endTimeEpoch && !onExit) {
            onExit = true;
            onCancel = true;
            onFlat = true;
            log.warning("END TIME limit reached. Square off called.");
            handleSquareoff();
        }
    }

    // =======================================================================
    //  HandleSquareoff — 双腿平仓
    // =======================================================================

    /**
     * 双腿平仓 — 设置退出标志 + 撤销所有订单 + 保存 daily_init。
     * 迁移自: PairwiseArbStrategy::HandleSquareoff()
     * Ref: PairwiseArbStrategy.cpp:586-626
     */
    @Override
    public void handleSquareoff() {
        // C++: 设置双腿退出标志
        firstStrat.onExit = true;
        firstStrat.onCancel = true;
        firstStrat.onFlat = true;
        secondStrat.onExit = true;
        secondStrat.onCancel = true;
        secondStrat.onFlat = true;

        // C++: 撤销两腿所有订单
        for (OrderStats order : new ArrayList<>(ordMap1.values())) {
            firstStrat.sendCancelOrder(firstinstru, order.orderID);
        }
        for (OrderStats order : new ArrayList<>(ordMap2.values())) {
            secondStrat.sendCancelOrder(secondinstru, order.orderID);
        }

        active = false;

        // C++: SaveMatrix2("../data/daily_init." + strategyID)
        saveMatrix2("../data/daily_init." + strategyID);
    }

    // =======================================================================
    //  CalcPendingNetposAgg
    // =======================================================================

    /**
     * 计算第二腿挂起的 CROSS/MATCH 订单净仓位。
     * 迁移自: PairwiseArbStrategy::CalcPendingNetposAgg()
     * Ref: PairwiseArbStrategy.cpp:688-698
     */
    public int calcPendingNetposAgg() {
        int netpos_agg_pending = 0;
        for (OrderStats order : ordMap2.values()) {
            if (order.hitType == OrderStats.HitType.CROSS || order.hitType == OrderStats.HitType.MATCH) {
                if (order.side == Constants.SIDE_BUY) {
                    netpos_agg_pending += order.openQty;
                } else {
                    netpos_agg_pending -= order.openQty;
                }
            }
        }
        return netpos_agg_pending;
    }

    // =======================================================================
    //  HandleAggOrder — 对冲订单处理
    // =======================================================================

    /**
     * 处理对冲订单的回报计数。
     * 迁移自: PairwiseArbStrategy::HandleAggOrder(ResponseMsg*, OrderStats*, ExtraStrategy*)
     * Ref: PairwiseArbStrategy.cpp:402-426
     */
    private void handleAggOrder(MemorySegment response, OrderStats order, ExtraStrategy strat) {
        int responseType = (int) Types.RESP_RESPONSE_TYPE_VH.get(response, 0L);
        int tradeQty = (int) Types.RESP_QUANTITY_VH.get(response, 0L);

        // C++: 在特定回报类型时递减 aggOrder 计数
        if (responseType == Constants.RESP_NEW_ORDER_FREEZE
                || responseType == Constants.RESP_ORDER_ERROR
                || responseType == Constants.RESP_CANCEL_ORDER_CONFIRM
                || (responseType == Constants.RESP_TRADE_CONFIRM && order.openQty == tradeQty)) {
            if (order.side == Constants.SIDE_BUY) {
                strat.buyAggOrder--;
            } else {
                strat.sellAggOrder--;
            }
        }
    }

    // =======================================================================
    //  GetBidPrice / GetAskPrice — 价格优化
    // =======================================================================

    /**
     * 获取第一腿买价（考虑隐藏订单簿优化）。
     * 迁移自: PairwiseArbStrategy::GetBidPrice_first(double&, OrderHitType&, int32_t&)
     * Ref: PairwiseArbStrategy.cpp:802-820
     */
    public double getBidPriceFirst(double price, int level) {
        // C++: 简化版 — 不含 invisible book 逻辑（configParams.bUseInvisibleBook 未迁移）
        return price;
    }

    /**
     * 获取第一腿卖价。
     * 迁移自: PairwiseArbStrategy::GetAskPrice_first(double&, OrderHitType&, int32_t&)
     * Ref: PairwiseArbStrategy.cpp:822-840
     */
    public double getAskPriceFirst(double price, int level) {
        return price;
    }

    // =======================================================================
    //  辅助方法
    // =======================================================================

    /**
     * 找买单价格 Map 中已确认的最低价。
     */
    private double findLowestConfirmedBidPrice(Map<Double, OrderStats> bidPriceMap) {
        double lowest = 0;
        for (OrderStats order : bidPriceMap.values()) {
            if (order.status == OrderStats.Status.NEW_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_REJECT) {
                if (lowest == 0 || order.price < lowest) {
                    lowest = order.price;
                }
            }
        }
        return lowest;
    }

    /**
     * 找卖单价格 Map 中已确认的最高价。
     */
    private double findHighestConfirmedAskPrice(Map<Double, OrderStats> askPriceMap) {
        double highest = 0;
        for (OrderStats order : askPriceMap.values()) {
            if (order.status == OrderStats.Status.NEW_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_REJECT) {
                if (highest == 0 || order.price > highest) {
                    highest = order.price;
                }
            }
        }
        return highest;
    }

    @Override
    public String toString() {
        return String.format("PairwiseArbStrategy[id=%d, spread=%.4f, avg=%.4f, firstNetpos=%d, secondNetpos=%d]",
                strategyID, currSpreadRatio, avgSpreadRatio,
                firstStrat != null ? firstStrat.netpos_pass : 0,
                secondStrat != null ? secondStrat.netpos_agg : 0);
    }
}
