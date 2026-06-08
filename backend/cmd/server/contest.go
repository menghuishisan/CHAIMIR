// contest 模块(M8,第2层 业务)装配。
// 职责:竞赛/赛制/撮合/排行。环境调 M2,判题调 M3,漏洞源 finalize 调 M5。
package main

import (
	"fmt"
	"log/slog"

	"chaimir/internal/modules/contest"
	"chaimir/pkg/crypto"
)

// assembleContest 装配 contest 模块并注册 HTTP 路由、事件订阅与跨模块契约实现。
func assembleContest(d *moduleDeps) error {
	if d.infra.identity == nil || d.infra.audit == nil {
		return fmt.Errorf("装配 contest 失败: identity/audit contract 不可用")
	}
	if d.infra.content == nil || d.infra.contentImport == nil {
		return fmt.Errorf("装配 contest 失败: content contract 不可用,无法锁定题目或固化漏洞题")
	}
	if d.infra.sandbox == nil {
		return fmt.Errorf("装配 contest 失败: sandbox contract 不可用,无法创建竞赛环境")
	}
	if d.infra.judge == nil {
		return fmt.Errorf("装配 contest 失败: judge contract 不可用,无法提交竞赛判题")
	}
	cipher, err := crypto.NewCipher([]byte(d.cfg.Auth.EncryptionKey))
	if err != nil {
		return fmt.Errorf("装配 contest 失败: 初始化漏洞源配置加密器失败: %w", err)
	}
	svc := contest.NewService(d.infra.db, d.infra.idgen, d.infra.audit, cipher, d.cfg.Contest, d.infra.identity, d.infra.content, d.infra.contentImport, d.infra.sandbox, d.infra.judge, d.infra.bus)
	api := contest.NewAPI(svc, d.infra.auth, d.infra.identity)
	api.Register(d.infra.server.apiV1())
	if err := svc.SubscribeEvents(); err != nil {
		return fmt.Errorf("装配 contest 失败: 订阅判题事件失败: %w", err)
	}
	d.infra.contest = svc
	slog.Info("模块装配", slog.String("module", "contest"), slog.String("layer", "2-business"))
	return nil
}
