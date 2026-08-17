// RouteErrorBoundary 拦截渲染期异常,避免整站白屏(规范 §6 壳层要求)。
// 两个硬约束:
// 1) 路由变化必须重置错误态 —— 否则一次出错后同壳内所有后续页面都停留在错误屏(S5);
// 2) 只展示后端 trace_id;前端不生成任何编号 —— 前端随机号运维查不到,等于把用户挡在门外。

import React from 'react'
import { TriangleAlert } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { useLocation } from 'react-router'
import { AppStatusScreen } from '../AppStatusScreen'
import { traceIdOf } from '../../utils/userFacingError'

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
 * ErrorBoundaryView 是真正的 class 边界(React 只在 class 组件提供捕获钩子)。
 * routeKey 变化时在 getDerivedStateFromProps 阶段清空错误,保证导航后立即恢复渲染。
 */
class ErrorBoundaryView extends React.Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
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
    console.error('页面渲染失败', error, info.componentStack)
  }

  render(): React.ReactNode {
    if (this.state.failed) {
      return (
        <AppStatusScreen
          icon={TriangleAlert}
          tone="danger"
          title="页面加载出现问题"
          description="这个页面暂时无法显示,可以重新加载再试一次。"
          traceId={this.state.traceId}
          actions={
            <Button variant="primary" onClick={() => window.location.reload()}>
              重新加载
            </Button>
          }
        />
      )
    }
    return this.props.children
  }
}

/**
 * RouteErrorBoundary 把当前路径作为 routeKey 交给 class 边界,实现「导航即恢复」。
 */
export function RouteErrorBoundary({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return <ErrorBoundaryView routeKey={location.pathname}>{children}</ErrorBoundaryView>
}
