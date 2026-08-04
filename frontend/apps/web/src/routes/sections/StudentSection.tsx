// 学台区懒加载壳:本文件与 studentNavigation 同属一个打包块,
// 只有通过 RoleGuard 的学生才会下载(铁律 2)。
// 沉浸态(实验工作台/仿真推演/竞赛答题/对局回放)与 MainLayout 并列注册 ——
// 并列而非嵌套,退出沉浸时才能回到本角色区而非站点根(审查 S1),
// 退出目标已在 layouts/immersive/immersiveRoutes.ts 登记。
// 并列的另一个后果是沉浸壳独占全屏:光面日常页在沉浸期间完全退场,不做覆盖层
// (WorkbenchShell 使用契约:覆盖层的背景可被 Tab 穿透与滚动)。

import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { MainLayout } from '../../layouts/main/MainLayout'
import { ImmersiveLayout } from '../../layouts/immersive/ImmersiveLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { studentNavigation } from './studentNavigation'
import { STUDENT_IMMERSIVE_ROUTES } from './studentImmersiveRoutes'

/* 日常页:按侧栏入口与其深页组织,路径只存在于本打包块 */
const StudentCoursesPage = lazy(() => import('../../features/teaching/pages/student/courses'))
const StudentCourseDetailPage = lazy(() => import('../../features/teaching/pages/student/course-detail'))
const StudentLessonPage = lazy(() => import('../../features/teaching/pages/student/lesson'))
const StudentAssignmentPage = lazy(() => import('../../features/teaching/pages/student/assignment'))
const StudentSubmissionsPage = lazy(() => import('../../features/teaching/pages/student/submissions'))
const StudentExperimentsPage = lazy(() => import('../../features/experiment/pages/student/experiments'))
const StudentExperimentDetailPage = lazy(
  () => import('../../features/experiment/pages/student/experiment-detail'),
)
const StudentSimulationsPage = lazy(() => import('../../features/sim/pages/student/simulations'))
const StudentContestsPage = lazy(() => import('../../features/contest/pages/student/contests'))
const StudentContestDetailPage = lazy(() => import('../../features/contest/pages/student/contest-detail'))
const StudentRecordsPage = lazy(() => import('../../features/contest/pages/student/records'))
const StudentGradesPage = lazy(() => import('../../features/grade/pages/student/grades'))
const StudentAlertsPage = lazy(() => import('../../features/grade/pages/student/alerts'))

/* 共享入口:三端共用同一实现,各区只负责注册路由 */
const NotificationInboxPage = lazy(() => import('../../features/notify/pages/inbox'))
const TransferTasksPage = lazy(() => import('../../features/transfer/pages/tasks'))
const ProfilePage = lazy(() => import('../../features/identity/pages/profile/profile'))

/* 沉浸态四条:各自独立成块,只有真的进工作台才下载(Monaco/xterm 都在这些块里) */
const StudentExperimentWorkspacePage = lazy(
  () => import('../../features/experiment/pages/student/workspace'),
)
const StudentSimWorkspacePage = lazy(() => import('../../features/sim/pages/student/workspace'))
const StudentContestWorkspacePage = lazy(
  () => import('../../features/contest/pages/student/workspace'),
)
const StudentContestReplayPage = lazy(() => import('../../features/contest/pages/student/replay'))

/**
 * StudentSection 装配学台区内部路由。
 * 区内路径写在后代 Routes 里(父级以 /student/* 挂载),
 * 因此这些路径只存在于本块,不会进入入口包。
 */
export default function StudentSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={studentNavigation} />}>
        {/* 区根落到侧栏第一个功能页(FE-5:无仪表盘落地页) */}
        <Route index element={<Navigate to={studentNavigation.homePath} replace />} />

        <Route path="courses" element={<StudentCoursesPage />} />
        <Route path="courses/:courseId" element={<StudentCourseDetailPage />} />
        <Route path="courses/:courseId/lessons/:lessonId" element={<StudentLessonPage />} />
        <Route path="courses/:courseId/assignments/:assignmentId" element={<StudentAssignmentPage />} />
        <Route
          path="courses/:courseId/assignments/:assignmentId/submissions"
          element={<StudentSubmissionsPage />}
        />

        <Route path="experiments" element={<StudentExperimentsPage />} />
        <Route path="experiments/:experimentId" element={<StudentExperimentDetailPage />} />

        <Route path="simulations" element={<StudentSimulationsPage />} />

        <Route path="contests" element={<StudentContestsPage />} />
        <Route path="contests/:contestId" element={<StudentContestDetailPage />} />
        <Route path="records" element={<StudentRecordsPage />} />

        <Route path="grades" element={<StudentGradesPage />} />
        <Route path="alerts" element={<StudentAlertsPage />} />

        {/* 顶栏派生的三条共享路径(壳层按 pathPrefix 推导,必须在此注册) */}
        <Route path="notifications" element={<NotificationInboxPage />} />
        <Route path="tasks" element={<TransferTasksPage />} />
        <Route path="profile" element={<ProfilePage canChangePhone />} />

        <Route path="*" element={<RoleNotFoundPage homePath={studentNavigation.homePath} />} />
      </Route>

      {/*
        沉浸态与上面的 MainLayout 并列:同一个角色区内的第二个壳。
        路径写全(不含角色前缀,父级已挂 /student/*),与 immersiveRoutes.ts 的登记逐字对应 ——
        标题与退出目标由那份登记提供,工作台本身不推断自己该退到哪。
      */}
      <Route element={<ImmersiveLayout routes={STUDENT_IMMERSIVE_ROUTES} />}>
        <Route
          path="experiments/:experimentId/workspace"
          element={<StudentExperimentWorkspacePage />}
        />
        <Route path="simulations/:packageCode/workspace" element={<StudentSimWorkspacePage />} />
        <Route path="contests/:contestId/workspace" element={<StudentContestWorkspacePage />} />
        <Route path="contests/:contestId/replay" element={<StudentContestReplayPage />} />
      </Route>
    </Routes>
  )
}
