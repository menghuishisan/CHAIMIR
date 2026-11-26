// 按管理员权限查询统一审计，批量解析授权范围内的操作人。
package admin

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// QueryAudit 查询统一审计日志。
func (s *Service) QueryAudit(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return contracts.AuditQueryResult{}, err
	}
	query = scopedAuditQuery(id, query)
	result, err := s.auditRead.QueryAuditLogs(ctx, query)
	if err != nil {
		return contracts.AuditQueryResult{}, apperr.ErrAdminAuditQueryInvalid.WithCause(err)
	}
	return result, nil
}

// AuditActor 是一条审计记录操作人的显示信息。
type AuditActor struct {
	Name string
	No   string
}

// QueryAuditPage 查询审计分页并解析当前页操作人:学校身份解析本校账号;平台身份只解析平台管理员,
// 不跨校读取学校人员姓名(学校的操作人在平台审计里只显示身份)。系统记录(操作人编号 0)没有姓名。
func (s *Service) QueryAuditPage(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, map[int64]AuditActor, error) {
	result, err := s.QueryAudit(ctx, query)
	if err != nil {
		return contracts.AuditQueryResult{}, nil, err
	}
	id, _ := tenant.FromContext(ctx)
	actors, err := s.auditActors(ctx, id.IsPlatform, result.List)
	if err != nil {
		return contracts.AuditQueryResult{}, nil, err
	}
	return result, actors, nil
}

// auditActors 按身份范围批量解析当前页操作人:平台视角只取平台管理员,学校视角只取本校账号。
func (s *Service) auditActors(ctx context.Context, platform bool, entries []contracts.AuditLogEntry) (map[int64]AuditActor, error) {
	seen := map[int64]struct{}{}
	wanted := make([]int64, 0, len(entries))
	for _, entry := range entries {
		platformAdmin := entry.ActorRole == contracts.RoleNumPlatformAdmin
		if _, ok := seen[entry.ActorID]; ok || entry.ActorID <= 0 || platformAdmin != platform {
			continue
		}
		seen[entry.ActorID] = struct{}{}
		wanted = append(wanted, entry.ActorID)
	}
	actors := make(map[int64]AuditActor, len(wanted))
	if len(wanted) == 0 {
		return actors, nil
	}
	if platform {
		names, err := s.identity.PlatformAdminNames(ctx, wanted)
		if err != nil {
			return nil, apperr.ErrInternal.WithCause(err)
		}
		for actorID, name := range names {
			actors[actorID] = AuditActor{Name: name}
		}
		return actors, nil
	}
	accounts, err := s.accounts.BatchGetAccounts(ctx, wanted)
	if err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	for _, account := range accounts {
		actors[account.AccountID] = AuditActor{Name: account.Name, No: account.No}
	}
	return actors, nil
}

// scopedAuditQuery 按调用方身份冻结审计读取范围,查询与导出共用:
// 平台身份可按学校筛选(TenantID 非 0 只读该校,为 0 读全平台含平台自身记录);学校身份永远固定在本校。
func scopedAuditQuery(id tenant.Identity, query contracts.AuditQuery) contracts.AuditQuery {
	if id.IsPlatform {
		query.IncludePlatform = true
		return query
	}
	query.TenantID = id.TenantID
	query.IncludePlatform = false
	return query
}
