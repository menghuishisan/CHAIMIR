// 试卷组卷页(教师侧栏,/teacher/exams)。设计 §7.4:
// 页头 +「新建试卷」;卡片网格,卡片显示试卷名、组卷方式(手动选题 / 按条件抽题)、创建与更新时间
// (题数和总分在详情抽屉里读取,列表里没有);整卡打开试卷详情抽屉(题目列表,「重新抽题」只对按条件抽题的试卷显示)。
// 新建试卷在内容区内切换为表单视图,选题嵌入该视图(沿用规范现有定案)。
//
// 两种组卷方式:手动选题(逐题选定并给分)与按条件抽题(设定类型、难度、知识点与题数,
// 由后端随机抽取)。按条件抽题可以重新抽取,手动选题不需要 —— 重抽会打乱教师的选择。
//
// 抽题条件按结构化字段渲染,不给裸 JSON(旧前端组卷条件是裸 JSON 文本域)。

import { useCallback, useState } from 'react'
import { FileText, Plus, RefreshCw } from 'lucide-react'
import {
  ContentDifficulty,
  ContentType,
  PaperMode,
  type Paper,
  type PaperCriteria,
  type PaperItemInput,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  Checkbox,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  Empty,
  FormField,
  Icon,
  Input,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { facetCount } from '../../../../utils/facets'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { CONTENT_DIFFICULTIES, CONTENT_TYPES } from '../../options'
import {
  contentDifficultyLabel,
  contentTypeLabel,
  paperModeLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContentItemPicker } from '../../components/ContentItemPicker'

/** 每页卡片数 */
const PAPER_PAGE_SIZE = 12

/**
 * TeacherExamsPage 管理试卷:卡片网格 + 详情抽屉;新建时切换到表单视图。
 */
export default function TeacherExamsPage() {
  const [createOpen, setCreateOpen] = useState(false)
  const [detailTarget, setDetailTarget] = useState<Paper>()

  const papers = usePagedResource<Paper>((params) => api.content.listPapers(params), [], {
    urlPageKey: 'page',
    pageSize: PAPER_PAGE_SIZE,
  })

  // 组卷方式分布取后端 facets.gen_mode:全量分组计数,不用当前页切片去数
  const facets = papers.total === undefined ? undefined : papers.data?.facets
  const manualCount = facetCount(facets, 'gen_mode', PaperMode.MANUAL)
  const randomCount = facetCount(facets, 'gen_mode', PaperMode.RANDOM)

  if (createOpen) {
    return (
      <PaperFormPage
        onClose={() => setCreateOpen(false)}
        onSaved={() => {
          setCreateOpen(false)
          papers.reload()
        }}
      />
    )
  }

  return (
    <PageScaffold>
      <PageHeader
        title="试卷组卷"
        meta={
          papers.total
            ? [
                { value: `共 ${papers.total} 份` },
                { label: '手动选题', value: manualCount },
                { label: '按条件抽题', value: randomCount },
              ]
            : undefined
        }
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            新建试卷
          </Button>
        }
      />

      {papers.status === 'error' ? (
        <ResourceError
          message={papers.error?.message ?? '试卷暂时读取不到'}
          traceId={papers.error?.traceId}
          onRetry={papers.reload}
        />
      ) : papers.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={FileText} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">还没有试卷</p>
          <p className="mt-1.5 text-sm text-ink-sub">把题库里已发布的题目组成试卷,布置作业时可以整卷引用。</p>
          <Button variant="primary" leftIcon={Plus} className="mt-5" onClick={() => setCreateOpen(true)}>
            新建试卷
          </Button>
        </div>
      ) : papers.status === 'loading' && !papers.data ? (
        <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取试卷">
          {Array.from({ length: 3 }, (_, index) => (
            <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
              <Skeleton variant="title" width="half" />
              <Skeleton variant="line" lines={2} />
            </li>
          ))}
        </ul>
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={papers.status === 'loading' || undefined}>
            {(papers.data?.list ?? []).map((paper) => (
              <li key={paper.id}>
                <PaperCard paper={paper} onOpen={() => setDetailTarget(paper)} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={papers.page} pageSize={papers.pageSize} total={papers.total} onPageChange={papers.setPage} />
          </div>
        </>
      )}

      {detailTarget ? (
        <PaperDetailDrawer paper={detailTarget} onClose={() => setDetailTarget(undefined)} onChanged={papers.reload} />
      ) : null}
    </PageScaffold>
  )
}

/**
 * PaperCard 一份试卷:试卷名(整卡按钮,打开详情抽屉)、组卷方式、按条件抽题的条件、创建与更新时间。
 * 题数和总分列表接口没有,不在卡片上猜。
 */
function PaperCard({ paper, onOpen }: { paper: Paper; onOpen: () => void }) {
  const random = paper.gen_mode === PaperMode.RANDOM
  return (
    <Card interactive className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-3">
        <button type="button" onClick={onOpen} className="card-link line-clamp-2 text-left text-lg font-bold text-ink">
          {paper.name}
        </button>
        <Badge tone={random ? 'info' : 'neutral'} className="shrink-0">
          {paperModeLabel(paper.gen_mode)}
        </Badge>
      </div>
      {random ? <p className="line-clamp-2 text-sm text-ink-sub">{criteriaSummary(paper.gen_criteria)}</p> : null}
      <p className="mt-auto pt-3 text-sm text-ink-sub">
        <span title={formatDateTime(paper.created_at)}>创建于 {formatRelativeOccurrence(paper.created_at)}</span>
        {paper.updated_at !== paper.created_at ? (
          <span title={formatDateTime(paper.updated_at)}> · 更新于 {formatRelativeOccurrence(paper.updated_at)}</span>
        ) : null}
      </p>
    </Card>
  )
}

/**
 * criteriaSummary 把抽题条件写成一句话。
 * 条件是开放对象,只呈现已登记的键 —— 未登记键不猜语义。
 */
function criteriaSummary(criteria: PaperCriteria): string {
  const parts: string[] = []
  if (criteria.type !== undefined) parts.push(contentTypeLabel(criteria.type))
  if (criteria.difficulty && criteria.difficulty.length > 0) {
    parts.push(criteria.difficulty.map(contentDifficultyLabel).join('、'))
  }
  if (criteria.knowledge_points && criteria.knowledge_points.length > 0) {
    parts.push(criteria.knowledge_points.join('、'))
  }
  if (criteria.count !== undefined) parts.push(`抽 ${criteria.count} 题`)
  return parts.length > 0 ? parts.join(' · ') : '未设置条件'
}

interface PaperFormPageProps {
  onClose: () => void
  onSaved: () => void
}

/**
 * PaperFormPage 在完整内容页创建试卷,与选题列表切换时保留当前编辑态。
 * 组卷方式决定填什么:手动选题填题目清单,条件抽题填筛选条件。
 */
function PaperFormPage({ onClose, onSaved }: PaperFormPageProps) {
  const [name, setName] = useState('')
  const [mode, setMode] = useState(String(PaperMode.MANUAL))
  const [items, setItems] = useState<Array<Omit<PaperItemInput, 'score'> & { score: string }>>([])
  const [criteriaType, setCriteriaType] = useState(String(ContentType.THEORY_QUESTION))
  const [criteriaDifficulties, setCriteriaDifficulties] = useState<ContentDifficulty[]>([
    ContentDifficulty.BASIC,
  ])
  const [criteriaKnowledge, setCriteriaKnowledge] = useState('')
  const [criteriaCount, setCriteriaCount] = useState('10')
  const [criteriaScore, setCriteriaScore] = useState('10')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const isRandom = Number(mode) === PaperMode.RANDOM
  const { errors, validate } = useFormValidation(() => ({
    name: name.trim() === '' ? '请输入试卷名称' : null,
    items: !isRandom && items.length === 0 ? '手动选题需要至少添加一道题目' : null,
    count: isRandom && (!Number.isInteger(Number(criteriaCount)) || Number(criteriaCount) <= 0)
      ? '抽题数量需要是大于 0 的整数' : null,
    score: isRandom && (!Number.isInteger(Number(criteriaScore)) || Number(criteriaScore) <= 0)
      ? '每题分值需要是大于 0 的整数' : null,
    ...Object.fromEntries(items.map((item, index) => [
      `item-score-${index}`,
      !isRandom && (!Number.isInteger(Number(item.score)) || Number(item.score) <= 0)
        ? '分值需要是大于 0 的整数' : null,
    ])),
  }))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (working || !validate()) return
      setFormError(undefined)
      setWorking(true)
      try {
        await api.content.createPaper({
          name: name.trim(),
          gen_mode: Number(mode) as PaperMode,
          // 抽题条件按结构化字段组装,不接受用户手写 JSON
          gen_criteria: isRandom
            ? {
                type: Number(criteriaType) as ContentType,
                difficulty: criteriaDifficulties,
                knowledge_points: splitCommas(criteriaKnowledge),
                count: Number(criteriaCount),
                default_score: Number(criteriaScore),
              }
            : {},
          items: isRandom ? [] : items.map((item) => ({ ...item, score: Number(item.score) })),
        })
        toast.success('试卷已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '试卷创建失败,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [
      criteriaCount,
      criteriaDifficulties,
      criteriaKnowledge,
      criteriaScore,
      criteriaType,
      isRandom,
      items,
      mode,
      name,
      onSaved,
      validate,
      working,
    ]
  )

  const totalScore = items.reduce(
    (sum, item) => sum + (Number.isFinite(Number(item.score)) ? Number(item.score) : 0), 0,
  )

  return (
    <PageScaffold>
      <PageHeader
        title={pickerOpen ? '从题库选题' : '新建试卷'}
        description="手动选题适合精心编排的考试;按条件抽题适合练习卷,可以反复重抽得到不同组合。"
      />
      {pickerOpen ? (
        <ContentItemPicker
          selectedCodes={new Set(items.map((item) => item.code))}
          targetName="试卷"
          onClose={() => setPickerOpen(false)}
          onPick={(picked) => {
            setItems((current) => [
              ...current,
              ...picked.map((item) => ({ code: item.code, version: item.version, score: '10' })),
            ])
            setPickerOpen(false)
          }}
        />
      ) : (
        <form onSubmit={submit} noValidate>
          <div className="flex flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
            <FormField label="试卷名称" htmlFor="paper-name" required error={errors.name}>
              <Input
                id="paper-name"
                value={name}
                invalid={Boolean(errors.name)}
                onBlur={() => validate(['name'])}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>

            <FormField label="组卷方式" required>
              <SegmentedControl
                aria-label="组卷方式"
                options={[
                  { value: String(PaperMode.MANUAL), label: paperModeLabel(PaperMode.MANUAL) },
                  { value: String(PaperMode.RANDOM), label: paperModeLabel(PaperMode.RANDOM) },
                ]}
                value={mode}
                onValueChange={setMode}
              />
            </FormField>

            {isRandom ? (
              <div className="flex flex-col gap-4 rounded-md bg-surface-hover p-4">
                <div className="text-sm font-medium text-ink">抽题条件</div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField label="题目类型" htmlFor="criteria-type" required>
                    <Select
                      id="criteria-type"
                      options={CONTENT_TYPES.map((value) => ({
                        value: String(value),
                        label: contentTypeLabel(value),
                      }))}
                      value={criteriaType}
                      onValueChange={setCriteriaType}
                    />
                  </FormField>
                  <FormField label="抽题数量" htmlFor="criteria-count" required error={errors.count}>
                    <Input
                      id="criteria-count"
                      type="number"
                      min="1"
                      value={criteriaCount}
                      invalid={Boolean(errors.count)}
                      onBlur={() => validate(['count'])}
                      onChange={(event) => setCriteriaCount(event.target.value)}
                    />
                  </FormField>
                </div>

                <FormField label="难度范围" required helper="可以多选,系统在选中的难度里抽题">
                  <div className="flex flex-wrap gap-3">
                    {CONTENT_DIFFICULTIES.map((value) => (
                      <Checkbox
                        key={value}
                        checked={criteriaDifficulties.includes(value)}
                        label={contentDifficultyLabel(value)}
                        onCheckedChange={(checked) =>
                          setCriteriaDifficulties((current) =>
                            checked === true
                              ? [...current, value]
                              : current.filter((one) => one !== value)
                          )
                        }
                      />
                    ))}
                  </div>
                </FormField>

                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField
                    label="知识点"
                    htmlFor="criteria-knowledge"
                    helper="用逗号分隔,留空表示不限知识点"
                  >
                    <Input
                      id="criteria-knowledge"
                      value={criteriaKnowledge}
                      onChange={(event) => setCriteriaKnowledge(event.target.value)}
                    />
                  </FormField>
                  <FormField label="每题分值" htmlFor="criteria-score" required error={errors.score}>
                    <Input
                      id="criteria-score"
                      type="number"
                      min="1"
                      value={criteriaScore}
                      invalid={Boolean(errors.score)}
                      onBlur={() => validate(['score'])}
                      onChange={(event) => setCriteriaScore(event.target.value)}
                    />
                  </FormField>
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-sm font-medium text-ink">
                    题目({items.length} 题 · 合计 {totalScore} 分)
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    leftIcon={Plus}
                    onClick={() => setPickerOpen(true)}
                  >
                    从题库选题
                  </Button>
                </div>

                {errors.items ? <Callout tone="danger">{errors.items}</Callout> : null}

                {items.length === 0 ? (
                  <Empty
                    icon={FileText}
                    title="还没有题目"
                    description="从题库选题后设置每题分值。"
                  />
                ) : (
                  // 井内的并列条目用分隔线区分,不再各自画盒(规范 §6.5.1:不出现第三级容器)
                  <div className="flex flex-col divide-y divide-line">
                    {items.map((item, index) => (
                      <div
                        key={`${item.code}-${item.version}`}
                        className="flex flex-wrap items-end gap-3 py-3 first:pt-0 last:pb-0"
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-base text-ink">
                            第 {index + 1} 题 · {item.code}
                          </div>
                          <div className="truncate font-mono text-xs text-ink-sub">
                            版本 {item.version}
                          </div>
                        </div>
                        <FormField
                          label="分值"
                          htmlFor={`paper-item-score-${index}`}
                          error={errors[`item-score-${index}`]}
                          className="mb-0 w-24"
                        >
                          <Input
                            id={`paper-item-score-${index}`}
                            type="number"
                            min="1"
                            value={item.score}
                            invalid={Boolean(errors[`item-score-${index}`])}
                            onBlur={() => validate([`item-score-${index}`])}
                            onChange={(event) =>
                              setItems((current) =>
                                current.map((one, i) =>
                                  i === index
                                    ? { ...one, score: event.target.value }
                                    : one
                                )
                              )
                            }
                          />
                        </FormField>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            setItems((current) => current.filter((_, i) => i !== index))
                          }
                        >
                          移除
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </div>
          <div className="mt-5 flex flex-wrap justify-end gap-3">
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              创建试卷
            </Button>
          </div>
        </form>
      )}
    </PageScaffold>
  )
}

interface PaperDetailDrawerProps {
  paper: Paper
  onClose: () => void
  onChanged: () => void
}

/**
 * PaperDetailDrawer 展示试卷题目(序号、题目、题型与难度、分值)与总分,并支持重新抽题。
 * 重抽只对按条件抽题的试卷可用:手动选定的试卷重抽会丢掉教师的编排。
 */
function PaperDetailDrawer({ paper, onClose, onChanged }: PaperDetailDrawerProps) {
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const detail = useAsyncResource(() => api.content.getPaper(paper.id), [paper.id], () => false)

  const regenerate = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    try {
      await api.content.regeneratePaper(paper.id)
      toast.success('已按条件重新抽题')
      detail.reload()
      onChanged()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '重新抽题没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [detail, onChanged, paper.id])

  const isRandom = paper.gen_mode === PaperMode.RANDOM

  return (
    <Drawer open onOpenChange={(open) => !open && !working && onClose()}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{paper.name}</DrawerTitle>
          <DrawerDescription>
            {paperModeLabel(paper.gen_mode)}
            {isRandom ? ` · ${criteriaSummary(paper.gen_criteria)}` : ''}
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
          <ResourceState
            resource={detail}
            emptyIcon={FileText}
            emptyTitle="试卷里还没有题目"
            emptyDescription={isRandom ? '点「重新抽题」按条件抽取题目。' : '这份试卷没有题目。'}
            skeleton={<Skeleton variant="line" lines={6} />}
          >
            {(data) => (
              <>
                <p className="text-sm text-ink-sub">
                  共 {data.items.length} 题 · 合计 {data.items.reduce((sum, item) => sum + item.score, 0)} 分
                </p>
                {data.items.length === 0 ? (
                  <Empty
                    icon={FileText}
                    title="试卷里还没有题目"
                    description={isRandom ? '点「重新抽题」按条件抽取题目。' : '这份试卷没有题目。'}
                  />
                ) : (
                  <ol className="divide-y divide-line">
                    {[...data.items]
                      .sort((a, b) => a.seq - b.seq)
                      .map((item, index) => (
                        <li key={item.id} className="flex items-start gap-3 py-3">
                          <span className="w-6 shrink-0 text-right text-sm tabular-nums text-ink-sub">{index + 1}</span>
                          <span className="min-w-0 flex-1">
                            <span className="block text-sm font-medium text-ink">{item.item.title}</span>
                            <span className="block text-xs text-ink-sub">
                              {contentTypeLabel(item.item.type)} · {contentDifficultyLabel(item.item.difficulty)}
                            </span>
                          </span>
                          <span className="shrink-0 text-sm tabular-nums text-ink">{item.score} 分</span>
                        </li>
                      ))}
                  </ol>
                )}
              </>
            )}
          </ResourceState>
          {isRandom ? (
            <p className="text-sm text-ink-sub">重新抽题会按同样的条件抽取一批新题目,替换当前题目列表。</p>
          ) : null}
        </DrawerBody>
        <DrawerFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
            关闭
          </Button>
          {isRandom ? (
            <Button variant="primary" leftIcon={RefreshCw} loading={working} onClick={() => void regenerate()}>
              重新抽题
            </Button>
          ) : null}
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}

/** splitCommas 把逗号分隔文本拆成数组,去掉空项。 */
function splitCommas(text: string): string[] {
  return text
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '')
}
