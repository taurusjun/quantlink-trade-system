## 1. ConfigParser loadThresholds 重写

- [x] 1.1 重写 loadThresholds() 为 switch-case 链，对齐 C++ AddThreshold() 97 个分支
- [x] 1.2 实现时间单位转换（PAUSE×1e6, SQROFF_TIME×1e9 等 8 个时间字段）
- [x] 1.3 实现副作用赋值（SIZE→BEGIN_SIZE/BID_SIZE/ASK_SIZE, SMS_RATIO 计算等 10 个副作用）
- [x] 1.4 实现字段重映射（DECAY→DECAY1, PRODUCT→productName 等 11 个重映射）
- [x] 1.5 实现特殊布尔处理（USE_LINEAR_THOLD）
- [x] 1.6 未知参数抛出 IllegalArgumentException
- [x] 1.7 修正 bu tickSize 从 2.0 到 1.0
- [x] 1.8 更新 ConfigParser 类级 Javadoc（从 Go 源改为 C++ 源）

## 2. CommonClient 行情处理补齐

- [x] 2.1 sendInfraMDUpdate() 添加 endPkt==1 处理块
- [x] 2.2 实现 checkLastUpdate() 僵尸行情检测方法
- [x] 2.3 sendInfraMDUpdate() 添加 CheckLastUpdate 调用
- [x] 2.4 sendINDUpdate() 添加 UpdateActive() 交易时段检查
- [x] 2.5 修正 INVALID 判断从 AND 为 OR（bidQty==0 || askQty==0）

## 3. SimConfig DateConfig 补齐

- [x] 3.1 添加 startTimeEpoch/endTimeEpoch 字段
- [x] 3.2 修改 simActive 默认值从 true 到 false
- [x] 3.3 实现 updateActive(long currentTime) 方法
- [x] 3.4 实现 initDateConfigEpoch() 含夜盘跨日支持

## 4. TraderMain 集成

- [x] 4.1 添加 simConfig1.initDateConfigEpoch() 调用
- [x] 4.2 添加 simConfig2.initDateConfigEpoch() 调用

## 5. 验证

- [x] 5.1 编译通过 build_deploy_java.sh
- [x] 5.2 loadThresholds 97/97 分支与 C++ 对照验证通过
