// AuthLayout 提供登录、激活、找回密码和入驻申请页面的品牌认证外壳。
import React from 'react'
import { Outlet } from 'react-router-dom'
import { Blocks, Shield } from 'lucide-react'
import styles from './AuthLayout.module.css'

// AuthLayout 将品牌视觉区与认证表单区组合成响应式双栏布局。
const AuthLayout: React.FC = () => {
  return (
    <div className={styles.container}>
      {/* 桌面端展示稳定的品牌识别区，小屏专注认证表单。 */}
      <div className={styles.visualArea}>
        <div className={styles.brandInfo}>
          <Blocks size={40} aria-hidden="true" />
          <h1 className={styles.title}>Chaimir</h1>
          <p className={styles.subtitle}>
            区块链教学、实验与竞赛平台
          </p>
        </div>
      </div>

      {/* 认证表单由子路由注入，避免各认证页重复壳层。 */}
      <div className={styles.formArea}>
        <div className={styles.formContainer}>
          <div className={styles.logo}>
            <Shield className={styles.logoIcon} size={28} />
            <span>安全登录</span>
          </div>
          {/* 渲染登录、激活、找回密码等具体认证页面。 */}
          <Outlet />
        </div>
      </div>
    </div>
  )
}

export default AuthLayout
