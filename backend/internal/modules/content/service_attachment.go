// content service_attachment 文件实现 M5 附件上传规划和下载授权,统一复用基础层文件服务。
package content

import (
	"bytes"
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// UploadAttachmentRequest 是题库附件上传请求。
type UploadAttachmentRequest struct {
	ResourceID  string
	FileName    string
	ContentType string
	Content     []byte
}

// AttachmentUploadDTO 是附件上传后的受控对象引用。
type AttachmentUploadDTO struct {
	ObjectRef string `json:"object_ref"`
	FileName  string `json:"file_name"`
	Size      int64  `json:"size"`
}

// AttachmentDownloadGrantDTO 是附件短时下载授权响应。
type AttachmentDownloadGrantDTO struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// UploadAttachment 通过统一文件服务校验并写入附件对象,正文只应保存返回的 object_ref。
func (s *Service) UploadAttachment(ctx context.Context, req UploadAttachmentRequest) (AttachmentUploadDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AttachmentUploadDTO{}, err
	}
	resourceID := strings.TrimSpace(req.ResourceID)
	if resourceID == "" {
		resourceID = "draft"
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          contentModuleName,
		ResourceType:    contentAttachmentResourceType,
		ResourceID:      resourceID,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		Size:            int64(len(req.Content)),
		MaxBytes:        s.contentAttachmentMaxBytes,
		ExpectedBucket:  s.storage.BucketAttach(),
		AllowedFileName: true,
		Content:         req.Content,
	})
	if err != nil {
		return AttachmentUploadDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return AttachmentUploadDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.attachment.upload", contentAuditTargetItem, 0, map[string]any{"resource_id": resourceID, "file_name": plan.FileName}); err != nil {
		return AttachmentUploadDTO{}, err
	}
	return AttachmentUploadDTO{ObjectRef: plan.ObjectRef, FileName: plan.FileName, Size: plan.Size}, nil
}

// IssueAttachmentDownloadGrant 在业务鉴权后为 M5 附件对象签发短时下载授权。
func (s *Service) IssueAttachmentDownloadGrant(ctx context.Context, resourceID, objectRef string) (AttachmentDownloadGrantDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return AttachmentDownloadGrantDTO{}, err
	}
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" || strings.TrimSpace(objectRef) == "" {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		ObjectRef:    objectRef,
		Module:       contentModuleName,
		ResourceType: contentAttachmentResourceType,
		ResourceID:   resourceID,
	})
	if err != nil {
		return AttachmentDownloadGrantDTO{}, apperr.ErrContentAttachmentInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "content.attachment.download", contentAuditTargetItem, 0, map[string]any{"resource_id": resourceID}); err != nil {
		return AttachmentDownloadGrantDTO{}, err
	}
	return AttachmentDownloadGrantDTO{Token: token, ExpiresAt: grant.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")}, nil
}
