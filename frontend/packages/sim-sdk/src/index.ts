// 本文件是仿真 SDK 主入口,集中导出类型、校验与 Worker 运行时。
// 刻意不导出 registry/builtinRegistry:它 import 了全部内置包内核,
// 从主入口导出会把 builtin/ 整棵树拉进消费者主线程包(内置包只在 Worker 内解析)。

export * from './types';
export * from './validation';
export * from './registry/builtinCode';
export * from './runtime/deterministic';
export * from './runtime/SimWorkerClient';
export * from './authoring/manifest';
export * from './authoring/templates';
