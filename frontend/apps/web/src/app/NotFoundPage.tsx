// NotFoundPage 提供全局未知路由的用户向兜底页面。
import React from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldAlert, ArrowLeft, Home } from 'lucide-react'
import { Button } from '@chaimir/ui'
import styles from './NotFoundPage.module.css'

// NotFoundPage 在未知路由时提供清晰的返回路径，避免暴露路由实现细节。
const NotFoundPage: React.FC = () => {
  const navigate = useNavigate()

  return (
    <div className={styles.page}>
      <div className={styles.panel}>
        <div className={styles.iconWrap}>
          <ShieldAlert className={styles.alertIcon} size={64} />
        </div>

        <h1 className={styles.code}>
          404
        </h1>
        <h2 className={styles.title}>
          页面不存在或暂时无法访问
        </h2>
        <p className={styles.description}>
          请检查访问链接是否正确，或返回上一页继续操作。
        </p>

        <div className={styles.actions}>
          <Button variant="on-dark" icon={<ArrowLeft size={18} />} onClick={() => navigate(-1)}>
            返回上一级
          </Button>

          <Button icon={<Home size={18} />} onClick={() => navigate('/')}>
            回到首页
          </Button>
        </div>
      </div>
    </div>
  )
}

export default NotFoundPage
