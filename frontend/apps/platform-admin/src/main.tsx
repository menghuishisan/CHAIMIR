// 平台管理端入口：登录后直达学校管理，并接入共享路由与后端 API。

import React from 'react'
import { createRoot } from 'react-dom/client'
import { ChaimirApp } from '@chaimir/shared'
import { AuthGate } from '@chaimir/auth'
import { platformAdminApp } from './features/app-definition'

const root = document.getElementById('root')

if (!root) {
  document.body.textContent = '页面加载失败，请刷新后重试'
} else {
  createRoot(root).render(
    <React.StrictMode>
      <AuthGate>
        <ChaimirApp definition={platformAdminApp} />
      </AuthGate>
    </React.StrictMode>
  )
}
