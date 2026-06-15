// content service 文件定义 M5 服务依赖注入和通用业务编排,不接收数据库连接。
package content

import (
	"context"
	"fmt"
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

const (
	contentModuleName             = "content"
	contentAttachmentResourceType = "attachment"
)

type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketAttach() string
}

type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// Service 承载 content 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store                     Store
	ids                       snowflake.Generator
	audit                     audit.Writer
	storage                   objectStorage
	files                     fileService
	contentAttachmentMaxBytes int64
}

// ServiceDeps 是 content service 的装配依赖集合。
type ServiceDeps struct {
	Store                     Store
	IDs                       snowflake.Generator
	Audit                     audit.Writer
	Storage                   *storage.Storage
	FileService               fileService
	ContentAttachmentMaxBytes int64
}

// NewService 构造 content 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("content service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("content service 缺少 ID 生成器")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("content service 缺少审计写入器")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("content service 缺少统一对象存储")
	}
	if deps.FileService == nil {
		return nil, fmt.Errorf("content service 缺少统一文件服务")
	}
	if deps.ContentAttachmentMaxBytes <= 0 {
		return nil, fmt.Errorf("content service 缺少附件大小配置")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, storage: deps.Storage, files: deps.FileService, contentAttachmentMaxBytes: deps.ContentAttachmentMaxBytes}, nil
}

// currentIdentity 读取租户账号身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentServiceTenant 读取内部服务租户边界。
func currentServiceTenant(ctx context.Context) (int64, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0, apperr.ErrServiceUnauthorized
	}
	return id.TenantID, nil
}

// mapContentReadError 将数据库未命中归一为内容不存在。
func mapContentReadError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrContentNotFound
	}
	return apperr.ErrContentInvalid.WithCause(err)
}

// mapContentMutationError 将状态不匹配导致的未命中归一为状态错误。
func mapContentMutationError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrContentStateInvalid
	}
	return apperr.ErrContentInvalid.WithCause(err)
}

// mapPaperError 将试卷读取错误归一。
func mapPaperError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrPaperNotFound
	}
	return apperr.ErrPaperInvalid.WithCause(err)
}

// toContractImport 转换内部系统导入 contract 请求。
func toContractImport(req contracts.ContentSystemImportRequest) SystemImportRequest {
	return SystemImportRequest{
		Code:             req.Code,
		Version:          req.Version,
		Type:             req.Type,
		Title:            req.Title,
		CategoryID:       req.CategoryID,
		Difficulty:       req.Difficulty,
		Tags:             req.Tags,
		KnowledgePoints:  req.KnowledgePoints,
		AuthorID:         req.AuthorID,
		AuthorType:       req.AuthorType,
		Visibility:       req.Visibility,
		Body:             req.Body,
		SensitiveFields:  req.SensitiveFields,
		AutoPublish:      req.AutoPublish,
		SystemImportNote: req.SystemImportNote,
	}
}
