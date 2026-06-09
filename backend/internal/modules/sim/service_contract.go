// M4 契约实现:把 Service 适配为 internal/contracts.SimService。
package sim

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
)

// CreateSimSession 按 contracts 输入创建仿真会话。
func (s *Service) CreateSimSession(ctx context.Context, req contracts.SimCreateSessionRequest) (contracts.SimSessionInfo, error) {
	dto, err := s.CreateSession(ctx, req.TenantID, CreateSessionRequest{
		PackageCode: req.PackageCode, Version: req.Version, Seed: req.Seed,
		InitParams: req.InitParams, OwnerAccountID: ids.Format(req.OwnerAccountID), SourceRef: req.SourceRef,
	})
	if err != nil {
		return contracts.SimSessionInfo{}, err
	}
	sessionID, _ := ids.Parse(dto.SessionID)
	return contracts.SimSessionInfo{
		SessionID: sessionID, TenantID: req.TenantID, PackageCode: dto.PackageCode,
		Version: dto.Version, Compute: dto.Compute, BundleRef: dto.BundleRef, SourceRef: req.SourceRef,
	}, nil
}

// GetSimReplay 查询指定租户会话的回放数据。
func (s *Service) GetSimReplay(ctx context.Context, tenantID, sessionID int64) (contracts.SimReplayInfo, error) {
	dto, err := s.replayInTenant(ctx, tenantID, sessionID)
	if err != nil {
		return contracts.SimReplayInfo{}, err
	}
	return replayToContract(dto), nil
}

// ReportSimCheckpoint 保存仿真检查点快照。
func (s *Service) ReportSimCheckpoint(ctx context.Context, req contracts.SimCheckpointRequest) error {
	return s.reportCheckpointInTenant(ctx, req.TenantID, req.SessionID, ReportCheckpointRequest{
		CheckpointID: req.CheckpointID, Answer: req.Answer, Achieved: req.Achieved,
	})
}

// RecycleSimBySourceRef 按来源标识归档仿真会话。
func (s *Service) RecycleSimBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error {
	return s.RecycleBySourceRef(ctx, tenantID, sourceRef, reason)
}

// replayToContract 把 HTTP DTO 转为 contracts DTO。
func replayToContract(dto ReplayDTO) contracts.SimReplayInfo {
	out := contracts.SimReplayInfo{PackageCode: dto.PackageCode, Version: dto.Version, Seed: dto.Seed, InitParams: dto.InitParams}
	for _, action := range dto.Actions {
		out.Actions = append(out.Actions, contracts.SimActionInfo{Seq: action.Seq, AtTick: action.AtTick, EventType: action.EventType, Payload: action.Payload})
	}
	return out
}
