// 申诉处理页(校管侧栏,/school-admin/appeals)。
//
// 与教师侧的申诉处理是同一后端能力(GET /appeals 在 teacher 组,校管也在其中),
// 差别在归属:教师只处理自己授课课程的申诉,校管能看到全校 —— 边界由后端
// ensureAppealHandlerCanAccessCourse 判定,前端不做二次过滤。
//
// 复用教师侧已实现的 GradeAppeals 区块:同一件事一个实现,校管端只多一层页面外壳
// (教师侧它是批改中心内区块,校管侧它本身就是侧栏页,对齐清单 §3.3)。

import { Scale } from 'lucide-react'
import { Breadcrumb, Callout, PageHeader, PageScaffold } from '@chaimir/ui'
import { GradeAppeals } from '../teacher/grade-appeals'

/**
 * SchoolAdminAppealsPage 承载全校成绩申诉处理。
 */
export default function SchoolAdminAppealsPage() {
  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '教务与成绩' }, { label: '申诉处理' }]} />}
        title="申诉处理"
        description="学生对课程成绩有异议时提交的申诉。受理会解锁该课程成绩,由授课教师调分后重新报送。"
        icon={Scale}
      />

      <GradeAppeals canRecompute />

      <Callout tone="info">
        授课教师也能处理自己课程的申诉。这里能看到全校范围,便于跟进长时间未处理的申诉。
      </Callout>
    </PageScaffold>
  )
}
