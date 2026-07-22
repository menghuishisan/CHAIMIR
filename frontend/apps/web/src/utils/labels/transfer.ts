// transfer labels 文件维护导入导出任务主题和状态文案。

import type { TransferTask } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** transferTaskSubjectLabel 将内部任务主题转换为下载中心业务名称。 */
export function transferTaskSubjectLabel(subject: string): string {
  return labelFromMap(subject, {
    'admin.audit_export': '平台审计记录导出', 'teaching.course_grade_export': '课程成绩导出',
  }, '数据处理任务')
}

/** transferTaskStatusLabel 返回导入导出任务状态文案。 */
export function transferTaskStatusLabel(status: TransferTask['status']): string {
  return labelFromMap(status, {
    pending: '等待处理', running: '处理中', retrying: '准备重试', succeeded: '已完成', failed: '处理失败',
  }, '未知')
}
