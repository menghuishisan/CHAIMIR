// AuthGate：角色路径统一认证门禁，未登录时加载单入口内的公共认证页面。

import React, { useEffect, useState } from 'react'
import { AUTH_CHANGE_EVENT, getAccessToken } from '../../lib/storage'
import { AuthApp } from './AuthApp'

export interface AuthGateProps {
  /** 已登录后渲染的角色应用内容。 */
  children: React.ReactNode
}

/**
 * AuthGate 在角色路径复用同一套登录前页面，避免复制认证页面或自建登录态。
 */
export function AuthGate({ children }: AuthGateProps): React.ReactElement {
  const [authenticated, setAuthenticated] = useState(() => Boolean(getAccessToken()))

  useEffect(() => {
    const onStorage = () => setAuthenticated(Boolean(getAccessToken()))
    window.addEventListener('storage', onStorage)
    window.addEventListener(AUTH_CHANGE_EVENT, onStorage)
    return () => {
      window.removeEventListener('storage', onStorage)
      window.removeEventListener(AUTH_CHANGE_EVENT, onStorage)
    }
  }, [])

  return authenticated ? <>{children}</> : <AuthApp />
}
