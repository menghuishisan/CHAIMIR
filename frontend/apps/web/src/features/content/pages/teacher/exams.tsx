// 试卷组卷页(教师侧栏,/teacher/exams)。
//
// 两种组卷方式:手动选题(逐题选定并给分)与按条件抽题(设定类型、难度、知识点与题数,
// 由后端随机抽取)。按条件抽题可以重新抽取,手动选题不需要 —— 重抽会打乱教师的选择。
//
// 抽题条件按结构化字段渲染,不给裸 JSON(旧前端组卷条件是裸 JSON 文本域)。

import { useCallback, useMemo, useState } from 'react'
import { Dices, FileText, Plus, RefreshCw } from 'lucide-react'
import {
  ContentDifficulty,
  ContentType,
  PaperMode,
  type ContentItem,
  type Paper,
  type PaperCriteria,
  type PaperItemInput,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  DescriptionList,
  Empty,
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
  Select,
  Skeleton,
  Stat,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  CONTENT_DIFFICULTIES,
  CONTENT_TYPES,
  contentDifficultyLabel,
  contentTypeLabel,
  paperModeLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 题目选择器一次取回的条数:后端分页上限 100。 */
const ITEM_PICKER_SIZE = 100

/**
 * TeacherExamsPage 管理试卷。
 */
export default function TeacherExamsPage() {
  const [createOpen, setCreateOpen] = useState(false)
  const [detailTarget, setDetailTarget] = useState<Paper>()

  const papers = usePagedResource<Paper>((params) => api.content.listPapers(params), [])

  const stats = useMemo(() => {
    const list = papers.data ? papers.data.list : []
    return {
      manual: list.filter((paper) => paper.gen_mode === PaperMode.MANUAL).length,
      random: list.filter((paper) => paper.gen_mode === PaperMode.RANDOM).length,
    }
  }, [papers.data])

  const columns: TableColumn<Paper>[] = [
    {
      key: 'name',
      header: '试卷',
      render: (paper) => <span className="font-medium text-ink">{paper.name}</span>,
    },
    {
      key: 'gen_mode',
      header: '组卷方式',
      render: (paper) => <Badge tone="neutral">{paperModeLabel(paper.gen_mode)}</Badge>,
    },
    {
      key: 'criteria',
      header: '抽题条件',
      render: (paper) =>
        paper.gen_mode === PaperMode.RANDOM ? (
          <span className="text-xs text-ink-sub">{criteriaSummary(paper.gen_criteria)}</span>
        ) : (
          <span className="text-ink-sub">手动选定</span>
        ),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (paper) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(paper.updated_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (paper) => (
        <Button variant="ghost" size="sm" onClick={() => setDetailTarget(paper)}>
          查看题目
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '资源' }, { label: '试卷组卷' }]} />}
        title="试卷组卷"
        description="把题库里的题目组成试卷。按条件抽题的试卷可以重新抽取,手动选定的保持不变。"
        icon={FileText}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            新建试卷
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="试卷总数" value={papers.total} icon={FileText} />
          <Stat label="本页手动选题" value={stats.manual} icon={FileText} />
          <Stat label="本页条件抽题" value={stats.random} icon={Dices} hint="可重新抽取" />
        </div>
      </PageSection>

      <PageSection title="试卷列表" description={`共 ${papers.total} 份试卷`}>
        <ResourceState
          resource={papers}
          emptyIcon={FileText}
          emptyTitle="还没有试卷"
          emptyDescription="新建试卷后可以在作业里引用整卷题目。"
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              新建试卷
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table columns={columns} data={page.list} rowKey={(paper) => paper.id} />
              <Pagination
                page={papers.page}
                pageSize={papers.pageSize}
                total={papers.total}
                onPageChange={papers.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>

      {createOpen ? (
        <PaperFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            papers.reload()
          }}
        />
      ) : null}

      {detailTarget ? (
        <PaperDetailModal
          paper={detailTarget}
          onClose={() => setDetailTarget(undefined)}
          onChanged={papers.reload}
        />
      ) : null}
    </PageScaffold>
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

interface PaperFormModalProps {
  onClose: () => void
  onSaved: () => void
}

/**
 * PaperFormModal 创建试卷。
 * 组卷方式决定填什么:手动选题填题目清单,条件抽题填筛选条件。
 */
function PaperFormModal({ onClose, onSaved }: PaperFormModalProps) {
  const [name, setName] = useState('')
  const [mode, setMode] = useState(String(PaperMode.MANUAL))
  const [items, setItems] = useState<PaperItemInput[]>([])
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

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入试卷名称')
        return
      }
      if (!isRandom && items.length === 0) {
        setFormError('手动选题需要至少添加一道题目')
        return
      }
      if (isRandom && (Number(criteriaCount) || 0) <= 0) {
        setFormError('抽题数量需要大于 0')
        return
      }
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
                count: Number(criteriaCount) || 0,
                default_score: Number(criteriaScore) || 0,
              }
            : {},
          items: isRandom ? [] : items,
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
    ],
  )

  const totalScore = items.reduce((sum, item) => sum + item.score, 0)

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>新建试卷</ModalTitle>
          <ModalDescription>
            手动选题适合精心编排的考试;按条件抽题适合练习卷,可以反复重抽得到不同组合。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="试卷名称" htmlFor="paper-name" required>
              <Input id="paper-name" value={name} onChange={(event) => setName(event.target.value)} />
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
              <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
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
                  <FormField label="抽题数量" htmlFor="criteria-count" required>
                    <Input
                      id="criteria-count"
                      type="number"
                      min="1"
                      value={criteriaCount}
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
                              : current.filter((one) => one !== value),
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
                  <FormField label="每题分值" htmlFor="criteria-score" required>
                    <Input
                      id="criteria-score"
                      type="number"
                      min="1"
                      value={criteriaScore}
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

                {items.length === 0 ? (
                  <Empty
                    icon={FileText}
                    title="还没有题目"
                    description="从题库选题后设置每题分值。"
                  />
                ) : (
                  <div className="flex flex-col gap-2">
                    {items.map((item, index) => (
                      <div
                        key={`${item.code}-${item.version}`}
                        className="flex flex-wrap items-end gap-3 rounded-md border border-line p-3"
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-base text-ink">
                            第 {index + 1} 题 · {item.code}
                          </div>
                          <div className="truncate font-mono text-xs text-ink-sub">
                            版本 {item.version}
                          </div>
                        </div>
                        <FormField label="分值" htmlFor={`paper-item-score-${index}`} className="mb-0 w-24">
                          <Input
                            id={`paper-item-score-${index}`}
                            type="number"
                            min="1"
                            value={String(item.score)}
                            onChange={(event) =>
                              setItems((current) =>
                                current.map((one, i) =>
                                  i === index ? { ...one, score: Number(event.target.value) || 0 } : one,
                                ),
                              )
                            }
                          />
                        </FormField>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => setItems((current) => current.filter((_, i) => i !== index))}
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
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              创建试卷
            </Button>
          </ModalFooter>
        </form>

        {pickerOpen ? (
          <PaperItemPicker
            selectedCodes={new Set(items.map((item) => item.code))}
            onClose={() => setPickerOpen(false)}
            onPick={(picked) => {
              setItems((current) => [
                ...current,
                ...picked.map((item) => ({ code: item.code, version: item.version, score: 10 })),
              ])
              setPickerOpen(false)
            }}
          />
        ) : null}
      </ModalContent>
    </Modal>
  )
}

interface PaperItemPickerProps {
  selectedCodes: Set<string>
  onClose: () => void
  onPick: (items: ContentItem[]) => void
}

/**
 * PaperItemPicker 从题库选题。
 */
function PaperItemPicker({ selectedCodes, onClose, onPick }: PaperItemPickerProps) {
  const [picked, setPicked] = useState<ContentItem[]>([])
  const items = useAsyncResource(() => api.content.getItems({ page: 1, size: ITEM_PICKER_SIZE }), [])

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
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => {
        const alreadyIn = selectedCodes.has(item.code)
        const isPicked = picked.some((one) => one.code === item.code)
        return (
          <Button
            type="button"
            variant={isPicked ? 'outline' : 'ghost'}
            size="sm"
            disabled={alreadyIn}
            onClick={() =>
              setPicked((current) =>
                isPicked ? current.filter((one) => one.code !== item.code) : [...current, item],
              )
            }
          >
            {alreadyIn ? '已在试卷中' : isPicked ? '取消选择' : '选择'}
          </Button>
        )
      },
    },
  ]

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>从题库选题</ModalTitle>
          <ModalDescription>题目按当前版本锁进试卷。</ModalDescription>
        </ModalHeader>
        <ModalBody>
          <ResourceState
            resource={items}
            emptyIcon={FileText}
            emptyTitle="题库里还没有题目"
            emptyDescription="先在题库内容里创建题目,再回来组卷。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="max-h-96 overflow-y-auto">
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              </div>
            )}
          </ResourceState>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button type="button" variant="seal" disabled={picked.length === 0} onClick={() => onPick(picked)}>
            添加 {picked.length > 0 ? `${picked.length} 道` : ''}题目
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface PaperDetailModalProps {
  paper: Paper
  onClose: () => void
  onChanged: () => void
}

/**
 * PaperDetailModal 展示试卷题目并支持重新抽题。
 * 重抽只对条件抽题的试卷可用:手动选定的试卷重抽会丢掉教师的编排。
 */
function PaperDetailModal({ paper, onClose, onChanged }: PaperDetailModalProps) {
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const detail = useAsyncResource(() => api.content.getPaper(paper.id), [paper.id], () => false)

  const regenerate = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    try {
      await api.content.regeneratePaper(paper.id)
      toast.success('已重新抽题')
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
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{paper.name}</ModalTitle>
          <ModalDescription>
            {paperModeLabel(paper.gen_mode)}
            {isRandom ? ` · ${criteriaSummary(paper.gen_criteria)}` : ''}
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={detail}
            emptyIcon={FileText}
            emptyTitle="试卷里还没有题目"
            emptyDescription={isRandom ? '点「重新抽题」按条件抽取题目。' : '这份试卷没有题目。'}
            skeleton={<Skeleton variant="line" lines={5} />}
          >
            {(data) => (
              <Card>
                <CardHeader
                  title={`共 ${data.items.length} 题`}
                  description={`合计 ${data.items.reduce((sum, item) => sum + item.score, 0)} 分`}
                />
                <CardBody>
                  <DescriptionList
                    items={data.items.map((item, index) => ({
                      term: `第 ${index + 1} 题 · ${item.score} 分`,
                      description: `${item.item.title}(${contentTypeLabel(item.item.type)} · ${contentDifficultyLabel(item.item.difficulty)})`,
                    }))}
                  />
                </CardBody>
              </Card>
            )}
          </ResourceState>

          {isRandom ? (
            <Callout tone="info">
              重新抽题会按同样的条件抽取一批新题目,替换当前题目列表。
            </Callout>
          ) : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          {isRandom ? (
            <Button variant="seal" leftIcon={RefreshCw} loading={working} onClick={() => void regenerate()}>
              重新抽题
            </Button>
          ) : null}
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** splitCommas 把逗号分隔文本拆成数组,去掉空项。 */
function splitCommas(text: string): string[] {
  return text
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '')
}
