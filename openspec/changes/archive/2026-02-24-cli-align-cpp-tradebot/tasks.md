## Tasks

### Task 1: 新增 C++ 配置文件解析器
**Files**: `pkg/config/control_file.go`, `pkg/config/cfg_file.go`, `pkg/config/model_file.go`, `pkg/config/build_config.go`
- [x] ParseControlFile: 解析 controlFile 单行格式
- [x] ParseCfgFile: 解析 .cfg INI 格式（支持 [SECTION]）
- [x] ParseModelFile: 解析 model file（阈值 + indicator）
- [x] BuildConfig: 组合三者 + CLI 参数构建 config.Config
- [x] baseName 映射: ag_F_3_SFE → ag2603

### Task 2: 重写 main.go CLI 参数
**Files**: `cmd/trader/main.go`
- [x] 替换 `-config`/`-data` 为 `--controlFile`/`--configFile`/`--strategyID` 等
- [x] daily_init 路径硬编码为 `../data/daily_init.<strategyID>`
- [x] 阈值热加载改为重新读取 modelFile
- [x] 保持其余逻辑（Connector、策略创建等）不变

### Task 3: 创建 C++ 格式配置数据文件
**Files**: `data_new/config/config_CHINA.92201.cfg`, `data_new/data/daily_init.92201`
- [x] 创建 .cfg 文件（新系统 SHM key: 0x1001/0x2001/0x3001/0x4001）
- [x] 创建 daily_init.92201/92202（当前持仓数据）

### Task 4: 更新构建和启动脚本
**Files**: `scripts/build_deploy_new.sh`
- [x] 更新 start_strategy.sh: --controlFile/--configFile/--strategyID + session (day/night)
- [x] 更新 start_gateway.sh: 从 control 文件提取合约列表（替代 YAML）
- [x] 更新 start_all.sh: 从 config_CHINA.*.cfg 发现策略（替代 trader.*.yaml）
- [x] 确保 deploy_new/ 目录结构包含 controls/models/data

### Task 5: 单元测试
**Files**: `pkg/config/cpp_parsers_test.go`
- [x] 测试各解析器正确性（17 个测试用例）
- [x] 测试 BuildConfig 辅助函数
- [x] 测试 baseName 映射

### Task 6: 编译部署 + 端到端验证
- [x] `go test ./pkg/...` 全部通过
- [x] `build_deploy_new.sh --go` 编译成功
- [x] 模拟器模式端到端测试通过（策略正常接收行情、计算价差、graceful shutdown）
