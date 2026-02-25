## ADDED Requirements

### Requirement: MarketUpdateNew StructLayout 定义

系统 SHALL 定义 `MarketUpdateNew` 的 Panama `StructLayout`，总大小 MUST 等于 816 bytes，由 `MDHeaderPart`（96 bytes）和 `MDDataPart`（720 bytes）组成。

- 迁移自: `hftbase/CommonUtils/include/marketupdateNew.h` — `class MarketUpdateNew : public MDHeaderPart, public MDDataPart`

所有字段的 offset MUST 与 C++ 编译器输出完全一致。

#### Scenario: MarketUpdateNew 总大小验证
- **WHEN** 计算 `MARKET_UPDATE_NEW_LAYOUT.byteSize()`
- **THEN** 结果 MUST 等于 816

#### Scenario: MDHeaderPart 字段 offset 验证
- **WHEN** 检查 MDHeaderPart 各字段 offset
- **THEN** `exchTS=0, timestamp=8, seqnum=16, rptSeqnum=24, tokenId=32, symbol=40, symbolID=88, exchangeName=90`，总大小 96 bytes（offset 91 后有 5 bytes padding）

#### Scenario: MDDataPart 字段 offset 验证
- **WHEN** 检查 MDDataPart 各字段 offset（相对于 MDDataPart 起始）
- **THEN** `newPrice=0, oldPrice=8, lastTradedPrice=16, lastTradedTime=24, totalTradedValue=32, totalTradedQuantity=40, yield=48, bidUpdates=56, askUpdates=376, newQuant=696, oldQuant=700, lastTradedQuantity=704, validBids=708, validAsks=709, updateLevel=710, endPkt=711, side=712, updateType=713, feedType=714`，总大小 720 bytes

#### Scenario: BookElement 子结构
- **WHEN** 检查 `BookElement` 布局
- **THEN** `quantity(int32)=0, orderCount(int32)=4, price(float64)=8`，总大小 16 bytes。`bidUpdates` 和 `askUpdates` 各含 20 个 BookElement（320 bytes）

### Requirement: RequestMsg StructLayout 定义

系统 SHALL 定义 `RequestMsg` 的 Panama `StructLayout`，总大小 MUST 等于 256 bytes。

- 迁移自: `hftbase/CommonUtils/include/orderresponse.h` — `struct RequestMsg __attribute__((aligned(64)))`

#### Scenario: RequestMsg 总大小验证
- **WHEN** 计算 `REQUEST_MSG_LAYOUT.byteSize()`
- **THEN** 结果 MUST 等于 256

#### Scenario: RequestMsg 字段 offset 验证
- **WHEN** 检查 RequestMsg 各字段 offset
- **THEN** `contractDesc=0, requestType=96, ordType=100, duration=104, pxType=108, posDirection=112, orderID=116, token=120, quantity=124, quantityFilled=128, disclosedQnty=132, price=136, timeStamp=144, accountID=152, transactionType=163, exchangeType=164, padding=165, product=185, strategyID=220`，offset 217-219 有 3 bytes 隐式 padding，offset 224-255 有 32 bytes 尾部 padding（aligned(64)）

#### Scenario: QueueElem<RequestMsg> 大小
- **WHEN** 计算 RequestMsg 的 QueueElem 大小
- **THEN** MUST 等于 320 bytes（非 264），因为 `aligned(64)` 使编译器将 256+8 向上对齐到 5×64=320

### Requirement: ResponseMsg StructLayout 定义

系统 SHALL 定义 `ResponseMsg` 的 Panama `StructLayout`，总大小 MUST 等于 176 bytes。

- 迁移自: `hftbase/CommonUtils/include/orderresponse.h` — `struct ResponseMsg`

#### Scenario: ResponseMsg 总大小验证
- **WHEN** 计算 `RESPONSE_MSG_LAYOUT.byteSize()`
- **THEN** 结果 MUST 等于 176

#### Scenario: ResponseMsg 字段 offset 验证
- **WHEN** 检查 ResponseMsg 各字段 offset
- **THEN** `responseType=0, childResponse=4, orderID=8, errorCode=12, quantity=16, price=24(前有4B padding), timeStamp=32, side=40, symbol=41, accountID=91, exchangeOrderId=104(前有2B padding), exchangeTradeId=112, openClose=133, exchangeID=134, product=135, strategyID=168(前有1B padding)`，offset 172-175 有 4 bytes 尾部 padding

### Requirement: ContractDescription StructLayout 定义

系统 SHALL 定义 `ContractDescription` 的 Panama `StructLayout`，总大小 MUST 等于 96 bytes。

- 迁移自: `hftbase/CommonUtils/include/orderresponse.h` — `struct ContractDescription`

#### Scenario: ContractDescription 字段 offset 验证
- **WHEN** 检查 ContractDescription 各字段 offset
- **THEN** `instrumentName=0(char[32]), symbol=32(char[50]), expiryDate=84(前有2B padding), strikePrice=88, optionType=92(char[2]), caLevel=94`，总大小 96 bytes

### Requirement: 枚举常量定义

系统 SHALL 定义与 C++ 完全一致的枚举常量，使用 Java `public static final int` 或 enum 类型。

- 迁移自: `hftbase/CommonUtils/include/orderresponse.h` — 全部枚举定义

MUST 包含以下枚举类型：
- `RequestType`（8 值：NEWORDER=0 ~ OPTEXEC_CANCEL=7）
- `ResponseType`（19 值：NEW_ORDER_CONFIRM=0 ~ NULL_RESPONSE=18）
- `OrderType`（5 值：LIMIT=1 ~ BEST_PRICE=5）
- `OrderDuration`（5 值：DAY=0 ~ FAK=4）
- `PriceType`（3 值：PERCENTAGE=1, PERUNIT=2, YIELD=9）
- `PositionDirection`（4 值：POS_OPEN=10 ~ POS_ERROR=13）
- `SubResponseType`（4 值）
- `OpenCloseType`（int8，4 值：OC_NULL_TYPE=0 ~ OC_CLOSE_TODAY=3）
- `TsExchangeID`（int8，7 值：TS_NULL_EXCHANGE=0 ~ TS_GFEX=6）
- `ExchangeType`（int32，13 值）
- `InstrumentType`（4 值：STK=0 ~ XXX=3）

#### Scenario: RequestType 枚举值验证
- **WHEN** 读取 `RequestType.NEWORDER`
- **THEN** 值 MUST 等于 0

#### Scenario: ResponseType 枚举值验证
- **WHEN** 读取 `ResponseType.TRADE_CONFIRM` 和 `ResponseType.ORDER_ERROR`
- **THEN** 值 MUST 分别等于 4 和 5

### Requirement: 关键常量定义

系统 SHALL 定义以下常量，值与 C++ 头文件完全一致：

- `ORDERID_RANGE = 1_000_000`
- `InterestLevels = 20`
- `MaxSymbolSize = 50`
- `MaxAccntIDLen = 10`（C++: `ACCNT_ID_LEN = 11` 含 null terminator）
- `MaxInstrNameSz = 32`
- `MaxTradeIDSize = 21`
- `MaxProductSize = 32`
- `MaxORSClients = 250`
- `REQ_QUEUE_ELEM_SIZE = 320`

迁移自: `hftbase/CommonUtils/include/orderresponse.h` 和 `hftbase/CommonUtils/include/marketupdateNew.h`

#### Scenario: ORDERID_RANGE 值验证
- **WHEN** 读取 `Constants.ORDERID_RANGE`
- **THEN** 值 MUST 等于 1000000

#### Scenario: REQ_QUEUE_ELEM_SIZE 值验证
- **WHEN** 读取 `Constants.REQ_QUEUE_ELEM_SIZE`
- **THEN** 值 MUST 等于 320

### Requirement: VarHandle 访问器

系统 SHALL 为每个结构体的每个字段提供 `VarHandle` 访问器，支持通过 `MemorySegment` + base offset 进行类型安全的读写。

#### Scenario: 读取 MarketUpdateNew 的 symbol 字段
- **WHEN** 从 SHM 中的 `MarketUpdateNew` 读取 symbol
- **THEN** 从 offset 40 读取 48 bytes 并转换为 String（去除 null 尾部）

#### Scenario: 写入 RequestMsg 的 price 字段
- **WHEN** 向 `RequestMsg` 的 price 字段写入 3500.0
- **THEN** 在 offset 136 处写入 8 bytes 的 IEEE 754 double
