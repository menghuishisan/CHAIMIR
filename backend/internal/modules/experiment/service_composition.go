// service_composition 文件负责把教师提交的实验环境声明交给 M2 编译并回写服务端快照。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// compileEnvironmentComponents 把教师声明编译为仅含完整快照的持久化组件。
func (s *Service) compileEnvironmentComponents(ctx context.Context, tenantID int64, config ComponentConfigRequest) (ComponentConfig, error) {
	if len(config.Envs) == 0 {
		return ComponentConfig{Envs: []EnvComponent{}, Sims: config.Sims, Checkpoints: config.Checkpoints, Stages: config.Stages}, nil
	}
	if s.sandbox == nil {
		return ComponentConfig{}, apperr.ErrExperimentSandboxUnavailable
	}
	envs := make([]EnvComponent, 0, len(config.Envs))
	for idx := range config.Envs {
		env := &config.Envs[idx]
		snapshot, err := s.sandbox.CompileSandboxComposition(ctx, tenantID, contracts.SandboxCompositionSpec{
			ID:                       env.ID,
			Runtimes:                 env.Runtimes,
			WorkspaceRuntimeInstance: env.WorkspaceRuntimeInstance,
			Infra:                    env.Infra,
			Tools:                    env.Tools,
			Links:                    env.Links,
			AccessProfile:            env.AccessProfile,
			ResourceProfile:          env.ResourceProfile,
			NetworkProfile:           env.NetworkProfile,
		})
		if err != nil {
			return ComponentConfig{}, apperr.ErrExperimentSandboxUnavailable.WithCause(err)
		}
		envs = append(envs, EnvComponent{
			ID:                       env.ID,
			CompositionSnapshot:      snapshot,
			InitCodeRef:              env.InitCodeRef,
			InitScriptRef:            env.InitScriptRef,
			KeepAlive:                env.KeepAlive,
			SnapshotEnabled:          env.SnapshotEnabled,
			KeepAliveMinutes:         env.KeepAliveMinutes,
			SnapshotRetentionMinutes: env.SnapshotRetentionMinutes,
		})
	}
	return ComponentConfig{Envs: envs, Sims: config.Sims, Checkpoints: config.Checkpoints, Stages: config.Stages}, nil
}
