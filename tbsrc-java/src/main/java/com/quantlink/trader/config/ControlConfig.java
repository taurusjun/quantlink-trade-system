package com.quantlink.trader.config;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * controlFile 解析结果。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h:602-613 — struct ControlConfig
 * C++: LoadControlFile() in TradeBotUtils.cpp:1820-1865
 *
 * 格式: 单行空格分隔
 *   baseName modelFile exchange id execStrat startTime endTime [secondName] [thirdName]
 */
public class ControlConfig {
    public String baseName;    // Token[0]: e.g. ag_F_3_SFE
    public String modelFile;   // Token[1]: e.g. ./models/model.ag2603.ag2605.par.txt.92201
    public String exchange;    // Token[2]: e.g. SFE
    public String id;          // Token[3]: e.g. 16
    public String execStrat;   // Token[4]: e.g. TB_PAIR_STRAT
    public String startTime;   // Token[5]: e.g. 0900
    public String endTime;     // Token[6]: e.g. 1500
    public String secondName;  // Token[7]: e.g. ag_F_5_SFE (pair strategies)
    public String thirdName;   // Token[8]: (butterfly/arb strategies)

    /**
     * 解析 C++ controlFile 格式。
     * C++: LoadControlFile() in TradeBotUtils.cpp:1820-1865
     */
    public static ControlConfig parse(String path) throws IOException {
        String content = Files.readString(Path.of(path));
        String line = null;
        for (String l : content.split("\n")) {
            l = l.trim();
            if (!l.isEmpty() && !l.startsWith("#")) {
                line = l;
                break;
            }
        }
        if (line == null) {
            throw new IOException("controlFile: " + path + ": 文件为空或仅包含注释");
        }

        String[] tokens = line.split("\\s+");
        if (tokens.length < 7) {
            throw new IOException("controlFile: " + path + ": 至少需要 7 个字段，实际 " + tokens.length);
        }

        ControlConfig cc = new ControlConfig();
        cc.baseName = tokens[0];
        cc.modelFile = tokens[1];
        cc.exchange = tokens[2];
        cc.id = tokens[3];
        cc.execStrat = tokens[4];
        cc.startTime = tokens[5];
        cc.endTime = tokens[6];
        if (tokens.length >= 8) cc.secondName = tokens[7];
        if (tokens.length >= 9) cc.thirdName = tokens[8];
        return cc;
    }
}
