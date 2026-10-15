// judge events 文件负责订阅跨模块运行时自检完成事件。
package judge

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// RegisterEventSubscriptions 注册判题目录对运行时自检结果的唯一刷新入口。
func RegisterEventSubscriptions(bus eventbus.Bus, svc *Service) error {
	if bus == nil || svc == nil {
		return fmt.Errorf("judge event subscriptions require initialized bus and service")
	}
	_, err := bus.Subscribe(contracts.SubjectSandboxRuntimeSelftestCompleted, "judge-runtime-catalog", func(ctx context.Context, data []byte) error {
		var event contracts.SandboxRuntimeSelftestCompletedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrJudgeSpecUnavailable); err != nil {
			return err
		}
		return svc.ReconcileRuntimeDependentJudgers(ctx, event)
	})
	if err != nil {
		return fmt.Errorf("judge subscribe runtime selftest: %w", err)
	}
	return nil
}
