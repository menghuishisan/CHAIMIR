// identity service_audit 文件实现审计日志查询的权限边界和结果转换。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// QueryAuditLogsForCurrent 按当前身份收敛审计查询范围。
func (s *Service) QueryAuditLogsForCurrent(ctx context.Context, req AuditQueryRequest) (AuditListResponse, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return AuditListResponse{}, apperr.ErrUnauthorized
	}
	query := contracts.AuditQuery{
		TenantID:   req.TenantID,
		ActorID:    req.ActorID,
		Action:     req.Action,
		TargetType: req.TargetType,
		From:       req.From,
		To:         req.To,
		Page:       req.Page,
		Size:       req.Size,
	}
	if id.IsPlatform {
		query.IncludePlatform = true
		query.TenantID = 0
	}
	if !id.IsPlatform {
		if id.TenantID <= 0 || id.AccountID <= 0 {
			return AuditListResponse{}, apperr.ErrForbidden
		}
		has, err := s.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
		if err != nil {
			return AuditListResponse{}, apperr.ErrForbidden.WithCause(err)
		}
		if !has {
			return AuditListResponse{}, apperr.ErrForbidden
		}
		query.TenantID = id.TenantID
	}
	result, err := s.QueryAuditLogs(ctx, query)
	if err != nil {
		return AuditListResponse{}, err
	}
	list := make([]AuditLogDTO, 0, len(result.List))
	for _, row := range result.List {
		list = append(list, ToAuditLogDTO(row))
	}
	return AuditListResponse{List: list, Total: result.Total, Page: result.Page, Size: result.Size}, nil
}
