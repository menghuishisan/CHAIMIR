// 申诉处理页(校管侧栏,/school-admin/appeals)。
//
// 与教师侧的申诉处理是同一后端能力(GET /appeals 在 teacher 组,校管也在其中),
// 差别在归属:教师只处理自己授课课程的申诉,校管能看到全校 —— 边界由后端
// ensureAppealHandlerCanAccessCourse 判定,前端不做二次过滤。
//
// 复用教师侧已实现的 GradeAppeals 区块:同一件事一个实现,校管端只多一层页面外壳
// (教师侧它是批改中心内区块,校管侧它本身就是侧栏页,对齐清单 §3.3)。

import { PageHeader, PageScaffold } from '@chaimir/ui'
import { GradeAppeals } from '../../components/GradeAppeals'

/**
 * SchoolAdminAppealsPage 承载全校成绩申诉处理。
 */
export default function SchoolAdminAppealsPage() {
  return (
    <PageScaffold>
      <PageHeader title="申诉处理" />

      {/* 归族:审阅队列族(§6.5.3 第 ⑤)。骨架由 GradeAppeals 自带 ——
          它在教师侧是批改中心的内区块,在校管侧是整页主体,两处同一实现 */}
      <GradeAppeals scope="school" />

    </PageScaffold>
  )
}
