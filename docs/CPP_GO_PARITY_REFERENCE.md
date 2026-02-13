# C++/Go Parity Reference

This document maps the exact C++ naming conventions, config fields, strategy ID handling, and structural patterns to their Go equivalents. The Go implementation **must** match these exactly.

## 1. Class Hierarchy → Go Structs

| C++ Class | C++ File | Go Equivalent | Go File |
|-----------|----------|---------------|---------|
| `ExecutionStrategy` | `tbsrc/Strategies/include/ExecutionStrategy.h` | `BaseStrategy` | `golang/pkg/strategy/base_strategy.go` |
| `PairwiseArbStrategy` (extends `ExecutionStrategy`) | `tbsrc/Strategies/include/PairwiseArbStrategy.h` | `PairwiseArbStrategy` (embeds `BaseStrategy`) | `golang/pkg/strategy/pairwise_arb_strategy.go` |
| `ExtraStrategy` (per-leg execution) | `tbsrc/Strategies/include/ExtraStrategy.h` | Leg-specific logic within `PairwiseArbStrategy` | |
| `ThresholdSet` | `tbsrc/main/include/TradeBotUtils.h:237` | `ThresholdSet` | `golang/pkg/strategy/types.go` |
| `SimConfig` | `tbsrc/main/include/TradeBotUtils.h:707` | Config structs in `pkg/config/` | |
| `ControlConfig` | `tbsrc/main/include/TradeBotUtils.h:602` | Part of YAML config | |
| `ConfigParams` (singleton) | `tbsrc/main/include/TradeBotUtils.h:615` | Config structs in `pkg/config/` | |
| `OrderStats` | `tbsrc/Strategies/include/ExecutionStrategyStructs.h:44` | Order tracking structs | |
| `SpreadState` | `tbsrc/main/include/TradeBotUtils.h:749` | Spread state in strategy | |

## 2. Key Enums

### OrderStatus (`ExecutionStrategyStructs.h:20`)
```cpp
enum OrderStatus { NEW_ORDER, NEW_CONFIRM, NEW_REJECT, MODIFY_ORDER, MODIFY_CONFIRM,
                   MODIFY_REJECT, CANCEL_ORDER, CANCEL_CONFIRM, CANCEL_REJECT, TRADED, INIT };
```

### OrderHitType (`ExecutionStrategyStructs.h:35`)
```cpp
enum OrderHitType { STANDARD, IMPROVE, CROSS, DETECT, MATCH };
```
- `STANDARD` → passive/quote order
- `CROSS` → aggressive order (crosses spread)
- `IMPROVE` → price improvement order
- `MATCH` → matching/追单 order

### TransactionType (from hftbase)
```
BUY, SELL
```

### TypeOfOrder (from hftbase)
```
QUOTE, IOC, ...
```

## 3. ExecutionStrategy Member Variables

These `m_` prefixed names are the canonical names. Go fields should map to these semantically.

### Position Tracking
| C++ Name | Type | Go Name Must Reflect | Description |
|----------|------|---------------------|-------------|
| `m_netpos` | `int32_t` | Net position | `m_buyTotalQty - m_sellTotalQty` |
| `m_netpos_pass` | `int32_t` | Passive net position | Position from STANDARD orders |
| `m_netpos_pass_ytd` | `int32_t` | Yesterday's passive position | Carried over from previous day |
| `m_netpos_agg` | `int32_t` | Aggressive net position | Position from CROSS/MATCH orders |
| `m_buyTotalQty` | `double` | Total buy quantity | Cumulative |
| `m_sellTotalQty` | `double` | Total sell quantity | Cumulative |
| `m_buyQty` | `double` | Current round buy qty | Reset when `m_netpos == 0` |
| `m_sellQty` | `double` | Current round sell qty | Reset when `m_netpos == 0` |
| `m_buyOpenQty` | `double` | Open buy order qty | Pending fills |
| `m_sellOpenQty` | `double` | Open sell order qty | Pending fills |

### PNL Tracking
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_realisedPNL` | `double` | Realized PNL (reset when flat) |
| `m_unrealisedPNL` | `double` | Unrealized (mark-to-market) |
| `m_netPNL` | `double` | Net PNL (realized + unrealized - costs) |
| `m_grossPNL` | `double` | Gross PNL |
| `m_maxPNL` | `double` | High water mark |
| `m_drawdown` | `double` | Current drawdown from `m_maxPNL` |
| `m_transTotalValue` | `double` | Total transaction costs |
| `m_transValue` | `double` | Current round transaction costs |

### Price Tracking
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_buyAvgPrice` | `double` | Average buy price (total) |
| `m_sellAvgPrice` | `double` | Average sell price (total) |
| `m_buyPrice` | `double` | Current round avg buy price |
| `m_sellPrice` | `double` | Current round avg sell price |
| `m_buyTotalValue` | `double` | Cumulative buy value (price * qty) |
| `m_sellTotalValue` | `double` | Cumulative sell value |
| `m_targetPrice` | `double` | Target/theoretical price |
| `m_lastTradePx` | `double` | Last trade price |

### Threshold State
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_tholdBidPlace` | `double` | Current bid place threshold |
| `m_tholdBidRemove` | `double` | Current bid remove threshold |
| `m_tholdAskPlace` | `double` | Current ask place threshold |
| `m_tholdAskRemove` | `double` | Current ask remove threshold |
| `m_tholdMaxPos` | `int32_t` | Max position size |
| `m_tholdBeginPos` | `int32_t` | Begin position size |
| `m_tholdSize` | `int32_t` | Quote size per order |
| `m_tholdBidSize` | `int32_t` | Bid-specific quote size |
| `m_tholdAskSize` | `int32_t` | Ask-specific quote size |
| `m_tholdBidMaxPos` | `int32_t` | Max bid-side position |
| `m_tholdAskMaxPos` | `int32_t` | Max ask-side position |

### Order Counting
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_tradeCount` | `int32_t` | Total trade fills |
| `m_improveCount` | `int32_t` | IMPROVE type trade count |
| `m_crossCount` | `int32_t` | CROSS type trade count |
| `m_rejectCount` | `int32_t` | Order reject count |
| `m_orderCount` | `int32_t` | Total orders sent |
| `m_cancelCount` | `int32_t` | Cancel requests sent |
| `m_confirmCount` | `int32_t` | Order confirmations received |

### Control Flags
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_Active` | `bool` | Strategy is active |
| `m_onExit` | `bool` | Exit mode (cancel and close) |
| `m_onCancel` | `bool` | Cancel-only mode |
| `m_onFlat` | `bool` | Flatten mode (square off) |
| `m_onStopLoss` | `bool` | Stop loss triggered |
| `m_onTimeSqOff` | `bool` | Time-based square off |
| `m_aggFlat` | `bool` | Aggressive flatten |

### Identity
| C++ Name | Type | Description |
|----------|------|-------------|
| `m_strategyID` | `int32_t` | **Numeric** strategy ID |
| `m_product` | `char[32]` | Product name |
| `m_account` | `char[11]` | Trading account |

## 4. PairwiseArbStrategy Member Variables

| C++ Name | Type | Description |
|----------|------|-------------|
| `m_firstinstru` | `Instrument*` | Leg 1 instrument |
| `m_secondinstru` | `Instrument*` | Leg 2 instrument |
| `m_firstStrat` | `ExtraStrategy*` | Leg 1 execution |
| `m_secondStrat` | `ExtraStrategy*` | Leg 2 execution |
| `m_thold_first` | `ThresholdSet*` | Leg 1 thresholds |
| `m_thold_second` | `ThresholdSet*` | Leg 2 thresholds |
| `avgSpreadRatio` | `double` | Running average spread |
| `avgSpreadRatio_ori` | `double` | Original average spread |
| `currSpreadRatio` | `double` | Current spread value |
| `currSpreadRatio_prev` | `double` | Previous spread value |
| `expectedRatio` | `double` | Expected/target ratio |
| `m_netpos_agg1` | `int32_t` | Leg 1 aggressive net pos |
| `m_netpos_agg2` | `int32_t` | Leg 2 aggressive net pos |
| `m_agg_repeat` | `uint32_t` | Aggressive retry count (default 1) |
| `m_maxloss_limit` | `double` | Max loss limit |
| `m_ordMap1` / `m_ordMap2` | `OrderMap*` | Per-leg order maps |

### Key Methods
| C++ Method | Description |
|------------|-------------|
| `SendOrder()` | Main order placement (called on signal) |
| `SendAggressiveOrder()` | Aggressive/追单 order logic |
| `SetThresholds()` | Dynamic threshold calculation based on position |
| `MDCallBack(MarketUpdateNew*)` | Market data handler (entry point) |
| `ORSCallBack(ResponseMsg*)` | Order response handler |
| `HandlePassOrder()` | Handle passive order fill |
| `HandleAggOrder()` | Handle aggressive order fill |
| `CalcPendingNetposAgg()` | Calculate pending aggressive net position |
| `HandleSquareON()` / `HandleSquareoff()` | Square-off logic |

## 5. ThresholdSet — Config Parameter Names (Model File Keys)

These are the **exact string keys** used in model files, parsed by `ThresholdSet::AddThreshold()` in `TradeBotUtils.cpp:2661`. The Go config must use matching parameter names.

### Core Trading Parameters
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `SIZE` | `SIZE` | int32 | — | Base order size |
| `BEGIN_SIZE` | `BEGIN_SIZE` | int32 | =SIZE | Initial position size |
| `MAX_SIZE` | `MAX_SIZE` | int32 | — | Maximum position size |
| `BEGIN_PLACE` | `BEGIN_PLACE` | double | — | Entry threshold (empty position) |
| `BEGIN_REMOVE` | `BEGIN_REMOVE` | double | — | Exit threshold (empty position) |
| `LONG_PLACE` | `LONG_PLACE` | double | — | Entry threshold (long position) |
| `LONG_REMOVE` | `LONG_REMOVE` | double | — | Exit threshold (long position) |
| `SHORT_PLACE` | `SHORT_PLACE` | double | — | Entry threshold (short position) |
| `SHORT_REMOVE` | `SHORT_REMOVE` | double | — | Exit threshold (short position) |
| `LONG_INC` | `LONG_INC` | double | 0 | Long increment |

### Aggressive Order Parameters
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `SUPPORTING_ORDERS` | `SUPPORTING_ORDERS` | int32 | 0 | Max aggressive orders allowed |
| `MAX_ORDERS` | `MAX_ORDERS` | int32 | 0 | Max orders |
| `TAILING_ORDERS` | `TAILING_ORDERS` | int32 | 0 | Tailing order count |
| `CROSS` | `CROSS` | double | 1e9 | Aggressive cross threshold |
| `CLOSE_CROSS` | `CLOSE_CROSS` | double | 1e11 | Close position cross threshold |
| `MAX_CROSS` | `MAX_CROSS` | int32 | 1e9 | Max cross orders |
| `MAX_LONG_CROSS` | `MAX_LONG_CROSS` | int32 | 1e9 | Max long cross |
| `MAX_SHORT_CROSS` | `MAX_SHORT_CROSS` | int32 | 1e9 | Max short cross |
| `CROSS_TARGET` | `CROSS_TARGET` | int32 | 0 | Cross target |
| `CROSS_TICKS` | `CROSS_TICKS` | int32 | 0 | Cross tick offset |
| `AGG_COOL_OFF` | `AGG_COOL_OFF` | int64 | 0 | Cooloff between aggressive orders (ns) |
| `SLOP` | `SLOP` | int | 20 | Max slippage ticks for aggressive orders |

### Multi-Level Quoting
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `MAX_QUOTE_LEVEL` | `MAX_QUOTE_LEVEL` | int | 3 | Max quote levels |
| `MAX_OS_ORDER` | `MAX_OS_ORDER` | int32 | 5 | Max outstanding orders |
| `QUOTE_SKEW` | `QUOTE_SKEW` | double | 0 | Quote skew |
| `MAX_QUOTE_SPREAD` | `MAX_QUOTE_SPREAD` | int32 | 1e9 | Max quote spread |

### Risk Management
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `STOP_LOSS` | `STOP_LOSS` | double | 1e10 | Stop loss threshold |
| `MAX_LOSS` | `MAX_LOSS` | double | 1e11 | Maximum loss |
| `UPNL_LOSS` | `UPNL_LOSS` | double | 1e10 | Unrealized PNL loss limit |
| `PT_PROFIT` | `PT_PROFIT` | double | 1e6 | Profit target |
| `PT_LOSS` | `PT_LOSS` | double | 1e6 | Loss target |
| `MAX_PRICE` | `MAX_PRICE` | double | 1e12 | Max price limit |
| `MIN_PRICE` | `MIN_PRICE` | double | -1000 | Min price limit |

### Spread/Deviation Parameters
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `AVG_SPREAD_AWAY` | `AVG_SPREAD_AWAY` | int | 20 | Avg spread deviation ticks |
| `PLACE_SPREAD` | `PLACE_SPREAD` | double | 0 | Placement spread |
| `CLOSE_SPREAD` | `CLOSE_SPREAD` | double | — | Close spread |
| `SPREAD_EWA` | `SPREAD_EWA` | double | 0.6 | Spread EWA factor |

### Statistical Parameters
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `DECAY` | `DECAY` | double | — | EWA decay factor |
| `DECAY1` | `DECAY1` | double | — | Decay factor 1 |
| `DECAY2` | `DECAY2` | double | — | Decay factor 2 |
| `LOOKBACK_TIME` | `LOOKBACK_TIME` | long | — | Lookback window |
| `HISTORICAL_STDDEV` | `HISTORICAL_STDDEV` | double | — | Historical std dev |
| `WINDOW_DURATION` | `WINDOW_DURATION` | long | — | Window duration |
| `ALPHA` | `ALPHA` | double | — | Alpha coefficient |
| `PRICE_RATIO` | `PRICE_RATIO` | double | — | Price ratio |
| `HEDGE_RATIO` | `HEDGE_RATIO` | double | — | Hedge ratio |
| `HEDGE_THRES` | `HEDGE_THRES` | double | — | Hedge threshold |
| `HEDGE_SIZE_RATIO` | `HEDGE_SIZE_RATIO` | double | — | Hedge size ratio |

### Directional Size Controls
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `BID_SIZE` | `BID_SIZE` | int32 | 0 (falls back to SIZE) | Bid-specific order size |
| `BID_MAX_SIZE` | `BID_MAX_SIZE` | int32 | 0 (falls back to MAX_SIZE) | Bid-side max position |
| `ASK_SIZE` | `ASK_SIZE` | int32 | 0 (falls back to SIZE) | Ask-specific order size |
| `ASK_MAX_SIZE` | `ASK_MAX_SIZE` | int32 | 0 (falls back to MAX_SIZE) | Ask-side max position |

### Time Controls
| Model File Key | C++ Field | Type | Default | Description |
|---------------|-----------|------|---------|-------------|
| `PAUSE` | `PAUSE` | int64 | 0 | Pause between orders (μs, stored as ns) |
| `CANCELREQ_PAUSE` | `CANCELREQ_PAUSE` | int64 | 0 | Cancel request pause |
| `SQROFF_TIME` | `SQROFF_TIME` | int64 | 0 | Square off time (s, stored as ns) |
| `SQROFF_AGG` | `SQROFF_AGG` | int | 0 | Aggressive square off |
| `SQUARE_OFF_TIME` | `SQUARE_OFF_TIME` | long | — | Square off time |
| `SKIP_TIME` | `SKIP_TIME` | long | — | Skip time |

## 6. Strategy ID Handling

- **C++ format**: `m_strategyID` is `int32_t` — a pure numeric value (e.g., `92201`, `92202`)
- **Assignment**: Passed via command line `--strategyID 92201`, stored in `ConfigParams::m_strategyID`
- **Control file format**: `<baseName> <modelFile> <exchange> <id> <execStrat> <startTime> <endTime> [<secondName>]`
- **Go config**: Strategy IDs like `test_92201` include a prefix; the numeric portion (`92201`) corresponds to the C++ strategy ID
- **Order ID → Strategy mapping**: `OrderIDStrategyMap` maps `uint32_t` order IDs to `ExecutionStrategy*`

### Strategy ID Numbering Convention

IDs follow a product-based scheme: first digit(s) = commodity, last digits = instance.

| ID Range | Product | Example |
|----------|---------|---------|
| `92xxx` | ag (silver) | `92201` = ag pair instance 201 |
| `93xxx` | al (aluminum) | `93201` = al pair instance 201 |
| `41xxx` | rb (rebar) | `41231` = rb pair |
| `9999` | Special/testing | |

### Strategy Type Constants (Factory Strings)

These exact strings appear in control files and are matched in `main.cpp` for instantiation:

| Config String | C++ Class |
|---------------|-----------|
| `TB_PAIR_STRAT` | `PairwiseArbStrategy` |
| `TB_PAIR_ETF_STRAT` | `PairwiseArbETFStrategy` |
| `TB_PAIR_OPT_STRAT` | `PairwiseArbStrategyOpt` |
| `TB_PASSIVE_STRAT` | `PassiveStrategy` |
| `TB_AGGRESSIVE_STRAT` | `AggressiveStrategy` |
| `TB_HEDGING_STRAT` | `HedgingStrategy` |
| `TB_SIMPLEHEDGING_STRAT` | `SimpleHedgingStrategy` |
| `TB_HIT_STRAT` | `HitStrategy` |
| `TB_SWEEP_STRAT` | `SweepStrategy` |
| `TB_TARGET_STRAT` | `TargetStrategy` |
| `VWAP_STRAT` | `VWAPStrategy` |
| `BUTTERFLY_STRAT` | `ButterflyStrategy` |
| `TB_SUPPTAIL_STRAT` | `SuppTailStrategy` |
| `DUMMY_STRAT` | `DummyStrategy` |

### Strategy ID Flow

```
Command line: --strategyID 92201
  → main.cpp: configParams->m_strategyID = strategyID
    → ExecutionStrategy constructor: m_strategyID = configParams->m_strategyID
      → CommonClient: m_reqMsg.StrategyID = execStrategy->m_strategyID
        → RequestMsg.StrategyID (sent to ORS via SHM)
          → ORS echoes in ResponseMsg.StrategyID
```

## 7. Config File Formats

### a) System Config (.cfg) — INI-style key-value

Parsed by `ConfigfileTB` / `illuminati::Configfile::LoadCfg()`.

```ini
INTERACTION_MODE = LIVE
EXCHANGES = CHINA_SHFE
PRODUCT = hl5
MONITORSHMKEY = 789

[CHINA_SHFE]
MDSHMKEY           = 872
ORSREQUESTSHMKEY   = 3872
ORSRESPONSESHMKEY  = 4872
MDSHMSIZE          = 100000
```

File naming: `config_CHINA.<strategyID>.cfg`

### b) Control File — Space-delimited single line

Parsed by `LoadControlFile()`. Single line, 7-9 tokens:
```
ag_F_2_SFE ../models/model.ag2502.ag2504.par.txt FUTCOM 0 TB_PAIR_STRAT 0900 1500 ag_F_4_SFE
```

Maps to `ControlConfig`: `m_baseName`, `m_modelFile`, `m_exchange`, `m_id`, `m_execStrat`, `m_startTime`, `m_endTime`, `[m_secondName]`, `[m_thirdName]`

File naming: `control.<pair>.par.txt.<strategyID>`

### c) Model/Parameter File — Key-value with instrument header

Parsed by `LoadModelFile()` → `ThresholdSet::AddThreshold(name, value)`.

First lines define instruments, subsequent lines define thresholds:
```
ag_F_2_SFE FUTCOM Dependant 0 MID_PX
ag_F_4_SFE FUTCOM Dependant 0 MID_PX
MAX_QUOTE_LEVEL 3
SIZE 1
MAX_SIZE 3
BEGIN_PLACE 6
LONG_PLACE 10
SHORT_PLACE 1.5
BEGIN_REMOVE 4.5
LONG_REMOVE 8.5
SHORT_REMOVE 0
CROSS 3
SUPPORTING_ORDERS 2
SLOP 20
STOP_LOSS 100000
MAX_LOSS 100000
ALPHA 0.00002407
PRICE_RATIO 1
HEDGE_THRES 1
```

File naming: `model.<pair>.par.txt`

### d) Daily Init File — Position carry-over

Per-strategy file with yesterday's positions, loaded in `PairwiseArbStrategy.cpp`:
```cpp
auto &row = mx_daily_init2.at(m_strategyID);
// Columns: "avgPx", "ytd1", "2day", "ytd2"
```

File naming: `daily_init.<strategyID>`

## 8. Position Management (SHFE-specific)

### Yesterday vs Today Position
The C++ code tracks `m_netpos_pass_ytd` (yesterday's carried position) separately from today's trading. This is critical for SHFE's CLOSE_TODAY vs CLOSE_YESTERDAY distinction.

The `m_netpos_pass_ytd` is loaded from daily initialization data:
```cpp
// PairwiseArbStrategy.cpp / PairwiseArbOptStrategy.cpp
auto &row = mx_daily_init2.at(m_strategyID);
int netpos_ytd1 = std::stoi(row["ytd1"]);
m_firstStrat->m_netpos_pass_ytd = netpos_ytd1;
m_firstStrat->m_netpos = netpos_ytd1 + netpos_2day1;
m_firstStrat->m_netpos_pass = netpos_ytd1 + netpos_2day1;
```

### Position Netting by Order Type
In `ProcessTrade()`:
```cpp
if (ordType == CROSS)      → m_netpos_agg ± qty
if (ordType == STANDARD)   → m_netpos_pass ± qty
if (ordType == MATCH)      → m_netpos_agg ± qty
m_netpos = m_buyTotalQty - m_sellTotalQty  // always recalculated
```

### Round Reset
When `m_netpos == 0`, current round values reset:
```cpp
m_buyValue = 0; m_buyQty = 0; m_buyPrice = 0;
m_sellValue = 0; m_sellQty = 0; m_sellPrice = 0;
m_transValue = 0;
```

## 9. Event Flow

```
MDCallBack(MarketUpdateNew*)
  → Update spread: currSpreadRatio = f(leg1, leg2)
  → Update avgSpreadRatio (EWA or lookback)
  → SetThresholds() — adjust thresholds based on current m_netpos
  → SendOrder() — evaluate entry/exit signals against thresholds
    → If signal: SendNewOrder() per leg via ExtraStrategy
  → SendAggressiveOrder() — chase unfilled orders
    → Check SUPPORTING_ORDERS limit
    → Apply SLOP ticks for price adjustment
```

```
ORSCallBack(ResponseMsg*)
  → Find order in m_ordMap via OrderID
  → Route to: ProcessTrade / ProcessNewReject / ProcessModifyConfirm / ProcessCancelConfirm / etc.
  → ProcessTrade:
    → Update m_netpos, m_buyTotalQty, m_sellTotalQty
    → Track by OrderHitType (STANDARD→m_netpos_pass, CROSS→m_netpos_agg)
    → CalculatePNL()
    → If m_netpos == 0: reset round values
```

## 10. SpreadState Structure

Used for spread tracking between legs (`TradeBotUtils.h:749`):

| C++ Field | Type | Description |
|-----------|------|-------------|
| `spread` | `double` | Raw spread value |
| `normSpread` | `double` | Normalized spread (z-score equivalent) |
| `localMean` | `double` | Rolling mean |
| `localStd` | `double` | Rolling standard deviation |
| `depPrice` / `indepPrice` | `double` | Dependent/independent leg prices |
| `depRet` / `indepRet` | `double` | Returns |
| `historicIndPrice` / `historicDepPrice` | `double` | Historical reference prices |
| `retSpread` | `double` | Return-based spread |
| `indepStatus` / `depStatus` | `bool` | Data validity flags |
| `spreadStatus` | `bool` | Spread calculation validity |

## 11. OrderStats Structure

Each order tracked in `m_ordMap`:

| C++ Field | Type | Description |
|-----------|------|-------------|
| `m_orderID` | `uint32_t` | Order ID |
| `m_side` | `TransactionType` | BUY or SELL |
| `m_price` | `double` | Current price |
| `m_newprice` | `double` | Modified price |
| `m_oldprice` | `double` | Previous price |
| `m_Qty` | `int32_t` | Original quantity |
| `m_openQty` | `int32_t` | Remaining open quantity |
| `m_doneQty` | `int32_t` | Filled quantity |
| `m_cxlQty` | `int32_t` | Cancelled quantity |
| `m_status` | `OrderStatus` | Current order status |
| `m_ordType` | `OrderHitType` | STANDARD/IMPROVE/CROSS/MATCH |
| `m_typeOfOrder` | `TypeOfOrder` | QUOTE/IOC |
| `m_active` | `bool` | Order is active |
| `m_cancel` | `bool` | Cancel requested |
| `m_modifywait` | `bool` | Modify pending |
| `m_quantAhead` | `double` | Queue quantity ahead |
| `m_quantBehind` | `double` | Queue quantity behind |

## 12. C++ Naming Convention Summary

| Element | Convention | Examples |
|---------|-----------|----------|
| Classes | PascalCase | `ExecutionStrategy`, `PairwiseArbStrategy` |
| Member variables | `m_` prefix + camelCase | `m_netpos`, `m_strategyID`, `m_buyTotalQty` |
| Methods (public) | PascalCase | `SendOrder()`, `MDCallBack()`, `CalculatePNL()` |
| ThresholdSet fields | ALL_CAPS, no prefix | `BEGIN_PLACE`, `MAX_QUOTE_LEVEL`, `SUPPORTING_ORDERS` |
| Enums | PascalCase name, ALL_CAPS members | `enum OrderHitType { STANDARD, IMPROVE, CROSS }` |
| Macros | ALL_CAPS | `TBLOG`, `BUFFER_LEN` |
| Namespaces | lowercase | `illuminati::md`, `illuminati::infra` |
| Typedefs | PascalCase | `OrderMap`, `PriceMap`, `InstruMapIter` |
| Indicator classes | PascalCase or abbreviation | `AvgSpread`, `BookDelta`, `BD`, `TVBM` |

**Critical rule**: When migrating to Go, use the C++ original names as the semantic basis. Do NOT invent new names (e.g., do not rename `ExtraStrategy` to `LegStrategy`).

## 13. Deployment Model (C++)

- **One process per strategy pair** — not multi-strategy per process
- **Cron-based**: night session starts 20:53, day session 08:53 (via `cronjob/`)
- **Runtime control**: `SIGUSR1` signal triggers start-trading
- **Lock files**: `/home/TradeBot/locks/lock.<strategyID>` prevents duplicate instances
- **Model setup**: `setup.arbi.py` runs at 20:45 to prepare model files
- **Log files**: `log.control.<pair>.<strategyID>.<date>`

## 14. Go Migration Patterns

| C++ Pattern | Go Equivalent |
|-------------|---------------|
| Virtual methods / inheritance | Interface + struct embedding |
| `ExecutionStrategy` (abstract base) | `Strategy` interface |
| `new PassiveStrategy(...)` (factory via string) | `map[string]func(...) Strategy` registry |
| `ConfigParams::GetInstance()` (singleton) | Package-level var + `sync.Once`, or DI |
| `std::thread` + spin-wait | Goroutines + channels |
| SHM IPC (`ShmManager`) | NATS / gRPC (already done in new system) |
| `CasLock` / `std::atomic` | `sync.Mutex` / `atomic` package |
| `TBLOG << ... << endl` (stream logging) | Structured logging (`slog` / `zerolog`) |
| `exit(1)` on config error | Return `error`, `log.Fatal` only at top level |
| `Callback<Client, MemFunc>` (templates) | Interface method calls or `func` values |
| `typedef OrderMap` | `type OrderMap map[uint32]*OrderStats` |
