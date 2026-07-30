// transfer labels 文件维护导入导出任务主题和状态文案。

import type { TransferStatus, TransferTask } from '@chaimir/api-client'

/** 主题由各业务模块自行取名,是开放字符串:未登记的主题给通用业务名,不把内部标识抛到界面上。 */
const TRANSFER_TASK_SUBJECT_LABELS: Record<string, string> = {
  'admin.audit_export': '平台审计记录导出',
  'teaching.course_grade_export': '课程成绩导出',
}

/** transferTaskSubjectLabel 将内部任务主题转换为下载中心业务名称。 */
export function transferTaskSubjectLabel(subject: string): string {
  return TRANSFER_TASK_SUBJECT_LABELS[subject] ?? '数据处理任务'
}

/** 状态是封闭枚举,按 Record 全量映射,新增状态在编译期暴露。 */
const TRANSFER_TASK_STATUS_LABELS: Record<TransferStatus, string> = {
  pending: '等待处理',
  running: '处理中',
  retrying: '准备重试',
  succeeded: '已完成',
  failed: '处理失败',
}

/** transferTaskStatusLabel 返回导入导出任务状态文案。 */
export function transferTaskStatusLabel(status: TransferTask['status']): string {
  return TRANSFER_TASK_STATUS_LABELS[status]
}
