// 题库内容页(教师侧栏,/teacher/questions)。设计 §7.4:
// 页头 +「新建题目」;标签「题目 / 分类」写进地址。题目标签:工具条(题型、状态、分类、共享、搜索,都写进地址)
// + 表格(题目名、题型、当前版本、状态、共享、更新时间),整行打开题目编辑大抽屉;行内更多菜单按状态
// 只显示可用项(发布 / 弃用 / 共享或取消 / 版本历史 / 删除)。分类标签:分类树的增删改。
//
// 已发布版本不可变(后端只改草稿):点开已发布或已弃用的题目时先问一句,确认后建出下一个版本的草稿再打开编辑;
// 有人引用时说明「修改会生成新版本,已布置的作业和实验不受影响」。只有作者本人能改、能建新版本。
// 答案与判题配置属敏感字段:列表与题面视角都不展示。

import { useCallback, useMemo, useState } from 'react'
import { Database, GitBranch, History, MoreVertical, Plus, Send, Share2, Trash2, Undo2 } from 'lucide-react'
import {
  ContentStatus,
  ContentType,
  ContentVisibility,
  type ContentItem,
} from '@chaimir/api-client'
import {
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
  Pagination,
  Select,
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
import { useSession } from '../../../../components/RoleGuard/session'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { contentStatusLabel, contentTypeLabel } from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { nextFreeVersion } from '../../contentVersion'
import { CONTENT_TYPES } from '../../options'
import { contentStatusTone } from '../../statusPresentation'
import { ContentItemFormModal } from './content-form'
import { ContentCategories } from './content-categories'
import { ContentVersionsModal } from './content-versions'

/** 地址参数:tab 标签;type 题型,status 状态,category 分类,shared 共享,q 搜索;页码在 page */
const QUESTIONS_QUERY = {
  tab: { defaultValue: 'items', allowed: ['items', 'categories'] },
  type: { defaultValue: 'all', allowed: ['all', ...CONTENT_TYPES.map(String)] },
  status: { defaultValue: 'all', allowed: ['all', 'draft', 'published', 'deprecated'] },
  category: { defaultValue: '' },
  shared: { defaultValue: 'all', allowed: ['all', 'shared', 'local'] },
  q: { defaultValue: '' },
} as const

const STATUS_OPTIONS = [
  { value: 'all', label: '全部状态' },
  { value: 'draft', label: '草稿', status: ContentStatus.DRAFT },
  { value: 'published', label: '已发布', status: ContentStatus.PUBLISHED },
  { value: 'deprecated', label: '已弃用', status: ContentStatus.DEPRECATED },
] as const

const SHARED_OPTIONS = [
  { value: 'all', label: '共享与否' },
  { value: 'shared', label: '已共享' },
  { value: 'local', label: '未共享' },
] as const

/** 分类筛选里「全部分类」的取值(下拉不接受空串作选项值) */
const ALL_CATEGORIES = 'all'

type ItemAction = 'publish' | 'deprecate' | 'delete'

/**
 * TeacherQuestionsPage 管理题库内容。
 */
export default function TeacherQuestionsPage() {
  const { me } = useSession()
  const [query, setQuery] = useUrlState(QUESTIONS_QUERY)
  const [editor, setEditor] = useState<{ item?: ContentItem; derivedFrom?: { version: string; usageCount: number } }>()
  const [reviseTarget, setReviseTarget] = useState<ContentItem>()
  const [versionsTarget, setVersionsTarget] = useState<ContentItem>()
  const [confirm, setConfirm] = useState<{ action: ItemAction; item: ContentItem }>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const categories = useAsyncResource(() => api.content.listCategories(), [], () => false)
  const categoryName = useMemo(
    () => new Map((categories.data ?? []).map((category) => [category.id, category.name])),
    [categories.data],
  )

  const statusFilter = STATUS_OPTIONS.find((option) => option.value === query.status)
  const items = usePagedResource<ContentItem>(
    (params) =>
      api.content.getItems({
        type: query.type === 'all' ? undefined : (Number(query.type) as ContentType),
        status: statusFilter && 'status' in statusFilter ? statusFilter.status : undefined,
        category: query.category || undefined,
        // 未共享包括「仅自己可见」与「本校可见」,接口只接受单个可见范围,故「未共享」按本校可见筛
        visibility:
          query.shared === 'shared' ? ContentVisibility.SHARED : query.shared === 'local' ? ContentVisibility.TENANT : undefined,
        keyword: query.q || undefined,
        ...params,
      }),
    [query.category, query.q, query.shared, query.status, query.type],
    { urlPageKey: 'page' },
  )

  /** openItem 整行打开:自己的草稿直接编辑;自己已发布或弃用的先确认建新版本;别人的题只看版本 */
  const openItem = useCallback(
    (item: ContentItem) => {
      if (item.author_id !== me.account.id) setVersionsTarget(item)
      else if (item.status === ContentStatus.DRAFT) setEditor({ item })
      else setReviseTarget(item)
    },
    [me.account.id],
  )

  /** runAction 执行发布、弃用与删除。删除只对未被引用的草稿可用(后端也这样限制)。 */
  const runAction = useCallback(async () => {
    if (!confirm) return
    setWorking(true)
    setActionError(undefined)
    try {
      if (confirm.action === 'publish') await api.content.publishItem(confirm.item.id)
      if (confirm.action === 'deprecate') await api.content.deprecateItem(confirm.item.id)
      if (confirm.action === 'delete') await api.content.deleteItem(confirm.item.id)
      toast.success(confirm.action === 'publish' ? '题目已发布' : confirm.action === 'deprecate' ? '题目已弃用' : '题目已删除')
      setConfirm(undefined)
      items.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [confirm, items])

  /** toggleShare 切换共享到跨校资源库;只有已发布的题目能共享(后端同一口径) */
  const toggleShare = useCallback(
    async (item: ContentItem) => {
      setActionError(undefined)
      try {
        if (item.visibility === ContentVisibility.SHARED) {
          await api.content.unshareItem(item.id)
          toast.success('已取消共享')
        } else {
          await api.content.shareItem(item.id)
          toast.success('已共享到资源库,其他学校可以克隆使用')
        }
        items.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
      }
    },
    [items],
  )

  const menuFor = (item: ContentItem) => (
    <ItemActionMenu
      item={item}
      mine={item.author_id === me.account.id}
      onRequest={(action) => setConfirm({ action, item })}
      onToggleShare={() => void toggleShare(item)}
      onOpenVersions={() => setVersionsTarget(item)}
    />
  )

  const columns: TableColumn<ContentItem>[] = [
    {
      key: 'title',
      header: '题目',
      primary: true,
      render: (item) => (
        <span className="block min-w-0">
          <span className="block truncate font-medium text-ink">{item.title}</span>
          <span className="block truncate text-xs text-ink-sub">
            {item.category_id ? (categoryName.get(item.category_id) ?? '已删除的分类') : '未分类'}
          </span>
        </span>
      ),
    },
    { key: 'type', header: '题型', render: (item) => <span className="whitespace-nowrap text-ink-sub">{contentTypeLabel(item.type)}</span> },
    { key: 'version', header: '当前版本', mono: true, render: (item) => item.version },
    {
      key: 'status',
      header: '状态',
      render: (item) => <StatusIndicator tone={contentStatusTone(item.status)} label={contentStatusLabel(item.status)} />,
    },
    {
      key: 'shared',
      header: '共享',
      render: (item) =>
        item.visibility === ContentVisibility.SHARED ? (
          <span className="text-primary">已共享</span>
        ) : (
          <span className="text-ink-sub">{item.visibility === ContentVisibility.PRIVATE ? '仅自己' : '本校'}</span>
        ),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (item) => (
        <span title={formatDateTime(item.updated_at)} className="whitespace-nowrap text-ink-sub">
          {formatRelativeOccurrence(item.updated_at)}
        </span>
      ),
    },
    { key: 'actions', header: '操作', align: 'right', render: menuFor },
  ]

  const filtered =
    query.type !== 'all' || query.status !== 'all' || query.category !== '' || query.shared !== 'all' || query.q !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="题库内容"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setEditor({})}>
            新建题目
          </Button>
        }
      />

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      <Tabs value={query.tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="题库">
          <TabsTrigger value="items" count={items.total}>
            题目
          </TabsTrigger>
          <TabsTrigger value="categories" count={categories.status === 'success' ? categories.data?.length : undefined}>
            分类
          </TabsTrigger>
        </TabsList>

        <TabsContent value="items">
          <div className="mb-4 flex flex-wrap items-center gap-3">
            <Select
              aria-label="按题型筛选"
              value={query.type}
              onValueChange={(value) => setQuery({ type: value })}
              options={[
                { value: 'all', label: '全部题型' },
                ...CONTENT_TYPES.map((type) => ({ value: String(type), label: contentTypeLabel(type) })),
              ]}
              className="w-full sm:w-40"
            />
            <Select
              aria-label="按状态筛选"
              value={query.status}
              onValueChange={(value) => setQuery({ status: value })}
              options={STATUS_OPTIONS.map((option) => ({ value: option.value, label: option.label }))}
              className="w-full sm:w-36"
            />
            <Select
              aria-label="按分类筛选"
              value={query.category || ALL_CATEGORIES}
              onValueChange={(value) => setQuery({ category: value === ALL_CATEGORIES ? '' : value })}
              options={[
                { value: ALL_CATEGORIES, label: '全部分类' },
                ...(categories.data ?? []).map((category) => ({ value: category.id, label: category.name })),
              ]}
              className="w-full sm:w-40"
            />
            <Select
              aria-label="按共享筛选"
              value={query.shared}
              onValueChange={(value) => setQuery({ shared: value })}
              options={SHARED_OPTIONS.map((option) => ({ value: option.value, label: option.label }))}
              className="w-full sm:w-36"
            />
            <SearchBox
              label="搜索题目"
              placeholder="搜索题目名或短名"
              value={query.q}
              onSearch={(value) => setQuery({ q: value })}
              className="sm:ml-auto"
            />
          </div>

          {items.status === 'error' ? (
            <ResourceError
              message={items.error?.message ?? '题目暂时读取不到'}
              traceId={items.error?.traceId}
              onRetry={items.reload}
            />
          ) : items.status === 'empty' ? (
            <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
              <Database aria-hidden className="size-8 text-ink-faint" />
              <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有符合条件的题目' : '题库还是空的'}</p>
              <p className="mt-1.5 text-sm text-ink-sub">
                {filtered ? '换个条件试试。' : '新建题目并发布后,可以在作业、实验与竞赛里引用。'}
              </p>
              {filtered ? (
                <Button
                  variant="outline"
                  className="mt-5"
                  onClick={() => setQuery({ type: 'all', status: 'all', category: '', shared: 'all', q: '' })}
                >
                  清空筛选
                </Button>
              ) : (
                <Button variant="primary" leftIcon={Plus} className="mt-5" onClick={() => setEditor({})}>
                  新建题目
                </Button>
              )}
            </div>
          ) : (
            <>
              <Table
                columns={columns}
                data={items.data?.list ?? []}
                rowKey={(item) => item.id}
                loading={items.status === 'loading' && !items.data}
                onRowClick={openItem}
                // <md 换行卡(§6.4.1 规则 3):题目名一行、题型与版本一行,状态在右,更多菜单在行尾
                mobileCard={(item) => ({
                  title: item.title,
                  meta: `${contentTypeLabel(item.type)} · 版本 ${item.version}${item.visibility === ContentVisibility.SHARED ? ' · 已共享' : ''}`,
                  badge: <StatusIndicator tone={contentStatusTone(item.status)} label={contentStatusLabel(item.status)} />,
                  action: menuFor(item),
                })}
              />
              <div className="mt-4">
                <Pagination page={items.page} pageSize={items.pageSize} total={items.total} onPageChange={items.setPage} />
              </div>
            </>
          )}
        </TabsContent>

        <TabsContent value="categories">
          <ContentCategories onChanged={categories.reload} />
        </TabsContent>
      </Tabs>

      {editor ? (
        <ContentItemFormModal
          item={editor.item}
          derivedFrom={editor.derivedFrom}
          onClose={() => setEditor(undefined)}
          onSaved={() => {
            setEditor(undefined)
            items.reload()
          }}
        />
      ) : null}

      {reviseTarget ? (
        <ReviseModal
          item={reviseTarget}
          onClose={() => setReviseTarget(undefined)}
          onCreated={(draft) => {
            setReviseTarget(undefined)
            items.reload()
            setEditor({ item: draft, derivedFrom: { version: reviseTarget.version, usageCount: reviseTarget.usage_count } })
          }}
        />
      ) : null}

      {versionsTarget ? (
        <ContentVersionsModal
          item={versionsTarget}
          onClose={() => setVersionsTarget(undefined)}
          onCreated={(draft) => {
            setVersionsTarget(undefined)
            items.reload()
            setEditor({ item: draft })
          }}
        />
      ) : null}

      <Modal open={confirm !== undefined} onOpenChange={(open) => !open && !working && setConfirm(undefined)}>
        <ModalContent size="sm">
          {confirm ? (
            <>
              <ModalHeader>
                <ModalTitle>
                  {confirm.action === 'publish' ? '发布这道题?' : confirm.action === 'deprecate' ? '弃用这道题?' : '删除这道题?'}
                </ModalTitle>
                <ModalDescription>
                  {confirm.action === 'publish'
                    ? '发布后可以在作业、实验与竞赛里引用,这个版本就不能再改了;以后要改题会建出新版本。'
                    : confirm.action === 'deprecate'
                      ? '弃用后不能再被新的作业或实验引用,已经引用它的不受影响。'
                      : '删除不能撤销。只有没被引用过的草稿可以删除。'}
                </ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base font-medium text-ink">{confirm.item.title}</p>
                <p className="mt-1 text-sm text-ink-sub">版本 {confirm.item.version}</p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" disabled={working} onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button
                  variant={confirm.action === 'delete' ? 'danger' : confirm.action === 'publish' ? 'seal' : 'primary'}
                  loading={working}
                  onClick={() => void runAction()}
                >
                  {confirm.action === 'publish' ? '确认发布' : confirm.action === 'deprecate' ? '确认弃用' : '确认删除'}
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface ReviseModalProps {
  item: ContentItem
  onClose: () => void
  /** 新版本草稿已建好,调用方接着打开编辑抽屉 */
  onCreated: (draft: ContentItem) => void
}

/**
 * ReviseModal 改已发布或已弃用的题目:说明会建出下一个版本的草稿(已布置的作业和实验不受影响),
 * 确认后读全部版本挑一个没用过的版本号并创建。想自己定版本号的去「版本历史」。
 */
function ReviseModal({ item, onClose, onCreated }: ReviseModalProps) {
  const [working, setWorking] = useState(false)
  const [error, setError] = useState<string>()

  const create = useCallback(async () => {
    setWorking(true)
    setError(undefined)
    try {
      const versions = await api.content.getVersions(item.code)
      const newVersion = nextFreeVersion(item.version, versions.map((one) => one.version))
      const draft = await api.content.createNewVersion(item.code, { source_version: item.version, new_version: newVersion })
      toast.success(`已建出版本 ${newVersion} 的草稿`)
      onCreated(draft)
    } catch (createError) {
      setError(userFacingErrorMessage(createError, '新版本没有建成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [item.code, item.version, onCreated])

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>建新版本再修改</ModalTitle>
          <ModalDescription>
            「{item.title}」的版本 {item.version} 已{item.status === ContentStatus.DEPRECATED ? '弃用' : '发布'},不能直接修改。会复制出下一个版本的草稿供你修改。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-3">
          <p className="text-sm text-ink">
            {item.usage_count > 0
              ? `修改会生成新版本,已布置的作业和实验(${item.usage_count} 处)仍然使用版本 ${item.version},不受影响。`
              : `版本 ${item.version} 保持不变,新版本发布后才会被新的引用用到。`}
          </p>
          {error ? <Callout tone="danger">{error}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" leftIcon={GitBranch} loading={working} onClick={() => void create()}>
            建新版本并编辑
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface ItemActionMenuProps {
  item: ContentItem
  /** 是不是自己出的题:只有作者能发布、弃用、共享和删除(后端同一口径) */
  mine: boolean
  onRequest: (action: ItemAction) => void
  onToggleShare: () => void
  onOpenVersions: () => void
}

/**
 * ItemActionMenu 按当前状态只给可执行的动作:草稿可发布、未被引用的草稿可删除,已发布可弃用与共享,
 * 已共享可取消共享;版本历史总是可看。删除是不可逆动作,与常规项用分隔线隔开。
 */
function ItemActionMenu({ item, mine, onRequest, onToggleShare, onOpenVersions }: ItemActionMenuProps) {
  const isShared = item.visibility === ContentVisibility.SHARED
  const canPublish = mine && item.status === ContentStatus.DRAFT
  const canDeprecate = mine && item.status === ContentStatus.PUBLISHED
  const canShare = mine && (isShared || item.status === ContentStatus.PUBLISHED)
  const canDelete = mine && item.status === ContentStatus.DRAFT && item.usage_count === 0

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${item.title} 的更多操作`} />
      </MenuTrigger>
      <MenuContent align="end">
        {canPublish ? (
          <MenuItem icon={Send} onSelect={() => onRequest('publish')}>
            发布
          </MenuItem>
        ) : null}
        {canDeprecate ? (
          <MenuItem icon={Undo2} onSelect={() => onRequest('deprecate')}>
            弃用
          </MenuItem>
        ) : null}
        {canShare ? (
          <MenuItem icon={Share2} onSelect={onToggleShare}>
            {isShared ? '取消共享' : '共享到资源库'}
          </MenuItem>
        ) : null}
        <MenuItem icon={History} onSelect={onOpenVersions}>
          版本历史
        </MenuItem>
        {canDelete ? (
          <>
            <MenuSeparator />
            <MenuItem danger icon={Trash2} onSelect={() => onRequest('delete')}>
              删除
            </MenuItem>
          </>
        ) : null}
      </MenuContent>
    </Menu>
  )
}
