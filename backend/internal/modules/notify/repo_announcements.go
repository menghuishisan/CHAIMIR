// 读写公告及当前账号已读状态。
package notify

import (
	"context"
	"strings"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateAnnouncement 创建系统公告。
func (t *txStore) CreateAnnouncement(ctx context.Context, id, tenantID, publisherID int64, req AnnouncementRequest) (AnnouncementDTO, error) {
	expireAt, err := parseOptionalTime(req.ExpireAt)
	if err != nil {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid.WithCause(err)
	}
	row, err := t.q.CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{ID: id, TenantID: pgtypex.Int8When(tenantID, req.Scope != AnnouncementScopePlatform), Title: strings.TrimSpace(req.Title), Content: strings.TrimSpace(req.Content), Scope: req.Scope, TargetRoles: req.TargetRoles, PublisherID: publisherID, ExpireAt: timex.Timestamptz(expireAt)})
	if err != nil {
		return AnnouncementDTO{}, err
	}
	return announcementDTO(row, false), nil
}

// ListAnnouncements 查询系统公告。
func (t *txStore) ListAnnouncements(ctx context.Context, tenantID, accountID int64, roleNumbers []int16, page, size int) ([]AnnouncementDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{TenantID: tenantID, AccountID: accountID, RoleNumbers: roleNumbers, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := t.q.CountAnnouncements(ctx, sqlcgen.CountAnnouncementsParams{TenantID: pgtypex.Int8(tenantID), AccountID: accountID, RoleNumbers: roleNumbers})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AnnouncementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementRowDTO(row))
	}
	return out, total, nil
}

// CountVisibleAnnouncementFacets 按当前身份的可见边界统计公告列表的两组计数:
// scope 按发布范围,is_read 按本人是否已读(收件箱「公告」标签的未读数)。
func (t *txStore) CountVisibleAnnouncementFacets(ctx context.Context, tenantID, accountID int64, roleNumbers []int16) (pagex.Facets, error) {
	rows, err := t.q.CountVisibleAnnouncementFacets(ctx, sqlcgen.CountVisibleAnnouncementFacetsParams{TenantID: tenantID, AccountID: accountID, RoleNumbers: roleNumbers})
	if err != nil {
		return nil, err
	}
	byScope, byRead := pagex.FacetGroup{}, pagex.FacetGroup{}
	for _, row := range rows {
		byScope.Add(row.Scope, row.Count)
		byRead.Add(row.IsRead, row.Count)
	}
	return pagex.Facets{"scope": byScope, "is_read": byRead}, nil
}

// GetVisibleAnnouncement 查询当前用户可见公告。
func (t *txStore) GetVisibleAnnouncement(ctx context.Context, tenantID, accountID int64, roleNumbers []int16, announcementID int64) (AnnouncementDTO, error) {
	row, err := t.q.GetVisibleAnnouncement(ctx, sqlcgen.GetVisibleAnnouncementParams{TenantID: tenantID, AccountID: accountID, ID: announcementID, RoleNumbers: roleNumbers})
	if err != nil {
		return AnnouncementDTO{}, err
	}
	return announcementVisibleRowDTO(row), nil
}

// MarkAnnouncementRead 标记公告已读。
func (t *txStore) MarkAnnouncementRead(ctx context.Context, id, tenantID, announcementID, accountID int64) error {
	_, err := t.q.MarkAnnouncementRead(ctx, sqlcgen.MarkAnnouncementReadParams{ID: id, TenantID: tenantID, AnnouncementID: announcementID, AccountID: accountID})
	return err
}
