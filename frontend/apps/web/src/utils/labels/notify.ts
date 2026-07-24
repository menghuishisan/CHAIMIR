// notify labels 文件维护通知偏好事件文案。

import { labelFromMap } from './map'

/** notificationPreferenceLabel 将通知事件键转换为业务名称。 */
export function notificationPreferenceLabel(type: string): string {
  return labelFromMap(type, {
    'assignment.due': '作业截止提醒', 'assignment.graded': '作业批改结果', 'contest.start': '竞赛开始提醒',
    'experiment.ready': '实验环境就绪提醒', 'grade.published': '成绩发布提醒',
    'system.announcement': '系统公告提醒',
  }, '其他业务提醒')
}

/** notificationTypeLabel 将站内通知事件键转换为用户向类别。 */
export function notificationTypeLabel(type: string): string {
  return notificationPreferenceLabel(type)
}
