// main.tsx 挂载 React 应用并注入全局 UI 令牌。
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './app/App'

// 全局样式统一来自 @chaimir/ui，应用层不维护第二套令牌。
import '@chaimir/ui/styles'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
