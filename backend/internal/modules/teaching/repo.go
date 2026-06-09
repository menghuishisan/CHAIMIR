// M6 数据访问层:封装 teaching 自有表的 sqlc 事务入口与 RLS 注入。
package teaching

import (
	"context"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// repo 是 M6 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// teachingStatsRow 是 repo 返回给 service 的 M6 自有统计原始数据。
type teachingStatsRow struct {
	TenantID            int64
	CourseCount         int64
	ActiveCourseCount   int64
	LearningDurationSec int64
}

// newRepo 构造 M6 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M6 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从请求上下文读取租户并注入 RLS。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供事件与 contracts 内部入口使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inPrivileged 使用受控特权连接读取 M6 自有表的跨租户待派发任务。
func (r *repo) inPrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// upsertLessonProgress 写入学生课时进度,由 service 负责先完成成员校验。
func (r *repo) upsertLessonProgress(ctx context.Context, tenantID, progressID, lessonID, studentID int64, req ProgressRequest) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpsertLessonProgress(ctx, sqlcgen.UpsertLessonProgressParams{
			ID: progressID, TenantID: tenantID, LessonID: lessonID, StudentID: studentID,
			Status: req.Status, VideoPos: pgtypex.Int4When(req.VideoPos, req.VideoPos > 0), DurationSec: req.DurationSec,
		})
		if db.IsNoRows(err) {
			return apperr.ErrProgressForbidden
		}
		return err
	})
}

// listLessonProgressByCourse 读取课程全部课时进度行供 service 聚合统计。
func (r *repo) listLessonProgressByCourse(ctx context.Context, courseID int64) ([]ProgressSnapshot, error) {
	var rows []sqlcgen.LessonProgress
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListLessonProgressByCourse(ctx, courseID)
		rows = found
		return e
	})
	return progressSnapshotsFromRows(rows), err
}

// listLessonProgressByCourseAndStudent 读取单个学生在课程内的进度行供 service 聚合。
func (r *repo) listLessonProgressByCourseAndStudent(ctx context.Context, courseID, studentID int64) ([]ProgressSnapshot, error) {
	var rows []sqlcgen.LessonProgress
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListLessonProgressByCourseAndStudent(ctx, sqlcgen.ListLessonProgressByCourseAndStudentParams{CourseID: courseID, StudentID: studentID})
		rows = found
		return e
	})
	return progressSnapshotsFromRows(rows), err
}

// listDiscussionPosts 分页读取课程讨论帖。
func (r *repo) listDiscussionPosts(ctx context.Context, courseID int64, size, offset int) ([]PostDTO, error) {
	var rows []sqlcgen.DiscussionPost
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListDiscussionPosts(ctx, sqlcgen.ListDiscussionPostsParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32(offset)})
		rows = found
		return e
	})
	return postDTOsFromRows(rows), err
}

// createDiscussionPost 写入讨论帖或回复内容。
func (r *repo) createDiscussionPost(ctx context.Context, tenantID, postID, courseID, parentID, authorID int64, content string) (PostDTO, error) {
	var row sqlcgen.DiscussionPost
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateDiscussionPost(ctx, sqlcgen.CreateDiscussionPostParams{
			ID: postID, TenantID: tenantID, CourseID: courseID, ParentID: pgtypex.Int8(parentID), AuthorID: authorID, Content: content,
		})
		row = created
		return e
	})
	return postDTOFromRow(row), err
}

// incrementPostLike 增加讨论帖点赞数并把缺失或越权统一映射为点赞非法。
func (r *repo) incrementPostLike(ctx context.Context, postID int64, isPlatform bool, actorID int64) (PostDTO, error) {
	var row sqlcgen.DiscussionPost
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.IncrementPostLike(ctx, sqlcgen.IncrementPostLikeParams{ID: postID, IsPlatform: isPlatform, ActorID: actorID})
		if db.IsNoRows(e) {
			return apperr.ErrDiscussionLikeInvalid
		}
		row = updated
		return e
	})
	return postDTOFromRow(row), err
}

// togglePostPin 切换讨论帖置顶并把缺失或越权统一映射为管理非法。
func (r *repo) togglePostPin(ctx context.Context, postID int64, isPlatform bool, actorID int64) (PostDTO, error) {
	var row sqlcgen.DiscussionPost
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.TogglePostPin(ctx, sqlcgen.TogglePostPinParams{ID: postID, IsPlatform: isPlatform, ActorID: actorID})
		if db.IsNoRows(e) {
			return apperr.ErrDiscussionModerationInvalid
		}
		row = updated
		return e
	})
	return postDTOFromRow(row), err
}

// softDeletePost 软删讨论帖并把缺失或越权统一映射为管理非法。
func (r *repo) softDeletePost(ctx context.Context, postID int64, isPlatform bool, actorID int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.SoftDeletePost(ctx, sqlcgen.SoftDeletePostParams{ID: postID, IsPlatform: isPlatform, ActorID: actorID})
		if db.IsNoRows(err) {
			return apperr.ErrDiscussionModerationInvalid
		}
		return err
	})
}

// listAnnouncements 分页读取课程公告。
func (r *repo) listAnnouncements(ctx context.Context, courseID int64, size, offset int) ([]AnnouncementDTO, error) {
	var rows []sqlcgen.Announcement
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{CourseID: courseID, LimitCount: int32(size), OffsetCount: int32(offset)})
		rows = found
		return e
	})
	return announcementDTOsFromRows(rows), err
}

// createAnnouncement 写入课程公告内容。
func (r *repo) createAnnouncement(ctx context.Context, tenantID, announcementID, courseID int64, title, content string) (AnnouncementDTO, error) {
	var row sqlcgen.Announcement
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{
			ID: announcementID, TenantID: tenantID, CourseID: courseID, Title: title, Content: content,
		})
		row = created
		return e
	})
	return announcementDTOFromRow(row), err
}

// toggleAnnouncementPin 切换公告置顶并把缺失或越权统一映射为公告管理非法。
func (r *repo) toggleAnnouncementPin(ctx context.Context, announcementID int64, isPlatform bool, actorID int64) (AnnouncementDTO, error) {
	var row sqlcgen.Announcement
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.ToggleAnnouncementPin(ctx, sqlcgen.ToggleAnnouncementPinParams{ID: announcementID, IsPlatform: isPlatform, ActorID: actorID})
		if db.IsNoRows(e) {
			return apperr.ErrAnnouncementModerationInvalid
		}
		row = updated
		return e
	})
	return announcementDTOFromRow(row), err
}

// upsertCourseReview 写入学生课程评价,由 service 先完成学生成员校验。
func (r *repo) upsertCourseReview(ctx context.Context, reviewID, tenantID, courseID, studentID int64, rating int16, comment string) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpsertCourseReview(ctx, sqlcgen.UpsertCourseReviewParams{
			ID: reviewID, TenantID: tenantID, CourseID: courseID, StudentID: studentID, Rating: rating, Comment: pgtypex.Text(comment),
		})
		if db.IsNoRows(err) {
			return apperr.ErrReviewForbidden
		}
		return err
	})
}

// createChapter 写入课程章节。
func (r *repo) createChapter(ctx context.Context, tenantID, chapterID, courseID int64, req ChapterRequest) (ChapterDTO, error) {
	var row sqlcgen.Chapter
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateChapter(ctx, sqlcgen.CreateChapterParams{
			ID: chapterID, TenantID: tenantID, CourseID: courseID, Title: req.Title, Sort: req.Sort,
		})
		row = created
		return e
	})
	return chapterDTOFromRow(row), err
}

// listChaptersByCourse 查询课程章节列表。
func (r *repo) listChaptersByCourse(ctx context.Context, courseID int64) ([]ChapterDTO, error) {
	var rows []sqlcgen.Chapter
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListChaptersByCourse(ctx, courseID)
		rows = found
		return e
	})
	out := make([]ChapterDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, chapterDTOFromRow(row))
	}
	return out, err
}

// updateChapter 更新章节并把未命中映射为课程不存在。
func (r *repo) updateChapter(ctx context.Context, chapterID int64, req ChapterRequest) (ChapterDTO, error) {
	var row sqlcgen.Chapter
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateChapter(ctx, sqlcgen.UpdateChapterParams{ID: chapterID, Title: req.Title, Sort: req.Sort})
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = updated
		return e
	})
	return chapterDTOFromRow(row), err
}

// softDeleteChapter 软删章节并把未命中映射为课程不存在。
func (r *repo) softDeleteChapter(ctx context.Context, chapterID int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.SoftDeleteChapter(ctx, chapterID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	})
}

// createLesson 写入课时内容引用。
func (r *repo) createLesson(ctx context.Context, tenantID, lessonID, chapterID int64, req LessonRequest, contentRef []byte) (LessonDTO, error) {
	var row sqlcgen.Lesson
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateLesson(ctx, sqlcgen.CreateLessonParams{
			ID: lessonID, TenantID: tenantID, ChapterID: chapterID, Title: req.Title,
			ContentType: req.ContentType, ContentRef: contentRef, Sort: req.Sort,
		})
		row = created
		return e
	})
	return lessonDTOFromRow(row), err
}

// listLessonsByChapter 查询章节课时列表。
func (r *repo) listLessonsByChapter(ctx context.Context, chapterID int64) ([]LessonDTO, error) {
	var rows []sqlcgen.Lesson
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListLessonsByChapter(ctx, chapterID)
		rows = found
		return e
	})
	return lessonDTOsFromRows(rows), err
}

// updateLesson 更新课时基础信息与内容引用。
func (r *repo) updateLesson(ctx context.Context, lessonID int64, req LessonRequest, contentRef []byte) (LessonDTO, error) {
	var row sqlcgen.Lesson
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateLesson(ctx, sqlcgen.UpdateLessonParams{
			ID: lessonID, Title: req.Title, ContentType: req.ContentType, ContentRef: contentRef, Sort: req.Sort,
		})
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = updated
		return e
	})
	return lessonDTOFromRow(row), err
}

// updateLessonContent 只更新课时内容引用。
func (r *repo) updateLessonContent(ctx context.Context, lessonID int64, req LessonContentRequest, contentRef []byte) (LessonDTO, error) {
	var row sqlcgen.Lesson
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateLessonContent(ctx, sqlcgen.UpdateLessonContentParams{
			ID: lessonID, ContentType: req.ContentType, ContentRef: contentRef,
		})
		row = updated
		return e
	})
	return lessonDTOFromRow(row), err
}

// softDeleteLesson 软删课时并把未命中映射为课程不存在。
func (r *repo) softDeleteLesson(ctx context.Context, lessonID int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.SoftDeleteLesson(ctx, lessonID)
		if db.IsNoRows(err) {
			return apperr.ErrCourseNotFound
		}
		return err
	})
}

// listCourses 查询教师或学生课程列表,角色判断由 service 完成后传入。
func (r *repo) listCourses(ctx context.Context, accountID int64, status int16, size, offset int, studentView bool) ([]CourseDTO, error) {
	var rows []sqlcgen.Course
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		if studentView {
			rows, e = q.ListStudentCourses(ctx, sqlcgen.ListStudentCoursesParams{
				StudentID: accountID, Status: pgtypex.Int2(status), LimitCount: int32(size), OffsetCount: int32(offset),
			})
		} else {
			rows, e = q.ListTeacherCourses(ctx, sqlcgen.ListTeacherCoursesParams{
				TeacherID: accountID, Status: pgtypex.Int2(status), LimitCount: int32(size), OffsetCount: int32(offset),
			})
		}
		return e
	})
	return courseDTOsFromRows(rows), err
}

// createCourse 写入课程草稿。
func (r *repo) createCourse(ctx context.Context, tenantID, courseID, teacherID int64, req CourseRequest, schedule []byte, inviteCode string) (CourseDTO, error) {
	credits, err := pgtypex.Numeric(req.Credits)
	if err != nil {
		return CourseDTO{}, err
	}
	var row sqlcgen.Course
	err = r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		created, e := q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			ID: courseID, TenantID: tenantID, TeacherID: teacherID, Name: req.Name,
			Description: req.Description, Type: req.Type, Difficulty: req.Difficulty, CoverUrl: pgtypex.Text(req.CoverURL),
			Semester: req.Semester, Credits: credits, Schedule: schedule, InviteCode: inviteCode,
			Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate,
		})
		row = created
		return e
	})
	return courseDTOFromRow(row), err
}

// updateCourse 更新课程基础信息并把未命中映射为课程不存在。
func (r *repo) updateCourse(ctx context.Context, courseID int64, req CourseRequest, schedule []byte) (CourseDTO, error) {
	credits, err := pgtypex.Numeric(req.Credits)
	if err != nil {
		return CourseDTO{}, err
	}
	var row sqlcgen.Course
	err = r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateCourse(ctx, sqlcgen.UpdateCourseParams{
			ID: courseID, Name: req.Name, Description: req.Description, Type: req.Type, Difficulty: req.Difficulty,
			CoverUrl: pgtypex.Text(req.CoverURL), Semester: req.Semester, Credits: credits, Schedule: schedule,
		})
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = updated
		return e
	})
	return courseDTOFromRow(row), err
}

// cloneCourseWithStructure 在一个事务内复制课程基础信息、章节和课时结构。
func (r *repo) cloneCourseWithStructure(ctx context.Context, tenantID, newCourseID, teacherID, sourceCourseID int64, cloneName, inviteCode string, nextID func() int64) (CourseDTO, error) {
	var row sqlcgen.Course
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		source, e := q.GetCourseByID(ctx, sourceCourseID)
		if e != nil {
			return e
		}
		created, e := q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			ID: newCourseID, TenantID: tenantID, TeacherID: teacherID, Name: cloneName,
			Description: source.Description, Type: source.Type, Difficulty: source.Difficulty, CoverUrl: source.CoverUrl,
			Semester: source.Semester, Credits: source.Credits, Schedule: source.Schedule, InviteCode: inviteCode,
			Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate,
		})
		if e != nil {
			return e
		}
		row = created

		// 克隆只复制课程内容结构,成员、提交、进度和成绩必须保持新课程独立。
		chapters, e := q.ListChaptersByCourse(ctx, sourceCourseID)
		if e != nil {
			return e
		}
		for _, chapter := range chapters {
			newChapter, e := q.CreateChapter(ctx, sqlcgen.CreateChapterParams{
				ID: nextID(), TenantID: tenantID, CourseID: newCourseID, Title: chapter.Title, Sort: chapter.Sort,
			})
			if e != nil {
				return e
			}
			lessons, e := q.ListLessonsByChapter(ctx, chapter.ID)
			if e != nil {
				return e
			}
			for _, lesson := range lessons {
				if _, e := q.CreateLesson(ctx, sqlcgen.CreateLessonParams{
					ID: nextID(), TenantID: tenantID, ChapterID: newChapter.ID, Title: lesson.Title,
					ContentType: lesson.ContentType, ContentRef: lesson.ContentRef, Sort: lesson.Sort,
				}); e != nil {
					return e
				}
			}
		}
		return nil
	})
	return courseDTOFromRow(row), err
}

// updateCourseInviteCode 刷新课程邀请码并把未命中映射为课程不存在。
func (r *repo) updateCourseInviteCode(ctx context.Context, courseID int64, inviteCode string) (CourseDTO, error) {
	var row sqlcgen.Course
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateCourseInviteCode(ctx, sqlcgen.UpdateCourseInviteCodeParams{ID: courseID, InviteCode: inviteCode})
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = updated
		return e
	})
	return courseDTOFromRow(row), err
}

// joinCourseByInvite 按邀请码读取课程并加入成员,保证邀请码命中和成员写入在同一事务内。
func (r *repo) joinCourseByInvite(ctx context.Context, memberID, tenantID, studentID int64, inviteCode string) (MemberDTO, error) {
	var member sqlcgen.CourseMember
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		course, e := q.GetCourseByInviteCode(ctx, inviteCode)
		if db.IsNoRows(e) {
			return apperr.ErrCourseJoinInvalid
		}
		if e != nil {
			return e
		}
		member, e = q.AddCourseMember(ctx, sqlcgen.AddCourseMemberParams{
			ID: memberID, TenantID: tenantID, CourseID: course.ID, StudentID: studentID, JoinMode: JoinModeInvite,
		})
		return e
	})
	return memberDTOFromRow(member), err
}

// listCourseMembers 分页读取课程成员。
func (r *repo) listCourseMembers(ctx context.Context, courseID int64, size, offset int) ([]MemberDTO, error) {
	var rows []sqlcgen.CourseMember
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListCourseMembers(ctx, sqlcgen.ListCourseMembersParams{
			CourseID: courseID, LimitCount: int32(size), OffsetCount: int32(offset),
		})
		rows = found
		return e
	})
	return memberDTOsFromRows(rows), err
}

// addCourseMembers 批量写入课程成员,调用方负责先校验目标账号学生身份。
func (r *repo) addCourseMembers(ctx context.Context, tenantID, courseID int64, studentIDs []int64, nextID func() int64) ([]MemberDTO, error) {
	rows := make([]sqlcgen.CourseMember, 0, len(studentIDs))
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		for _, studentID := range studentIDs {
			member, e := q.AddCourseMember(ctx, sqlcgen.AddCourseMemberParams{
				ID: nextID(), TenantID: tenantID, CourseID: courseID, StudentID: studentID, JoinMode: JoinModeTeacher,
			})
			if e != nil {
				return e
			}
			rows = append(rows, member)
		}
		return nil
	})
	return memberDTOsFromRows(rows), err
}

// removeCourseMember 移除课程成员。
func (r *repo) removeCourseMember(ctx context.Context, courseID, studentID int64) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		return q.RemoveCourseMember(ctx, sqlcgen.RemoveCourseMemberParams{CourseID: courseID, StudentID: studentID})
	})
}

// listCourseGradesPage 分页读取课程成绩。
func (r *repo) listCourseGradesPage(ctx context.Context, courseID int64, size, offset int) ([]CourseGradeDTO, error) {
	var rows []sqlcgen.CourseGrade
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{
			CourseID: courseID, LimitCount: int32(size), OffsetCount: int32(offset),
		})
		rows = found
		return e
	})
	out := make([]CourseGradeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeDTOFromRow(row))
	}
	return out, err
}

// replaceGradeWeights 替换课程成绩权重配置。
func (r *repo) replaceGradeWeights(ctx context.Context, tenantID, courseID int64, items []GradeWeightInput, nextID func() int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if err := q.DeleteGradeWeightsByCourse(ctx, courseID); err != nil {
			return err
		}
		for _, item := range items {
			weight, err := pgtypex.Numeric(item.Weight)
			if err != nil {
				return apperr.ErrGradeWeightInvalid.WithCause(err)
			}
			if _, err = q.CreateGradeWeight(ctx, sqlcgen.CreateGradeWeightParams{
				ID: nextID(), TenantID: tenantID, CourseID: courseID, SourceType: item.SourceType, SourceRef: item.SourceRef, Weight: weight,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// listGradeWeights 查询课程成绩权重并转换为模块统一权重模型。
func (r *repo) listGradeWeights(ctx context.Context, courseID int64) ([]GradeWeightInput, error) {
	var rows []sqlcgen.GradeWeight
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListGradeWeightsByCourse(ctx, courseID)
		rows = found
		return e
	})
	return gradeWeightInputsFromRows(rows), err
}

// listLatestAssignmentScoresForCourse 查询成绩计算需要的最新作业得分。
func (r *repo) listLatestAssignmentScoresForCourse(ctx context.Context, courseID int64) ([]AssignmentScoreSnapshot, error) {
	var rows []sqlcgen.ListLatestAssignmentScoresForCourseRow
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListLatestAssignmentScoresForCourse(ctx, courseID)
		rows = found
		return e
	})
	return assignmentScoreSnapshotsFromRows(rows), err
}

// upsertAutoCourseGrade 写入自动计算总评。
func (r *repo) upsertAutoCourseGrade(ctx context.Context, tenantID, gradeID, courseID, studentID int64, total float64) (CourseGradeDTO, error) {
	autoTotal, err := pgtypex.Numeric(total)
	if err != nil {
		return CourseGradeDTO{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	var row sqlcgen.CourseGrade
	err = r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		updated, e := q.UpsertCourseGrade(ctx, sqlcgen.UpsertCourseGradeParams{
			ID: gradeID, TenantID: tenantID, CourseID: courseID, StudentID: studentID, AutoTotal: autoTotal,
		})
		row = updated
		return e
	})
	return gradeDTOFromRow(row), err
}

// listCourseGradesWithCourseInTenant 读取单课程成绩及课程学分供 M11 聚合只读。
func (r *repo) listCourseGradesWithCourseInTenant(ctx context.Context, tenantID, courseID int64, limit int) ([]CourseGradeSnapshot, error) {
	var course sqlcgen.Course
	var rows []CourseGradeSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		foundCourse, e := q.GetCourseByID(ctx, courseID)
		if e != nil {
			return e
		}
		course = foundCourse
		foundRows, e := q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{CourseID: courseID, LimitCount: int32(limit)})
		if e != nil {
			return e
		}
		rows = make([]CourseGradeSnapshot, 0, len(foundRows))
		for _, row := range foundRows {
			rows = append(rows, courseGradeSnapshotWithCourseFromRows(course, row))
		}
		return e
	})
	return rows, err
}

// listStudentCourseGradesInTenant 读取学生跨课程成绩并返回 M11 只读聚合需要的课程成绩投影。
func (r *repo) listStudentCourseGradesInTenant(ctx context.Context, tenantID, studentID int64) ([]CourseGradeSnapshot, error) {
	var rows []CourseGradeSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.ListStudentCourseGrades(ctx, studentID)
		rows = make([]CourseGradeSnapshot, 0, len(found))
		for _, row := range found {
			rows = append(rows, courseGradeSnapshotFromStudentCourseRow(row))
		}
		return e
	})
	return rows, err
}

// teachingStatsInTenant 读取 M9 看板需要的 M6 自有统计。
func (r *repo) teachingStatsInTenant(ctx context.Context, tenantID int64) (teachingStatsRow, error) {
	stats := teachingStatsRow{TenantID: tenantID}
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		stats.CourseCount, e = q.CountCourses(ctx)
		if e != nil {
			return e
		}
		stats.ActiveCourseCount, e = q.CountActiveCourses(ctx)
		if e != nil {
			return e
		}
		stats.LearningDurationSec, e = q.SumLearningDuration(ctx)
		return e
	})
	return stats, err
}

// getCourse 读取课程并把缺失映射为课程不存在。
func (r *repo) getCourse(ctx context.Context, courseID int64) (CourseAccessSnapshot, error) {
	var row sqlcgen.Course
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetCourseByID(ctx, courseID)
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = found
		return e
	})
	return courseAccessSnapshotFromRow(row), err
}

// getChapter 读取章节并把缺失映射为课程不存在。
func (r *repo) getChapter(ctx context.Context, chapterID int64) (ChapterLocation, error) {
	var row sqlcgen.Chapter
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetChapterByID(ctx, chapterID)
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = found
		return e
	})
	return chapterLocationFromRow(row), err
}

// getLesson 读取课时并把缺失映射为课程不存在。
func (r *repo) getLesson(ctx context.Context, lessonID int64) (LessonContentSnapshot, error) {
	var row sqlcgen.Lesson
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetLessonByID(ctx, lessonID)
		if db.IsNoRows(e) {
			return apperr.ErrCourseNotFound
		}
		row = found
		return e
	})
	return lessonContentSnapshotFromRow(row), err
}

// getAssignment 读取作业并把缺失映射为作业不存在。
func (r *repo) getAssignment(ctx context.Context, assignmentID int64) (AssignmentPolicySnapshot, error) {
	var row sqlcgen.Assignment
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetAssignmentByID(ctx, assignmentID)
		if db.IsNoRows(e) {
			return apperr.ErrAssignmentNotFound
		}
		row = found
		return e
	})
	return assignmentPolicySnapshotFromRow(row), err
}

// getSubmission 读取提交并把缺失映射为提交不存在。
func (r *repo) getSubmission(ctx context.Context, submissionID int64) (SubmissionScoreSnapshot, error) {
	var row sqlcgen.Submission
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetSubmissionByID(ctx, submissionID)
		if db.IsNoRows(e) {
			return apperr.ErrSubmissionNotFound
		}
		row = found
		return e
	})
	return submissionScoreSnapshotFromRow(row), err
}

// createAssignmentWithItems 在同一事务内创建作业和题目引用。
func (r *repo) createAssignmentWithItems(ctx context.Context, tenantID, assignmentID, courseID int64, req AssignmentRequest, latePenalty []byte, nextID func() int64) (AssignmentPolicySnapshot, []AssignmentItemSnapshot, error) {
	var assignment AssignmentPolicySnapshot
	var items []AssignmentItemSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.CreateAssignment(ctx, sqlcgen.CreateAssignmentParams{
			ID: assignmentID, TenantID: tenantID, CourseID: courseID, Title: req.Title, ChapterID: pgtypex.Int8(ids.ParseOrZero(req.ChapterID)),
			DueAt: timex.RequiredTimestamptz(req.DueAt), MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy,
			LatePenalty: latePenalty, Status: AssignmentStatusDraft,
		})
		if e != nil {
			return e
		}
		assignment = assignmentPolicySnapshotFromRow(row)
		items, e = r.createAssignmentItems(ctx, q, tenantID, assignmentID, req.Items, nextID)
		return e
	})
	return assignment, items, err
}

// updateAssignmentWithItems 在同一事务内更新草稿作业并重建题目引用。
func (r *repo) updateAssignmentWithItems(ctx context.Context, tenantID, assignmentID int64, req AssignmentRequest, latePenalty []byte, nextID func() int64) (AssignmentPolicySnapshot, []AssignmentItemSnapshot, []AssignmentItemSnapshot, error) {
	var assignment AssignmentPolicySnapshot
	var items []AssignmentItemSnapshot
	var oldItems []AssignmentItemSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		foundOld, e := q.ListAssignmentItems(ctx, assignmentID)
		if e != nil {
			return e
		}
		oldItems = assignmentItemSnapshotsFromRows(foundOld)
		row, e := q.UpdateAssignment(ctx, sqlcgen.UpdateAssignmentParams{
			ID: assignmentID, Title: req.Title, ChapterID: pgtypex.Int8(ids.ParseOrZero(req.ChapterID)),
			DueAt: timex.RequiredTimestamptz(req.DueAt), MaxAttempts: req.MaxAttempts, LatePolicy: req.LatePolicy, LatePenalty: latePenalty,
		})
		if e != nil {
			return e
		}
		assignment = assignmentPolicySnapshotFromRow(row)
		if e = q.DeleteAssignmentItems(ctx, assignmentID); e != nil {
			return e
		}
		items, e = r.createAssignmentItems(ctx, q, tenantID, assignmentID, req.Items, nextID)
		return e
	})
	return assignment, items, oldItems, err
}

// publishAssignment 更新作业发布状态。
func (r *repo) publishAssignment(ctx context.Context, assignmentID int64) (AssignmentPolicySnapshot, error) {
	var assignment AssignmentPolicySnapshot
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.UpdateAssignmentStatus(ctx, sqlcgen.UpdateAssignmentStatusParams{ID: assignmentID, Status: AssignmentStatusPublished})
		assignment = assignmentPolicySnapshotFromRow(row)
		return e
	})
	return assignment, err
}

// createAssignmentItems 重建作业题目引用,调用方负责事务边界。
func (r *repo) createAssignmentItems(ctx context.Context, q *sqlcgen.Queries, tenantID, assignmentID int64, req []AssignmentItemInput, nextID func() int64) ([]AssignmentItemSnapshot, error) {
	out := make([]AssignmentItemSnapshot, 0, len(req))
	for idx, item := range req {
		seq := item.Seq
		if seq <= 0 {
			seq = int32(idx + 1)
		}
		row, err := q.CreateAssignmentItem(ctx, sqlcgen.CreateAssignmentItemParams{
			ID: nextID(), TenantID: tenantID, AssignmentID: assignmentID, ItemCode: item.ItemCode,
			ItemVersion: item.ItemVersion, Score: item.Score, Seq: seq, GradingMode: item.GradingMode, JudgerCode: pgtypex.Text(item.JudgerCode),
		})
		if err != nil {
			return nil, err
		}
		out = append(out, assignmentItemSnapshotFromRow(row))
	}
	return out, nil
}

// listAssignmentItems 查询作业题目引用。
func (r *repo) listAssignmentItems(ctx context.Context, assignmentID int64) ([]AssignmentItemSnapshot, error) {
	var items []AssignmentItemSnapshot
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListAssignmentItems(ctx, assignmentID)
		items = assignmentItemSnapshotsFromRows(rows)
		return e
	})
	return items, err
}

// upsertSubmissionDraft 写入学生服务端草稿并映射越权写入。
func (r *repo) upsertSubmissionDraft(ctx context.Context, tenantID, draftID, assignmentID, studentID int64, content []byte) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpsertSubmissionDraft(ctx, sqlcgen.UpsertSubmissionDraftParams{ID: draftID, TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID, Content: content})
		if db.IsNoRows(err) {
			return apperr.ErrSubmissionForbidden
		}
		return err
	})
}

// getSubmissionDraftContent 读取学生服务端草稿,不存在时返回空内容。
func (r *repo) getSubmissionDraftContent(ctx context.Context, assignmentID, studentID int64) (map[string]any, error) {
	content := map[string]any{}
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		draft, e := q.GetSubmissionDraft(ctx, sqlcgen.GetSubmissionDraftParams{AssignmentID: assignmentID, StudentID: studentID})
		if db.IsNoRows(e) {
			return nil
		}
		if e != nil {
			return e
		}
		content = jsonx.ObjectMap(draft.Content)
		return nil
	})
	return content, err
}

// createSubmissionWithOutbox 在一个事务内写正式提交、可选判题 outbox 并清理草稿。
func (r *repo) createSubmissionWithOutbox(ctx context.Context, tenantID, submissionID, assignmentID, studentID int64, attempt int32, content []byte, isLate bool, status int16, outbox *SubmissionJudgeOutboxCreate, nextID func() int64) (SubmissionScoreSnapshot, error) {
	var submission SubmissionScoreSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.CreateSubmission(ctx, sqlcgen.CreateSubmissionParams{
			ID: submissionID, TenantID: tenantID, AssignmentID: assignmentID, StudentID: studentID, AttemptNo: attempt,
			ContentRef: content, JudgeTaskRef: pgtypex.Text(""), IsLate: isLate, Status: status,
		})
		if db.IsNoRows(e) {
			return apperr.ErrSubmissionForbidden
		}
		if e != nil {
			return e
		}
		submission = submissionScoreSnapshotFromRow(row)
		if outbox != nil {
			if _, e = q.CreateSubmissionJudgeOutbox(ctx, sqlcgen.CreateSubmissionJudgeOutboxParams{
				ID: nextID(), TenantID: tenantID, SubmissionID: submissionID, AssignmentID: assignmentID, StudentID: studentID,
				ItemCode: outbox.ItemCode, ItemVersion: outbox.ItemVersion, JudgerCode: outbox.JudgerCode,
				CodeStorageKey: outbox.CodeStorageKey, CodeHash: outbox.CodeHash, ExtraInput: outbox.ExtraInput,
				SourceRef: outbox.SourceRef, Status: SubmissionJudgeOutboxPending,
			}); e != nil {
				return e
			}
		}
		return q.DeleteSubmissionDraft(ctx, sqlcgen.DeleteSubmissionDraftParams{AssignmentID: assignmentID, StudentID: studentID})
	})
	return submission, err
}

// listSubmissionsByAssignment 分页读取作业提交列表。
func (r *repo) listSubmissionsByAssignment(ctx context.Context, assignmentID int64, size, offset int) ([]SubmissionScoreSnapshot, error) {
	var rows []SubmissionScoreSnapshot
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.ListSubmissionsByAssignment(ctx, sqlcgen.ListSubmissionsByAssignmentParams{AssignmentID: assignmentID, LimitCount: int32(size), OffsetCount: int32(offset)})
		rows = make([]SubmissionScoreSnapshot, 0, len(found))
		for _, row := range found {
			rows = append(rows, submissionScoreSnapshotFromRow(row))
		}
		return e
	})
	return rows, err
}

// updateSubmissionManualScore 写入教师人工批改分。
func (r *repo) updateSubmissionManualScore(ctx context.Context, submissionID, score, finalScore int64, comment string) (SubmissionScoreSnapshot, error) {
	var submission SubmissionScoreSnapshot
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		row, e := q.UpdateSubmissionManualScore(ctx, sqlcgen.UpdateSubmissionManualScoreParams{
			ID: submissionID, ManualScore: pgtypex.Int4(int32(score)), FinalScore: pgtypex.Int4(int32(finalScore)), Comment: pgtypex.Text(comment), Status: SubmissionStatusGraded,
		})
		submission = submissionScoreSnapshotFromRow(row)
		return e
	})
	return submission, err
}

// getSubmissionWithAssignmentByJudgeTask 读取判题事件对应提交和作业策略。
func (r *repo) getSubmissionWithAssignmentByJudgeTask(ctx context.Context, tenantID int64, taskRef string) (SubmissionScoreSnapshot, AssignmentPolicySnapshot, error) {
	var submission SubmissionScoreSnapshot
	var assignment AssignmentPolicySnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		foundSubmission, e := q.GetSubmissionByJudgeTaskRef(ctx, pgtypex.Text(taskRef))
		if db.IsNoRows(e) {
			return apperr.ErrSubmissionEventUnmatched.WithCause(e)
		}
		if e != nil {
			return e
		}
		submission = submissionScoreSnapshotFromRow(foundSubmission)
		foundAssignment, e := q.GetAssignmentByID(ctx, foundSubmission.AssignmentID)
		if e != nil {
			return e
		}
		assignment = assignmentPolicySnapshotFromRow(foundAssignment)
		return nil
	})
	return submission, assignment, err
}

// updateSubmissionAutoScoreForEvent 写入自动判题分和最终分。
func (r *repo) updateSubmissionAutoScoreForEvent(ctx context.Context, tenantID, submissionID int64, score, finalScore int32) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateSubmissionAutoScore(ctx, sqlcgen.UpdateSubmissionAutoScoreParams{
			ID: submissionID, AutoScore: pgtypex.Int4(score), FinalScore: pgtypex.Int4(finalScore), Status: SubmissionStatusGraded,
		})
		return err
	})
}

// markSubmissionJudgeFailedForEvent 标记判题失败提交等待教师处理。
func (r *repo) markSubmissionJudgeFailedForEvent(ctx context.Context, tenantID int64, taskRef, comment string) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		submission, err := q.GetSubmissionByJudgeTaskRef(ctx, pgtypex.Text(taskRef))
		if db.IsNoRows(err) {
			return apperr.ErrSubmissionEventUnmatched.WithCause(err)
		}
		if err != nil {
			return err
		}
		_, err = q.UpdateSubmissionManualScore(ctx, sqlcgen.UpdateSubmissionManualScoreParams{ID: submission.ID, Comment: pgtypex.Text(comment), Status: SubmissionStatusPending})
		return err
	})
}

// countSubmissionsByStudent 统计学生在作业下已有提交次数。
func (r *repo) countSubmissionsByStudent(ctx context.Context, assignmentID, studentID int64) (int64, error) {
	var count int64
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var e error
		count, e = q.CountSubmissionsByStudent(ctx, sqlcgen.CountSubmissionsByStudentParams{AssignmentID: assignmentID, StudentID: studentID})
		return e
	})
	return count, err
}

// listPendingSubmissionJudgeOutboxTenants 读取存在待派发判题任务的租户。
func (r *repo) listPendingSubmissionJudgeOutboxTenants(ctx context.Context, limit int) ([]int64, error) {
	var tenantIDs []int64
	err := r.inPrivileged(ctx, func(q *sqlcgen.Queries) error {
		rows, e := q.ListPendingSubmissionJudgeOutboxTenants(ctx, sqlcgen.ListPendingSubmissionJudgeOutboxTenantsParams{
			Status: SubmissionJudgeOutboxPending, LimitCount: int32(limit),
		})
		tenantIDs = rows
		return e
	})
	return tenantIDs, err
}

// listPendingSubmissionJudgeOutbox 读取单租户待派发判题 outbox。
func (r *repo) listPendingSubmissionJudgeOutbox(ctx context.Context, tenantID int64, limit int) ([]SubmissionJudgeOutboxSnapshot, error) {
	var rows []SubmissionJudgeOutboxSnapshot
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.ListPendingSubmissionJudgeOutbox(ctx, sqlcgen.ListPendingSubmissionJudgeOutboxParams{
			Status: SubmissionJudgeOutboxPending, LimitCount: int32(limit),
		})
		rows = submissionJudgeOutboxSnapshotsFromRows(found)
		return e
	})
	return rows, err
}

// claimPendingSubmissionJudgeOutbox 原子领取 pending outbox,未抢到时返回 found=false。
func (r *repo) claimPendingSubmissionJudgeOutbox(ctx context.Context, tenantID, outboxID int64) (SubmissionJudgeOutboxSnapshot, bool, error) {
	var row SubmissionJudgeOutboxSnapshot
	found := false
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		claimed, e := q.MarkSubmissionJudgeOutboxRunning(ctx, sqlcgen.MarkSubmissionJudgeOutboxRunningParams{
			RunningStatus: SubmissionJudgeOutboxRunning, ID: outboxID, PendingStatus: SubmissionJudgeOutboxPending,
		})
		if db.IsNoRows(e) {
			return nil
		}
		if e != nil {
			return e
		}
		row = submissionJudgeOutboxSnapshotFromRow(claimed)
		found = true
		return nil
	})
	return row, found, err
}

// completeSubmissionJudgeOutbox 绑定 M3 task 并标记 outbox 完成。
func (r *repo) completeSubmissionJudgeOutbox(ctx context.Context, row SubmissionJudgeOutboxSnapshot, taskID int64) error {
	return r.inTenantID(ctx, row.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateSubmissionJudgeTaskRef(ctx, sqlcgen.UpdateSubmissionJudgeTaskRefParams{ID: row.SubmissionID, JudgeTaskRef: pgtypex.Text(ids.Format(taskID))}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		if _, err := q.CompleteSubmissionJudgeOutbox(ctx, sqlcgen.CompleteSubmissionJudgeOutboxParams{ID: row.ID, DoneStatus: SubmissionJudgeOutboxDone}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		return nil
	})
}

// failSubmissionJudgeOutbox 记录派发失败原因并恢复 pending。
func (r *repo) failSubmissionJudgeOutbox(ctx context.Context, tenantID, outboxID int64, cause error) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.FailSubmissionJudgeOutbox(ctx, sqlcgen.FailSubmissionJudgeOutboxParams{
			ID: outboxID, PendingStatus: SubmissionJudgeOutboxPending, LastError: pgtypex.Text(cause.Error()),
		}); err != nil {
			return apperr.ErrSubmissionJudgeLink.WithCause(err)
		}
		return nil
	})
}

// ensureCourseMember 判断学生是否已加入课程。
func (r *repo) ensureCourseMember(ctx context.Context, courseID, accountID int64, forbidden *apperr.Error) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, err := q.GetCourseMember(ctx, sqlcgen.GetCourseMemberParams{CourseID: courseID, StudentID: accountID})
		if db.IsNoRows(err) {
			return forbidden
		}
		return err
	})
}

// isCourseMember 返回当前账号是否为课程成员。
func (r *repo) isCourseMember(ctx context.Context, courseID, accountID int64) (bool, error) {
	member := false
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.GetCourseMember(ctx, sqlcgen.GetCourseMemberParams{CourseID: courseID, StudentID: accountID})
		if db.IsNoRows(e) {
			return nil
		}
		if e != nil {
			return e
		}
		member = true
		return nil
	})
	return member, err
}

// updateCourseStatusIfAllowed 校验发布完整性后更新课程状态。
func (r *repo) updateCourseStatusIfAllowed(ctx context.Context, courseID int64, status int16) (CourseDTO, error) {
	var row sqlcgen.Course
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if status == CourseStatusPublished {
			publishable, e := q.EnsureCoursePublishable(ctx, courseID)
			if e != nil {
				return e
			}
			if !publishable {
				return apperr.ErrCourseInvalidState
			}
		}
		updated, e := q.UpdateCourseStatus(ctx, sqlcgen.UpdateCourseStatusParams{ID: courseID, Status: status})
		row = updated
		return e
	})
	return courseDTOFromRow(row), err
}

// updateCourseVisibility 更新课程共享状态。
func (r *repo) updateCourseVisibility(ctx context.Context, courseID int64, visibility int16) (CourseDTO, error) {
	var row sqlcgen.Course
	err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateCourseVisibility(ctx, sqlcgen.UpdateCourseVisibilityParams{ID: courseID, Visibility: visibility})
		row = updated
		return e
	})
	return courseDTOFromRow(row), err
}

// upsertOverrideCourseGrade 写入覆盖成绩并保留已有自动成绩。
func (r *repo) upsertOverrideCourseGrade(ctx context.Context, tenantID, gradeID, courseID, studentID int64, score float64) (CourseGradeSnapshot, error) {
	override, err := pgtypex.Numeric(score)
	if err != nil {
		return CourseGradeSnapshot{}, apperr.ErrGradeInvalid.WithCause(err)
	}
	var row sqlcgen.CourseGrade
	err = r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetCourseGrade(ctx, sqlcgen.GetCourseGradeParams{CourseID: courseID, StudentID: studentID})
		autoTotal := override
		if e == nil {
			autoTotal = current.AutoTotal
		} else if !db.IsNoRows(e) {
			return e
		}
		updated, e := q.UpsertCourseGrade(ctx, sqlcgen.UpsertCourseGradeParams{
			ID: gradeID, TenantID: tenantID, CourseID: courseID, StudentID: studentID,
			AutoTotal: autoTotal, OverrideTotal: override, IsOverridden: true,
		})
		row = updated
		return e
	})
	return courseGradeSnapshotFromRow(row), err
}

// getCourseGradeOptional 读取成绩快照,未命中时返回 false。
func (r *repo) getCourseGradeOptional(ctx context.Context, tenantID, courseID, studentID int64) (CourseGradeSnapshot, bool, error) {
	var row sqlcgen.CourseGrade
	found := false
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		current, e := q.GetCourseGrade(ctx, sqlcgen.GetCourseGradeParams{CourseID: courseID, StudentID: studentID})
		if db.IsNoRows(e) {
			return nil
		}
		if e != nil {
			return e
		}
		row = current
		found = true
		return nil
	})
	return courseGradeSnapshotFromRow(row), found, err
}

// tenantFromContext 读取当前请求租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) { return tenant.FromContext(ctx) }
