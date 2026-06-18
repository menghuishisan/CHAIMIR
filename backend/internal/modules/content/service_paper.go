// content service_paper 文件实现手动组卷、随机组卷和试卷详情展开。
package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
)

// ListPapers 查询试卷分页。
func (s *Service) ListPapers(ctx context.Context, page, size int) ([]PaperDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var papers []Paper
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		papers, total, err = tx.ListPapers(ctx, id.TenantID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, mapPaperError(err)
	}
	out := make([]PaperDTO, 0, len(papers))
	for _, paper := range papers {
		out = append(out, paperDTO(paper))
	}
	return out, total, page, size, nil
}

// CreatePaper 创建手动或随机试卷。
func (s *Service) CreatePaper(ctx context.Context, req CreatePaperRequest) (PaperDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PaperDetailDTO{}, err
	}
	req, err = validatePaperRequest(req)
	if err != nil {
		return PaperDetailDTO{}, err
	}
	paper := Paper{ID: s.ids.Generate(), TenantID: id.TenantID, Name: req.Name, AuthorID: id.AccountID, GenMode: req.GenMode, GenCriteria: req.GenCriteria}
	var detail PaperWithItems
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		created, err := tx.CreatePaper(ctx, paper)
		if err != nil {
			return err
		}
		items, err := s.buildPaperItems(ctx, tx, id.TenantID, created.ID, req)
		if err != nil {
			return err
		}
		detail, err = s.paperDetailFromItems(ctx, tx, id.TenantID, created, items)
		return err
	}); err != nil {
		return PaperDetailDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.paper.create", contentAuditTargetPaper, detail.Paper.ID, map[string]any{"name": detail.Paper.Name, "gen_mode": detail.Paper.GenMode}); err != nil {
		return PaperDetailDTO{}, err
	}
	return paperDetailDTO(detail)
}

// GetPaperDetail 查询试卷详情并展开题面。
func (s *Service) GetPaperDetail(ctx context.Context, paperID int64) (PaperDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PaperDetailDTO{}, err
	}
	var detail PaperWithItems
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		paper, err := tx.GetPaper(ctx, id.TenantID, paperID)
		if err != nil {
			return mapPaperError(err)
		}
		items, err := tx.ListPaperItems(ctx, id.TenantID, paperID)
		if err != nil {
			return err
		}
		detail, err = s.paperDetailFromItems(ctx, tx, id.TenantID, paper, items)
		return err
	}); err != nil {
		return PaperDetailDTO{}, err
	}
	return paperDetailDTO(detail)
}

// RegeneratePaper 按原随机条件重新抽题。
func (s *Service) RegeneratePaper(ctx context.Context, paperID int64) (PaperDetailDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PaperDetailDTO{}, err
	}
	var detail PaperWithItems
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		paper, err := tx.GetPaper(ctx, id.TenantID, paperID)
		if err != nil {
			return mapPaperError(err)
		}
		if paper.GenMode != PaperModeRandom {
			return apperr.ErrPaperRegenerateFailed
		}
		req := CreatePaperRequest{Name: paper.Name, GenMode: paper.GenMode, GenCriteria: paper.GenCriteria}
		items, err := s.buildPaperItems(ctx, tx, id.TenantID, paper.ID, req)
		if err != nil {
			return err
		}
		detail, err = s.paperDetailFromItems(ctx, tx, id.TenantID, paper, items)
		return err
	}); err != nil {
		return PaperDetailDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.paper.regenerate", contentAuditTargetPaper, paperID, map[string]any{}); err != nil {
		return PaperDetailDTO{}, err
	}
	return paperDetailDTO(detail)
}

// buildPaperItems 生成手动或随机组卷题目集合。
func (s *Service) buildPaperItems(ctx context.Context, tx TxStore, tenantID, paperID int64, req CreatePaperRequest) ([]PaperItem, error) {
	if req.GenMode == PaperModeManual {
		items := make([]PaperItem, 0, len(req.Items))
		for i, input := range req.Items {
			item, err := tx.GetItemWithBodyByRef(ctx, tenantID, input.Code, input.Version)
			if err != nil {
				return nil, mapContentReadError(err)
			}
			if item.TenantID != tenantID {
				return nil, apperr.ErrContentSharedNotFound
			}
			if item.Status != StatusPublished {
				return nil, apperr.ErrContentVersionNotPublished
			}
			items = append(items, PaperItem{ID: s.ids.Generate(), TenantID: tenantID, PaperID: paperID, ItemCode: input.Code, ItemVersion: input.Version, Score: input.Score, Seq: int32(i + 1)})
		}
		return tx.ReplacePaperItems(ctx, tenantID, paperID, items)
	}
	picked, err := tx.RandomPickItems(ctx, tenantID, req.GenCriteria)
	if err != nil {
		return nil, apperr.ErrPaperGenerateFailed.WithCause(err)
	}
	if int32(len(picked)) < req.GenCriteria.Count {
		return nil, apperr.ErrPaperPickNotEnough
	}
	items := make([]PaperItem, 0, len(picked))
	for i, item := range picked {
		items = append(items, PaperItem{ID: s.ids.Generate(), TenantID: tenantID, PaperID: paperID, ItemCode: item.Code, ItemVersion: item.Version, Score: req.GenCriteria.DefaultScore, Seq: int32(i + 1)})
	}
	return tx.ReplacePaperItems(ctx, tenantID, paperID, items)
}

// paperDetailFromItems 展开试卷题面快照。
func (s *Service) paperDetailFromItems(ctx context.Context, tx TxStore, tenantID int64, paper Paper, items []PaperItem) (PaperWithItems, error) {
	out := PaperWithItems{Paper: paper, Items: make([]PaperItemFace, 0, len(items))}
	for _, item := range items {
		snapshot, err := tx.GetItemWithBodyByRef(ctx, tenantID, item.ItemCode, item.ItemVersion)
		if err != nil {
			return PaperWithItems{}, mapContentReadError(err)
		}
		face, err := faceSnapshot(snapshot)
		if err != nil {
			return PaperWithItems{}, apperr.ErrContentBodyInvalid.WithCause(err)
		}
		out.Items = append(out.Items, PaperItemFace{PaperItem: item, Item: itemDTO(face.Item), Body: face.Body})
	}
	return out, nil
}

// refsFromBatchDTO 转换批量 HTTP 引用为 contract 引用。
func refsFromBatchDTO(req BatchItemsRequest) ([]contracts.ContentItemRef, error) {
	if len(req.Items) == 0 || len(req.Items) > 100 {
		return nil, apperr.ErrContentQueryInvalid
	}
	refs := make([]contracts.ContentItemRef, 0, len(req.Items))
	for _, item := range req.Items {
		if !validCode(item.Code) || !validVersion(item.Version) {
			return nil, apperr.ErrContentQueryInvalid
		}
		refs = append(refs, contracts.ContentItemRef{ItemCode: item.Code, ItemVersion: item.Version})
	}
	return refs, nil
}
