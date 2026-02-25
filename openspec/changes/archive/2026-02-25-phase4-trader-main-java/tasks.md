# Phase 4 Tasks: Trader 主程序入口

## 4.1 配置解析层
- [x] 4.1.1 创建 `config/ControlConfig.java` — controlFile 解析
- [x] 4.1.2 创建 `config/CfgConfig.java` — .cfg INI 文件解析
- [x] 4.1.3 创建 `config/ModelConfig.java` — model .par.txt 文件解析
- [x] 4.1.4 创建 `config/ConfigParser.java` — 统一入口 + baseName→symbol 转换
- [x] 4.1.5 创建 ConfigParserTest.java — 配置解析单元测试

## 4.2 TraderMain 主程序
- [x] 4.2.1 创建 `TraderMain.java` — CLI 参数解析 + main()
- [x] 4.2.2 实现初始化流程 — config→Connector→Client→Strategy→daily_init
- [x] 4.2.3 实现信号处理 — SIGUSR1/SIGUSR2/SIGTSTP/SIGTERM
- [x] 4.2.4 实现关闭流程 — 平仓→停止轮询→释放 SHM
- [x] 4.2.5 创建 TraderMainTest.java — 主程序单元测试

## 4.3 编译验证
- [x] 4.3.1 全量编译通过（`mvn compile`）
- [x] 4.3.2 全量测试通过（`mvn test`） — 168 tests, 0 failures
