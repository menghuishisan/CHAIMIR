// Package sandbox 实现 M2 沙箱引擎(第1层 引擎)。
// 职责:沙箱编排/运行时适配/工具接入/K8s 调度;每沙箱 = 动态命名空间 + Pod 组。
// 边界:仅依赖第0层;反向通知高层走 sandbox.recycled 事件,不 import 业务模块。
package sandbox
