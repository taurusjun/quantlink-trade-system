## 1. 修复守卫条件

- [x] 1.1 将 PairwiseArbStrategy.java L633-634 的 `secondStrat.instru.bidPx[0] <= 0 && secondStrat.instru.askPx[0] <= 0` 改为 `secondStrat.instru.bidPx[0] <= 0 || secondStrat.instru.askPx[0] <= 0`，确保任一腿的 bid 或 ask 为 0 时都跳过 spread 计算
- [x] 1.2 更新守卫条件上方的注释，加 `[C++差异]` 标注说明与 C++ 原代码的区别及修改原因
