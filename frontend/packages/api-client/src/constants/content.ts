// 内容契约常量：维护前端需要与后端 content 模块枚举编号对齐的值。

export enum ContentType {
  EXPERIMENT_TEMPLATE = 1,
  CONTEST_PROBLEM = 2,
  THEORY_QUESTION = 3,
}

export enum ContentDifficulty {
  INTRO = 1,
  BASIC = 2,
  ADVANCED = 3,
  CHALLENGE = 4,
}

export enum ContentAuthorType {
  TEACHER = 1,
  SYSTEM = 2,
  EXTERNAL = 3,
}

export enum ContentVisibility {
  PRIVATE = 1,
  TENANT = 2,
  SHARED = 3,
}

export enum ContentStatus {
  DRAFT = 1,
  PUBLISHED = 2,
  DEPRECATED = 3,
}

export enum PaperMode {
  MANUAL = 1,
  RANDOM = 2,
}
