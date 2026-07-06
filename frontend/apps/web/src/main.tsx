// 单入口前端应用：统一承载登录页和四个角色路径。

import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { UserRole } from '@chaimir/api-client'
import type { Account } from '@chaimir/api-client'
import { AuthApp, AuthGate } from './features/auth'
import { ChaimirApp } from './app/ChaimirApp'
import { AUTH_CHANGE_EVENT, getAccessToken, getStoredUser } from './lib/storage'
import { ERROR_MESSAGES } from './copy/errors'
import type { AppDefinition } from './app/types'
import { platformAdminApp } from './features/platform-admin-app'
import { schoolAdminApp } from './features/school-admin-app'
import { studentApp } from './features/student-app'
import { teacherApp } from './features/teacher-app'
import './styles/student-experience.css'
import './styles/teacher-experience.css'
import './styles/school-admin-experience.css'
import './styles/platform-admin-experience.css'
import './styles/page-type-experience.css'

interface RoleEntry {
  path: string
  definition: AppDefinition
}

const roleEntries: RoleEntry[] = [
  { path: '/student', definition: studentApp },
  { path: '/teacher', definition: teacherApp },
  { path: '/school-admin', definition: schoolAdminApp },
  { path: '/platform-admin', definition: platformAdminApp },
]

/**
 * ChaimirWebApp 根据当前路径选择登录页或角色应用，角色内部页面继续使用共享 hash 路由。
 */
function ChaimirWebApp(): React.ReactElement {
  const [pathname, setPathname] = useState(() => window.location.pathname)
  const roleEntry = resolveRoleEntry(pathname)

  useEffect(() => {
    const syncPathname = () => setPathname(window.location.pathname)
    window.addEventListener('popstate', syncPathname)
    window.addEventListener(AUTH_CHANGE_EVENT, syncPathname)
    return () => {
      window.removeEventListener('popstate', syncPathname)
      window.removeEventListener(AUTH_CHANGE_EVENT, syncPathname)
    }
  }, [])

  useEffect(() => {
    if (!roleEntry && getAccessToken()) {
      const targetPath = resolveStoredRolePath(getStoredUser<Account>())
      if (targetPath) {
        window.location.replace(targetPath)
      }
    }
  }, [roleEntry])

  if (!roleEntry) {
    return <AuthApp />
  }

  return (
    <AuthGate>
      <ChaimirApp definition={roleEntry.definition} />
    </AuthGate>
  )
}

/**
 * resolveRoleEntry 把同源路径映射到角色应用定义。
 */
function resolveRoleEntry(pathname: string): RoleEntry | null {
  const normalized = pathname.replace(/\/+$/, '') || '/'
  return roleEntries.find((entry) => normalized === entry.path || normalized.startsWith(`${entry.path}/`)) ?? null
}

/**
 * resolveStoredRolePath 使用后端账号角色缓存恢复登录后的默认路径；无缓存时停留在登录页。
 */
function resolveStoredRolePath(account: Account | null): string | null {
  if (!account) {
    return null
  }
  const roles = account.roles ?? []
  if (roles.includes(UserRole.PLATFORM_ADMIN)) return '/platform-admin/'
  if (roles.includes(UserRole.SCHOOL_ADMIN)) return '/school-admin/'
  if (roles.includes(UserRole.TEACHER)) return '/teacher/'
  if (roles.includes(UserRole.STUDENT)) return '/student/'
  return null
}

const root = document.getElementById('root')

if (!root) {
  document.body.textContent = ERROR_MESSAGES.BOOTSTRAP_CRASH
} else {
  createRoot(root).render(
    <React.StrictMode>
      <ChaimirWebApp />
    </React.StrictMode>,
  )
}
