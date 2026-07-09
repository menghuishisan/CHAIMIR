// TopNavbar 渲染学生端与教师端共用顶栏,根据当前角色路径生成共享入口。
import React from 'react'
import { Hexagon, Menu } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { RoleTopbarActions } from '../RoleTopbarActions/RoleTopbarActions'
import styles from './TopNavbar.module.css'

interface TopNavbarProps {
  onMenuClick: () => void
}

const TopNavbar: React.FC<TopNavbarProps> = ({ onMenuClick }) => {
  const location = useLocation()
  const isTeacher = location.pathname.startsWith('/teacher')
  const roleBasePath = isTeacher ? '/teacher' : '/student'
  const brandName = isTeacher ? 'Chaimir 教学端' : 'Chaimir 学台'

  return (
    <header className={styles.navbar}>
      <div className={styles.leftSection}>
        <button
          className={styles.menuButton}
          onClick={onMenuClick}
          aria-label="Toggle Menu"
        >
          <Menu size={20} />
        </button>
        <div className={styles.brand}>
          <Hexagon className={styles.brandIcon} size={24} />
          <span className={styles.brandName}>{brandName}</span>
        </div>
      </div>

      <div className={styles.rightSection}>
        <RoleTopbarActions basePath={roleBasePath} loginPath="/auth/login" />
      </div>
    </header>
  )
}

export default TopNavbar
