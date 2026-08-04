// 题库内容页(教师侧栏,/teacher/questions)。
//
// 一页承载题目列表、分类管理与版本操作。题目正文按内容类型渲染显式表单字段,
// 不给裸 JSON 文本域(旧前端 16 处裸 JSON 是被审查列为 P0 的问题)。
//
// 答案与判题配置属敏感字段:创建与编辑时由 sensitive_fields 声明哪些路径要对学生剥离,
// 列表与题面视角都不展示这些内容。

import { useCallback, useMemo, useState } from 'react'
import { Database, FolderTree, MoreVertical, Pencil, Plus, Send, Share2, Trash2 } from 'lucide-react'
import {
  ContentStatus,
  ContentType,
  ContentVisibility,
  type ContentItem,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
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
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  CONTENT_TYPES,
  contentDifficultyLabel,
  contentStatusLabel,
  contentStatusTone,
  contentTypeLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContentItemFormModal } from './content-form'
import { ContentCategories } from './content-categories'
import { ContentVersionsModal } from './content-versions'

/** 类型筛选项:值为空串表示不过滤。 */
const TYPE_FILTERS = [
  { value: '', label: '全部' },
  ...CONTENT_TYPES.map((type) => ({ value: String(type), label: contentTypeLabel(type) })),
]

/**
 * TeacherQuestionsPage 管理题库内容。
 */
export default function TeacherQuestionsPage() {
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [formTarget, setFormTarget] = useState<{ item?: ContentItem } | undefined>()
  const [versionsTarget, setVersionsTarget] = useState<ContentItem>()
  const [confirm, setConfirm] = useState<{ action: 'publish' | 'deprecate' | 'delete'; item: ContentItem }>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const items = usePagedResource<ContentItem>(
    (params) =>
      api.content.getItems({
        type: typeFilter ? (Number(typeFilter) as ContentType) : undefined,
        ...params,
      }),
    [typeFilter],
  )

  /** runAction 执行发布、弃用与删除。删除只对草稿可用(后端也这样限制)。 */
  const runAction = useCallback(async () => {
    if (!confirm) return
    setWorking(true)
    setActionError(undefined)
    try {
      if (confirm.action === 'publish') await api.content.publishItem(confirm.item.id)
      if (confirm.action === 'deprecate') await api.content.deprecateItem(confirm.item.id)
      if (confirm.action === 'delete') await api.content.deleteItem(confirm.item.id)
      toast.success('操作已完成')
      setConfirm(undefined)
      items.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [confirm, items])

  /** toggleShare 切换共享到跨校资源库。 */
  const toggleShare = useCallback(
    async (item: ContentItem) => {
      setActionError(undefined)
      try {
        if (item.visibility === ContentVisibility.SHARED) {
          await api.content.unshareItem(item.id)
          toast.success('已取消共享')
        } else {
          await api.content.shareItem(item.id)
          toast.success('已共享到资源库')
        }
        items.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
      }
    },
    [items],
  )

  const stats = useMemo(() => {
    const list = items.data ? items.data.list : []
    return {
      published: list.filter((item) => item.status === ContentStatus.PUBLISHED).length,
      draft: list.filter((item) => item.status === ContentStatus.DRAFT).length,
      shared: list.filter((item) => item.visibility === ContentVisibility.SHARED).length,
    }
  }, [items.data])

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
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="neutral">{contentTypeLabel(item.type)}</Badge>
          <Badge tone="jade">{contentDifficultyLabel(item.difficulty)}</Badge>
        </div>
      ),
    },
    {
      key: 'tags',
      header: '标签',
      render: (item) =>
        item.tags.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {item.tags.slice(0, 3).map((tag) => (
              <Badge key={tag} tone="neutral">
                {tag}
              </Badge>
            ))}
            {item.tags.length > 3 ? <span className="text-xs text-ink-sub">等 {item.tags.length} 个</span> : null}
          </div>
        ) : (
          <span className="text-ink-sub">—</span>
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
      key: 'status',
      header: '状态',
      render: (item) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator tone={contentStatusTone(item.status)} label={contentStatusLabel(item.status)} />
          {item.visibility === ContentVisibility.SHARED ? <Badge tone="jade">已共享</Badge> : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => (
        <div className="flex items-center justify-end gap-1">
          <IconButton
            variant="ghost"
            size="sm"
            icon={Pencil}
            aria-label={`编辑题目 ${item.title}`}
            onClick={() => setFormTarget({ item })}
          />
          <ItemActionMenu
            item={item}
            onRequest={(action) => setConfirm({ action, item })}
            onToggleShare={() => void toggleShare(item)}
            onOpenVersions={() => setVersionsTarget(item)}
          />
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '资源' }, { label: '题库内容' }]} />}
        title="题库内容"
        description="维护实验模板、竞赛题与理论题。题目按版本锁定,已被作业或实验引用的版本不会因后续修改而变化。"
        icon={Database}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            新建题目
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="题目总数" value={items.total} icon={Database} />
          <Stat label="本页已发布" value={stats.published} icon={Send} hint="可被作业与实验引用" />
          <Stat label="本页草稿" value={stats.draft} icon={Pencil} />
          <Stat label="本页已共享" value={stats.shared} icon={Share2} hint="其他学校可复用" />
        </div>
      </PageSection>

      <Tabs defaultValue="items">
        <TabsList>
          <TabsTrigger value="items" icon={Database}>
            题目列表
          </TabsTrigger>
          <TabsTrigger value="categories" icon={FolderTree}>
            题库分类
          </TabsTrigger>
        </TabsList>

        <TabsContent value="items">
          <PageSection
            title="题目列表"
            description={`共 ${items.total} 道题目`}
            actions={
              <SegmentedControl
                aria-label="按题目类型筛选"
                size="sm"
                options={TYPE_FILTERS}
                value={typeFilter}
                onValueChange={setTypeFilter}
              />
            }
          >
            <div className="flex flex-col gap-4">
              {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

              <ResourceState
                resource={items}
                emptyIcon={Database}
                emptyTitle={typeFilter ? '这个类型下没有题目' : '题库还是空的'}
                emptyDescription={
                  typeFilter ? '换个类型看看。' : '新建题目后可以在作业、实验与竞赛里引用。'
                }
                emptyAction={
                  typeFilter ? undefined : (
                    <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                      新建题目
                    </Button>
                  )
                }
                skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
              >
                {(page) => (
                  <>
                    <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                    <Pagination
                      page={items.page}
                      pageSize={items.pageSize}
                      total={items.total}
                      onPageChange={items.setPage}
                    />
                  </>
                )}
              </ResourceState>
            </div>
          </PageSection>
        </TabsContent>

        <TabsContent value="categories">
          <ContentCategories />
        </TabsContent>
      </Tabs>

      {formTarget ? (
        <ContentItemFormModal
          item={formTarget.item}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            items.reload()
          }}
        />
      ) : null}

      {versionsTarget ? (
        <ContentVersionsModal
          item={versionsTarget}
          onClose={() => setVersionsTarget(undefined)}
          onChanged={() => {
            setVersionsTarget(undefined)
            items.reload()
          }}
        />
      ) : null}

      <Modal open={confirm !== undefined} onOpenChange={(open) => !open && setConfirm(undefined)}>
        <ModalContent size="sm">
          {confirm ? (
            <>
              <ModalHeader>
                <ModalTitle>
                  {confirm.action === 'publish'
                    ? '确认发布题目'
                    : confirm.action === 'deprecate'
                      ? '确认弃用题目'
                      : '确认删除题目'}
                </ModalTitle>
                <ModalDescription>
                  {confirm.action === 'publish'
                    ? '发布后题目可被作业、实验与竞赛引用。正文与答案不宜再改,需要修改请建新版本。'
                    : confirm.action === 'deprecate'
                      ? '弃用后不能再被新的作业或实验引用,已引用的历史版本不受影响。'
                      : '删除只对未被引用的草稿可用。删除不可撤销。'}
                </ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base text-ink">{confirm.item.title}</p>
                <p className="mt-1 font-mono text-xs text-ink-sub">
                  {confirm.item.code} · {confirm.item.version}
                </p>
                {confirm.action === 'delete' && confirm.item.usage_count > 0 ? (
                  <Callout tone="warning" className="mt-3">
                    这道题已被引用 {confirm.item.usage_count} 次,后端会拒绝删除。请改为弃用。
                  </Callout>
                ) : null}
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button
                  variant={confirm.action === 'delete' ? 'danger' : 'seal'}
                  loading={working}
                  onClick={() => void runAction()}
                >
                  确认
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface ItemActionMenuProps {
  item: ContentItem
  onRequest: (action: 'publish' | 'deprecate' | 'delete') => void
  onToggleShare: () => void
  onOpenVersions: () => void
}

/**
 * ItemActionMenu 按当前状态给出可执行动作。
 * 删除是不可逆动作,与常规项用分隔线隔开。
 */
function ItemActionMenu({ item, onRequest, onToggleShare, onOpenVersions }: ItemActionMenuProps) {
  const canPublish = item.status === ContentStatus.DRAFT
  const canDeprecate = item.status === ContentStatus.PUBLISHED
  const canDelete = item.status === ContentStatus.DRAFT
  const isShared = item.visibility === ContentVisibility.SHARED

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton
          variant="ghost"
          size="sm"
          icon={MoreVertical}
          aria-label={`${item.title} 的更多操作`}
        />
      </MenuTrigger>
      <MenuContent align="end">
        <MenuItem onSelect={onOpenVersions}>版本与复用</MenuItem>
        {canPublish ? (
          <MenuItem icon={Send} onSelect={() => onRequest('publish')}>
            发布题目
          </MenuItem>
        ) : null}
        <MenuItem icon={Share2} onSelect={onToggleShare}>
          {isShared ? '取消共享' : '共享到资源库'}
        </MenuItem>
        {canDeprecate ? <MenuItem onSelect={() => onRequest('deprecate')}>弃用题目</MenuItem> : null}
        {canDelete ? (
          <>
            <MenuSeparator />
            <MenuItem danger icon={Trash2} onSelect={() => onRequest('delete')}>
              删除题目
            </MenuItem>
          </>
        ) : null}
      </MenuContent>
    </Menu>
  )
}
