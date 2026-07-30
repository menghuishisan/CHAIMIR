// roleRouting 定义角色路径前缀、登录入口和默认功能页的唯一应用契约。

import { UserRole } from '@chaimir/api-client'

export interface RoleRouteConfig {
  role: UserRole
  pathPrefix: string
  homePath: string
  /** 退出登录与鉴权失败的回落入口;平台管理员与学校侧用户入口分离(仅 SaaS 存在平台入口) */
  loginPath: string
}

export const ROLE_ROUTES = {
  platformAdmin: { role: UserRole.PLATFORM_ADMIN, pathPrefix: '/platform-admin', homePath: '/platform-admin/schools', loginPath: '/auth/platform-login' },
  schoolAdmin: { role: UserRole.SCHOOL_ADMIN, pathPrefix: '/school-admin', homePath: '/school-admin/users', loginPath: '/auth/login' },
  teacher: { role: UserRole.TEACHER, pathPrefix: '/teacher', homePath: '/teacher/courses', loginPath: '/auth/login' },
  student: { role: UserRole.STUDENT, pathPrefix: '/student', homePath: '/student/courses', loginPath: '/auth/login' },
} satisfies Record<string, RoleRouteConfig>

const ROLE_ROUTE_PRIORITY: RoleRouteConfig[] = [
  ROLE_ROUTES.platformAdmin,
  ROLE_ROUTES.schoolAdmin,
  ROLE_ROUTES.teacher,
  ROLE_ROUTES.student,
]

/** roleRouteForRoles 按平台管理、学校管理、教师、学生的固定优先级选择已授权入口。 */
export function roleRouteForRoles(roles: UserRole[]): RoleRouteConfig | undefined {
  return ROLE_ROUTE_PRIORITY.find((config) => roles.includes(config.role))
}

/** isRoleHomePath 判断路径是否为当前四角色之一的规范默认功能页。 */
export function isRoleHomePath(path: string): boolean {
  return ROLE_ROUTE_PRIORITY.some((config) => config.homePath === path)
}

/**
 * loginPathForPath 按路径所属角色区选择登录入口。
 * 平台管理区回平台通道,学校侧三角色回学校登录页;角色区之外的路径(如全站 404)
 * 归属学校登录入口 —— 平台通道是特权入口,不做默认落点。
 */
export function loginPathForPath(pathname: string): string {
  const route = ROLE_ROUTE_PRIORITY.find(
    (config) => pathname === config.pathPrefix || pathname.startsWith(`${config.pathPrefix}/`),
  )
  return route ? route.loginPath : ROLE_ROUTES.schoolAdmin.loginPath
}
