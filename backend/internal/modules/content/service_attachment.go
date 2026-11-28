// 通过统一文件服务上传、授权下载并补偿失败附件。
package content

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// UploadAttachment 通过统一文件服务校验并写入附件对象,正文只应保存返回的 object_ref。
func (s *Service) UploadAttachment(ctx context.Context, actor auditActorSnapshot, req UploadAttachmentRequest) (AttachmentUploadDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AttachmentUploadDTO{}, err
	}
	if err := actor.validate(id); err != nil {
		return AttachmentUploadDTO{}, err
	}
	resourceID := strings.TrimSpace(req.ResourceID)
	if resourceID == "" {
		resourceID = "draft"
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:          id.TenantID,
		AccountID:         id.AccountID,
		Module:            contentModuleName,
		ResourceType:      contentAttachmentResourceType,
		ResourceID:        resourceID,
		NestedResourceIDs: []string{ids.Format(s.ids.Generate())},
		FileName:          req.FileName,
		ContentType:       req.ContentType,
		MaxBytes:          s.contentAttachmentMaxBytes,
		ExpectedBucket:    s.storage.BucketAttach(),
		AllowedFileName:   true,
		Content:           req.Content,
		KindValidator:     upload.AttachmentKindValid,
		ScanPolicy:        s.attachmentScanPolicy,
	})
	if err != nil {
		if errors.Is(err, upload.ErrInvalidFile) {
			return AttachmentUploadDTO{}, apperr.ErrContentAttachmentUploadInvalid.WithCause(err)
		}
		return AttachmentUploadDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return AttachmentUploadDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(errors.Join(err, s.cleanupAttachments(ctx, []storage.ObjectRef{{Bucket: plan.Bucket, Key: plan.Key}})))
	}
	if err := s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.attachment.upload", contentAuditTargetItem, 0, map[string]any{"resource_id": resourceID, "file_name": plan.FileName}); err != nil {
		return AttachmentUploadDTO{}, apperr.AsAppError(errors.Join(err, s.cleanupAttachments(ctx, []storage.ObjectRef{{Bucket: plan.Bucket, Key: plan.Key}})))
	}
	return AttachmentUploadDTO{ObjectRef: plan.ObjectRef, FileName: plan.FileName, Size: plan.Size}, nil
}

// IssueAttachmentDownloadGrant 在业务鉴权后为 M5 附件对象签发短时下载授权。
func (s *Service) IssueAttachmentDownloadGrant(ctx context.Context, actor auditActorSnapshot, itemID int64, objectRef string) (AttachmentDownloadGrantDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AttachmentDownloadGrantDTO{}, err
	}
	if err := actor.validate(id); err != nil {
		return AttachmentDownloadGrantDTO{}, err
	}
	objectRef = strings.TrimSpace(objectRef)
	if itemID <= 0 || objectRef == "" {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid
	}
	var item ItemWithBody
	if err := s.store.TenantReadTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var readErr error
		item, readErr = tx.GetItemWithBodyByID(ctx, id.TenantID, itemID)
		return readErr
	}); err != nil {
		return AttachmentDownloadGrantDTO{}, mapContentReadError(err)
	}
	if item.AuthorID != id.AccountID && (item.Visibility == VisibilityPrivate || (item.Status != StatusPublished && item.Status != StatusDeprecated)) {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentNotFound
	}
	if !contentBodyHasAttachment(item.Body, objectRef) {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid
	}
	objectResourceID, err := contentAttachmentObjectResourceID(objectRef, id.TenantID, s.storage.BucketAttach())
	if err != nil {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:         id.TenantID,
		ResourceTenantID: id.TenantID,
		AccountID:        id.AccountID,
		ObjectRef:        objectRef,
		Module:           contentModuleName,
		ResourceType:     contentAttachmentResourceType,
		ResourceID:       objectResourceID,
	})
	if err != nil {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, actor.accountID, actor.role, "content.attachment.download", contentAuditTargetItem, item.ID, map[string]any{"resource_id": objectResourceID}); err != nil {
		return AttachmentDownloadGrantDTO{}, err
	}
	return AttachmentDownloadGrantDTO{Token: token, ExpiresAt: timex.RFC3339OrEmpty(grant.ExpiresAt)}, nil
}

// contentBodyHasAttachment 只接受正文标准 attachments 数组中精确保存的对象引用。
func contentBodyHasAttachment(body map[string]any, objectRef string) bool {
	attachments, ok := body["attachments"].([]any)
	if !ok {
		return false
	}
	for _, value := range attachments {
		attachment, ok := value.(map[string]any)
		if ok && strings.TrimSpace(jsonx.StringField(attachment, "object_ref")) == objectRef {
			return true
		}
	}
	return false
}

// contentAttachmentObjectResourceID 从已确认属于 M5 附件前缀的对象引用提取真实资源段。
func contentAttachmentObjectResourceID(objectRef string, tenantID int64, expectedBucket string) (string, error) {
	parsed, err := storage.ParseObjectRef(objectRef)
	if err != nil || parsed.Bucket != expectedBucket {
		return "", fmt.Errorf("附件对象引用不属于题库附件桶")
	}
	prefix, err := storage.ObjectKey(tenantID, contentModuleName, contentAttachmentResourceType)
	if err != nil {
		return "", err
	}
	relative := strings.TrimPrefix(parsed.Key, prefix+"/")
	resourceID, _, ok := strings.Cut(relative, "/")
	if !ok || resourceID == "" || relative == parsed.Key {
		return "", fmt.Errorf("附件对象引用缺少资源段")
	}
	return resourceID, nil
}

func (s *Service) cleanupAttachments(ctx context.Context, refs []storage.ObjectRef) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	defer cancel()
	var failures []error
	for _, ref := range refs {
		if err := s.storage.Delete(ctx, ref.Bucket, ref.Key); err != nil {
			logging.ErrorContext(ctx, "清理题库附件失败", err.Error(), slog.String("bucket", ref.Bucket), slog.String("key", ref.Key))
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
