package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// PrepareContentClone 只为已授权课程准备自包含版本，不向调用方暴露隐藏正文。
func (s *Service) PrepareContentClone(ctx context.Context, sourceTenantID int64, refs []contracts.ContentItemRef) (plan contracts.ContentClonePlan, err error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return plan, err
	}
	if sourceTenantID <= 0 || id.IsSystem || id.IsPlatform {
		return plan, apperr.ErrContentCloneInvalid
	}
	plan.Refs = make(map[contracts.ContentItemRef]contracts.ContentItemRef, len(refs))
	var items []ItemWithBody
	var objects []storage.ObjectRef
	plan.Discard = func(ctx context.Context) error { return s.cleanupAttachments(ctx, objects) }
	defer func() {
		if err != nil {
			err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, false, plan.Discard)
		}
	}()
	err = s.store.TenantTx(ctx, sourceTenantID, func(ctx context.Context, tx TxStore) error {
		for _, ref := range refs {
			if _, ok := plan.Refs[ref]; ok {
				continue
			}
			if !validCode(ref.ItemCode) || !validVersion(ref.ItemVersion) {
				return apperr.ErrContentCloneInvalid
			}
			source, err := tx.GetItemWithBodyByRef(ctx, sourceTenantID, ref.ItemCode, ref.ItemVersion)
			if err != nil {
				return mapContentReadError(err)
			}
			if source.TenantID != sourceTenantID || (source.Status != StatusPublished && source.Status != StatusDeprecated) {
				return apperr.ErrContentVersionNotPublished
			}
			hash, err := versionHash(source.Item, source.Body, source.SensitiveFields)
			if err != nil || hash != source.VersionHash {
				return apperr.ErrContentBodyInvalid.WithCause(err)
			}
			source.ID = s.ids.Generate()
			source.TenantID, source.AuthorID = id.TenantID, id.AccountID
			source.AuthorType, source.Visibility, source.Status = AuthorTeacher, VisibilityPrivate, StatusPublished
			source.UsageCount = 0
			source.Code = "clone-" + ids.Format(source.ID)
			if sourceTenantID != id.TenantID {
				source.CategoryID = 0
			}
			plan.Refs[ref] = contracts.ContentItemRef{ItemCode: source.Code, ItemVersion: source.Version}
			items = append(items, source)
		}
		return nil
	})
	if err != nil {
		return plan, err
	}
	for i := range items {
		item := &items[i]
		var copied []storage.ObjectRef
		item.Body, copied, err = s.cloneBodyAttachments(ctx, item.Body, sourceTenantID, id.TenantID, item.ID, item.Code, item.Version)
		if err != nil {
			return plan, err
		}
		objects = append(objects, copied...)
		item.VersionHash, err = versionHash(item.Item, item.Body, item.SensitiveFields)
		if err != nil {
			return plan, err
		}
	}
	plan.Save = func(ctx context.Context) error {
		return s.store.CloneTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			for _, item := range items {
				if _, err := tx.CreateItem(ctx, item); err != nil {
					return err
				}
				if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.course.clone", contentAuditTargetItem, item.ID, map[string]any{"source_tenant_id": ids.Format(sourceTenantID)}); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return plan, nil
}

var _ contracts.ContentCloneService = (*Service)(nil)
