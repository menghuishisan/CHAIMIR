// AppealsPage 配置学校管理员成绩申诉页面文案。

import React from 'react'
import { GradeAppealsPage } from '../../../components/GradeAppealsPage'

/** AppealsPage 使用统一申诉处理能力。 */
export default function AppealsPage(): React.ReactElement {
  return <GradeAppealsPage title="学生申诉工单" subtitle="查看申诉进度，并受理或驳回待处理申请。" ariaLabel="学校成绩申诉列表" />
}
