// 教学契约常量：维护前端需要与后端 teaching 模块枚举编号对齐的值。

export enum CourseType {
  THEORY = 1,
  LAB = 2,
  MIXED = 3,
  PROJECT = 4,
}

export enum TeachingDifficulty {
  INTRO = 1,
  ADVANCED = 2,
  EXPERT = 3,
  RESEARCH = 4,
}

export enum CourseStatus {
  DRAFT = 1,
  PUBLISHED = 2,
  RUNNING = 3,
  ENDED = 4,
  ARCHIVED = 5,
}

export enum CourseVisibility {
  PRIVATE = 1,
  SHARED = 2,
}

export enum LessonContentType {
  VIDEO = 1,
  MARKDOWN = 2,
  ATTACHMENT = 3,
  EXPERIMENT = 4,
  SIMULATION = 5,
}

export enum JoinMode {
  INVITE = 1,
  TEACHER = 2,
}

export enum AssignmentStatus {
  DRAFT = 1,
  PUBLISHED = 2,
}

export enum LatePolicy {
  REJECT = 1,
  PENALIZE = 2,
  NO_PENALTY = 3,
}

export enum GradingMode {
  AUTO = 1,
  MANUAL = 2,
}

export enum SubmissionStatus {
  SUBMITTED = 1,
  PENDING = 2,
  GRADED = 3,
}

export enum ProgressStatus {
  NOT_STARTED = 1,
  IN_PROGRESS = 2,
  DONE = 3,
}

export enum GradeSource {
  ASSIGNMENT = 1,
  EXPERIMENT = 2,
}
