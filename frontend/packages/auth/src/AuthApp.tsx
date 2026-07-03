// AuthApp：登录前认证入口，只负责公共认证页的路由、API 装配和外层布局。

import React, { useEffect, useMemo, useState } from 'react'
import { createApi } from '@chaimir/api-client'
import type { ChaimirApi } from '@chaimir/api-client'
import { ArrowLeft } from 'lucide-react'
import { getTraceId, readFrontendConfig } from '@chaimir/shared'
import type { AuthPage } from './types'
import { ActivatePage } from './pages/ActivatePage'
import { ApplyPage } from './pages/ApplyPage'
import { ChangePasswordGate } from './pages/ChangePasswordGate'
import { ForgotPage } from './pages/ForgotPage'
import { LoginPage } from './pages/LoginPage'
import { PlatformLoginPage } from './pages/PlatformLoginPage'
import { SsoPage } from './pages/SsoPage'
import './AuthApp.css'

/**
 * AuthApp 通过 hash 路由加载登录前页面，避免公共页复制到四个角色端。
 */
export function AuthApp(): React.ReactElement {
  const [page, setPage] = useState<AuthPage>(() => parsePage(window.location.hash))
  const config = useMemo(() => readFrontendConfig(), [])
  const api = useMemo<ChaimirApi>(() => createApi({
    baseURL: config.apiBaseUrl,
    timeout: config.requestTimeoutMs,
    getTraceId,
  }), [config.apiBaseUrl, config.requestTimeoutMs])

  useEffect(() => {
    const onHashChange = () => setPage(parsePage(window.location.hash))
    if (!window.location.hash) {
      window.location.hash = '#login'
    }
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return (
    <main className="public-app">
      <section className="public-hero" aria-label="平台能力概览">
        <a className="public-brand" href="#login">
          <span className="public-brand__mark" aria-hidden="true">C</span>
          <span>
            <strong>Chaimir</strong>
            <small>区块链教学、实验与竞赛平台</small>
          </span>
        </a>
        <div className="public-hero__content">
          <p className="public-kicker">统一身份入口</p>
          <h1>进入你的教学、实验或管理空间</h1>
          <p>师生账号由学校统一开通；学校入驻申请经平台审核后开通首个学校管理员账号。</p>
        </div>
        <div className="public-hero__grid" aria-hidden="true">
          <span />
          <span />
          <span />
          <span />
        </div>
      </section>
      <section className="public-panel" aria-label="账号入口">
        {page !== 'login' && page !== 'platform-login' && (
          <a className="public-back" href="#login">
            <ArrowLeft size={16} aria-hidden="true" />
            返回登录
          </a>
        )}
        {page === 'login' && <LoginPage api={api} config={config} />}
        {page === 'forgot' && <ForgotPage api={api} />}
        {page === 'sso' && <SsoPage api={api} config={config} />}
        {page === 'apply' && <ApplyPage api={api} />}
        {page === 'activate' && <ActivatePage api={api} config={config} />}
        {page === 'platform-login' && <PlatformLoginPage api={api} config={config} />}
        {page === 'change-pwd' && <ChangePasswordGate />}
      </section>
    </main>
  )
}

/**
 * parsePage 从 hash 中解析公共页名称，未知入口统一回到登录页。
 */
function parsePage(hash: string): AuthPage {
  const page = hash.replace(/^#\/?/, '').split('?')[0]
  if (page === 'forgot' || page === 'sso' || page === 'apply' || page === 'activate' || page === 'platform-login' || page === 'change-pwd') {
    return page
  }
  return 'login'
}
