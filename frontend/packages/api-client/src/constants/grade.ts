export enum GradeReviewStatus {
  PENDING = 1,
  APPROVED = 2,
  REJECTED = 3,
}

export enum GradeAppealStatus {
  PENDING = 1,
  // 等待任课教师改分。
  ACCEPTED = 2,
  COMPLETED = 3,
  REJECTED = 4,
}

export enum GradeWarningType {
  FAILED_COURSE = 1,
  LOW_GPA = 2,
}

export enum GradeWarningStatus {
  PENDING = 1,
  ACKNOWLEDGED = 2,
}

export enum TranscriptScope {
  SEMESTER = 1,
  FULL = 2,
}
