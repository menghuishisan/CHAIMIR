// immersiveRoutes 公开沉浸态路由清单(免认证深链,全屏沉浸):
// 目录设计 §4 铁律 2 —— 路由层只维护清单与懒加载入口,不写业务逻辑。
// 本清单只收录不在角色区内的公开沉浸路由,页面与后端接口一一对应:
//   /sim/shared/:shareCode 仿真分享回放(GET sim/shared/:code,全平台唯一匿名业务接口)
// 学生区四条沉浸路由(实验工作台 / 仿真推演 / 竞赛对抗 / 对局回放)连同它们的标题与退出目标
// 登记在 routes/sections/studentImmersiveRoutes.ts,随学生区块下载,不进入口包。

import { lazy } from 'react'
import { Route } from 'react-router-dom'
import type { ImmersiveRouteConfig } from '../layouts/immersive/immersiveRoutes'

/* 懒加载:访客只下载公开回放代码,不触碰任何角色端代码碎片 */
const PublicReplayPage = lazy(() => import('../features/sim/pages/public/shared-replay'))

/**
 * PUBLIC_IMMERSIVE_ROUTES 是公开沉浸路由的标题与退出目标登记。
 * 访客没有可退回的日常页,退出即回到登录。
 */
export const PUBLIC_IMMERSIVE_ROUTES: ImmersiveRouteConfig[] = [
  {
    pattern: '/sim/shared/:shareCode',
    title: '公开回放',
    exitPath: () => '/auth/login',
  },
]

/**
 * immersiveRoutes 返回公开沉浸态路由。
 * 与 /auth、角色区并列挂载:访客无会话,不经任何角色守卫。
 */
export function immersiveRoutes() {
  return (
    <>
      <Route path="/sim/shared/:shareCode" element={<PublicReplayPage />} />
    </>
  )
}
