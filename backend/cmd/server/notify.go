// notify 模块(M10,第3层 聚合/横切)装配。
// 职责:通知服务/WS Hub/事件消费。提供 contracts.NotifyService 给全平台。
package main

import (
	"log/slog"

	"chaimir/internal/modules/notify"
)

// assembleNotify 装配 notify 模块并注册 HTTP/WS 路由与事件订阅。
func assembleNotify(d *moduleDeps) error {
	svc := notify.NewService(d.infra.db, d.infra.idgen, d.infra.redis, d.infra.hub, d.cfg.Notify)
	api := notify.NewAPI(svc, d.infra.auth, d.infra.identity, d.infra.hub)
	api.Register(d.infra.server.apiV1())
	if err := svc.SubscribeEvents(d.infra.bus); err != nil {
		return err
	}
	d.infra.notify = svc
	slog.Info("模块装配", slog.String("module", "notify"), slog.String("layer", "3-aggregation"))
	return nil
}
