// AdminSidebar 维护学校管理端侧栏主入口,共享入口统一交给顶栏触发。
import React from 'react'
import { NavLink } from 'react-router-dom'
import {
  Users, Network, LayoutDashboard,
  CheckCircle, Scale, AlertTriangle, Settings2,
  Settings, Shield, FileText, BellRing,
  PanelLeftClose, PanelLeftOpen
} from 'lucide-react'
import styles from './AdminSidebar.module.css'

interface AdminSidebarProps {
  isCollapsed: boolean
  isMobileDrawerOpen: boolean
  onCloseMobileDrawer: () => void
  onToggleCollapse: () => void
}

const AdminSidebar: React.FC<AdminSidebarProps> = ({
  isCollapsed,
  isMobileDrawerOpen,
  onCloseMobileDrawer,
  onToggleCollapse
}) => {

  const menuGroups = [
    {
      title: '用户与组织',
      items: [
        { name: '账号管理', path: '/school-admin/users', icon: Users },
        { name: '组织架构', path: '/school-admin/organization', icon: Network },
      ]
    },
    {
      title: '概览',
      items: [
        { name: '学校看板', path: '/school-admin/dashboard', icon: LayoutDashboard },
      ]
    },
    {
      title: '教务与成绩',
      items: [
        { name: '成绩审核', path: '/school-admin/approvals', icon: CheckCircle },
        { name: '申诉处理', path: '/school-admin/appeals', icon: Scale },
        { name: '学业预警', path: '/school-admin/alerts', icon: AlertTriangle },
        { name: '成绩配置', path: '/school-admin/grade-settings', icon: Settings2 },
      ]
    },
    {
      title: '系统配置',
      items: [
        { name: '租户配置', path: '/school-admin/settings', icon: Settings },
        { name: '认证配置', path: '/school-admin/auth-config', icon: Shield },
        { name: '审计日志', path: '/school-admin/audit', icon: FileText },
        { name: '学校告警', path: '/school-admin/system-alerts', icon: BellRing },
      ]
    }
  ]

  return (
    <aside className={`
      ${styles.sidebar}
      ${isCollapsed ? styles.collapsed : ''}
      ${isMobileDrawerOpen ? styles.mobileOpen : ''}
    `}>
      <div className={styles.scrollArea}>
        {menuGroups.map((group) => (
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
                      `${styles.navItem} ${isActive ? styles.active : ''}`
                    }
                    title={isCollapsed ? item.name : undefined}
                  >
                    <Icon size={20} className={styles.navIcon} />
                    {!isCollapsed && <span className={styles.navText}>{item.name}</span>}
                  </NavLink>
                )
              })}
            </nav>
          </div>
        ))}
      </div>

      <div className={styles.footer}>
        <button className={styles.toggleBtn} onClick={onToggleCollapse}>
          {isCollapsed ? <PanelLeftOpen size={20} /> : <PanelLeftClose size={20} />}
        </button>
      </div>
    </aside>
  )
}

export default AdminSidebar
