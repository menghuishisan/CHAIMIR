// 共享资源库页(教师侧栏,/teacher/shared)。
//
// 浏览其他学校共享出来的题目,克隆到本校后可自行修改与使用。
// 克隆是唯一的复用方式:跨租户不能直接引用他校题目(否则他校改题会影响本校作业),
// 也不能读他校题目的答案(GET full 只对本租户教师开放)。

import { useCallback, useMemo, useState } from 'react'
import { Copy, Search, Share2 } from 'lucide-react'
import { ContentType, type ContentItem } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
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
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  CONTENT_TYPES,
  contentAuthorTypeLabel,
  contentDifficultyLabel,
  contentTypeLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 类型筛选项:值为空串表示不过滤。 */
const TYPE_FILTERS = [
  { value: '', label: '全部' },
  ...CONTENT_TYPES.map((type) => ({ value: String(type), label: contentTypeLabel(type) })),
]

/**
 * TeacherSharedLibraryPage 浏览共享资源库并克隆题目。
 */
export default function TeacherSharedLibraryPage() {
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [keyword, setKeyword] = useState('')
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const [cloneTarget, setCloneTarget] = useState<ContentItem>()

  const shared = usePagedResource<ContentItem>(
    (params) =>
      api.content.listShared({
        type: typeFilter ? (Number(typeFilter) as ContentType) : undefined,
        keyword: submittedKeyword || undefined,
        ...params,
      }),
    [submittedKeyword, typeFilter],
  )

  const handleSearch = useCallback(
    (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      setSubmittedKeyword(keyword.trim())
    },
    [keyword],
  )

  const stats = useMemo(() => {
    const list = shared.data ? shared.data.list : []
    return {
      types: new Set(list.map((item) => item.type)).size,
      knowledgePoints: new Set(list.flatMap((item) => item.knowledge_points)).size,
    }
  }, [shared.data])

  const columns: TableColumn<ContentItem>[] = [
    {
      key: 'title',
      header: '题目',
      render: (item) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{item.title}</div>
          <div className="truncate font-mono text-xs text-ink-sub">
            {item.code} · {item.version}
          </div>
        </div>
      ),
    },
    {
      key: 'type',
      header: '类型',
      render: (item) => (
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="neutral">{contentTypeLabel(item.type)}</Badge>
          <Badge tone="jade">{contentDifficultyLabel(item.difficulty)}</Badge>
        </div>
      ),
    },
    {
      key: 'knowledge_points',
      header: '知识点',
      render: (item) =>
        item.knowledge_points.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {item.knowledge_points.slice(0, 3).map((point) => (
              <Badge key={point} tone="neutral">
                {point}
              </Badge>
            ))}
          </div>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
    {
      key: 'author_type',
      header: '来源',
      render: (item) => (
        <span className="text-ink-sub">{contentAuthorTypeLabel(item.author_type)}</span>
      ),
    },
    { key: 'usage_count', header: '被引用', align: 'right', mono: true },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (item) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(item.updated_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => (
        <Button variant="ghost" size="sm" leftIcon={Copy} onClick={() => setCloneTarget(item)}>
          克隆到本校
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '资源' }, { label: '共享资源库' }]} />}
        title="共享资源库"
        description="其他学校共享出来的题目。克隆到本校后成为你的独立草稿,可以自由修改。"
        icon={Share2}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="共享题目" value={shared.total} icon={Share2} />
          <Stat label="本页涵盖类型" value={stats.types} icon={Share2} />
          <Stat label="本页涵盖知识点" value={stats.knowledgePoints} icon={Search} />
        </div>
      </PageSection>

      <PageSection
        title="共享题目"
        description={`共 ${shared.total} 道题目`}
        actions={
          <div className="flex flex-wrap items-end gap-2">
            <SegmentedControl
              aria-label="按题目类型筛选"
              size="sm"
              options={TYPE_FILTERS}
              value={typeFilter}
              onValueChange={setTypeFilter}
            />
            <form onSubmit={handleSearch} className="flex items-end gap-2">
              <FormField label="按标题搜索" className="mb-0">
                <Input
                  value={keyword}
                  leftIcon={Search}
                  placeholder="输入关键词"
                  onChange={(event) => setKeyword(event.target.value)}
                />
              </FormField>
              <Button type="submit" variant="outline">
                搜索
              </Button>
            </form>
          </div>
        }
      >
        <ResourceState
          resource={shared}
          emptyIcon={Share2}
          emptyTitle={submittedKeyword || typeFilter ? '没有匹配的共享题目' : '共享资源库还是空的'}
          emptyDescription={
            submittedKeyword || typeFilter
              ? '换个条件再试,或清空筛选查看全部。'
              : '其他学校把题目共享到资源库后会显示在这里。'
          }
          emptyAction={
            submittedKeyword || typeFilter ? (
              <Button
                variant="outline"
                onClick={() => {
                  setKeyword('')
                  setSubmittedKeyword('')
                  setTypeFilter('')
                }}
              >
                清空筛选
              </Button>
            ) : undefined
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              <Pagination
                page={shared.page}
                pageSize={shared.pageSize}
                total={shared.total}
                onPageChange={shared.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>

      {cloneTarget ? (
        <CloneSharedModal
          item={cloneTarget}
          onClose={() => setCloneTarget(undefined)}
          onCloned={() => {
            setCloneTarget(undefined)
            shared.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface CloneSharedModalProps {
  item: ContentItem
  onClose: () => void
  onCloned: () => void
}

/**
 * CloneSharedModal 把共享题目克隆到本校。
 * 克隆结果是本校独立草稿:后续修改不影响源题,源校改题也不影响本校。
 */
function CloneSharedModal({ item, onClose, onCloned }: CloneSharedModalProps) {
  const [newCode, setNewCode] = useState(`${item.code}-local`)
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
      toast.success('已克隆到本校题库')
      onCloned()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '克隆失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [item.code, item.version, newCode, newVersion, onCloned])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>克隆到本校题库</ModalTitle>
          <ModalDescription>{item.title}</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <Callout tone="info" title="克隆得到的是独立副本">
            克隆后的题目属于本校,你可以自由修改。源学校后续改动不会影响你的副本,
            你的修改也不会回传给源学校。
          </Callout>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="本校题目编号" htmlFor="shared-clone-code" required error={formError}>
              <Input
                id="shared-clone-code"
                value={newCode}
                invalid={Boolean(formError)}
                onChange={(event) => setNewCode(event.target.value)}
              />
            </FormField>
            <FormField label="版本号" htmlFor="shared-clone-version" required>
              <Input
                id="shared-clone-version"
                value={newVersion}
                onChange={(event) => setNewVersion(event.target.value)}
              />
            </FormField>
          </div>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="seal" leftIcon={Copy} loading={working} onClick={() => void submit()}>
            确认克隆
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
