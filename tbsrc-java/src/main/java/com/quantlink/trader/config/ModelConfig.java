package com.quantlink.trader.config;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;

/**
 * model .par.txt 文件解析结果。
 * 迁移自: tbsrc/main/TradeBotUtils.cpp:1983-2276 — LoadModelFile()
 *
 * 规则:
 *   - # 开头为注释（特殊 #DEP_STD_DEV, #LOOKAHEAD, #TRGT_STD_DEV 除外）
 *   - 3+ tokens: indicator 行（忽略）
 *   - 2 tokens: 阈值 key-value
 *   - 1 token: 跳过
 */
public class ModelConfig {
    /** C++ UPPER_CASE key → string value */
    public final Map<String, String> thresholds = new HashMap<>();

    /**
     * 解析 model .par.txt 文件。
     * C++: LoadModelFile() in TradeBotUtils.cpp:1983-2276
     */
    public static ModelConfig loadModelFile(String path) throws IOException {
        ModelConfig mc = new ModelConfig();

        try (BufferedReader reader = Files.newBufferedReader(Path.of(path))) {
            String line;
            while ((line = reader.readLine()) != null) {
                line = line.trim();
                if (line.isEmpty()) continue;

                // # 开头为注释，但 #DEP_STD_DEV, #LOOKAHEAD, #TRGT_STD_DEV 是特殊参数
                if (line.startsWith("#")) {
                    String rest = line.substring(1).trim();
                    String[] tokens = rest.split("\\s+");
                    if (tokens.length == 2) {
                        String key = tokens[0];
                        if (key.equals("DEP_STD_DEV") || key.equals("LOOKAHEAD") || key.equals("TRGT_STD_DEV")) {
                            mc.thresholds.put(key, tokens[1]);
                        }
                    }
                    continue;
                }

                String[] tokens = line.split("\\s+");
                if (tokens.length == 2) {
                    // Threshold: KEY VALUE
                    mc.thresholds.put(tokens[0], tokens[1]);
                }
                // 3+ tokens: indicator line (ignored for now)
            }
        }

        return mc;
    }
}
