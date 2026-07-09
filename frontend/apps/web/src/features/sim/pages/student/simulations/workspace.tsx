// SimulationWorkspacePage 通过后端 bundle 授权加载 sim-sdk 沉浸式工作台。

import React, { useCallback } from 'react'
import type { SimEvent } from '@chaimir/sim-sdk'
import { SimulationWorkbench } from '@chaimir/sim-sdk'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'

const SimulationWorkspacePage: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams()
  const [searchParams] = useSearchParams()
  const code = String(id || '')
  const requestedVersion = searchParams.get('version') || ''
  const sessionId = searchParams.get('sessionId') || ''
  const seed = Number(searchParams.get('seed') || '1')
  const resource = useAsyncResource(async () => {
    const versions = await api.sim.getPackageVersions(code)
    const selected = requestedVersion
      ? versions.find((item) => item.version === requestedVersion)
      : versions[0]
    if (!selected) {
      throw new Error('未找到可用的仿真包版本。')
    }
    const grant = await api.sim.getBundleGrant(selected.code, selected.version)
    return { grant, selected }
  }, [code, requestedVersion])

  /**
   * reportActionLog 将工作台用户操作回写后端会话，未绑定 sessionId 时只本地运行。
   */
  const reportActionLog = useCallback((event: SimEvent) => {
    if (!sessionId) {
      return
    }
    void api.sim.reportAction(sessionId, {
      seq: event.seq,
      at_tick: event.atTick,
      event_type: event.type,
      payload: event.payload,
    })
  }, [sessionId])

  if (resource.status === 'loading') {
    return <LoadingState title="正在准备仿真工作台" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  if (!resource.data) {
    return <LoadingState title="正在准备仿真工作台" />
  }

  return (
    <SimulationWorkbench
      moduleUrl={resource.data.grant.module_url}
      builtinCode={resource.data.grant.builtin_code || resource.data.selected.code}
      initParams={{}}
      seed={seed}
      workerCommandTimeoutMs={5000}
      onActionLog={reportActionLog}
      onExit={() => navigate('/student/simulations')}
    />
  )
}

export default SimulationWorkspacePage
