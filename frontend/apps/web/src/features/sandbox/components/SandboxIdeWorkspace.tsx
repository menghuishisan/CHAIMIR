// SandboxIdeWorkspace 是 M2 沙箱代码环境在应用层的唯一装配(实验工作台与竞赛答题工作台共用)。
//
// 它把 M2 的能力拼成一个可用的工作面:文件树 + 编辑器 + 终端 + 受控命令 + 链操作 + 网页工具。
// 三条边界必须守住:
//   1. 出现什么由运行时说,不由页面猜 —— `SandboxResponse.capabilities` 是权威来源
//      (file_workspace / terminal / command_tools / chain_operations)。运行时没声明的能力
//      连标签页都不出现,而不是渲染出来再报错。
//   2. 终端与进度是 WebSocket,一律经 useTicketedWebSocket 换短时票据建连,页面不自己 new。
//   3. 编辑器与终端的生命周期归 @chaimir/ide(Monaco/xterm 的装配与销毁),
//      本组件只负责挂载点、数据流与布局 —— 引擎包不含 React 视图层。
//
// 保存工作区会返回代码对象引用与哈希,调用方拿它去做检查点判分或竞赛提交;
// 因此「保存」是本组件对外的唯一数据出口,不在组件内替调用方决定交给谁。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  FileCode,
  FolderOpen,
  Link2,
  Play,
  RefreshCw,
  Save,
  SquareTerminal,
  TriangleAlert,
  Wrench,
} from 'lucide-react'
import {
  SandboxPhase,
  SandboxStatus,
  SandboxToolKind,
  SandboxToolStatus,
  type SandboxChainOperation,
  type SandboxChainRequest,
  type SandboxChainResponse,
  type SandboxCommandToolRunRequest,
  type SandboxCommandToolRunResponse,
  type SandboxFileEntry,
  type SandboxFileListResponse,
  type SandboxFileReadResponse,
  type SandboxFileSaveResponse,
  type SandboxFileWriteRequest,
  type SandboxInstance,
  type SandboxToolAccess,
} from '@chaimir/api-client'
import {
  mountMonacoEditor,
  mountTerminal,
  type MountedEditor,
  type MountedTerminal,
} from '@chaimir/ide'
import {
  Button,
  ChainProgress,
  Input,
  Modal,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Select,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { appConfig } from '../../../app/config'
import { useTicketedWebSocket } from '../../../hooks'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { decodeUtf8Base64, encodeUtf8Base64 } from '../../../utils/base64'
import { sandboxChainOperationLabel, sandboxPhaseLabel } from '../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { SANDBOX_PHASES, SANDBOX_STATUSES } from '../options'
import { isSandboxReadyForOperation } from '../rules'
import { asRecord } from '../runtimeSpec'

/**
 * SandboxProgress 是 M2 进度推送的形状(sandbox/dto.go ProgressMessage)。
 * stage 是后端给出的用户向阶段标识,message 已是用户向文案 —— 前端不再翻译。
 */
interface SandboxProgress {
  phase: SandboxPhase
  status: SandboxStatus
  stage: string
  message: string
  trace_id?: string
}

/** 按扩展名判定编辑器语言;未登记的按纯文本,不猜。 */
const LANGUAGE_BY_EXTENSION: Record<string, string> = {
  sol: 'sol',
  js: 'javascript',
  mjs: 'javascript',
  ts: 'typescript',
  json: 'json',
  py: 'python',
  go: 'go',
  rs: 'rust',
  java: 'java',
  md: 'markdown',
  yaml: 'yaml',
  yml: 'yaml',
  sh: 'shell',
  toml: 'ini',
}

export interface SandboxIdeWorkspaceProps {
  /** 沙箱编号:由实验实例或竞赛环境响应给出 */
  sandboxId: string
  /** 上层业务页面重读实例摘要后递增,用于让工作台同步重读沙箱详情。 */
  refreshKey?: number
  /** 保存工作区成功后回调:调用方据此提交或触发判分 */
  onSaved?: (result: SandboxFileSaveResponse) => void
  /** 当前可操作性变化后回调,业务页面据此关闭提交或恢复权威工作区引用。 */
  onReadinessChange?: (ready: boolean) => Promise<void>
  /** 工作区能力入口。竞赛页面注入 M8 授权网关,实验页面使用 M2 租户内入口。 */
  workspaceApi?: SandboxWorkspaceApi
  /** 来源页面提供受控重建入口，工作台不自行创建或重放初始化。 */
  onRebuild?: () => void
  rebuilding?: boolean
}

/**
 * SandboxWorkspaceApi 是工作台需要的最小能力面。
 * 视图不再绑定某个业务模块,由调用方选择 M2 本地入口或 M8 竞赛授权网关。
 */
export interface SandboxWorkspaceApi {
  getInstance(instanceId: string): Promise<SandboxWorkspaceInstance>
  getTerminalWsUrl(instanceId: string, container?: string): string
  getProgressWsUrl(instanceId: string): string
  readFile(instanceId: string, path: string): Promise<SandboxFileReadResponse>
  listFiles(instanceId: string, path?: string): Promise<SandboxFileListResponse>
  writeFile(
    instanceId: string,
    data: SandboxFileWriteRequest
  ): Promise<{ workspace_revision: number }>
  saveFiles(instanceId: string): Promise<SandboxWorkspaceSaveResult>
  runCommandTool(
    instanceId: string,
    toolCode: string,
    data: SandboxCommandToolRunRequest
  ): Promise<SandboxCommandToolRunResponse>
  chainDeploy(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse>
  chainSendTx(instanceId: string, data: SandboxChainRequest): Promise<SandboxChainResponse>
  chainQuery(
    instanceId: string,
    runtimeInstance: string,
    target: string
  ): Promise<SandboxChainResponse>
  getToolProxyUrl(
    instanceId: string,
    toolCode: string,
    proxyPath: string,
    toolOrigin: string
  ): Promise<string>
}

/** SandboxWorkspaceSaveResult 允许受控网关只返回修订号,不向页面暴露对象存储引用。 */
export type SandboxWorkspaceSaveResult = Pick<SandboxFileSaveResponse, 'workspace_revision'> &
  Partial<Pick<SandboxFileSaveResponse, 'code_storage_key' | 'code_hash'>>

/** 工作台只依赖公开运行时字段,租户编号由具体网关决定是否返回。 */
export type SandboxWorkspaceInstance = Omit<SandboxInstance, 'tenant_id'> & {
  tenant_id?: SandboxInstance['tenant_id']
}

/**
 * SandboxIdeWorkspace 渲染沙箱代码环境。
 * 能力与工具入口一律读 M2 的沙箱详情,不接受调用方传入 ——
 * 上游模块(M7 实例、M8 环境)的响应里工具字段命名与 M2 不同(`code` 对 `tool_code`),
 * 更重要的是「出现什么」的权威在 M2,让调用方传等于给了绕过这层判定的口子。
 */
export function SandboxIdeWorkspace(props: SandboxIdeWorkspaceProps) {
  return <SandboxIdeWorkspaceSession key={props.sandboxId} {...props} />
}

/** SandboxIdeWorkspaceSession 的状态仅属于一个沙箱,切换实例时整组工作面板重新装配。 */
function SandboxIdeWorkspaceSession({
  sandboxId,
  refreshKey = 0,
  onSaved,
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

  /** loadInstance 读取沙箱详情:能力声明与工具入口都以它为准。 */
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

  // 准备失败:环境起不来,写代码的面板都没有意义,给出原因与重试
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

  // 回收完成后的沙箱不能再操作。业务页面会提供重新进入入口,这里明确告诉用户返回详情页。
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

  // 还在准备:按阶段给出链式进度,不让人对着空白等
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
  const webTools = toolAccess.filter((tool) => tool.kind === SandboxToolKind.WEB_EMBED)
  const commandTools = toolAccess.filter((tool) => tool.kind === SandboxToolKind.COMMAND)

  // 能力全无:运行时只提供了一个不可交互的容器,说明清楚而不是给空标签页
  const hasAnyPanel =
    capabilities.file_workspace ||
    capabilities.terminal ||
    (capabilities.command_tools && commandTools.length > 0) ||
    capabilities.chain_operations.length > 0 ||
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
      : capabilities.chain_operations.length > 0
        ? 'chain'
        : capabilities.command_tools && commandTools.length > 0
          ? 'commands'
          : `web:${webTools[0].tool_code}`

  return (
    <div className="flex h-full min-h-0 flex-col">
      {instance.resource_usage === null ? (
        <p role="status" className="shrink-0 px-3 py-2 text-xs text-on-dark-sub">
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
          {capabilities.chain_operations.length > 0 ? (
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
            <FilesPanel sandboxId={sandboxId} workspaceApi={workspace} onSaved={onSaved} />
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

        {capabilities.chain_operations.length > 0 ? (
          <TabsContent
            value="chain"
            forceMount={defaultTab === 'chain' || visitedTabs.has('chain') ? true : undefined}
            className="min-h-0 flex-1 data-[state=inactive]:hidden"
          >
            <ChainPanel
              sandboxId={sandboxId}
              workspaceApi={workspace}
              operations={capabilities.chain_operations}
              runtimeInstances={instance.runtime_instances}
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

/* ---------------------------------------------------------------- 代码面板 */

interface FilesPanelProps {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  onSaved?: (result: SandboxFileSaveResponse) => void
}

/**
 * FilesPanel 渲染工作区目录与编辑器。
 * 目录逐层展开而不是一次拉全树:后端 listFiles 是按路径列一层,递归拉整棵树在
 * 大工程上会打出几十个请求,而学生真正会点开的往往只有一两层。
 */
function FilesPanel({ sandboxId, workspaceApi, onSaved }: FilesPanelProps) {
  const [path, setPath] = useState('.')
  const [entries, setEntries] = useState<SandboxFileEntry[]>([])
  const [directoryLoading, setDirectoryLoading] = useState(true)
  const [directoryError, setDirectoryError] = useState<string>()
  const [openPath, setOpenPath] = useState<string>()
  const [fileReadVersion, setFileReadVersion] = useState(0)
  const [dirty, setDirty] = useState(false)
  const [busy, setBusy] = useState(false)
  const [confirmReload, setConfirmReload] = useState(false)
  const [panelError, setPanelError] = useState<string>()
  const [editorError, setEditorError] = useState<string>()

  const editorHost = useRef<HTMLDivElement>(null)
  const editorRef = useRef<MountedEditor | undefined>(undefined)
  const directoryRequestRef = useRef(0)
  const fileRequestRef = useRef(0)
  const mountedRef = useRef(false)
  const busyRef = useRef(false)
  const dirtyRef = useRef(false)
  const editVersionRef = useRef(0)
  // 编辑版本只属于实际读入或写入的内容,不能由详情轮询或归档保存推进。
  const fileRevisionRef = useRef(0)
  // 编辑内容放进 ref:Monaco 的 onChange 回调在装配时固定,不能依赖闭包里的 state
  const contentRef = useRef('')

  /** markDirty 同步更新事件门禁,不等待下一次渲染才保护尚未写入的编辑。 */
  const markDirty = useCallback((value: boolean) => {
    dirtyRef.current = value
    setDirty(value)
  }, [])

  const language = useMemo(() => languageFor(openPath), [openPath])

  /** listDirectory 只接收最新目录读取结果,读取完成前不把初始数组显示为空目录。 */
  const listDirectory = useCallback(
    async (target: string) => {
      const request = ++directoryRequestRef.current
      setDirectoryLoading(true)
      setDirectoryError(undefined)
      try {
        const result = await workspaceApi.listFiles(sandboxId, target)
        if (request !== directoryRequestRef.current) return
        setPath(result.relative_path)
        setEntries(result.entries)
      } catch (error) {
        if (request !== directoryRequestRef.current) return
        setDirectoryError(userFacingErrorMessage(error, '这个目录暂时打不开,请稍后重试。'))
      } finally {
        if (request === directoryRequestRef.current) setDirectoryLoading(false)
      }
    },
    [sandboxId, workspaceApi]
  )

  useEffect(() => {
    mountedRef.current = true
    void listDirectory('.')
    return () => {
      mountedRef.current = false
      // 路由、沙箱或网关切换后,旧请求不能继续写入当前文件面板。
      directoryRequestRef.current += 1
      fileRequestRef.current += 1
    }
  }, [listDirectory])

  /** openFile 只载入最后一次选择的文件,避免慢请求覆盖后选文件及其修订号。 */
  const openFile = useCallback(
    async (relativePath: string, discardChanges = false) => {
      if (busyRef.current) return
      if (dirtyRef.current && !discardChanges) {
        setPanelError('当前文件有未写入的修改,请先写入,或重新读取并放弃修改。')
        return
      }
      const request = ++fileRequestRef.current
      const editVersion = editVersionRef.current
      setPanelError(undefined)
      try {
        const file = await workspaceApi.readFile(sandboxId, relativePath)
        if (request !== fileRequestRef.current) return
        if (editVersionRef.current !== editVersion) {
          setPanelError('当前文件有新修改,请先写入后再打开其他文件。')
          return
        }
        const nextContent = decodeUtf8Base64(file.content_base64)
        contentRef.current = nextContent
        fileRevisionRef.current = file.workspace_revision
        setOpenPath(file.relative_path)
        markDirty(false)
        setFileReadVersion((version) => version + 1)
      } catch (error) {
        if (request !== fileRequestRef.current) return
        setPanelError(userFacingErrorMessage(error, '这个文件暂时读不出来,请稍后重试。'))
      }
    },
    [markDirty, sandboxId, workspaceApi]
  )

  // 每次成功读取都重建编辑器,同路径重读也必须显示本次内容与语言。
  useEffect(() => {
    const host = editorHost.current
    if (!host || openPath === undefined) return
    let disposed = false
    let mounted: MountedEditor | undefined
    setEditorError(undefined)

    void mountMonacoEditor(host, {
      value: contentRef.current,
      language,
      onChange: (value) => {
        if (disposed) return
        contentRef.current = value
        editVersionRef.current += 1
        markDirty(true)
      },
    })
      .then((editor) => {
        if (disposed) {
          editor.dispose()
          return
        }
        mounted = editor
        editorRef.current = editor
      })
      .catch((error) => {
        if (!disposed) {
          setEditorError(
            userFacingErrorMessage(error, '编辑器没有加载成功,请检查网络后重新加载页面。')
          )
        }
      })

    return () => {
      disposed = true
      mounted?.dispose()
      editorRef.current = undefined
    }
    // language 随 openPath 推导,不单独列进依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileReadVersion, openPath])

  /** writeCurrentFile 把编辑内容写回工作区(还没落对象存储)。 */
  const writeCurrentFile = useCallback(async () => {
    if (!openPath || busyRef.current) return
    busyRef.current = true
    fileRequestRef.current += 1
    const savedContent = contentRef.current
    const savedEditVersion = editVersionRef.current
    setBusy(true)
    setPanelError(undefined)
    try {
      const result = await workspaceApi.writeFile(sandboxId, {
        relative_path: openPath,
        content_base64: encodeUtf8Base64(savedContent),
        expected_revision: fileRevisionRef.current,
      })
      if (!mountedRef.current) return
      fileRevisionRef.current = result.workspace_revision
      markDirty(editVersionRef.current !== savedEditVersion)
      toast.success('已写入工作区')
    } catch (error) {
      if (!mountedRef.current) return
      setPanelError(userFacingErrorMessage(error, '没能写入工作区,请稍后重试。'))
    } finally {
      busyRef.current = false
      if (mountedRef.current) setBusy(false)
    }
  }, [markDirty, openPath, sandboxId, workspaceApi])

  /**
   * persistWorkspace 把整个工作区持久化并交出代码引用。
   * 判分与提交都要这个引用:它是「这一刻的代码」的不可变快照,
   * 比让后端再去容器里抓一次可靠(容器随时可能被回收)。
   */
  const persistWorkspace = useCallback(async () => {
    if (busyRef.current) return
    busyRef.current = true
    fileRequestRef.current += 1
    setBusy(true)
    setPanelError(undefined)
    try {
      if (openPath && dirtyRef.current) {
        const savedContent = contentRef.current
        const savedEditVersion = editVersionRef.current
        const result = await workspaceApi.writeFile(sandboxId, {
          relative_path: openPath,
          content_base64: encodeUtf8Base64(savedContent),
          expected_revision: fileRevisionRef.current,
        })
        if (!mountedRef.current) return
        fileRevisionRef.current = result.workspace_revision
        markDirty(editVersionRef.current !== savedEditVersion)
      }
      const result = await workspaceApi.saveFiles(sandboxId)
      if (!mountedRef.current) return
      if (onSaved && (!result.code_storage_key || !result.code_hash)) {
        setPanelError('工作区已保存,但没有生成判分所需的快照。请稍后重试。')
        return
      }
      toast.success(dirtyRef.current ? '已保存写入的内容,还有修改需要写入' : '工作区已保存')
      if (onSaved && result.code_storage_key && result.code_hash) {
        onSaved({
          workspace_revision: result.workspace_revision,
          code_storage_key: result.code_storage_key,
          code_hash: result.code_hash,
        })
      }
    } catch (error) {
      if (!mountedRef.current) return
      setPanelError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
    } finally {
      busyRef.current = false
      if (mountedRef.current) setBusy(false)
    }
  }, [markDirty, onSaved, openPath, sandboxId, workspaceApi])

  return (
    <div className="flex h-full min-h-0 flex-col lg:flex-row">
      <div className="flex max-h-56 min-h-0 shrink-0 flex-col border-b border-dark-line lg:max-h-none lg:w-64 lg:border-b-0 lg:border-r">
        <div className="flex shrink-0 items-center justify-between gap-2 px-3 py-2">
          <span className="truncate font-mono text-xs text-on-dark-sub">{path}</span>
          <div className="flex shrink-0 items-center gap-1">
            {path !== '.' ? (
              <Button
                variant="on-dark"
                size="sm"
                onClick={() => void listDirectory(parentPath(path))}
              >
                上一层
              </Button>
            ) : null}
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              onClick={() => void listDirectory(path)}
            >
              刷新
            </Button>
          </div>
        </div>
        <ul
          aria-label="工作区目录"
          aria-busy={directoryLoading}
          className="min-h-0 flex-1 overflow-y-auto px-1 pb-2"
        >
          {directoryLoading ? (
            <li role="status" className="px-2 py-1 text-xs text-on-dark-sub">
              正在读取目录…
            </li>
          ) : directoryError ? (
            <li role="alert" className="px-2 py-1 text-xs text-on-dark-danger">
              {directoryError}
            </li>
          ) : entries.length === 0 ? (
            <li className="px-2 py-1 text-xs text-on-dark-sub">这个目录是空的</li>
          ) : (
            entries.map((entry) => (
              <li key={entry.relative_path}>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() =>
                    entry.is_dir
                      ? void listDirectory(entry.relative_path)
                      : void openFile(entry.relative_path)
                  }
                  className={
                    'hit-target relative flex w-full items-center gap-2 truncate rounded-md px-2 py-1 text-left text-sm text-on-dark-sub hover:bg-dark-surface hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2' +
                    (openPath === entry.relative_path ? ' bg-dark-surface text-on-dark' : '')
                  }
                >
                  <span aria-hidden="true" className="shrink-0 text-on-dark-faint">
                    {entry.is_dir ? '/' : '·'}
                  </span>
                  <span className="min-w-0 flex-1 truncate font-mono text-xs">{entry.name}</span>
                </button>
              </li>
            ))
          )}
        </ul>
      </div>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-dark-line px-3 py-2">
          <span className="min-w-0 truncate font-mono text-xs text-on-dark-sub">
            {openPath ?? '从左侧选一个文件开始'}
            {dirty ? ' · 有未写入的修改' : ''}
          </span>
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              disabled={!openPath || busy}
              onClick={() => {
                if (!openPath) return
                if (dirtyRef.current) setConfirmReload(true)
                else void openFile(openPath)
              }}
            >
              重新读取文件
            </Button>
            <Button
              variant="on-dark"
              size="sm"
              disabled={!openPath || !dirty || busy}
              onClick={() => void writeCurrentFile()}
            >
              写入
            </Button>
            <Button
              variant="primary"
              size="sm"
              leftIcon={Save}
              loading={busy}
              onClick={() => void persistWorkspace()}
            >
              保存工作区
            </Button>
          </div>
        </div>

        {panelError ? (
          <p role="alert" className="shrink-0 px-3 py-2 text-xs text-on-dark-danger">
            {panelError}
          </p>
        ) : null}

        {editorError ? (
          <div role="alert" className="flex shrink-0 items-center gap-3 px-3 py-2">
            <p className="flex-1 text-sm text-on-dark-danger">{editorError}</p>
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              onClick={() => window.location.reload()}
            >
              重新加载页面
            </Button>
          </div>
        ) : null}

        {openPath === undefined ? (
          <div className="flex min-h-0 flex-1 items-center justify-center px-6 text-center">
            <p className="text-sm text-on-dark-sub">
              选一个文件就能开始写。改完先「写入」,再「保存工作区」交给判分。
            </p>
          </div>
        ) : (
          <div ref={editorHost} className="min-h-0 flex-1" />
        )}
      </div>
      <Modal open={confirmReload} onOpenChange={setConfirmReload}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>放弃未写入的修改并重新读取？</ModalTitle>
            <ModalDescription>
              请先复制需要保留的内容。成功读取后会用工作区里的文件替换当前编辑,读取失败则保留当前内容。
            </ModalDescription>
          </ModalHeader>
          <ModalFooter>
            <Button variant="outline" onClick={() => setConfirmReload(false)}>
              继续编辑
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                setConfirmReload(false)
                if (openPath) void openFile(openPath, true)
              }}
            >
              放弃修改并重新读取
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  )
}

/* ---------------------------------------------------------------- 终端面板 */

/**
 * TerminalPanel 把 xterm 接到沙箱终端。
 * 终端是双向流:xterm 的输入直接送 WS,服务端输出写回 xterm ——
 * 中间不做行缓冲或命令解析,那会让 vim 这类交互程序不可用。
 */
function TerminalPanel({
  sandboxId,
  workspaceApi,
}: {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
}) {
  const host = useRef<HTMLDivElement>(null)
  const terminalRef = useRef<MountedTerminal | undefined>(undefined)
  const [ready, setReady] = useState(false)
  const [mountAttempt, setMountAttempt] = useState(0)
  const [mountError, setMountError] = useState<string>()

  const url = useMemo(() => workspaceApi.getTerminalWsUrl(sandboxId), [sandboxId, workspaceApi])

  const socket = useTicketedWebSocket({
    url: ready ? url : undefined,
    onBinaryMessage: (data) => terminalRef.current?.write(data),
    onOpen: () => terminalRef.current?.focus(),
  })

  // xterm 与本面板同生共死;输入转发经 socket.send,不在此持有任何凭据
  useEffect(() => {
    const container = host.current
    if (!container) return
    let disposed = false
    let mounted: MountedTerminal | undefined
    setReady(false)
    setMountError(undefined)

    void mountTerminal(container, {
      onData: (data) => socket.send(data),
      onBinary: (data) => socket.send(data),
    })
      .then((terminal) => {
        if (disposed) {
          terminal.dispose()
          return
        }
        mounted = terminal
        terminalRef.current = terminal
        setReady(true)
      })
      .catch((error) => {
        if (!disposed) setMountError(userFacingErrorMessage(error, '终端没有加载成功,请重新加载。'))
      })

    return () => {
      disposed = true
      mounted?.dispose()
      terminalRef.current = undefined
      setReady(false)
    }
    // socket.send 引用稳定(useCallback 空依赖),不列进依赖以免重建终端
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mountAttempt, sandboxId])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-dark-line px-3 py-2">
        <span className="text-xs text-on-dark-sub">{socketStatusText(socket.status)}</span>
        <div className="flex items-center gap-2">
          {mountError ? (
            <Button
              variant="on-dark"
              size="sm"
              leftIcon={RefreshCw}
              onClick={() => setMountAttempt((current) => current + 1)}
            >
              重新加载终端
            </Button>
          ) : null}
          <Button variant="on-dark" size="sm" onClick={() => terminalRef.current?.clear()}>
            清屏
          </Button>
          {socket.status === 'error' || socket.status === 'closed' ? (
            <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={socket.reconnect}>
              重新连接
            </Button>
          ) : null}
        </div>
      </div>
      {mountError || socket.error ? (
        <p className="shrink-0 px-3 py-2 text-xs text-on-dark-danger">
          {mountError ?? socket.error}
        </p>
      ) : null}
      <div ref={host} className="min-h-0 flex-1 bg-terminal" />
    </div>
  )
}

/* ---------------------------------------------------------------- 链操作面板 */

interface ChainPanelProps {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  operations: SandboxChainOperation[]
  runtimeInstances: string[]
}

/**
 * ChainPanel 调用运行时统一的链能力。
 * 三种操作的入参形状不同:部署与交易吃一个由运行时定义的参数对象(键不可枚举,
 * 故用文档编辑器并在本地校验合法性),查询只要一个目标标识。响应原样呈现 ——
 * 它是链上事实,不该被前端改写。
 */
function ChainPanel({ sandboxId, workspaceApi, operations, runtimeInstances }: ChainPanelProps) {
  const [operation, setOperation] = useState<SandboxChainOperation>(operations[0])
  const [runtimeInstance, setRuntimeInstance] = useState(runtimeInstances[0] ?? '')
  const [payloadText, setPayloadText] = useState('{\n  \n}')
  const [target, setTarget] = useState('')
  const [result, setResult] = useState<string>()
  const [panelError, setPanelError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const { errors, validate } = useFormValidation(() => {
    let payloadError: string | null = null
    if (operation !== 'query') {
      try {
        const parsed: unknown = JSON.parse(payloadText)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          payloadError = '内容格式不正确,最外层应当是大括号包裹的内容。'
        }
      } catch {
        payloadError = '内容格式不正确,请检查是否漏了逗号或引号。'
      }
    }
    return {
      target: operation === 'query' && !target.trim()
        ? '请填写要查询的目标,例如合约地址或状态键。' : null,
      runtimeInstance: operation === 'query' && !runtimeInstance
        ? '当前环境没有可用的运行时实例。' : null,
      payloadText: payloadError,
    }
  }, operation)

  const run = useCallback(async () => {
    if (!validate()) return
    setBusy(true)
    setPanelError(undefined)
    setResult(undefined)
    try {
      if (operation === 'query') {
        const response = await workspaceApi.chainQuery(sandboxId, runtimeInstance, target.trim())
        setResult(JSON.stringify(response, null, 2))
        return
      }

      // 同步字段校验已确认参数为对象,接口继续接收运行时规定的原对象。
      const payload = JSON.parse(payloadText) as Record<string, unknown>

      const response =
        operation === 'deploy'
          ? await workspaceApi.chainDeploy(sandboxId, {
              runtime_instance: runtimeInstance,
              payload,
            })
          : await workspaceApi.chainSendTx(sandboxId, {
              runtime_instance: runtimeInstance,
              payload,
            })
      setResult(JSON.stringify(response, null, 2))
    } catch (error) {
      setPanelError(userFacingErrorMessage(error, '这次链操作没有成功,请检查参数后重试。'))
    } finally {
      setBusy(false)
    }
  }, [operation, payloadText, runtimeInstance, sandboxId, target, validate, workspaceApi])

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          id={`${sandboxId}-chain-runtime`}
          options={runtimeInstances.map((item) => ({ value: item, label: item }))}
          value={runtimeInstance}
          placeholder="选择运行时实例"
          disabled={runtimeInstances.length === 0}
          invalid={Boolean(errors.runtimeInstance)}
          aria-describedby={errors.runtimeInstance ? `${sandboxId}-chain-runtime-error` : undefined}
          onBlur={() => validate(['runtimeInstance'])}
          onValueChange={setRuntimeInstance}
        />
        {operations.map((item) => (
          <Button
            key={item}
            variant={item === operation ? 'primary' : 'on-dark'}
            size="sm"
            onClick={() => {
              setOperation(item)
              setResult(undefined)
              setPanelError(undefined)
            }}
          >
            {sandboxChainOperationLabel(item)}
          </Button>
        ))}
      </div>

      {errors.runtimeInstance ? (
        <p id={`${sandboxId}-chain-runtime-error`} role="alert" className="text-xs text-on-dark-danger">
          {errors.runtimeInstance}
        </p>
      ) : null}

      {operation === 'query' ? (
        <label className="flex flex-col gap-1">
          <span className="text-xs text-on-dark-sub">查询目标</span>
          <Input
            variant="underline"
            value={target}
            placeholder="合约地址或状态键"
            className="font-mono text-sm"
            invalid={Boolean(errors.target)}
            aria-describedby={errors.target ? `${sandboxId}-chain-input-error` : undefined}
            onChange={(event) => setTarget(event.target.value)}
            onBlur={() => validate(['target'])}
          />
        </label>
      ) : (
        <label className="flex min-h-0 flex-col gap-1">
          <span className="text-xs text-on-dark-sub">
            {operation === 'deploy' ? '部署参数' : '交易参数'}
          </span>
          <Textarea
            onDark
            value={payloadText}
            rows={8}
            spellCheck={false}
            className="font-mono text-sm"
            invalid={Boolean(errors.payloadText)}
            aria-describedby={errors.payloadText ? `${sandboxId}-chain-input-error` : undefined}
            onChange={(event) => setPayloadText(event.target.value)}
            onBlur={() => validate(['payloadText'])}
          />
        </label>
      )}

      {errors.target || errors.payloadText ? (
        <p id={`${sandboxId}-chain-input-error`} role="alert" className="text-xs text-on-dark-danger">
          {errors.target ?? errors.payloadText}
        </p>
      ) : null}

      <div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={Play}
          loading={busy}
          onClick={() => void run()}
        >
          执行
        </Button>
      </div>

      {panelError ? <p className="text-xs text-on-dark-danger">{panelError}</p> : null}

      {result ? (
        <pre className="min-h-0 overflow-auto rounded-md border border-dark-line bg-terminal p-3 font-mono text-xs text-on-dark">
          {result}
        </pre>
      ) : null}
    </div>
  )
}

/* ---------------------------------------------------------------- 工具命令面板 */

interface CommandPanelProps {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  tools: SandboxToolAccess[]
  available: boolean
  onReload: () => void
}

/**
 * CommandPanel 执行受控命令工具。
 * 允许哪些命令由平台在工具定义里配白名单,学生侧读不到那份清单(工具定义接口在平台组),
 * 所以这里不做本地过滤 —— 白名单在服务端强制,越界命令会被拒绝并给出用户向说明。
 */
function CommandPanel({ sandboxId, workspaceApi, tools, available, onReload }: CommandPanelProps) {
  const [toolCode, setToolCode] = useState(tools[0].tool_code)
  const [commandLine, setCommandLine] = useState('')
  const [output, setOutput] = useState<string>()
  const [panelError, setPanelError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const selectedTool = tools.find((tool) => tool.tool_code === toolCode)
  const toolReady = available && selectedTool?.status === SandboxToolStatus.READY
  const { errors, validate } = useFormValidation(() => ({
    commandLine: !commandLine.trim() ? '请输入要执行的命令。' : null,
  }))

  const run = useCallback(async () => {
    if (busy || !toolReady) return
    const command = commandLine
      .split(/\s+/)
      .map((item) => item.trim())
      .filter((item) => item !== '')
    if (!validate()) return
    setBusy(true)
    setPanelError(undefined)
    setOutput(undefined)
    try {
      const response = await workspaceApi.runCommandTool(sandboxId, toolCode, { command })
      const stdout = decodeUtf8Base64(response.stdout_base64)
      const stderr = decodeUtf8Base64(response.stderr_base64)
      setOutput(
        [stdout, stderr ? `\n${stderr}` : '', `\n[结束状态 ${response.exit_code}]`].join('')
      )
    } catch (error) {
      setPanelError(
        userFacingErrorMessage(error, '这条命令没能执行。它可能不在这个工具允许的范围内。')
      )
    } finally {
      setBusy(false)
    }
  }, [busy, commandLine, sandboxId, toolCode, toolReady, validate, workspaceApi])

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      {tools.length > 1 ? (
        <div className="flex flex-wrap items-center gap-2">
          {tools.map((tool) => (
            <Button
              key={tool.tool_code}
              variant={tool.tool_code === toolCode ? 'primary' : 'on-dark'}
              size="sm"
              disabled={busy}
              onClick={() => setToolCode(tool.tool_code)}
            >
              {tool.tool_code}
            </Button>
          ))}
        </div>
      ) : null}

      {!toolReady ? (
        <ToolReadinessNotice
          status={available ? selectedTool?.status : undefined}
          onReload={onReload}
        />
      ) : null}

      <label className="flex flex-col gap-1">
        <span className="text-xs text-on-dark-sub">命令与参数(按空格分隔)</span>
        <Input
          variant="underline"
          value={commandLine}
          disabled={!toolReady}
          placeholder="forge test"
          className="font-mono text-sm"
          invalid={Boolean(errors.commandLine)}
          aria-describedby={errors.commandLine ? `${sandboxId}-command-input-error` : undefined}
          onChange={(event) => setCommandLine(event.target.value)}
          onBlur={() => validate(['commandLine'])}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !busy) void run()
          }}
        />
      </label>

      {errors.commandLine ? (
        <p id={`${sandboxId}-command-input-error`} role="alert" className="text-xs text-on-dark-danger">
          {errors.commandLine}
        </p>
      ) : null}

      <div>
        <Button
          variant="primary"
          size="sm"
          leftIcon={Play}
          loading={busy}
          disabled={!toolReady}
          onClick={() => void run()}
        >
          执行
        </Button>
      </div>

      {panelError ? <p className="text-xs text-on-dark-danger">{panelError}</p> : null}

      {output ? (
        <pre className="min-h-0 overflow-auto whitespace-pre-wrap rounded-md border border-dark-line bg-terminal p-3 font-mono text-xs text-on-dark">
          {output}
        </pre>
      ) : null}
    </div>
  )
}

/* ---------------------------------------------------------------- 网页工具面板 */

/**
 * WebToolPanel 嵌入网页类工具。
 * 地址是平台自己的代理入口(不是工具容器的直连地址),鉴权由路径受限令牌承载 ——
 * 页面不拼容器地址,也就不存在把内网地址暴露给浏览器的问题。
 */
function WebToolPanel({
  sandboxId,
  workspaceApi,
  toolCode,
  status,
  onReload,
}: {
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  toolCode: string
  status: SandboxToolStatus | undefined
  onReload: () => void
}) {
  const [src, setSrc] = useState<string>()
  const [loadError, setLoadError] = useState('')
  const [loadAttempt, setLoadAttempt] = useState(0)

  useEffect(() => {
    let active = true
    setSrc(undefined)
    setLoadError('')
    if (status !== SandboxToolStatus.READY) return
    // 首次打开和主动重开都重读权威状态,旧标签中的Ready不能直接换票。
    void (async () => {
      const current = await workspaceApi.getInstance(sandboxId)
      if (!active) return
      const tool = current.tool_access.find((item) => item.tool_code === toolCode)
      if (!isSandboxReadyForOperation(current) || tool?.status !== SandboxToolStatus.READY) {
        setLoadError('工具尚未准备好,请稍后重新打开。')
        onReload()
        return
      }
      const url = await workspaceApi.getToolProxyUrl(
        sandboxId,
        toolCode,
        '',
        appConfig.sandboxToolOrigin
      )
      if (active) setSrc(url)
    })().catch((error) => {
      if (active) setLoadError(userFacingErrorMessage(error, '这个工具暂时打不开,请稍后重试。'))
    })
    return () => {
      active = false
    }
  }, [loadAttempt, onReload, sandboxId, status, toolCode, workspaceApi])

  if (status !== SandboxToolStatus.READY) {
    return (
      <div className="grid h-full place-items-center px-4">
        <ToolReadinessNotice status={status} onReload={onReload} />
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 px-4 text-center">
        <p role="alert" className="text-sm text-on-dark-danger">
          {loadError}
        </p>
        <Button
          variant="on-dark"
          size="sm"
          leftIcon={RefreshCw}
          onClick={() => setLoadAttempt((current) => current + 1)}
        >
          重新打开工具
        </Button>
      </div>
    )
  }
  if (!src) {
    return (
      <div className="grid h-full place-items-center px-4 text-sm text-on-dark-sub">
        工具正在准备...
      </div>
    )
  }
  return (
    <iframe
      src={src}
      title={`${toolCode} 工具`}
      sandbox="allow-scripts allow-same-origin allow-forms allow-downloads allow-modals"
      referrerPolicy="no-referrer"
      className="h-full w-full border-0 bg-dark-surface"
    />
  )
}

/** ToolReadinessNotice 统一呈现网页和命令工具的准备、失败与状态重读入口。 */
function ToolReadinessNotice({
  status,
  onReload,
}: {
  status: SandboxToolStatus | undefined
  onReload: () => void
}) {
  if (status === SandboxToolStatus.STARTING) {
    return (
      <p role="status" className="text-sm text-on-dark-sub">
        工具正在准备,请稍候。
      </p>
    )
  }
  return (
    <div className="flex flex-col items-start gap-3">
      <p role="alert" className="text-sm text-on-dark-danger">
        {status === SandboxToolStatus.FAILED
          ? '工具没有准备成功,请重新读取状态。'
          : '当前工具暂时无法使用,请重新读取状态。'}
      </p>
      <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={onReload}>
        重新读取状态
      </Button>
    </div>
  )
}

/* ---------------------------------------------------------------- 工具函数 */

/** languageFor 按扩展名给出编辑器语言;未登记的按纯文本。 */
function languageFor(relativePath: string | undefined): string {
  if (!relativePath) return 'plaintext'
  const extension = relativePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_BY_EXTENSION[extension] ?? 'plaintext'
}

/** parentPath 返回上一层目录路径;已在根则仍是根。 */
function parentPath(path: string): string {
  const trimmed = path.replace(/\/+$/, '')
  const index = trimmed.lastIndexOf('/')
  return index <= 0 ? '.' : trimmed.slice(0, index)
}

/** socketStatusText 把连接状态换成用户向说明。 */
function socketStatusText(status: ReturnType<typeof useTicketedWebSocket>['status']): string {
  switch (status) {
    case 'connecting':
      return '正在连接终端…'
    case 'open':
      return '终端已连接'
    case 'closed':
      return '终端连接已断开'
    case 'error':
      return '终端连接出错'
    default:
      return '终端未连接'
  }
}

/**
 * parseProgress 解析一条进度推送。
 * 推送经 M10 统一通道下发,可能带信封;这里按需要的四个字段宽松取值,
 * 形状不符就当没收到 —— 一条读不懂的推送不该让整个工作台失效。
 */
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

/** isSandboxPhase 校验进度消息中的阶段属于公开封闭枚举。 */
function isSandboxPhase(value: unknown): value is SandboxPhase {
  return typeof value === 'number' && (SANDBOX_PHASES as readonly number[]).includes(value)
}

/** isSandboxStatus 校验进度消息中的状态属于公开封闭枚举。 */
function isSandboxStatus(value: unknown): value is SandboxStatus {
  return typeof value === 'number' && (SANDBOX_STATUSES as readonly number[]).includes(value)
}
