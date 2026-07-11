// roleRouting 定义角色路径前缀、登录入口和默认功能页的唯一应用契约。

import { UserRole } from '@chaimir/api-client'

export interface RoleRouteConfig {
  role: UserRole
  pathPrefix: string
  homePath: string
  loginPath: string
  loadUnread: boolean
}

export const ROLE_ROUTES = {
  platformAdmin: { role: UserRole.PLATFORM_ADMIN, pathPrefix: '/platform-admin', homePath: '/platform-admin/schools', loginPath: '/auth/platform-login', loadUnread: false },
  schoolAdmin: { role: UserRole.SCHOOL_ADMIN, pathPrefix: '/school-admin', homePath: '/school-admin/users', loginPath: '/auth/login', loadUnread: true },
  teacher: { role: UserRole.TEACHER, pathPrefix: '/teacher', homePath: '/teacher/courses', loginPath: '/auth/login', loadUnread: true },
  student: { role: UserRole.STUDENT, pathPrefix: '/student', homePath: '/student/courses', loginPath: '/auth/login', loadUnread: true },
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
