// AuthLayout 提供登录、激活、找回密码和入驻申请页面的品牌认证外壳。
import React from 'react'
import { Outlet } from 'react-router-dom'
import { Shield } from 'lucide-react'
import { NetworkAnimation } from './NetworkAnimation'
import styles from './AuthLayout.module.css'

// AuthLayout 将品牌视觉区与认证表单区组合成响应式双栏布局。
const AuthLayout: React.FC = () => {
  return (
    <div className={styles.container}>
      {/* 桌面端展示品牌视觉区，小屏专注认证表单。 */}
      <div className={styles.visualArea}>
        <div className={styles.animationWrapper}>
          <NetworkAnimation />
        </div>

        <div className={styles.brandInfo}>
          <h1 className={styles.title}>虚实相生，铸造安全之盾</h1>
          <p className={styles.subtitle}>
            下一代区块链与网络安全实训靶场
            <br />
            PBFT 共识仿真、对抗演练与教学一站式平台
          </p>
        </div>
      </div>

      {/* 认证表单由子路由注入，避免各认证页重复壳层。 */}
      <div className={styles.formArea}>
        <div className={styles.formContainer}>
          <div className={styles.logo}>
            <Shield className={styles.logoIcon} size={28} />
            <span>Chaimir</span>
          </div>
          {/* 渲染登录、激活、找回密码等具体认证页面。 */}
          <Outlet />
        </div>
      </div>
    </div>
  )
}

export default AuthLayout
