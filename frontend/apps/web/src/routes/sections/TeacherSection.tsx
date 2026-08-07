// 教学端区懒加载壳:本文件与 teacherNavigation 同属一个打包块,
// 只有通过 RoleGuard 的教师才会下载(铁律 2)。
// 沉浸态是学生侧能力(实验工作台/仿真推演/竞赛答题/对局回放),教师区不注册。

import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { teacherNavigation } from './teacherNavigation'

/* 日常页:按侧栏入口与其深页组织,路径只存在于本打包块 */
const TeacherCoursesPage = lazy(() => import('../../features/teaching/pages/teacher/courses'))
const TeacherCourseDetailPage = lazy(() => import('../../features/teaching/pages/teacher/course-detail'))
const TeacherGradingPage = lazy(() => import('../../features/teaching/pages/teacher/grading'))
const TeacherMonitoringPage = lazy(() => import('../../features/teaching/pages/teacher/monitoring'))
const TeacherExperimentsPage = lazy(() => import('../../features/experiment/pages/teacher/experiments'))
const TeacherExperimentWizardPage = lazy(
  () => import('../../features/experiment/pages/teacher/experiment-wizard'),
)
const TeacherExperimentReportsPage = lazy(
  () => import('../../features/experiment/pages/teacher/experiment-reports'),
)
const TeacherContestsPage = lazy(() => import('../../features/contest/pages/teacher/contests'))
const TeacherContestDetailPage = lazy(() => import('../../features/contest/pages/teacher/contest-detail'))
const TeacherVulnWorkshopPage = lazy(() => import('../../features/contest/pages/teacher/vuln-workshop'))
const TeacherQuestionsPage = lazy(() => import('../../features/content/pages/teacher/questions'))
const TeacherExamsPage = lazy(() => import('../../features/content/pages/teacher/exams'))
const TeacherSimulationsPage = lazy(() => import('../../features/sim/pages/teacher/simulations'))
const TeacherSharedLibraryPage = lazy(() => import('../../features/content/pages/teacher/shared-library'))
const TeacherGradesPage = lazy(() => import('../../features/grade/pages/teacher/grades'))
const TeacherOrganizationPage = lazy(() => import('../../features/identity/pages/teacher/organization'))

/* 共享入口:三端共用同一实现,各区只负责注册路由 */
const NotificationInboxPage = lazy(() => import('../../features/notify/pages/inbox'))
const TransferTasksPage = lazy(() => import('../../features/transfer/pages/tasks'))
const ProfilePage = lazy(() => import('../../features/identity/pages/profile/profile'))

/**
 * TeacherSection 装配教学端区内部路由。
 * 区内路径写在后代 Routes 里(父级以 /teacher/* 挂载),
 * 因此这些路径只存在于本块,不会进入入口包。
 */
export default function TeacherSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={teacherNavigation} />}>
        {/* 区根落到侧栏第一个功能页(FE-5:无仪表盘落地页) */}
        <Route index element={<Navigate to={teacherNavigation.homePath} replace />} />

        <Route path="courses" element={<TeacherCoursesPage />} />
        <Route path="courses/:courseId" element={<TeacherCourseDetailPage />} />
        <Route path="grading" element={<TeacherGradingPage />} />

        {/* 实验编排向导:新建与继续编排是同一页面,按 experimentId 分叉 */}
        <Route path="experiments" element={<TeacherExperimentsPage />} />
        <Route path="experiments/new" element={<TeacherExperimentWizardPage />} />
        <Route path="experiments/:experimentId" element={<TeacherExperimentWizardPage />} />
        {/* 报告批改与小组编排是实验的下游动作,按实验进入 */}
        <Route path="experiments/:experimentId/reports" element={<TeacherExperimentReportsPage />} />

        <Route path="contests" element={<TeacherContestsPage />} />
        {/* 赛事深页:赛题编排 + 防作弊审查 + 归档榜单 */}
        <Route path="contests/:contestId" element={<TeacherContestDetailPage />} />
        {/* 漏洞题工坊:漏洞源维护与漏洞题转化,从赛事组织进入 */}
        <Route path="vuln-workshop" element={<TeacherVulnWorkshopPage />} />
        <Route path="monitoring" element={<TeacherMonitoringPage />} />

        <Route path="questions" element={<TeacherQuestionsPage />} />
        <Route path="exams" element={<TeacherExamsPage />} />
        <Route path="simulations" element={<TeacherSimulationsPage />} />
        <Route path="shared" element={<TeacherSharedLibraryPage />} />

        <Route path="grades" element={<TeacherGradesPage />} />
        <Route path="organization" element={<TeacherOrganizationPage />} />

        {/* 顶栏派生的三条共享路径(壳层按 pathPrefix 推导,必须在此注册) */}
        <Route path="notifications" element={<NotificationInboxPage />} />
        <Route path="tasks" element={<TransferTasksPage />} />
        <Route path="profile" element={<ProfilePage canChangePhone />} />

        <Route path="*" element={<RoleNotFoundPage homePath={teacherNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
