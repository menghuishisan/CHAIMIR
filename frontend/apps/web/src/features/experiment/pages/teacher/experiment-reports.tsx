// 实验报告批改与小组编排(教师深页,/teacher/experiments/:experimentId/reports)。
//
// 两件事同页:批改学生提交的实验报告、给小组实验编排分组与角色。
// 它们都以同一个实验为上下文;编排向导负责"实验怎么配",本页负责"学生做完之后怎么处理"。
//
// 报告正文是对象存储引用,取件统一走 transfer/storage 授权,不暴露对象存储直链。
// 小组分区只在协作方式为小组实验时出现 —— 后端 CreateGroup 对独立完成的实验直接拒绝。

import { useCallback, useMemo, useState, type KeyboardEvent } from 'react'
import { useParams } from 'react-router'
import {
  ChevronLeft,
  ClipboardCheck,
  Download,
  FileText,
  LayoutTemplate,
  Plus,
  Trash2,
  UserPlus,
  Users,
} from 'lucide-react'
import {
  ExperimentCollabMode,
  ExperimentReportStatus,
  PAGINATION_MAX_SIZE,
  type Class,
  type ClassStudent,
  type Experiment,
  type ExperimentGroup,
  type ReportDTO,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  cn,
  DataPanel,
  Empty,
  FilterBar,
  FilterField,
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
  QueueDetailLayout,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import type { PagedResourceState } from '../../../../hooks/usePagedResource'
import { formatDateTime } from '../../../../utils/formatters'
import { experimentCollabModeLabel, experimentReportStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { experimentReportStatusTone } from '../../statusPresentation'

/**
 * TeacherExperimentReportsPage 读取实验本体并承载报告批改与分组编排。
 */
export default function TeacherExperimentReportsPage() {
  const { experimentId = '' } = useParams<{ experimentId: string }>()

  // 单读:深链与刷新都直接读这一条,不再拉全量列表在浏览器里筛
  const experiment = useAsyncResource(
    () => api.experiment.getExperiment(experimentId),
    [experimentId],
    () => false,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={experiment}
        emptyIcon={LayoutTemplate}
        emptyTitle="实验不存在"
        emptyDescription="这个实验可能已被删除,请回到实验编排查看。"
      >
        {(data) => <ReportsContent experiment={data} />}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * ReportsContent 渲染实验头部、报告批改队列与小组编排。
 */
function ReportsContent({ experiment }: { experiment: Experiment }) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const isGroup = experiment.collab_mode === ExperimentCollabMode.GROUP

  const reports = usePagedResource<ReportDTO>(
    (params) =>
      api.experiment.listReports(experiment.id, {
        status: statusFilter ? (Number(statusFilter) as ExperimentReportStatus) : undefined,
        ...params,
      }),
    [experiment.id, statusFilter],
  )

  // 待办摘要取服务端全量口径:待批改是教师最常看的数字,不能用当前页数出来的近似值
  const totalCount = useResourceTotal(
    (params) => api.experiment.listReports(experiment.id, params),
    [experiment.id],
  )
  const pendingCount = useResourceTotal(
    (params) =>
      api.experiment.listReports(experiment.id, {
        status: ExperimentReportStatus.SUBMITTED,
        ...params,
      }),
    [experiment.id],
  )
  const gradedCount = useResourceTotal(
    (params) =>
      api.experiment.listReports(experiment.id, {
        status: ExperimentReportStatus.GRADED,
        ...params,
      }),
    [experiment.id],
  )

  /** downloadReport 经业务授权和统一文件服务下载报告,不暴露对象存储地址。 */
  const downloadReport = useCallback(async (report: ReportDTO) => {
    try {
      const grant = await api.experiment.issueReportAccess(report.id)
      const file = await api.storage.consumeGrant(grant.token)
      downloadAttachment(file)
    } catch (error) {
      toast.error(userFacingErrorMessage(error, '报告没能下载,请稍后重试。'))
    }
  }, [])


  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实践' },
              { label: '实验编排', href: '/teacher/experiments' },
            ]}
          />
        }
        title={`${experiment.name} · 报告与小组`}
        description="批改学生提交的实验报告;小组实验在这里编排分组与角色。"
        icon={ClipboardCheck}
        actions={
          <Badge tone="neutral">{experimentCollabModeLabel(experiment.collab_mode)}</Badge>
        }
      />

      {/*
        待办摘要一行(§6.5.3 第 ⑤ 族:审阅队列不做指标带,待办数量放标题下一行)。
        三个数字都取服务端全量口径(§6.5.4);检查点合计是实验定义的静态属性,
        故与报告数量分开表述,不混进「待办」里。
      */}
      <p className="mb-4 text-sm text-ink-sub">
        共 {totalCount ?? '—'} 份报告,待批改 {pendingCount ?? '—'} 份、已批改{' '}
        {gradedCount ?? '—'} 份。检查点合计{' '}
        {experiment.components.checkpoints.reduce((sum, item) => sum + item.score, 0)} 分,
        与报告分相加即实验得分。
      </p>

      {experiment.require_report ? null : (
        <Callout tone="info" className="mb-4">
          这个实验没有要求提交报告,学生的得分只来自检查点。要收报告请在编排向导第 1 步勾选。
        </Callout>
      )}

      {/*
        小组实验多一件事(编排分组),故用 Tabs 把两件事分开而不是把小组塞进右栏 ——
        右栏在这一族是「当前报告的批改表单」,再挤一个小组面板会让两栏都不够用。
        独立完成的实验没有小组可编排,那时不出 Tabs:一个标签的标签栏是纯噪声。
      */}
      {isGroup ? (
        <Tabs defaultValue="reports">
          <TabsList aria-label="报告与小组">
            <TabsTrigger value="reports">实验报告</TabsTrigger>
            <TabsTrigger value="groups">小组编排</TabsTrigger>
          </TabsList>
          <TabsContent value="reports">
            <ReportsQueue
              reports={reports}
              onDownload={downloadReport}
              onSaved={reports.reload}
              statusFilter={statusFilter}
              onStatusFilterChange={setStatusFilter}
            />
          </TabsContent>
          <TabsContent value="groups">
            <GroupsSection experiment={experiment} />
          </TabsContent>
        </Tabs>
      ) : (
        <ReportsQueue
          reports={reports}
          onDownload={downloadReport}
          onSaved={reports.reload}
          statusFilter={statusFilter}
          onStatusFilterChange={setStatusFilter}
        />
      )}
    </>
  )
}

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(ExperimentReportStatus.SUBMITTED), label: '待批改' },
  { value: String(ExperimentReportStatus.GRADED), label: '已批改' },
] as const

interface ReportsQueueProps {
  reports: PagedResourceState<ReportDTO>
  onDownload: (report: ReportDTO) => void
  onSaved: () => void
  statusFilter: string
  onStatusFilterChange: (value: string) => void
}

/**
 * ReportsQueue 是报告批改的审阅队列(§6.5.3 第 ⑤ 族):左队列 + 右批改表单。
 *
 * 批改不走弹窗:一个班的报告要逐份看正文再给分,为每份开窗关窗会把主动作变成开窗动作
 * (§7.2 只在不可撤销的批量后果时才要二次确认,给分不是)。
 * 处理完自动落到下一份待批改报告。
 */
function ReportsQueue({
  reports,
  onDownload,
  onSaved,
  statusFilter,
  onStatusFilterChange,
}: ReportsQueueProps) {
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面(§6.4.1 规则 4) */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')

  const list = useMemo(() => reports.data?.list ?? [], [reports.data])
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  /** 批完一份直接落到下一份待批改报告;没有下一份就退回队列层 */
  const advanceToNext = useCallback(() => {
    const nextPending = list.find(
      (item, index) => index > selectedIndex && item.status !== ExperimentReportStatus.GRADED,
    )
    setSelectedId(nextPending?.id ?? undefined)
    if (!nextPending) setMobileView('queue')
  }, [list, selectedIndex])

  /** 键盘上下切条:整批批改靠键盘才快 */
  const onQueueKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const step = event.key === 'ArrowDown' ? 1 : -1
    const next = list[Math.min(Math.max(selectedIndex + step, 0), list.length - 1)]
    if (next) setSelectedId(next.id)
  }

  return (
    <QueueDetailLayout
      view={mobileView}
      detailHeader={
        selected ? (
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              leftIcon={ChevronLeft}
              onClick={() => setMobileView('queue')}
            >
              返回队列
            </Button>
            <span className="text-sm text-ink-sub tabular-nums">
              第 {selectedIndex + 1} 份 / 共 {reports.total} 份
            </span>
          </div>
        ) : undefined
      }
      queue={
        <DataPanel
          label="报告队列"
          className="min-h-0 flex-1"
          filter={
            <FilterBar label="报告筛选">
              <FilterField label="批改状态" group>
                <SegmentedControl
                  aria-label="按批改状态筛选"
                  size="sm"
                  options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                  value={statusFilter}
                  onValueChange={(value) => {
                    onStatusFilterChange(value)
                    setSelectedId(undefined)
                  }}
                />
              </FilterField>
            </FilterBar>
          }
          footer={
            <Pagination
              page={reports.page}
              pageSize={reports.pageSize}
              total={reports.total}
              onPageChange={reports.setPage}
            />
          }
        >
          <ResourceState
            resource={reports}
            emptyIcon={FileText}
            emptyTitle={statusFilter ? '这个状态下没有报告' : '还没有报告'}
            emptyDescription={
              statusFilter ? '换个状态看看,或查看全部报告。' : '学生在实验里提交报告后会出现在这里。'
            }
            skeleton={<Skeleton variant="line" lines={5} />}
          >
            {(page) => (
              <div
                role="listbox"
                aria-label="报告队列"
                aria-activedescendant={selected ? `report-${selected.id}` : undefined}
                tabIndex={0}
                onKeyDown={onQueueKeyDown}
                className="focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
              >
                {page.list.map((report) => {
                  const isActive = report.id === selected?.id
                  return (
                    <div
                      key={report.id}
                      id={`report-${report.id}`}
                      role="option"
                      aria-selected={isActive}
                      onClick={() => {
                        setSelectedId(report.id)
                        setMobileView('detail')
                      }}
                      className={cn(
                        'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                        isActive ? 'bg-primary-soft' : 'hover:bg-surface-hover',
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="truncate text-sm font-medium text-ink">
                          {report.student_name}
                        </span>
                        <StatusIndicator
                          tone={experimentReportStatusTone(report.status)}
                          label={experimentReportStatusLabel(report.status)}
                        />
                      </div>
                      {report.student_no ? (
                        <p className="mt-1 truncate font-mono text-xs text-ink-sub">
                          {report.student_no}
                        </p>
                      ) : null}
                      <p className="mt-1 font-mono text-xs text-ink-faint">
                        {formatDateTime(report.submitted_at)}
                      </p>
                    </div>
                  )
                })}
                <p className="border-t border-line px-4 py-2 text-xs text-ink-faint">↑↓ 切换条目</p>
              </div>
            )}
          </ResourceState>
        </DataPanel>
      }
      detail={
        selected ? (
          <ReportGradePane
            key={selected.id}
            report={selected}
            onDownload={() => onDownload(selected)}
            onSaved={() => {
              advanceToNext()
              onSaved()
            }}
          />
        ) : (
          <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
            左侧选一份报告,先下载正文再在这里给分。
          </div>
        )
      }
    />
  )
}

interface ReportGradePaneProps {
  report: ReportDTO
  onDownload: () => void
  onSaved: () => void
}

/**
 * ReportGradePane 是审阅队列族的右侧详情与批改表单(§6.5.3 第 ⑤ 族)。
 * 报告正文是对象存储引用,故这里给的是取件按钮而不是内嵌预览:
 * 报告格式不固定(文档/压缩包/笔记本),统一走一次性下载授权最可靠。
 *
 * key 由调用方按报告编号给出:切换条目时表单重建,上一份的草稿不会串到下一份。
 */
function ReportGradePane({ report, onDownload, onSaved }: ReportGradePaneProps) {
  const [score, setScore] = useState(String(report.manual_score || ''))
  const [comment, setComment] = useState(report.comment ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    const value = Number(score)
    if (!Number.isFinite(value) || value < 0) {
      setFormError('分数需要是 0 或更大的数字')
      return
    }
    if (comment.trim() === '') {
      setFormError('请写下批改意见,让学生知道得分依据')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.experiment.gradeReport(report.id, { manual_score: value, comment: comment.trim() })
      toast.success('已提交批改结果')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '批改没有保存成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [comment, onSaved, report.id, score])

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-md font-semibold text-ink">{report.student_name}</h3>
          <p className="mt-0.5 text-xs text-ink-sub">
            {report.student_no ? `${report.student_no} · ` : ''}
            {formatDateTime(report.submitted_at)}
          </p>
        </div>
        <StatusIndicator
          tone={experimentReportStatusTone(report.status)}
          label={experimentReportStatusLabel(report.status)}
        />
      </div>

      <div>
        <Button variant="outline" size="sm" leftIcon={Download} onClick={onDownload}>
          下载报告正文
        </Button>
      </div>

      <FormField label="报告得分" htmlFor="report-score" required>
        <Input
          id="report-score"
          type="number"
          min="0"
          step="0.5"
          value={score}
          onChange={(event) => setScore(event.target.value)}
        />
      </FormField>

      <FormField
        label="批改意见"
        htmlFor="report-comment"
        required
        error={formError}
        helper="说清楚得分依据与改进方向,学生会在实验详情里看到"
      >
        <Textarea
          id="report-comment"
          value={comment}
          rows={6}
          invalid={Boolean(formError)}
          onChange={(event) => setComment(event.target.value)}
        />
      </FormField>

      {/* 动作条贴详情片底边;<lg 由 QueueDetailLayout 钉到屏幕底部 */}
      <div className="mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4">
        <span className="mr-auto text-sm text-ink-sub">报告分与检查点得分相加成为实验总分。</span>
        <Button variant="primary" loading={working} onClick={() => void submit()}>
          提交批改并看下一份
        </Button>
      </div>
    </div>
  )
}

/**
 * GroupsSection 编排小组与成员角色(页内子视图,§6.5.5 B)。
 * 角色只能从实验定义的角色集合里选(后端 roleAllowed 同一口径);
 * 学生从账号档案里选,不让教师手填学生编号。
 *
 * 小组卡按网格排开而不是纵向堆一列:它从右栏窄条挪到了整幅宽度,
 * 一列到底会让每张卡横向留出大片空白(§6.5.0 通则 3:不补版面,也不浪费版面)。
 */
function GroupsSection({ experiment }: { experiment: Experiment }) {
  const [createOpen, setCreateOpen] = useState(false)
  const [memberTarget, setMemberTarget] = useState<ExperimentGroup>()
  const [removeTarget, setRemoveTarget] = useState<{ group: ExperimentGroup; studentId: string; studentName: string }>()

  const groups = useAsyncResource(
    () => api.experiment.listGroups(experiment.id),
    [experiment.id],
    (value) => (value ?? []).length === 0,
  )

  return (
    <PageSection
      title="小组编排"
      description={`每组 ${experiment.group_config?.size ?? 0} 人。同组成员共享同一套实验环境。`}
      actions={
        <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
          新建小组
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {(experiment.group_config?.roles ?? []).length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {(experiment.group_config?.roles ?? []).map((role) => (
              <Badge key={role} tone="jade">
                {role}
              </Badge>
            ))}
          </div>
        ) : (
          <Callout tone="info">
            实验没有定义角色,成员角色可以自由填写。要限定角色请在编排向导第 1 步配置。
          </Callout>
        )}

        <ResourceState
          resource={groups}
          emptyIcon={Users}
          emptyTitle="还没有小组"
          emptyDescription="建好小组并加入成员,学生进入实验时才会共享同一套环境。"
          emptyAction={
            <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              新建小组
            </Button>
          }
        >
          {(list) => (
            <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
              {list.map((group) => (
                // 每个小组自成一块抬起片:它们是并列的编排入口,不是表格行
                <div
                  key={group.id}
                  className="flex flex-col gap-2 rounded-lg bg-surface p-4 shadow-xs"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="min-w-0 truncate text-base text-ink">{group.name}</span>
                    <Badge
                      tone={
                        (experiment.group_config?.size ?? 0) > 0 &&
                        (group.members ?? []).length >= (experiment.group_config?.size ?? 0)
                          ? 'success'
                          : 'neutral'
                      }
                    >
                      {(group.members ?? []).length}
                      {(experiment.group_config?.size ?? 0) > 0
                        ? ` / ${experiment.group_config?.size}`
                        : ''}{' '}
                      人
                    </Badge>
                  </div>

                  {(group.members ?? []).length === 0 ? (
                    <p className="text-sm text-ink-sub">还没有成员。</p>
                  ) : (
                    <ul className="flex flex-col gap-1">
                      {(group.members ?? []).map((member) => (
                        <li key={member.id} className="flex items-center justify-between gap-2 text-sm">
                          <span className="min-w-0 truncate text-ink">{member.student_name}</span>
                          <span className="flex items-center gap-2">
                            <Badge tone="neutral">{member.role}</Badge>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              aria-label={`移除${member.student_name}`}
                              title="移除成员"
                              onClick={() => setRemoveTarget({ group, studentId: member.student_id, studentName: member.student_name })}
                            >
                              <Trash2 aria-hidden="true" />
                            </Button>
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}

                  <Button
                    variant="ghost"
                    size="sm"
                    leftIcon={UserPlus}
                    onClick={() => setMemberTarget(group)}
                  >
                    加入成员
                  </Button>
                </div>
              ))}
            </div>
          )}
        </ResourceState>
      </div>

      {createOpen ? (
        <CreateGroupModal
          experimentId={experiment.id}
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false)
            groups.reload()
          }}
        />
      ) : null}

      {memberTarget ? (
        <AddMemberModal
          group={memberTarget}
          roles={experiment.group_config?.roles ?? []}
          onClose={() => setMemberTarget(undefined)}
          onSaved={() => {
            setMemberTarget(undefined)
            groups.reload()
          }}
        />
      ) : null}
      {removeTarget ? (
        <RemoveMemberModal
          group={removeTarget.group}
          studentId={removeTarget.studentId}
          studentName={removeTarget.studentName}
          onClose={() => setRemoveTarget(undefined)}
          onRemoved={() => {
            setRemoveTarget(undefined)
            groups.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface RemoveMemberModalProps {
  group: ExperimentGroup
  studentId: string
  studentName: string
  onClose: () => void
  onRemoved: () => void
}

/** RemoveMemberModal 确认移除成员,让教师明确知道共享资源权限会被收回。 */
function RemoveMemberModal({ group, studentId, studentName, onClose, onRemoved }: RemoveMemberModalProps) {
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const remove = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      await api.experiment.removeGroupMember(group.id, studentId)
      toast.success('成员已移除')
      onRemoved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '移除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [group.id, onRemoved, studentId])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>确认移除成员</ModalTitle>
          <ModalDescription>
            将从「{group.name}」移除 {studentName},并立即收回其共享实验环境和仿真会话权限。
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="outline" onClick={onClose} disabled={working}>
            取消
          </Button>
          <Button type="button" variant="seal" onClick={() => void remove()} loading={working}>
            移除成员
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface CreateGroupModalProps {
  experimentId: string
  onClose: () => void
  onCreated: () => void
}

/**
 * CreateGroupModal 新建一个协作小组。
 */
function CreateGroupModal({ experimentId, onClose, onCreated }: CreateGroupModalProps) {
  const [name, setName] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请给小组起一个名字')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.experiment.createGroup(experimentId, { name: name.trim() })
        toast.success('小组已创建')
        onCreated()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '创建没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [experimentId, name, onCreated],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>新建小组</ModalTitle>
          <ModalDescription>创建后再加入成员并分配角色。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody>
            <FormField label="小组名称" htmlFor="group-name" required error={formError}>
              <Input
                id="group-name"
                value={name}
                placeholder="例如 第一组"
                invalid={Boolean(formError)}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              创建小组
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface AddMemberModalProps {
  group: ExperimentGroup
  roles: string[]
  onClose: () => void
  onSaved: () => void
}

/**
 * AddMemberModal 把学生加入小组并分配角色。
 * 已在组内的学生再提交即为调整角色(后端按 (组, 学生) 唯一约束做 upsert)。
 * 学生从班内名录里选:组织结构对教师只读开放,而账号目录是学校管理员能力,
 * 故先选班级再选人,不让教师端取全校账号(§安全边界)。
 */
function AddMemberModal({ group, roles, onClose, onSaved }: AddMemberModalProps) {
  const [classId, setClassId] = useState('')
  const [studentId, setStudentId] = useState('')
  const [role, setRole] = useState(roles[0] ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)

  /**
   * 班内学生一次取齐(PAGINATION_MAX_SIZE)。
   * 不给下拉配翻页控件:那等于让教师先翻页才能选到人,而下拉本身就该是可滚动的完整名单;
   * 浮层里也不出现分页 —— 需要分页说明它其实是一页(规范 §6.5.5 A)。
   */
  const students = useAsyncResource(
    () =>
      classId === ''
        ? Promise.resolve({
            list: [] as ClassStudent[],
            total: 0,
            page: 1,
            size: PAGINATION_MAX_SIZE,
          })
        : api.identity.listClassStudents(classId, { page: 1, size: PAGINATION_MAX_SIZE }),
    [classId],
    (value) => value.list.length === 0,
  )

  const classOptions = useMemo(
    () => (classes.data ?? []).map((item: Class) => ({ value: item.id, label: item.name })),
    [classes.data],
  )

  const studentOptions = useMemo(
    () =>
      (students.data?.list ?? []).map((student) => ({
        value: student.id,
        label: student.no ? `${student.name} · ${student.no}` : student.name,
      })),
    [students.data],
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (studentId === '') {
        setFormError('请选择要加入的学生')
        return
      }
      if (role.trim() === '') {
        setFormError('请填写这名学生在组内的角色')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.experiment.upsertGroupMember(group.id, {
          student_id: studentId,
          role: role.trim(),
        })
        toast.success('成员已加入')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '加入没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [group.id, onSaved, role, studentId],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>加入 {group.name}</ModalTitle>
          <ModalDescription>
            已在组内的学生再次提交会更新其角色。组满后无法继续加入。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <ResourceState
              resource={classes}
              emptyIcon={Users}
              emptyTitle="学校还没有建班级"
              emptyDescription="请联系学校管理员在组织架构里创建班级。"
            >
              {() => (
                <FormField label="班级" htmlFor="member-class" required>
                  <Select
                    id="member-class"
                    options={classOptions}
                    value={classId}
                    placeholder="选择班级"
                    onValueChange={(value) => {
                      setClassId(value)
                      setStudentId('')
                    }}
                  />
                </FormField>
              )}
            </ResourceState>

            {classId === '' ? (
              <p className="text-sm text-ink-sub">先选班级,再从班里挑要加入的学生。</p>
            ) : (
              <ResourceState
                resource={students}
                emptyIcon={Users}
                emptyTitle="这个班级没有在校学生"
                emptyDescription="换一个班级,或联系学校管理员核对班级名单。"
              >
                {() => (
                  <FormField label="学生" htmlFor="member-student" required>
                    <Select
                      id="member-student"
                      options={studentOptions}
                      value={studentId}
                      placeholder="选择学生"
                      onValueChange={setStudentId}
                    />
                  </FormField>
                )}
              </ResourceState>
            )}

            {roles.length > 0 ? (
              <FormField label="组内角色" htmlFor="member-role" required helper="只能选实验定义的角色">
                <Select
                  id="member-role"
                  options={roles.map((item) => ({ value: item, label: item }))}
                  value={role}
                  placeholder="选择角色"
                  onValueChange={setRole}
                />
              </FormField>
            ) : (
              <FormField
                label="组内角色"
                htmlFor="member-role-free"
                required
                helper="实验没有限定角色,填一个便于识别的名字"
              >
                <Input
                  id="member-role-free"
                  value={role}
                  placeholder="例如 攻击方"
                  onChange={(event) => setRole(event.target.value)}
                />
              </FormField>
            )}

            {group.members.length > 0 ? (
              <div className="well p-3">
                <p className="text-sm text-ink-sub">当前已有 {group.members.length} 名成员。</p>
              </div>
            ) : (
              <Empty icon={Users} title="这个组还是空的" description="加入第一名成员。" />
            )}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              加入小组
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
