// PlatformLayout 提供平台管理员端的顶栏、侧栏和内容区外壳。
import React, { useState } from 'react'
import { Outlet } from 'react-router-dom'
import PlatformSidebar from './PlatformSidebar'
import PlatformNavbar from './PlatformNavbar'
import styles from './PlatformLayout.module.css'

// PlatformLayout 统一管理平台端桌面折叠和移动端抽屉状态。
const PlatformLayout: React.FC = () => {
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false)
  const [isMobileDrawerOpen, setIsMobileDrawerOpen] = useState(false)

  const toggleSidebar = () => {
    setIsSidebarCollapsed(!isSidebarCollapsed)
  }

  const toggleMobileDrawer = () => {
    setIsMobileDrawerOpen(!isMobileDrawerOpen)
  }

  return (
    <div className={styles.layout}>
      <PlatformNavbar onMenuClick={toggleMobileDrawer} />

      <div className={styles.mainContainer}>
        {/* 移动端遮罩用于关闭侧栏抽屉。 */}
        {isMobileDrawerOpen && (
          <div
            className={styles.backdrop}
            onClick={() => setIsMobileDrawerOpen(false)}
          />
        )}

        <PlatformSidebar
          isCollapsed={isSidebarCollapsed}
          isMobileDrawerOpen={isMobileDrawerOpen}
          onCloseMobileDrawer={() => setIsMobileDrawerOpen(false)}
          onToggleCollapse={toggleSidebar}
        />

        <main className={styles.content}>
          <div className={styles.pageWrapper}>
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}

export default PlatformLayout
