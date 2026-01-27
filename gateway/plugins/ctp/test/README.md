# CTP持仓查询和平仓工具

## 工具说明

`ctp_query_and_close` 是一个独立的CTP持仓查询和平仓工具，用于：
- 查询当前持仓
- 查询指定合约持仓
- 平仓指定合约（支持平今/平昨仓）

## 编译

工具已经在CTP插件的CMakeLists.txt中配置，使用标准编译流程：

```bash
cd gateway
mkdir -p build && cd build
cmake ..
make ctp_query_and_close
```

编译完成后，可执行文件位于：`gateway/build/plugins/ctp/ctp_query_and_close`

## 使用方法

### 1. 查询所有持仓

```bash
./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml
```

**输出示例**（有持仓时）：

```
========================================
当前持仓列表 (2)
========================================

合约: ag2603
  方向: 多头
  今仓: 1
  昨仓: 0
  总持仓: 1
  持仓均价: 29633.00
  浮动盈亏: 28.00
  保证金: 17780.00

合约: ag2605
  方向: 空头
  今仓: 0
  昨仓: 2
  总持仓: 2
  持仓均价: 30100.00
  浮动盈亏: -45.00
  保证金: 36120.00
========================================
```

**输出示例**（无持仓时）：

```
✅ 当前无持仓
```

### 2. 查询指定合约持仓

```bash
./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml ag2603
```

只显示ag2603的持仓信息。

### 3. 平仓指定合约

```bash
./gateway/build/plugins/ctp/ctp_query_and_close config/ctp/ctp_td.yaml ag2603 close
```

**平仓逻辑**：
- 自动识别持仓方向（多头→卖出平仓，空头→买入平仓）
- 先平昨仓（CLOSE_YESTERDAY），再平今仓（CLOSE_TODAY）
- 平仓价格：持仓均价 ± 50（快速成交）
- 等待5秒后再次查询持仓确认

**输出示例**：

```
⚠️  开始平仓操作...
========================================

平仓 ag2603:
  方向: 多头→卖出
  数量: 1
  平仓价: 29583.00

  [2/2] 平今仓 1 手...
  ✅ 平今仓订单已发送: ORD_1769240567123456789

⏳ 等待成交确认（5秒）...

📊 查询最新持仓...

✅ 当前无持仓
```

## 配置文件

工具使用标准的CTP交易配置文件：`config/ctp/ctp_td.yaml`

确保配置文件包含正确的：
- CTP服务器地址
- 经纪商ID
- 投资者账号和密码（存储在 `config/ctp/ctp_td.secret.yaml`）

## 注意事项

1. **平仓价格策略**：
   - 多头平仓：使用持仓均价 - 50（确保快速成交）
   - 空头平仓：使用持仓均价 + 50（确保快速成交）
   - 可以根据市场情况调整价格偏移量

2. **平仓顺序**：
   - 先平昨仓（手续费较低）
   - 再平今仓（某些品种今仓手续费较高）

3. **查询限制**：
   - CTP限制查询频率（通常1秒1次）
   - 工具会自动等待适当的时间

4. **实盘使用警告**：
   - 在实盘环境使用前，请先在SimNow测试
   - 确认平仓价格合理后再执行
   - 建议先手动检查持仓，再执行平仓

## 技术细节

### 核心数据结构

使用统一的交易插件接口 (`plugin/td_plugin_interface.h`)：

- `PositionInfo`: 持仓信息
  - `symbol`: 合约代码
  - `direction`: 持仓方向（BUY多头/SELL空头）
  - `volume`: 总持仓量
  - `today_volume`: 今仓
  - `yesterday_volume`: 昨仓
  - `avg_price`: 持仓均价
  - `position_profit`: 浮动盈亏
  - `margin`: 占用保证金

- `OrderRequest`: 订单请求
  - `symbol`: 合约代码
  - `direction`: 买卖方向
  - `offset`: 开平标志（OPEN/CLOSE/CLOSE_TODAY/CLOSE_YESTERDAY）
  - `price_type`: 价格类型（LIMIT/MARKET）
  - `price`: 价格
  - `volume`: 数量

### 实现文件

- **源文件**: `gateway/plugins/ctp/test/query_and_close.cpp`
- **依赖插件**: `CTPTDPlugin` (gateway/plugins/ctp/src/ctp_td_plugin.cpp)
- **接口定义**: `ITDPlugin` (gateway/include/plugin/td_plugin_interface.h)

### 编译配置

在 `gateway/plugins/ctp/CMakeLists.txt` 中已添加：

```cmake
add_executable(ctp_query_and_close
    src/ctp_td_config.cpp
    src/ctp_td_plugin.cpp
    test/query_and_close.cpp
)
```

## 故障排查

### 问题：编译错误

**解决方案**：
- 确保yaml-cpp已安装：`brew install yaml-cpp`
- 确保CTP SDK已下载到 `gateway/third_party/ctp/`
- 重新运行cmake：`cd gateway/build && cmake .. && make`

### 问题：连接超时

**解决方案**：
- 检查网络连接
- 确认CTP服务器地址正确（SimNow: tcp://182.254.243.31:30001）
- 检查配置文件中的账号和密码

### 问题：查询持仓失败

**解决方案**：
- 等待系统就绪（登录后等待3-5秒）
- 检查日志文件：`log/ctp_td.log`
- 确认账号有查询权限

### 问题：平仓订单被拒绝

**可能原因**：
- 价格超出涨跌停板
- 今仓/昨仓标志错误
- 持仓不足
- 交易时段不正确

**解决方案**：
- 查看错误信息
- 调整平仓价格
- 确认持仓数量
- 检查交易时间

## 相关文档

- [CTP持仓查询和平仓指南](../../../../CTP_POSITION_GUIDE.md)
- [CTP交易插件实现](../src/ctp_td_plugin.cpp)
- [交易插件接口定义](../../../include/plugin/td_plugin_interface.h)

## 版本历史

- **v1.0** (2026-01-27): 初始版本
  - 持仓查询功能
  - 指定合约查询
  - 平仓功能（支持平今/平昨仓）

---

**最后更新**: 2026-01-27
**维护者**: QuantLink Team
