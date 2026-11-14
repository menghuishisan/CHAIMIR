// teaching service_list_facts 文件为课程与作业列表补充页面要显示的事实:
// 授课教师姓名、选课人数、本人学习进度、作业的本人提交情况与全班提交统计。
// 每类事实按当前页编号一次聚合读取(不逐行请求),再按编号合并回列表项;没有记录的项取零值。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// courseFacts 是一页课程的补充事实,键为课程或教师编号。
type courseFacts struct {
	teachers map[int64]contracts.AccountInfo
	members  map[int64]int64
	progress map[int64]CourseProgressDTO
}

// assignmentFacts 是一页作业的补充事实;teacherView 决定给全班统计还是本人提交情况。
type assignmentFacts struct {
	teacherView bool
	submitters  map[int64]AssignmentSubmissionStatsDTO
	mine        map[int64]MyAssignmentSubmissionDTO
}

// applyCourseFacts 把补充事实合并到当前页课程(返回新切片,不改动传入的列表);studentView 时每门课都带本人进度(还没有课时即 0/0)。
func applyCourseFacts(courses []CourseDTO, facts courseFacts, studentView bool) []CourseDTO {
	out := append([]CourseDTO(nil), courses...)
	for i := range out {
		courseID := int64(out[i].ID)
		out[i].TeacherName = facts.teachers[int64(out[i].TeacherID)].Name
		out[i].MemberCount = facts.members[courseID]
		if studentView {
			progress := facts.progress[courseID]
			out[i].MyProgress = &progress
		}
	}
	return out
}

// applyAssignmentFacts 把补充事实合并到当前页作业(返回新切片,不改动传入的列表):教师视角带全班提交统计,学生视角带本人提交情况。
func applyAssignmentFacts(assignments []AssignmentDTO, facts assignmentFacts) []AssignmentDTO {
	out := append([]AssignmentDTO(nil), assignments...)
	for i := range out {
		assignmentID := int64(out[i].ID)
		if facts.teacherView {
			stats := facts.submitters[assignmentID]
			out[i].SubmissionStats = &stats
			continue
		}
		mine := facts.mine[assignmentID]
		out[i].MySubmission = &mine
	}
	return out
}

// loadCourseFacts 在调用方事务内按当前页课程编号聚合选课人数与本人进度。
func loadCourseFacts(ctx context.Context, tx TxStore, tenantID, studentID int64, courses []Course, studentView bool) (courseFacts, error) {
	courseIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		courseIDs = append(courseIDs, course.ID)
	}
	facts := courseFacts{members: map[int64]int64{}, progress: map[int64]CourseProgressDTO{}}
	if len(courseIDs) == 0 {
		return facts, nil
	}
	members, err := tx.CountMembersByCourses(ctx, tenantID, courseIDs)
	if err != nil {
		return courseFacts{}, err
	}
	facts.members = members
	if studentView {
		progress, err := tx.CountStudentLessonProgressByCourses(ctx, tenantID, studentID, courseIDs)
		if err != nil {
			return courseFacts{}, err
		}
		facts.progress = progress
	}
	return facts, nil
}

// courseTeachers 一次批量解析当前页课程的授课教师档案;账号已注销读不到时该课不显示姓名。
func (s *Service) courseTeachers(ctx context.Context, courses []Course) (map[int64]contracts.AccountInfo, error) {
	seen := map[int64]struct{}{}
	teacherIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		if _, ok := seen[course.TeacherID]; ok {
			continue
		}
		seen[course.TeacherID] = struct{}{}
		teacherIDs = append(teacherIDs, course.TeacherID)
	}
	if len(teacherIDs) == 0 {
		return map[int64]contracts.AccountInfo{}, nil
	}
	accounts, err := s.identity.BatchGetAccounts(ctx, teacherIDs)
	if err != nil {
		return nil, apperr.ErrTeachingCourseInvalid.WithCause(err)
	}
	teachers := make(map[int64]contracts.AccountInfo, len(accounts))
	for _, account := range accounts {
		teachers[account.AccountID] = account
	}
	return teachers, nil
}

// loadAssignmentFacts 在调用方事务内按当前页作业编号聚合:教师取全班提交统计,学生取本人提交情况。
func loadAssignmentFacts(ctx context.Context, tx TxStore, tenantID, accountID int64, assignments []Assignment, teacherView bool) (assignmentFacts, error) {
	assignmentIDs := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		assignmentIDs = append(assignmentIDs, assignment.ID)
	}
	facts := assignmentFacts{teacherView: teacherView, submitters: map[int64]AssignmentSubmissionStatsDTO{}, mine: map[int64]MyAssignmentSubmissionDTO{}}
	if len(assignmentIDs) == 0 {
		return facts, nil
	}
	if teacherView {
		submitters, err := tx.CountSubmittersByAssignments(ctx, tenantID, assignmentIDs)
		if err != nil {
			return assignmentFacts{}, err
		}
		facts.submitters = submitters
		return facts, nil
	}
	mine, err := tx.SummarizeStudentSubmissionsByAssignments(ctx, tenantID, accountID, assignmentIDs)
	if err != nil {
		return assignmentFacts{}, err
	}
	facts.mine = mine
	return facts, nil
}
