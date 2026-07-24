// AppSidebar 提供四类角色共用的日常侧栏导航。
import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { roleNavigationForPath } from '../../app/roleNavigation'
import styles from './AppSidebar.module.css'

interface AppSidebarProps {
  isCollapsed: boolean
  isMobileDrawerOpen: boolean
  onCloseMobileDrawer: () => void
  onToggleCollapse: () => void
}

// AppSidebar 根据当前角色路径切换菜单，并负责桌面折叠与移动抽屉关闭。
const AppSidebar = React.forwardRef<HTMLElement, AppSidebarProps>(({
  isCollapsed,
  isMobileDrawerOpen,
  onCloseMobileDrawer,
  onToggleCollapse
}, ref) => {
  const location = useLocation()
  const navigation = roleNavigationForPath(location.pathname)

  return (
    <aside ref={ref} aria-label="功能导航" className={`
      ${styles.sidebar}
      ${isCollapsed ? styles.collapsed : ''}
      ${isMobileDrawerOpen ? styles.mobileOpen : ''}
    `}>
      <div className={styles.scrollArea}>
        {navigation.groups.map((group) => (
          <div key={group.title} className={styles.menuGroup}>
            {!isCollapsed && (
              <div className={styles.groupTitle}>{group.title}</div>
            )}
            <nav className={styles.nav}>
              {group.items.map((item) => {
                const Icon = item.icon
                return (
                  <NavLink
                    key={item.path}
                    to={item.path}
                    onClick={onCloseMobileDrawer}
                    className={({ isActive }) =>
                      `${styles.navItem} ${isActive || location.pathname.startsWith(item.path + '/') ? styles.active : ''}`
                    }
                    title={isCollapsed ? item.name : undefined}
                    aria-label={isCollapsed ? item.name : undefined}
                  >
                    <Icon size={20} className={styles.navIcon} aria-hidden="true" />
                    {!isCollapsed && <span className={styles.navText}>{item.name}</span>}
                  </NavLink>
                )
              })}
            </nav>
          </div>
        ))}
      </div>

      {/* 桌面端折叠按钮；移动端抽屉不展示该控制。 */}
      <div className={styles.footer}>
        <button type="button" className={styles.toggleBtn} onClick={onToggleCollapse} aria-label={isCollapsed ? '展开侧栏' : '收起侧栏'}>
          {isCollapsed ? <PanelLeftOpen size={20} /> : <PanelLeftClose size={20} />}
        </button>
      </div>
    </aside>
  )
})

AppSidebar.displayName = 'AppSidebar'

export default AppSidebar
