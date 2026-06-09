// storage 提供统一文件服务唯一对外入口,收敛上传规划、下载授权与对象引用约束。
package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
)

// Service 收敛统一文件服务的上传规划和下载授权能力,避免模块各自拼装第二套链路。
type Service struct {
	Scanner          upload.Scanner
	SigningKey       string
	DownloadGrantTTL time.Duration
}

// PlanUploadRequest 描述统一文件服务生成对象引用前需要确认的资源边界和安全校验参数。
type PlanUploadRequest struct {
	TenantID        int64
	AccountID       int64
	Module          string
	ResourceType    string
	ResourceID      string
	FileName        string
	ContentType     string
	Size            int64
	MaxBytes        int64
	ExpectedBucket  string
	AllowedFileName bool
	Content         []byte
	KindValidator   func(fileName, contentType string, content []byte) bool
	ScanPolicy      upload.ScanPolicy
}

// UploadPlan 表示统一文件服务规划出的受控对象路径和默认下载授权边界。
type UploadPlan struct {
	TenantID          int64
	AccountID         int64
	Module            string
	ResourceType      string
	ResourceID        string
	FileName          string
	ContentType       string
	Bucket            string
	Key               string
	ObjectRef         string
	Size              int64
	DownloadExpiresAt time.Time
}

// IssueDownloadGrantRequest 描述统一文件服务为某个对象引用签发短时下载授权所需参数。
type IssueDownloadGrantRequest struct {
	TenantID     int64
	AccountID    int64
	ObjectRef    string
	Module       string
	ResourceType string
	ResourceID   string
	ExpiresAt    time.Time
}

// PlanUpload 在统一入口完成大小、文件名、类型和病毒扫描校验,并生成租户作用域对象引用。
func (s Service) PlanUpload(req PlanUploadRequest) (UploadPlan, error) {
	if req.TenantID <= 0 {
		return UploadPlan{}, fmt.Errorf("上传规划缺少 tenant_id")
	}
	if req.AccountID <= 0 {
		return UploadPlan{}, fmt.Errorf("上传规划缺少 account_id")
	}
	if strings.TrimSpace(req.ExpectedBucket) == "" {
		return UploadPlan{}, fmt.Errorf("上传规划缺少目标 bucket")
	}
	if err := validateScopedResource(req.Module, req.ResourceType, req.ResourceID); err != nil {
		return UploadPlan{}, err
	}
	fileName, err := validateUploadFileName(req.FileName, req.AllowedFileName)
	if err != nil {
		return UploadPlan{}, err
	}
	switch upload.CheckSize(req.Size, req.MaxBytes) {
	case upload.SizeEmpty:
		return UploadPlan{}, fmt.Errorf("上传文件不能为空")
	case upload.SizeTooLarge:
		return UploadPlan{}, fmt.Errorf("上传文件超出大小限制")
	}
	if req.KindValidator != nil && !req.KindValidator(fileName, req.ContentType, req.Content) {
		return UploadPlan{}, fmt.Errorf("上传文件类型不符合统一校验规则")
	}
	if err := upload.VerifyScan(s.Scanner, req.ScanPolicy, upload.ScanRequest{
		FileName: fileName,
		Content:  req.Content,
	}); err != nil {
		return UploadPlan{}, err
	}

	key, err := ObjectKey(req.TenantID, req.Module, req.ResourceType, req.ResourceID, fileName)
	if err != nil {
		return UploadPlan{}, err
	}
	expiresAt, err := s.defaultDownloadExpiry()
	if err != nil {
		return UploadPlan{}, err
	}
	return UploadPlan{
		TenantID:          req.TenantID,
		AccountID:         req.AccountID,
		Module:            req.Module,
		ResourceType:      req.ResourceType,
		ResourceID:        req.ResourceID,
		FileName:          fileName,
		ContentType:       strings.TrimSpace(req.ContentType),
		Bucket:            strings.TrimSpace(req.ExpectedBucket),
		Key:               key,
		ObjectRef:         "minio://" + strings.TrimSpace(req.ExpectedBucket) + "/" + key,
		Size:              req.Size,
		DownloadExpiresAt: expiresAt,
	}, nil
}

// IssueDownloadGrant 为统一文件服务产出的对象引用签发短时下载授权并返回可校验令牌。
func (s Service) IssueDownloadGrant(req IssueDownloadGrantRequest) (string, DownloadGrant, error) {
	expiresAt := timex.UTC(req.ExpiresAt)
	if expiresAt.IsZero() {
		var err error
		expiresAt, err = s.defaultDownloadExpiry()
		if err != nil {
			return "", DownloadGrant{}, err
		}
	}
	grant, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     req.TenantID,
		AccountID:    req.AccountID,
		ObjectRef:    req.ObjectRef,
		Module:       req.Module,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return "", DownloadGrant{}, err
	}
	token, err := SignDownloadGrantToken(grant, s.SigningKey)
	if err != nil {
		return "", DownloadGrant{}, err
	}
	return token, grant, nil
}

// defaultDownloadExpiry 统一计算默认下载授权过期时间,避免不同模块自行决定 TTL。
func (s Service) defaultDownloadExpiry() (time.Time, error) {
	if s.DownloadGrantTTL <= 0 {
		return time.Time{}, fmt.Errorf("统一文件服务下载授权 TTL 必须大于 0")
	}
	return timex.Now().Add(s.DownloadGrantTTL), nil
}

// validateScopedResource 校验统一文件服务对象路径绑定所需的基础资源边界。
func validateScopedResource(module, resourceType, resourceID string) error {
	if strings.TrimSpace(module) == "" || strings.TrimSpace(resourceType) == "" || strings.TrimSpace(resourceID) == "" {
		return fmt.Errorf("上传规划缺少资源边界")
	}
	return nil
}

// validateUploadFileName 统一校验文件名只能是单段名称,禁止把路径语义留给业务层兜底。
func validateUploadFileName(fileName string, required bool) (string, error) {
	clean := strings.TrimSpace(fileName)
	if clean == "" {
		if required {
			return "", fmt.Errorf("上传文件名不能为空")
		}
		return "", nil
	}
	if strings.Contains(clean, "/") || strings.Contains(clean, "\\") || filepath.Base(clean) != clean {
		return "", fmt.Errorf("上传文件名非法")
	}
	if clean == "." || clean == ".." {
		return "", fmt.Errorf("上传文件名非法")
	}
	return clean, nil
}
