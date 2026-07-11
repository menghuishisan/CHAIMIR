// SimulationWorkspacePage 复用 sim-sdk 工作台，并接入持久化回放、公开分享和后端计算实时流。

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { SimShareResult } from '@chaimir/api-client'
import { SIM_COMPUTE } from '@chaimir/api-client'
import type { JsonObject, SimEvent, SimInitParams, SimState, SimulationBackendState, SimulationInitialAction } from '@chaimir/sim-sdk'
import { SimulationWorkbench } from '@chaimir/sim-sdk'
import { Button } from '@chaimir/ui'
import { Copy, History, Radio, Share2 } from 'lucide-react'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource, useTicketedWebSocket } from '../../../../../hooks'
import { getStoredAccessToken } from '../../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

/** SimulationWorkspacePage 根据实时、会话回放或公开分享模式装配同一个仿真工作台。 */
const SimulationWorkspacePage: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const { id, shareCode: routeShareCode } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [share, setShare] = useState<SimShareResult>()
  const [backendState, setBackendState] = useState<SimulationBackendState>()
  const [actionMessage, setActionMessage] = useState('')
  const [actionError, setActionError] = useState('')
  const nextActionSeqRef = useRef(1)
  const actionQueueRef = useRef<Promise<void>>(Promise.resolve())
  const code = String(id || '')
  const requestedVersion = searchParams.get('version') || ''
  const sessionId = searchParams.get('sessionId') || ''
  const sharedCode = routeShareCode || searchParams.get('shareCode') || ''
  const publicShare = Boolean(routeShareCode)
  const replayMode = searchParams.get('replay') === '1' || Boolean(sharedCode)
  const requestedSeed = Number(searchParams.get('seed') || '1')

  const resource = useAsyncResource(async () => {
    const replay = sharedCode
      ? await api.sim.getSharedReplay(sharedCode)
      : sessionId
        ? await api.sim.getReplay(sessionId)
        : undefined
    const packageCode = replay?.package_code || code
    const version = replay?.version || requestedVersion

    // 平台内置公开包由封闭注册表装配；外部包在已有登录态时继续走受保护授权。
    if (sharedCode) {
      if (packageCode.startsWith('builtin__')) {
        return {
          grant: { builtin_code: packageCode, bundle_hash: '', expires_at: '' },
          compute: SIM_COMPUTE.FRONTEND,
          replay,
          requiresLogin: false,
        }
      }
      if (!getStoredAccessToken()) return {
        grant: { bundle_hash: '', expires_at: '' },
        compute: SIM_COMPUTE.FRONTEND,
        replay,
        requiresLogin: true,
      }
      const versions = await api.sim.getPackageVersions(packageCode)
      const selected = versions.find((item) => item.version === version)
      if (!selected) throw new Error('分享内容对应的仿真包版本已不可用。')
      return {
        grant: await api.sim.getBundleGrant(selected.code, selected.version),
        compute: selected.compute,
        replay,
        requiresLogin: false,
      }
    }

    const versions = await api.sim.getPackageVersions(packageCode)
    const selected = versions.find((item) => item.version === (version || versions[0]?.version))
    if (!selected) throw new Error('未找到可用的仿真包版本。')
    const grant = await api.sim.getBundleGrant(selected.code, selected.version)
    return { grant, compute: selected.compute, replay, requiresLogin: false }
  }, [code, requestedVersion, sessionId, sharedCode])

  const backendCompute = resource.data?.compute === SIM_COMPUTE.BACKEND && !replayMode
  const streamUrl = useMemo(
    () => sessionId && backendCompute ? api.sim.getStreamWsUrl(sessionId) : null,
    [backendCompute, sessionId],
  )
  const handleStreamMessage = useCallback((event: MessageEvent) => {
    try {
      const state = parseBackendState(event.data)
      setBackendState(state)
      setActionMessage(`后端仿真已推进到第 ${state.tick} 步。`)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '后端仿真返回了无法识别的状态，请稍后重试。'))
    }
  }, [])
  const stream = useTicketedWebSocket({ url: streamUrl, onMessage: handleStreamMessage })
  const initParams = useMemo(
    () => (resource.data?.replay?.init_params || {}) as SimInitParams,
    [resource.data?.replay?.init_params],
  )
  const initialActions = useMemo<SimulationInitialAction[]>(
    () => backendCompute ? [] : (resource.data?.replay?.actions || []).map((action) => ({
      eventType: action.event_type,
      payload: action.payload as JsonObject,
      target: typeof action.payload.target === 'string' ? action.payload.target : undefined,
      atTick: action.at_tick,
    })),
    [backendCompute, resource.data?.replay?.actions],
  )

  useEffect(() => {
    const actions = resource.data?.replay?.actions || []
    nextActionSeqRef.current = actions.reduce((max, action) => Math.max(max, action.seq), 0) + 1
    actionQueueRef.current = Promise.resolve()
    setBackendState(undefined)
  }, [resource.data?.replay?.actions, sessionId])

  /** reportActionLog 只把用户交互按独立连续序列串行写回，tick 和系统事件不进入操作日志。 */
  const reportActionLog = useCallback((event: SimEvent) => {
    if (!sessionId || replayMode || event.source !== 'user') return
    const action = {
      seq: nextActionSeqRef.current,
      at_tick: event.atTick,
      event_type: event.type,
      payload: event.payload,
    }
    nextActionSeqRef.current += 1
    actionQueueRef.current = actionQueueRef.current.then(async () => {
      await api.sim.reportAction(sessionId, action)
    })
    void actionQueueRef.current.catch((error) => {
      setActionError(userFacingErrorMessage(error, '操作记录同步中断，请刷新工作台后重试。'))
    })
  }, [replayMode, sessionId])

  /** sendBackendInteraction 把后端计算交互交给已鉴权实时通道，状态只由服务端返回。 */
  const sendBackendInteraction = useCallback((eventType: string, payload: JsonObject, target?: string) => {
    const sent = stream.send(JSON.stringify({
      event_type: eventType,
      payload: target ? { ...payload, target } : payload,
    }))
    if (!sent) setActionError('后端仿真仍在连接，请稍后再试。')
  }, [stream])

  /** shareSession 创建服务端分享码，公开回放不允许再次分享。 */
  const shareSession = async () => {
    if (!sessionId || replayMode) return
    setActionError('')
    try {
      const result = await api.sim.shareSession(sessionId)
      setShare(result)
      setActionMessage('分享码已创建。')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '暂时无法创建分享码。'))
    }
  }

  /** copyShareCode 把完整公开地址写入剪贴板并显式处理浏览器拒绝。 */
  const copyShareCode = async () => {
    if (!share?.code) return
    try {
      await navigator.clipboard.writeText(`${window.location.origin}/sim/shared/${encodeURIComponent(share.code)}`)
      setActionMessage('分享地址已复制。')
    } catch {
      setActionError('浏览器未允许复制，请手动记录分享码。')
    }
  }

  /** switchReplay 在当前会话的实时模式和服务端回放模式之间切换。 */
  const switchReplay = () => {
    const next = new URLSearchParams(searchParams)
    if (replayMode) next.delete('replay')
    else next.set('replay', '1')
    setSearchParams(next)
  }

  if (resource.status === 'loading') return <LoadingState title="正在准备仿真工作台" />
  if (resource.status === 'error') return <ErrorState error={resource.error} onRetry={resource.reload} />
  if (!resource.data) return <LoadingState title="正在准备仿真工作台" />
  if (resource.data.requiresLogin) return (
    <ErrorState
      error={null}
      title="登录后查看完整回放"
      description="这个分享使用了发布者上传的仿真包，登录后可通过受保护的运行授权继续查看。"
      actionLabel="前往登录"
      onRetry={() => navigate('/auth/login', { state: { from: `${location.pathname}${location.search}` } })}
    />
  )

  const actions = (
    <>
      {sessionId && !sharedCode && <Button variant="on-dark" size="sm" icon={<History size={15} />} onClick={switchReplay}>{replayMode ? '返回实时仿真' : '查看会话回放'}</Button>}
      {sessionId && !replayMode && <Button variant="on-dark" size="sm" icon={<Share2 size={15} />} onClick={() => void shareSession()}>创建分享地址</Button>}
      {share && <Button variant="on-dark" size="sm" icon={<Copy size={15} />} onClick={() => void copyShareCode()}>复制分享地址</Button>}
      {streamUrl && <span title={actionError || actionMessage}><Radio size={15} /> {stream.status === 'open' ? '后端计算已连接' : '后端计算连接中'}</span>}
      {actionError && <span role="alert">{actionError}</span>}
    </>
  )

  return (
    <SimulationWorkbench
      moduleUrl={resource.data.grant.module_url}
      builtinCode={resource.data.grant.builtin_code}
      initParams={initParams}
      initialActions={initialActions}
      seed={resource.data.replay?.seed || requestedSeed}
      workerCommandTimeoutMs={5000}
      computeMode={backendCompute ? 'backend' : 'frontend'}
      backendState={backendState}
      onBackendInteraction={backendCompute ? sendBackendInteraction : undefined}
      onActionLog={!replayMode && !backendCompute ? reportActionLog : undefined}
      actions={actions}
      exitLabel={publicShare ? '返回登录' : '返回仿真实验室'}
      onExit={() => navigate(publicShare ? '/auth/login' : '/student/simulations')}
    />
  )
}

/** parseBackendState 校验后端计算帧的最小渲染契约。 */
function parseBackendState(raw: unknown): SimulationBackendState {
  const value = typeof raw === 'string' ? JSON.parse(raw) as unknown : raw
  if (!value || typeof value !== 'object') throw new Error('后端仿真状态为空。')
  const frame = value as { tick?: unknown; state?: unknown }
  if (!Number.isSafeInteger(frame.tick) || Number(frame.tick) < 0 || !frame.state || typeof frame.state !== 'object' || Array.isArray(frame.state)) {
    throw new Error('后端仿真状态格式不正确。')
  }
  return { tick: Number(frame.tick), state: frame.state as SimState }
}

export default SimulationWorkspacePage
