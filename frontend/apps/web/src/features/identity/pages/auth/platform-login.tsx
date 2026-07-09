// PlatformLoginPage 提供平台管理员登录入口，调用 identity 平台登录接口。

import React, { useCallback, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { Button, Input } from '@chaimir/ui'
import { Hexagon } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { persistLoginTokens, roleEntryPath } from '../../../../utils/authSession'
import styles from './public-auth.module.css'

const PlatformLoginPage: React.FC = () => {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleLogin 调用平台管理员专用登录接口并落到平台端首个功能页。
   */
  const handleLogin = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginPlatform({ username, password })
      persistLoginTokens(response, false)
      navigate(roleEntryPath(response), { replace: true })
    } catch (loginError) {
      setError((loginError as ApiError).message || '登录失败，请检查管理员账号和密码。')
    } finally {
      setSubmitting(false)
    }
  }, [navigate, password, username])

  return (
    <main className={styles.platformPage}>
      <section className={styles.platformVisual} aria-hidden="true">
        <div className={styles.grid} />
        <div className={styles.symbol}>
          <Hexagon size={120} strokeWidth={1} />
        </div>
      </section>

      <section className={styles.platformForm} aria-label="平台管理员登录">
        <div className={styles.brand}>
          <Hexagon size={24} />
          <span>CHAIMIR PLATFORM</span>
        </div>

        <h1>平台管理员登录</h1>
        <p>受限通道，仅允许已授权的平台管理员访问。</p>
        {error && <div className={styles.error}>{error}</div>}

        <div className={styles.darkFields}>
          <div className={styles.field}>
            <label htmlFor="platform-username">管理员账号</label>
            <Input
              id="platform-username"
              fullWidth
              value={username}
              placeholder="请输入管理员账号"
              onChange={(event) => setUsername(event.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="platform-password">登录密码</label>
            <Input
              id="platform-password"
              fullWidth
              type="password"
              value={password}
              placeholder="请输入登录密码"
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>

          <Button loading={submitting} onClick={handleLogin}>
            授权登录
          </Button>
        </div>
      </section>
    </main>
  )
}

export default PlatformLoginPage
