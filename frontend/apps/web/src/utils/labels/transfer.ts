// transfer labels 文件维护导入导出任务主题、通道和状态文案。

import { TRANSFER_CHANNEL, TRANSFER_STATUS, type TransferChannel, type TransferStatus } from '@chaimir/api-client'

/** 主题由各业务模块自行取名,是开放字符串:未登记的主题给通用业务名,不把内部标识抛到界面上。 */
const TRANSFER_TASK_SUBJECT_LABELS: Record<string, string> = {
  'admin.audit_export': '平台审计记录导出',
  'teaching.course_grade_export': '课程成绩导出',
  'identity.account_import': '账号导入',
  'identity.org_import': '组织架构导入',
  'grade.transcript_export': '成绩单导出',
}

/** transferTaskSubjectLabel 将内部任务主题转换为下载中心业务名称。 */
export function transferTaskSubjectLabel(subject: string): string {
  return TRANSFER_TASK_SUBJECT_LABELS[subject] ?? '数据处理任务'
}

/** 状态是封闭枚举,按 Record 全量映射,新增状态在编译期暴露。 */
const TRANSFER_TASK_STATUS_LABELS: Record<TransferStatus, string> = {
  [TRANSFER_STATUS.PENDING]: '等待处理',
  [TRANSFER_STATUS.RUNNING]: '处理中',
  [TRANSFER_STATUS.RETRYING]: '准备重试',
  [TRANSFER_STATUS.SUCCEEDED]: '已完成',
  [TRANSFER_STATUS.FAILED]: '处理失败',
}

/** transferTaskStatusLabel 返回导入导出任务状态文案。 */
export function transferTaskStatusLabel(status: TransferStatus): string {
  return TRANSFER_TASK_STATUS_LABELS[status]
}

const TRANSFER_CHANNEL_LABELS: Record<TransferChannel, string> = {
  [TRANSFER_CHANNEL.IMPORT]: '导入',
  [TRANSFER_CHANNEL.EXPORT]: '导出',
}

/** transferChannelLabel 返回任务类型文案。 */
export function transferChannelLabel(channel: TransferChannel): string {
  return TRANSFER_CHANNEL_LABELS[channel]
}
