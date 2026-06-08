// Package notify 实现 M10 通知与实时推送(第3层 聚合/横切)。
// 职责:通知服务/WS Hub/事件消费;提供 contracts.NotifyService 给全平台。
// 边界:不存业务数据;全平台唯一的通知与 WS 广播入口。
package notify
