// admin 模块(M9,第3层 聚合)装配。
// 职责:运营看板/配置/审计查询(只读聚合)与 M9 自有运维元数据。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/admin"
	"chaimir/pkg/crypto"
)

// assembleAdmin 装配 admin 模块并注册 HTTP 路由。
func assembleAdmin(d *moduleDeps) error {
	if d.infra.identity == nil || d.infra.identityAdmin == nil {
		return fmt.Errorf("装配 admin 失败: identity 管理契约不可用")
	}
	if d.infra.audit == nil {
		return fmt.Errorf("装配 admin 失败: audit writer 不可用")
	}
	cipher, err := crypto.NewCipher([]byte(d.cfg.Auth.EncryptionKey))
	if err != nil {
		return fmt.Errorf("装配 admin 失败: 初始化配置加密器失败: %w", err)
	}
	svc := admin.NewService(
		d.infra.db,
		d.infra.idgen,
		d.infra.audit,
		cipher,
		d.cfg.Deploy,
		d.cfg.Monitoring,
		d.infra.identityAdmin,
		d.infra.sandbox,
		d.infra.teaching,
		d.infra.experiment,
		d.infra.contest,
		d.infra.notify,
	)
	api := admin.NewAPI(svc, d.infra.auth, d.infra.identity, d.cfg.Deploy)
	api.Register(d.infra.server.apiV1())
	slog.Info("模块装配", slog.String("module", "admin"), slog.String("layer", "3-aggregation"))
	return nil
}
