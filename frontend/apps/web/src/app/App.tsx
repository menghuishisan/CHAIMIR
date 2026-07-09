// App.tsx 是单体前端应用的路由组合根,负责按角色懒加载页面并挂载对应布局。
import React, { Suspense, lazy } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { UserRole } from '@chaimir/api-client'
import AuthLayout from '../layouts/auth/AuthLayout'
import ImmersiveLayout from '../layouts/immersive/ImmersiveLayout'
import AdminLayout from '../layouts/admin/AdminLayout'
import MainLayout from '../layouts/main/MainLayout'
import PlatformLayout from '../layouts/platform/PlatformLayout'
import { RoleGuard } from '../components/RoleGuard'
import styles from './App.module.css'

const LoginPage = lazy(() => import('../pages/auth/login'))
const ForgotPasswordPage = lazy(() => import('../pages/auth/forgot'))
const ActivatePage = lazy(() => import('../pages/auth/activate'))
const TenantSelectPage = lazy(() => import('../pages/auth/tenant-select'))
const ApplyPage = lazy(() => import('../pages/auth/apply'))
const SSOPage = lazy(() => import('../pages/auth/sso'))
const PlatformLoginPage = lazy(() => import('../pages/auth/platform-login'))

const StudentCoursesPage = lazy(() => import('../pages/student/courses'))
const StudentCourseDetailPage = lazy(() => import('../pages/student/courses/detail'))
const StudentLessonPage = lazy(() => import('../pages/student/courses/lesson'))
const StudentAssignmentPage = lazy(() => import('../pages/student/courses/assignment'))
const StudentAssignmentResultPage = lazy(() => import('../pages/student/courses/result'))
const StudentExperimentsPage = lazy(() => import('../pages/student/experiments'))
const StudentExperimentDetailPage = lazy(() => import('../pages/student/experiments/detail'))
const StudentExperimentWorkspacePage = lazy(() => import('../pages/student/experiments/workspace'))
const StudentSimulationsPage = lazy(() => import('../pages/student/simulations'))
const StudentSimulationWorkspacePage = lazy(() => import('../pages/student/simulations/workspace'))
const StudentContestsPage = lazy(() => import('../pages/student/contests'))
const StudentContestDetailPage = lazy(() => import('../pages/student/contests/detail'))
const StudentContestApplyPage = lazy(() => import('../pages/student/contests/apply'))
const StudentContestWorkspacePage = lazy(() => import('../pages/student/contests/workspace'))
const StudentContestReplayPage = lazy(() => import('../pages/student/contests/replay'))
const StudentRecordsPage = lazy(() => import('../pages/student/records'))
const StudentRecordProfilePage = lazy(() => import('../pages/student/records/profile'))
const StudentGradesPage = lazy(() => import('../pages/student/grades'))
const StudentAlertsPage = lazy(() => import('../pages/student/alerts'))

const TeacherCoursesPage = lazy(() => import('../pages/teacher/courses'))
const TeacherCourseEditPage = lazy(() => import('../pages/teacher/courses/edit'))
const TeacherCourseOutlinePage = lazy(() => import('../pages/teacher/courses/outline'))
const TeacherCourseMembersPage = lazy(() => import('../pages/teacher/courses/members'))
const TeacherCourseDiscussionPage = lazy(() => import('../pages/teacher/courses/discussion'))
const TeacherCourseAssignmentsPage = lazy(() => import('../pages/teacher/courses/assignments'))
const TeacherCourseAssignmentEditPage = lazy(() => import('../pages/teacher/courses/assignments/edit'))
const TeacherGradingPage = lazy(() => import('../pages/teacher/grading'))
const TeacherExperimentsPage = lazy(() => import('../pages/teacher/experiments'))
const TeacherExperimentOrchestrationPage = lazy(() => import('../pages/teacher/experiments/orchestration'))
const TeacherExperimentGradingPage = lazy(() => import('../pages/teacher/experiments/grading'))
const TeacherContestsPage = lazy(() => import('../pages/teacher/contests'))
const TeacherContestConfigPage = lazy(() => import('../pages/teacher/contests/config'))
const TeacherContestAuthoringPage = lazy(() => import('../pages/teacher/contests/authoring'))
const TeacherMonitoringPage = lazy(() => import('../pages/teacher/monitoring'))
const TeacherAntiCheatPage = lazy(() => import('../pages/teacher/monitoring/anti-cheat'))
const TeacherVulnerabilitiesPage = lazy(() => import('../pages/teacher/vulnerabilities'))
const TeacherVulnerabilityWizardPage = lazy(() => import('../pages/teacher/vulnerabilities/wizard'))
const TeacherQuestionsPage = lazy(() => import('../pages/teacher/questions'))
const TeacherQuestionCategoriesPage = lazy(() => import('../pages/teacher/questions/categories'))
const TeacherQuestionEditPage = lazy(() => import('../pages/teacher/questions/edit'))
const TeacherExamsPage = lazy(() => import('../pages/teacher/exams'))
const TeacherExamsEditPage = lazy(() => import('../pages/teacher/exams/edit'))
const TeacherSimulationsPage = lazy(() => import('../pages/teacher/simulations'))
const TeacherSharedPage = lazy(() => import('../pages/teacher/shared'))
const TeacherGradesPage = lazy(() => import('../pages/teacher/grades'))
const TeacherGradesDetailsPage = lazy(() => import('../pages/teacher/grades/details'))
const TeacherGradesAppealsPage = lazy(() => import('../pages/teacher/grades/appeals'))
const TeacherOrganizationPage = lazy(() => import('../pages/teacher/organization'))

const SchoolAdminUsersPage = lazy(() => import('../pages/school-admin/users'))
const SchoolAdminUserEditPage = lazy(() => import('../pages/school-admin/users/edit'))
const SchoolAdminUserImportPage = lazy(() => import('../pages/school-admin/users/import'))
const SchoolAdminUserHistoryPage = lazy(() => import('../pages/school-admin/users/history'))
const SchoolAdminOrganizationPage = lazy(() => import('../pages/school-admin/organization'))
const SchoolAdminDashboardPage = lazy(() => import('../pages/school-admin/dashboard'))
const SchoolAdminStatisticsPage = lazy(() => import('../pages/school-admin/dashboard/statistics'))
const SchoolAdminApprovalsPage = lazy(() => import('../pages/school-admin/approvals'))
const SchoolAdminAppealsPage = lazy(() => import('../pages/school-admin/appeals'))
const SchoolAdminAlertsPage = lazy(() => import('../pages/school-admin/alerts'))
const SchoolAdminGradeSettingsPage = lazy(() => import('../pages/school-admin/grade-settings'))
const SchoolAdminSettingsPage = lazy(() => import('../pages/school-admin/settings'))
const SchoolAdminAuthConfigPage = lazy(() => import('../pages/school-admin/auth-config'))
const SchoolAdminAuditPage = lazy(() => import('../pages/school-admin/audit'))
const SchoolAdminSystemAlertsPage = lazy(() => import('../pages/school-admin/system-alerts'))
const SchoolAdminAnnouncementsPage = lazy(() => import('../pages/school-admin/announcements'))

const PlatformSchoolsPage = lazy(() => import('../pages/platform-admin/schools'))
const PlatformSchoolDetailPage = lazy(() => import('../pages/platform-admin/schools/detail'))
const PlatformSchoolQuotasPage = lazy(() => import('../pages/platform-admin/schools/quotas'))
const PlatformApplicationsPage = lazy(() => import('../pages/platform-admin/applications'))
const PlatformApplicationDetailPage = lazy(() => import('../pages/platform-admin/applications/detail'))
const PlatformDashboardPage = lazy(() => import('../pages/platform-admin/dashboard'))
const PlatformRuntimesPage = lazy(() => import('../pages/platform-admin/runtimes'))
const PlatformSandboxToolsPage = lazy(() => import('../pages/platform-admin/sandbox-tools'))
const PlatformJudgesPage = lazy(() => import('../pages/platform-admin/judges'))
const PlatformSimulationsPage = lazy(() => import('../pages/platform-admin/simulations'))
const PlatformVulnerabilitiesPage = lazy(() => import('../pages/platform-admin/vulnerabilities'))
const PlatformAlertsPage = lazy(() => import('../pages/platform-admin/alerts'))
const PlatformAlertRulesPage = lazy(() => import('../pages/platform-admin/alerts/rules'))
const PlatformSettingsPage = lazy(() => import('../pages/platform-admin/settings'))
const PlatformMonitoringPage = lazy(() => import('../pages/platform-admin/monitoring'))
const PlatformBackupsPage = lazy(() => import('../pages/platform-admin/backups'))
const PlatformAuditPage = lazy(() => import('../pages/platform-admin/audit'))

const NotificationsPage = lazy(() => import('../pages/shared/notifications'))
const TasksPage = lazy(() => import('../pages/shared/tasks'))
const ProfilePage = lazy(() => import('../pages/shared/profile'))
const NotFoundPage = lazy(() => import('./NotFoundPage'))

// PageFallback 为懒加载页面提供轻量反馈,避免首屏或切页时出现空白区域。
const PageFallback: React.FC = () => (
  <div role="status" aria-live="polite" className={styles.pageFallback}>
    页面正在加载,请稍候
  </div>
)

// App 挂载浏览器路由,并把旧角色路径重定向到当前规范路径。
const App: React.FC = () => {
  return (
    <BrowserRouter>
      <Suspense fallback={<PageFallback />}>
        <Routes>
          <Route path="/" element={<Navigate to="/auth/login" replace />} />
          <Route path="/admin/*" element={<Navigate to="/school-admin" replace />} />
          <Route path="/platform/*" element={<Navigate to="/platform-admin" replace />} />

          <Route path="/auth" element={<AuthLayout />}>
            <Route path="login" element={<LoginPage />} />
            <Route path="forgot" element={<ForgotPasswordPage />} />
            <Route path="activate" element={<ActivatePage />} />
            <Route path="tenant-select" element={<TenantSelectPage />} />
          </Route>
          <Route path="/auth/apply" element={<ApplyPage />} />
          <Route path="/auth/sso" element={<SSOPage />} />
          <Route path="/auth/platform-login" element={<PlatformLoginPage />} />

          <Route element={<RoleGuard allowedRoles={[UserRole.SCHOOL_ADMIN]} />}>
            <Route path="/school-admin" element={<AdminLayout />}>
              <Route index element={<Navigate to="users" replace />} />
              <Route path="users" element={<SchoolAdminUsersPage />} />
              <Route path="users/edit" element={<SchoolAdminUserEditPage />} />
              <Route path="users/import" element={<SchoolAdminUserImportPage />} />
              <Route path="users/history" element={<SchoolAdminUserHistoryPage />} />
              <Route path="organization" element={<SchoolAdminOrganizationPage />} />
              <Route path="dashboard" element={<SchoolAdminDashboardPage />} />
              <Route path="dashboard/statistics" element={<SchoolAdminStatisticsPage />} />
              <Route path="approvals" element={<SchoolAdminApprovalsPage />} />
              <Route path="appeals" element={<SchoolAdminAppealsPage />} />
              <Route path="alerts" element={<SchoolAdminAlertsPage />} />
              <Route path="grade-settings" element={<SchoolAdminGradeSettingsPage />} />
              <Route path="settings" element={<SchoolAdminSettingsPage />} />
              <Route path="auth-config" element={<SchoolAdminAuthConfigPage />} />
              <Route path="audit" element={<SchoolAdminAuditPage />} />
              <Route path="system-alerts" element={<SchoolAdminSystemAlertsPage />} />
              <Route path="announcements" element={<SchoolAdminAnnouncementsPage />} />
              <Route path="notifications" element={<NotificationsPage />} />
              <Route path="tasks" element={<TasksPage />} />
              <Route path="profile" element={<ProfilePage />} />
            </Route>
          </Route>

          <Route element={<RoleGuard allowedRoles={[UserRole.PLATFORM_ADMIN]} />}>
            <Route path="/platform-admin" element={<PlatformLayout />}>
              <Route index element={<Navigate to="schools" replace />} />
              <Route path="schools" element={<PlatformSchoolsPage />} />
              <Route path="schools/:id" element={<PlatformSchoolDetailPage />} />
              <Route path="schools/:id/quotas" element={<PlatformSchoolQuotasPage />} />
              <Route path="applications" element={<PlatformApplicationsPage />} />
              <Route path="applications/:id" element={<PlatformApplicationDetailPage />} />
              <Route path="dashboard" element={<PlatformDashboardPage />} />
              <Route path="runtimes" element={<PlatformRuntimesPage />} />
              <Route path="sandbox-tools" element={<PlatformSandboxToolsPage />} />
              <Route path="judges" element={<PlatformJudgesPage />} />
              <Route path="simulations" element={<PlatformSimulationsPage />} />
              <Route path="vulnerabilities" element={<PlatformVulnerabilitiesPage />} />
              <Route path="alerts" element={<PlatformAlertsPage />} />
              <Route path="alerts/rules" element={<PlatformAlertRulesPage />} />
              <Route path="settings" element={<PlatformSettingsPage />} />
              <Route path="monitoring" element={<PlatformMonitoringPage />} />
              <Route path="backups" element={<PlatformBackupsPage />} />
              <Route path="audit" element={<PlatformAuditPage />} />
              <Route path="notifications" element={<NotificationsPage />} />
              <Route path="tasks" element={<TasksPage />} />
              <Route path="profile" element={<ProfilePage />} />
            </Route>
          </Route>

          <Route element={<RoleGuard allowedRoles={[UserRole.STUDENT]} />}>
            <Route path="/student" element={<MainLayout />}>
              <Route index element={<Navigate to="courses" replace />} />
              <Route path="courses" element={<StudentCoursesPage />} />
              <Route path="courses/:id" element={<StudentCourseDetailPage />} />
              <Route path="courses/:id/lesson/:lessonId" element={<StudentLessonPage />} />
              <Route path="courses/assignment/:id" element={<StudentAssignmentPage />} />
              <Route path="courses/assignment/:id/result" element={<StudentAssignmentResultPage />} />
              <Route path="experiments" element={<StudentExperimentsPage />} />
              <Route path="experiments/:id" element={<StudentExperimentDetailPage />} />
              <Route path="simulations" element={<StudentSimulationsPage />} />
              <Route path="contests" element={<StudentContestsPage />} />
              <Route path="contests/:id" element={<StudentContestDetailPage />} />
              <Route path="contests/:id/apply" element={<StudentContestApplyPage />} />
              <Route path="records" element={<StudentRecordsPage />} />
              <Route path="records/profile" element={<StudentRecordProfilePage />} />
              <Route path="grades" element={<StudentGradesPage />} />
              <Route path="alerts" element={<StudentAlertsPage />} />
              <Route path="notifications" element={<NotificationsPage />} />
              <Route path="tasks" element={<TasksPage />} />
              <Route path="profile" element={<ProfilePage />} />
            </Route>

            <Route path="/student" element={<ImmersiveLayout />}>
              <Route path="experiments/:id/workspace" element={<StudentExperimentWorkspacePage />} />
              <Route path="simulations/:id/workspace" element={<StudentSimulationWorkspacePage />} />
              <Route path="contests/:id/workspace" element={<StudentContestWorkspacePage />} />
              <Route path="contests/:id/replay" element={<StudentContestReplayPage />} />
            </Route>
          </Route>

          <Route element={<RoleGuard allowedRoles={[UserRole.TEACHER]} />}>
            <Route path="/teacher" element={<MainLayout />}>
              <Route index element={<Navigate to="courses" replace />} />
              <Route path="courses" element={<TeacherCoursesPage />} />
              <Route path="courses/edit" element={<TeacherCourseEditPage />} />
              <Route path="courses/:id/outline" element={<TeacherCourseOutlinePage />} />
              <Route path="courses/:id/members" element={<TeacherCourseMembersPage />} />
              <Route path="courses/:id/discussion" element={<TeacherCourseDiscussionPage />} />
              <Route path="courses/assignments" element={<TeacherCourseAssignmentsPage />} />
              <Route path="courses/assignments/edit" element={<TeacherCourseAssignmentEditPage />} />
              <Route path="grading" element={<TeacherGradingPage />} />
              <Route path="experiments" element={<TeacherExperimentsPage />} />
              <Route path="experiments/orchestration" element={<TeacherExperimentOrchestrationPage />} />
              <Route path="experiments/:id/grading" element={<TeacherExperimentGradingPage />} />
              <Route path="contests" element={<TeacherContestsPage />} />
              <Route path="contests/config" element={<TeacherContestConfigPage />} />
              <Route path="contests/:id/config" element={<TeacherContestConfigPage />} />
              <Route path="contests/:id/authoring" element={<TeacherContestAuthoringPage />} />
              <Route path="monitoring" element={<TeacherMonitoringPage />} />
              <Route path="monitoring/anti-cheat" element={<TeacherAntiCheatPage />} />
              <Route path="questions" element={<TeacherQuestionsPage />} />
              <Route path="questions/categories" element={<TeacherQuestionCategoriesPage />} />
              <Route path="questions/edit" element={<TeacherQuestionEditPage />} />
              <Route path="exams" element={<TeacherExamsPage />} />
              <Route path="exams/edit" element={<TeacherExamsEditPage />} />
              <Route path="vulnerabilities" element={<TeacherVulnerabilitiesPage />} />
              <Route path="vulnerabilities/wizard" element={<TeacherVulnerabilityWizardPage />} />
              <Route path="simulations" element={<TeacherSimulationsPage />} />
              <Route path="shared" element={<TeacherSharedPage />} />
              <Route path="grades" element={<TeacherGradesPage />} />
              <Route path="grades/details" element={<TeacherGradesDetailsPage />} />
              <Route path="grades/appeals" element={<TeacherGradesAppealsPage />} />
              <Route path="organization" element={<TeacherOrganizationPage />} />
              <Route path="notifications" element={<NotificationsPage />} />
              <Route path="tasks" element={<TasksPage />} />
              <Route path="profile" element={<ProfilePage />} />
            </Route>
          </Route>

          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </Suspense>
    </BrowserRouter>
  )
}

export default App
