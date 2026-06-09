// sandbox 模块(M2,第1层 引擎)装配。
// 职责:沙箱编排/运行时适配/工具接入/K8s 调度。依赖 platform/k8s。
// 本文件注入 K8s 客户端与对象存储等基础设施,注册 /sandbox 路由与跨模块契约实现。
package main

import (
	"fmt"
	"log/slog"
	"time"

	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/ws"
)

// assembleSandbox 装配 sandbox 模块(M2)并注册 HTTP 路由与跨模块契约实现。
func assembleSandbox(d *moduleDeps) error {
	if d.infra.k8s == nil {
		return fmt.Errorf("装配 sandbox 失败: K8s 客户端不可用,无法提供生产沙箱编排能力")
	}
	orch := sandbox.NewK8sOrchestrator(d.infra.k8s, d.cfg.Sandbox)
	capabilities := sandbox.NewStaticCapabilityRegistry(map[string]sandbox.ChainCapability{
		"evm-jsonrpc": sandbox.NewEVMCapability(orch, time.Duration(d.cfg.Sandbox.ChainRPCTimeoutSeconds)*time.Second),
	})
	svc := sandbox.NewService(
		d.infra.db,
		d.infra.idgen,
		orch,
		capabilities,
		d.infra.bus,
		d.infra.store,
		d.infra.hub,
		d.cfg.Sandbox,
		ws.NewOriginPolicy(d.cfg.Server.WSAllowedOrigins),
		d.infra.audit,
		d.infra.identity,
	)
	api := sandbox.NewAPI(svc, d.infra.auth)
	api.Register(d.infra.server.apiV1())
	d.infra.sandbox = svc
	go svc.StartRecycleScheduler(d.ctx)
	slog.Info("模块装配", slog.String("module", "sandbox"), slog.String("layer", "1-engine"))
	return nil
}
