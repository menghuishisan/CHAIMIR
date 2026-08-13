// teaching service_material 文件实现课时材料上传和统一文件服务授权。
package teaching

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// LessonMaterialUploadRequest 描述课时材料的已读取文件内容。
type LessonMaterialUploadRequest struct {
	FileName    string
	ContentType string
	Content     []byte
}

// LessonMaterialAccessDTO 是课时材料的短时投放授权响应。
type LessonMaterialAccessDTO struct {
	Token       string `json:"token"`
	Mode        string `json:"mode"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	ExpiresAt   string `json:"expires_at"`
}

// UploadLessonMaterial 上传材料并只在 lesson.content_ref 中保存对象引用。
func (s *Service) UploadLessonMaterial(ctx context.Context, lessonID int64, req LessonMaterialUploadRequest) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	if len(req.Content) == 0 || !upload.CourseMaterialKindValid(req.FileName, req.ContentType, req.Content) {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid
	}

	var current Lesson
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		current, err = tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, current.ChapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return err
		}
		return ensureTeacherOwned(course, id.AccountID)
	}); err != nil {
		return LessonDTO{}, mapCourseError(err)
	}

	contentType := normalizedContentType(req.ContentType)
	lessonType := LessonContentAttachment
	if upload.CourseVideoKindValid(req.FileName, req.ContentType, req.Content) {
		lessonType = LessonContentVideo
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          teachingModuleName,
		ResourceType:    lessonMaterialResourceType,
		ResourceID:      strconv.FormatInt(lessonID, 10),
		FileName:        req.FileName,
		ContentType:     contentType,
		Size:            int64(len(req.Content)),
		MaxBytes:        s.materialMaxBytes,
		ExpectedBucket:  s.storage.BucketAttach(),
		AllowedFileName: true,
		Content:         req.Content,
		KindValidator:   upload.CourseMaterialKindValid,
		ScanPolicy:      s.materialScanPolicy,
	})
	if err != nil {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}

	updated := current
	updated.ContentType = lessonType
	updated.ContentRef = map[string]any{
		"object_ref":   plan.ObjectRef,
		"file_name":    plan.FileName,
		"size":         plan.Size,
		"content_type": contentType,
	}
	var previousObjectRef string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		latest, err := tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		previousObjectRef = strings.TrimSpace(jsonx.StringFromAny(latest.ContentRef["object_ref"]))
		latest.ContentType = updated.ContentType
		latest.ContentRef = updated.ContentRef
		updated, err = tx.UpdateLesson(ctx, latest)
		return err
	}); err != nil {
		if cleanupErr := s.storage.Delete(ctx, plan.Bucket, plan.Key); cleanupErr != nil {
			logging.ErrorContext(ctx, "清理未关联课时材料失败", cleanupErr.Error())
		}
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	oldObjectRef := previousObjectRef
	if oldObjectRef != "" && oldObjectRef != plan.ObjectRef {
		oldRef, parseErr := storage.ParseObjectRef(oldObjectRef)
		if parseErr != nil {
			logging.ErrorContext(ctx, "替换课时材料时旧对象引用无效", parseErr.Error())
		} else {
			prefix, prefixErr := storage.ObjectKey(id.TenantID, teachingModuleName, lessonMaterialResourceType, strconv.FormatInt(lessonID, 10))
			if prefixErr != nil {
				logging.ErrorContext(ctx, "替换课时材料时无法生成受控材料路径", prefixErr.Error())
			} else if oldRef.Bucket != s.storage.BucketAttach() || !strings.HasPrefix(oldRef.Key, prefix+"/") {
				logging.ErrorContext(ctx, "替换课时材料时旧对象不在受控材料路径", "bucket 或 key 不符合当前课时材料边界")
			} else if deleteErr := s.storage.Delete(ctx, oldRef.Bucket, oldRef.Key); deleteErr != nil {
				logging.ErrorContext(ctx, "清理已替换课时材料失败", deleteErr.Error())
			}
		}
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.lesson.material.upload", auditTargetLesson, lessonID, map[string]any{"file_name": plan.FileName, "content_type": contentType}); err != nil {
		return LessonDTO{}, err
	}
	return lessonDTO(updated)
}

// IssueLessonMaterialAccess 校验课程可读权限并签发视频流式或附件一次性授权。
func (s *Service) IssueLessonMaterialAccess(ctx context.Context, lessonID int64) (LessonMaterialAccessDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonMaterialAccessDTO{}, err
	}
	var lesson Lesson
	var teacher bool
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		lesson, err = tx.GetLesson(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, lesson.ChapterID)
		if err != nil {
			return err
		}
		teacher, err = s.courseReadAccess(ctx, tx, id.TenantID, chapter.CourseID, id.AccountID)
		return err
	}); err != nil {
		return LessonMaterialAccessDTO{}, mapCourseError(err)
	}
	objectRef := strings.TrimSpace(jsonx.StringFromAny(lesson.ContentRef["object_ref"]))
	fileName := strings.TrimSpace(jsonx.StringFromAny(lesson.ContentRef["file_name"]))
	contentType := normalizedContentType(jsonx.StringFromAny(lesson.ContentRef["content_type"]))
	size := jsonx.Int64FromAny(lesson.ContentRef["size"], 0)
	if (lesson.ContentType != LessonContentVideo && lesson.ContentType != LessonContentAttachment) || objectRef == "" || fileName == "" || filepath.Base(fileName) != fileName || size <= 0 || contentType == "" {
		return LessonMaterialAccessDTO{}, apperr.ErrTeachingLessonInvalid
	}
	mode := storage.DownloadModeDownload
	if lesson.ContentType == LessonContentVideo {
		mode = storage.DownloadModeStream
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		ObjectRef:    objectRef,
		Module:       teachingModuleName,
		ResourceType: lessonMaterialResourceType,
		ResourceID:   strconv.FormatInt(lessonID, 10),
		Mode:         mode,
	})
	if err != nil {
		return LessonMaterialAccessDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	role := int16(contracts.RoleNumStudent)
	if teacher {
		role = contracts.RoleNumTeacher
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, role, "teaching.lesson.material.access", auditTargetLesson, lessonID, map[string]any{"mode": mode}); err != nil {
		return LessonMaterialAccessDTO{}, err
	}
	return LessonMaterialAccessDTO{Token: token, Mode: grant.Mode, FileName: fileName, Size: size, ContentType: contentType, ExpiresAt: grant.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")}, nil
}

// normalizedContentType 去掉上传 MIME 的参数,保证 lesson.content_ref 只有一个可比较的类型值。
func normalizedContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}
