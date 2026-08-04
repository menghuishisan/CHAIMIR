// studentImmersiveRoutes 学生区沉浸路由的标题与退出目标登记。
//
// 为什么单独一个文件:这四条路径与它们的中文标题都是学生区专有,必须随学生区块下载。
// 放进壳层(layouts/immersive)会被公开深链一起带进入口包 —— 那就等于把「学生端有哪些
// 沉浸工作台」写在未登录访客的首屏代码里(铁律 2)。壳层只留类型与查找,登记表按区分放。
//
// 退出目标显式声明:相对跳转在嵌套路由下会落到角色区之外并被登录守卫拦回登录页(审查 S1),
// 且登记的必须是真实存在的日常页。

import type { ImmersiveRouteConfig } from '../../layouts/immersive/immersiveRoutes'

export const STUDENT_IMMERSIVE_ROUTES: ImmersiveRouteConfig[] = [
  {
    pattern: '/student/experiments/:experimentId/workspace',
    title: '实验工作台',
    exitPath: (params) => `/student/experiments/${params.experimentId}`,
  },
  {
    // 仿真无详情页,退出回到仿真列表
    pattern: '/student/simulations/:packageCode/workspace',
    title: '仿真推演',
    exitPath: () => '/student/simulations',
  },
  {
    pattern: '/student/contests/:contestId/workspace',
    title: '竞赛对抗',
    exitPath: (params) => `/student/contests/${params.contestId}`,
  },
  {
    pattern: '/student/contests/:contestId/replay',
    title: '对局回放',
    exitPath: (params) => `/student/contests/${params.contestId}`,
  },
]
