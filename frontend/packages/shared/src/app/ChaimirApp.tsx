// 四端共享应用壳：统一 API 装配、角色导航、顶栏通知、错误边界和响应式侧栏。

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { createApi } from '@chaimir/api-client'
import type { Account, ChaimirApi } from '@chaimir/api-client'
import { AlertCircle, Bell, CheckCircle2, ChevronDown, ChevronLeft, Download, LogOut, Menu, PanelLeftClose, PanelLeftOpen, RefreshCw, UserCircle } from 'lucide-react'
import { Badge, Button, Card, CardBody, CardHeader, Empty, FormField, Input, Pagination, Skeleton, Spinner, Stat, Table, Textarea } from '@chaimir/ui'
import type { TableColumn } from '@chaimir/ui'
import { clearSession, getAccessToken, getStoredUser, getTraceId } from './storage'
import { parseHashRoute, routeHref } from './router'
import { readFrontendConfig } from './config'
import { toUserFacingError, UserFacingError } from './errors'
import type { ActionValues, AppDefinition, AppRoute, DataRow, PageAction, ResourceResult, RowAction, WorkspaceResult, WorkspaceTool } from './types'
import './ChaimirApp.css'

const SIDEBAR_COLLAPSED_STORAGE_PREFIX = 'chaimir.sidebar.collapsed'

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
  const [collapsed, setCollapsed] = useState(() => readSidebarCollapsed(app))
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [accountMenuOpen, setAccountMenuOpen] = useState(false)
  const [unread, setUnread] = useState<number | null>(null)
  const [noticeError, setNoticeError] = useState<string | null>(null)
  const [logoutError, setLogoutError] = useState<string | null>(null)
  const drawerPanelRef = useRef<HTMLDivElement>(null)
  const mobileMenuRef = useRef<HTMLButtonElement>(null)
  const accountMenuRef = useRef<HTMLDivElement>(null)
  const wasDrawerOpenRef = useRef(false)
  const currentUser = getStoredUser<Account>()

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
          setNoticeError(formatErrorMessage(userError))
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
        setAccountMenuOpen(false)
      }
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [])

  useEffect(() => {
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!accountMenuRef.current?.contains(event.target as Node)) {
        setAccountMenuOpen(false)
      }
    }
    window.addEventListener('mousedown', closeOnOutsideClick)
    return () => window.removeEventListener('mousedown', closeOnOutsideClick)
  }, [])

  useEffect(() => {
    if (!drawerOpen) {
      document.body.style.overflow = ''
      if (wasDrawerOpenRef.current) {
        mobileMenuRef.current?.focus()
      }
      wasDrawerOpenRef.current = false
      return
    }

    wasDrawerOpenRef.current = true
    document.body.style.overflow = 'hidden'
    drawerPanelRef.current?.querySelector<HTMLElement>('a, button')?.focus()
    return () => {
      document.body.style.overflow = ''
    }
  }, [drawerOpen])

  useEffect(() => {
    setCollapsed(readSidebarCollapsed(app))
  }, [app])

  const toggleCollapsed = () => {
    setCollapsed((value) => {
      const next = !value
      writeSidebarCollapsed(app, next)
      return next
    })
  }

  const handleLogout = () => {
    setAccountMenuOpen(false)
    void logout(api, setLogoutError)
  }

  return (
    <div className={`chaimir-app is-app-${sanitizeClassName(app.role)} ${collapsed ? 'is-collapsed' : ''}`}>
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <header className="chaimir-app__topbar">
        <button ref={mobileMenuRef} className="chaimir-app__icon-button chaimir-app__mobile-menu" type="button" aria-label="打开导航" onClick={() => setDrawerOpen(true)}>
          <Menu size={20} aria-hidden="true" />
        </button>
        <a className="chaimir-app__brand" href={routeHref(app.homePath)}>
          <span className="chaimir-app__brand-mark" aria-hidden="true">C</span>
          <span className="chaimir-app__brand-copy">
            <strong>Chaimir</strong>
            <span>{app.title}</span>
          </span>
        </a>
        <div className="chaimir-app__top-actions">
          <a className="chaimir-app__icon-button" href={routeHref('notifications')} aria-label={noticeError ?? '查看通知'}>
            <Bell size={19} aria-hidden="true" />
            {unread !== null && unread > 0 && <span className="chaimir-app__badge">{unread > 99 ? '99+' : unread}</span>}
          </a>
          <a className="chaimir-app__icon-button" href={routeHref('transfer-tasks')} aria-label="查看任务与下载">
            <Download size={18} aria-hidden="true" />
          </a>
          <div className="chaimir-app__account-menu" ref={accountMenuRef}>
            <button
              className="chaimir-app__account-button"
              type="button"
              aria-label="打开账号菜单"
              aria-haspopup="menu"
              aria-expanded={accountMenuOpen}
              onClick={() => setAccountMenuOpen((open) => !open)}
            >
              <UserCircle size={20} aria-hidden="true" />
              <span className="chaimir-app__account-copy">
                <strong>{currentUser?.name ?? '当前账号'}</strong>
                <span>{currentUser?.no || app.title}</span>
              </span>
              <ChevronDown size={16} aria-hidden="true" />
            </button>
            {accountMenuOpen && (
              <div className="chaimir-app__account-popover" role="menu">
                <div className="chaimir-app__account-card">
                  <strong>{currentUser?.name ?? '当前账号'}</strong>
                  <span>{currentUser?.no || app.title}</span>
                </div>
                <a href={routeHref('profile')} role="menuitem" onClick={() => setAccountMenuOpen(false)}>
                  <UserCircle size={17} aria-hidden="true" />
                  <span>个人中心</span>
                </a>
                <a href={routeHref('notifications')} role="menuitem" onClick={() => setAccountMenuOpen(false)}>
                  <Bell size={17} aria-hidden="true" />
                  <span>消息中心</span>
                </a>
                <button type="button" role="menuitem" onClick={handleLogout}>
                  <LogOut size={17} aria-hidden="true" />
                  <span>退出登录</span>
                </button>
              </div>
            )}
          </div>
        </div>
      </header>
      <aside className="chaimir-app__sidebar" aria-label={`${app.title}导航`}>
        <Sidebar app={app} activePath={activePath} collapsed={collapsed} />
        <div className="chaimir-app__sidebar-footer">
          <button className="chaimir-app__nav-item" type="button" onClick={toggleCollapsed} aria-label={collapsed ? '展开侧栏' : '收起侧栏'}>
            {collapsed ? <PanelLeftOpen size={18} aria-hidden="true" /> : <PanelLeftClose size={18} aria-hidden="true" />}
            <span>收起侧栏</span>
          </button>
          {logoutError && <div className="chaimir-app__nav-empty" role="alert">{logoutError}</div>}
        </div>
      </aside>
      {drawerOpen && (
        <div className="chaimir-app__drawer" role="dialog" aria-modal="true" aria-label="移动端导航">
          <button className="chaimir-app__drawer-scrim" type="button" aria-label="关闭导航" onClick={() => setDrawerOpen(false)} />
          <div className="chaimir-app__drawer-panel" ref={drawerPanelRef} onKeyDown={(event) => trapDialogFocus(event, drawerPanelRef.current)}>
            <button className="chaimir-app__drawer-close" type="button" onClick={() => setDrawerOpen(false)}>关闭导航</button>
            <Sidebar app={app} activePath={activePath} collapsed={false} onNavigate={() => setDrawerOpen(false)} />
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
  onNavigate,
}: {
  app: AppDefinition
  activePath: string
  collapsed: boolean
  onNavigate?: () => void
}): React.ReactElement {
  const visibleRoutes = app.routes
    .filter((route) => !route.hidden)
  const groupedRoutes = visibleRoutes.reduce<Array<{ group: string; routes: AppRoute[] }>>((groups, route) => {
    const group = route.group || '功能'
    const existing = groups.find((item) => item.group === group)
    if (existing) {
      existing.routes.push(route)
    } else {
      groups.push({ group, routes: [route] })
    }
    return groups
  }, [])

  return (
    <nav className="chaimir-app__nav" aria-label={`${app.title}功能`}>
      <div className="chaimir-app__nav-heading">{collapsed ? app.title.slice(0, 2) : app.subtitle}</div>
      {groupedRoutes.map((group) => (
        <div className="chaimir-app__nav-group" key={group.group}>
          {!collapsed && <div className="chaimir-app__nav-group-title">{group.group}</div>}
          {group.routes.map((route) => {
            const Icon = route.icon
            return (
              <a
                key={route.path}
                className={`chaimir-app__nav-item ${activePath === route.path ? 'is-active' : ''}`}
                href={routeHref(route.path)}
                title={route.label}
                aria-current={activePath === route.path ? 'page' : undefined}
                onClick={onNavigate}
              >
                <Icon size={18} aria-hidden="true" />
                <span>{route.label}</span>
              </a>
            )
          })}
        </div>
      ))}
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
    if (route.render) {
      setState((current) => ({ loading: false, refreshKey: current.refreshKey }))
      return () => {
        active = false
      }
    }
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

  if (route.render) {
    return route.render({ api, params, route, app, refresh })
  }

  if (route.immersive) {
    return <ImmersivePage app={app} route={route} state={state} onRefresh={refresh} />
  }

  return (
    <section className={`chaimir-page ${pageClassNames(app, route)}`} aria-labelledby="page-title">
      <PageHeading app={app} route={route} />
      {state.loading && <LoadingState />}
      {state.error && <ErrorState error={state.error} />}
      {state.result && 'columns' in state.result && <ResourceView result={state.result} route={route} params={params} onRefresh={refresh} />}
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
function ResourceView({
  result,
  route,
  params,
  onRefresh,
}: {
  result: ResourceResult
  route: AppRoute
  params: URLSearchParams
  onRefresh: () => void
}): React.ReactElement {
  const [operation, setOperation] = useState<OperationState>({ loading: false })
  const hasActions = Boolean(result.actions && result.actions.length > 0)
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
      {operation.message && (
        <div className="chaimir-operation is-success" role="status">
          <CheckCircle2 size={18} aria-hidden="true" />
          <span>{operation.message}</span>
        </div>
      )}
      {operation.error && (
        <div className="chaimir-operation is-error" role="alert">
          <AlertCircle size={18} aria-hidden="true" />
          <ErrorDescription error={operation.error} />
        </div>
      )}
      <div className={`chaimir-page__workspace ${hasActions ? 'has-actions' : ''}`}>
        <section className="chaimir-resource-panel" aria-label="数据列表">
          <div className="chaimir-resource-panel__head">
            <div>
              <span className="chaimir-resource-panel__eyebrow">本页数据</span>
              <strong>数据列表</strong>
              <span>{result.rows.length > 0 ? `当前显示 ${result.rows.length} 条记录` : result.emptyTitle}</span>
            </div>
            <Button type="button" variant="outline" size="sm" icon={<RefreshCw size={16} />} onClick={onRefresh}>刷新</Button>
          </div>
          <Table
            columns={tableColumns}
            rows={result.rows}
            rowKey="id"
            emptyTitle={result.emptyTitle}
            emptyDescription={result.emptyDescription}
          />
          {result.pagination && result.pagination.totalPages > 1 && (
            <Pagination
              className="chaimir-resource-panel__pagination"
              current={result.pagination.page}
              total={result.pagination.totalPages}
              pageSize={result.pagination.size}
              totalItems={result.pagination.total}
              onChange={(page) => navigateResourcePage(route, params, page)}
              ariaLabel="数据列表分页"
            />
          )}
        </section>
        {result.actions && result.actions.length > 0 && (
          <aside className="chaimir-action-rail" aria-label="页面操作">
            <div className="chaimir-action-rail__head">
              <span>可执行操作</span>
              <p>按当前页面上下文提交，完成后自动刷新列表。</p>
            </div>
            <div className="chaimir-page__actions">
              {result.actions.map((action, index) => (
                <ActionCard
                  key={action.key}
                  action={action}
                  index={index}
                  operation={operation}
                  setOperation={setOperation}
                  onRefresh={onRefresh}
                />
              ))}
            </div>
          </aside>
        )}
      </div>
    </div>
  )
}

/**
 * navigateResourcePage 保留当前资源页查询条件，只替换分页页码。
 */
function navigateResourcePage(route: AppRoute, params: URLSearchParams, page: number): void {
  const nextParams = new URLSearchParams(params.toString())
  nextParams.set('page', String(page))
  window.location.hash = routeHref(route.path, paramsToRecord(nextParams)).slice(1)
}

/**
 * paramsToRecord 将 URLSearchParams 转成 routeHref 可接收的查询对象。
 */
function paramsToRecord(params: URLSearchParams): Record<string, string> {
  const record: Record<string, string> = {}
  params.forEach((value, key) => {
    record[key] = value
  })
  return record
}

/**
 * ActionCard 渲染服务端操作表单，提交后刷新当前页面数据。
 */
function ActionCard({
  action,
  index,
  operation,
  setOperation,
  onRefresh,
}: {
  action: PageAction
  index?: number
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
        <div className="chaimir-action-card__head">
          {typeof index === 'number' && <span aria-hidden="true">{String(index + 1).padStart(2, '0')}</span>}
          <div>
            <strong>{action.label}</strong>
            <p>{action.description}</p>
          </div>
        </div>
      </CardHeader>
      <CardBody>
        <form className="chaimir-action-form" onSubmit={submit} onInvalidCapture={focusInvalidControl}>
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

/**
 * actionColumn 为数据表追加行级操作列，操作结果由共享状态统一反馈。
 */
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

/**
 * runRowAction 执行行级动作，成功后刷新列表，失败时只展示用户向错误。
 */
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
function ImmersivePage({ app, route, state, onRefresh }: { app: AppDefinition; route: AppRoute; state: PageState; onRefresh: () => void }): React.ReactElement {
  const [operation, setOperation] = useState<OperationState>({ loading: false })
  const result = state.result && 'panels' in state.result ? state.result : undefined

  return (
    <section className="chaimir-immersive" aria-labelledby="immersive-title">
      <header className="chaimir-immersive__bar">
        <a className="chaimir-immersive__back" href={routeHref(app.homePath)}>
          <ChevronLeft size={18} aria-hidden="true" />
          返回{app.title}
        </a>
        <div>
          <h1 id="immersive-title">{route.label}</h1>
          <p>{route.description}</p>
        </div>
        <Button variant="on-dark" size="sm" icon={<RefreshCw size={16} />} onClick={onRefresh}>刷新状态</Button>
      </header>
      {state.loading && <LoadingState onDark />}
      {state.error && <ErrorState error={state.error} onDark />}
      {result && (
        <div className="chaimir-immersive__grid">
          <aside className="chaimir-immersive__panel">
            <h2>{result.title}</h2>
            <p>{result.description}</p>
            <div className="chaimir-immersive__metrics">
              {result.details.map((item) => (
                <Badge key={item.label} variant={badgeTone(item.tone)}>{item.label}: {item.value}</Badge>
              ))}
            </div>
            {result.tools && result.tools.length > 0 && <WorkspaceTools tools={result.tools} />}
          </aside>
          <section className="chaimir-immersive__stage" aria-label="工作台主区域">
            <div className="chaimir-immersive__stage-inner">
              <div className="chaimir-immersive__process" aria-label="当前流程概览">
                <span>读取状态</span>
                <span>准备工具</span>
                <span>执行与复盘</span>
              </div>
              <div className="chaimir-immersive__terminal">
                <span>工作台状态</span>
                <strong>{result.title}</strong>
                <p>{result.description}</p>
                <dl>
                  {result.details.map((item) => (
                    <div key={item.label}>
                      <dt>{item.label}</dt>
                      <dd>{item.value}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            </div>
          </section>
          <aside className="chaimir-immersive__panel">
            {result.actions && result.actions.length > 0 && (
              <div className="chaimir-immersive__actions">
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
                <ErrorDescription error={operation.error} />
              </div>
            )}
            {result.panels.map((panel) => (
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

/**
 * WorkspaceTools 渲染由后端沙箱/仿真声明的动态工具入口。
 */
function WorkspaceTools({ tools }: { tools: WorkspaceTool[] }): React.ReactElement {
  return (
    <div className="chaimir-immersive__tools" aria-label="可用工具">
      {tools.map((tool) => {
        const content = (
          <>
            <strong>{tool.label}</strong>
            <span>{tool.description}</span>
          </>
        )
        return tool.href ? (
          <a key={tool.key} className={`chaimir-immersive__tool is-${tool.kind}`} href={tool.href} target="_blank" rel="noreferrer">
            {content}
          </a>
        ) : (
          <div key={tool.key} className={`chaimir-immersive__tool is-${tool.kind}`} role="group">
            {content}
          </div>
        )
      })}
    </div>
  )
}

class AppErrorBoundary extends React.Component<{ children: React.ReactNode }, { error?: UserFacingError }> {
  state: { error?: UserFacingError } = {}

  /**
   * getDerivedStateFromError 把渲染异常转换为用户向错误，避免白屏和技术细节外露。
   */
  static getDerivedStateFromError(error: unknown): { error: UserFacingError } {
    return { error: toUserFacingError(error, { allowPlainMessage: false }) }
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
            description={this.state.error.message}
          />
          {this.state.error.traceId && <TraceId traceId={this.state.error.traceId} />}
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
  if (!onDark) {
    return (
      <div className="chaimir-loading" role="status" aria-live="polite">
        <div className="chaimir-loading__stats" aria-hidden="true">
          <Skeleton variant="block" height={92} />
          <Skeleton variant="block" height={92} />
          <Skeleton variant="block" height={92} />
        </div>
        <div className="chaimir-loading__body" aria-hidden="true">
          <Skeleton variant="block" height={320} />
          <Skeleton variant="block" height={220} />
        </div>
        <span className="sr-only">正在加载，请稍候</span>
      </div>
    )
  }

  return (
    <div className="chaimir-state is-on-dark" role="status" aria-live="polite">
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
        description={error.message}
      />
      {error.traceId && <TraceId traceId={error.traceId} />}
    </div>
  )
}

/**
 * ErrorDescription 输出用户向错误文案和独立报障编号，避免拼出内部错误结构。
 */
function ErrorDescription({ error }: { error: UserFacingError }): React.ReactElement {
  return (
    <span className="chaimir-error-copy">
      <span>{error.message}</span>
      {error.traceId && <TraceId traceId={error.traceId} compact />}
    </span>
  )
}

/**
 * TraceId 只展示报障编号，不暴露后端错误结构。
 */
function TraceId({ traceId, compact = false }: { traceId: string; compact?: boolean }): React.ReactElement {
  return (
    <span className={`chaimir-trace-id ${compact ? 'is-compact' : ''}`}>
      如需帮助，请提供编号 <code>{traceId}</code>
    </span>
  )
}

/**
 * formatErrorMessage 将错误整理为控件 aria-label 可读的一句话。
 */
function formatErrorMessage(error: UserFacingError): string {
  return error.traceId ? `${error.message} 如需帮助，请提供编号 ${error.traceId}。` : error.message
}

/**
 * focusInvalidControl 让浏览器校验失败时焦点停在第一个无效字段。
 */
function focusInvalidControl(event: React.InvalidEvent<HTMLFormElement>): void {
  const target = event.target
  if (target instanceof HTMLElement) {
    target.focus()
  }
}

/**
 * badgeTone 将业务色调映射到共享 Badge 变体，避免页面自造颜色。
 */
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

/**
 * pageClassNames 将角色和功能分组转成稳定类名，用于全端一致的视觉分层。
 */
function pageClassNames(app: AppDefinition, route: AppRoute): string {
  const role = `is-role-${sanitizeClassName(app.role)}`
  const group = route.group ? `is-group-${sanitizeClassName(route.group)}` : 'is-group-default'
  const path = `is-path-${sanitizeClassName(route.path)}`
  return `${role} ${group} ${path}`
}

/**
 * sanitizeClassName 仅保留 CSS 类名安全字符，避免业务文案进入选择器时破坏样式。
 */
function sanitizeClassName(value: string): string {
  const normalized = value.trim().toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
  return normalized.replace(/^-+|-+$/g, '') || 'default'
}

/**
 * normalizeRoute 根据 hash 定位当前路由，缺省时回到角色首页。
 */
function normalizeRoute(app: AppDefinition, parsed: ReturnType<typeof parseHashRoute>): { route: AppRoute; params: URLSearchParams } {
  const route = app.routes.find((item) => item.path === parsed.path) ?? app.routes.find((item) => item.path === app.homePath) ?? app.routes[0]
  return { route, params: parsed.params }
}

/**
 * logout 先吊销服务端会话，再清理本地登录态；吊销失败时给出用户向提示。
 */
async function logout(api: ChaimirApi, setLogoutError: (message: string | null) => void): Promise<void> {
  setLogoutError(null)
  try {
    await api.identity.logout()
  } catch (error) {
    const userError = toUserFacingError(error)
    setLogoutError(formatErrorMessage(userError))
  } finally {
    clearSession()
    window.location.assign('/#login')
  }
}

/**
 * readSidebarCollapsed 从浏览器本地状态恢复桌面侧栏折叠偏好；读取失败不影响主流程。
 */
function readSidebarCollapsed(app: AppDefinition): boolean {
  try {
    return window.localStorage.getItem(sidebarCollapsedStorageKey(app)) === 'true'
  } catch (error) {
    console.warn('无法读取侧栏显示偏好', error)
    return false
  }
}

/**
 * writeSidebarCollapsed 在用户切换侧栏时保存偏好，刷新后保持同一角色端的布局状态。
 */
function writeSidebarCollapsed(app: AppDefinition, collapsed: boolean): void {
  try {
    window.localStorage.setItem(sidebarCollapsedStorageKey(app), String(collapsed))
  } catch (error) {
    console.warn('无法保存侧栏显示偏好', error)
  }
}

/**
 * sidebarCollapsedStorageKey 按角色隔离折叠偏好，避免四端互相影响。
 */
function sidebarCollapsedStorageKey(app: AppDefinition): string {
  return `${SIDEBAR_COLLAPSED_STORAGE_PREFIX}.${app.role}`
}

/**
 * trapDialogFocus 让自定义移动端抽屉符合模态窗口的键盘焦点闭环。
 */
function trapDialogFocus(event: React.KeyboardEvent<HTMLElement>, container: HTMLElement | null): void {
  if (event.key !== 'Tab' || !container) {
    return
  }
  const focusable = getFocusableElements(container)
  if (focusable.length === 0) {
    event.preventDefault()
    container.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
    return
  }
  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

/**
 * getFocusableElements 收集当前可见且可交互的焦点目标。
 */
function getFocusableElements(container: HTMLElement): HTMLElement[] {
  const selector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')
  return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter((element) => element.offsetParent !== null)
}
