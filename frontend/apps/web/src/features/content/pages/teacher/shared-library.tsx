// 共享资源库页(教师侧栏,/teacher/shared)。
//
// 浏览其他学校共享出来的题目,看过题面后克隆到本校自行修改与使用。
// 克隆是唯一的复用方式:跨租户不能直接引用他校题目(否则他校改题会影响本校作业),
// 也不能读他校题目的答案 —— 题面接口已剥离 answer/judge_config/flag,GET full 只对本租户作者开放。

import { useCallback, useState } from 'react'
import { Copy, Eye, Search, Share2 } from 'lucide-react'
import { ContentType, type ContentItem } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  DescriptionList,
  FilterBar,
  FilterField,
  FormField,
  IconButton,
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
  Pagination,
  SegmentedControl,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useAsyncResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { CONTENT_TYPES } from '../../options'
import {
  contentAuthorTypeLabel,
  contentDifficultyLabel,
  contentTypeLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  ContentItemClassificationCell,
  ContentItemIdentityCell,
} from '../../components/ContentItemCells'

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
  const [previewTarget, setPreviewTarget] = useState<ContentItem>()
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

  // FilterBar 自己是 form 并接住回车,故这里只需要提交动作本身
  const handleSearch = useCallback(() => {
    setSubmittedKeyword(keyword.trim())
  }, [keyword])

  const columns: TableColumn<ContentItem>[] = [
    {
      key: 'title',
      header: '题目',
      render: (item) => <ContentItemIdentityCell item={item} />,
    },
    {
      key: 'type',
      header: '类型',
      render: (item) => <ContentItemClassificationCell item={item} />,
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
        <div className="flex items-center justify-end gap-1">
          <IconButton
            variant="ghost"
            size="sm"
            icon={Eye}
            aria-label={`预览题面 ${item.title}`}
            onClick={() => setPreviewTarget(item)}
          />
          <Button variant="ghost" size="sm" leftIcon={Copy} onClick={() => setCloneTarget(item)}>
            克隆到本校
          </Button>
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '资源' }]} />}
        title="共享资源库"
        description="其他学校共享出来的题目。克隆到本校后成为你的独立草稿,可以自由修改。"
        icon={Share2}
      />

      {/* 不做指标带:共享题目数由 Pagination 的「共 N 条」给出,而「涵盖类型/涵盖知识点」需要跨全库去重统计,
          服务端没有这个聚合,用当前页数出来就是错数(§6.5.4 全量口径);类型与知识点在每一行里可见。 */}
      <DataPanel
        label="共享题目"
        filter={
          <FilterBar label="共享题目筛选" onSubmit={handleSearch} submitLabel="搜索">
            <FilterField label="题目类型" group>
              <SegmentedControl
                aria-label="按题目类型筛选"
                size="sm"
                options={TYPE_FILTERS}
                value={typeFilter}
                onValueChange={setTypeFilter}
              />
            </FilterField>
            <FilterField label="标题关键词" htmlFor="shared-keyword">
              <Input
                id="shared-keyword"
                value={keyword}
                leftIcon={Search}
                placeholder="输入关键词"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={shared.page}
            pageSize={shared.pageSize}
            total={shared.total}
            onPageChange={shared.setPage}
          />
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
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):题目标题一行、类型与来源学校一行
                mobileCard={(item) => ({
                  title: item.title,
                  meta: `${contentTypeLabel(item.type)} · ${item.code} · ${item.version}`,
                })}
              />
            )}
          </ResourceState>
      </DataPanel>

      {previewTarget ? (
        <SharedFaceModal item={previewTarget} onClose={() => setPreviewTarget(undefined)} />
      ) : null}

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

interface SharedFaceModalProps {
  item: ContentItem
  onClose: () => void
}

/**
 * SharedFaceModal 预览共享题目的题面。
 *
 * 走题面接口(GET /content/items/{code}/{version}),它已按作者声明的 sensitive_fields 与平台默认
 * 敏感路径剥离答案、判题配置与 flag —— 跨校可见的只有题面。克隆前先看清内容,免得克隆一堆用不上的题。
 */
function SharedFaceModal({ item, onClose }: SharedFaceModalProps) {
  const face = useAsyncResource(
    () => api.content.getItemFace(item.code, item.version),
    [item.code, item.version],
    () => false,
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{item.title}</ModalTitle>
          <ModalDescription>
            {contentTypeLabel(item.type)} · {item.code} · {item.version}。
            答案与判题配置不在共享范围内,这里只呈现题面。
          </ModalDescription>
        </ModalHeader>
        <ResourceState
          resource={face}
          emptyIcon={Eye}
          emptyTitle="这道题暂时打不开"
          emptyDescription="源学校可能已取消共享或弃用了这个版本。"
          skeleton={
            <ModalBody>
              <Skeleton variant="line" lines={6} />
            </ModalBody>
          }
        >
          {(data) => (
            <ModalBody className="flex flex-col gap-4">
              <DescriptionList
                dense
                columns={2}
                items={[
                  { term: '难度', description: contentDifficultyLabel(data.difficulty) },
                  { term: '来源', description: contentAuthorTypeLabel(data.author_type) },
                  { term: '被引用', description: String(data.usage_count), mono: true },
                  { term: '更新时间', description: formatShortDateTime(data.updated_at), mono: true },
                ]}
              />
              {data.knowledge_points.length > 0 ? (
                <div className="flex flex-wrap gap-1">
                  {data.knowledge_points.map((point) => (
                    <Badge key={point} tone="neutral">
                      {point}
                    </Badge>
                  ))}
                </div>
              ) : null}
              {/* 正文按类型有不同字段,统一按「键 → 可读文本」平铺:共享库只做浏览,不做结构化编辑 */}
              <dl className="flex flex-col gap-3">
                {Object.entries(data.body).map(([key, value]) => (
                  <div key={key}>
                    <dt className="font-mono text-xs text-ink-sub">{key}</dt>
                    <dd className="mt-0.5 whitespace-pre-wrap break-words text-sm text-ink">
                      {renderBodyValue(value)}
                    </dd>
                  </div>
                ))}
              </dl>
            </ModalBody>
          )}
        </ResourceState>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** renderBodyValue 把正文字段值转成可读文本:数组按行、对象按 JSON、其余原样。 */
function renderBodyValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (Array.isArray(value)) return value.map((item) => (typeof item === 'string' ? item : JSON.stringify(item))).join('\n')
  if (value === null || value === undefined) return '—'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  return String(value)
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
      setFormError('短名用小写字母、数字与连字符,至少 3 位')
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
            <FormField label="本校题目短名" htmlFor="shared-clone-code" required error={formError}>
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
          <Button variant="primary" leftIcon={Copy} loading={working} onClick={() => void submit()}>
            确认克隆
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
