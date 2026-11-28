// 课程封面经教师暂存、课程独立对象和短时授权投放；对象引用一旦签发就不再覆盖。
package teaching

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
)

// 文件名由实际内容类型决定，上传编号负责隔离不同表单。
var coverCanonicalNames = map[string]string{
	"image/jpeg": "cover.jpg",
	"image/png":  "cover.png",
	"image/webp": "cover.webp",
}

// UploadCourseCover 校验封面并写入上传教师的暂存位,只返回对象引用,不改课程记录。
func (s *Service) UploadCourseCover(ctx context.Context, req CourseCoverUploadRequest) (CourseCoverUploadDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseCoverUploadDTO{}, err
	}
	if !upload.BrandImageKindValid(req.FileName, req.ContentType, req.Content) {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid
	}
	if upload.CheckSize(int64(len(req.Content)), s.coverMaxBytes) != upload.SizeOK {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverTooLarge
	}
	// 落点用内容嗅探出的类型决定,不沿用客户端声明:声明值只用于上面的一致性校验。
	contentType := upload.BrandImageContentType(req.Content)
	fileName, ok := coverCanonicalNames[contentType]
	if !ok {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:          id.TenantID,
		AccountID:         id.AccountID,
		Module:            teachingModuleName,
		ResourceType:      courseCoverResourceType,
		ResourceID:        "stage",
		NestedResourceIDs: []string{ids.Format(id.AccountID), ids.Format(s.ids.Generate())},
		FileName:          fileName,
		ContentType:       contentType,
		MaxBytes:          s.coverMaxBytes,
		ExpectedBucket:    s.storage.BucketAttach(),
		AllowedFileName:   true,
		Content:           req.Content,
		KindValidator:     upload.BrandImageKindValid,
		ScanPolicy:        s.materialScanPolicy,
	})
	if err != nil {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if err := s.storage.PutTemporary(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(errors.Join(err, s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, 0, plan.ObjectRef)))
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.course.cover.upload", auditTargetCourse, 0, map[string]any{"file_name": plan.FileName, "size": plan.Size}); err != nil {
		return CourseCoverUploadDTO{}, errors.Join(err, s.discardCourseCoverObject(ctx, id.TenantID, id.AccountID, 0, plan.ObjectRef))
	}
	return CourseCoverUploadDTO{ObjectRef: plan.ObjectRef, FileName: plan.FileName, Size: plan.Size}, nil
}

// IssueCourseCoverAccess 校验课程可读权限并签发可重复读取的图片流授权。
// 页面展示不写审计；上传和课程变更才是需要追溯的操作。
func (s *Service) IssueCourseCoverAccess(ctx context.Context, courseID int64) (CourseCoverAccessDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseCoverAccessDTO{}, err
	}
	var course Course
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		course, err = tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		// 只要求「能看到这门课」,不区分师生:授课教师与课程成员都该看到同一张封面。
		_, err = s.courseReadAccess(ctx, tx, id.TenantID, courseID, id.AccountID)
		return err
	}); err != nil {
		return CourseCoverAccessDTO{}, mapCourseError(err)
	}
	if strings.TrimSpace(course.CoverRef) == "" {
		return CourseCoverAccessDTO{}, apperr.ErrTeachingCourseCoverMissing
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:         id.TenantID,
		ResourceTenantID: id.TenantID,
		AccountID:        id.AccountID,
		ObjectRef:        course.CoverRef,
		Module:           teachingModuleName,
		ResourceType:     courseCoverResourceType,
		ResourceID:       ids.Format(courseID),
		Mode:             storage.DownloadModeStream,
	})
	if err != nil {
		return CourseCoverAccessDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	return CourseCoverAccessDTO{Token: token, Mode: grant.Mode, ExpiresAt: timex.RFC3339OrEmpty(grant.ExpiresAt)}, nil
}

// promoteCourseCover 只复制本人暂存对象；提交前保留暂存源，且永不覆盖当前封面。
func (s *Service) promoteCourseCover(ctx context.Context, tenantID, accountID, courseID int64, coverRef, currentRef string) (string, error) {
	coverRef = strings.TrimSpace(coverRef)
	if coverRef == "" || coverRef == strings.TrimSpace(currentRef) {
		return coverRef, nil
	}
	ref, err := storage.ParseObjectRef(coverRef)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	currentPrefix, err := storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType, ids.Format(courseID))
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if ref.Bucket == s.storage.BucketAttach() && coverObjectInPrefix(ref.Key, currentPrefix) {
		return "", apperr.ErrConflict
	}
	stagePrefix, err := s.courseCoverStagePrefix(tenantID, accountID)
	if err != nil {
		return "", err
	}
	fileName := path.Base(ref.Key)
	if ref.Bucket != s.storage.BucketAttach() || !coverObjectInPrefix(ref.Key, stagePrefix) {
		return "", apperr.ErrTeachingCourseCoverInvalid
	}
	targetKey, err := storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType,
		ids.Format(courseID), ids.Format(s.ids.Generate()), fileName)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	promoted, err := storage.ObjectRefString(s.storage.BucketAttach(), targetKey)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if _, err := s.storage.CopyObject(ctx, ref.Bucket, ref.Key, s.storage.BucketAttach(), targetKey); err != nil {
		// 复制成功后的元数据读取也可能失败，目标仍须补偿。
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(errors.Join(err, s.discardCourseCoverObject(ctx, tenantID, accountID, courseID, promoted)))
	}
	return promoted, nil
}

// courseCoverStagePrefix 返回某位教师的封面暂存位前缀。
func (s *Service) courseCoverStagePrefix(tenantID, accountID int64) (string, error) {
	prefix, err := storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType, "stage", ids.Format(accountID))
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	return prefix, nil
}

// coverCanonicalName 判断文件名是否是封面的归一名称之一。
func coverCanonicalName(fileName string) bool {
	for _, name := range coverCanonicalNames {
		if fileName == name {
			return true
		}
	}
	return false
}

func coverObjectInPrefix(key, prefix string) bool {
	suffix, ok := strings.CutPrefix(key, prefix+"/")
	parts := strings.Split(suffix, "/")
	if !ok || len(parts) != 2 || !coverCanonicalName(parts[1]) {
		return false
	}
	_, ok = ids.Parse(parts[0])
	return ok
}

// courseID 为零时只清理本人暂存对象，否则只清理指定课程的正式对象。
func (s *Service) discardCourseCoverObject(ctx context.Context, tenantID, teacherID, courseID int64, objectRef string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	defer cancel()
	objectRef = strings.TrimSpace(objectRef)
	if objectRef == "" {
		return nil
	}
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return fmt.Errorf("清理课程封面时对象引用无效: %w", err)
	}
	prefix, err := s.courseCoverStagePrefix(tenantID, teacherID)
	if courseID > 0 {
		prefix, err = storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType, ids.Format(courseID))
	}
	if err != nil {
		return err
	}
	if ref.Bucket != s.storage.BucketAttach() || !coverObjectInPrefix(ref.Key, prefix) {
		return fmt.Errorf("清理课程封面时对象不在受控封面路径")
	}
	return s.storage.Delete(ctx, ref.Bucket, ref.Key)
}
