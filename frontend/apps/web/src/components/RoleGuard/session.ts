// session.ts 定义受保护区域的会话上下文:RoleGuard 校验通过后下发账号信息,
// 壳层(顶栏头像/角标)与页面经 useSession 消费,避免重复请求 /me。

import { createContext, useContext } from 'react'
import type { api } from '../../app/api'

export type SessionData = Awaited<ReturnType<typeof api.identity.getMe>>

export interface SessionValue {
  /** 当前登录账号(经服务端 /me 校验) */
  me: SessionData
  /** 重新拉取会话(如资料更新后) */
  reload: () => void
}

export const SessionContext = createContext<SessionValue | null>(null)

/**
 * useSession 读取受保护区域的会话;只能在 RoleGuard 内层使用。
 */
export function useSession(): SessionValue {
  const value = useContext(SessionContext)
  if (!value) {
    throw new Error('useSession 只能在 RoleGuard 保护的路由内使用')
  }
  return value
}
