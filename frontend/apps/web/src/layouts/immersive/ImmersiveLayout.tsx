// ImmersiveLayout 提供实验、仿真和竞赛工作台的深色全屏外壳。
import React from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { ArrowLeft, Maximize } from 'lucide-react'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary/RouteErrorBoundary'
import styles from './ImmersiveLayout.module.css'

const ImmersiveLayout: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()

  // 根据沉浸式路由语义展示工作台标题，避免各工作台重复声明壳层。
  const getTitle = () => {
    if (location.pathname.includes('/experiments/')) return '实验工作台'
    if (location.pathname.includes('/simulations/')) return '仿真推演'
    if (location.pathname.includes('/contests/')) return '竞赛沙箱'
    return '沉浸式工作台'
  }

  const handleExit = () => {
    // 沉浸模式退出到当前功能的上一层列表或详情页。
    navigate('..')
  }

  // 嵌入式 iframe 抢占焦点时，提供可键盘触达的焦点回收入口。
  const handleFocusEscape = () => {
    window.focus()
  }

  return (
    <div className={styles.immersiveContainer}>
      {/* 深色顶栏保留退出路径和运行状态提示。 */}
      <header className={styles.topbar}>
        <div className={styles.left}>
          <button className={styles.exitBtn} onClick={handleExit} aria-label="退出工作台">
            <ArrowLeft size={18} />
            <span>退出</span>
          </button>
          <div className={styles.divider} />
          <h1 className={styles.title}>{getTitle()}</h1>
        </div>

        <div className={styles.right}>
          <div className={styles.statusPill}>
            <span className={styles.statusDot}></span>
            <span>引擎就绪</span>
          </div>
        </div>
      </header>

      {/* 全局焦点回收按钮服务于 iframe 工具和实验终端。 */}
      <button
        className={styles.focusEscapeBtn}
        onClick={handleFocusEscape}
        title="夺回焦点"
      >
        <Maximize size={16} />
      </button>

      {/* 子路由渲染实验、仿真或竞赛的主体工作区。 */}
      <div className={styles.workspaceArea}>
        <RouteErrorBoundary variant="immersive">
          <Outlet />
        </RouteErrorBoundary>
      </div>
    </div>
  )
}

export default ImmersiveLayout
