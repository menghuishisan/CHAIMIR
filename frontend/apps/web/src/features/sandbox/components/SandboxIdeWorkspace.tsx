// 实验与竞赛共用的沙箱状态装配；面板管理各自的输入与外部资源。
import { useCallback, useEffect, useImperativeHandle, useRef, useState, type Ref } from 'react'
import {
  FileCode, FolderOpen, Link2, RefreshCw, SquareTerminal, TriangleAlert, Wrench,
} from 'lucide-react'
import {
  SandboxPhase, SandboxStatus, SandboxToolKind,
} from '@chaimir/api-client'
import { Button, ChainProgress, Tabs, TabsContent, TabsList, TabsTrigger } from '@chaimir/ui'
import { api } from '../../../app/api'
import { useTicketedWebSocket } from '../../../hooks/useTicketedWebSocket'
import { sandboxPhaseLabel } from '../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { SANDBOX_PHASES, SANDBOX_STATUSES } from '../options'
import { isSandboxReadyForOperation } from '../rules'
import { asRecord } from '../dynamicSpec'
import { FilesPanel } from './workspace/FilesPanel'
import { TerminalPanel } from './workspace/TerminalPanel'
import { ChainPanel } from './workspace/ChainPanel'
import { CommandPanel, WebToolPanel } from './workspace/ToolsPanel'
import type { SandboxWorkspaceApi, SandboxWorkspaceHandle, SandboxWorkspaceInstance } from './workspace/types'
export type { SandboxWorkspaceApi, SandboxWorkspaceHandle } from './workspace/types'

interface SandboxProgress {
  phase: SandboxPhase
  status: SandboxStatus
  stage: string
  message: string
  trace_id?: string
}

export interface SandboxIdeWorkspaceProps {
  sandboxId: string
  ref?: Ref<SandboxWorkspaceHandle>
  /** 上层业务页面重读实例摘要后递增,用于让工作台同步重读沙箱详情。 */
  refreshKey?: number
  /** 当前可操作性变化后回调，业务页面据此关闭提交。 */
  onReadinessChange?: (ready: boolean) => void
  /** 工作区能力入口。竞赛页面注入 M8 授权网关,实验页面使用 M2 租户内入口。 */
  workspaceApi?: SandboxWorkspaceApi
  /** 来源页面提供受控重建入口，工作台不自行创建或重放初始化。 */
  onRebuild?: () => void
  rebuilding?: boolean
}

/** 按服务端能力装配工作台；切换沙箱时释放全部面板和连接。 */
export function SandboxIdeWorkspace(props: SandboxIdeWorkspaceProps) {
  return <SandboxIdeWorkspaceSession key={props.sandboxId} {...props} />
}

function SandboxIdeWorkspaceSession({
  sandboxId,
  ref,
  refreshKey = 0,
  onReadinessChange,
  workspaceApi,
  onRebuild,
  rebuilding,
}: SandboxIdeWorkspaceProps) {
  const workspace = workspaceApi ?? api.sandbox
  const [instance, setInstance] = useState<SandboxWorkspaceInstance>()
  const [loadError, setLoadError] = useState<string>()
  const [progress, setProgress] = useState<SandboxProgress>()
  // 同一沙箱已访问面板保留挂载,切标签不丢编辑、终端或嵌入工具状态。
  const [visitedTabs, setVisitedTabs] = useState<Set<string>>(() => new Set())
  const lastReadinessRef = useRef<boolean | undefined>(undefined)
  const instanceRequestRef = useRef(0)
  const filesRef = useRef<SandboxWorkspaceHandle>(null)
  useImperativeHandle(ref, () => ({
    save: (archive) => filesRef.current?.save(archive) ?? Promise.resolve(true),
  }), [])

  const loadInstance = useCallback(async () => {
    const request = ++instanceRequestRef.current
    try {
      const current = await workspace.getInstance(sandboxId)
      if (request !== instanceRequestRef.current) return
      setInstance(current)
      setLoadError(undefined)
    } catch (error) {
      if (request !== instanceRequestRef.current) return
      setLoadError(userFacingErrorMessage(error, '实验环境的状态读不到,请稍后重试。'))
    }
  }, [sandboxId, workspace])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- starts the sandbox detail request; updates occur in its callbacks
    void loadInstance()
    return () => {
      instanceRequestRef.current += 1
    }
  }, [loadInstance, refreshKey])

  const ready = loadError === undefined && isSandboxReadyForOperation(instance)
  useEffect(() => {
    if (!onReadinessChange || lastReadinessRef.current === ready) return
    lastReadinessRef.current = ready
    void onReadinessChange(ready)
  }, [onReadinessChange, ready])

  // 持续订阅生命周期变化,就绪后回收或暂停也必须及时关闭工作面。
  // 已回收的沙箱是终态,不能把旧 phase 当成“正在释放”继续展示,也不应再建立进度连接。
  const terminal =
    instance?.status === SandboxStatus.DESTROYED || instance?.status === SandboxStatus.FAILED
  // 当前实例阶段是服务端权威状态;进度 WebSocket 只补充同一阶段的说明,不能提前开放工作面。
  const notReady = instance !== undefined && !terminal && !isSandboxReadyForOperation(instance)
  // 进度消息只补充服务端当前 phase/status 的说明,旧消息不能覆盖失败或终态文案。
  const matchingProgress =
    instance !== undefined &&
    progress?.phase === instance.phase &&
    progress.status === instance.status
      ? progress
      : undefined
  const progressUrl =
    instance !== undefined && !terminal ? workspace.getProgressWsUrl(sandboxId) : undefined

  const progressSocket = useTicketedWebSocket({
    url: progressUrl,
    onOpen: () => void loadInstance(),
    onMessage: (data) => {
      const message = parseProgress(data)
      if (!message) return
      setProgress(message)
      // 推送只触发重读,能力和生命周期仍以 HTTP 详情为准。
      void loadInstance()
    },
  })

  const toolAccess = instance?.tool_access ?? []
  const hasDynamicTools = toolAccess.some(
    (tool) => tool.kind === SandboxToolKind.WEB_EMBED || tool.kind === SandboxToolKind.COMMAND
  )

  // 工具就绪可能回退,工作台存续期间持续读取实时状态,不把一次Ready永久当成可访问。
  const shouldPoll =
    !terminal &&
    (notReady ||
      hasDynamicTools ||
      loadError !== undefined ||
      progressSocket.status === 'error' ||
      progressSocket.status === 'closed')
  useEffect(() => {
    if (!shouldPoll) return
    let disposed = false
    let timer: number | undefined
    const poll = async () => {
      await loadInstance()
      if (disposed) return
      timer = window.setTimeout(() => void poll(), 3000)
    }
    timer = window.setTimeout(() => void poll(), 3000)
    return () => {
      disposed = true
      if (timer !== undefined) window.clearTimeout(timer)
    }
  }, [loadInstance, shouldPoll])

  if (loadError && !instance) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <TriangleAlert aria-hidden="true" className="size-6 text-on-dark-danger" />
        <p className="text-sm text-on-dark-sub">{loadError}</p>
        <Button
          variant="on-dark"
          size="sm"
          leftIcon={RefreshCw}
          onClick={() => void loadInstance()}
        >
          重新读取
        </Button>
      </div>
    )
  }

  if (!instance) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <p className="text-sm text-on-dark-sub">正在连接实验环境…</p>
      </div>
    )
  }

  if (instance.status === SandboxStatus.FAILED) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <TriangleAlert aria-hidden="true" className="size-6 text-on-dark-danger" />
        <p className="text-base text-on-dark">实验环境没能准备好</p>
        <p className="max-w-md text-sm text-on-dark-sub">
          {matchingProgress?.message ?? '环境准备失败，请重新准备环境后继续。'}
        </p>
        {matchingProgress?.trace_id ? (
          <p className="font-mono text-xs text-on-dark-faint">
            如需帮助,请提供编号 {matchingProgress.trace_id}
          </p>
        ) : null}
        {onRebuild ? (
          <Button variant="primary" size="sm" loading={rebuilding} onClick={onRebuild}>
            重新准备环境
          </Button>
        ) : null}
        <Button
          variant="on-dark"
          size="sm"
          leftIcon={RefreshCw}
          onClick={() => void loadInstance()}
        >
          重新读取状态
        </Button>
      </div>
    )
  }

  if (instance.status === SandboxStatus.DESTROYED) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-base text-on-dark">实验环境已回收</p>
        <p className="max-w-md text-sm text-on-dark-sub">
          已保存的代码和实验进度仍然保留。返回实验详情后重新进入,系统会按最新状态准备环境。
        </p>
        <Button
          variant="on-dark"
          size="sm"
          leftIcon={RefreshCw}
          onClick={() => void loadInstance()}
        >
          重新读取状态
        </Button>
      </div>
    )
  }

  if (instance.status === SandboxStatus.PAUSED || instance.status === SandboxStatus.RECYCLING) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-base text-on-dark">
          {instance.status === SandboxStatus.PAUSED ? '实验环境已暂停' : '实验环境正在释放'}
        </p>
        <p className="max-w-md text-sm text-on-dark-sub">
          已保存的代码和实验进度仍然保留,当前环境暂时无法操作。
        </p>
      </div>
    )
  }

  if (notReady) {
    const phase = instance.phase
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-base text-on-dark">
          {matchingProgress?.message ?? sandboxPhaseLabel(phase)}
        </p>
        <ChainProgress
          onDark
          size="md"
          label="环境准备进度"
          total={SANDBOX_PHASES.length}
          done={SANDBOX_PHASES.indexOf(phase) + 1}
        />
        <p className="max-w-md text-sm text-on-dark-sub">
          第一次进入要拉起容器,通常十几秒。就绪后代码、终端与链操作会自动出现。
        </p>
        {progressSocket.status === 'error' || progressSocket.status === 'closed' ? (
          <div className="flex items-center gap-2">
            <span className="text-xs text-on-dark-sub">进度连接断了,可以手动刷新状态。</span>
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              onClick={() => void loadInstance()}
            >
              刷新状态
            </Button>
          </div>
        ) : null}
      </div>
    )
  }

  const capabilities = instance.capabilities
  const hasChainOperations = Object.keys(capabilities.chain_operations).length > 0
  const webTools = toolAccess.filter((tool) => tool.kind === SandboxToolKind.WEB_EMBED)
  const commandTools = toolAccess.filter((tool) => tool.kind === SandboxToolKind.COMMAND)

  const hasAnyPanel =
    capabilities.file_workspace ||
    capabilities.terminal ||
    (capabilities.command_tools && commandTools.length > 0) ||
    hasChainOperations ||
    webTools.length > 0

  if (!hasAnyPanel) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 p-6 text-center">
        <p className="text-base text-on-dark">这个环境没有可交互的工作面</p>
        <p className="max-w-sm text-sm text-on-dark-sub">
          它按老师的编排只在后台运行。按下面的检查点判分即可,不需要在这里操作。
        </p>
      </div>
    )
  }

  const defaultTab = capabilities.file_workspace
    ? 'files'
    : capabilities.terminal
      ? 'terminal'
      : hasChainOperations
        ? 'chain'
        : capabilities.command_tools && commandTools.length > 0
          ? 'commands'
          : `web:${webTools[0].tool_code}`

  return (
    <div className="flex h-full min-h-0 flex-col">
      {instance.resource_usage === null ? (
        <p className="shrink-0 px-3 py-2 text-xs text-on-dark-sub">
          资源用量暂时无法读取，环境功能可继续使用。
        </p>
      ) : null}
      {loadError ? (
        <div
          role="alert"
          className="flex shrink-0 items-center gap-3 px-3 py-2 text-sm text-on-dark-danger"
        >
          <p className="flex-1">{loadError}</p>
          <Button
            variant="on-dark"
            size="sm"
            leftIcon={RefreshCw}
            onClick={() => void loadInstance()}
          >
            重新读取状态
          </Button>
        </div>
      ) : null}
      <Tabs
        inert={loadError !== undefined}
        defaultValue={defaultTab}
        onValueChange={(tab) => setVisitedTabs((current) => new Set([...current, tab]))}
        className="flex min-h-0 flex-1 flex-col"
      >
        <TabsList className="shrink-0">
          {capabilities.file_workspace ? (
            <TabsTrigger value="files" icon={FileCode}>
              代码
            </TabsTrigger>
          ) : null}
          {capabilities.terminal ? (
            <TabsTrigger value="terminal" icon={SquareTerminal}>
              终端
            </TabsTrigger>
          ) : null}
          {hasChainOperations ? (
            <TabsTrigger value="chain" icon={Link2}>
              链操作
            </TabsTrigger>
          ) : null}
          {capabilities.command_tools && commandTools.length > 0 ? (
            <TabsTrigger value="commands" icon={Wrench}>
              工具命令
            </TabsTrigger>
          ) : null}
          {webTools.map((tool) => (
            <TabsTrigger key={tool.tool_code} value={`web:${tool.tool_code}`} icon={FolderOpen}>
              {tool.tool_code}
            </TabsTrigger>
          ))}
        </TabsList>

        {capabilities.file_workspace ? (
          <TabsContent
            value="files"
            forceMount={defaultTab === 'files' || visitedTabs.has('files') ? true : undefined}
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <FilesPanel ref={filesRef} sandboxId={sandboxId} workspaceApi={workspace} available={ready} />
          </TabsContent>
        ) : null}

        {capabilities.terminal ? (
          <TabsContent
            value="terminal"
            forceMount={defaultTab === 'terminal' || visitedTabs.has('terminal') ? true : undefined}
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <TerminalPanel sandboxId={sandboxId} workspaceApi={workspace} />
          </TabsContent>
        ) : null}

        {hasChainOperations ? (
          <TabsContent
            value="chain"
            forceMount={defaultTab === 'chain' || visitedTabs.has('chain') ? true : undefined}
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <ChainPanel
              sandboxId={sandboxId}
              workspaceApi={workspace}
              operations={capabilities.chain_operations}
            />
          </TabsContent>
        ) : null}

        {capabilities.command_tools && commandTools.length > 0 ? (
          <TabsContent
            value="commands"
            forceMount={defaultTab === 'commands' || visitedTabs.has('commands') ? true : undefined}
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <CommandPanel
              sandboxId={sandboxId}
              workspaceApi={workspace}
              tools={commandTools}
              available={ready}
              onReload={() => void loadInstance()}
            />
          </TabsContent>
        ) : null}

        {webTools.map((tool) => (
          <TabsContent
            key={tool.tool_code}
            value={`web:${tool.tool_code}`}
            forceMount={
              defaultTab === `web:${tool.tool_code}` || visitedTabs.has(`web:${tool.tool_code}`)
                ? true
                : undefined
            }
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <WebToolPanel
              key={ready ? tool.status : 'unavailable'}
              sandboxId={sandboxId}
              workspaceApi={workspace}
              toolCode={tool.tool_code}
              status={ready ? tool.status : undefined}
              onReload={loadInstance}
            />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  )
}

// 推送只补充说明；非法或其他阶段的消息不能开放操作。
function parseProgress(data: string): SandboxProgress | undefined {
  let raw: unknown
  try {
    raw = JSON.parse(data)
  } catch {
    console.error('[sandbox] 进度推送不是合法 JSON')
    return undefined
  }
  const record = asRecord(raw)
  if (!record) return undefined
  // 统一通道会把业务负载放在 payload 里,直连时就是负载本身
  const body = asRecord(record.payload) ?? record
  if (typeof body.stage !== 'string' || typeof body.message !== 'string') return undefined
  if (!isSandboxPhase(body.phase) || !isSandboxStatus(body.status)) return undefined
  return {
    phase: body.phase,
    status: body.status,
    stage: body.stage,
    message: body.message,
    trace_id: typeof body.trace_id === 'string' ? body.trace_id : undefined,
  }
}

function isSandboxPhase(value: unknown): value is SandboxPhase {
  return typeof value === 'number' && (SANDBOX_PHASES as readonly number[]).includes(value)
}

function isSandboxStatus(value: unknown): value is SandboxStatus {
  return typeof value === 'number' && (SANDBOX_STATUSES as readonly number[]).includes(value)
}
