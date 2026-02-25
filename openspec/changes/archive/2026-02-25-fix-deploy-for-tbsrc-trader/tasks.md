# Tasks: 适配 deploy_new 到 tbsrc-golang trader

## 配置文件

- [x] 创建 `data_new/config/trader.92201.yaml`（ag2603/ag2605，从 model 文件提取参数）
- [x] 创建 `data_new/config/trader.92202.yaml`（au2604/au2606，从 model 文件提取参数）

## 启动脚本

- [x] 更新 `build_deploy_new.sh` 中 `start_strategy.sh` 模板：使用 `-config config/trader.{id}.yaml -data ./data`
- [x] 更新 `build_deploy_new.sh` 中 `start_all.sh` 模板：遍历 `config/trader.*.yaml` 而非 `controls/` 目录
- [x] 更新 `start_gateway.sh` 模板：使用 `md_shm_feeder` 替代 `ctp_md_gateway + md_gateway` (NATS → SysV SHM 直连)
- [x] 添加 `md_shm_feeder` 到 C++ 编译和部署列表

- [x] 移除 `ors_gateway`（tbsrc-golang 使用 SysV MWMR 直连，不需要 POSIX SHM + gRPC 中间层）
- [x] 适配 macOS SHM 限制（`kern.sysv.shmall=1024`）：MD queue=2048, ORS req/resp=1024

## 验证

- [x] 重新运行 `build_deploy_new.sh` 生成新脚本和二进制
- [x] 在 deploy_new 中启动网关 + 策略 92201 验证 trader 正常启动
