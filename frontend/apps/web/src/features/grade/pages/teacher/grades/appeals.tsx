// TeacherGradesAppealsPage 配置教师端成绩申诉页面文案。

import React from 'react'
import { GradeAppealsPage } from '../../../components/GradeAppealsPage'

/** TeacherGradesAppealsPage 使用统一申诉处理能力。 */
export default function TeacherGradesAppealsPage(): React.ReactElement {
  return <GradeAppealsPage title="成绩申诉工单复核" subtitle="查看学生申诉进度，并受理或驳回待处理申请。" ariaLabel="教师成绩申诉列表" />
}
