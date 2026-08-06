// 本文件是仿真 SDK 主入口,集中导出协议类型、校验、确定性工具与浏览器侧运行时客户端。
//
// 刻意不导出两类模块:
//   registry/builtinRegistry —— 它 import 了全部内置包内核,从主入口导出会把 builtin/
//     整棵树拉进消费者主线程包(内置包只在 Worker 内装配)。
//   runtime/container* —— 容器执行宿主 import 了 node:crypto/fs/zlib,
//     被浏览器包静态引用会污染前端产物;它由 images/sim/package-runner 单独构建入口。

export * from './types';
export * from './validation';
export * from './registry/builtinCode';
export * from './runtime/deterministic';
export * from './runtime/SimWorkerClient';
export * from './authoring/manifest';
export * from './authoring/templates';
