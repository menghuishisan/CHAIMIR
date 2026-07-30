// main.tsx 挂载 React 应用并注入全局 UI 令牌。
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './app/App'

// 全局样式入口:Tailwind v4 + @chaimir/ui 令牌(见 styles/app.css 三行接线说明)。
import './styles/app.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
