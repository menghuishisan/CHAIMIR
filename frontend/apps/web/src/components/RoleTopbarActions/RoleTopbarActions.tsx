// RoleTopbarActions 统一四端顶栏的任务、通知、个人中心和退出入口。

import { useState } from 'react'
import { Bell, ChevronDown, ListTodo, LogOut, User } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../app/api'
import { useTopbarData } from '../../hooks/useTopbarData'
import { clearLoginTokens } from '../../utils/authSession'
import styles from './RoleTopbarActions.module.css'

export interface RoleTopbarActionsProps {
  basePath: string
  loginPath: string
  loadUnread?: boolean
}

/**
 * RoleTopbarActions 用共享实现承载各端顶栏动作，避免角色壳内出现假数据和重复逻辑。
 */
export function RoleTopbarActions({
  basePath,
  loginPath,
  loadUnread = true,
}: RoleTopbarActionsProps) {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const topbar = useTopbarData({
    loadUnread,
  })

  /**
   * handleLogout 先通知后端吊销会话，再清除本地令牌并回到对应登录页。
   */
  async function handleLogout(): Promise<void> {
    try {
      await api.identity.logout()
    } catch (error) {
      console.warn('退出登录请求未完成，已清除本地登录状态', error)
    } finally {
      clearLoginTokens()
      navigate(loginPath, { replace: true })
    }
  }

  return (
    <div className={styles.actions}>
      <button className={styles.iconButton} onClick={() => navigate(`${basePath}/tasks`)} aria-label="任务中心">
        <ListTodo size={20} />
      </button>

      <button
        className={styles.iconButton}
        onClick={() => navigate(`${basePath}/notifications`)}
        aria-label="消息通知"
      >
        <Bell size={20} />
        {topbar.unreadCount !== null && topbar.unreadCount > 0 ? (
          <span className={styles.badge}>{topbar.unreadCount}</span>
        ) : null}
      </button>

      <div className={styles.divider} />

      <div className={styles.profileWrap}>
        <button
          className={styles.profileButton}
          onClick={() => setMenuOpen((open) => !open)}
          aria-label="打开个人菜单"
          aria-expanded={menuOpen}
        >
          <div className={styles.avatar}>{topbar.avatar}</div>
          <div className={styles.userInfo}>
            <span className={styles.userName}>{topbar.name}</span>
            <span className={styles.userMeta}>{topbar.meta}</span>
          </div>
          <ChevronDown size={16} className={styles.chevron} />
        </button>

        {menuOpen ? (
          <div className={styles.profileMenu}>
            <div className={styles.menuHeader}>
              <div className={styles.menuAvatar}>{topbar.avatar}</div>
              <div>
                <div className={styles.menuName}>{topbar.name}</div>
                <div className={styles.menuMeta}>{topbar.meta}</div>
              </div>
            </div>
            <button className={styles.menuItem} onClick={() => navigate(`${basePath}/profile`)}>
              <User size={16} />
              <span>个人中心</span>
            </button>
            <button className={`${styles.menuItem} ${styles.logoutItem}`} onClick={() => void handleLogout()}>
              <LogOut size={16} />
              <span>退出登录</span>
            </button>
          </div>
        ) : null}
      </div>
    </div>
  )
}
