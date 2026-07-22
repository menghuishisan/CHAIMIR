// RouteErrorBoundary 将页面渲染异常限制在当前内容区，并提供明确的恢复动作。

import React from 'react'
import { AlertTriangle, ArrowLeft, RefreshCw } from 'lucide-react'
import { Button } from '@chaimir/ui'
import styles from './RouteErrorBoundary.module.css'

interface RouteErrorBoundaryProps {
  children: React.ReactNode
  variant?: 'page' | 'immersive'
}

interface RouteErrorBoundaryState {
  error: Error | null
  incidentId: string
}

/** RouteErrorBoundary 捕获子树渲染异常，避免导航壳和退出路径一并消失。 */
export class RouteErrorBoundary extends React.Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { error: null, incidentId: '' }

  /** getDerivedStateFromError 切换到可恢复错误状态并生成浏览器诊断编号。 */
  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { error, incidentId: createIncidentId() }
  }

  /** componentDidCatch 将完整技术错误留在开发控制台，不向终端用户暴露堆栈。 */
  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    console.error('页面渲染失败', { error, componentStack: info.componentStack, incidentId: this.state.incidentId })
  }

  /** handleRetry 清除边界状态并重新挂载当前页面。 */
  private handleRetry = (): void => {
    this.setState({ error: null, incidentId: '' })
  }

  /** handleBack 返回上一条浏览历史，保留用户可控的退出路径。 */
  private handleBack = (): void => {
    window.history.back()
  }

  render(): React.ReactNode {
    if (!this.state.error) return this.props.children

    return (
      <section className={this.props.variant === 'immersive' ? styles.immersive : styles.page} role="alert">
        <AlertTriangle size={28} aria-hidden="true" />
        <h1>当前页面暂时无法显示</h1>
        <p>请重新加载当前页面；如问题持续出现，请向管理员提供编号 {this.state.incidentId}。</p>
        <div className={styles.actions}>
          <Button icon={<RefreshCw size={16} />} onClick={this.handleRetry}>重新加载</Button>
          <Button variant="outline" icon={<ArrowLeft size={16} />} onClick={this.handleBack}>返回上一页</Button>
        </div>
      </section>
    )
  }
}

/** createIncidentId 生成只用于本次浏览器错误定位的短编号。 */
function createIncidentId(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID().slice(0, 12)
    : `${Date.now()}`
}
