// PlatformLoginPage 提供平台管理员登录入口，调用 identity 平台登录接口。

import React, { useCallback, useState } from 'react'
import { Button, Input } from '@chaimir/ui'
import { ArrowLeft, Hexagon } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
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
  const handleLogin = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError('请输入管理员账号和密码。')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginPlatform({ username: username.trim(), password })
      persistLoginTokens(response, false)
      navigate(loginEntryPath(response), { replace: true })
    } catch (loginError) {
      setError(userFacingErrorMessage(loginError, '登录失败，请检查管理员账号和密码。'))
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

      <section className={styles.platformForm} data-surface="dark" aria-labelledby="platform-login-title">
        <div className={styles.brand}>
          <Hexagon size={24} />
          <span>CHAIMIR PLATFORM</span>
        </div>

        <h1 id="platform-login-title">平台超级管理员通道</h1>
        <p>受限通道，仅允许已授权的平台管理员访问。</p>
        {error && <div className={styles.error} role="alert">{error}</div>}

        <form className={styles.darkFields} onSubmit={handleLogin}>
          <div className={styles.field}>
            <label htmlFor="platform-username">管理员账号</label>
            <Input
              id="platform-username"
              fullWidth
              value={username}
              autoComplete="username"
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
              autoComplete="current-password"
              placeholder="请输入登录密码"
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>

          <Button type="submit" variant="secondary" loading={submitting}>
            授权登录
          </Button>
          <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/auth/login')}>
            返回学校用户登录
          </Button>
        </form>
      </section>
    </main>
  )
}

export default PlatformLoginPage
