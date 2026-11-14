// service_list_names 成绩中心列表随记录下发的姓名与课程名:当前页出现的账号与课程各批量解析一次
// (经身份与教学契约),客户端不再拉全校账号目录或课程列表自行匹配(后端层能力索引「学生档案」)。
package grade

import (
	"context"

	"chaimir/internal/contracts"
)

// listNames 是一页记录用到的账号与课程名;解析不到(账号已移除、课程已删除)的不在表里,对应字段留空。
type listNames struct {
	accounts map[int64]contracts.AccountInfo
	courses  map[int64]string
}

// loadListNames 按当前页出现的账号与课程编号各批量解析一次。
func (s *Service) loadListNames(ctx context.Context, tenantID int64, accountIDs, courseIDs []int64) (listNames, error) {
	names := listNames{accounts: map[int64]contracts.AccountInfo{}, courses: map[int64]string{}}
	if ids := distinctIDs(accountIDs); len(ids) > 0 {
		accounts, err := s.roles.BatchGetAccounts(ctx, ids)
		if err != nil {
			return listNames{}, err
		}
		for _, account := range accounts {
			names.accounts[account.AccountID] = account
		}
	}
	if ids := distinctIDs(courseIDs); len(ids) > 0 {
		courses, err := s.teaching.BatchGetCourses(ctx, tenantID, ids)
		if err != nil {
			return listNames{}, err
		}
		for _, course := range courses {
			names.courses[course.CourseID] = course.Name
		}
	}
	return names, nil
}

// distinctIDs 去掉重复与非正的编号,保持首次出现的顺序。
func distinctIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok || value <= 0 {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// withReviewNames 给成绩审核记录填报送教师姓名与课程名。
func (s *Service) withReviewNames(ctx context.Context, tenantID int64, items []ReviewDTO) ([]ReviewDTO, error) {
	accountIDs := make([]int64, 0, len(items))
	courseIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.SubmitterID.Int64())
		courseIDs = append(courseIDs, item.CourseID.Int64())
	}
	names, err := s.loadListNames(ctx, tenantID, accountIDs, courseIDs)
	if err != nil {
		return nil, err
	}
	return applyReviewNames(items, names), nil
}

// applyReviewNames 把报送教师姓名与课程名合并进审核记录(返回新切片,不改动传入的列表)。
func applyReviewNames(items []ReviewDTO, names listNames) []ReviewDTO {
	out := make([]ReviewDTO, len(items))
	for i, item := range items {
		item.SubmitterName = names.accounts[item.SubmitterID.Int64()].Name
		item.CourseName = names.courses[item.CourseID.Int64()]
		out[i] = item
	}
	return out
}

// withAppealNames 给申诉记录填学生姓名、学号与课程名。
func (s *Service) withAppealNames(ctx context.Context, tenantID int64, items []AppealDTO) ([]AppealDTO, error) {
	accountIDs := make([]int64, 0, len(items))
	courseIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID.Int64())
		courseIDs = append(courseIDs, item.CourseID.Int64())
	}
	names, err := s.loadListNames(ctx, tenantID, accountIDs, courseIDs)
	if err != nil {
		return nil, err
	}
	return applyAppealNames(items, names), nil
}

// applyAppealNames 把学生姓名、学号与课程名合并进申诉记录(返回新切片,不改动传入的列表)。
func applyAppealNames(items []AppealDTO, names listNames) []AppealDTO {
	out := make([]AppealDTO, len(items))
	for i, item := range items {
		student := names.accounts[item.StudentID.Int64()]
		item.StudentName, item.StudentNo = student.Name, student.No
		item.CourseName = names.courses[item.CourseID.Int64()]
		out[i] = item
	}
	return out
}

// withWarningNames 给学业预警记录填学生姓名与学号。
func (s *Service) withWarningNames(ctx context.Context, tenantID int64, items []WarningDTO) ([]WarningDTO, error) {
	accountIDs := make([]int64, 0, len(items))
	for _, item := range items {
		accountIDs = append(accountIDs, item.StudentID.Int64())
	}
	names, err := s.loadListNames(ctx, tenantID, accountIDs, nil)
	if err != nil {
		return nil, err
	}
	return applyWarningNames(items, names), nil
}

// applyWarningNames 把学生姓名与学号合并进预警记录(返回新切片,不改动传入的列表)。
func applyWarningNames(items []WarningDTO, names listNames) []WarningDTO {
	out := make([]WarningDTO, len(items))
	for i, item := range items {
		student := names.accounts[item.StudentID.Int64()]
		item.StudentName, item.StudentNo = student.Name, student.No
		out[i] = item
	}
	return out
}
