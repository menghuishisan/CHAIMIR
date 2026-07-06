// AuthBlock：公共认证页的标题、说明、成功和错误反馈容器。

import React from 'react'
import type { FormState } from '../types'

/**
 * AuthBlock 统一公共页标题、提示和错误展示。
 */
export function AuthBlock({
  title,
  description,
  state,
  children,
}: {
  title: string
  description: string
  state: FormState
  children: React.ReactNode
}): React.ReactElement {
  return (
    <div className="public-auth-block">
      <p className="public-kicker">账号入口</p>
      <h2>{title}</h2>
      <p>{description}</p>
      {state.message && <div className="public-alert is-success" role="status">{state.message}</div>}
      {state.error && <div className="public-alert is-error" role="alert">{state.error}</div>}
      {children}
    </div>
  )
}
