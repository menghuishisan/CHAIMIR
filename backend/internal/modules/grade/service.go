// Package grade 聚合教学成绩并负责审核、申诉、预警与成绩单。
package grade

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

type objectStorage interface {
	Delete(ctx context.Context, bucket, key string) error
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketReport() string
}

type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// aggregateCache 是 M11 GPA 聚合缓存所需的最小基础层 Redis 能力。
type aggregateCache interface {
	GetBytes(ctx context.Context, key string) ([]byte, bool, error)
	SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Service 承载 M11 成绩中心业务编排。
type Service struct {
	cleanupTimeout time.Duration
	store          Store
	ids            snowflake.Generator
	audit          audit.Writer
	roles          roleReader
	teaching       contracts.TeachingReadService
	bus            eventbus.Bus
	cache          aggregateCache
	storage        objectStorage
	files          fileService
	cfg            config.GradeConfig
}

// ServiceDeps 是 M11 服务装配依赖。
type ServiceDeps struct {
	CleanupTimeout time.Duration
	Store          Store
	IDs            snowflake.Generator
	Audit          audit.Writer
	Roles          roleReader
	Teaching       contracts.TeachingReadService
	Bus            eventbus.Bus
	Cache          aggregateCache
	Storage        objectStorage
	FileService    fileService
	Config         config.GradeConfig
}

// NewService 构造 M11 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.CleanupTimeout <= 0 {
		return nil, fmt.Errorf("上传清理超时必须大于 0")
	}
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Teaching == nil || deps.Bus == nil {
		return nil, fmt.Errorf("grade service 依赖不完整")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("grade service 对象存储依赖不完整")
	}
	if deps.FileService == nil {
		return nil, fmt.Errorf("grade service 统一文件服务依赖不完整")
	}
	if deps.Cache == nil {
		return nil, fmt.Errorf("grade service 缓存依赖不完整")
	}
	if deps.Config.AppealWindowDays <= 0 || deps.Config.GradeAggregateCacheTTLSeconds <= 0 || strings.TrimSpace(deps.Config.TranscriptSigningKey) == "" || deps.Config.TranscriptMaxBytes <= 0 || deps.Config.LockOutboxBatchSize <= 0 || deps.Config.LockOutboxStaleMs <= 0 {
		return nil, fmt.Errorf("grade service 配置不完整")
	}
	return &Service{cleanupTimeout: deps.CleanupTimeout, store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, teaching: deps.Teaching, bus: deps.Bus, cache: deps.Cache, storage: deps.Storage, files: deps.FileService, cfg: deps.Config}, nil
}

func (s *Service) canReadStudent(ctx context.Context, accountID int64) (bool, error) {
	for _, role := range []string{contracts.RoleTeacher, contracts.RoleSchoolAdmin} {
		has, err := s.roles.HasRole(ctx, accountID, role)
		if err != nil {
			return false, apperr.ErrGradeForbidden.WithCause(err)
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) gradeActorRole(ctx context.Context, accountID int64) (int16, error) {
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return contracts.RoleNumSchoolAdmin, nil
	}
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleTeacher); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return contracts.RoleNumTeacher, nil
	}
	if has, err := s.roles.HasRole(ctx, accountID, contracts.RoleStudent); err != nil {
		return 0, apperr.ErrGradeForbidden.WithCause(err)
	} else if has {
		return contracts.RoleNumStudent, nil
	}
	return 0, apperr.ErrGradeForbidden
}

func (s *Service) requireStudent(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	has, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
	if err != nil {
		return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !has {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

func (s *Service) isSchoolAdmin(ctx context.Context, accountID int64) (bool, error) {
	has, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return false, apperr.ErrGradeForbidden.WithCause(err)
	}
	return has, nil
}

func (s *Service) normalizeStudentOrSchoolAdmin(ctx context.Context, studentID int64) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	if studentID == 0 {
		studentID = id.AccountID
	}
	if studentID <= 0 {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	if studentID == id.AccountID {
		hasStudent, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleStudent)
		if err != nil {
			return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
		}
		if hasStudent {
			return id, nil
		}
	}
	hasAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !hasAdmin {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

func (s *Service) normalizeReadableStudent(ctx context.Context, studentID int64) (tenant.Identity, int64, error) {
	id, err := s.normalizeStudentOrSchoolAdmin(ctx, studentID)
	if err != nil {
		return tenant.Identity{}, 0, err
	}
	if studentID == 0 {
		studentID = id.AccountID
	}
	return id, studentID, nil
}

func requireTenantUser(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

func (s *Service) requireTeacherAdmin(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	allowed, err := s.canReadStudent(ctx, id.AccountID)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !allowed {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}

func (s *Service) requireSchoolAdmin(ctx context.Context) (tenant.Identity, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return tenant.Identity{}, err
	}
	has, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return tenant.Identity{}, apperr.ErrGradeForbidden.WithCause(err)
	}
	if !has {
		return tenant.Identity{}, apperr.ErrGradeForbidden
	}
	return id, nil
}
