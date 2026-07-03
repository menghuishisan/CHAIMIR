// 四端共享应用壳：统一 API 装配、角色导航、顶栏通知、错误边界和响应式侧栏。

import React, { useEffect, useMemo, useState } from 'react'
import { createApi } from '@chaimir/api-client'
import type { ChaimirApi } from '@chaimir/api-client'
import { AlertCircle, Bell, CheckCircle2, ChevronLeft, LogOut, Menu, PanelLeftClose, PanelLeftOpen, RefreshCw, Search } from 'lucide-react'
import { Badge, Button, Card, CardBody, CardHeader, Empty, FormField, Input, Spinner, Stat, Table, Textarea } from '@chaimir/ui'
import type { TableColumn } from '@chaimir/ui'
import { clearSession, getAccessToken, getTraceId } from './storage'
import { parseHashRoute, routeHref } from './router'
import { readFrontendConfig } from './config'
import { toUserFacingError, UserFacingError } from './errors'
import type { ActionValues, AppDefinition, AppRoute, DataRow, PageAction, ResourceResult, RowAction, WorkspaceResult } from './types'
import './ChaimirApp.css'

export interface ChaimirAppProps {
  /** 当前四端入口提供的应用定义，业务页面定义归属各端 features 目录。 */
  definition: AppDefinition
}

interface PageState {
  loading: boolean
  error?: UserFacingError
  result?: ResourceResult | WorkspaceResult
  refreshKey: number
}

interface OperationState {
  key?: string
  loading: boolean
  message?: string
  error?: UserFacingError
}

/**
 * ChaimirApp 接收四端业务定义，并将路由渲染交给共享页面容器。
 */
export function ChaimirApp({ definition }: ChaimirAppProps): React.ReactElement {
  const app = definition
  const config = useMemo(() => readFrontendConfig(), [])
  const api = useMemo<ChaimirApi>(() => createApi({
    baseURL: config.apiBaseUrl,
    timeout: config.requestTimeoutMs,
    getToken: getAccessToken,
    getTraceId,
  }), [config.apiBaseUrl, config.requestTimeoutMs])
  const [route, setRoute] = useState(() => normalizeRoute(app, parseHashRoute(window.location.hash)))

  useEffect(() => {
    const handleHashChange = () => setRoute(normalizeRoute(app, parseHashRoute(window.location.hash)))
    if (!window.location.hash) {
      window.location.hash = routeHref(app.homePath).slice(1)
    }
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [app])

  return (
    <AppErrorBoundary>
      <AppShell app={app} activePath={route.route.path} api={api}>
        <RoutePage api={api} route={route.route} params={route.params} app={app} />
      </AppShell>
    </AppErrorBoundary>
  )
}

/**
 * AppShell 渲染四端一致的深色框架、顶栏、侧栏和移动抽屉。
 */
function AppShell({
  app,
  activePath,
  api,
  children,
}: {
  app: AppDefinition
  activePath: string
  api: ChaimirApi
  children: React.ReactNode
}): React.ReactElement {
  const [collapsed, setCollapsed] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [unread, setUnread] = useState<number | null>(null)
  const [noticeError, setNoticeError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    api.notify.getUnreadCount()
      .then((result) => {
        if (active) {
          setUnread(result.unread)
          setNoticeError(null)
        }
      })
      .catch((error: unknown) => {
        if (active) {
          const userError = toUserFacingError(error)
          setNoticeError(userError.message)
        }
      })
    return () => {
      active = false
    }
  }, [api])

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setDrawerOpen(false)
      }
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [])

  return (
    <div className={`chaimir-app ${collapsed ? 'is-collapsed' : ''}`}>
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <header className="chaimir-app__topbar">
        <button className="chaimir-app__icon-button chaimir-app__mobile-menu" type="button" aria-label="打开导航" onClick={() => setDrawerOpen(true)}>
          <Menu size={20} aria-hidden="true" />
        </button>
        <a className="chaimir-app__brand" href={routeHref(app.homePath)}>
          <span className="chaimir-app__brand-mark" aria-hidden="true">C</span>
          <span className="chaimir-app__brand-copy">
            <strong>Chaimir</strong>
            <span>{app.title}</span>
          </span>
        </a>
        <Input
          className="chaimir-app__search"
          type="search"
          placeholder="搜索当前功能"
          aria-label="搜索当前功能"
          leftIcon={<Search size={16} />}
          value={search}
          fullWidth
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="chaimir-app__top-actions">
          <a className="chaimir-app__icon-button" href={routeHref('notifications')} aria-label={noticeError ?? '查看通知'}>
            <Bell size={19} aria-hidden="true" />
            {unread !== null && unread > 0 && <span className="chaimir-app__badge">{unread > 99 ? '99+' : unread}</span>}
          </a>
          <span className="chaimir-app__role-pill">{app.title}</span>
        </div>
      </header>
      <aside className="chaimir-app__sidebar" aria-label={`${app.title}导航`}>
        <Sidebar app={app} activePath={activePath} collapsed={collapsed} search={search} />
        <div className="chaimir-app__sidebar-footer">
          <button className="chaimir-app__nav-item" type="button" onClick={() => setCollapsed((value) => !value)} aria-label={collapsed ? '展开侧栏' : '收起侧栏'}>
            {collapsed ? <PanelLeftOpen size={18} aria-hidden="true" /> : <PanelLeftClose size={18} aria-hidden="true" />}
            <span>收起侧栏</span>
          </button>
          <button className="chaimir-app__nav-item is-muted" type="button" onClick={() => clearSession()}>
            <LogOut size={18} aria-hidden="true" />
            <span>退出登录</span>
          </button>
        </div>
      </aside>
      {drawerOpen && (
        <div className="chaimir-app__drawer" role="dialog" aria-modal="true" aria-label="移动端导航">
          <button className="chaimir-app__drawer-scrim" type="button" aria-label="关闭导航" onClick={() => setDrawerOpen(false)} />
          <div className="chaimir-app__drawer-panel">
            <Sidebar app={app} activePath={activePath} collapsed={false} search={search} onNavigate={() => setDrawerOpen(false)} />
          </div>
        </div>
      )}
      <main className="chaimir-app__main" id="main-content">
        {children}
      </main>
    </div>
  )
}

/**
 * Sidebar 输出桌面侧栏和移动抽屉共用的导航项。
 */
function Sidebar({
  app,
  activePath,
  collapsed,
  search,
  onNavigate,
}: {
  app: AppDefinition
  activePath: string
  collapsed: boolean
  search: string
  onNavigate?: () => void
}): React.ReactElement {
  const normalizedSearch = search.trim().toLowerCase()
  const visibleRoutes = app.routes
    .filter((route) => !route.hidden)
    .filter((route) => !normalizedSearch || `${route.label} ${route.description}`.toLowerCase().includes(normalizedSearch))

  return (
    <nav className="chaimir-app__nav" aria-label={`${app.title}功能`}>
      <div className="chaimir-app__nav-heading">{collapsed ? app.title.slice(0, 2) : app.subtitle}</div>
      {visibleRoutes.map((route) => {
        const Icon = route.icon
        return (
          <a
            key={route.path}
            className={`chaimir-app__nav-item ${activePath === route.path ? 'is-active' : ''}`}
            href={routeHref(route.path)}
            title={route.label}
            onClick={onNavigate}
          >
            <Icon size={18} aria-hidden="true" />
            <span>{route.label}</span>
          </a>
        )
      })}
      {visibleRoutes.length === 0 && !collapsed && (
        <div className="chaimir-app__nav-empty" role="status">没有找到匹配的功能</div>
      )}
    </nav>
  )
}

/**
 * RoutePage 拉取当前页面数据，统一处理加载、错误、空态和沉浸态。
 */
function RoutePage({
  api,
  route,
  params,
  app,
}: {
  api: ChaimirApi
  route: AppRoute
  params: URLSearchParams
  app: AppDefinition
}): React.ReactElement {
  const [state, setState] = useState<PageState>({ loading: true, refreshKey: 0 })

  useEffect(() => {
    let active = true
    setState((current) => ({ loading: true, refreshKey: current.refreshKey }))
    route.load(api, params)
      .then((result) => {
        if (active) {
          setState((current) => ({ loading: false, result, refreshKey: current.refreshKey }))
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setState((current) => ({ loading: false, error: toUserFacingError(error), refreshKey: current.refreshKey }))
        }
      })
    return () => {
      active = false
    }
  }, [api, route, params, state.refreshKey])

  const refresh = () => setState((current) => ({ ...current, refreshKey: current.refreshKey + 1 }))

  if (route.immersive) {
    return <ImmersivePage route={route} state={state} onRefresh={refresh} />
  }

  return (
    <section className="chaimir-page" aria-labelledby="page-title">
      <PageHeading app={app} route={route} />
      {state.loading && <LoadingState />}
      {state.error && <ErrorState error={state.error} />}
      {state.result && 'columns' in state.result && <ResourceView result={state.result} onRefresh={refresh} />}
    </section>
  )
}

/**
 * PageHeading 输出统一页面标题，登录后直达功能页而不是工作台落地页。
 */
function PageHeading({ app, route }: { app: AppDefinition; route: AppRoute }): React.ReactElement {
  const Icon = route.icon
  return (
    <div className="chaimir-page__heading">
      <div>
        <nav className="chaimir-page__breadcrumb" aria-label="当前位置">
          <a href={routeHref(app.homePath)}>{app.title}</a>
          <span aria-hidden="true">/</span>
          <span aria-current="page">{route.label}</span>
        </nav>
        <h1 id="page-title">{route.label}</h1>
        <p>{route.description}</p>
      </div>
      <div className="chaimir-page__heading-icon" aria-hidden="true">
        <Icon size={22} />
      </div>
    </div>
  )
}

/**
 * ResourceView 渲染后端资源页的指标和数据表格。
 */
function ResourceView({ result, onRefresh }: { result: ResourceResult; onRefresh: () => void }): React.ReactElement {
  const [operation, setOperation] = useState<OperationState>({ loading: false })
  const columns: TableColumn<DataRow>[] = result.columns.map((column) => ({
    key: column.key,
    title: column.title,
    dataIndex: column.key,
    priority: column.priority,
    align: column.align,
  }))
  const tableColumns = result.rowActions && result.rowActions.length > 0
    ? [...columns, actionColumn(result.rowActions, setOperation, onRefresh)]
    : columns

  return (
    <div className="chaimir-page__content">
      {result.metrics && result.metrics.length > 0 && (
        <div className="chaimir-page__stats">
          {result.metrics.map((metric) => (
            <Stat key={metric.label} label={metric.label} value={metric.value} description={metricDescription(metric.tone)} />
          ))}
        </div>
      )}
      {result.actions && result.actions.length > 0 && (
        <div className="chaimir-page__actions" aria-label="页面操作">
          {result.actions.map((action) => (
            <ActionCard
              key={action.key}
              action={action}
              operation={operation}
              setOperation={setOperation}
              onRefresh={onRefresh}
            />
          ))}
        </div>
      )}
      {operation.message && (
        <div className="chaimir-operation is-success" role="status">
          <CheckCircle2 size={18} aria-hidden="true" />
          <span>{operation.message}</span>
        </div>
      )}
      {operation.error && (
        <div className="chaimir-operation is-error" role="alert">
          <AlertCircle size={18} aria-hidden="true" />
          <span>{operation.error.traceId ? `${operation.error.message} 如需帮助，请提供编号 ${operation.error.traceId}。` : operation.error.message}</span>
        </div>
      )}
      <Table
        columns={tableColumns}
        rows={result.rows}
        rowKey="id"
        emptyTitle={result.emptyTitle}
        emptyDescription={result.emptyDescription}
      />
    </div>
  )
}

/**
 * ActionCard 渲染服务端操作表单，提交后刷新当前页面数据。
 */
function ActionCard({
  action,
  operation,
  setOperation,
  onRefresh,
}: {
  action: PageAction
  operation: OperationState
  setOperation: React.Dispatch<React.SetStateAction<OperationState>>
  onRefresh: () => void
}): React.ReactElement {
  const [values, setValues] = useState<ActionValues>({})
  const loading = operation.loading && operation.key === action.key

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setOperation({ key: action.key, loading: true })
    try {
      const message = await action.execute(values)
      setOperation({ key: action.key, loading: false, message })
      setValues({})
      onRefresh()
    } catch (error) {
      setOperation({ key: action.key, loading: false, error: toUserFacingError(error) })
    }
  }

  return (
    <Card className="chaimir-action-card">
      <CardHeader>
        <div>
          <strong>{action.label}</strong>
          <p>{action.description}</p>
        </div>
      </CardHeader>
      <CardBody>
        <form className="chaimir-action-form" onSubmit={submit}>
          {action.fields.map((field) => {
            const fieldId = `action-${action.key}-${field.name}`
            const value = typeof values[field.name] === 'string' ? values[field.name] as string : ''
            return (
            <FormField
              key={field.name}
              className="chaimir-action-field"
              label={field.label}
              htmlFor={fieldId}
              required={field.required}
              helperText={field.helper}
            >
              {field.type === 'textarea' ? (
                <Textarea
                  id={fieldId}
                  name={field.name}
                  required={field.required}
                  placeholder={field.placeholder}
                  value={value}
                  onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
                />
              ) : (
                <Input
                  id={fieldId}
                  name={field.name}
                  type={field.type}
                  required={field.required}
                  placeholder={field.placeholder}
                  fullWidth
                  value={field.type === 'file' ? undefined : value}
                  onChange={(event) => setValues((current) => ({
                    ...current,
                    [field.name]: field.type === 'file' && event.target.files?.[0] ? event.target.files[0] : event.target.value,
                  }))}
                />
              )}
            </FormField>
            )
          })}
          <Button type="submit" loading={loading}>{action.submitLabel}</Button>
        </form>
      </CardBody>
    </Card>
  )
}

function actionColumn(
  actions: RowAction[],
  setOperation: React.Dispatch<React.SetStateAction<OperationState>>,
  onRefresh: () => void
): TableColumn<DataRow> {
  return {
    key: 'row_actions',
    title: '操作',
    priority: 'secondary',
    render: (row) => (
      <div className="chaimir-row-actions">
        {actions.map((action) => (
          <Button
            key={action.key}
            size="sm"
            variant="outline"
            onClick={() => runRowAction(action, row, setOperation, onRefresh)}
          >
            {action.label}
          </Button>
        ))}
      </div>
    ),
  }
}

async function runRowAction(
  action: RowAction,
  row: DataRow,
  setOperation: React.Dispatch<React.SetStateAction<OperationState>>,
  onRefresh: () => void
): Promise<void> {
  setOperation({ key: action.key, loading: true })
  try {
    const message = await action.execute(row)
    setOperation({ key: action.key, loading: false, message })
    onRefresh()
  } catch (error) {
    setOperation({ key: action.key, loading: false, error: toUserFacingError(error) })
  }
}

/**
 * ImmersivePage 渲染深色全屏工作台，窄屏自动堆叠。
 */
function ImmersivePage({ route, state, onRefresh }: { route: AppRoute; state: PageState; onRefresh: () => void }): React.ReactElement {
  return (
    <section className="chaimir-immersive" aria-labelledby="immersive-title">
      <header className="chaimir-immersive__bar">
        <a className="chaimir-immersive__back" href={routeHref('experiments')}>
          <ChevronLeft size={18} aria-hidden="true" />
          返回实验
        </a>
        <div>
          <h1 id="immersive-title">{route.label}</h1>
          <p>{route.description}</p>
        </div>
        <Button variant="on-dark" size="sm" icon={<RefreshCw size={16} />} onClick={onRefresh}>刷新状态</Button>
      </header>
      {state.loading && <LoadingState onDark />}
      {state.error && <ErrorState error={state.error} onDark />}
      {state.result && 'panels' in state.result && (
        <div className="chaimir-immersive__grid">
          <aside className="chaimir-immersive__panel">
            <h2>{state.result.title}</h2>
            <p>{state.result.description}</p>
            <div className="chaimir-immersive__metrics">
              {state.result.details.map((item) => (
                <Badge key={item.label} variant={badgeTone(item.tone)}>{item.label}: {item.value}</Badge>
              ))}
            </div>
          </aside>
          <section className="chaimir-immersive__stage" aria-label="工作台主区域">
            <div className="chaimir-immersive__terminal">
              <span>工作台状态</span>
              <strong>{state.result.title}</strong>
              <p>{state.result.description}</p>
              <dl>
                {state.result.details.map((item) => (
                  <div key={item.label}>
                    <dt>{item.label}</dt>
                    <dd>{item.value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          </section>
          <aside className="chaimir-immersive__panel">
            {state.result.panels.map((panel) => (
              <Card key={panel.title} className="chaimir-immersive__card">
                <CardHeader>{panel.title}</CardHeader>
                <CardBody>{panel.body}</CardBody>
              </Card>
            ))}
          </aside>
        </div>
      )}
    </section>
  )
}

class AppErrorBoundary extends React.Component<{ children: React.ReactNode }, { error?: UserFacingError }> {
  state: { error?: UserFacingError } = {}

  /**
   * getDerivedStateFromError 把渲染异常转换为用户向错误，避免白屏和技术细节外露。
   */
  static getDerivedStateFromError(error: unknown): { error: UserFacingError } {
    return { error: toUserFacingError(error) }
  }

  /**
   * render 在正常状态下透传子树，异常时展示可恢复的兜底页。
   */
  render(): React.ReactNode {
    if (this.state.error) {
      return (
        <main className="chaimir-app-error" role="alert">
          <Empty
            icon={<AlertCircle size={30} />}
            title={this.state.error.title}
            description={this.state.error.traceId ? `${this.state.error.message} 如需帮助，请提供编号 ${this.state.error.traceId}。` : this.state.error.message}
          />
          <Button type="button" onClick={() => window.location.reload()}>重新加载</Button>
        </main>
      )
    }

    return this.props.children
  }
}

/**
 * LoadingState 使用统一加载态，避免页面空白。
 */
function LoadingState({ onDark = false }: { onDark?: boolean }): React.ReactElement {
  return (
    <div className={`chaimir-state ${onDark ? 'is-on-dark' : ''}`} role="status" aria-live="polite">
      <Spinner size="lg" />
      <span>正在加载，请稍候</span>
    </div>
  )
}

/**
 * ErrorState 展示用户向错误和 trace_id，不泄漏内部技术原因。
 */
function ErrorState({ error, onDark = false }: { error: UserFacingError; onDark?: boolean }): React.ReactElement {
  return (
    <div className={`chaimir-state ${onDark ? 'is-on-dark' : ''}`} role="alert">
      <Empty
        icon={<AlertCircle size={28} />}
        title={error.title}
        description={error.traceId ? `${error.message} 如需帮助，请提供编号 ${error.traceId}。` : error.message}
      />
    </div>
  )
}

function badgeTone(tone: string | undefined): 'primary' | 'secondary' | 'success' | 'warning' | 'danger' {
  if (tone === 'success' || tone === 'warning' || tone === 'danger' || tone === 'secondary') {
    return tone
  }
  return 'primary'
}

function metricDescription(tone: string | undefined): string | undefined {
  if (tone === 'warning') return '需要关注'
  if (tone === 'danger') return '需要处理'
  if (tone === 'success') return '状态正常'
  return undefined
}

function normalizeRoute(app: AppDefinition, parsed: ReturnType<typeof parseHashRoute>): { route: AppRoute; params: URLSearchParams } {
  const route = app.routes.find((item) => item.path === parsed.path) ?? app.routes.find((item) => item.path === app.homePath) ?? app.routes[0]
  return { route, params: parsed.params }
}
