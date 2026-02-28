## Tasks

- [x] 创建 data_new/live/controls/{day,night}/ 目录和 control 文件（model 路径 → ./live/models/）
- [x] 创建 data_new/sim/controls/{day,night}/ 目录和 control 文件（model 路径 → ./sim/models/）
- [x] 删除 data_new/common/controls/ 目录
- [x] 更新 data_new/live/models/ 风控参数（MAX_SIZE=100, UPNL_LOSS=60000 等）
- [x] 更新 build_deploy_java.sh 合并逻辑：按 live/sim 分别部署 controls/models/data
- [x] 更新内嵌 start_strategy.sh：按 gateway_mode 选择 ENV_DIR，移除 model 覆盖
- [x] 更新内嵌 start_gateway.sh：从 sim/controls/ 读取合约列表
- [x] 更新内嵌 start_all.sh：按环境目录扫描 controls
- [x] 更新清理逻辑和部署摘要
- [x] 重建 deploy_java 为 live/sim 分离结构
- [x] 验证策略启动：model 路径 ./live/models/model.* 正确加载，参数 BEGIN_PLACE=0.8 MAX_SIZE=100
