// App.tsx 是单体前端应用的路由组合根,负责按角色懒加载页面并挂载对应布局。
import React, { Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { UserRole } from '@chaimir/api-client'
import AuthLayout from '../layouts/auth/AuthLayout'
import ImmersiveLayout from '../layouts/immersive/ImmersiveLayout'
import AdminLayout from '../layouts/admin/AdminLayout'
import MainLayout from '../layouts/main/MainLayout'
import PlatformLayout from '../layouts/platform/PlatformLayout'
import { RoleGuard } from '../components/RoleGuard'
import styles from './App.module.css'

import {
  LoginPage,
  ForgotPasswordPage,
  ActivatePage,
  ChangePasswordPage,
  TenantSelectPage,
  ApplyPage,
  SSOPage,
  PlatformLoginPage,
  StudentCoursesPage,
  StudentCourseDetailPage,
  StudentLessonPage,
  StudentAssignmentPage,
  StudentAssignmentResultPage,
  StudentExperimentsPage,
  StudentExperimentDetailPage,
  StudentExperimentWorkspacePage,
  StudentSimulationsPage,
  StudentSimulationWorkspacePage,
  StudentContestsPage,
  StudentContestDetailPage,
  StudentContestApplyPage,
  StudentContestWorkspacePage,
  StudentContestReplayPage,
  StudentRecordsPage,
  StudentRecordProfilePage,
  StudentGradesPage,
  StudentAlertsPage,
  TeacherCoursesPage,
  TeacherCourseEditPage,
  TeacherCourseOutlinePage,
  TeacherCourseMembersPage,
  TeacherCourseDiscussionPage,
  TeacherCourseAssignmentsPage,
  TeacherCourseAssignmentEditPage,
  TeacherGradingPage,
  TeacherExperimentsPage,
  TeacherExperimentOrchestrationPage,
  TeacherExperimentGradingPage,
  TeacherContestsPage,
  TeacherContestConfigPage,
  TeacherContestAuthoringPage,
  TeacherMonitoringPage,
  TeacherAntiCheatPage,
  TeacherVulnerabilitiesPage,
  TeacherVulnerabilityWizardPage,
  TeacherQuestionsPage,
  TeacherQuestionCategoriesPage,
  TeacherQuestionEditPage,
  TeacherExamsPage,
  TeacherExamsEditPage,
  TeacherSimulationsPage,
  TeacherSharedPage,
  TeacherGradesPage,
  TeacherGradesDetailsPage,
  TeacherGradesAppealsPage,
  TeacherOrganizationPage,
  SchoolAdminUsersPage,
  SchoolAdminUserEditPage,
  SchoolAdminUserImportPage,
  SchoolAdminUserHistoryPage,
  SchoolAdminOrganizationPage,
  SchoolAdminDashboardPage,
  SchoolAdminStatisticsPage,
  SchoolAdminApprovalsPage,
  SchoolAdminAppealsPage,
  SchoolAdminAlertsPage,
  SchoolAdminGradeSettingsPage,
  SchoolAdminSettingsPage,
  SchoolAdminAuthConfigPage,
  SchoolAdminAuditPage,
  SchoolAdminSystemAlertsPage,
  SchoolAdminAnnouncementsPage,
  PlatformSchoolsPage,
  PlatformSchoolDetailPage,
  PlatformSchoolQuotasPage,
  PlatformApplicationsPage,
  PlatformApplicationDetailPage,
  PlatformDashboardPage,
  PlatformStatisticsPage,
  PlatformRuntimesPage,
  PlatformRuntimeDetailPage,
  PlatformSandboxToolsPage,
  PlatformJudgesPage,
  PlatformSimulationsPage,
  PlatformVulnerabilitiesPage,
  PlatformAlertsPage,
  PlatformAlertRulesPage,
  PlatformSettingsPage,
  PlatformMonitoringPage,
  PlatformBackupsPage,
  PlatformAuditPage,
  NotificationsPage,
  TasksPage,
  ProfilePage,
  NotFoundPage
} from '../routes/lazy-pages'
import { platformLayerEnabled } from './config'

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
          {platformLayerEnabled ? <Route path="/platform/*" element={<Navigate to="/platform-admin" replace />} /> : null}

          <Route path="/auth" element={<AuthLayout />}>
            <Route path="login" element={<LoginPage />} />
            <Route path="forgot" element={<ForgotPasswordPage />} />
            <Route path="activate" element={<ActivatePage />} />
            <Route path="change-pwd" element={<ChangePasswordPage />} />
            <Route path="tenant-select" element={<TenantSelectPage />} />
          </Route>
          <Route path="/auth/apply" element={<ApplyPage />} />
          <Route path="/auth/sso" element={<SSOPage />} />
          {platformLayerEnabled ? <Route path="/auth/platform-login" element={<PlatformLoginPage />} /> : null}

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

          {platformLayerEnabled ? (
            <Route element={<RoleGuard allowedRoles={[UserRole.PLATFORM_ADMIN]} />}>
              <Route path="/platform-admin" element={<PlatformLayout />}>
                <Route index element={<Navigate to="schools" replace />} />
                <Route path="schools" element={<PlatformSchoolsPage />} />
                <Route path="schools/:id" element={<PlatformSchoolDetailPage />} />
                <Route path="schools/:id/quotas" element={<PlatformSchoolQuotasPage />} />
                <Route path="applications" element={<PlatformApplicationsPage />} />
                <Route path="applications/:id" element={<PlatformApplicationDetailPage />} />
                <Route path="dashboard" element={<PlatformDashboardPage />} />
                <Route path="dashboard/statistics" element={<PlatformStatisticsPage />} />
                <Route path="runtimes" element={<PlatformRuntimesPage />} />
                <Route path="runtimes/:id" element={<PlatformRuntimeDetailPage />} />
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
          ) : null}

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
