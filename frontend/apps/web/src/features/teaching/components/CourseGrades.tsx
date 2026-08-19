// 教学领域的课程成绩复用区块。
// 三件事:配置权重(和须为 100%)、按权重计算全班成绩、人工调分与导出。
// 权重来源是封闭枚举(作业/实验/考试),来源引用填对应作业编号 ——
// 作业选项从课程作业清单取,不让教师手填内部编号。
// 课程详情与成绩报送页都嵌入本组件，页面只负责各自的路由与上下文。

import { useCallback, useMemo, useState } from 'react'
import { Calculator, Download, Pencil, Percent, Send } from 'lucide-react'
import {
  GradeSource,
  type Assignment,
  type GradeWeightInput,
  type TeachingCourseGrade,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageSection,
  Pagination,
  Select,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../hooks'
import { formatScore, formatShortDateTime } from '../../../utils/formatters'
import { gradeSourceLabel } from '../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { GRADE_SOURCES } from '../options'

export interface CourseGradesProps {
  courseId: string
}

/**
 * CourseGrades 承载权重配置、成绩计算、调分与导出;成绩汇总按 M6 契约一次读取完整数组。
 */
export function CourseGrades({ courseId }: CourseGradesProps) {
  const [weightOpen, setWeightOpen] = useState(false)
  const [overrideTarget, setOverrideTarget] = useState<TeachingCourseGrade>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const weights = useAsyncResource(() => api.teaching.listGradeWeights(courseId), [courseId], () => false)
  const grades = usePagedResource<TeachingCourseGrade>((params) => api.teaching.listGrades(courseId, params), [courseId])

  /** computeGrades 按权重重算全班成绩。 */
  const computeGrades = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    try {
      await api.teaching.computeGrades(courseId)
      toast.success('成绩已按权重重新计算')
      grades.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '成绩计算没有完成,请检查权重配置后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, grades])

  /** exportGrades 创建导出任务:结果文件在任务与下载页取件。 */
  const exportGrades = useCallback(async () => {
    setActionError(undefined)
    try {
      await api.teaching.exportGrades(courseId)
      toast.success('导出任务已创建,可在任务与下载中取件')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '导出任务创建失败,请稍后重试。'))
    }
  }, [courseId])

  const weightTotal = useMemo(
    () => (weights.data ?? []).reduce((sum, item) => sum + item.weight, 0),
    [weights.data],
  )

  const columns: TableColumn<TeachingCourseGrade>[] = [
    {
      key: 'student_id',
      header: '学生',
      // 成绩记录只回 student_id;按序号呈现,不把内部编号当学生名显示
      render: (grade) => (
        <span className="text-ink">{grade.is_locked ? '成绩已锁定' : '在读学生'}</span>
      ),
    },
    {
      key: 'auto_total',
      header: '按权重计算',
      align: 'right',
      mono: true,
      render: (grade) => formatScore(grade.auto_total),
    },
    {
      key: 'override_total',
      header: '人工调整',
      align: 'right',
      mono: true,
      render: (grade) => (grade.is_overridden ? formatScore(grade.override_total) : '—'),
    },
    {
      key: 'final_total',
      header: '总评',
      align: 'right',
      mono: true,
      render: (grade) => (
        <span className="font-medium text-ink">{formatScore(grade.final_total)}</span>
      ),
    },
    { key: 'credits', header: '学分', align: 'right', mono: true },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (grade) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(grade.updated_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (grade) =>
        grade.is_locked ? (
          <Badge tone="neutral">已锁定</Badge>
        ) : (
          <Button variant="ghost" size="sm" leftIcon={Pencil} onClick={() => setOverrideTarget(grade)}>
            调分
          </Button>
        ),
    },
  ]

  return (
    <>
      <PageSection
        title="成绩权重"
        description="设置作业、实验与考试在总评里的占比,合计需要等于 100%。"
        actions={
          <Button variant="outline" leftIcon={Percent} onClick={() => setWeightOpen(true)}>
            配置权重
          </Button>
        }
      >
        <ResourceState
          resource={weights}
          emptyIcon={Percent}
          emptyTitle="还没有配置权重"
          emptyDescription="配置权重后才能按权重计算全班成绩。"
          emptyAction={
            <Button variant="primary" leftIcon={Percent} onClick={() => setWeightOpen(true)}>
              配置权重
            </Button>
          }
          skeleton={<Skeleton variant="line" lines={2} />}
        >
          {(list) => (
            <Card>
              <CardHeader
                title={`已配置 ${list.length} 项`}
                description={`合计 ${weightTotal}%`}
                actions={weightTotal !== 100 ? <Badge tone="warning">合计不等于 100%</Badge> : null}
              />
              <CardBody className="flex flex-wrap gap-2">
                {list.map((item) => (
                  <Badge key={item.id} tone="neutral">
                    {gradeSourceLabel(item.source_type)} {item.weight}%
                  </Badge>
                ))}
              </CardBody>
            </Card>
          )}
        </ResourceState>
      </PageSection>

      <PageSection
        title="全班成绩"
        description={`共 ${grades.data?.total ?? 0} 名学生。调分只影响总评,不改变各项得分。`}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" leftIcon={Calculator} loading={working} onClick={() => void computeGrades()}>
              按权重计算
            </Button>
            <Button variant="outline" leftIcon={Download} onClick={() => void exportGrades()}>
              导出成绩
            </Button>
          </div>
        }
      >
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={grades}
            emptyIcon={Send}
            emptyTitle="还没有成绩记录"
            emptyDescription="配置权重后点「按权重计算」生成全班成绩。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <Table columns={columns} data={page.list} rowKey={(grade) => grade.student_id} />
                <Pagination page={grades.page} pageSize={grades.pageSize} total={grades.total} onPageChange={grades.setPage} />
              </div>
            )}
          </ResourceState>
        </div>
      </PageSection>

      {weightOpen ? (
        <WeightFormModal
          courseId={courseId}
          current={weights.data ?? []}
          onClose={() => setWeightOpen(false)}
          onSaved={() => {
            setWeightOpen(false)
            weights.reload()
          }}
        />
      ) : null}

      {overrideTarget ? (
        <OverrideGradeModal
          courseId={courseId}
          grade={overrideTarget}
          onClose={() => setOverrideTarget(undefined)}
          onSaved={() => {
            setOverrideTarget(undefined)
            grades.reload()
          }}
        />
      ) : null}
    </>
  )
}

interface WeightFormModalProps {
  courseId: string
  current: { source_type: GradeSource; source_ref: string; weight: number }[]
  onClose: () => void
  onSaved: () => void
}

/**
 * WeightFormModal 配置成绩权重。
 * 合计必须等于 100%(后端也校验),提交前在此就近提示,不让用户白跑一次请求。
 */
function WeightFormModal({ courseId, current, onClose, onSaved }: WeightFormModalProps) {
  const [rows, setRows] = useState<GradeWeightInput[]>(
    current.length > 0
      ? current.map((item) => ({
          source_type: item.source_type,
          source_ref: item.source_ref,
          weight: item.weight,
        }))
      : [{ source_type: GradeSource.ASSIGNMENT, source_ref: '', weight: 100 }],
  )
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  // 作业选项:权重的 source_ref 指向具体作业,教师从清单里选而不是手填编号
  const assignmentList = usePagedResource<Assignment>((params) => api.teaching.listCourseAssignments(courseId, params), [courseId])

  const total = rows.reduce((sum, row) => sum + row.weight, 0)

  const updateRow = useCallback((index: number, patch: Partial<GradeWeightInput>) => {
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }, [])

  const submit = useCallback(async () => {
    if (total !== 100) {
      setFormError(`权重合计当前是 ${total}%,需要正好等于 100%`)
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.teaching.setGradeWeights(courseId, { items: rows })
      toast.success('权重已保存')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '权重保存失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, onSaved, rows, total])

  const assignmentOptions = useMemo(
    () =>
      (assignmentList.data?.list ?? []).map((item: Assignment) => ({
        value: item.id,
        label: item.title,
      })),
    [assignmentList.data],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>配置成绩权重</ModalTitle>
          <ModalDescription>各项占比合计需要等于 100%,系统按此计算总评。</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {rows.map((row, index) => (
            <div key={index} className="flex flex-wrap items-end gap-3 well p-3">
              <FormField label="成绩来源" htmlFor={`weight-source-${index}`} className="mb-0 w-36">
                <Select
                  id={`weight-source-${index}`}
                  size="sm"
                  options={GRADE_SOURCES.map((value) => ({
                    value: String(value),
                    label: gradeSourceLabel(value),
                  }))}
                  value={String(row.source_type)}
                  onValueChange={(value) =>
                    updateRow(index, { source_type: Number(value) as GradeSource })
                  }
                />
              </FormField>
              {row.source_type === GradeSource.ASSIGNMENT ? (
                <ResourceState
                  resource={assignmentList}
                  emptyIcon={Send}
                  emptyTitle="课程还没有作业"
                  emptyDescription="先创建作业,再把它纳入成绩权重。"
                  skeleton={<Skeleton variant="line" lines={1} />}
                >
                  {() => (
                    <div className="flex min-w-48 flex-1 flex-col gap-2">
                      <FormField label="对应作业" htmlFor={`weight-ref-${index}`} className="mb-0">
                        <Select
                          id={`weight-ref-${index}`}
                          size="sm"
                          options={assignmentOptions}
                          value={row.source_ref}
                          placeholder="选择作业"
                          onValueChange={(value) => updateRow(index, { source_ref: value })}
                        />
                      </FormField>
                      <Pagination
                        page={assignmentList.page}
                        pageSize={assignmentList.pageSize}
                        total={assignmentList.total}
                        onPageChange={assignmentList.setPage}
                      />
                    </div>
                  )}
                </ResourceState>
              ) : (
                <FormField
                  label="来源说明"
                  htmlFor={`weight-ref-${index}`}
                  className="mb-0 min-w-48 flex-1"
                  helper="实验与考试按整体计入,填写便于识别的名称"
                >
                  <Input
                    id={`weight-ref-${index}`}
                    value={row.source_ref}
                    onChange={(event) => updateRow(index, { source_ref: event.target.value })}
                  />
                </FormField>
              )}
              <FormField label="占比 %" htmlFor={`weight-value-${index}`} className="mb-0 w-24">
                <Input
                  id={`weight-value-${index}`}
                  type="number"
                  min="0"
                  max="100"
                  value={String(row.weight)}
                  onChange={(event) => updateRow(index, { weight: Number(event.target.value) || 0 })}
                />
              </FormField>
              {rows.length > 1 ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setRows((current) => current.filter((_, i) => i !== index))}
                >
                  移除
                </Button>
              ) : null}
            </div>
          ))}

          <div className="flex flex-wrap items-center justify-between gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setRows((current) => [
                  ...current,
                  { source_type: GradeSource.ASSIGNMENT, source_ref: '', weight: 0 },
                ])
              }
            >
              添加一项
            </Button>
            <Badge tone={total === 100 ? 'success' : 'warning'}>合计 {total}%</Badge>
          </div>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            保存权重
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface OverrideGradeModalProps {
  courseId: string
  grade: TeachingCourseGrade
  onClose: () => void
  onSaved: () => void
}

/**
 * OverrideGradeModal 人工调整学生总评。
 * 调分只覆盖总评,不改变各项得分,故弹层里把两者并列展示。
 */
function OverrideGradeModal({ courseId, grade, onClose, onSaved }: OverrideGradeModalProps) {
  const [total, setTotal] = useState(String(grade.final_total))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    const value = Number(total)
    if (!Number.isFinite(value) || value < 0 || value > 100) {
      setFormError('总评需要是 0 到 100 之间的数字')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.teaching.overrideGrade(courseId, grade.student_id, { total: value })
      toast.success('总评已调整')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '调分没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, grade.student_id, onSaved, total])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>人工调整总评</ModalTitle>
          <ModalDescription>
            调整后以人工分为最终总评。按权重计算的分数仍然保留,便于对照。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <div className="well p-3 text-sm text-ink-sub">
            按权重计算:{formatScore(grade.auto_total)} 分
          </div>
          <FormField label="调整后的总评" htmlFor="override-total" required error={formError}>
            <Input
              id="override-total"
              type="number"
              min="0"
              max="100"
              step="0.5"
              value={total}
              invalid={Boolean(formError)}
              onChange={(event) => setTotal(event.target.value)}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            保存调分
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
