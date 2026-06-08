// M5 契约实现:把 Service 适配为 internal/contracts 的内容读取与判题配置能力。
package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// GetJudgeSpec 按锁定题目版本读取判题配置与答案快照,供 M3 构建判题输入。
func (s *Service) GetJudgeSpec(ctx context.Context, itemCode, itemVersion string) (contracts.ContentJudgeSpec, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return contracts.ContentJudgeSpec{}, apperr.ErrUnauthorized
	}
	item, err := s.getContentFullInTenant(ctx, id.TenantID, itemCode, itemVersion)
	if err != nil {
		return contracts.ContentJudgeSpec{}, err
	}
	return judgeSpecFromItem(item)
}

// GetContentFace 按锁定版本读取题面视角内容,敏感字段已剥离。
func (s *Service) GetContentFace(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getContentFaceInTenant(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	return contractSnapshotFromItem(item), nil
}

// GetContentFull 按锁定版本读取全量内容,供内部服务取实验模板/判题配置。
func (s *Service) GetContentFull(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) (contracts.ContentItemSnapshot, error) {
	item, err := s.getContentFullInTenant(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	return contractSnapshotFromItem(item), nil
}

// BatchGetContentFace 批量读取题面内容,用于作业或竞赛组卷展开。
func (s *Service) BatchGetContentFace(ctx context.Context, tenantID int64, refs []contracts.ContentItemRef) ([]contracts.ContentItemSnapshot, error) {
	out := make([]contracts.ContentItemSnapshot, 0, len(refs))
	for _, ref := range refs {
		item, err := s.GetContentFace(ctx, tenantID, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// IncrementContentUsage 记录内容被上游业务引用。
func (s *Service) IncrementContentUsage(ctx context.Context, tenantID int64, ref contracts.ContentItemRef) error {
	return s.IncrementUsage(ctx, tenantID, ref.ItemCode, ref.ItemVersion)
}

// SystemImportContent 把外部源预验证后的自包含内容固化入 M5。
func (s *Service) SystemImportContent(ctx context.Context, req contracts.ContentSystemImportRequest) (contracts.ContentItemSnapshot, error) {
	out, err := s.CreateItem(ctx, CreateItemRequest{
		Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title,
		CategoryID: ids.Format(req.CategoryID), Difficulty: req.Difficulty, Tags: req.Tags,
		KnowledgePoints: req.KnowledgePoints, AuthorID: ids.Format(req.AuthorID), AuthorType: req.AuthorType,
		Visibility: req.Visibility, Body: req.Body, SensitiveFields: req.SensitiveFields,
		AutoPublish: req.AutoPublish, SystemImportNote: req.SystemImportNote,
	})
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	return contractSnapshotFromItem(out), nil
}

// getContentFaceInTenant 在显式租户 RLS 下读取题面。
func (s *Service) getContentFaceInTenant(ctx context.Context, tenantID int64, code, version string) (ItemDTO, error) {
	return s.getContentInTenant(ctx, tenantID, code, version, true)
}

// getContentFullInTenant 在显式租户 RLS 下读取全量。
func (s *Service) getContentFullInTenant(ctx context.Context, tenantID int64, code, version string) (ItemDTO, error) {
	return s.getContentInTenant(ctx, tenantID, code, version, false)
}

// getContentInTenant 是 contracts 内部调用的显式租户读取入口。
func (s *Service) getContentInTenant(ctx context.Context, tenantID int64, code, version string, face bool) (ItemDTO, error) {
	var row itemRow
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.GetContentByCodeVersion(ctx, sqlcgen.GetContentByCodeVersionParams{Code: code, Version: version})
		row = contentRowFromOwn(found)
		return err
	}); err != nil {
		return ItemDTO{}, apperr.ErrContentNotFound.WithCause(err)
	}
	item, err := itemDTOFromRow(row, face)
	if err != nil {
		return ItemDTO{}, err
	}
	if face && item.Status != ItemStatusPublished {
		return ItemDTO{}, apperr.ErrContentUnavailable
	}
	return item, nil
}
