// AuthApp：登录前认证入口，只负责公共认证页的路由、API 装配和外层布局。

import React, { useEffect, useMemo, useState } from 'react'
import { createApi } from '@chaimir/api-client'
import type { ChaimirApi } from '@chaimir/api-client'
import { ArrowLeft, BookOpenCheck, FlaskConical, Trophy } from 'lucide-react'
import { readFrontendConfig } from '../../lib/config'
import { getTraceId } from '../../lib/storage'
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
  const effectivePage: AuthPage = config.deployMode === 'school' && page === 'platform-login' ? 'login' : page
  const api = useMemo<ChaimirApi>(() => createApi({
    baseURL: config.apiBaseUrl,
    wsBaseURL: config.wsBaseUrl,
    timeout: config.requestTimeoutMs,
    getTraceId,
  }), [config.apiBaseUrl, config.requestTimeoutMs, config.wsBaseUrl])

  useEffect(() => {
    const onHashChange = () => setPage(parsePage(window.location.hash))
    if (!window.location.hash) {
      window.location.hash = '#login'
    }
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  useEffect(() => {
    if (config.deployMode === 'school' && page === 'platform-login') {
      window.location.hash = '#login'
    }
  }, [config.deployMode, page])

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
          <p className="public-kicker">教学 · 实验 · 竞赛</p>
          <h1>进入你的区块链学习与实践空间</h1>
          <p>师生账号由学校统一开通。登录后，系统会按账号权限进入对应的功能页。</p>
        </div>
        <div className="public-hero__flow" aria-hidden="true">
          <svg className="public-flow-map" viewBox="0 0 320 220" focusable="false">
            <line className="public-flow-path is-teaching-lab" x1="48" y1="62" x2="160" y2="152" />
            <line className="public-flow-path is-lab-contest" x1="160" y1="152" x2="272" y2="82" />
            <line className="public-flow-path is-teaching-contest" x1="48" y1="62" x2="272" y2="82" />
          </svg>
          <span className="public-flow-node is-teaching"><BookOpenCheck size={18} /></span>
          <span className="public-flow-node is-lab"><FlaskConical size={18} /></span>
          <span className="public-flow-node is-contest"><Trophy size={18} /></span>
        </div>
        <div className="public-hero__signals" aria-label="平台范围">
          <span>课程学习</span>
          <span>沙箱实验</span>
          <span>竞赛训练</span>
        </div>
      </section>
      <section className="public-panel" aria-label="账号入口">
        <div className="public-panel__surface">
          {effectivePage !== 'login' && (
            <a className="public-back" href="#login">
              <ArrowLeft size={16} aria-hidden="true" />
              返回登录
            </a>
          )}
          {effectivePage === 'login' && <LoginPage api={api} config={config} />}
          {effectivePage === 'forgot' && <ForgotPage api={api} />}
          {effectivePage === 'sso' && <SsoPage api={api} config={config} />}
          {effectivePage === 'apply' && <ApplyPage api={api} />}
          {effectivePage === 'activate' && <ActivatePage api={api} />}
          {effectivePage === 'platform-login' && <PlatformLoginPage api={api} config={config} />}
          {effectivePage === 'change-pwd' && <ChangePasswordGate />}
        </div>
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
