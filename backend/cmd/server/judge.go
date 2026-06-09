// judge 模块(M3,第1层 引擎)装配。
// 职责:判题器/判题调度/查重。可依赖 sandbox、content;判完发 judge.completed 事件。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/judge"
)

// assembleJudge 装配 judge 模块。
func assembleJudge(d *moduleDeps) error {
	if d.infra.sandbox == nil {
		return fmt.Errorf("装配 judge 失败: sandbox contract 不可用,无法提供生产判题沙箱能力")
	}
	if d.infra.contentJudge == nil {
		return fmt.Errorf("装配 judge 失败: content contract 不可用,无法读取判题配置")
	}
	svc := judge.NewService(
		d.infra.db,
		d.infra.idgen,
		d.infra.redis,
		d.infra.bus,
		d.infra.hub,
		d.infra.store,
		d.cfg.Judge,
		d.infra.sandbox,
		d.infra.contentJudge,
		d.infra.audit,
		d.infra.identity,
	)
	api := judge.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	d.infra.judge = svc
	go svc.StartWorker(d.ctx)
	slog.Info("模块装配", slog.String("module", "judge"), slog.String("layer", "1-engine"))
	return nil
}
