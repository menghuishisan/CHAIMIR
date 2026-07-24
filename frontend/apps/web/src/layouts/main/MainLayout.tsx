// MainLayout 提供四类角色共用的日常导航外壳。
import React, { useEffect, useRef, useState } from 'react'
import { Outlet } from 'react-router-dom'
import { breakpoints, PageScaffold, useMediaQuery } from '@chaimir/ui'
import TopNavbar from '../../components/TopNavbar/TopNavbar'
import AppSidebar from '../../components/AppSidebar/AppSidebar'
import { RouteErrorBoundary } from '../../components/RouteErrorBoundary/RouteErrorBoundary'
import styles from './MainLayout.module.css'

// MainLayout 统一管理顶栏、侧栏、移动遮罩和子路由内容区域。
const MainLayout: React.FC = () => {
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false)
  const [isMobileDrawerOpen, setIsMobileDrawerOpen] = useState(false)
  const menuButtonRef = useRef<HTMLButtonElement>(null)
  const sidebarRef = useRef<HTMLElement>(null)
  const isDesktop = useMediaQuery(`(min-width: ${breakpoints.md}px)`)

  // toggleSidebar 切换桌面侧栏的展开状态。
  const toggleSidebar = () => setIsSidebarCollapsed((current) => !current)
  // toggleMobileDrawer 切换窄屏导航抽屉。
  const toggleMobileDrawer = () => setIsMobileDrawerOpen((current) => !current)
  // closeMobileDrawer 在导航或遮罩触发后关闭抽屉。
  const closeMobileDrawer = () => setIsMobileDrawerOpen(false)

  useEffect(() => {
    if (isDesktop) setIsMobileDrawerOpen(false)
  }, [isDesktop])

  useEffect(() => {
    if (!isMobileDrawerOpen) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const focusable = sidebarRef.current?.querySelectorAll<HTMLElement>('a[href], button:not([disabled])') || []
    focusable[0]?.focus()

    const handleKeyDown = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        event.preventDefault()
        setIsMobileDrawerOpen(false)
        menuButtonRef.current?.focus()
        return
      }
      if (event.key !== 'Tab' || focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last?.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first?.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isMobileDrawerOpen])

  return (
    <div className={styles.layoutContainer}>
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <TopNavbar menuButtonRef={menuButtonRef} isMenuOpen={isMobileDrawerOpen} onMenuClick={toggleMobileDrawer} />

      <div className={`${styles.mainWrapper} ${isSidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
        <AppSidebar
          ref={sidebarRef}
          isCollapsed={isSidebarCollapsed}
          isMobileDrawerOpen={isMobileDrawerOpen}
          onCloseMobileDrawer={closeMobileDrawer}
          onToggleCollapse={toggleSidebar}
        />

        {/* 子路由内容区承载四角色的日常功能页面。 */}
        <main id="main-content" className={styles.contentArea} tabIndex={-1}>
          <div className={styles.contentInner}>
            <PageScaffold as="div">
              <RouteErrorBoundary>
                <Outlet />
              </RouteErrorBoundary>
            </PageScaffold>
          </div>
        </main>
      </div>

      {/* 移动端遮罩用于关闭侧栏抽屉并保持焦点路径清晰。 */}
      {isMobileDrawerOpen && (
        <button type="button" className={styles.mobileOverlay} onClick={() => { closeMobileDrawer(); menuButtonRef.current?.focus() }} aria-label="关闭导航菜单" />
      )}
    </div>
  )
}

export default MainLayout
