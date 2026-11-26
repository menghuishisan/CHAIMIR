package teaching

import (
	"context"
	"errors"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/apperr"
)

// 来源读取成功后再保存目标，避免把特权来源事务借给其他模块。
type courseClone struct {
	course              Course
	chapters            []Chapter
	lessons             []Lesson
	assignments         []Assignment
	items               map[int64][]AssignmentItem
	materials           []storage.ObjectRef
	dependencies        []func(context.Context) error
	discardDependencies []func(context.Context) error
}

// CloneCourse 克隆本人课程或已发布的共享课程为独立私有草稿。
func (s *Service) CloneCourse(ctx context.Context, courseID int64, req CloneCourseRequest) (CourseDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CourseDTO{}, err
	}
	var clone courseClone
	var out CourseDTO
	commitAttempted := false
	sourceMissing := false
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetCourse(ctx, id.TenantID, courseID); err != nil {
			sourceMissing = isNoRows(err)
			return err
		}
		source, err := tx.GetCloneableCourse(ctx, courseID, id.TenantID)
		if err != nil {
			return err
		}
		clone, err = s.prepareCourseClone(ctx, tx, source, id.TenantID, id.AccountID, req.Name)
		return err
	})
	if sourceMissing {
		err = s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			source, err := tx.GetCloneableCourse(ctx, courseID, id.TenantID)
			if err != nil {
				return err
			}
			if source.TenantID == id.TenantID {
				return apperr.ErrTeachingCourseNotFound
			}
			clone, err = s.prepareCourseClone(ctx, tx, source, id.TenantID, id.AccountID, req.Name)
			return err
		})
	}
	if err == nil {
		err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			if err := s.saveCourseClone(ctx, tx, &clone); err != nil {
				return err
			}
			var err error
			out, err = courseDTO(clone.course)
			if err != nil {
				return err
			}
			if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "teaching.course.clone", auditTargetCourse, clone.course.ID, map[string]any{"source_course_id": courseID}); err != nil {
				return err
			}
			commitAttempted = true
			return nil
		})
	}
	if err != nil {
		err = storage.CompensateUpload(ctx, s.cleanupTimeout, err, commitAttempted, func(ctx context.Context) error {
			errs := []error{s.discardMaterialObjects(ctx, clone.materials...)}
			for _, discard := range clone.discardDependencies {
				errs = append(errs, discard(ctx))
			}
			return errors.Join(errs...)
		})
		return CourseDTO{}, mapCourseError(err)
	}
	return out, nil
}

func (s *Service) prepareCourseClone(ctx context.Context, tx TxStore, source Course, targetTenantID, teacherID int64, name string) (courseClone, error) {
	var clone courseClone
	if source.TenantID != targetTenantID || source.TeacherID != teacherID {
		if source.Visibility != CourseVisibilityShared || source.Status == CourseStatusDraft {
			return clone, apperr.ErrTeachingCourseForbidden
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = source.Name + " 副本"
	}
	inviteCode, err := newInviteCode()
	if err != nil {
		return clone, err
	}
	schedule, err := jsonx.CloneObjectStrict(source.Schedule)
	if err != nil {
		return clone, err
	}
	// 封面由新作者上传，不能继续引用会随源课程替换而删除的对象。
	clone.course = Course{ID: s.ids.Generate(), TenantID: targetTenantID, TeacherID: teacherID, Name: name, Description: source.Description, Type: source.Type, Difficulty: source.Difficulty, Semester: source.Semester, Credits: source.Credits, Schedule: schedule, StartAt: source.StartAt, EndAt: source.EndAt, InviteCode: inviteCode, Status: CourseStatusDraft, Visibility: CourseVisibilityPrivate}
	clone.items = make(map[int64][]AssignmentItem)
	chapters, err := s.prepareClonedLessons(ctx, tx, source, &clone)
	if err != nil {
		return clone, err
	}
	if err := s.prepareClonedAssignments(ctx, tx, source, &clone, chapters); err != nil {
		return clone, err
	}
	if err := s.prepareClonedDependencies(ctx, source, &clone); err != nil {
		return clone, err
	}
	return clone, nil
}

func (s *Service) prepareClonedDependencies(ctx context.Context, source Course, clone *courseClone) error {
	var refs []contracts.ContentItemRef
	for _, assignment := range clone.assignments {
		for _, item := range clone.items[assignment.ID] {
			refs = append(refs, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		}
	}
	if len(refs) > 0 {
		plan, err := s.contentClone.PrepareContentClone(ctx, source.TenantID, refs)
		if err != nil {
			return err
		}
		clone.dependencies = append(clone.dependencies, plan.Save)
		clone.discardDependencies = append(clone.discardDependencies, plan.Discard)
		for assignmentID, items := range clone.items {
			for i := range items {
				ref, ok := plan.Refs[contracts.ContentItemRef{ItemCode: items[i].ItemCode, ItemVersion: items[i].ItemVersion}]
				if !ok {
					return apperr.ErrTeachingAssignmentInvalid
				}
				items[i].ItemCode, items[i].ItemVersion = ref.ItemCode, ref.ItemVersion
			}
			clone.items[assignmentID] = items
		}
	}
	var experimentIDs []int64
	validatedSims := make(map[string]bool)
	for _, lesson := range clone.lessons {
		switch lesson.ContentType {
		case LessonContentExperiment:
			experimentID, ok := ids.Parse(jsonx.StringField(lesson.ContentRef, "experiment_id"))
			if !ok || experimentID <= 0 {
				return apperr.ErrTeachingLessonInvalid
			}
			experimentIDs = append(experimentIDs, experimentID)
		case LessonContentSimulation:
			code, version := jsonx.StringField(lesson.ContentRef, "package_code"), jsonx.StringField(lesson.ContentRef, "version")
			key := code + "\x00" + version
			if !validatedSims[key] {
				if err := s.sim.ValidatePackage(ctx, clone.course.TenantID, code, version); err != nil {
					return err
				}
				validatedSims[key] = true
			}
		}
	}
	plan, err := s.experimentClone.PrepareExperimentClone(ctx, contracts.ExperimentCloneRequest{SourceTenantID: source.TenantID, SourceCourseID: source.ID, SourceTeacherID: source.TeacherID, TargetCourseID: clone.course.ID, LessonExperimentIDs: experimentIDs})
	if err != nil {
		return err
	}
	clone.dependencies = append(clone.dependencies, plan.Save)
	clone.discardDependencies = append(clone.discardDependencies, plan.Discard)
	for i := range clone.lessons {
		lesson := &clone.lessons[i]
		if lesson.ContentType != LessonContentExperiment {
			continue
		}
		sourceID, ok := ids.Parse(jsonx.StringField(lesson.ContentRef, "experiment_id"))
		if !ok || plan.IDs[sourceID] <= 0 {
			return apperr.ErrTeachingLessonInvalid
		}
		lesson.ContentRef["experiment_id"] = ids.Format(plan.IDs[sourceID])
	}
	return nil
}

func (s *Service) prepareClonedLessons(ctx context.Context, tx TxStore, source Course, clone *courseClone) (map[int64]int64, error) {
	chapters, err := tx.ListChapters(ctx, source.TenantID, source.ID)
	if err != nil {
		return nil, err
	}
	chapterIDs := make(map[int64]int64, len(chapters))
	for _, chapter := range chapters {
		target := Chapter{ID: s.ids.Generate(), TenantID: clone.course.TenantID, CourseID: clone.course.ID, Title: chapter.Title, Sort: chapter.Sort}
		clone.chapters = append(clone.chapters, target)
		chapterIDs[chapter.ID] = target.ID
		lessons, err := tx.ListLessonsByChapter(ctx, source.TenantID, chapter.ID)
		if err != nil {
			return nil, err
		}
		for _, lesson := range lessons {
			lesson, err := tx.GetLessonForUpdate(ctx, source.TenantID, lesson.ID)
			if err != nil {
				return nil, err
			}
			content, err := jsonx.CloneObjectStrict(lesson.ContentRef)
			if err != nil {
				return nil, err
			}
			cloned := Lesson{ID: s.ids.Generate(), TenantID: clone.course.TenantID, ChapterID: target.ID, Title: lesson.Title, ContentType: lesson.ContentType, ContentRef: content, Sort: lesson.Sort}
			ref, err := s.copyLessonMaterial(ctx, lesson, &cloned)
			if ref.Key != "" {
				clone.materials = append(clone.materials, ref)
			}
			if err != nil {
				return nil, err
			}
			clone.lessons = append(clone.lessons, cloned)
		}
	}
	return chapterIDs, nil
}

func (s *Service) prepareClonedAssignments(ctx context.Context, tx TxStore, source Course, clone *courseClone, chapters map[int64]int64) error {
	for page := 1; ; page++ {
		assignments, total, err := tx.ListAssignmentsByCourse(ctx, source.TenantID, source.ID, false, page, pagex.MaximumSize())
		if err != nil {
			return err
		}
		for _, assignment := range assignments {
			chapterID := chapters[assignment.ChapterID]
			if assignment.ChapterID > 0 && chapterID == 0 {
				return apperr.ErrTeachingAssignmentInvalid
			}
			penalty, err := jsonx.CloneObjectStrict(assignment.LatePenalty)
			if err != nil {
				return err
			}
			cloned := Assignment{ID: s.ids.Generate(), TenantID: clone.course.TenantID, CourseID: clone.course.ID, Title: assignment.Title, ChapterID: chapterID, DueAt: assignment.DueAt, MaxAttempts: assignment.MaxAttempts, LatePolicy: assignment.LatePolicy, LatePenalty: penalty, Status: AssignmentStatusDraft}
			items, err := tx.ListAssignmentItems(ctx, source.TenantID, assignment.ID)
			if err != nil {
				return err
			}
			for _, item := range items {
				clone.items[cloned.ID] = append(clone.items[cloned.ID], AssignmentItem{ID: s.ids.Generate(), TenantID: clone.course.TenantID, AssignmentID: cloned.ID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, Seq: item.Seq})
			}
			clone.assignments = append(clone.assignments, cloned)
		}
		if int64(page*pagex.MaximumSize()) >= total || len(assignments) == 0 {
			return nil
		}
	}
}

func (s *Service) saveCourseClone(ctx context.Context, tx TxStore, clone *courseClone) error {
	course, err := tx.CreateCourse(ctx, clone.course)
	if err != nil {
		return err
	}
	clone.course = course
	for _, save := range clone.dependencies {
		if err := save(ctx); err != nil {
			return err
		}
	}
	for _, chapter := range clone.chapters {
		if _, err := tx.CreateChapter(ctx, chapter); err != nil {
			return err
		}
	}
	for _, lesson := range clone.lessons {
		if _, err := tx.CreateLesson(ctx, lesson); err != nil {
			return err
		}
	}
	for _, assignment := range clone.assignments {
		created, err := tx.CreateAssignment(ctx, assignment)
		if err != nil {
			return err
		}
		items, err := tx.ReplaceAssignmentItems(ctx, created.TenantID, created.ID, clone.items[assignment.ID])
		if err != nil {
			return err
		}
		if err := s.refreshAssignmentUsageRefs(ctx, created.TenantID, created, items); err != nil {
			return err
		}
	}
	return nil
}
