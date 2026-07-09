// PlatformNavbar 渲染平台管理端顶栏,固定承载任务、通知和个人入口。
import React from 'react'
import { Hexagon, Menu } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { RoleTopbarActions } from '../../components/RoleTopbarActions/RoleTopbarActions'
import styles from './PlatformNavbar.module.css'

interface PlatformNavbarProps {
  onMenuClick: () => void
}

const PlatformNavbar: React.FC<PlatformNavbarProps> = ({ onMenuClick }) => {
  const navigate = useNavigate()

  return (
    <header className={styles.header}>
      <div className={styles.left}>
        <button className={styles.menuBtn} onClick={onMenuClick} aria-label="打开导航菜单">
          <Menu size={20} />
        </button>
        <div className={styles.brand} onClick={() => navigate('/platform-admin/schools')}>
          <Hexagon className={styles.logoIcon} size={24} />
          <span className={styles.brandName}>Chaimir 平台管理</span>
        </div>
      </div>

      <div className={styles.right}>
        <RoleTopbarActions basePath="/platform-admin" loginPath="/auth/platform-login" loadUnread={false} />
      </div>
    </header>
  )
}

export default PlatformNavbar
