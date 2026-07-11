// MainLayout 提供四类角色共用的日常导航外壳。
import React, { useState } from 'react'
import { Outlet } from 'react-router-dom'
import TopNavbar from '../../components/TopNavbar/TopNavbar'
import AppSidebar from '../../components/AppSidebar/AppSidebar'
import styles from './MainLayout.module.css'

// MainLayout 统一管理顶栏、侧栏、移动遮罩和子路由内容区域。
const MainLayout: React.FC = () => {
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false)
  const [isMobileDrawerOpen, setIsMobileDrawerOpen] = useState(false)

  // toggleSidebar 切换桌面侧栏的展开状态。
  const toggleSidebar = () => setIsSidebarCollapsed((current) => !current)
  // toggleMobileDrawer 切换窄屏导航抽屉。
  const toggleMobileDrawer = () => setIsMobileDrawerOpen((current) => !current)
  // closeMobileDrawer 在导航或遮罩触发后关闭抽屉。
  const closeMobileDrawer = () => setIsMobileDrawerOpen(false)

  return (
    <div className={styles.layoutContainer}>
      <TopNavbar onMenuClick={toggleMobileDrawer} />

      <div className={styles.mainWrapper}>
        <AppSidebar
          isCollapsed={isSidebarCollapsed}
          isMobileDrawerOpen={isMobileDrawerOpen}
          onCloseMobileDrawer={closeMobileDrawer}
          onToggleCollapse={toggleSidebar}
        />

        {/* 子路由内容区承载四角色的日常功能页面。 */}
        <main className={styles.contentArea}>
          <div className={styles.contentInner}>
            <Outlet />
          </div>
        </main>
      </div>

      {/* 移动端遮罩用于关闭侧栏抽屉并保持焦点路径清晰。 */}
      {isMobileDrawerOpen && (
        <button type="button" className={styles.mobileOverlay} onClick={closeMobileDrawer} aria-label="关闭导航菜单" />
      )}
    </div>
  )
}

export default MainLayout
