// 发布公告并按当前身份查询可见范围。
package notify

import (
	"context"
	"log/slog"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// isValidRoleNumber 在 service 边界校验跨模块角色数字契约,保持 rules 层纯净。
func isValidRoleNumber(role int16) bool {
	return role != contracts.RoleNumPlatformAdmin && contracts.RoleCode(role) != "unknown"
}

// CreateAnnouncement 发布系统公告。
func (s *Service) CreateAnnouncement(ctx context.Context, req AnnouncementRequest) (AnnouncementDTO, error) {
	id, err := currentIdentityAllowPlatform(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	if err := validateAnnouncementRequest(req, id.IsPlatform, isValidRoleNumber); err != nil {
		return AnnouncementDTO{}, err
	}
	tenantID := id.TenantID
	if req.Scope == AnnouncementScopePlatform {
		tenantID = 0
	}
	var out AnnouncementDTO
	write := func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAnnouncement(ctx, s.ids.Generate(), tenantID, id.AccountID, req)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, tenantID, id.AccountID, auditRoleForIdentity(id), "notify.announcement.publish", auditTargetAnnouncement, out.ID.Int64(), map[string]any{"scope": out.Scope, "tenant_id": out.TenantID.String()})
	}
	if tenantID == 0 {
		err = s.store.PlatformTx(ctx, write)
	} else {
		err = s.store.TenantTx(ctx, tenantID, write)
	}
	if err != nil {
		return AnnouncementDTO{}, err
	}
	s.broadcastAnnouncement(ctx, out)
	return out, nil
}

// ListAnnouncements 查询可见公告分页,并带回范围与本人已读两组计数(当前身份可见的全部公告)。
func (s *Service) ListAnnouncements(ctx context.Context, page, size int) ([]AnnouncementDTO, int64, int, int, map[string]map[string]int64, error) {
	id, err := currentIdentityAllowPlatform(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	var out []AnnouncementDTO
	var total int64
	var facets pagex.Facets
	if id.IsPlatform {
		err = s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			out, total, err = tx.ListAnnouncements(ctx, 0, id.AccountID, []int16{contracts.RoleNumPlatformAdmin}, page, size)
			if err == nil {
				facets, err = tx.CountVisibleAnnouncementFacets(ctx, 0, id.AccountID, []int16{contracts.RoleNumPlatformAdmin})
			}
			return err
		})
		if err != nil {
			return nil, 0, page, size, nil, err
		}
		return out, total, page, size, facets.Map(), nil
	}
	roleNumbers, err := s.currentRoleNumbers(ctx, id.AccountID)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListAnnouncements(ctx, id.TenantID, id.AccountID, roleNumbers, page, size)
		if err == nil {
			facets, err = tx.CountVisibleAnnouncementFacets(ctx, id.TenantID, id.AccountID, roleNumbers)
		}
		return err
	})
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	return out, total, page, size, facets.Map(), nil
}

// MarkAnnouncementRead 标记公告已读。
func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	roleNumbers, err := s.currentRoleNumbers(ctx, id.AccountID)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetVisibleAnnouncement(ctx, id.TenantID, id.AccountID, roleNumbers, announcementID); err != nil {
			return apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
		}
		return tx.MarkAnnouncementRead(ctx, s.ids.Generate(), id.TenantID, announcementID, id.AccountID)
	})
}

// broadcastAnnouncement 用租户告警主题提示在线用户刷新公告列表。
func (s *Service) broadcastAnnouncement(ctx context.Context, ann AnnouncementDTO) {
	if ann.TenantID <= 0 {
		return
	}
	data, err := jsonx.AnyBytes(map[string]any{"type": "announcement", "announcement_id": ids.Format(ann.ID.Int64()), "scope": ann.Scope}, apperr.ErrNotifyPushFailed)
	if err != nil {
		logging.ErrorContext(ctx, "公告实时提醒序列化失败", err.Error(), slog.Int64("tenant_id", ann.TenantID.Int64()), slog.Int64("announcement_id", ann.ID.Int64()))
		return
	}
	s.hub.Broadcast(tenantAlertTopic(ann.TenantID.Int64()), data)
}

// currentRoleNumbers 读取当前用户角色并转成公告查询所需的数字角色集合。
func (s *Service) currentRoleNumbers(ctx context.Context, accountID int64) ([]int16, error) {
	candidates := []string{contracts.RoleSchoolAdmin, contracts.RoleTeacher, contracts.RoleStudent}
	out := make([]int16, 0, len(candidates))
	for _, role := range candidates {
		has, err := s.roles.HasRole(ctx, accountID, role)
		if err != nil {
			return nil, apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
		}
		if !has {
			continue
		}
		if n, ok := contracts.RoleNumber(role); ok {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = []int16{-1}
	}
	return out, nil
}
