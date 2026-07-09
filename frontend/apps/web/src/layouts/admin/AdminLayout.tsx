// AdminLayout 提供学校管理员端的顶栏、侧栏和内容区外壳。
import React, { useState } from 'react'
import { Outlet } from 'react-router-dom'
import AdminSidebar from './AdminSidebar'
import AdminNavbar from './AdminNavbar'
import styles from './AdminLayout.module.css'

// AdminLayout 统一管理管理员端桌面折叠和移动端抽屉状态。
const AdminLayout: React.FC = () => {
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
      <AdminNavbar onMenuClick={toggleMobileDrawer} />

      <div className={styles.mainContainer}>
        {isMobileDrawerOpen && (
          <div
            className={styles.backdrop}
            onClick={() => setIsMobileDrawerOpen(false)}
          />
        )}

        <AdminSidebar
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

export default AdminLayout
