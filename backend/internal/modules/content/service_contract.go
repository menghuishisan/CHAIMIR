// content service_contract 文件实现 M5 对其他模块暴露的判题配置和系统导入契约。
package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// GetJudgeSpec 按租户与锁定版本读取判题配置与答案快照。
func (s *Service) GetJudgeSpec(ctx context.Context, tenantID int64, itemCode, itemVersion string) (contracts.ContentJudgeSpec, error) {
	item, err := s.GetContentFull(ctx, tenantID, contracts.ContentItemRef{ItemCode: itemCode, ItemVersion: itemVersion})
	if err != nil {
		return contracts.ContentJudgeSpec{}, err
	}
	spec := contracts.ContentJudgeSpec{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, VersionHash: item.VersionHash}
	if raw, ok := item.Body["judge_config"].(map[string]any); ok {
		spec.JudgerCode = stringFromAny(raw["judger_code"])
		spec.SuiteRef = stringFromAny(raw["suite_ref"])
		spec.MaxScore = int32FromAny(raw["max_score"])
		if expectation, ok := raw["expectation"].(map[string]any); ok {
			cloned, err := cloneMapStrict(expectation)
			if err != nil {
				return contracts.ContentJudgeSpec{}, apperr.ErrContentBodyInvalid.WithCause(err)
			}
			spec.Expectation = cloned
		}
	}
	if spec.MaxScore == 0 {
		spec.MaxScore = int32FromAny(item.Body["max_score"])
	}
	if spec.SuiteRef == "" {
		spec.SuiteRef = stringFromAny(item.Body["suite_ref"])
	}
	if spec.Expectation == nil {
		spec.Expectation = map[string]any{}
		if expectation, ok := item.Body["expectation"].(map[string]any); ok {
			cloned, err := cloneMapStrict(expectation)
			if err != nil {
				return contracts.ContentJudgeSpec{}, apperr.ErrContentBodyInvalid.WithCause(err)
			}
			spec.Expectation = cloned
		}
	}
	if spec.JudgerCode == "" || spec.MaxScore <= 0 {
		return contracts.ContentJudgeSpec{}, apperr.ErrContentBodyInvalid
	}
	return spec, nil
}

// SystemImportContent 把预验证后的自包含题目固化到内容中心。
func (s *Service) SystemImportContent(ctx context.Context, req contracts.ContentSystemImportRequest) (contracts.ContentItemSnapshot, error) {
	tenantID, err := currentServiceTenant(ctx)
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	if req.TenantID != 0 && req.TenantID != tenantID {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentSystemImportInvalid
	}
	httpReq, err := validateSystemImport(toContractImport(req))
	if err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	body, err := jsonx.CloneObjectStrict(httpReq.Body)
	if err != nil {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	item := ItemWithBody{Item: Item{ID: s.ids.Generate(), TenantID: tenantID, Code: httpReq.Code, Version: httpReq.Version, Type: httpReq.Type, Title: httpReq.Title, CategoryID: httpReq.CategoryID, Difficulty: httpReq.Difficulty, Tags: httpReq.Tags, KnowledgePoints: httpReq.KnowledgePoints, AuthorID: httpReq.AuthorID, AuthorType: httpReq.AuthorType, Visibility: httpReq.Visibility, Status: StatusDraft}, Body: body, SensitiveFields: httpReq.SensitiveFields}
	if httpReq.AutoPublish {
		item.Status = StatusPublished
	}
	item.VersionHash, err = versionHash(item.Item, item.Body, item.SensitiveFields)
	if err != nil {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentBodyInvalid.WithCause(err)
	}
	var created ItemWithBody
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		created, err = tx.CreateItem(ctx, item)
		return err
	}); err != nil {
		return contracts.ContentItemSnapshot{}, apperr.ErrContentVersionConflict.WithCause(err)
	}
	detail := map[string]any{"code": created.Code, "version": created.Version, "auto_publish": httpReq.AutoPublish, "note": httpReq.SystemImportNote}
	if sourceRef, ok := auth.ServiceSourceRefFromContext(ctx); ok {
		detail["source_ref"] = sourceRef
	}
	if err := s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, "content.system_import", contentAuditTargetItem, created.ID, detail); err != nil {
		return contracts.ContentItemSnapshot{}, err
	}
	return contractSnapshot(created)
}

// SystemImportContentFromHTTP 适配内部 HTTP 系统建题入口。
func (s *Service) SystemImportContentFromHTTP(ctx context.Context, req SystemImportRequest) (ItemSnapshotDTO, error) {
	tenantID, err := currentServiceTenant(ctx)
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	snapshot, err := s.SystemImportContent(ctx, contracts.ContentSystemImportRequest{TenantID: tenantID, Code: req.Code, Version: req.Version, Type: req.Type, Title: req.Title, CategoryID: req.CategoryID, Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints, AuthorID: req.AuthorID, AuthorType: req.AuthorType, Visibility: req.Visibility, Body: req.Body, SensitiveFields: req.SensitiveFields, AutoPublish: req.AutoPublish, SystemImportNote: req.SystemImportNote})
	if err != nil {
		return ItemSnapshotDTO{}, err
	}
	return ItemSnapshotDTO{ItemDTO: ItemDTO{Code: snapshot.ItemCode, Version: snapshot.ItemVersion, Type: snapshot.Type, Title: snapshot.Title, Difficulty: snapshot.Difficulty, Visibility: snapshot.Visibility, Tags: snapshot.Tags, KnowledgePoints: snapshot.KnowledgePoints, VersionHash: snapshot.VersionHash, Status: snapshot.Status}, Body: snapshot.Body}, nil
}

// stringFromAny 从 JSON 动态值读取字符串。
func stringFromAny(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// int32FromAny 从 JSON 动态值读取整数。
func int32FromAny(value any) int32 {
	return jsonx.Int32FromAny(value, 0)
}
