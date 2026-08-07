// roleRoutes 四端角色区路由清单(目录设计 §4 铁律 2:清单 + 懒加载入口 + 权限装配都在路由层)。
//
// 每个角色区一律 RoleGuard(服务端会话定角色,不接受客户端传参)→ 该区懒加载壳。
// 区壳自带本角色的导航数据与区内页面清单,因此:
//   - 各角色的路径清单与栏目名只出现在自己的打包块里,入口包不含任何后台路由结构;
//   - 壳层(MainLayout)不认识全角色清单,越权访问在守卫处就被拦下,连代码碎片都拿不到。
// 平台管理区仅 SaaS 形态注册 —— 私有化部署下该路径不存在,而非前端隐藏入口。

import { lazy } from 'react'
import type { ReactNode } from 'react'
import { Route } from 'react-router'
import { UserRole } from '@chaimir/api-client'
import { RoleGuard } from '../components/RoleGuard'
import { ROLE_ROUTES } from '../utils/roleRouting'

/* 懒加载:每个区一个独立块,守卫通过后才会发起请求 */
const PlatformAdminSection = lazy(() => import('./sections/PlatformAdminSection'))
const SchoolAdminSection = lazy(() => import('./sections/SchoolAdminSection'))
const TeacherSection = lazy(() => import('./sections/TeacherSection'))
const StudentSection = lazy(() => import('./sections/StudentSection'))

/**
 * roleSection 生成「守卫 + 该区懒加载壳」两层。
 * 路径带 `/*` 是因为区内清单写在壳内部的后代 Routes 里 —— 这正是路径不进入口包的原因。
 */
function roleSection(options: {
  key: string
  pathPrefix: string
  allowedRoles: UserRole[]
  section: ReactNode
}) {
  const { key, pathPrefix, allowedRoles, section } = options
  return (
    <Route key={key} element={<RoleGuard allowedRoles={allowedRoles} />}>
      <Route path={`${pathPrefix}/*`} element={section} />
    </Route>
  )
}

/**
 * roleRoutes 返回四端角色区路由。
 * platformEnabled 决定平台管理区是否注册(与后端 /platform 路由注册条件一致)。
 */
export function roleRoutes(platformEnabled: boolean) {
  return (
    <>
      {platformEnabled
        ? roleSection({
            key: 'platform-admin',
            pathPrefix: ROLE_ROUTES.platformAdmin.pathPrefix,
            allowedRoles: [UserRole.PLATFORM_ADMIN],
            section: <PlatformAdminSection />,
          })
        : null}
      {roleSection({
        key: 'school-admin',
        pathPrefix: ROLE_ROUTES.schoolAdmin.pathPrefix,
        allowedRoles: [UserRole.SCHOOL_ADMIN],
        section: <SchoolAdminSection />,
      })}
      {roleSection({
        key: 'teacher',
        pathPrefix: ROLE_ROUTES.teacher.pathPrefix,
        allowedRoles: [UserRole.TEACHER],
        section: <TeacherSection />,
      })}
      {roleSection({
        key: 'student',
        pathPrefix: ROLE_ROUTES.student.pathPrefix,
        allowedRoles: [UserRole.STUDENT],
        section: <StudentSection />,
      })}
    </>
  )
}
