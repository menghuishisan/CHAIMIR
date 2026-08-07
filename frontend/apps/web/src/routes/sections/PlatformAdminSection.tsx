// 平台管理区懒加载壳:本文件与 platformAdminNavigation 同属一个打包块,
// 只有 SaaS 形态下通过 RoleGuard 的平台管理员才会下载 —— 这是铁律 2 的关键落点,
// 未登录访客的入口包里不得出现任何 /platform-admin 路径或后台栏目名。
//
// 平台端没有通知收件箱(导航配置 hasNotificationInbox=false):站内信要求租户身份,
// 平台账号无租户,顶栏不渲染铃铛,故本区不注册 notifications 路由。
// 沉浸态(实验工作台/仿真推演/竞赛答题/对局回放)是学生侧能力,平台区不注册。

import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router'
import { AdminScope } from '@chaimir/api-client'
import { MainLayout } from '../../layouts/main/MainLayout'
import { RoleNotFoundPage } from '../../components/StatusPages'
import { platformAdminNavigation } from './platformAdminNavigation'

/* 租户:学校与入驻申请,各带一个深页 */
const PlatformSchoolsPage = lazy(
  () => import('../../features/identity/pages/platform-admin/schools'),
)
const PlatformSchoolDetailPage = lazy(
  () => import('../../features/identity/pages/platform-admin/school-detail'),
)
const PlatformApplicationsPage = lazy(
  () => import('../../features/identity/pages/platform-admin/applications'),
)
const PlatformApplicationDetailPage = lazy(
  () => import('../../features/identity/pages/platform-admin/application-detail'),
)

/* 运营:平台看板(运营统计是它的内页区块) */
const PlatformDashboardPage = lazy(
  () => import('../../features/admin/pages/platform-admin/dashboard'),
)

/* 底层资源 */
const PlatformRuntimesPage = lazy(
  () => import('../../features/sandbox/pages/platform-admin/runtimes'),
)
const PlatformRuntimeDetailPage = lazy(
  () => import('../../features/sandbox/pages/platform-admin/runtime-detail'),
)
const PlatformSandboxToolsPage = lazy(
  () => import('../../features/sandbox/pages/platform-admin/sandbox-tools'),
)
const PlatformJudgesPage = lazy(() => import('../../features/judge/pages/platform-admin/judges'))
const PlatformSimulationsPage = lazy(
  () => import('../../features/sim/pages/platform-admin/simulations'),
)
const PlatformVulnerabilitiesPage = lazy(
  () => import('../../features/contest/pages/platform-admin/vulnerabilities'),
)
const PlatformSettingsPage = lazy(
  () => import('../../features/admin/pages/platform-admin/settings'),
)
const PlatformMonitoringPage = lazy(
  () => import('../../features/admin/pages/platform-admin/monitoring'),
)
const PlatformBackupsPage = lazy(() => import('../../features/admin/pages/platform-admin/backups'))
const PlatformAuditPage = lazy(() => import('../../features/admin/pages/platform-admin/audit'))

/* 两端共用同一实现,按范围与发布方声明 */
const SystemAlertsPage = lazy(() => import('../../features/admin/pages/system-alerts'))
const AnnouncementsPage = lazy(() => import('../../features/notify/pages/announcements'))

/* 共享入口:平台端只有任务与下载、个人中心(无收件箱) */
const TransferTasksPage = lazy(() => import('../../features/transfer/pages/tasks'))
const ProfilePage = lazy(() => import('../../features/identity/pages/profile/profile'))

/**
 * PlatformAdminSection 装配平台管理区内部路由。
 * 区内路径写在后代 Routes 里(父级以 /platform-admin/* 挂载),
 * 因此这些路径只存在于本块,不会进入入口包。
 */
export default function PlatformAdminSection() {
  return (
    <Routes>
      <Route element={<MainLayout config={platformAdminNavigation} />}>
        {/* 区根落到侧栏第一个功能页(FE-5:无仪表盘落地页) */}
        <Route index element={<Navigate to={platformAdminNavigation.homePath} replace />} />

        <Route path="schools" element={<PlatformSchoolsPage />} />
        <Route path="schools/:tenantId" element={<PlatformSchoolDetailPage />} />
        <Route path="applications" element={<PlatformApplicationsPage />} />
        <Route path="applications/:applicationId" element={<PlatformApplicationDetailPage />} />

        <Route path="dashboard" element={<PlatformDashboardPage />} />

        <Route path="runtimes" element={<PlatformRuntimesPage />} />
        <Route path="runtimes/:runtimeId" element={<PlatformRuntimeDetailPage />} />
        <Route path="sandbox-tools" element={<PlatformSandboxToolsPage />} />
        <Route path="judges" element={<PlatformJudgesPage />} />
        <Route path="simulations" element={<PlatformSimulationsPage />} />
        <Route path="vulnerabilities" element={<PlatformVulnerabilitiesPage />} />
        <Route path="alerts" element={<SystemAlertsPage scope={AdminScope.GLOBAL} />} />
        <Route path="announcements" element={<AnnouncementsPage publisher="platform" />} />
        <Route path="settings" element={<PlatformSettingsPage />} />
        <Route path="monitoring" element={<PlatformMonitoringPage />} />
        <Route path="backups" element={<PlatformBackupsPage />} />
        <Route path="audit" element={<PlatformAuditPage />} />

        {/* 顶栏派生的共享路径:平台端无收件箱,故只有这两条 */}
        <Route path="tasks" element={<TransferTasksPage />} />
        {/* 平台账号无手机号列,不渲染换绑手机号(后端 ChangeMyPhone 拒平台身份) */}
        <Route path="profile" element={<ProfilePage canChangePhone={false} />} />

        <Route path="*" element={<RoleNotFoundPage homePath={platformAdminNavigation.homePath} />} />
      </Route>
    </Routes>
  )
}
