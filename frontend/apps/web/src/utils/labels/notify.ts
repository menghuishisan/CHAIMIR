// notify labels 文件维护 M10 通知类型与公告范围的用户向文案。
// 通知类型是后端 notification_template 表的开放字符串:后端只给类型标识,界面名称在此登记。

import { AnnouncementScope } from '@chaimir/api-client'

/** 已登记的通知类型名称,取值与后端 notification_template.type 一一对应。 */
const NOTIFICATION_TYPE_LABELS: Record<string, string> = {
  'account.opened': '账号开通',
  'account.security': '账号安全提醒',
  'assignment.published': '新作业发布',
  'assignment.due': '作业即将截止',
  'experiment.timeout': '实验环境即将到期',
  'experiment.completed': '实验结果更新',
  'contest.registration': '竞赛报名状态',
  'contest.started': '竞赛开始',
  'grade.review': '成绩审核状态',
  'grade.appeal': '成绩申诉状态',
  'grade.warning': '学业预警',
  'system.maintenance': '系统维护',
  'system.alert': '系统告警',
}

/** notificationTypeLabel 返回通知类型名称;未登记类型给通用名,不把内部标识抛到界面上。 */
export function notificationTypeLabel(type: string): string {
  return NOTIFICATION_TYPE_LABELS[type] ?? '平台通知'
}

/**
 * FORCED_PREFERENCE_HINT 是强制类通知不可关闭的原因说明。
 * 按组说明一次(不再逐条重复):后端把账号安全、成绩流转与平台可用性三类都标成强制,
 * 因此文案要覆盖这三类,不能只写「账号安全与成绩」把系统维护类漏在外面。
 */
export const FORCED_PREFERENCE_HINT =
  '以下提醒关系到账号安全、成绩流转与平台可用性,不能关闭。'

const ANNOUNCEMENT_SCOPE_LABELS: Record<AnnouncementScope, string> = {
  [AnnouncementScope.PLATFORM]: '平台公告',
  [AnnouncementScope.TENANT]: '学校公告',
  [AnnouncementScope.ROLES]: '定向公告',
}

/** announcementScopeLabel 返回公告范围文案。 */
export function announcementScopeLabel(scope: AnnouncementScope): string {
  return ANNOUNCEMENT_SCOPE_LABELS[scope]
}
