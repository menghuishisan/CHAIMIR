// Package sim 实现 M4 仿真(第1层 引擎,仅后端部分)。
// 职责:仿真包管理/审核/版本 + 少数 compute=backend 重计算仿真。
// 渲染 SDK/前端确定性运行时在 frontend/packages/sim-sdk,不在此。
package sim
