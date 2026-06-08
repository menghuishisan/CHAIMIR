// content 模块(M5,第1层 引擎)装配。
// 职责:题库/模板/版本/共享。题面与全量分离,全量内容仅供教师和内部服务取用。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/content"
)

// assembleContent 装配 content 模块并注册 HTTP 路由与跨模块契约实现。
func assembleContent(d *moduleDeps) error {
	if d.infra.identity == nil || d.infra.audit == nil {
		return fmt.Errorf("装配 content 失败: identity/audit contract 不可用")
	}
	svc := content.NewService(d.infra.db, d.infra.idgen, d.infra.audit, d.infra.identity)
	api := content.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	d.infra.content = svc
	d.infra.contentImport = svc
	d.infra.contentJudge = svc
	slog.Info("模块装配", slog.String("module", "content"), slog.String("layer", "1-engine"))
	return nil
}
