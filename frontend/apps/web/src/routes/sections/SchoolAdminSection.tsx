// 校管端区懒加载壳:本文件与 schoolAdminNavigation 同属一个打包块,
// 只有通过 RoleGuard 的学校管理员才会下载(铁律 2)。
// 沉浸态是学生侧能力(实验工作台/仿真推演/竞赛答题/对局回放),校管区不注册。

import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AdminScope } from '@chaimir/api-client'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { schoolAdminNavigation } from './schoolAdminNavigation'

/* 日常页:按侧栏入口与其内页组织,路径只存在于本打包块 */
const SchoolAdminUsersPage = lazy(() => import('../../features/identity/pages/school-admin/users'))
const SchoolAdminOrganizationPage = lazy(
  () => import('../../features/identity/pages/school-admin/organization'),
)
const SchoolAdminDashboardPage = lazy(() => import('../../features/admin/pages/school-admin/dashboard'))
const SchoolAdminApprovalsPage = lazy(() => import('../../features/grade/pages/school-admin/approvals'))
const SchoolAdminAppealsPage = lazy(() => import('../../features/grade/pages/school-admin/appeals'))
const SchoolAdminAlertsPage = lazy(() => import('../../features/grade/pages/school-admin/alerts'))
const SchoolAdminGradeSettingsPage = lazy(
  () => import('../../features/grade/pages/school-admin/grade-settings'),
)
const SchoolAdminSettingsPage = lazy(() => import('../../features/identity/pages/school-admin/settings'))
const SchoolAdminAuthConfigPage = lazy(
  () => import('../../features/identity/pages/school-admin/auth-config'),
)
const SchoolAdminAnnouncementsPage = lazy(() => import('../../features/notify/pages/announcements'))
const SchoolAdminAuditPage = lazy(() => import('../../features/admin/pages/school-admin/audit'))
/* 告警页两端共用同一实现,校管端按本校范围声明(平台端声明全局范围) */
const SystemAlertsPage = lazy(() => import('../../features/admin/pages/system-alerts'))

/* 共享入口:三端共用同一实现,各区只负责注册路由 */
const NotificationInboxPage = lazy(() => import('../../features/notify/pages/inbox'))
const TransferTasksPage = lazy(() => import('../../features/transfer/pages/tasks'))
const ProfilePage = lazy(() => import('../../features/identity/pages/profile/profile'))

/**
 * SchoolAdminSection 装配校管端区内部路由。
 * 区内路径写在后代 Routes 里(父级以 /school-admin/* 挂载),
 * 因此这些路径只存在于本块,不会进入入口包。
 */
export default function SchoolAdminSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={schoolAdminNavigation} />}>
        {/* 区根落到侧栏第一个功能页(FE-5:无仪表盘落地页) */}
        <Route index element={<Navigate to={schoolAdminNavigation.homePath} replace />} />

        <Route path="users" element={<SchoolAdminUsersPage />} />
        <Route path="organization" element={<SchoolAdminOrganizationPage />} />

        <Route path="dashboard" element={<SchoolAdminDashboardPage />} />

        <Route path="approvals" element={<SchoolAdminApprovalsPage />} />
        <Route path="appeals" element={<SchoolAdminAppealsPage />} />
        <Route path="alerts" element={<SchoolAdminAlertsPage />} />
        <Route path="grade-settings" element={<SchoolAdminGradeSettingsPage />} />

        <Route path="settings" element={<SchoolAdminSettingsPage />} />
        <Route path="auth-config" element={<SchoolAdminAuthConfigPage />} />
        <Route
          path="announcements"
          element={<SchoolAdminAnnouncementsPage publisher="school" />}
        />
        <Route path="audit" element={<SchoolAdminAuditPage />} />
        <Route path="system-alerts" element={<SystemAlertsPage scope={AdminScope.TENANT} />} />

        {/* 顶栏派生的三条共享路径(壳层按 pathPrefix 推导,必须在此注册) */}
        <Route path="notifications" element={<NotificationInboxPage />} />
        <Route path="tasks" element={<TransferTasksPage />} />
        <Route path="profile" element={<ProfilePage canChangePhone />} />

        <Route path="*" element={<RoleNotFoundPage homePath={schoolAdminNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
