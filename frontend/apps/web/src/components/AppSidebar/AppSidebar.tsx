// AppSidebar 提供学生端和教师端的日常侧栏导航。
import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import {
  BookOpen, FlaskConical, Network, Swords, Trophy, GraduationCap, AlertTriangle,
  PanelLeftClose, PanelLeftOpen,
  Book, CheckSquare, LayoutTemplate, Activity, Database, FileText, Share2, Send, Users
} from 'lucide-react'
import styles from './AppSidebar.module.css'

interface AppSidebarProps {
  isCollapsed: boolean
  isMobileDrawerOpen: boolean
  onCloseMobileDrawer: () => void
  onToggleCollapse: () => void
}

// AppSidebar 根据当前角色路径切换菜单，并负责桌面折叠与移动抽屉关闭。
const AppSidebar: React.FC<AppSidebarProps> = ({
  isCollapsed,
  isMobileDrawerOpen,
  onCloseMobileDrawer,
  onToggleCollapse
}) => {
  const location = useLocation()
  const isTeacher = location.pathname.startsWith('/teacher')

  const studentMenuGroups = [
    {
      title: '学习区 LEARNING',
      items: [
        { name: '课程', path: '/student/courses', icon: BookOpen },
        { name: '实验', path: '/student/experiments', icon: FlaskConical },
        { name: '仿真', path: '/student/simulations', icon: Network },
        { name: '参赛', path: '/student/contests', icon: Swords },
        { name: '战绩', path: '/student/records', icon: Trophy },
      ]
    },
    {
      title: '学业区 PERFORMANCE',
      items: [
        { name: '成绩', path: '/student/grades', icon: GraduationCap },
        { name: '预警', path: '/student/alerts', icon: AlertTriangle },
      ]
    }
  ]

  const teacherMenuGroups = [
    {
      title: '教学 TEACHING',
      items: [
        { name: '课程管理', path: '/teacher/courses', icon: Book },
        { name: '批改中心', path: '/teacher/grading', icon: CheckSquare },
      ]
    },
    {
      title: '实践 PRACTICE',
      items: [
        { name: '实验编排', path: '/teacher/experiments', icon: LayoutTemplate },
        { name: '赛事组织', path: '/teacher/contests', icon: Trophy },
        { name: '实时监控', path: '/teacher/monitoring', icon: Activity },
      ]
    },
    {
      title: '资源 RESOURCES',
      items: [
        { name: '题库内容', path: '/teacher/questions', icon: Database },
        { name: '试卷组卷', path: '/teacher/exams', icon: FileText },
        { name: '漏洞题源转化', path: '/teacher/vulnerabilities', icon: AlertTriangle },
        { name: '仿真场景', path: '/teacher/simulations', icon: Network },
        { name: '共享资源库', path: '/teacher/shared', icon: Share2 },
      ]
    },
    {
      title: '组织与成绩 GRADES',
      items: [
        { name: '成绩报送', path: '/teacher/grades', icon: Send },
        { name: '组织查看', path: '/teacher/organization', icon: Users },
      ]
    }
  ]

  const menuGroups = isTeacher ? teacherMenuGroups : studentMenuGroups

  return (
    <aside className={`
      ${styles.sidebar}
      ${isCollapsed ? styles.collapsed : ''}
      ${isMobileDrawerOpen ? styles.mobileOpen : ''}
    `}>
      <div className={styles.scrollArea}>
        {menuGroups.map((group, index) => (
          <div key={index} className={styles.menuGroup}>
            {!isCollapsed && (
              <div className={styles.groupTitle}>{group.title}</div>
            )}
            <nav className={styles.nav}>
              {group.items.map((item, itemIdx) => {
                const Icon = item.icon
                return (
                  <NavLink
                    key={itemIdx}
                    to={item.path}
                    onClick={onCloseMobileDrawer}
                    className={({ isActive }) =>
                      `${styles.navItem} ${isActive || location.pathname.startsWith(item.path + '/') ? styles.active : ''}`
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

      {/* 桌面端折叠按钮；移动端抽屉不展示该控制。 */}
      <div className={styles.footer}>
        <button className={styles.toggleBtn} onClick={onToggleCollapse}>
          {isCollapsed ? <PanelLeftOpen size={20} /> : <PanelLeftClose size={20} />}
        </button>
      </div>
    </aside>
  )
}

export default AppSidebar
