// experiment 模块(M7,第2层 业务)装配。
// 职责:实验编排(组装 M2/M4 引擎)。订阅 sandbox.recycled 更新实例状态。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/experiment"
)

// assembleExperiment 装配 experiment 模块并注册 HTTP 路由、事件订阅与跨模块契约实现。
func assembleExperiment(d *moduleDeps) error {
	if d.infra.identity == nil || d.infra.audit == nil {
		return fmt.Errorf("装配 experiment 失败: identity/audit contract 不可用")
	}
	if d.infra.content == nil {
		return fmt.Errorf("装配 experiment 失败: content contract 不可用,无法校验检查点题目版本")
	}
	if d.infra.sandbox == nil {
		return fmt.Errorf("装配 experiment 失败: sandbox contract 不可用,无法创建实验环境")
	}
	if d.infra.judge == nil {
		return fmt.Errorf("装配 experiment 失败: judge contract 不可用,无法提交检查点判题")
	}
	if d.infra.sim == nil {
		return fmt.Errorf("装配 experiment 失败: sim contract 不可用,无法创建仿真会话")
	}
	svc := experiment.NewService(d.infra.db, d.infra.idgen, d.infra.audit, d.infra.identity, d.infra.content, d.infra.sandbox, d.infra.judge, d.infra.sim, d.infra.bus)
	api := experiment.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	if err := svc.SubscribeEvents(); err != nil {
		return fmt.Errorf("装配 experiment 失败: 订阅引擎事件失败: %w", err)
	}
	d.infra.experiment = svc
	slog.Info("模块装配", slog.String("module", "experiment"), slog.String("layer", "2-business"))
	return nil
}
