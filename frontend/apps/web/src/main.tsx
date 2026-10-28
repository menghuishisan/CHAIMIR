// main.tsx 挂载 React 应用并注入全局 UI 令牌。
import React, { lazy, Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { LoaderCircle } from 'lucide-react'
import { loadRuntimeConfiguration } from './app/bootstrap'
import { AppStatusScreen } from './components/AppStatusScreen'
import { RenderErrorBoundary } from './components/RouteErrorBoundary'

// 全局样式入口:Tailwind v4 + @chaimir/ui 令牌(见 styles/app.css 三行接线说明)。
import './styles/app.css'

// 配置及其消费者必须在边界挂载后加载,模块初始化错误才能进入同一恢复入口。
const App = lazy(async () => {
  await loadRuntimeConfiguration()
  return import('./app/App')
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RenderErrorBoundary routeKey={window.location.pathname}>
      <Suspense fallback={<AppStatusScreen icon={LoaderCircle} spinning title="正在打开" />}>
        <App />
      </Suspense>
    </RenderErrorBoundary>
  </React.StrictMode>,
)
