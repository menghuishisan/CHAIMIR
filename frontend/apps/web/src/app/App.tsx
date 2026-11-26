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

// 根边界处理壳层失败，日常壳自己的边界保留导航。
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

// 配置在入口处加载完毕。数据路由支持用 useBlocker 拦截离开未保存页面。
const router = createBrowserRouter(
  createRoutesFromElements(
    <Route element={<RootFrame />} errorElement={<RootRouteError />}>
      {/* 站点根进登录页;已登录者由登录页按服务端角色直达首个功能页(FE-5) */}
      <Route path="/" element={<Navigate to="/auth/login" replace />} />

      <Route path="/auth" element={<AuthLayout />}>
        {authRoutes(platformLayerEnabled)}
      </Route>

      {roleRoutes(platformLayerEnabled)}

      <Route element={<ImmersiveLayout routes={PUBLIC_IMMERSIVE_ROUTES} />}>
        {immersiveRoutes()}
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Route>
  )
)

export default function App() {
  return <RouterProvider router={router} />
}
