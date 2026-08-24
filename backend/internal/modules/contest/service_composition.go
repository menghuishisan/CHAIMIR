// contest composition 文件负责从 M5 已发布题目版本读取唯一环境组合声明。
package contest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// compileCompositionFromContent 在教师编排题目时读取 M5 正文并编译唯一不可变快照。
func (s *Service) compileCompositionFromContent(ctx context.Context, tenantID int64, itemCode, itemVersion string, profile contracts.SandboxAccessProfile) (contracts.SandboxCompositionSnapshot, error) {
	face, err := s.content.GetContentFull(ctx, tenantID, contracts.ContentItemRef{ItemCode: itemCode, ItemVersion: itemVersion})
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	rawValue, ok := face.Body["composition"]
	if !ok {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid
	}
	raw, err := json.Marshal(rawValue)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	var spec contracts.SandboxCompositionSpec
	if err := jsonx.DecodeStrictKnownFields(raw, &spec); err != nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	spec, err = normalizeContestComposition(spec, profile)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, err
	}
	snapshot, err := s.sandbox.CompileSandboxComposition(ctx, tenantID, spec)
	if err != nil {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return snapshot, nil
}

// compositionFromProblem 只返回竞赛题已锁定的快照,运行阶段禁止再次读取 M5 或编译组合。
func (s *Service) compositionFromProblem(_ context.Context, _ int64, problem ContestProblem, profile contracts.SandboxAccessProfile) (contracts.SandboxCompositionSnapshot, error) {
	if problem.CompositionSnapshot == nil || strings.TrimSpace(problem.CompositionDigest) == "" {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid
	}
	snapshot := *problem.CompositionSnapshot
	if snapshot.Spec.AccessProfile != profile || snapshot.Digest != problem.CompositionDigest {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid
	}
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil || digest != problem.CompositionDigest {
		return contracts.SandboxCompositionSnapshot{}, apperr.ErrContestProblemInvalid
	}
	return snapshot, nil
}

func normalizeContestComposition(spec contracts.SandboxCompositionSpec, profile contracts.SandboxAccessProfile) (contracts.SandboxCompositionSpec, error) {
	spec.ID = strings.TrimSpace(spec.ID)
	if spec.ID == "" || spec.PrimaryRuntime.Code == "" || spec.PrimaryRuntime.ImageVersion == "" {
		return contracts.SandboxCompositionSpec{}, fmt.Errorf("题目组合缺少主运行时或作用域")
	}
	if spec.AccessProfile != "" && spec.AccessProfile != profile {
		return contracts.SandboxCompositionSpec{}, fmt.Errorf("题目组合访问配置与竞赛场景不一致")
	}
	spec.AccessProfile = profile
	return spec, nil
}
