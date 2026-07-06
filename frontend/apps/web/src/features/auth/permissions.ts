// 权限检查：基于角色的权限控制

import type { Account } from '@chaimir/api-client'
import { UserRole } from '@chaimir/api-client'

/**
 * 检查用户是否有指定角色
 */
export function hasRole(user: Account | null, role: UserRole): boolean {
  if (!user) return false
  return user.roles.includes(role)
}

/**
 * 检查用户是否有任一指定角色
 */
export function hasAnyRole(user: Account | null, roles: UserRole[]): boolean {
  if (!user) return false
  return roles.some((role) => user.roles.includes(role))
}

/**
 * 检查用户是否有所有指定角色
 */
export function hasAllRoles(user: Account | null, roles: UserRole[]): boolean {
  if (!user) return false
  return roles.every((role) => user.roles.includes(role))
}

/**
 * 检查用户是否为平台管理员
 */
export function isPlatformAdmin(user: Account | null): boolean {
  return hasRole(user, UserRole.PLATFORM_ADMIN)
}

/**
 * 检查用户是否为学校管理员
 */
export function isSchoolAdmin(user: Account | null): boolean {
  return hasRole(user, UserRole.SCHOOL_ADMIN)
}

/**
 * 检查用户是否为教师
 */
export function isTeacher(user: Account | null): boolean {
  return hasRole(user, UserRole.TEACHER)
}

/**
 * 检查用户是否为学生
 */
export function isStudent(user: Account | null): boolean {
  return hasRole(user, UserRole.STUDENT)
}

/**
 * 检查用户是否有管理权限（平台或学校管理员）
 */
export function isAdmin(user: Account | null): boolean {
  return hasAnyRole(user, [UserRole.PLATFORM_ADMIN, UserRole.SCHOOL_ADMIN])
}

/**
 * 检查用户是否有教学权限（教师或管理员）
 */
export function canTeach(user: Account | null): boolean {
  return hasAnyRole(user, [UserRole.PLATFORM_ADMIN, UserRole.SCHOOL_ADMIN, UserRole.TEACHER])
}
