// App 是驱动层的组合根(目录设计 §4):只做全局 Provider、渲染错误边界、Suspense 与路由树装配,
// 路由清单与权限装配在 routes/ 层(authRoutes/roleRoutes),本文件不列页面。
// 三种壳层各自独占路由挂载(FE-6 底层曝光系统,同一墨色世界的三种状态):
//   AuthLayout(底层全裸露)/ MainLayout(宣纸光面浮起)/ ImmersiveLayout(全屏沉浸)。
// 平台管理区与平台特权入口仅 SaaS 形态注册 —— 私有化部署下路径不存在,而非前端隐藏。

import { Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { LoaderCircle } from 'lucide-react'
import { Toaster, TooltipProvider } from '@chaimir/ui'
import AuthLayout from '../layouts/auth/AuthLayout'
import { AppStatusScreen } from '../components/AppStatusScreen'
import { RouteErrorBoundary } from '../components/RouteErrorBoundary'
import { NotFoundPage } from '../components/StatusPages'
import { ImmersiveLayout } from '../layouts/immersive/ImmersiveLayout'
import { authRoutes } from '../routes/authRoutes'
import { immersiveRoutes } from '../routes/immersiveRoutes'
import { roleRoutes } from '../routes/roleRoutes'
import { platformLayerEnabled } from './config'

/**
 * App 渲染路由树;Tooltip 与 Toast 宿主全站各挂一次。
 * 根边界覆盖壳层自身(含 AuthLayout 与 MainLayout/ImmersiveLayout 的框架渲染)与懒加载失败;
 * 两个日常壳内另有一层同名边界,作用不同:那一层只换掉内容区,保留导航让用户能走开。
 */
export default function App() {
  return (
    <BrowserRouter>
      <TooltipProvider>
        <RouteErrorBoundary>
          <Suspense fallback={<AppStatusScreen icon={LoaderCircle} spinning title="正在打开" />}>
            <Routes>
              {/* 站点根进登录页;已登录者由登录页按服务端角色直达首个功能页(FE-5) */}
              <Route path="/" element={<Navigate to="/auth/login" replace />} />

              {/* ---------- 认证域:底层全裸露 ---------- */}
              <Route path="/auth" element={<AuthLayout />}>
                {authRoutes(platformLayerEnabled)}
              </Route>

              {/* ---------- 四端角色区:守卫 + 该区懒加载壳 ---------- */}
              {roleRoutes(platformLayerEnabled)}

              {/* ---------- 公开沉浸深链:全屏沉浸,访客无会话不经角色守卫 ---------- */}
              <Route element={<ImmersiveLayout />}>{immersiveRoutes()}</Route>

              <Route path="*" element={<NotFoundPage />} />
            </Routes>
          </Suspense>
        </RouteErrorBoundary>
      </TooltipProvider>
      <Toaster />
    </BrowserRouter>
  )
}
