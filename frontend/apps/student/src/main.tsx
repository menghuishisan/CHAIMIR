// 学生端入口：登录后直达学生课程页，并复用四端共享应用壳。

import React from 'react'
import { createRoot } from 'react-dom/client'
import { ChaimirApp } from '@chaimir/shared'
import { AuthGate } from '@chaimir/auth'
import { studentApp } from './features/app-definition'

const root = document.getElementById('root')

if (!root) {
  document.body.textContent = '页面加载失败，请刷新后重试'
} else {
  createRoot(root).render(
    <React.StrictMode>
      <AuthGate>
        <ChaimirApp definition={studentApp} />
      </AuthGate>
    </React.StrictMode>
  )
}
