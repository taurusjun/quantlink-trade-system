package com.quantlink.trader.config;

import com.quantlink.trader.core.ThresholdSet;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.PrintWriter;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * 配置解析单元测试。
 */
class ConfigParserTest {

    // =======================================================================
    //  ControlConfig 测试
    // =======================================================================

    @Test
    void test_controlConfig_parse(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("control.txt");
        Files.writeString(file,
            "ag_F_3_SFE ./models/model.ag2603.ag2605.par.txt.92201 SFE 16 TB_PAIR_STRAT 0900 1500 ag_F_5_SFE\n");

        ControlConfig cc = ControlConfig.loadControlFile(file.toString());

        assertEquals("ag_F_3_SFE", cc.baseName);
        assertEquals("./models/model.ag2603.ag2605.par.txt.92201", cc.modelFile);
        assertEquals("SFE", cc.exchange);
        assertEquals("16", cc.id);
        assertEquals("TB_PAIR_STRAT", cc.execStrat);
        assertEquals("0900", cc.startTime);
        assertEquals("1500", cc.endTime);
        assertEquals("ag_F_5_SFE", cc.secondName);
    }

    @Test
    void test_controlConfig_skipComments(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("control.txt");
        Files.writeString(file,
            "# comment line\n\nag_F_3_SFE ./models/model.txt SFE 16 TB_PAIR_STRAT 0900 1500 ag_F_5_SFE\n");

        ControlConfig cc = ControlConfig.loadControlFile(file.toString());
        assertEquals("ag_F_3_SFE", cc.baseName);
    }

    @Test
    void test_controlConfig_tooFewTokens(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("control.txt");
        Files.writeString(file, "ag_F_3_SFE ./models/model.txt SFE\n");

        assertThrows(Exception.class, () -> ControlConfig.loadControlFile(file.toString()));
    }

    // =======================================================================
    //  CfgConfig 测试
    // =======================================================================

    @Test
    void test_cfgConfig_parse(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("config.cfg");
        Files.writeString(file, """
            EXCHANGES=CHINA_SHFE
            PRODUCT = AG
            [CHINA_SHFE]
            MDSHMKEY           = 4097
            ORSREQUESTSHMKEY   = 8193
            ORSRESPONSESHMKEY  = 12289
            CLIENTSTORESHMKEY  = 16385
            MDSHMSIZE          = 2048
            ORSREQUESTSHMSIZE  = 1024
            ORSRESPONSESHMSIZE = 1024
            """);

        CfgConfig cfg = CfgConfig.loadCfg(file.toString());

        assertEquals("AG", cfg.product);
        assertEquals("CHINA_SHFE", cfg.exchanges);

        int[] shm = cfg.getExchangeShmConfig("CHINA_SHFE");
        assertEquals(4097, shm[0]);  // mdKey
        assertEquals(8193, shm[1]);  // reqKey
        assertEquals(12289, shm[2]); // respKey
        assertEquals(16385, shm[3]); // clientStoreKey
        assertEquals(2048, shm[4]);  // mdSize
    }

    @Test
    void test_cfgConfig_defaultExchange(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("config.cfg");
        Files.writeString(file, """
            EXCHANGES=CHINA_SHFE
            [CHINA_SHFE]
            MDSHMKEY = 4097
            ORSREQUESTSHMKEY = 8193
            ORSRESPONSESHMKEY = 12289
            CLIENTSTORESHMKEY = 16385
            MDSHMSIZE = 2048
            ORSREQUESTSHMSIZE = 1024
            ORSRESPONSESHMSIZE = 1024
            """);

        CfgConfig cfg = CfgConfig.loadCfg(file.toString());
        // 传空字符串，应使用默认 EXCHANGES
        int[] shm = cfg.getExchangeShmConfig("");
        assertEquals(4097, shm[0]);
    }

    @Test
    void test_cfgConfig_missingSection(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("config.cfg");
        Files.writeString(file, "EXCHANGES=CHINA_SHFE\n");

        CfgConfig cfg = CfgConfig.loadCfg(file.toString());
        assertThrows(IllegalArgumentException.class, () -> cfg.getExchangeShmConfig("CHINA_SHFE"));
    }

    // =======================================================================
    //  ModelConfig 测试
    // =======================================================================

    @Test
    void test_modelConfig_parse(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("model.txt");
        Files.writeString(file, """
            # Symbol definitions
            ag_F_3_SFE FUTCOM Dependant 0 MID_PX
            ag_F_5_SFE FUTCOM Dependant 0 MID_PX

            # Thresholds
            MAX_QUOTE_LEVEL 3
            SIZE 4
            MAX_SIZE 16
            BEGIN_PLACE 5.006894
            LONG_PLACE 7.510341
            SHORT_PLACE 2.503447
            BEGIN_REMOVE 2.0027576
            ALPHA 0.0000240672
            AVG_SPREAD_AWAY 40
            SUPPORTING_ORDERS 2

            # Special commented parameters
            #DEP_STD_DEV 0.5
            #LOOKAHEAD 10
            """);

        ModelConfig mc = ModelConfig.loadModelFile(file.toString());

        assertEquals("3", mc.thresholds.get("MAX_QUOTE_LEVEL"));
        assertEquals("4", mc.thresholds.get("SIZE"));
        assertEquals("16", mc.thresholds.get("MAX_SIZE"));
        assertEquals("5.006894", mc.thresholds.get("BEGIN_PLACE"));
        assertEquals("7.510341", mc.thresholds.get("LONG_PLACE"));
        assertEquals("0.0000240672", mc.thresholds.get("ALPHA"));
        assertEquals("40", mc.thresholds.get("AVG_SPREAD_AWAY"));

        // Special # parameters
        assertEquals("0.5", mc.thresholds.get("DEP_STD_DEV"));
        assertEquals("10", mc.thresholds.get("LOOKAHEAD"));
    }

    @Test
    void test_modelConfig_ignoreIndicatorLines(@TempDir Path tempDir) throws Exception {
        Path file = tempDir.resolve("model.txt");
        Files.writeString(file, """
            ag_F_3_SFE FUTCOM Dependant 0 MID_PX
            SIZE 4
            """);

        ModelConfig mc = ModelConfig.loadModelFile(file.toString());
        assertEquals(1, mc.thresholds.size()); // only SIZE, not the indicator line
        assertEquals("4", mc.thresholds.get("SIZE"));
    }

    // =======================================================================
    //  baseName→symbol 转换测试
    // =======================================================================

    @Test
    void test_baseNameToSymbol() {
        assertEquals("ag2603", ConfigParser.baseNameToSymbol("ag_F_3_SFE", "26"));
        assertEquals("ag2605", ConfigParser.baseNameToSymbol("ag_F_5_SFE", "26"));
        assertEquals("au2604", ConfigParser.baseNameToSymbol("au_F_4_SFE", "26"));
        assertEquals("rb2512", ConfigParser.baseNameToSymbol("rb_F_12_SFE", "25"));
    }

    @Test
    void test_baseNameToSymbol_singleDigitMonth() {
        assertEquals("ag2601", ConfigParser.baseNameToSymbol("ag_F_1_SFE", "26"));
        assertEquals("ag2609", ConfigParser.baseNameToSymbol("ag_F_9_SFE", "26"));
    }

    @Test
    void test_baseNameToSymbol_invalidFormat() {
        assertThrows(IllegalArgumentException.class,
            () -> ConfigParser.baseNameToSymbol("ag_O_C_10", "26"));
    }

    // =======================================================================
    //  exchangeToName 测试
    // =======================================================================

    @Test
    void test_exchangeToName() {
        assertEquals("SHFE", ConfigParser.exchangeToName("SFE"));
        assertEquals("ZCE", ConfigParser.exchangeToName("ZCE"));
        assertEquals("DCE", ConfigParser.exchangeToName("DCE"));
        assertEquals("CFFEX", ConfigParser.exchangeToName("CFFEX"));
    }

    // =======================================================================
    //  extractProduct 测试
    // =======================================================================

    @Test
    void test_extractProduct() {
        assertEquals("ag", ConfigParser.extractProduct("ag2603"));
        assertEquals("au", ConfigParser.extractProduct("au2604"));
        assertEquals("rb", ConfigParser.extractProduct("rb2505"));
    }

    // =======================================================================
    //  loadThresholds 测试
    // =======================================================================

    @Test
    void test_loadThresholds() {
        ThresholdSet ts = new ThresholdSet();
        Map<String, String> thresholds = Map.of(
            "BEGIN_PLACE", "5.0",
            "LONG_PLACE", "8.0",
            "SHORT_PLACE", "2.0",
            "SIZE", "4",
            "MAX_SIZE", "16",
            "ALPHA", "0.001",
            "CHECK_PNL", "1"
        );

        ConfigParser.loadThresholds(ts, thresholds);

        assertEquals(5.0, ts.BEGIN_PLACE, 0.001);
        assertEquals(8.0, ts.LONG_PLACE, 0.001);
        assertEquals(2.0, ts.SHORT_PLACE, 0.001);
        assertEquals(4, ts.SIZE);
        assertEquals(16, ts.MAX_SIZE);
        assertEquals(0.001, ts.ALPHA, 0.0001);
        assertTrue(ts.CHECK_PNL);
    }

    @Test
    void test_loadThresholds_unknownField() {
        ThresholdSet ts = new ThresholdSet();
        Map<String, String> thresholds = Map.of("UNKNOWN_FIELD", "123");

        // Should not throw
        ConfigParser.loadThresholds(ts, thresholds);
    }

    // =======================================================================
    //  产品查表测试
    // =======================================================================

    @Test
    void test_getTickSize() {
        assertEquals(1.0, ConfigParser.getTickSize("ag"));
        assertEquals(0.02, ConfigParser.getTickSize("au"));
        assertEquals(5.0, ConfigParser.getTickSize("al"));
    }

    @Test
    void test_getLotSize() {
        assertEquals(15.0, ConfigParser.getLotSize("ag"));
        assertEquals(1000.0, ConfigParser.getLotSize("au"));
        assertEquals(10.0, ConfigParser.getLotSize("rb"));
    }
}
