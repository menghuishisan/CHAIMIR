// App 是驱动层的组合根(目录设计 §4):只做全局 Provider、渲染错误边界、Suspense 与路由树装配,
// 路由清单与权限装配在 routes/ 层(authRoutes/roleRoutes),本文件不列页面。
// 三种壳层各自独占路由挂载(FE-6 底层曝光系统,同一墨色世界的三种状态):
//   AuthLayout(底层全裸露)/ MainLayout(宣纸光面浮起)/ ImmersiveLayout(全屏沉浸)。
// 平台管理区与平台特权入口仅 SaaS 形态注册 —— 私有化部署下路径不存在,而非前端隐藏。
//
// 用数据路由(createBrowserRouter)而不是 <BrowserRouter>:作答、编排等页面要在有未保存内容时
// 拦下站内跳转(useBlocker 只在数据路由里可用,规范 §6.7)。路由树本身不变,不使用加载器。

import { Suspense } from 'react'
import {
  createBrowserRouter,
  createRoutesFromElements,
  Navigate,
  Outlet,
  Route,
} from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { LoaderCircle } from 'lucide-react'
import { Toaster, TooltipProvider } from '@chaimir/ui'
import AuthLayout from '../layouts/auth/AuthLayout'
import { AppStatusScreen } from '../components/AppStatusScreen'
import { RootRouteError, RouteErrorBoundary } from '../components/RouteErrorBoundary'
import { NotFoundPage } from '../components/StatusPages'
import { ImmersiveLayout } from '../layouts/immersive/ImmersiveLayout'
import { authRoutes } from '../routes/authRoutes'
import { PUBLIC_IMMERSIVE_ROUTES, immersiveRoutes } from '../routes/immersiveRoutes'
import { roleRoutes } from '../routes/roleRoutes'
import { platformLayerEnabled } from './config'

/**
 * RootFrame 是全部路由的根:Tooltip 与 Toast 宿主全站各挂一次。
 * 根边界覆盖壳层自身(含 AuthLayout 与 MainLayout/ImmersiveLayout 的框架渲染)与懒加载失败;
 * 两个日常壳内另有一层同名边界,作用不同:那一层只换掉内容区,保留导航让用户能走开。
 */
function RootFrame() {
  return (
    <>
      <TooltipProvider>
        <RouteErrorBoundary>
          <Suspense fallback={<AppStatusScreen icon={LoaderCircle} spinning title="正在打开" />}>
            <Outlet />
          </Suspense>
        </RouteErrorBoundary>
      </TooltipProvider>
      <Toaster />
    </>
  )
}

/**
 * router 在模块加载时创建一次:App 本身由 main.tsx 在运行时配置就绪后懒加载,
 * 此时 platformLayerEnabled 已是最终值。errorElement 接住根边界之外的渲染错误,
 * 不让路由库的默认错误页(会显示错误原文)出现在用户面前。
 */
const router = createBrowserRouter(
  createRoutesFromElements(
    <Route element={<RootFrame />} errorElement={<RootRouteError />}>
      {/* 站点根进登录页;已登录者由登录页按服务端角色直达首个功能页(FE-5) */}
      <Route path="/" element={<Navigate to="/auth/login" replace />} />

      {/* ---------- 认证域:底层全裸露 ---------- */}
      <Route path="/auth" element={<AuthLayout />}>
        {authRoutes(platformLayerEnabled)}
      </Route>

      {/* ---------- 四端角色区:守卫 + 该区懒加载壳 ---------- */}
      {roleRoutes(platformLayerEnabled)}

      {/* ---------- 公开沉浸深链:全屏沉浸,访客无会话不经角色守卫 ---------- */}
      <Route element={<ImmersiveLayout routes={PUBLIC_IMMERSIVE_ROUTES} />}>
        {immersiveRoutes()}
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Route>
  )
)

/** App 挂载路由 */
export default function App() {
  return <RouterProvider router={router} />
}
