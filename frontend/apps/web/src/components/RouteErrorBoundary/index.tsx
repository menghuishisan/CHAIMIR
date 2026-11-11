// RouteErrorBoundary 拦截渲染期异常,避免整站白屏(规范 §6 壳层要求)。
// 两个硬约束:
// 1) 路由变化必须重置错误态 —— 否则一次出错后同壳内所有后续页面都停留在错误屏(S5);
// 2) 只展示后端 trace_id;前端不生成任何编号 —— 前端随机号运维查不到,等于把用户挡在门外。

import React, { useEffect } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { useLocation, useRouteError } from 'react-router'
import { AppStatusScreen } from '../AppStatusScreen'
import { errorDiagnostics, traceIdOf } from '../../utils/userFacingError'

interface RouteErrorBoundaryProps {
  /** 路由标识:变化即重置错误态 */
  routeKey: string
  children: React.ReactNode
}

interface RouteErrorBoundaryState {
  failed: boolean
  traceId?: string
  /** 记录错误发生时的路由:与当前 routeKey 不同即说明已导航,错误随之作废 */
  failedKey?: string
}

/**
 * RenderErrorBoundary 供启动层与路由层复用渲染错误边界。
 * routeKey 变化时在 getDerivedStateFromProps 阶段清空错误,保证导航后立即恢复渲染。
 */
export class RenderErrorBoundary extends React.Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { failed: false }

  static getDerivedStateFromError(error: unknown): Partial<RouteErrorBoundaryState> {
    return { failed: true, traceId: traceIdOf(error) }
  }

  static getDerivedStateFromProps(
    props: RouteErrorBoundaryProps,
    state: RouteErrorBoundaryState,
  ): Partial<RouteErrorBoundaryState> | null {
    // 错误屏渲染后 failedKey 才落定;之后路径一变即重置,回到正常渲染
    if (state.failed && state.failedKey !== undefined && state.failedKey !== props.routeKey) {
      return { failed: false, traceId: undefined, failedKey: undefined }
    }
    if (state.failed && state.failedKey === undefined) {
      return { failedKey: props.routeKey }
    }
    return null
  }

  componentDidCatch(error: unknown, info: React.ErrorInfo): void {
    // 渲染异常必须留下可定位记录:不吞错(§8),同时不把技术细节展示给用户
    console.error('页面渲染失败', { error: errorDiagnostics(error), component_stack: info.componentStack })
  }

  render(): React.ReactNode {
    if (this.state.failed) {
      return <RenderFailureScreen traceId={this.state.traceId} />
    }
    return this.props.children
  }
}

/** RenderFailureScreen 是渲染失败时的整屏提示:不展示错误原文,只给后端签发的报障编号 */
function RenderFailureScreen({ traceId }: { traceId?: string }) {
  return (
    <AppStatusScreen
      icon={TriangleAlert}
      tone="danger"
      title="页面加载出现问题"
      description="这个页面暂时无法显示,可以重新加载再试一次。"
      traceId={traceId}
      actions={
        <Button variant="primary" onClick={() => window.location.reload()}>
          重新加载
        </Button>
      }
    />
  )
}

/**
 * RootRouteError 是根路由的 errorElement:接住根边界之外(如 Tooltip 宿主自身)的渲染错误,
 * 与类边界同一张提示屏并同样留下结构化记录,代替路由库会显示错误原文的默认页。
 */
export function RootRouteError() {
  const error = useRouteError()
  useEffect(() => {
    console.error('页面渲染失败', { error: errorDiagnostics(error), boundary: 'root-route' })
  }, [error])
  return <RenderFailureScreen traceId={traceIdOf(error)} />
}

/**
 * RouteErrorBoundary 把当前路径作为 routeKey 交给 class 边界,实现「导航即恢复」。
 */
export function RouteErrorBoundary({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return <RenderErrorBoundary routeKey={location.pathname}>{children}</RenderErrorBoundary>
}
