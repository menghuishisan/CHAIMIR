// Package judge 实现 M3 评测引擎(第1层 引擎)。
// 职责:判题器/判题调度/查重;终态事件先写 outbox,再经 eventbus 供高层订阅。
// 边界:可依赖 sandbox/content;不反向 import 业务模块。
package judge
