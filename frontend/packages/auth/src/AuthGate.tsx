// AuthGate：四端入口统一认证门禁，未登录时加载 packages/auth 内的公共认证页面。

import React, { useEffect, useState } from 'react'
import { getAccessToken } from '@chaimir/shared'
import { AuthApp } from './AuthApp'

export interface AuthGateProps {
  /** 已登录后渲染的四端应用内容。 */
  children: React.ReactNode
}

/**
 * AuthGate 在四端入口复用同一套登录前页面，避免新增第五个 apps 入口或复制认证页面。
 */
export function AuthGate({ children }: AuthGateProps): React.ReactElement {
  const [authenticated, setAuthenticated] = useState(() => Boolean(getAccessToken()))

  useEffect(() => {
    const onStorage = () => setAuthenticated(Boolean(getAccessToken()))
    window.addEventListener('storage', onStorage)
    window.addEventListener('chaimir-auth-change', onStorage)
    return () => {
      window.removeEventListener('storage', onStorage)
      window.removeEventListener('chaimir-auth-change', onStorage)
    }
  }, [])

  return authenticated ? <>{children}</> : <AuthApp />
}
