// 题目版本与复用弹层(题库内容页内)。
//
// 三件事:看某个编号下的全部版本、基于已有版本建新版本、把题目克隆成新编号。
// 版本是内容复现的基础:作业与实验按 code + version 锁定引用,
// 所以「改题」的正确做法是建新版本,而不是改已发布版本。

import { useCallback, useMemo, useState } from 'react'
import { Copy, GitBranch, History } from 'lucide-react'
import { ContentStatus, type ContentItem } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  SegmentedControl,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  contentAuthorTypeLabel,
  contentStatusLabel,
  contentStatusTone,
  contentVisibilityLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

export interface ContentVersionsModalProps {
  item: ContentItem
  onClose: () => void
  onChanged: () => void
}

/**
 * ContentVersionsModal 展示版本清单并承载建新版本与克隆。
 */
export function ContentVersionsModal({ item, onClose, onChanged }: ContentVersionsModalProps) {
  const [mode, setMode] = useState<'versions' | 'new-version' | 'clone'>('versions')

  const versions = useAsyncResource(() => api.content.getVersions(item.code), [item.code])

  const columns: TableColumn<ContentItem>[] = [
    {
      key: 'version',
      header: '版本',
      mono: true,
      render: (version) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-medium text-ink">{version.version}</span>
          {version.version === item.version ? <Badge tone="jade">当前查看</Badge> : null}
        </div>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (version) => (
        <StatusIndicator
          tone={contentStatusTone(version.status)}
          label={contentStatusLabel(version.status)}
        />
      ),
    },
    {
      key: 'visibility',
      header: '可见范围',
      render: (version) => (
        <span className="text-ink-sub">{contentVisibilityLabel(version.visibility)}</span>
      ),
    },
    {
      key: 'author_type',
      header: '来源',
      render: (version) => (
        <span className="text-ink-sub">{contentAuthorTypeLabel(version.author_type)}</span>
      ),
    },
    { key: 'usage_count', header: '被引用', align: 'right', mono: true },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (version) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(version.updated_at)}
        </span>
      ),
    },
  ]

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>版本与复用</ModalTitle>
          <ModalDescription>
            {item.title} · 编号 {item.code}
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <SegmentedControl
            aria-label="选择要做的操作"
            options={[
              { value: 'versions', label: '版本清单', icon: History },
              { value: 'new-version', label: '建新版本', icon: GitBranch },
              { value: 'clone', label: '克隆为新题', icon: Copy },
            ]}
            value={mode}
            onValueChange={(value) => setMode(value as 'versions' | 'new-version' | 'clone')}
          />

          {mode === 'versions' ? (
            <ResourceState
              resource={versions}
              emptyIcon={History}
              emptyTitle="暂无版本记录"
              emptyDescription="这道题目只有当前版本。"
              skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
            >
              {(list) => <Table columns={columns} data={list} rowKey={(version) => version.id} />}
            </ResourceState>
          ) : null}

          {mode === 'new-version' ? (
            <NewVersionForm
              item={item}
              versions={versions.data ?? []}
              onDone={() => {
                versions.reload()
                onChanged()
              }}
            />
          ) : null}

          {mode === 'clone' ? <CloneForm item={item} onDone={onChanged} /> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface NewVersionFormProps {
  item: ContentItem
  versions: ContentItem[]
  onDone: () => void
}

/**
 * NewVersionForm 基于已有版本创建新版本。
 * 新版本复制源版本正文并落草稿态,可以放心修改而不影响已发布版本的引用。
 */
function NewVersionForm({ item, versions, onDone }: NewVersionFormProps) {
  const [sourceVersion, setSourceVersion] = useState(item.version)
  const [newVersion, setNewVersion] = useState(suggestNextVersion(item.version))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const existingVersions = useMemo(() => new Set(versions.map((one) => one.version)), [versions])

  const submit = useCallback(async () => {
    const target = newVersion.trim()
    if (!/^\d+\.\d+\.\d+$/.test(target)) {
      setFormError('版本号形如 1.0.1')
      return
    }
    if (existingVersions.has(target)) {
      setFormError('这个版本号已存在,请换一个')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.content.createNewVersion(item.code, {
        source_version: sourceVersion,
        new_version: target,
      })
      toast.success(`已创建版本 ${target}`)
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '创建新版本失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [existingVersions, item.code, newVersion, onDone, sourceVersion])

  return (
    <div className="flex flex-col gap-4">
      <Callout tone="info" title="为什么要建新版本">
        已发布版本被作业或实验引用后不应再改 —— 改了会让学生看到的题目与批改依据不一致。
        建新版本可以放心修改,旧版本继续为历史引用服务。
      </Callout>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="基于哪个版本" htmlFor="new-version-source" required>
          <Input
            id="new-version-source"
            value={sourceVersion}
            onChange={(event) => setSourceVersion(event.target.value)}
          />
        </FormField>
        <FormField
          label="新版本号"
          htmlFor="new-version-target"
          required
          error={formError}
          helper="形如 1.0.1"
        >
          <Input
            id="new-version-target"
            value={newVersion}
            invalid={Boolean(formError)}
            onChange={(event) => setNewVersion(event.target.value)}
          />
        </FormField>
      </div>

      <Button variant="primary" leftIcon={GitBranch} loading={working} onClick={() => void submit()}>
        创建新版本
      </Button>
    </div>
  )
}

interface CloneFormProps {
  item: ContentItem
  onDone: () => void
}

/**
 * CloneForm 把题目克隆成新编号。
 * 克隆用于「以这道题为模板做一道新题」;跨校复用共享库题目也走克隆,
 * 克隆结果是本校独立草稿,不再与源题共享版本历史。
 */
function CloneForm({ item, onDone }: CloneFormProps) {
  const [newCode, setNewCode] = useState(`${item.code}-copy`)
  const [newVersion, setNewVersion] = useState('1.0.0')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    const code = newCode.trim()
    if (!/^[a-z0-9-]{3,}$/.test(code)) {
      setFormError('编号用小写字母、数字与连字符,至少 3 位')
      return
    }
    if (!/^\d+\.\d+\.\d+$/.test(newVersion.trim())) {
      setFormError('版本号形如 1.0.0')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.content.cloneItem(item.code, item.version, {
        new_code: code,
        new_version: newVersion.trim(),
      })
      toast.success('已克隆为新题目草稿')
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '克隆失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [item.code, item.version, newCode, newVersion, onDone])

  return (
    <div className="flex flex-col gap-4">
      <Callout tone="info" title="克隆与新版本的区别">
        建新版本是同一道题的迭代,共享编号与版本历史。克隆是另一道题,有独立编号,
        适合「参考这道题出一道新题」或复用其他学校共享库里的题目。
      </Callout>

      <div className="grid gap-4 sm:grid-cols-2">
        <FormField label="新题目编号" htmlFor="clone-code" required error={formError}>
          <Input
            id="clone-code"
            value={newCode}
            invalid={Boolean(formError)}
            onChange={(event) => setNewCode(event.target.value)}
          />
        </FormField>
        <FormField label="新题目版本" htmlFor="clone-version" required>
          <Input
            id="clone-version"
            value={newVersion}
            onChange={(event) => setNewVersion(event.target.value)}
          />
        </FormField>
      </div>

      {item.status !== ContentStatus.PUBLISHED ? (
        <Callout tone="warning">
          源题目还是草稿。克隆草稿会得到一份同样未完成的副本,建议先完成源题目。
        </Callout>
      ) : null}

      <Button variant="primary" leftIcon={Copy} loading={working} onClick={() => void submit()}>
        克隆为新题目
      </Button>
    </div>
  )
}

/**
 * suggestNextVersion 按语义化版本推荐下一个修订号。
 * 只推荐修订位加一:大版本变更由出题人自己决定,工具不替他判断改动幅度。
 */
function suggestNextVersion(version: string): string {
  const parts = version.split('.')
  if (parts.length !== 3) return '1.0.1'
  const patch = Number(parts[2])
  if (!Number.isFinite(patch)) return '1.0.1'
  return `${parts[0]}.${parts[1]}.${patch + 1}`
}
