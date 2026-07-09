// PlatformSidebar 维护平台管理端侧栏主入口,深层配置入口不在侧栏平铺。
import React from 'react'
import { NavLink } from 'react-router-dom'
import {
  Building, Inbox, LayoutDashboard, Server, Package,
  Cpu, Shield, Bug, BellRing, Settings, Monitor, Save, FileText,
  PanelLeftClose, PanelLeftOpen
} from 'lucide-react'
import styles from './PlatformSidebar.module.css'

interface PlatformSidebarProps {
  isCollapsed: boolean
  isMobileDrawerOpen: boolean
  onCloseMobileDrawer: () => void
  onToggleCollapse: () => void
}

const PlatformSidebar: React.FC<PlatformSidebarProps> = ({
  isCollapsed,
  isMobileDrawerOpen,
  onCloseMobileDrawer,
  onToggleCollapse
}) => {

  const menuGroups = [
    {
      title: '租户',
      items: [
        { name: '学校管理', path: '/platform-admin/schools', icon: Building },
        { name: '入驻申请', path: '/platform-admin/applications', icon: Inbox },
      ]
    },
    {
      title: '运营',
      items: [
        { name: '平台看板', path: '/platform-admin/dashboard', icon: LayoutDashboard },
      ]
    },
    {
      title: '底层资源',
      items: [
        { name: '链运行时', path: '/platform-admin/runtimes', icon: Server },
        { name: '沙箱工具', path: '/platform-admin/sandbox-tools', icon: Package },
        { name: '判题器', path: '/platform-admin/judges', icon: Cpu },
        { name: '仿真治理', path: '/platform-admin/simulations', icon: Shield },
        { name: '漏洞题源', path: '/platform-admin/vulnerabilities', icon: Bug },
        { name: '告警中心', path: '/platform-admin/alerts', icon: BellRing },
        { name: '系统配置', path: '/platform-admin/settings', icon: Settings },
        { name: '监控面板', path: '/platform-admin/monitoring', icon: Monitor },
        { name: '备份记录', path: '/platform-admin/backups', icon: Save },
        { name: '平台审计', path: '/platform-admin/audit', icon: FileText },
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

export default PlatformSidebar
