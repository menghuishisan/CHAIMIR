// sim 模块(M4,第1层 引擎)装配。
// 职责:仿真包管理/审核/版本 + 会话/操作/分享/检查点持久化 + 少数后端计算仿真。
package main

import (
	"log/slog"

	"chaimir/internal/modules/sim"
	"chaimir/internal/platform/ws"
)

// assembleSim 装配 sim 模块(M4)并注册 HTTP/WS 路由与跨模块契约实现。
func assembleSim(d *moduleDeps) error {
	backendRegistry := sim.NewBackendAdapterRegistry()
	svc := sim.NewService(
		d.infra.db,
		d.infra.idgen,
		d.infra.store,
		d.infra.bus,
		d.infra.audit,
		d.infra.identity,
		backendRegistry,
		ws.NewOriginPolicy(d.cfg.Server.WSAllowedOrigins),
	)
	api := sim.NewAPI(svc, d.infra.auth, d.infra.identity, d.cfg.Upload)
	api.Register(d.infra.server.apiV1())
	d.infra.sim = svc
	slog.Info("模块装配", slog.String("module", "sim"), slog.String("layer", "1-engine"))
	return nil
}
