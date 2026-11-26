import React, { lazy, Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { LoaderCircle } from 'lucide-react'
import { loadRuntimeConfiguration } from './app/bootstrap'
import { AppStatusScreen } from './components/AppStatusScreen'
import { RenderErrorBoundary } from './components/RouteErrorBoundary'

import './styles/app.css'

// 延后加载配置与路由，启动失败由已挂载的错误边界呈现。
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
