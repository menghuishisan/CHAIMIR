// options 文件集中维护 apps/web 页面复用的后端枚举选择项。

import {
  AccountStatus,
  AlertStatus,
  AnnouncementScope,
  AuthMode,
  BaseIdentity,
  BattleRole,
  ClassStatus,
  ContentDifficulty,
  ContentType,
  ContentVisibility,
  ContestMode,
  CourseStatus,
  CourseType,
  ExperimentCollabMode,
  GradingMode,
  GradeAppealStatus,
  GradeReviewStatus,
  ImportTarget,
  LatePolicy,
  LessonContentType,
  MatchMode,
  PaperMode,
  SIM_COMPUTE,
  SIM_REVIEW_RESULT,
  SsoMatchField,
  SsoType,
  TeachingDifficulty,
  TeamMode,
  UserRole,
  VulnLevel,
  VulnRuntimeMode,
} from '@chaimir/api-client'
import type { SelectOption } from '@chaimir/ui'
import {
  accountStatusLabel,
  alertStatusLabel,
  announcementScopeLabel,
  authModeLabel,
  baseIdentityLabel,
  battleRoleLabel,
  classStatusLabel,
  contestModeLabel,
  contentDifficultyLabel,
  contentTypeLabel,
  contentVisibilityLabel,
  courseStatusLabel,
  courseTypeLabel,
  experimentCollabModeLabel,
  gradingModeLabel,
  gradeAppealStatusLabel,
  gradeReviewStatusLabel,
  latePolicyLabel,
  lessonContentTypeLabel,
  matchModeLabel,
  paperModeLabel,
  simComputeLabel,
  simReviewResultLabel,
  ssoMatchFieldLabel,
  ssoTypeLabel,
  teamModeLabel,
  teachingDifficultyLabel,
  vulnLevelLabel,
  vulnRuntimeModeLabel,
} from './labels'

/**
 * option 以统一方式把后端枚举值转成 Select 需要的字符串值。
 */
function option(value: string | number, label: string): SelectOption {
  return { value: String(value), label }
}

/**
 * withAllOption 为列表筛选项添加“全部”入口。
 */
export function withAllOption(label: string, options: SelectOption[]): SelectOption[] {
  return [option('', label), ...options]
}

export const courseStatusOptions = [
  option(CourseStatus.DRAFT, courseStatusLabel(CourseStatus.DRAFT)),
  option(CourseStatus.PUBLISHED, courseStatusLabel(CourseStatus.PUBLISHED)),
  option(CourseStatus.RUNNING, courseStatusLabel(CourseStatus.RUNNING)),
  option(CourseStatus.ENDED, courseStatusLabel(CourseStatus.ENDED)),
  option(CourseStatus.ARCHIVED, courseStatusLabel(CourseStatus.ARCHIVED)),
]

export const studentCourseStatusFilterOptions = withAllOption('全部课程', [
  option(CourseStatus.RUNNING, courseStatusLabel(CourseStatus.RUNNING)),
  option(CourseStatus.ENDED, courseStatusLabel(CourseStatus.ENDED)),
])

export const courseTypeOptions = [
  option(CourseType.THEORY, courseTypeLabel(CourseType.THEORY)),
  option(CourseType.LAB, courseTypeLabel(CourseType.LAB)),
  option(CourseType.MIXED, courseTypeLabel(CourseType.MIXED)),
  option(CourseType.PROJECT, courseTypeLabel(CourseType.PROJECT)),
]

export const teachingDifficultyOptions = [
  option(TeachingDifficulty.INTRO, teachingDifficultyLabel(TeachingDifficulty.INTRO)),
  option(TeachingDifficulty.ADVANCED, teachingDifficultyLabel(TeachingDifficulty.ADVANCED)),
  option(TeachingDifficulty.EXPERT, teachingDifficultyLabel(TeachingDifficulty.EXPERT)),
  option(TeachingDifficulty.RESEARCH, teachingDifficultyLabel(TeachingDifficulty.RESEARCH)),
]

export const lessonContentTypeOptions = [
  option(LessonContentType.VIDEO, lessonContentTypeLabel(LessonContentType.VIDEO)),
  option(LessonContentType.MARKDOWN, lessonContentTypeLabel(LessonContentType.MARKDOWN)),
  option(LessonContentType.ATTACHMENT, lessonContentTypeLabel(LessonContentType.ATTACHMENT)),
  option(LessonContentType.EXPERIMENT, lessonContentTypeLabel(LessonContentType.EXPERIMENT)),
  option(LessonContentType.SIMULATION, lessonContentTypeLabel(LessonContentType.SIMULATION)),
]

export const latePolicyOptions = [
  option(LatePolicy.REJECT, latePolicyLabel(LatePolicy.REJECT)),
  option(LatePolicy.PENALIZE, latePolicyLabel(LatePolicy.PENALIZE)),
  option(LatePolicy.NO_PENALTY, latePolicyLabel(LatePolicy.NO_PENALTY)),
]

export const gradingModeOptions = [
  option(GradingMode.AUTO, gradingModeLabel(GradingMode.AUTO)),
  option(GradingMode.MANUAL, gradingModeLabel(GradingMode.MANUAL)),
]

export const experimentCollabModeOptions = [
  option(ExperimentCollabMode.SOLO, experimentCollabModeLabel(ExperimentCollabMode.SOLO)),
  option(ExperimentCollabMode.GROUP, experimentCollabModeLabel(ExperimentCollabMode.GROUP)),
]

export const contestModeOptions = [
  option(ContestMode.SOLVE, contestModeLabel(ContestMode.SOLVE)),
  option(ContestMode.BATTLE, contestModeLabel(ContestMode.BATTLE)),
]

export const teamModeOptions = [
  option(TeamMode.SOLO, teamModeLabel(TeamMode.SOLO)),
  option(TeamMode.GROUP, teamModeLabel(TeamMode.GROUP)),
]

export const matchModeOptions = [
  option(MatchMode.ROUND_ROBIN, matchModeLabel(MatchMode.ROUND_ROBIN)),
  option(MatchMode.ELO, matchModeLabel(MatchMode.ELO)),
]

export const battleRoleOptions = [
  option(BattleRole.ATTACK, battleRoleLabel(BattleRole.ATTACK)),
  option(BattleRole.DEFENSE, battleRoleLabel(BattleRole.DEFENSE)),
  option(BattleRole.STRATEGY, battleRoleLabel(BattleRole.STRATEGY)),
]

export const vulnLevelOptions = [
  option(VulnLevel.A, vulnLevelLabel(VulnLevel.A)),
  option(VulnLevel.B, vulnLevelLabel(VulnLevel.B)),
  option(VulnLevel.C, vulnLevelLabel(VulnLevel.C)),
]

export const vulnRuntimeModeOptions = [
  option(VulnRuntimeMode.ISOLATED, vulnRuntimeModeLabel(VulnRuntimeMode.ISOLATED)),
  option(VulnRuntimeMode.FORKED, vulnRuntimeModeLabel(VulnRuntimeMode.FORKED)),
]

export const contentTypeOptions = [
  option(ContentType.EXPERIMENT_TEMPLATE, contentTypeLabel(ContentType.EXPERIMENT_TEMPLATE)),
  option(ContentType.CONTEST_PROBLEM, contentTypeLabel(ContentType.CONTEST_PROBLEM)),
  option(ContentType.THEORY_QUESTION, contentTypeLabel(ContentType.THEORY_QUESTION)),
]

export const contentDifficultyOptions = [
  option(ContentDifficulty.INTRO, contentDifficultyLabel(ContentDifficulty.INTRO)),
  option(ContentDifficulty.BASIC, contentDifficultyLabel(ContentDifficulty.BASIC)),
  option(ContentDifficulty.ADVANCED, contentDifficultyLabel(ContentDifficulty.ADVANCED)),
  option(ContentDifficulty.CHALLENGE, contentDifficultyLabel(ContentDifficulty.CHALLENGE)),
]

export const contentVisibilityOptions = [
  option(ContentVisibility.PRIVATE, contentVisibilityLabel(ContentVisibility.PRIVATE)),
  option(ContentVisibility.TENANT, contentVisibilityLabel(ContentVisibility.TENANT)),
  option(ContentVisibility.SHARED, contentVisibilityLabel(ContentVisibility.SHARED)),
]

export const paperModeOptions = [
  option(PaperMode.MANUAL, paperModeLabel(PaperMode.MANUAL)),
  option(PaperMode.RANDOM, paperModeLabel(PaperMode.RANDOM)),
]

export const accountRoleFilterOptions = withAllOption('全部角色', [
  option(UserRole.STUDENT, '学生'),
  option(UserRole.TEACHER, '教师'),
  option(UserRole.SCHOOL_ADMIN, '学校管理员'),
])

export const announcementTargetRoleOptions = [
  option(UserRole.STUDENT, '学生'),
  option(UserRole.TEACHER, '教师'),
  option(UserRole.SCHOOL_ADMIN, '学校管理员'),
]

export const accountStatusFilterOptions = withAllOption('全部状态', [
  option(AccountStatus.PENDING, accountStatusLabel(AccountStatus.PENDING)),
  option(AccountStatus.ACTIVE, accountStatusLabel(AccountStatus.ACTIVE)),
  option(AccountStatus.DISABLED, accountStatusLabel(AccountStatus.DISABLED)),
  option(AccountStatus.ARCHIVED, accountStatusLabel(AccountStatus.ARCHIVED)),
])

export const baseIdentityOptions = [
  option(BaseIdentity.STUDENT, baseIdentityLabel(BaseIdentity.STUDENT)),
  option(BaseIdentity.TEACHER, baseIdentityLabel(BaseIdentity.TEACHER)),
]

export const importTargetOptions = [
  option(ImportTarget.TEACHER, '教师账号'),
  option(ImportTarget.STUDENT, '学生账号'),
  option(ImportTarget.ORG, '组织架构'),
]

export const accountImportTargetOptions = [
  option('student', '学生账号'),
  option('teacher', '教师账号'),
]

export const tenantApplicationSchoolTypeOptions = [
  option(1, '本科院校'),
  option(2, '高职高专'),
  option(3, '其他教育机构'),
]

export const authModeOptions = [
  option(AuthMode.LOCAL, authModeLabel(AuthMode.LOCAL)),
  option(AuthMode.CAS, authModeLabel(AuthMode.CAS)),
  option(AuthMode.LDAP, authModeLabel(AuthMode.LDAP)),
]

export const ssoTypeOptions = [
  option(SsoType.CAS, ssoTypeLabel(SsoType.CAS)),
  option(SsoType.LDAP, ssoTypeLabel(SsoType.LDAP)),
]

export const ssoMatchFieldOptions = [
  option(SsoMatchField.NO, ssoMatchFieldLabel(SsoMatchField.NO)),
  option(SsoMatchField.PHONE, ssoMatchFieldLabel(SsoMatchField.PHONE)),
]

export const classStatusOptions = [
  option(ClassStatus.ACTIVE, classStatusLabel(ClassStatus.ACTIVE)),
  option(ClassStatus.ARCHIVED, classStatusLabel(ClassStatus.ARCHIVED)),
]

export const gradeReviewStatusFilterOptions = withAllOption('全部状态', [
  option(GradeReviewStatus.PENDING, gradeReviewStatusLabel(GradeReviewStatus.PENDING)),
  option(GradeReviewStatus.APPROVED, gradeReviewStatusLabel(GradeReviewStatus.APPROVED)),
  option(GradeReviewStatus.REJECTED, gradeReviewStatusLabel(GradeReviewStatus.REJECTED)),
])

export const gradeAppealStatusFilterOptions = withAllOption('全部状态', [
  option(GradeAppealStatus.PENDING, gradeAppealStatusLabel(GradeAppealStatus.PENDING)),
  option(GradeAppealStatus.ACCEPTED, gradeAppealStatusLabel(GradeAppealStatus.ACCEPTED)),
  option(GradeAppealStatus.COMPLETED, gradeAppealStatusLabel(GradeAppealStatus.COMPLETED)),
  option(GradeAppealStatus.REJECTED, gradeAppealStatusLabel(GradeAppealStatus.REJECTED)),
])

export const announcementScopeOptions = [
  option(AnnouncementScope.TENANT, announcementScopeLabel(AnnouncementScope.TENANT)),
  option(AnnouncementScope.ROLES, announcementScopeLabel(AnnouncementScope.ROLES)),
]

export const alertStatusFilterOptions = withAllOption('全部状态', [
  option(AlertStatus.PENDING, alertStatusLabel(AlertStatus.PENDING)),
  option(AlertStatus.HANDLED, alertStatusLabel(AlertStatus.HANDLED)),
  option(AlertStatus.IGNORED, alertStatusLabel(AlertStatus.IGNORED)),
])

export const alertLevelOptions = [
  option(1, '一般提醒'),
  option(2, '重要告警'),
  option(3, '严重告警'),
]

export const simComputeOptions = [
  option(SIM_COMPUTE.FRONTEND, simComputeLabel(SIM_COMPUTE.FRONTEND)),
  option(SIM_COMPUTE.BACKEND, simComputeLabel(SIM_COMPUTE.BACKEND)),
]

export const simReviewResultOptions = withAllOption('全部结果', [
  option(SIM_REVIEW_RESULT.PENDING, simReviewResultLabel(SIM_REVIEW_RESULT.PENDING)),
  option(SIM_REVIEW_RESULT.APPROVED, simReviewResultLabel(SIM_REVIEW_RESULT.APPROVED)),
  option(SIM_REVIEW_RESULT.REJECTED, simReviewResultLabel(SIM_REVIEW_RESULT.REJECTED)),
])
