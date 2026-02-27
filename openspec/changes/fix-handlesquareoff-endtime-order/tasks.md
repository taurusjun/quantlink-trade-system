# Tasks

- [x] Fix 1: ExecutionStrategy.checkSquareoff() — useArbStrat=true 时跳过 handleSquareoff
- [x] Fix 2: ExecutionStrategy.handleSquareoff() — active=false 时拒绝发送平仓订单
- [x] Fix 3: PairwiseArbStrategy.mdCallBack() — active=false 时跳过 endTime 检查
- [x] Fix 4: PairwiseArbStrategy.mdCallBack() — 从行情读取 exchTS（C++ 用 Watch 全局时钟）
- [x] 测试: ExecutionStrategyTest — 子 strat checkSquareoff 不调用 handleSquareoff (useArbStrat=true)
- [x] 测试: ExecutionStrategyTest — 对比 useArbStrat=false 正常调用 handleSquareoff
- [x] 测试: ExecutionStrategyTest — handleSquareoff active=false 不发单（多仓+空仓）
- [x] 测试: ExecutionStrategyTest — handleSquareoff active=true 正常发单
- [x] 测试: ExecutionStrategyTest — CTP模式 active=false checkSquareoff 不发单
- [x] 测试: PairwiseArbStrategyTest — active=false 时 endTime 不触发
- [x] 测试: PairwiseArbStrategyTest — active=true 时 endTime 正常触发
- [x] 测试: PairwiseArbStrategyTest — 综合事故重现（昨仓82/-83+endTime过+active=false=0笔订单）
- [x] 测试: PairwiseArbStrategyTest — 记录 handleSquareoff 使用 POS_OPEN flag 的行为
