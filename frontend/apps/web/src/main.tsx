// 单入口前端应用：统一承载登录页和四个角色路径。

import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import type { Account } from '@chaimir/api-client'
import { AuthApp, AuthGate } from '@chaimir/auth'
import { ChaimirApp, getAccessToken, getStoredUser } from '@chaimir/shared'
import type { AppDefinition } from '@chaimir/shared'
import { platformAdminApp } from './features/platform-admin-app'
import { schoolAdminApp } from './features/school-admin-app'
import { studentApp } from './features/student-app'
import { teacherApp } from './features/teacher-app'

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
    window.addEventListener('chaimir-auth-change', syncPathname)
    return () => {
      window.removeEventListener('popstate', syncPathname)
      window.removeEventListener('chaimir-auth-change', syncPathname)
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
  if (roles.includes(1) || account.base_identity === 1) return '/platform-admin/'
  if (roles.includes(2) || account.base_identity === 2) return '/school-admin/'
  if (roles.includes(3) || account.base_identity === 3) return '/teacher/'
  return '/student/'
}

const root = document.getElementById('root')

if (!root) {
  document.body.textContent = '页面加载失败，请刷新后重试'
} else {
  createRoot(root).render(
    <React.StrictMode>
      <ChaimirWebApp />
    </React.StrictMode>,
  )
}
