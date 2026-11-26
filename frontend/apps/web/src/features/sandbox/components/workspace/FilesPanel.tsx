import { useCallback, useEffect, useImperativeHandle, useRef, useState, type Ref } from 'react'
import { RefreshCw, Save } from 'lucide-react'
import type { SandboxFileEntry } from '@chaimir/api-client'
import { mountMonacoEditor, type MountedEditor } from '@chaimir/ide'
import {
  Button, Modal, ModalContent, ModalDescription, ModalFooter, ModalHeader, ModalTitle,
  toast, useMediaQuery, useWorkbenchShortcuts,
} from '@chaimir/ui'
import { decodeUtf8Base64, encodeUtf8Base64 } from '../../../../utils/base64'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import type { SandboxWorkspaceApi, SandboxWorkspaceHandle } from './types'

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

const FINE_POINTER_QUERY = '(any-pointer: fine)'

interface FilesPanelProps {
  ref?: Ref<SandboxWorkspaceHandle>
  sandboxId: string
  workspaceApi: SandboxWorkspaceApi
  available: boolean
}

// 按需读取单层目录，避免大工程递归请求整棵树。
export function FilesPanel({ ref, sandboxId, workspaceApi, available }: FilesPanelProps) {
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
  // 触屏设备只读，避免编辑器的软键盘及输入法限制损坏文件。
  const editable = useMediaQuery(FINE_POINTER_QUERY)

  const editorHost = useRef<HTMLDivElement>(null)
  const directoryRequestRef = useRef(0)
  const fileRequestRef = useRef(0)
  const activeRef = useRef(false)
  const busyRef = useRef(false)
  const dirtyRef = useRef(false)
  const editVersionRef = useRef(0)
  // 编辑版本只属于实际读入或写入的内容,不能由详情轮询或归档保存推进。
  const fileRevisionRef = useRef(0)
  // 编辑内容放进 ref:Monaco 的 onChange 回调在装配时固定,不能依赖闭包里的 state
  const contentRef = useRef('')

  // 事件门禁同步更新，不等待 React 渲染。
  const markDirty = useCallback((value: boolean) => {
    dirtyRef.current = value
    setDirty(value)
  }, [])

  const listDirectory = useCallback(
    async (target: string) => {
      if (!activeRef.current) return
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
    activeRef.current = available
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reads the external directory when access is available
    if (available) void listDirectory('.')
    return () => {
      activeRef.current = false
      // 切换或失去可操作性后，旧请求不能继续保存或覆盖编辑。
      directoryRequestRef.current += 1
      fileRequestRef.current += 1
    }
  }, [available, listDirectory])

  async function openFile(relativePath: string, discardChanges = false) {
    if (!activeRef.current || busyRef.current) return
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
      setPanelError(
        userFacingErrorMessage(error, '文件暂时无法作为文本打开，请检查内容或选择其他文件。')
      )
    }
  }

  // 每次成功读取都重建编辑器,同路径重读也必须显示本次内容与语言。
  useEffect(() => {
    const host = editorHost.current
    if (!host || openPath === undefined) return
    let disposed = false
    let mounted: MountedEditor | undefined
    setEditorError(undefined)

    void mountMonacoEditor(host, {
      value: contentRef.current,
      language: languageFor(openPath),
      readOnly: !editable,
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
    }
  }, [editable, fileReadVersion, markDirty, openPath])

  // 禁用时仍登记快捷键，拦下浏览器的「另存网页」。
  useWorkbenchShortcuts([
    {
      key: 's',
      mod: true,
      description: '写入当前文件',
      disabled: !available || !openPath || !dirty || busy,
      run: () => void persistWorkspace(false),
    },
  ])

  useImperativeHandle(ref, () => ({ save: persistWorkspace }))

  async function persistWorkspace(archive = false): Promise<boolean> {
    if (!activeRef.current || busyRef.current) return false
    if (!archive && !dirtyRef.current) return true
    busyRef.current = true
    const request = ++fileRequestRef.current
    const isCurrent = () => activeRef.current && request === fileRequestRef.current
    setBusy(true)
    setPanelError(undefined)
    try {
      if (openPath && dirtyRef.current) {
        const savedEditVersion = editVersionRef.current
        const written = await workspaceApi.writeFile(sandboxId, {
          relative_path: openPath,
          content_base64: encodeUtf8Base64(contentRef.current),
          expected_revision: fileRevisionRef.current,
        })
        if (!isCurrent()) return false
        fileRevisionRef.current = written.workspace_revision
        markDirty(editVersionRef.current !== savedEditVersion)
      }
      if (dirtyRef.current) {
        setPanelError('保存期间又有修改,请再次保存后继续。')
        return false
      }
      if (!archive) {
        toast.success('已写入工作区')
        return true
      }
      await workspaceApi.saveFiles(sandboxId)
      if (!isCurrent()) return false
      if (dirtyRef.current) {
        setPanelError('保存期间又有修改,请再次保存后继续。')
        return false
      }
      toast.success('工作区已保存')
      return true
    } catch (error) {
      if (isCurrent()) setPanelError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      return false
    } finally {
      busyRef.current = false
      setBusy(false)
    }
  }

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
              disabled={!available || !openPath || busy}
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
              disabled={!available || !openPath || !dirty || busy}
              onClick={() => void persistWorkspace(false)}
            >
              写入
            </Button>
            <Button
              variant="primary"
              size="sm"
              leftIcon={Save}
              loading={busy}
              disabled={!available}
              onClick={() => void persistWorkspace(true)}
            >
              保存工作区
            </Button>
          </div>
        </div>

        {editable ? null : (
          <p className="shrink-0 px-3 py-2 text-xs text-on-dark-sub">
            这台设备没有鼠标或触控板,代码只能查看;改代码请换到电脑上。保存工作区、终端和链操作照常可用。
          </p>
        )}

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
              {editable
                ? '选一个文件就能开始写。判分和退出前会保存修改,也可以随时手动写入。'
                : '选一个文件查看内容。'}
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

function languageFor(relativePath: string): string {
  const extension = relativePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_BY_EXTENSION[extension] ?? 'plaintext'
}

function parentPath(path: string): string {
  const trimmed = path.replace(/\/+$/, '')
  const index = trimmed.lastIndexOf('/')
  return index <= 0 ? '.' : trimmed.slice(0, index)
}
