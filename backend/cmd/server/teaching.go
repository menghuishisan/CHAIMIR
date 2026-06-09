// teaching 模块(M6,第2层 业务)装配。
// 职责:课程/课时/作业/进度/互动/单课程成绩。判题调 M3,内容引用调 M5,订阅 M3 判题事件。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/teaching"
)

// assembleTeaching 装配 teaching 模块并注册 HTTP 路由、事件订阅与跨模块契约实现。
func assembleTeaching(d *moduleDeps) error {
	if d.infra.identity == nil || d.infra.audit == nil {
		return fmt.Errorf("装配 teaching 失败: identity/audit contract 不可用")
	}
	if d.infra.content == nil {
		return fmt.Errorf("装配 teaching 失败: content contract 不可用,无法锁定教学题目版本")
	}
	if d.infra.judge == nil {
		return fmt.Errorf("装配 teaching 失败: judge contract 不可用,无法提交自动判题任务")
	}
	svc := teaching.NewService(d.infra.db, d.infra.idgen, d.infra.audit, d.infra.identity, d.infra.content, d.infra.judge, d.infra.bus, d.cfg.Teaching)
	api := teaching.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	if err := svc.SubscribeEvents(); err != nil {
		return fmt.Errorf("装配 teaching 失败: 订阅判题事件失败: %w", err)
	}
	go svc.StartJudgeOutboxWorker(d.ctx)
	d.infra.teaching = svc
	slog.Info("模块装配", slog.String("module", "teaching"), slog.String("layer", "2-business"))
	return nil
}
