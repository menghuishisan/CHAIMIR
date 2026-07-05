// 认证上下文：管理用户登录状态、双 Token 轮转和服务端会话退出。

import React, { createContext, useContext, useState, useCallback, useEffect } from 'react'
import type { Account, LoginResponse } from '@chaimir/api-client'
import { clearSession, getAccessToken, getRefreshToken, getStoredUser, saveSession, saveStoredUser } from '@chaimir/shared'

export interface AuthContextValue {
  /** 当前用户 */
  user: Account | null
  /** 是否已登录 */
  isAuthenticated: boolean
  /** 是否加载中 */
  isLoading: boolean
  /** 登录 */
  login: (response: LoginResponse) => void
  /** 登出 */
  logout: () => Promise<void>
  /** 获取 Token */
  getToken: () => string | null
  /** 刷新 Token */
  refreshToken: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export interface AuthProviderProps {
  children: React.ReactNode
  /** 调用后端 /auth/refresh 完成 Refresh Token 轮转。 */
  refreshSession: (refreshToken: string) => Promise<LoginResponse>
  /** 调用后端 /auth/logout 吊销当前服务端会话。 */
  revokeSession?: () => Promise<void>
  /** 认证异常上报入口，应用层可接入用户向提示或监控。 */
  onAuthError?: (error: unknown) => void
}

/**
 * AuthProvider 维护当前浏览器会话中的用户、access token 和 refresh token 状态。
 */
export const AuthProvider: React.FC<AuthProviderProps> = ({
  children,
  refreshSession,
  revokeSession,
  onAuthError,
}) => {
  const [user, setUser] = useState<Account | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    /**
     * initAuth 从本地缓存恢复浏览器会话，并在缓存损坏时清理残留状态。
     */
    const initAuth = () => {
      try {
        const token = getAccessToken()
        const userData = getStoredUser<Account>()

        if (token && userData) {
          setUser(userData)
        }
      } catch (error) {
        clearSession()
        onAuthError?.(error)
      } finally {
        setIsLoading(false)
      }
    }

    initAuth()
  }, [onAuthError])

  /**
   * login 写入后端登录响应中的 token 与账号信息。
   */
  const login = useCallback(
    (response: LoginResponse) => {
      if (!response.access_token || !response.account) {
        clearSession()
        setUser(null)
        onAuthError?.(new Error('登录状态异常，请重新登录'))
        return
      }

      // 存储 Token 和用户信息。
      saveSession(response.access_token, response.refresh_token)
      saveStoredUser(response.account)

      setUser(response.account)
    },
    [onAuthError]
  )

  /**
   * logout 优先吊销后端会话,随后清理本地认证状态。
   */
  const logout = useCallback(async () => {
    try {
      await revokeSession?.()
    } catch (error) {
      onAuthError?.(error)
    } finally {
      clearSession()
      setUser(null)
    }
  }, [onAuthError, revokeSession])

  /**
   * getToken 返回当前 access token,供 API 客户端注入 Authorization 头。
   */
  const getToken = useCallback((): string | null => {
    return getAccessToken()
  }, [])

  /**
   * refreshToken 使用后端 Refresh Token 轮转接口刷新浏览器登录态。
   */
  const refreshToken = useCallback(async () => {
    try {
      const refresh = getRefreshToken()
      if (!refresh) {
        throw new Error('登录状态已失效，请重新登录')
      }
      const response = await refreshSession(refresh)
      if (!response.access_token || !response.account) {
        throw new Error('登录状态已失效，请重新登录')
      }
      saveSession(response.access_token, response.refresh_token)
      saveStoredUser(response.account)
      setUser(response.account)
    } catch (error) {
      onAuthError?.(error)
      await logout()
    }
  }, [logout, onAuthError, refreshSession])

  const value: AuthContextValue = {
    user,
    isAuthenticated: !!user,
    isLoading,
    login,
    logout,
    getToken,
    refreshToken,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

/**
 * useAuth 读取认证上下文,并在缺少 Provider 时显式失败。
 */
export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth 必须在 AuthProvider 内使用')
  }
  return context
}
