// 共享资源库页(教师侧栏,/teacher/shared)。设计 §7.4:
// 标签「题目 / 课程」+ 搜索与筛选(都写进地址);卡片网格;整卡打开预览抽屉(题目显示题面,课程显示简介),
// 抽屉底部「克隆到本校」;克隆成功后在抽屉里给「去编辑」链接,留在资源库里可以继续挑。
// 「克隆得到的是独立副本」的说明写在克隆确认弹窗里,页面上不常驻。
//
// 克隆是唯一的复用方式:跨校不能直接引用他校题目(否则他校改题会影响本校作业),
// 也不能读他校题目的答案 —— 题面接口已剥离答案与判题配置;预览只显示登记过的题面字段(contentFace)。

import { useCallback, useState } from 'react'
import { Link } from 'react-router'
import { Copy, Share2 } from 'lucide-react'
import { ContentType, TENANT_MODULE, type ContentItem } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  FormField,
  Icon,
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
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useSession } from '../../../../components/RoleGuard/session'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { contentAuthorTypeLabel, contentDifficultyLabel, contentTypeLabel } from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { SharedCourseLibrary } from '../../../teaching/components/SharedCourseLibrary'
import { contentFaceSections } from '../../contentFace'
import { CONTENT_TYPES } from '../../options'

/** 地址参数:tab 标签,type 题型(只对题目),q 搜索;页码在 page。切换标签时搜索与筛选一并清空 */
const SHARED_QUERY = {
  tab: { defaultValue: 'questions', allowed: ['questions', 'courses'] },
  type: { defaultValue: 'all', allowed: ['all', ...CONTENT_TYPES.map(String)] },
  q: { defaultValue: '' },
} as const

/** 每页卡片数 */
const CARD_PAGE_SIZE = 12

/**
 * TeacherSharedLibraryPage 浏览共享资源库并克隆;学校没开放教学功能时只有题目。
 */
export default function TeacherSharedLibraryPage() {
  const { me } = useSession()
  const coursesEnabled = me.enabled_modules.includes(TENANT_MODULE.TEACHING)
  const [query, setQuery] = useUrlState(SHARED_QUERY)
  const tab = coursesEnabled ? query.tab : 'questions'

  return (
    <PageScaffold>
      <PageHeader title="共享资源库" />
      <Tabs value={tab} onValueChange={(value) => setQuery({ tab: value, type: 'all', q: '' })}>
        {coursesEnabled ? (
          <TabsList aria-label="共享资源类型">
            <TabsTrigger value="questions">题目</TabsTrigger>
            <TabsTrigger value="courses">课程</TabsTrigger>
          </TabsList>
        ) : null}
        <TabsContent value="questions">
          <SharedQuestionLibrary
            type={query.type}
            keyword={query.q}
            onTypeChange={(value) => setQuery({ type: value })}
            onSearch={(value) => setQuery({ q: value })}
            onClear={() => setQuery({ type: 'all', q: '' })}
          />
        </TabsContent>
        {coursesEnabled ? (
          <TabsContent value="courses">
            <SharedCourseLibrary keyword={query.q} onSearch={(value) => setQuery({ q: value })} />
          </TabsContent>
        ) : null}
      </Tabs>
    </PageScaffold>
  )
}

interface SharedQuestionLibraryProps {
  type: string
  keyword: string
  onTypeChange: (value: string) => void
  onSearch: (value: string) => void
  onClear: () => void
}

/** SharedQuestionLibrary 共享题目:题型分段 + 搜索 + 卡片网格,整卡打开题面预览抽屉 */
function SharedQuestionLibrary({ type, keyword, onTypeChange, onSearch, onClear }: SharedQuestionLibraryProps) {
  const [previewTarget, setPreviewTarget] = useState<ContentItem>()
  const shared = usePagedResource<ContentItem>(
    (params) =>
      api.content.listShared({
        type: type === 'all' ? undefined : (Number(type) as ContentType),
        keyword: keyword || undefined,
        ...params,
      }),
    [keyword, type],
    { urlPageKey: 'page', pageSize: CARD_PAGE_SIZE },
  )
  const filtered = type !== 'all' || keyword !== ''

  return (
    <>
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按题型筛选"
          options={[
            { value: 'all', label: '全部' },
            ...CONTENT_TYPES.map((value) => ({ value: String(value), label: contentTypeLabel(value) })),
          ]}
          value={type}
          onValueChange={onTypeChange}
          className="max-w-full overflow-x-auto"
        />
        <SearchBox label="搜索共享题目" placeholder="搜索题目名" value={keyword} onSearch={onSearch} />
      </div>

      {shared.status === 'error' ? (
        <ResourceError
          message={shared.error?.message ?? '共享题目暂时读取不到'}
          traceId={shared.error?.traceId}
          onRetry={shared.reload}
        />
      ) : shared.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Share2} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有匹配的共享题目' : '共享资源库还是空的'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个条件再试,或清空筛选查看全部。' : '其他学校把题目共享到资源库后会显示在这里。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={onClear}>
              清空筛选
            </Button>
          ) : null}
        </div>
      ) : shared.status === 'loading' && !shared.data ? (
        <CardGridSkeleton label="正在读取共享题目" />
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={shared.status === 'loading' || undefined}>
            {(shared.data?.list ?? []).map((item) => (
              <li key={item.id}>
                <SharedQuestionCard item={item} onOpen={() => setPreviewTarget(item)} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={shared.page} pageSize={shared.pageSize} total={shared.total} onPageChange={shared.setPage} />
          </div>
        </>
      )}

      {previewTarget ? <SharedQuestionDrawer item={previewTarget} onClose={() => setPreviewTarget(undefined)} /> : null}
    </>
  )
}

/** SharedQuestionCard 一道共享题目:题目名(整卡按钮)、题型与难度、知识点、来源与更新时间 */
function SharedQuestionCard({ item, onOpen }: { item: ContentItem; onOpen: () => void }) {
  return (
    <Card interactive className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-3">
        <button type="button" onClick={onOpen} className="card-link line-clamp-2 text-left text-lg font-bold text-ink">
          {item.title}
        </button>
        <Badge tone="neutral" className="shrink-0">
          {contentTypeLabel(item.type)}
        </Badge>
      </div>
      <p className="text-sm text-ink-sub">{contentDifficultyLabel(item.difficulty)}</p>
      {item.knowledge_points.length > 0 ? (
        <p className="line-clamp-1 text-sm text-ink-sub">{item.knowledge_points.slice(0, 4).join(' · ')}</p>
      ) : null}
      <p className="mt-auto pt-3 text-sm text-ink-sub" title={formatDateTime(item.updated_at)}>
        {contentAuthorTypeLabel(item.author_type)} · {formatRelativeOccurrence(item.updated_at)}更新
      </p>
    </Card>
  )
}

/**
 * SharedQuestionDrawer 预览共享题目的题面,底部「克隆到本校」。
 * 走题面接口(已剥离答案、判题配置与 flag),再只显示登记过的题面字段;附件跨校不能下载,只说明数量。
 * 克隆成功后在抽屉里给「去编辑」链接(题库里按短名定位到这道题)。
 */
function SharedQuestionDrawer({ item, onClose }: { item: ContentItem; onClose: () => void }) {
  const [cloneOpen, setCloneOpen] = useState(false)
  const [cloned, setCloned] = useState<{ code: string; version: string }>()
  const face = useAsyncResource(() => api.content.getItemFace(item.code, item.version), [item.code, item.version], () => false)

  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>{item.title}</DrawerTitle>
          <DrawerDescription>
            {contentTypeLabel(item.type)} · {contentDifficultyLabel(item.difficulty)} · {contentAuthorTypeLabel(item.author_type)}。
            答案与判题配置不在共享范围内,这里只显示题面。
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-5">
          {cloned ? (
            <Callout tone="success" title="已克隆到本校题库">
              <p>
                短名 {cloned.code},版本 {cloned.version},是一份草稿。{' '}
                <Link to={`/teacher/questions?q=${encodeURIComponent(cloned.code)}`} className="font-medium text-primary underline-offset-2 hover:underline">
                  去编辑
                </Link>
              </p>
            </Callout>
          ) : null}
          <ResourceState
            resource={face}
            emptyIcon={Share2}
            emptyTitle="这道题暂时打不开"
            emptyDescription="源学校可能已取消共享或弃用了这个版本。"
            skeleton={<Skeleton variant="line" lines={8} />}
          >
            {(data) => {
              const sections = contentFaceSections(data.body)
              const attachments = Array.isArray(data.body.attachments) ? data.body.attachments.length : 0
              return (
                <>
                  {data.knowledge_points.length > 0 ? (
                    <div className="flex flex-wrap gap-1.5">
                      {data.knowledge_points.map((point) => (
                        <Badge key={point} tone="neutral">
                          {point}
                        </Badge>
                      ))}
                    </div>
                  ) : null}
                  {sections.length === 0 ? (
                    <p className="text-sm text-ink-sub">这道题没有可以预览的题面文字。</p>
                  ) : (
                    sections.map((section) => (
                      <section key={section.key} aria-label={section.label}>
                        <h3 className="text-sm font-semibold text-ink">{section.label}</h3>
                        {section.lines.length > 1 ? (
                          <ol className="mt-1.5 list-decimal pl-5 text-sm text-ink">
                            {section.lines.map((line, index) => (
                              <li key={index} className="whitespace-pre-wrap break-words">
                                {line}
                              </li>
                            ))}
                          </ol>
                        ) : (
                          <p
                            className={
                              section.key === 'contract_source'
                                ? 'mt-1.5 overflow-x-auto whitespace-pre rounded-md bg-surface-hover p-3 font-mono text-xs text-ink'
                                : 'mt-1.5 whitespace-pre-wrap break-words text-sm text-ink'
                            }
                          >
                            {section.lines[0]}
                          </p>
                        )}
                      </section>
                    ))
                  )}
                  {attachments > 0 ? (
                    <p className="text-sm text-ink-sub">带 {attachments} 个附件,克隆到本校后可以下载。</p>
                  ) : null}
                </>
              )
            }}
          </ResourceState>
        </DrawerBody>
        <DrawerFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          <Button variant="primary" leftIcon={Copy} onClick={() => setCloneOpen(true)}>
            {cloned ? '再克隆一份' : '克隆到本校'}
          </Button>
        </DrawerFooter>
      </DrawerContent>
      {cloneOpen ? (
        <CloneSharedModal
          item={item}
          onClose={() => setCloneOpen(false)}
          onCloned={(result) => {
            setCloneOpen(false)
            setCloned(result)
          }}
        />
      ) : null}
    </Drawer>
  )
}

/** CardGridSkeleton 与真实卡片同形的骨架 */
function CardGridSkeleton({ label }: { label: string }) {
  return (
    <ul className="card-grid gap-4" aria-busy="true" aria-label={label}>
      {Array.from({ length: 3 }, (_, index) => (
        <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
          <Skeleton variant="title" width="half" />
          <Skeleton variant="line" lines={2} />
        </li>
      ))}
    </ul>
  )
}

interface CloneSharedModalProps {
  item: ContentItem
  onClose: () => void
  onCloned: (result: { code: string; version: string }) => void
}

/**
 * CloneSharedModal 把共享题目克隆到本校:填本校短名与版本号。
 * 克隆结果是本校独立草稿:后续修改不影响源题,源校改题也不影响本校。
 */
function CloneSharedModal({ item, onClose, onCloned }: CloneSharedModalProps) {
  const [newCode, setNewCode] = useState(`${item.code}-local`)
  const [newVersion, setNewVersion] = useState('1.0.0')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(() => ({
    code: /^[a-z0-9-]{3,}$/.test(newCode.trim()) ? null : '短名用小写字母、数字与连字符,至少 3 位',
    version: /^\d+\.\d+\.\d+$/.test(newVersion.trim()) ? null : '版本号形如 1.0.0',
  }))

  const submit = useCallback(async () => {
    if (working || !validate()) return
    const code = newCode.trim()
    const version = newVersion.trim()
    setFormError(undefined)
    setWorking(true)
    try {
      await api.content.cloneItem(item.code, item.version, { new_code: code, new_version: version })
      toast.success('已克隆到本校题库')
      onCloned({ code, version })
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '克隆失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [item.code, item.version, newCode, newVersion, onCloned, validate, working])

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>克隆到本校题库</ModalTitle>
          <ModalDescription>
            克隆得到的是本校的独立草稿,你可以自由修改;源学校后续改动不会影响你的副本,你的修改也不会回传给源学校。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <p className="text-base font-medium text-ink">{item.title}</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="本校题目短名" htmlFor="shared-clone-code" required error={errors.code}>
              <Input
                id="shared-clone-code"
                value={newCode}
                invalid={Boolean(errors.code)}
                onBlur={() => validate(['code'])}
                onChange={(event) => setNewCode(event.target.value)}
              />
            </FormField>
            <FormField label="版本号" htmlFor="shared-clone-version" required error={errors.version}>
              <Input
                id="shared-clone-version"
                value={newVersion}
                invalid={Boolean(errors.version)}
                onBlur={() => validate(['version'])}
                onChange={(event) => setNewVersion(event.target.value)}
              />
            </FormField>
          </div>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
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
