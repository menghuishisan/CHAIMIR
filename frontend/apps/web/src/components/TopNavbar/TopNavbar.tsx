// TopNavbar 渲染四类角色共用顶栏,根据当前角色路径生成品牌和共享入口。
import React from 'react'
import { Hexagon, Menu } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { RoleTopbarActions } from '../RoleTopbarActions/RoleTopbarActions'
import { roleNavigationForPath } from '../../app/roleNavigation'
import styles from './TopNavbar.module.css'

interface TopNavbarProps {
  onMenuClick: () => void
  menuButtonRef?: React.Ref<HTMLButtonElement>
  isMenuOpen?: boolean
}

const TopNavbar: React.FC<TopNavbarProps> = ({ onMenuClick, menuButtonRef, isMenuOpen = false }) => {
  const location = useLocation()
  const navigation = roleNavigationForPath(location.pathname)

  return (
    <header className={styles.navbar}>
      <div className={styles.leftSection}>
        <button
          ref={menuButtonRef}
          type="button"
          className={styles.menuButton}
          onClick={onMenuClick}
          aria-label={isMenuOpen ? '关闭导航菜单' : '打开导航菜单'}
          aria-expanded={isMenuOpen}
        >
          <Menu size={20} />
        </button>
        <Link className={styles.brand} to={navigation.homePath} aria-label={navigation.brandName}>
          <Hexagon className={styles.brandIcon} size={24} />
          <span className={styles.brandName}>{navigation.brandName}</span>
          <span className={styles.compactBrandName} aria-hidden="true">Chaimir</span>
        </Link>
      </div>

      <div className={styles.rightSection}>
        <RoleTopbarActions basePath={navigation.pathPrefix} loginPath={navigation.loginPath} loadUnread={navigation.loadUnread} />
      </div>
    </header>
  )
}

export default TopNavbar
