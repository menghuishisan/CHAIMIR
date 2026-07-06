// 本文件承接 M4 仿真工作台页面,只做授权装配和会话日志上报,可视化运行交给 sim-sdk。

import React, { useEffect, useMemo, useState } from 'react'
import type { ChaimirApi, SimBundleDownloadGrant } from '@chaimir/api-client'
import { routeHref } from '../../app/router'
import { readFrontendConfig } from '../../lib/config'
import { Button } from '@chaimir/ui'
import { SimulationWorkbench } from '@chaimir/sim-sdk'
import type { SimEvent, SimInitParams } from '@chaimir/sim-sdk'

interface SimWorkspacePageProps {
  api: ChaimirApi
  params: URLSearchParams
}

interface GrantState {
  loading: boolean
  grant?: SimBundleDownloadGrant
  message?: string
}

/**
 * SimWorkspacePage 通过后端授权结果装配真实仿真 Worker,不在业务页面复制内置包定义。
 */
export function SimWorkspacePage({ api, params }: SimWorkspacePageProps): React.ReactElement {
  const code = params.get('code')?.trim() ?? ''
  const version = params.get('version')?.trim() ?? ''
  const sessionId = params.get('session_id')?.trim() ?? ''
  const seed = normalizeSeed(params.get('seed'))
  const initParams = useMemo<SimInitParams>(() => readInitParams(params.get('init_params')), [params])
  const [state, setState] = useState<GrantState>({ loading: true })
  const exitToLibrary = () => {
    window.location.hash = routeHref('sim-lib').slice(1)
  }

  useEffect(() => {
    let active = true
    if (!code || !version) {
      setState({ loading: false, message: '请选择一个已发布的仿真包后再进入工作台' })
      return () => {
        active = false
      }
    }
    setState({ loading: true })
    api.sim.getBundleGrant(code, version)
      .then((grant) => {
        if (active) {
          setState({ loading: false, grant })
        }
      })
      .catch(() => {
        if (active) {
          setState({ loading: false, message: '仿真包暂时无法运行，请返回列表后重试' })
        }
      })
    return () => {
      active = false
    }
  }, [api, code, version])

  /**
   * reportUserAction 只上报学习者真实交互,自动 tick 不写入后端动作日志。
   */
  function reportUserAction(event: SimEvent): void {
    if (!sessionId || event.source !== 'user') {
      return
    }
    void api.sim.reportAction(sessionId, {
      seq: event.seq,
      at_tick: event.atTick,
      event_type: event.type,
      payload: event.payload,
    })
  }

  if (state.loading) {
    return <WorkspaceMessage title="仿真正在准备" message="正在获取仿真运行授权，请稍候" onExit={exitToLibrary} />
  }
  if (state.message || !state.grant) {
    return <WorkspaceMessage title="仿真暂时不可用" message={state.message ?? '仿真包暂时无法运行，请返回列表后重试'} onExit={exitToLibrary} />
  }
  if (state.grant.builtin_code) {
    return (
      <SimulationWorkbench
        builtinCode={state.grant.builtin_code}
        initParams={initParams}
        seed={seed}
        workerCommandTimeoutMs={readFrontendConfig().simWorkerCommandTimeoutMs}
        onActionLog={reportUserAction}
        onExit={exitToLibrary}
      />
    )
  }
  if (state.grant.module_url) {
    return (
      <SimulationWorkbench
        moduleUrl={state.grant.module_url}
        initParams={initParams}
        seed={seed}
        workerCommandTimeoutMs={readFrontendConfig().simWorkerCommandTimeoutMs}
        onActionLog={reportUserAction}
        onExit={exitToLibrary}
      />
    )
  }
  return <WorkspaceMessage title="仿真暂时不可用" message="该仿真包尚未提供可运行模块，请联系管理员完成发布配置" onExit={exitToLibrary} />
}

/**
 * WorkspaceMessage 复用 sim-sdk 工作台壳层语义,让授权失败、缺参和加载态都保持沉浸式布局。
 */
function WorkspaceMessage({ title, message, onExit }: { title: string; message: string; onExit: () => void }): React.ReactElement {
  return (
    <main className="sim-workbench" aria-label="仿真工作台">
      <header className="sim-workbench__bar">
        <div>
          <p className="sim-workbench__kicker">仿真可视化引擎</p>
          <h1>{title}</h1>
        </div>
        <Button variant="on-dark" size="sm" onClick={onExit}>
          返回仿真实验室
        </Button>
      </header>
      <section className="sim-workbench__empty">
        <p>{message}</p>
      </section>
    </main>
  )
}

/**
 * normalizeSeed 统一生成确定性 seed,避免无 seed 时每次刷新得到不同结果。
 */
function normalizeSeed(raw: string | null): number {
  if (!raw) {
    return 2026070501
  }
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : 2026070501
}

/**
 * readInitParams 解析 URL 初始化参数,无效 JSON 直接使用空对象并保持用户向错误边界简单。
 */
function readInitParams(raw: string | null): SimInitParams {
  if (!raw) {
    return {}
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as SimInitParams : {}
  } catch {
    return {}
  }
}
