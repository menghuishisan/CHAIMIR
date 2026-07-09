// AdminNavbar 渲染学校管理端顶栏,共享入口从这里进入而不占用侧栏。
import React from 'react'
import { Hexagon, Menu } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { RoleTopbarActions } from '../../components/RoleTopbarActions/RoleTopbarActions'
import styles from './AdminNavbar.module.css'

interface AdminNavbarProps {
  onMenuClick: () => void
}

const AdminNavbar: React.FC<AdminNavbarProps> = ({ onMenuClick }) => {
  const navigate = useNavigate()

  return (
    <header className={styles.header}>
      <div className={styles.left}>
        <button className={styles.menuBtn} onClick={onMenuClick} aria-label="打开导航菜单">
          <Menu size={20} />
        </button>
        <div className={styles.brand} onClick={() => navigate('/school-admin/users')}>
          <Hexagon className={styles.logoIcon} size={24} />
          <span className={styles.brandName}>Chaimir 校管端</span>
        </div>
      </div>

      <div className={styles.right}>
        <RoleTopbarActions basePath="/school-admin" loginPath="/auth/login" />
      </div>
    </header>
  )
}

export default AdminNavbar
