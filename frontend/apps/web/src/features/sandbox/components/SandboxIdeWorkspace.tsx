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
  type SandboxChainOperation,
  type SandboxFileEntry,
  type SandboxFileSaveResponse,
  type SandboxInstance,
  type SandboxToolAccess,
} from '@chaimir/api-client'
import { mountMonacoEditor, mountTerminal, type MountedEditor, type MountedTerminal } from '@chaimir/ide'
import {
  Button,
  ChainProgress,
  Input,
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
import { decodeUtf8Base64, encodeUtf8Base64 } from '../../../utils/base64'
import { sandboxChainOperationLabel, sandboxPhaseLabel } from '../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { SANDBOX_PHASES, SANDBOX_STATUSES } from '../options'
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
  /** 保存工作区成功后回调:调用方据此提交或触发判分 */
  onSaved?: (result: SandboxFileSaveResponse) => void
}

/**
 * SandboxIdeWorkspace 渲染沙箱代码环境。
 * 能力与工具入口一律读 M2 的沙箱详情,不接受调用方传入 ——
 * 上游模块(M7 实例、M8 环境)的响应里工具字段命名与 M2 不同(`code` 对 `tool_code`),
 * 更重要的是「出现什么」的权威在 M2,让调用方传等于给了绕过这层判定的口子。
 */
export function SandboxIdeWorkspace({ sandboxId, onSaved }: SandboxIdeWorkspaceProps) {
  const [instance, setInstance] = useState<SandboxInstance>()
  const [loadError, setLoadError] = useState<string>()
  const [progress, setProgress] = useState<SandboxProgress>()

  /** loadInstance 读取沙箱详情:能力声明与工具入口都以它为准。 */
  const loadInstance = useCallback(async () => {
    setLoadError(undefined)
    try {
      setInstance(await api.sandbox.getInstance(sandboxId))
    } catch (error) {
      setLoadError(userFacingErrorMessage(error, '实验环境的状态读不到,请稍后重试。'))
    }
  }, [sandboxId])

  useEffect(() => {
    void loadInstance()
  }, [loadInstance])

  // 环境还没就绪时订阅准备进度:第一次拉容器要十几秒,期间界面必须说清在做什么。
  // 就绪后断开 —— 后续状态变化(暂停/回收)由业务侧的动作驱动重读,不需要一直挂着连接。
  const notReady = instance !== undefined && instance.phase !== SandboxPhase.FULLY_READY
  const progressUrl = useMemo(
    () => (notReady ? api.sandbox.getProgressWsUrl(sandboxId) : undefined),
    [notReady, sandboxId],
  )

  const progressSocket = useTicketedWebSocket({
    url: progressUrl,
    onMessage: (data) => {
      const message = parseProgress(data)
      if (!message) return
      setProgress(message)
      // 就绪或失败都要重读实例:能力声明与工具入口只在详情里
      if (message.stage === 'ready' || message.stage === 'failed') void loadInstance()
    },
  })

  const toolAccess = instance?.tool_access ?? []

  if (loadError) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <TriangleAlert aria-hidden="true" className="size-6 text-on-dark-danger" />
        <p className="text-sm text-on-dark-sub">{loadError}</p>
        <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={() => void loadInstance()}>
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
          {progress?.message ?? '环境准备失败。退出后重新进入即可重新准备一次。'}
        </p>
        {progress?.trace_id ? (
          <p className="font-mono text-xs text-on-dark-faint">
            如需帮助,请提供编号 {progress.trace_id}
          </p>
        ) : null}
        <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={() => void loadInstance()}>
          重新读取状态
        </Button>
      </div>
    )
  }

  // 还在准备:按阶段给出链式进度,不让人对着空白等
  if (notReady) {
    const phase = progress ? progress.phase : instance.phase
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-base text-on-dark">{progress?.message ?? sandboxPhaseLabel(phase)}</p>
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
            <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={() => void loadInstance()}>
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
        : commandTools.length > 0
          ? 'commands'
          : 'web'

  return (
    <Tabs defaultValue={defaultTab} className="flex h-full min-h-0 flex-col">
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
        <TabsContent value="files" className="min-h-0 flex-1">
          <FilesPanel sandboxId={sandboxId} onSaved={onSaved} />
        </TabsContent>
      ) : null}

      {capabilities.terminal ? (
        <TabsContent value="terminal" className="min-h-0 flex-1">
          <TerminalPanel sandboxId={sandboxId} />
        </TabsContent>
      ) : null}

      {capabilities.chain_operations.length > 0 ? (
        <TabsContent value="chain" className="min-h-0 flex-1">
          <ChainPanel sandboxId={sandboxId} operations={capabilities.chain_operations} />
        </TabsContent>
      ) : null}

      {capabilities.command_tools && commandTools.length > 0 ? (
        <TabsContent value="commands" className="min-h-0 flex-1">
          <CommandPanel sandboxId={sandboxId} tools={commandTools} />
        </TabsContent>
      ) : null}

      {webTools.map((tool) => (
        <TabsContent
          key={tool.tool_code}
          value={`web:${tool.tool_code}`}
          className="min-h-0 flex-1"
        >
          <WebToolPanel sandboxId={sandboxId} toolCode={tool.tool_code} />
        </TabsContent>
      ))}
    </Tabs>
  )
}

/* ---------------------------------------------------------------- 代码面板 */

interface FilesPanelProps {
  sandboxId: string
  onSaved?: (result: SandboxFileSaveResponse) => void
}

/**
 * FilesPanel 渲染工作区目录与编辑器。
 * 目录逐层展开而不是一次拉全树:后端 listFiles 是按路径列一层,递归拉整棵树在
 * 大工程上会打出几十个请求,而学生真正会点开的往往只有一两层。
 */
function FilesPanel({ sandboxId, onSaved }: FilesPanelProps) {
  const [path, setPath] = useState('.')
  const [entries, setEntries] = useState<SandboxFileEntry[]>([])
  const [openPath, setOpenPath] = useState<string>()
  const [content, setContent] = useState('')
  const [dirty, setDirty] = useState(false)
  const [busy, setBusy] = useState(false)
  const [panelError, setPanelError] = useState<string>()

  const editorHost = useRef<HTMLDivElement>(null)
  const editorRef = useRef<MountedEditor | undefined>(undefined)
  // 编辑内容放进 ref:Monaco 的 onChange 回调在装配时固定,不能依赖闭包里的 state
  const contentRef = useRef('')
  contentRef.current = content

  const language = useMemo(() => languageFor(openPath), [openPath])

  /** listDirectory 列出一层目录。 */
  const listDirectory = useCallback(
    async (target: string) => {
      setPanelError(undefined)
      try {
        const result = await api.sandbox.listFiles(sandboxId, target)
        setPath(result.relative_path)
        setEntries(result.entries)
      } catch (error) {
        setPanelError(userFacingErrorMessage(error, '这个目录暂时打不开,请稍后重试。'))
      }
    },
    [sandboxId],
  )

  useEffect(() => {
    void listDirectory('.')
  }, [listDirectory])

  /** openFile 读一个文件并载入编辑器。 */
  const openFile = useCallback(
    async (relativePath: string) => {
      setPanelError(undefined)
      try {
        const file = await api.sandbox.readFile(sandboxId, relativePath)
        setOpenPath(file.relative_path)
        setContent(decodeUtf8Base64(file.content_base64))
        setDirty(false)
      } catch (error) {
        setPanelError(userFacingErrorMessage(error, '这个文件暂时读不出来,请稍后重试。'))
      }
    },
    [sandboxId],
  )

  // 编辑器与「当前打开的文件」同生共死:换文件即重新装配,拿到的 language 才是对的
  useEffect(() => {
    const host = editorHost.current
    if (!host || openPath === undefined) return
    let disposed = false
    let mounted: MountedEditor | undefined

    void mountMonacoEditor(host, {
      value: contentRef.current,
      language,
      onChange: (value) => {
        setContent(value)
        setDirty(true)
      },
    }).then((editor) => {
      if (disposed) {
        editor.dispose()
        return
      }
      mounted = editor
      editorRef.current = editor
    })

    return () => {
      disposed = true
      mounted?.dispose()
      editorRef.current = undefined
    }
    // language 随 openPath 推导,不单独列进依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openPath])

  /** writeCurrentFile 把编辑内容写回工作区(还没落对象存储)。 */
  const writeCurrentFile = useCallback(async () => {
    if (!openPath) return
    setBusy(true)
    setPanelError(undefined)
    try {
      await api.sandbox.writeFile(sandboxId, {
        relative_path: openPath,
        content_base64: encodeUtf8Base64(contentRef.current),
      })
      setDirty(false)
      toast.success('已写入工作区')
    } catch (error) {
      setPanelError(userFacingErrorMessage(error, '没能写入工作区,请稍后重试。'))
    } finally {
      setBusy(false)
    }
  }, [openPath, sandboxId])

  /**
   * persistWorkspace 把整个工作区持久化并交出代码引用。
   * 判分与提交都要这个引用:它是「这一刻的代码」的不可变快照,
   * 比让后端再去容器里抓一次可靠(容器随时可能被回收)。
   */
  const persistWorkspace = useCallback(async () => {
    setBusy(true)
    setPanelError(undefined)
    try {
      if (openPath && dirty) {
        await api.sandbox.writeFile(sandboxId, {
          relative_path: openPath,
          content_base64: encodeUtf8Base64(contentRef.current),
        })
        setDirty(false)
      }
      const result = await api.sandbox.saveFiles(sandboxId)
      toast.success('工作区已保存')
      onSaved?.(result)
    } catch (error) {
      setPanelError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
    } finally {
      setBusy(false)
    }
  }, [dirty, onSaved, openPath, sandboxId])

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
        <ul className="min-h-0 flex-1 overflow-y-auto px-1 pb-2">
          {entries.length === 0 ? (
            <li className="px-2 py-1 text-xs text-on-dark-sub">这个目录是空的</li>
          ) : (
            entries.map((entry) => (
              <li key={entry.relative_path}>
                <button
                  type="button"
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
          <div className="flex shrink-0 items-center gap-2">
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
          <p className="shrink-0 px-3 py-2 text-xs text-on-dark-danger">{panelError}</p>
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
    </div>
  )
}

/* ---------------------------------------------------------------- 终端面板 */

/**
 * TerminalPanel 把 xterm 接到沙箱终端。
 * 终端是双向流:xterm 的输入直接送 WS,服务端输出写回 xterm ——
 * 中间不做行缓冲或命令解析,那会让 vim 这类交互程序不可用。
 */
function TerminalPanel({ sandboxId }: { sandboxId: string }) {
  const host = useRef<HTMLDivElement>(null)
  const terminalRef = useRef<MountedTerminal | undefined>(undefined)
  const [ready, setReady] = useState(false)

  const url = useMemo(() => api.sandbox.getTerminalWsUrl(sandboxId), [sandboxId])

  const socket = useTicketedWebSocket({
    url,
    onMessage: (data) => terminalRef.current?.write(data),
    onOpen: () => terminalRef.current?.focus(),
  })

  // xterm 与本面板同生共死;输入转发经 socket.send,不在此持有任何凭据
  useEffect(() => {
    const container = host.current
    if (!container) return
    let disposed = false
    let mounted: MountedTerminal | undefined

    void mountTerminal(container, {
      onData: (data) => socket.send(data),
    }).then((terminal) => {
      if (disposed) {
        terminal.dispose()
        return
      }
      mounted = terminal
      terminalRef.current = terminal
      setReady(true)
    })

    return () => {
      disposed = true
      mounted?.dispose()
      terminalRef.current = undefined
      setReady(false)
    }
    // socket.send 引用稳定(useCallback 空依赖),不列进依赖以免重建终端
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sandboxId])

  // 终端尺寸随面板变化:切标签页回来时容器宽高才确定,需要重新适配
  useEffect(() => {
    if (!ready) return
    const timer = setTimeout(() => terminalRef.current?.resize(), 50)
    return () => clearTimeout(timer)
  }, [ready])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-dark-line px-3 py-2">
        <span className="text-xs text-on-dark-sub">{socketStatusText(socket.status)}</span>
        <div className="flex items-center gap-2">
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
      {socket.error ? (
        <p className="shrink-0 px-3 py-2 text-xs text-on-dark-danger">{socket.error}</p>
      ) : null}
      <div ref={host} className="min-h-0 flex-1 bg-terminal" />
    </div>
  )
}

/* ---------------------------------------------------------------- 链操作面板 */

interface ChainPanelProps {
  sandboxId: string
  operations: SandboxChainOperation[]
}

/**
 * ChainPanel 调用运行时统一的链能力。
 * 三种操作的入参形状不同:部署与交易吃一个由运行时定义的参数对象(键不可枚举,
 * 故用文档编辑器并在本地校验合法性),查询只要一个目标标识。响应原样呈现 ——
 * 它是链上事实,不该被前端改写。
 */
function ChainPanel({ sandboxId, operations }: ChainPanelProps) {
  const [operation, setOperation] = useState<SandboxChainOperation>(operations[0])
  const [payloadText, setPayloadText] = useState('{\n  \n}')
  const [target, setTarget] = useState('')
  const [result, setResult] = useState<string>()
  const [panelError, setPanelError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const run = useCallback(async () => {
    setBusy(true)
    setPanelError(undefined)
    setResult(undefined)
    try {
      if (operation === 'query') {
        if (target.trim() === '') {
          setPanelError('请填写要查询的目标,例如合约地址或状态键。')
          return
        }
        const response = await api.sandbox.chainQuery(sandboxId, target.trim())
        setResult(JSON.stringify(response, null, 2))
        return
      }

      let payload: Record<string, unknown>
      try {
        const parsed: unknown = JSON.parse(payloadText)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setPanelError('内容格式不正确,最外层应当是大括号包裹的内容。')
          return
        }
        payload = parsed as Record<string, unknown>
      } catch {
        setPanelError('内容格式不正确,请检查是否漏了逗号或引号。')
        return
      }

      const response =
        operation === 'deploy'
          ? await api.sandbox.chainDeploy(sandboxId, { payload })
          : await api.sandbox.chainSendTx(sandboxId, { payload })
      setResult(JSON.stringify(response, null, 2))
    } catch (error) {
      setPanelError(userFacingErrorMessage(error, '这次链操作没有成功,请检查参数后重试。'))
    } finally {
      setBusy(false)
    }
  }, [operation, payloadText, sandboxId, target])

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      <div className="flex flex-wrap items-center gap-2">
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

      {operation === 'query' ? (
        <label className="flex flex-col gap-1">
          <span className="text-xs text-on-dark-sub">查询目标</span>
          <Input
            variant="underline"
            value={target}
            placeholder="合约地址或状态键"
            className="font-mono text-sm"
            onChange={(event) => setTarget(event.target.value)}
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
            onChange={(event) => setPayloadText(event.target.value)}
          />
        </label>
      )}

      <div>
        <Button variant="primary" size="sm" leftIcon={Play} loading={busy} onClick={() => void run()}>
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
  tools: SandboxToolAccess[]
}

/**
 * CommandPanel 执行受控命令工具。
 * 允许哪些命令由平台在工具定义里配白名单,学生侧读不到那份清单(工具定义接口在平台组),
 * 所以这里不做本地过滤 —— 白名单在服务端强制,越界命令会被拒绝并给出用户向说明。
 */
function CommandPanel({ sandboxId, tools }: CommandPanelProps) {
  const [toolCode, setToolCode] = useState(tools[0].tool_code)
  const [commandLine, setCommandLine] = useState('')
  const [output, setOutput] = useState<string>()
  const [panelError, setPanelError] = useState<string>()
  const [busy, setBusy] = useState(false)

  const run = useCallback(async () => {
    const command = commandLine
      .split(/\s+/)
      .map((item) => item.trim())
      .filter((item) => item !== '')
    if (command.length === 0) {
      setPanelError('请输入要执行的命令。')
      return
    }
    setBusy(true)
    setPanelError(undefined)
    setOutput(undefined)
    try {
      const response = await api.sandbox.runCommandTool(sandboxId, toolCode, { command })
      const stdout = decodeUtf8Base64(response.stdout_base64)
      const stderr = decodeUtf8Base64(response.stderr_base64)
      setOutput(
        [
          stdout,
          stderr ? `\n${stderr}` : '',
          `\n[结束状态 ${response.exit_code}]`,
        ].join(''),
      )
    } catch (error) {
      setPanelError(
        userFacingErrorMessage(error, '这条命令没能执行。它可能不在这个工具允许的范围内。'),
      )
    } finally {
      setBusy(false)
    }
  }, [commandLine, sandboxId, toolCode])

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-y-auto p-3">
      {tools.length > 1 ? (
        <div className="flex flex-wrap items-center gap-2">
          {tools.map((tool) => (
            <Button
              key={tool.tool_code}
              variant={tool.tool_code === toolCode ? 'primary' : 'on-dark'}
              size="sm"
              onClick={() => setToolCode(tool.tool_code)}
            >
              {tool.tool_code}
            </Button>
          ))}
        </div>
      ) : null}

      <label className="flex flex-col gap-1">
        <span className="text-xs text-on-dark-sub">命令与参数(按空格分隔)</span>
        <Input
          variant="underline"
          value={commandLine}
          placeholder="forge test"
          className="font-mono text-sm"
          onChange={(event) => setCommandLine(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !busy) void run()
          }}
        />
      </label>

      <div>
        <Button variant="primary" size="sm" leftIcon={Play} loading={busy} onClick={() => void run()}>
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
function WebToolPanel({ sandboxId, toolCode }: { sandboxId: string; toolCode: string }) {
  const [src, setSrc] = useState<string>()
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let active = true
    setSrc(undefined)
    setLoadError('')
    void api.sandbox
      .getToolProxyUrl(sandboxId, toolCode, '', appConfig.sandboxToolOrigin)
      .then((url) => {
        if (active) setSrc(url)
      })
      .catch((error) => {
        if (active) setLoadError(userFacingErrorMessage(error, '这个工具暂时打不开,请稍后重试。'))
      })
    return () => {
      active = false
    }
  }, [sandboxId, toolCode])

  if (loadError) {
    return <div className="grid h-full place-items-center px-4 text-sm text-on-dark-danger">{loadError}</div>
  }
  if (!src) {
    return <div className="grid h-full place-items-center px-4 text-sm text-on-dark-sub">工具正在准备...</div>
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
