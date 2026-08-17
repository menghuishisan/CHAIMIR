// teaching service_cover 文件实现课程封面上传与投放授权。
//
// 封面与校徽的载体不同,原因是可见性不同:封面只对能看到该课程的人可见,读者一定已登录,
// 所以走统一文件服务的短时投放授权;校徽最主要的展示位是尚未登录的登录页,没有账号可绑,
// 只能由品牌读取口内联下发(见 identity/service_brand.go)。
//
// 上传不绑课程 id:新建课程时还没有 id,所以先把图落到**上传教师自己的暂存位**并拿到
// object_ref,再随 POST /courses 或 PATCH /courses/{id} 提交,由服务端搬到该课程的正式位。
//
// 暂存位的文件名按内容类型归一(cover.jpg / cover.png / cover.webp),因此同一教师反复
// 上传只会互相覆盖同一个对象:表单被放弃时最多留下三个小对象,且下次上传就被覆盖,
// 不会随放弃次数增长,也就不需要一个跨模块扫描对象存储的清理任务
// (那种任务要么让地基层认识业务表、要么在每个模块各写一份,两者都违反分层与统一原则)。
package teaching

import (
	"bytes"
	"context"
	"path"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// coverCanonicalNames 把内容嗅探出的 MIME 映射为暂存位的归一文件名。
// 用内容而不是客户端文件名决定落点:同一教师的暂存位因此只有固定的几个 key,
// 反复上传互相覆盖,不会按原始文件名无限铺开。
var coverCanonicalNames = map[string]string{
	"image/jpeg": "cover.jpg",
	"image/png":  "cover.png",
	"image/webp": "cover.webp",
}

// CourseCoverUploadRequest 描述已读入内存的封面文件。
type CourseCoverUploadRequest struct {
	FileName    string
	ContentType string
	Content     []byte
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
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		Module:       teachingModuleName,
		ResourceType: courseCoverResourceType,
		// 资源段用上传教师的账号:课程还不存在时也要有互不冲突的暂存位,
		// 否则同租户两位教师上传会互相冲掉对方的图。
		ResourceID:      ids.Format(id.AccountID),
		FileName:        fileName,
		ContentType:     contentType,
		Size:            int64(len(req.Content)),
		MaxBytes:        s.coverMaxBytes,
		ExpectedBucket:  s.storage.BucketAttach(),
		AllowedFileName: true,
		Content:         req.Content,
		KindValidator:   upload.BrandImageKindValid,
		ScanPolicy:      s.materialScanPolicy,
	})
	if err != nil {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return CourseCoverUploadDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.course.cover.upload", auditTargetCourse, 0, map[string]any{"file_name": plan.FileName, "size": plan.Size}); err != nil {
		return CourseCoverUploadDTO{}, err
	}
	return CourseCoverUploadDTO{ObjectRef: plan.ObjectRef, FileName: plan.FileName, Size: plan.Size}, nil
}

// IssueCourseCoverAccess 校验课程可读权限并签发封面投放授权。
// 用 mode=stream 而不是一次性 download:封面由 <img src> 直接取件,列表页同屏多张、
// 且浏览器可能重复请求同一地址,一次性令牌会在第一次请求后就作废。
//
// 本入口**不写审计**:它随课程页面渲染自动触发,而 audit_log 记的是关键操作
// (docs/01-身份与租户/02-数据模型.md §7.3)。给每次页面渲染记一行,会用装饰图的
// 加载记录淹没真正需要追溯的操作。封面的上传与替换仍然记审计。
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
		TenantID:     id.TenantID,
		AccountID:    id.AccountID,
		ObjectRef:    course.CoverRef,
		Module:       teachingModuleName,
		ResourceType: courseCoverResourceType,
		ResourceID:   ids.Format(courseID),
		Mode:         storage.DownloadModeStream,
	})
	if err != nil {
		return CourseCoverAccessDTO{}, apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	return CourseCoverAccessDTO{Token: token, Mode: grant.Mode, ExpiresAt: grant.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")}, nil
}

// promoteCourseCover 把暂存位的封面搬到该课程的正式位,返回要落库的对象引用。
//
// 三种入参各自明确,不做猜测:空串表示不设封面;等于该课程当前引用表示这次没动封面
// (编辑表单会把已有引用原样回传);其余只接受**本人暂存位**的引用 —— 否则教师就能把
// 封面指向他人上传的对象或别的资源类型,而正式位一旦被替换又会连带删掉那个对象。
func (s *Service) promoteCourseCover(ctx context.Context, tenantID, accountID, courseID int64, coverRef, currentRef string) (string, error) {
	coverRef = strings.TrimSpace(coverRef)
	if coverRef == "" || coverRef == strings.TrimSpace(currentRef) {
		return coverRef, nil
	}
	ref, err := storage.ParseObjectRef(coverRef)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	stagePrefix, err := s.courseCoverStagePrefix(tenantID, accountID)
	if err != nil {
		return "", err
	}
	fileName := path.Base(ref.Key)
	// 必须正好是暂存位下的归一文件名:多一层路径就说明它是某门课的正式位对象。
	if ref.Bucket != s.storage.BucketAttach() || ref.Key != stagePrefix+"/"+fileName || !coverCanonicalName(fileName) {
		return "", apperr.ErrTeachingCourseCoverInvalid
	}
	targetKey, err := storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType,
		ids.Format(accountID), ids.Format(courseID), fileName)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	if _, err := s.storage.CopyObject(ctx, ref.Bucket, ref.Key, s.storage.BucketAttach(), targetKey); err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	promoted, err := storage.ObjectRefString(s.storage.BucketAttach(), targetKey)
	if err != nil {
		return "", apperr.ErrTeachingCourseCoverInvalid.WithCause(err)
	}
	// 暂存对象已经搬走,留着只会在下次上传前占位;删不掉不影响本次结果,只记日志。
	if err := s.storage.Delete(ctx, ref.Bucket, ref.Key); err != nil {
		logging.ErrorContext(ctx, "清理课程封面暂存对象失败", err.Error())
	}
	return promoted, nil
}

// courseCoverStagePrefix 返回某位教师的封面暂存位前缀。
func (s *Service) courseCoverStagePrefix(tenantID, accountID int64) (string, error) {
	prefix, err := storage.ObjectKey(tenantID, teachingModuleName, courseCoverResourceType, ids.Format(accountID))
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

// discardCourseCoverObject 删除已确认不再被引用的封面对象,失败只记日志。
// 只删落在本人受控封面前缀内的对象:引用一旦被改写到别处,宁可留下垃圾也不误删。
func (s *Service) discardCourseCoverObject(ctx context.Context, tenantID, teacherID int64, objectRef string) {
	objectRef = strings.TrimSpace(objectRef)
	if objectRef == "" {
		return
	}
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		logging.ErrorContext(ctx, "清理课程封面时对象引用无效", err.Error())
		return
	}
	prefix, err := s.courseCoverStagePrefix(tenantID, teacherID)
	if err != nil {
		logging.ErrorContext(ctx, "清理课程封面时无法生成受控封面路径", err.Error())
		return
	}
	if ref.Bucket != s.storage.BucketAttach() || !strings.HasPrefix(ref.Key, prefix+"/") {
		logging.ErrorContext(ctx, "清理课程封面时对象不在受控封面路径", "bucket 或 key 不符合当前封面边界")
		return
	}
	if err := s.storage.Delete(ctx, ref.Bucket, ref.Key); err != nil {
		logging.ErrorContext(ctx, "清理课程封面对象失败", err.Error())
	}
}
