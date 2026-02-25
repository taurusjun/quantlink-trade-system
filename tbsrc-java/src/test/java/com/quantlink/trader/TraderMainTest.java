package com.quantlink.trader;

import com.quantlink.trader.config.*;
import com.quantlink.trader.core.ConfigParams;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TraderMain 主程序单元测试。
 * 测试范围: CLI 解析、配置加载逻辑（不涉及 SHM）。
 */
class TraderMainTest {

    @BeforeEach
    void resetSingleton() {
        ConfigParams.resetInstance();
    }

    // =======================================================================
    //  CLI 参数解析
    // =======================================================================

    @Test
    void test_parseArgs_allParams() {
        TraderMain trader = new TraderMain();
        trader.parseArgs(new String[]{
            "--Live",
            "-controlFile", "./controls/test.txt",
            "-strategyID", "92201",
            "-configFile", "./config/test.cfg",
            "-dataDir", "./mydata",
            "-yearPrefix", "26",
            "-printMod", "5"
        });

        assertEquals("./controls/test.txt", trader.controlFile);
        assertEquals(92201, trader.strategyID);
        assertEquals("./config/test.cfg", trader.configFile);
        assertEquals("./mydata", trader.dataDir);
        assertEquals("26", trader.yearPrefix);
        assertEquals(5, trader.printMod);
    }

    @Test
    void test_parseArgs_minimalParams() {
        TraderMain trader = new TraderMain();
        trader.parseArgs(new String[]{
            "--Live",
            "-controlFile", "./controls/test.txt",
            "-strategyID", "92201",
            "-configFile", "./config/test.cfg"
        });

        assertEquals("./controls/test.txt", trader.controlFile);
        assertEquals(92201, trader.strategyID);
        assertEquals("./config/test.cfg", trader.configFile);
        assertEquals("./data", trader.dataDir); // default
    }

    @Test
    void test_parseArgs_yearPrefixDefault() {
        TraderMain trader = new TraderMain();
        trader.parseArgs(new String[]{
            "--Live",
            "-controlFile", "./controls/test.txt",
            "-strategyID", "92201",
            "-configFile", "./config/test.cfg"
        });

        // yearPrefix 应自动填充当前年后两位
        assertFalse(trader.yearPrefix.isEmpty());
        assertEquals(2, trader.yearPrefix.length());
    }

    @Test
    void test_parseArgs_missingLiveMode() {
        TraderMain trader = new TraderMain();
        assertThrows(IllegalArgumentException.class, () ->
            trader.parseArgs(new String[]{"--Sim", "-controlFile", "x", "-strategyID", "1", "-configFile", "y"}));
    }

    @Test
    void test_parseArgs_missingControlFile() {
        TraderMain trader = new TraderMain();
        assertThrows(IllegalArgumentException.class, () ->
            trader.parseArgs(new String[]{"--Live", "-strategyID", "92201", "-configFile", "y"}));
    }

    @Test
    void test_parseArgs_missingStrategyID() {
        TraderMain trader = new TraderMain();
        assertThrows(IllegalArgumentException.class, () ->
            trader.parseArgs(new String[]{"--Live", "-controlFile", "x", "-configFile", "y"}));
    }

    @Test
    void test_parseArgs_missingConfigFile() {
        TraderMain trader = new TraderMain();
        assertThrows(IllegalArgumentException.class, () ->
            trader.parseArgs(new String[]{"--Live", "-controlFile", "x", "-strategyID", "92201"}));
    }

    @Test
    void test_parseArgs_emptyArgs() {
        TraderMain trader = new TraderMain();
        assertThrows(IllegalArgumentException.class, () ->
            trader.parseArgs(new String[]{}));
    }

    @Test
    void test_parseArgs_ignoreUnknownFlags() {
        TraderMain trader = new TraderMain();
        trader.parseArgs(new String[]{
            "--Live",
            "-controlFile", "./controls/test.txt",
            "-strategyID", "92201",
            "-configFile", "./config/test.cfg",
            "-adjustLTP", "100",
            "-updateInterval", "200",
            "-unknownFlag", "value"
        });

        assertEquals("./controls/test.txt", trader.controlFile);
        assertEquals(92201, trader.strategyID);
    }

    // =======================================================================
    //  shutdown 幂等性
    // =======================================================================

    @Test
    void test_shutdown_idempotent() {
        TraderMain trader = new TraderMain();
        // 未初始化时调用 shutdown 不应抛异常
        trader.shutdown();
        assertFalse(trader.running);
        // 再次调用也不应抛异常
        trader.shutdown();
        assertFalse(trader.running);
    }

    // =======================================================================
    //  shutdownLatch
    // =======================================================================

    @Test
    void test_shutdownLatch_countdown() {
        TraderMain trader = new TraderMain();
        assertEquals(1, trader.shutdownLatch.getCount());
        trader.shutdownLatch.countDown();
        assertEquals(0, trader.shutdownLatch.getCount());
    }

    // =======================================================================
    //  端到端配置加载 (无 SHM)
    // =======================================================================

    @Test
    void test_configFlow_endToEnd(@TempDir Path tempDir) throws Exception {
        // 准备 controlFile
        Path controlFile = tempDir.resolve("control.txt");
        Files.writeString(controlFile,
            "ag_F_3_SFE ./models/model.txt SFE 16 TB_PAIR_STRAT 0900 1500 ag_F_5_SFE\n");

        // 准备 configFile
        Path configFile = tempDir.resolve("config.cfg");
        Files.writeString(configFile, """
            EXCHANGES=CHINA_SHFE
            PRODUCT=AG
            [CHINA_SHFE]
            MDSHMKEY=4097
            ORSREQUESTSHMKEY=8193
            ORSRESPONSESHMKEY=12289
            CLIENTSTORESHMKEY=16385
            MDSHMSIZE=2048
            ORSREQUESTSHMSIZE=1024
            ORSRESPONSESHMSIZE=1024
            """);

        // 准备 model file
        Path modelFile = tempDir.resolve("models");
        Files.createDirectory(modelFile);
        Path model = modelFile.resolve("model.txt");
        Files.writeString(model, """
            ag_F_3_SFE FUTCOM Dependant 0 MID_PX
            ag_F_5_SFE FUTCOM Dependant 0 MID_PX
            SIZE 4
            MAX_SIZE 16
            BEGIN_PLACE 5.0
            LONG_PLACE 8.0
            SHORT_PLACE 2.0
            BEGIN_REMOVE 2.0
            ALPHA 0.0001
            AVG_SPREAD_AWAY 40
            """);

        // 解析 controlFile
        ControlConfig cc = ControlConfig.loadControlFile(controlFile.toString());
        assertEquals("ag_F_3_SFE", cc.baseName);
        assertEquals("ag_F_5_SFE", cc.secondName);

        // 解析 configFile
        CfgConfig cfg = CfgConfig.loadCfg(configFile.toString());
        assertEquals("AG", cfg.product);
        int[] shm = cfg.getExchangeShmConfig("CHINA_SHFE");
        assertEquals(4097, shm[0]);

        // 解析 model file
        ModelConfig mc = ModelConfig.loadModelFile(model.toString());
        assertEquals("4", mc.thresholds.get("SIZE"));
        assertEquals("16", mc.thresholds.get("MAX_SIZE"));

        // baseName → symbol
        String sym1 = ConfigParser.baseNameToSymbol(cc.baseName, "26");
        String sym2 = ConfigParser.baseNameToSymbol(cc.secondName, "26");
        assertEquals("ag2603", sym1);
        assertEquals("ag2605", sym2);

        // exchange → name
        assertEquals("SHFE", ConfigParser.exchangeToName(cc.exchange));
    }
}
