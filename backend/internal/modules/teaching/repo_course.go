// teaching repo_course 文件封装课程、章节、课时和成员数据访问。
package teaching

import (
	"context"
	"strconv"
	"time"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
)

// formatNumber 把浮点配置写入 numeric 字段时转为十进制字符串,避免二进制浮点格式污染 SQL。
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

// CreateCourse 创建课程草稿。
func (s *txStore) CreateCourse(ctx context.Context, course Course) (Course, error) {
	schedule, err := encodeMap(course.Schedule)
	if err != nil {
		return Course{}, err
	}
	row, err := s.q.CreateCourse(ctx, sqlcgen.CreateCourseParams{ID: course.ID, TenantID: course.TenantID, TeacherID: course.TeacherID, Name: course.Name, Description: course.Description, Type: course.Type, Difficulty: course.Difficulty, CoverUrl: textParam(course.CoverURL), Semester: course.Semester, Column10: formatNumber(course.Credits), Schedule: schedule, StartAt: timestamptzParam(course.StartAt), EndAt: timestamptzParam(course.EndAt), InviteCode: course.InviteCode, Status: course.Status, Visibility: course.Visibility})
	if err != nil {
		return Course{}, err
	}
	return courseFromCreateRow(row)
}

// GetCourse 按 ID 读取课程。
func (s *txStore) GetCourse(ctx context.Context, tenantID, id int64) (Course, error) {
	row, err := s.q.GetCourseByID(ctx, sqlcgen.GetCourseByIDParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Course{}, err
	}
	return courseFromGetRow(row)
}

// GetCloneableCourse 按 ID 读取本租户课程或共享课程库课程。
func (s *txStore) GetCloneableCourse(ctx context.Context, id, targetTenantID int64) (Course, error) {
	row, err := s.q.GetCloneableCourseByID(ctx, sqlcgen.GetCloneableCourseByIDParams{ID: id, TenantID: targetTenantID})
	if err != nil {
		return Course{}, err
	}
	return courseFromCloneableRow(row)
}

// GetCourseByInviteCode 按邀请码读取课程。
func (s *txStore) GetCourseByInviteCode(ctx context.Context, code string) (Course, error) {
	row, err := s.q.GetCourseByInviteCode(ctx, code)
	if err != nil {
		return Course{}, err
	}
	return courseFromInviteRow(row)
}

// ListTeacherCourses 查询教师课程分页。
func (s *txStore) ListTeacherCourses(ctx context.Context, tenantID, teacherID int64, filter CourseListFilter) ([]Course, int64, error) {
	rows, err := s.q.ListTeacherCourses(ctx, sqlcgen.ListTeacherCoursesParams{TenantID: tenantID, TeacherID: teacherID, Column3: filter.Status, Limit: int32(filter.Size), Offset: int32((filter.Page - 1) * filter.Size)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountTeacherCourses(ctx, sqlcgen.CountTeacherCoursesParams{TenantID: tenantID, TeacherID: teacherID, Column3: filter.Status})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Course, 0, len(rows))
	for _, row := range rows {
		course, err := courseFromTeacherRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, course)
	}
	return out, total, nil
}

// ListStudentCourses 查询学生课程分页。
func (s *txStore) ListStudentCourses(ctx context.Context, tenantID, studentID int64, filter CourseListFilter) ([]Course, int64, error) {
	rows, err := s.q.ListStudentCourses(ctx, sqlcgen.ListStudentCoursesParams{TenantID: tenantID, StudentID: studentID, Column3: filter.Status, Limit: int32(filter.Size), Offset: int32((filter.Page - 1) * filter.Size)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountStudentCourses(ctx, sqlcgen.CountStudentCoursesParams{TenantID: tenantID, StudentID: studentID, Column3: filter.Status})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Course, 0, len(rows))
	for _, row := range rows {
		course, err := courseFromStudentRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, course)
	}
	return out, total, nil
}

// UpdateCourse 更新课程基础信息。
func (s *txStore) UpdateCourse(ctx context.Context, course Course) (Course, error) {
	schedule, err := encodeMap(course.Schedule)
	if err != nil {
		return Course{}, err
	}
	row, err := s.q.UpdateCourse(ctx, sqlcgen.UpdateCourseParams{TenantID: course.TenantID, ID: course.ID, Name: course.Name, Description: course.Description, Type: course.Type, Difficulty: course.Difficulty, CoverUrl: textParam(course.CoverURL), Semester: course.Semester, Column9: formatNumber(course.Credits), Schedule: schedule, StartAt: timestamptzParam(course.StartAt), EndAt: timestamptzParam(course.EndAt)})
	if err != nil {
		return Course{}, err
	}
	return courseFromUpdateRow(row)
}

// SetCourseStatus 更新课程状态。
func (s *txStore) SetCourseStatus(ctx context.Context, tenantID, id int64, status int16) (Course, error) {
	row, err := s.q.SetCourseStatus(ctx, sqlcgen.SetCourseStatusParams{TenantID: tenantID, ID: id, Status: status})
	if err != nil {
		return Course{}, err
	}
	return courseFromStatusRow(row)
}

// SetCourseVisibility 更新课程共享状态。
func (s *txStore) SetCourseVisibility(ctx context.Context, tenantID, id int64, visibility int16) (Course, error) {
	row, err := s.q.SetCourseVisibility(ctx, sqlcgen.SetCourseVisibilityParams{TenantID: tenantID, ID: id, Visibility: visibility})
	if err != nil {
		return Course{}, err
	}
	return courseFromVisibilityRow(row)
}

// RefreshCourseInviteCode 刷新课程邀请码。
func (s *txStore) RefreshCourseInviteCode(ctx context.Context, tenantID, id int64, code string) (Course, error) {
	row, err := s.q.RefreshCourseInviteCode(ctx, sqlcgen.RefreshCourseInviteCodeParams{TenantID: tenantID, ID: id, InviteCode: code})
	if err != nil {
		return Course{}, err
	}
	return courseFromRefreshRow(row)
}

// CountCourseLessons 统计课程课时数量。
func (s *txStore) CountCourseLessons(ctx context.Context, tenantID, courseID int64) (int64, error) {
	return s.q.CountCourseLessons(ctx, sqlcgen.CountCourseLessonsParams{TenantID: tenantID, CourseID: courseID})
}

// ListCoursesDueToRun 查询已到开始时间但尚未进入进行中的课程。
func (s *txStore) ListCoursesDueToRun(ctx context.Context, now time.Time) ([]Course, error) {
	rows, err := s.q.ListCoursesDueToRun(ctx, timestamptzParam(now))
	if err != nil {
		return nil, err
	}
	out := make([]Course, 0, len(rows))
	for _, row := range rows {
		course, err := courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, course)
	}
	return out, nil
}

// ListCoursesDueToEnd 查询已到结束时间但尚未结束的课程。
func (s *txStore) ListCoursesDueToEnd(ctx context.Context, now time.Time) ([]Course, error) {
	rows, err := s.q.ListCoursesDueToEnd(ctx, timestamptzParam(now))
	if err != nil {
		return nil, err
	}
	out := make([]Course, 0, len(rows))
	for _, row := range rows {
		course, err := courseFromFields(row.ID, row.TenantID, row.TeacherID, row.Name, row.Description, row.Type, row.Difficulty, row.CoverUrl, row.Semester, row.Credits, row.Schedule, row.StartAt, row.EndAt, row.InviteCode, row.Status, row.Visibility, row.CreatedAt, row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, course)
	}
	return out, nil
}

// CreateChapter 创建章节。
func (s *txStore) CreateChapter(ctx context.Context, chapter Chapter) (Chapter, error) {
	row, err := s.q.CreateChapter(ctx, sqlcgen.CreateChapterParams{ID: chapter.ID, TenantID: chapter.TenantID, CourseID: chapter.CourseID, Title: chapter.Title, Sort: chapter.Sort})
	if err != nil {
		return Chapter{}, err
	}
	return chapterFromRow(row), nil
}

// GetChapter 读取章节。
func (s *txStore) GetChapter(ctx context.Context, tenantID, id int64) (Chapter, error) {
	row, err := s.q.GetChapter(ctx, sqlcgen.GetChapterParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Chapter{}, err
	}
	return chapterFromRow(row), nil
}

// ListChapters 查询课程章节。
func (s *txStore) ListChapters(ctx context.Context, tenantID, courseID int64) ([]Chapter, error) {
	rows, err := s.q.ListChapters(ctx, sqlcgen.ListChaptersParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]Chapter, 0, len(rows))
	for _, row := range rows {
		out = append(out, chapterFromRow(row))
	}
	return out, nil
}

// UpdateChapter 更新章节。
func (s *txStore) UpdateChapter(ctx context.Context, chapter Chapter) (Chapter, error) {
	row, err := s.q.UpdateChapter(ctx, sqlcgen.UpdateChapterParams{TenantID: chapter.TenantID, ID: chapter.ID, Title: chapter.Title, Sort: chapter.Sort})
	if err != nil {
		return Chapter{}, err
	}
	return chapterFromRow(row), nil
}

// DeleteChapter 软删章节。
func (s *txStore) DeleteChapter(ctx context.Context, tenantID, id int64) (Chapter, error) {
	row, err := s.q.SoftDeleteChapter(ctx, sqlcgen.SoftDeleteChapterParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Chapter{}, err
	}
	return chapterFromRow(row), nil
}

// CreateLesson 创建课时。
func (s *txStore) CreateLesson(ctx context.Context, lesson Lesson) (Lesson, error) {
	ref, err := encodeMap(lesson.ContentRef)
	if err != nil {
		return Lesson{}, err
	}
	row, err := s.q.CreateLesson(ctx, sqlcgen.CreateLessonParams{ID: lesson.ID, TenantID: lesson.TenantID, ChapterID: lesson.ChapterID, Title: lesson.Title, ContentType: lesson.ContentType, ContentRef: ref, Sort: lesson.Sort})
	if err != nil {
		return Lesson{}, err
	}
	return lessonFromRow(row)
}

// GetLesson 读取课时。
func (s *txStore) GetLesson(ctx context.Context, tenantID, id int64) (Lesson, error) {
	row, err := s.q.GetLesson(ctx, sqlcgen.GetLessonParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Lesson{}, err
	}
	return lessonFromRow(row)
}

// ListLessonsByChapter 查询章节课时。
func (s *txStore) ListLessonsByChapter(ctx context.Context, tenantID, chapterID int64) ([]Lesson, error) {
	rows, err := s.q.ListLessonsByChapter(ctx, sqlcgen.ListLessonsByChapterParams{TenantID: tenantID, ChapterID: chapterID})
	if err != nil {
		return nil, err
	}
	out := make([]Lesson, 0, len(rows))
	for _, row := range rows {
		lesson, err := lessonFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, lesson)
	}
	return out, nil
}

// ListLessonsByCourse 查询课程全部课时。
func (s *txStore) ListLessonsByCourse(ctx context.Context, tenantID, courseID int64) ([]Lesson, error) {
	rows, err := s.q.ListLessonsByCourse(ctx, sqlcgen.ListLessonsByCourseParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]Lesson, 0, len(rows))
	for _, row := range rows {
		lesson, err := lessonFromRow(sqlcgen.Lesson{ID: row.ID, TenantID: row.TenantID, ChapterID: row.ChapterID, Title: row.Title, ContentType: row.ContentType, ContentRef: row.ContentRef, Sort: row.Sort, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt})
		if err != nil {
			return nil, err
		}
		out = append(out, lesson)
	}
	return out, nil
}

// UpdateLesson 更新课时。
func (s *txStore) UpdateLesson(ctx context.Context, lesson Lesson) (Lesson, error) {
	ref, err := encodeMap(lesson.ContentRef)
	if err != nil {
		return Lesson{}, err
	}
	row, err := s.q.UpdateLesson(ctx, sqlcgen.UpdateLessonParams{TenantID: lesson.TenantID, ID: lesson.ID, Title: lesson.Title, ContentType: lesson.ContentType, ContentRef: ref, Sort: lesson.Sort})
	if err != nil {
		return Lesson{}, err
	}
	return lessonFromRow(row)
}

// SetLessonContent 更新课时内容引用。
func (s *txStore) SetLessonContent(ctx context.Context, tenantID, id int64, contentType int16, ref map[string]any) (Lesson, error) {
	raw, err := encodeMap(ref)
	if err != nil {
		return Lesson{}, err
	}
	row, err := s.q.SetLessonContent(ctx, sqlcgen.SetLessonContentParams{TenantID: tenantID, ID: id, ContentType: contentType, ContentRef: raw})
	if err != nil {
		return Lesson{}, err
	}
	return lessonFromRow(row)
}

// DeleteLesson 软删课时。
func (s *txStore) DeleteLesson(ctx context.Context, tenantID, id int64) (Lesson, error) {
	row, err := s.q.SoftDeleteLesson(ctx, sqlcgen.SoftDeleteLessonParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Lesson{}, err
	}
	return lessonFromRow(row)
}

// CreateCourseMember 创建课程成员关系。
func (s *txStore) CreateCourseMember(ctx context.Context, member CourseMember) (CourseMember, error) {
	row, err := s.q.CreateCourseMember(ctx, sqlcgen.CreateCourseMemberParams{ID: member.ID, TenantID: member.TenantID, CourseID: member.CourseID, StudentID: member.StudentID, JoinMode: member.JoinMode})
	if err != nil {
		return CourseMember{}, err
	}
	return memberFromRow(row), nil
}

// GetCourseMember 读取课程成员关系。
func (s *txStore) GetCourseMember(ctx context.Context, tenantID, courseID, studentID int64) (CourseMember, error) {
	row, err := s.q.GetCourseMember(ctx, sqlcgen.GetCourseMemberParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID})
	if err != nil {
		return CourseMember{}, err
	}
	return memberFromRow(row), nil
}

// ListCourseMembers 查询课程成员分页。
func (s *txStore) ListCourseMembers(ctx context.Context, tenantID, courseID int64, page, size int) ([]CourseMember, int64, error) {
	rows, err := s.q.ListCourseMembers(ctx, sqlcgen.ListCourseMembersParams{TenantID: tenantID, CourseID: courseID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountCourseMembers(ctx, sqlcgen.CountCourseMembersParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, 0, err
	}
	out := make([]CourseMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, memberFromRow(row))
	}
	return out, total, nil
}

// DeleteCourseMember 删除课程成员关系。
func (s *txStore) DeleteCourseMember(ctx context.Context, tenantID, courseID, studentID int64) error {
	return s.q.DeleteCourseMember(ctx, sqlcgen.DeleteCourseMemberParams{TenantID: tenantID, CourseID: courseID, StudentID: studentID})
}
