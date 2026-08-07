// immersiveRoutes 定义沉浸态路由登记的形状与查找。
//
// 登记表本身不放在这里:各区的沉浸路径要随该区的懒加载壳一起加载(铁律 2)。
// 公开深链的登记在 routes/immersiveRoutes.tsx(入口包 —— 访客必须能进),
// 学生区四条的登记在 routes/sections/studentImmersiveRoutes.ts(随学生区块下载)。
// 本文件只提供类型与查找:类型在编译期擦除,查找函数不含任何路径字面量。
//
// 退出目标必须显式声明:相对跳转(`..`)在嵌套路由下会落到角色区之外并被登录守卫拦回登录页,
// 因此每条沉浸路由都要登记它退回的日常页,且登记的必须是真实存在的路由。

import { matchPath } from 'react-router'

export interface ImmersiveRouteConfig {
  /** 完整路径模式(含角色前缀) */
  pattern: string
  /** 工作台标题:壳层统一提供,各工作台不重复声明 */
  title: string
  /** 退出后回到的日常页 */
  exitPath: (params: Record<string, string | undefined>) => string
}

export interface ImmersiveRouteInfo {
  title: string
  exitPath: string
}

/**
 * immersiveRouteForPath 在给定登记表里查当前沉浸路由的标题与退出目标。
 * 未登记的路径视为壳层装配错误:直接暴露而不是给一个猜出来的退出目标 ——
 * 猜错会把人甩到登录页,比报错更难排查。
 */
export function immersiveRouteForPath(
  routes: ImmersiveRouteConfig[],
  pathname: string,
): ImmersiveRouteInfo {
  for (const route of routes) {
    const match = matchPath(route.pattern, pathname)
    if (match) {
      return { title: route.title, exitPath: route.exitPath(match.params) }
    }
  }
  throw new Error(`未找到沉浸态路由配置: ${pathname}`)
}
