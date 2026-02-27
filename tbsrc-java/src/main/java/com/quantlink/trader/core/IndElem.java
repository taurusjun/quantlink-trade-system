package com.quantlink.trader.core;

/**
 * 指标列表元素 — 模型文件中一行的解析结果。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h:219 — struct IndElem
 *
 * C++: struct IndElem { char m_baseName[50]; char m_type[50]; char m_indName[50];
 *      double m_coefficient; int m_index; int m_argCount; string m_argList[20];
 *      TradeBot::Indicator *m_indicator; };
 */
public class IndElem {

    // 迁移自: IndElem::m_baseName — 合约 symbol（如 "ag_F_3_SFE"）
    public String baseName = "";

    // 迁移自: IndElem::m_type — 合约类型（如 "FUTCOM", "BASKET"）
    public String type = "";

    // 迁移自: IndElem::m_indName — 指标名称（如 "Dependant", "BookDelta"）
    public String indName = "";

    // 迁移自: IndElem::m_coefficient — 模型系数（权重）
    public double coefficient;

    // 迁移自: IndElem::m_index — 在指标列表中的位置
    public int index;

    // 迁移自: IndElem::m_argCount, m_argList[20] — 指标参数
    public int argCount;
    public String[] argList = new String[20];

    // 迁移自: IndElem::m_indicator — 指标实例引用
    public Indicator indicator;

    public IndElem() {
        for (int i = 0; i < argList.length; i++) {
            argList[i] = "";
        }
    }
}
