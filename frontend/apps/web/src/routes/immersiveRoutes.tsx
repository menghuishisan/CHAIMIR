// immersiveRoutes 公开沉浸态路由清单(免认证深链,全屏沉浸):
// 目录设计 §4 铁律 2 —— 路由层只维护清单与懒加载入口,不写业务逻辑。
// 本清单只收录不在角色区内的沉浸路由,页面与后端接口一一对应:
//   /sim/shared/:shareCode 仿真分享回放(GET sim/shared/:code,全平台唯一匿名业务接口)
// 学生区四条沉浸路由(实验工作台 / 仿真推演 / 竞赛对抗 / 对局回放)挂在角色区守卫之内,
// 属阶段 5 范围,本清单不注册;它们的标题与退出目标已在 layouts/immersive/immersiveRoutes.ts 登记。

import { lazy } from 'react'
import { Route } from 'react-router-dom'

/* 懒加载:访客只下载公开回放代码,不触碰任何角色端代码碎片 */
const PublicReplayPage = lazy(() => import('../features/sim/pages/public/shared-replay'))

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
