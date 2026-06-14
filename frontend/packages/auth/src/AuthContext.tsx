// 认证上下文：管理用户登录状态、Token

import React, { createContext, useContext, useState, useCallback, useEffect } from 'react'
import type { Account, LoginResponse } from '@chaimir/api-client'
import { StorageKeys } from '@chaimir/shared'

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
  logout: () => void
  /** 获取 Token */
  getToken: () => string | null
  /** 刷新 Token */
  refreshToken: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export interface AuthProviderProps {
  children: React.ReactNode
  onTokenExpired?: () => void
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children, onTokenExpired }) => {
  const [user, setUser] = useState<Account | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // 初始化：从 localStorage 恢复用户信息
  useEffect(() => {
    const initAuth = () => {
      try {
        const token = localStorage.getItem(StorageKeys.ACCESS_TOKEN)
        const userStr = localStorage.getItem(StorageKeys.USER_INFO)

        if (token && userStr) {
          const userData = JSON.parse(userStr)
          setUser(userData)
        }
      } catch (error) {
        console.error('初始化认证失败:', error)
      } finally {
        setIsLoading(false)
      }
    }

    initAuth()
  }, [])

  const login = useCallback((response: LoginResponse) => {
    if (response.access_token && response.account) {
      // 存储 Token 和用户信息
      localStorage.setItem(StorageKeys.ACCESS_TOKEN, response.access_token)
      if (response.refresh_token) {
        localStorage.setItem(StorageKeys.REFRESH_TOKEN, response.refresh_token)
      }
      localStorage.setItem(StorageKeys.USER_INFO, JSON.stringify(response.account))

      setUser(response.account)
    }
  }, [])

  const logout = useCallback(() => {
    // 清除所有认证信息
    localStorage.removeItem(StorageKeys.ACCESS_TOKEN)
    localStorage.removeItem(StorageKeys.REFRESH_TOKEN)
    localStorage.removeItem(StorageKeys.USER_INFO)

    setUser(null)
  }, [])

  const getToken = useCallback((): string | null => {
    return localStorage.getItem(StorageKeys.ACCESS_TOKEN)
  }, [])

  const refreshToken = useCallback(async () => {
    try {
      const refresh = localStorage.getItem(StorageKeys.REFRESH_TOKEN)
      if (!refresh) {
        throw new Error('无刷新令牌')
      }

      // 这里需要调用 API 刷新 Token
      // 由于 AuthProvider 不直接依赖 API 实例，由外部处理
      // 这里只是占位，实际实现在应用层
      onTokenExpired?.()
    } catch (error) {
      console.error('刷新 Token 失败:', error)
      logout()
    }
  }, [onTokenExpired, logout])

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

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth 必须在 AuthProvider 内使用')
  }
  return context
}
