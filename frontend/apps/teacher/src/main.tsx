// 教师端入口：登录后直达教师课程管理页，并接入共享 API 与设计系统。

import React from 'react'
import { createRoot } from 'react-dom/client'
import { ChaimirApp } from '@chaimir/shared'
import { AuthGate } from '@chaimir/auth'
import { teacherApp } from './features/app-definition'

const root = document.getElementById('root')

if (!root) {
  document.body.textContent = '页面加载失败，请刷新后重试'
} else {
  createRoot(root).render(
    <React.StrictMode>
      <AuthGate>
        <ChaimirApp definition={teacherApp} />
      </AuthGate>
    </React.StrictMode>
  )
}
