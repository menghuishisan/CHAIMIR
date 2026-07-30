// immersiveRoutes 声明沉浸态路由的标题与退出目标。
// 退出目标必须显式声明:相对跳转(`..`)在嵌套路由下会落到角色区之外并被登录守卫拦回登录页,
// 因此每条沉浸路由都在此登记它退回的日常页,且登记的必须是真实存在的路由。

import { matchPath } from 'react-router-dom'

interface ImmersiveRouteConfig {
  /** 完整路径模式(含角色前缀) */
  pattern: string
  /** 工作台标题:壳层统一提供,各工作台不重复声明 */
  title: string
  /** 退出后回到的日常页 */
  exitPath: (params: Record<string, string | undefined>) => string
}

const IMMERSIVE_ROUTES: ImmersiveRouteConfig[] = [
  {
    pattern: '/student/experiments/:id/workspace',
    title: '实验工作台',
    exitPath: (params) => `/student/experiments/${params.id}`,
  },
  {
    // 仿真无详情页,退出回到仿真列表
    pattern: '/student/simulations/:id/workspace',
    title: '仿真推演',
    exitPath: () => '/student/simulations',
  },
  {
    pattern: '/student/contests/:id/workspace',
    title: '竞赛对抗',
    exitPath: (params) => `/student/contests/${params.id}`,
  },
  {
    pattern: '/student/contests/:id/replay',
    title: '对局回放',
    exitPath: (params) => `/student/contests/${params.id}`,
  },
  {
    // 公开分享回放(唯一不在角色区内的沉浸路由):后端 `GET /api/v1/sim/shared/:code`
    // 是全平台唯一的匿名业务入口,访客没有可退回的日常页,退出即回到登录。
    pattern: '/sim/shared/:shareCode',
    title: '公开回放',
    exitPath: () => '/auth/login',
  },
]

export interface ImmersiveRouteInfo {
  title: string
  exitPath: string
}

/** immersiveRouteForPath 返回当前沉浸路由的标题与退出目标;未登记的路径视为壳层装配错误。 */
export function immersiveRouteForPath(pathname: string): ImmersiveRouteInfo {
  for (const route of IMMERSIVE_ROUTES) {
    const match = matchPath(route.pattern, pathname)
    if (match) {
      return { title: route.title, exitPath: route.exitPath(match.params) }
    }
  }
  throw new Error(`未找到沉浸态路由配置: ${pathname}`)
}
