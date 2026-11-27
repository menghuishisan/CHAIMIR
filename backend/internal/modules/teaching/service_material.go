package teaching

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// UploadLessonMaterial 上传材料并只在 lesson.content_ref 中保存对象引用。
func (s *Service) UploadLessonMaterial(ctx context.Context, lessonID int64, req LessonMaterialUploadRequest) (LessonDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	contentType, _, err := mime.ParseMediaType(req.ContentType)
	if err != nil {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	if len(req.Content) == 0 || !upload.CourseMaterialKindValid(req.FileName, req.ContentType, req.Content) {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid
	}

	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetLesson(ctx, id.TenantID, lessonID)
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

	lessonType := LessonContentAttachment
	if upload.CourseVideoKindValid(req.FileName, req.ContentType, req.Content) {
		lessonType = LessonContentVideo
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:          id.TenantID,
		AccountID:         id.AccountID,
		Module:            teachingModuleName,
		ResourceType:      lessonMaterialResourceType,
		ResourceID:        ids.Format(lessonID),
		NestedResourceIDs: []string{ids.Format(s.ids.Generate())},
		FileName:          req.FileName,
		ContentType:       contentType,
		Size:              int64(len(req.Content)),
		MaxBytes:          s.materialMaxBytes,
		ExpectedBucket:    s.storage.BucketAttach(),
		AllowedFileName:   true,
		Content:           req.Content,
		KindValidator:     upload.CourseMaterialKindValid,
		ScanPolicy:        s.materialScanPolicy,
	})
	if err != nil {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return LessonDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(errors.Join(err, s.discardMaterialObjects(ctx, storage.ObjectRef{Bucket: plan.Bucket, Key: plan.Key})))
	}

	var out LessonDTO
	var previousObjectRef string
	commitAttempted := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		latest, err := tx.GetLessonForUpdate(ctx, id.TenantID, lessonID)
		if err != nil {
			return err
		}
		chapter, err := tx.GetChapter(ctx, id.TenantID, latest.ChapterID)
		if err != nil {
			return err
		}
		course, err := tx.GetCourse(ctx, id.TenantID, chapter.CourseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		previousObjectRef = strings.TrimSpace(jsonx.StringFromAny(latest.ContentRef["object_ref"]))
		latest.ContentType = lessonType
		latest.ContentRef = map[string]any{
			"object_ref":   plan.ObjectRef,
			"file_name":    plan.FileName,
			"size":         plan.Size,
			"content_type": contentType,
		}
		updated, err := tx.UpdateLesson(ctx, latest)
		if err != nil {
			return err
		}
		out, err = lessonDTO(updated)
		if err != nil {
			return err
		}
		if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.lesson.material.upload", auditTargetLesson, lessonID, map[string]any{"file_name": plan.FileName, "content_type": contentType}); err != nil {
			return err
		}
		commitAttempted = true
		return nil
	}); err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error {
			return s.discardMaterialObjects(ctx, storage.ObjectRef{Bucket: plan.Bucket, Key: plan.Key})
		})
		return LessonDTO{}, mapCourseError(err)
	}
	if previousObjectRef != "" && previousObjectRef != plan.ObjectRef {
		oldRef, parseErr := storage.ParseObjectRef(previousObjectRef)
		if parseErr != nil {
			logging.ErrorContext(ctx, "替换课时材料时旧对象引用无效", parseErr.Error())
		} else {
			prefix, prefixErr := storage.ObjectKey(id.TenantID, teachingModuleName, lessonMaterialResourceType, ids.Format(lessonID))
			if prefixErr != nil {
				logging.ErrorContext(ctx, "替换课时材料时无法生成受控材料路径", prefixErr.Error())
			} else if oldRef.Bucket != s.storage.BucketAttach() || !strings.HasPrefix(oldRef.Key, prefix+"/") {
				logging.ErrorContext(ctx, "替换课时材料时旧对象不在受控材料路径", "bucket 或 key 不符合当前课时材料边界")
			} else if deleteErr := s.discardMaterialObjects(ctx, oldRef); deleteErr != nil {
				logging.ErrorContext(ctx, "清理已替换课时材料失败", deleteErr.Error())
			}
		}
	}
	return out, nil
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
	contentType, _, err := mime.ParseMediaType(jsonx.StringFromAny(lesson.ContentRef["content_type"]))
	if err != nil {
		return LessonMaterialAccessDTO{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	size := jsonx.Int64FromAny(lesson.ContentRef["size"], 0)
	if (lesson.ContentType != LessonContentVideo && lesson.ContentType != LessonContentAttachment) || objectRef == "" || fileName == "" || filepath.Base(fileName) != fileName || size <= 0 || contentType == "" {
		return LessonMaterialAccessDTO{}, apperr.ErrTeachingLessonInvalid
	}
	mode := storage.DownloadModeDownload
	if lesson.ContentType == LessonContentVideo {
		mode = storage.DownloadModeStream
	}
	token, grant, err := s.files.IssueDownloadGrant(storage.IssueDownloadGrantRequest{
		TenantID:         id.TenantID,
		ResourceTenantID: id.TenantID,
		AccountID:        id.AccountID,
		ObjectRef:        objectRef,
		Module:           teachingModuleName,
		ResourceType:     lessonMaterialResourceType,
		ResourceID:       ids.Format(lessonID),
		Mode:             mode,
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
	return LessonMaterialAccessDTO{Token: token, Mode: grant.Mode, FileName: fileName, Size: size, ContentType: contentType, ExpiresAt: timex.RFC3339OrEmpty(grant.ExpiresAt)}, nil
}

// copyLessonMaterial 在源课时锁内复制，目标对象只属于新课时。
func (s *Service) copyLessonMaterial(ctx context.Context, source Lesson, target *Lesson) (storage.ObjectRef, error) {
	if source.ContentType != LessonContentVideo && source.ContentType != LessonContentAttachment {
		return storage.ObjectRef{}, nil
	}
	raw := jsonx.StringFromAny(source.ContentRef["object_ref"])
	if raw == "" {
		return storage.ObjectRef{}, nil
	}
	ref, err := storage.ParseObjectRef(raw)
	if err != nil {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	prefix, err := storage.ObjectKey(source.TenantID, teachingModuleName, lessonMaterialResourceType, ids.Format(source.ID))
	if err != nil {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	fileName := jsonx.StringFromAny(source.ContentRef["file_name"])
	size := jsonx.Int64FromAny(source.ContentRef["size"], 0)
	if _, _, err := mime.ParseMediaType(jsonx.StringFromAny(source.ContentRef["content_type"])); err != nil {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	if ref.Bucket != s.storage.BucketAttach() || !strings.HasPrefix(ref.Key, prefix+"/") || fileName == "" || filepath.Base(fileName) != fileName || size <= 0 {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid
	}
	key, err := storage.ObjectKey(target.TenantID, teachingModuleName, lessonMaterialResourceType, ids.Format(target.ID), ids.Format(s.ids.Generate()), fileName)
	if err != nil {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	objectRef, err := storage.ObjectRefString(s.storage.BucketAttach(), key)
	if err != nil {
		return storage.ObjectRef{}, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	copied := storage.ObjectRef{Bucket: s.storage.BucketAttach(), Key: key}
	copiedSize, err := s.storage.CopyObject(ctx, ref.Bucket, ref.Key, copied.Bucket, copied.Key)
	if err != nil {
		return copied, apperr.ErrTeachingLessonInvalid.WithCause(err)
	}
	if copiedSize != size {
		return copied, apperr.ErrTeachingLessonInvalid.WithCause(fmt.Errorf("克隆材料大小与课时元数据不符: actual=%d expected=%d", copiedSize, size))
	}
	target.ContentRef["object_ref"] = objectRef
	return copied, nil
}

func (s *Service) discardMaterialObjects(ctx context.Context, refs ...storage.ObjectRef) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	defer cancel()
	var failures []error
	for _, ref := range refs {
		if err := s.storage.Delete(ctx, ref.Bucket, ref.Key); err != nil {
			logging.ErrorContext(ctx, "清理教学材料失败", err.Error(), slog.String("bucket", ref.Bucket), slog.String("key", ref.Key))
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
