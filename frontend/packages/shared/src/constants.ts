// 常量定义

/**
 * 用户角色
 */
export enum UserRole {
  PLATFORM_ADMIN = 1,
  SCHOOL_ADMIN = 2,
  TEACHER = 3,
  STUDENT = 4,
}

/**
 * 用户状态
 */
export enum UserStatus {
  ACTIVE = 1,
  INACTIVE = 2,
  BANNED = 3,
}

/**
 * 内容类型
 */
export enum ContentType {
  PROBLEM = 1,           // 题目
  TEMPLATE = 2,          // 模板
  EXPERIMENT_TEMPLATE = 3, // 实验模板
}

/**
 * 内容可见性
 */
export enum ContentVisibility {
  PRIVATE = 1,   // 私有
  TENANT = 2,    // 租户内
  PUBLIC = 3,    // 公开
}

/**
 * 内容状态
 */
export enum ContentStatus {
  DRAFT = 1,     // 草稿
  PUBLISHED = 2, // 已发布
  ARCHIVED = 3,  // 已归档
}

/**
 * 难度等级
 */
export enum DifficultyLevel {
  EASY = 1,
  MEDIUM = 2,
  HARD = 3,
}

/**
 * 课程状态
 */
export enum CourseStatus {
  DRAFT = 1,      // 草稿
  PUBLISHED = 2,  // 已发布
  ONGOING = 3,    // 进行中
  ENDED = 4,      // 已结束
  ARCHIVED = 5,   // 已归档
}

/**
 * 作业状态
 */
export enum AssignmentStatus {
  DRAFT = 1,      // 草稿
  PUBLISHED = 2,  // 已发布
  CLOSED = 3,     // 已关闭
}

/**
 * 提交状态
 */
export enum SubmissionStatus {
  DRAFT = 1,       // 草稿
  SUBMITTED = 2,   // 已提交
  JUDGING = 3,     // 判题中
  JUDGED = 4,      // 已判题
  REVIEWED = 5,    // 已批改
}

/**
 * 沙箱状态
 */
export enum SandboxStatus {
  PENDING = 1,    // 等待中
  CREATING = 2,   // 创建中
  RUNNING = 3,    // 运行中
  STOPPING = 4,   // 停止中
  STOPPED = 5,    // 已停止
  ERROR = 6,      // 错误
}

/**
 * 判题状态
 */
export enum JudgeStatus {
  PENDING = 1,    // 等待中
  JUDGING = 2,    // 判题中
  COMPLETED = 3,  // 已完成
  FAILED = 4,     // 失败
  CANCELLED = 5,  // 已取消
}

/**
 * 判题结果
 */
export enum JudgeResult {
  AC = 'AC',           // Accepted 通过
  WA = 'WA',           // Wrong Answer 答案错误
  TLE = 'TLE',         // Time Limit Exceeded 超时
  MLE = 'MLE',         // Memory Limit Exceeded 内存超限
  RE = 'RE',           // Runtime Error 运行错误
  CE = 'CE',           // Compile Error 编译错误
  SE = 'SE',           // System Error 系统错误
}

/**
 * 存储键名
 */
export const StorageKeys = {
  ACCESS_TOKEN: 'chaimir_access_token',
  REFRESH_TOKEN: 'chaimir_refresh_token',
  USER_INFO: 'chaimir_user_info',
  THEME: 'chaimir_theme',
} as const

/**
 * API 路径前缀
 */
export const API_BASE_PATH = '/api/v1'

/**
 * 分页默认配置
 */
export const DEFAULT_PAGE_SIZE = 20
export const DEFAULT_PAGE = 1
