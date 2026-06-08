// grade 模块(M11,第3层 聚合)装配。
// 职责:跨课程聚合/GPA/审核/申诉(只读 teaching)。不算单课程成绩。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/grade"
)

// assembleGrade 装配 grade 模块并注册 HTTP 路由与事件订阅。
func assembleGrade(d *moduleDeps) error {
	if d.infra.teaching == nil {
		return fmt.Errorf("装配 grade 失败: teaching 只读契约不可用")
	}
	if d.infra.audit == nil {
		return fmt.Errorf("装配 grade 失败: audit writer 不可用")
	}
	if d.infra.identity == nil {
		return fmt.Errorf("装配 grade 失败: identity 契约不可用")
	}
	if d.infra.store == nil {
		return fmt.Errorf("装配 grade 失败: 对象存储不可用")
	}
	svc := grade.NewService(d.infra.db, d.infra.idgen, d.infra.audit, d.infra.identity, d.infra.teaching, d.infra.contest, d.infra.notify, d.infra.store, d.cfg.Grade)
	api := grade.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	if err := svc.SubscribeEvents(d.infra.bus); err != nil {
		return err
	}
	slog.Info("模块装配", slog.String("module", "grade"), slog.String("layer", "3-aggregation"))
	return nil
}
