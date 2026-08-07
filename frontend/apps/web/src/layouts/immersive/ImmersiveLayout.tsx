// ImmersiveLayout 沉浸态壳层(FE-6 底层曝光系统的第三种状态):
// 独占路由挂载的全屏墨色世界(WorkbenchShell 使用契约),做实验/仿真/答题时光面收拢让位。
// 壳层只提供全屏底层与进出场;顶条、三栏与业务内容由各工作台页用 WorkbenchShell 自行装配,
// 避免壳层与工作台争夺同一块顶条。
//
// 登记表由挂载方传入而不是壳层内置:学生区四条沉浸路径要随学生区块下载,
// 不能因为公开深链需要本壳而被带进入口包(铁律 2)。退出目标由登记显式给出(不用 `..`,S1)。

import { Outlet, useLocation, useNavigate } from 'react-router'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary'
import { immersiveRouteForPath, type ImmersiveRouteConfig } from './immersiveRoutes'
import { ImmersiveContext } from './context'

export interface ImmersiveLayoutProps {
  /** 本次挂载覆盖的沉浸路由登记(标题 + 退出目标) */
  routes: ImmersiveRouteConfig[]
}

/**
 * ImmersiveLayout 渲染全屏底层并向工作台下传标题与退出动作。
 * 进场为展开、退场为收拢(§4.4),动画只作用于 opacity/transform。
 */
export function ImmersiveLayout({ routes }: ImmersiveLayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const route = immersiveRouteForPath(routes, location.pathname)

  return (
    <ImmersiveContext.Provider
      value={{
        title: route.title,
        exit: () => navigate(route.exitPath),
      }}
    >
      <div className="animate-immersive-in fixed inset-0 z-immersive flex flex-col bg-substrate text-on-dark">
        <RouteErrorBoundary>
          <Outlet />
        </RouteErrorBoundary>
      </div>
    </ImmersiveContext.Provider>
  )
}
