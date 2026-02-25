## ADDED Requirements

### Requirement: data_new 三层目录结构

`data_new/` 目录 SHALL 组织为 `common/`、`sim/`、`live/` 三个顶层子目录。

- `common/` 包含两种模式共享的文件：`config/`、`controls/`、`models/`
- `sim/` 包含模拟盘专用文件：`config/simulator.yaml`、`data/daily_init.*`
- `live/` 包含实盘专用文件：`config/ctp/`、`data/daily_init.*`、`ctp_flow/`

#### Scenario: 目录结构完整性

- **WHEN** 检查 `data_new/` 目录
- **THEN** 存在 `common/config/`、`common/controls/`、`common/models/`
- **AND** 存在 `sim/config/`、`sim/data/`
- **AND** 存在 `live/config/ctp/`、`live/data/`、`live/ctp_flow/`
- **AND** `data_new/` 根目录下不存在旧扁平目录

### Requirement: build_deploy_new.sh --mode 参数

`build_deploy_new.sh` SHALL 支持 `--mode sim|live` 参数（默认 `sim`）。

#### Scenario: config 按模式部署

- **WHEN** 执行 `build_deploy_new.sh --mode sim`
- **THEN** `deploy_new/config/` 包含 common + sim 的配置
- **AND** 不包含 `ctp/` 子目录

#### Scenario: config 按模式部署 (live)

- **WHEN** 执行 `build_deploy_new.sh --mode live`
- **THEN** `deploy_new/config/` 包含 common + live 的配置
- **AND** 包含 `ctp/` 子目录

### Requirement: deploy_new data 按模式分目录

`deploy_new/data/` SHALL 包含 `sim/` 和 `live/` 子目录，分别存储各模式的运行时状态。

#### Scenario: 两个模式的 daily_init 同时存在

- **WHEN** 执行 `build_deploy_new.sh`（任意 mode）
- **THEN** `deploy_new/data/sim/daily_init.*` 来自 `data_new/sim/data/`
- **AND** `deploy_new/data/live/daily_init.*` 来自 `data_new/live/data/`
- **AND** 两者互不影响

#### Scenario: 已有运行时数据不被覆盖

- **WHEN** `deploy_new/data/sim/daily_init.92201` 已存在
- **AND** 重新执行 `build_deploy_new.sh`
- **THEN** 该文件保持原值不变

### Requirement: start_gateway.sh 记录运行模式

`start_gateway.sh` SHALL 在启动时写入 `.gateway_mode` 文件，内容为当前模式（`sim` 或 `ctp`）。

#### Scenario: sim 模式写入 .gateway_mode

- **WHEN** 执行 `start_gateway.sh sim`
- **THEN** `deploy_new/.gateway_mode` 内容为 `sim`

#### Scenario: ctp 模式写入 .gateway_mode

- **WHEN** 执行 `start_gateway.sh ctp`
- **THEN** `deploy_new/.gateway_mode` 内容为 `ctp`

### Requirement: start_strategy.sh 传递 -dataDir

`start_strategy.sh` SHALL 读取 `.gateway_mode` 文件，根据模式传递 `-dataDir` 参数给 trader。

#### Scenario: sim 模式传递 data/sim

- **WHEN** `.gateway_mode` 内容为 `sim`
- **THEN** trader 启动命令包含 `-dataDir ./data/sim`

#### Scenario: ctp 模式传递 data/live

- **WHEN** `.gateway_mode` 内容为 `ctp`
- **THEN** trader 启动命令包含 `-dataDir ./data/live`

#### Scenario: .gateway_mode 不存在时报错

- **WHEN** `.gateway_mode` 文件不存在
- **THEN** 输出错误提示「请先启动网关」并退出

### Requirement: trader -dataDir flag

Go trader SHALL 支持 `-dataDir` 命令行参数，控制 daily_init 的读写路径。

#### Scenario: 使用 -dataDir 加载 daily_init

- **WHEN** trader 启动参数包含 `-dataDir ./data/sim`
- **THEN** daily_init 从 `./data/sim/daily_init.{strategyID}` 加载

#### Scenario: 使用 -dataDir 保存 daily_init

- **WHEN** trader 运行时执行 squareoff 保存
- **THEN** daily_init 保存到 `-dataDir` 指定的目录

#### Scenario: 默认值向后兼容

- **WHEN** trader 启动参数不包含 `-dataDir`
- **THEN** 默认使用 `./data` 目录（向后兼容）
