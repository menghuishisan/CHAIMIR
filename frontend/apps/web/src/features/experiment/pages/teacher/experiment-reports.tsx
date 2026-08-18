// 实验报告批改与小组编排(教师深页,/teacher/experiments/:experimentId/reports)。
//
// 两件事同页:批改学生提交的实验报告、给小组实验编排分组与角色。
// 它们都以同一个实验为上下文;编排向导负责"实验怎么配",本页负责"学生做完之后怎么处理"。
//
// 报告正文是对象存储引用,取件统一走 transfer/storage 授权,不暴露对象存储直链。
// 小组分区只在协作方式为小组实验时出现 —— 后端 CreateGroup 对独立完成的实验直接拒绝。

import { useCallback, useMemo, useState } from 'react'
import { useParams } from 'react-router'
import {
  ClipboardCheck,
  FileText,
  LayoutTemplate,
  Plus,
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
  Card,
  CardBody,
  CardHeader,
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
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Select,
  Stat,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import { experimentCollabModeLabel, experimentReportStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { experimentReportStatusTone } from '../../statusPresentation'

/**
 * TeacherExperimentReportsPage 读取实验本体并承载报告批改与分组编排。
 */
export default function TeacherExperimentReportsPage() {
  const { experimentId = '' } = useParams<{ experimentId: string }>()

  // 教师侧没有单个实验的读取接口,从 teacher 组列表里定位(该列表返回完整定义)
  const experiment = useAsyncResource(
    () =>
      api.experiment
        .getExperiments({ page: 1, size: PAGINATION_MAX_SIZE })
        .then((page) => page.list.find((item) => item.id === experimentId)),
    [experimentId],
    (value) => value === undefined,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={experiment}
        emptyIcon={LayoutTemplate}
        emptyTitle="实验不存在"
        emptyDescription="这个实验可能已被删除,请回到实验编排查看。"
      >
        {(data) => (data ? <ReportsContent experiment={data} /> : null)}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * ReportsContent 渲染实验头部、报告列表与小组编排。
 */
function ReportsContent({ experiment }: { experiment: Experiment }) {
  const [gradeTarget, setGradeTarget] = useState<ReportDTO>()
  const isGroup = experiment.collab_mode === ExperimentCollabMode.GROUP

  const reports = usePagedResource<ReportDTO>(
    (params) => api.experiment.listReports(experiment.id, params),
    [experiment.id],
  )

  // 指标带取服务端全量口径:待批改是教师最常看的数字,不能用当前页数出来的近似值
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

  const columns: TableColumn<ReportDTO>[] = [
    {
      key: 'student_id',
      header: '提交人',
      render: (report) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{report.student_name}</div>
          {report.student_no ? (
            <div className="truncate font-mono text-xs text-ink-sub">{report.student_no}</div>
          ) : null}
        </div>
      ),
    },
    {
      key: 'submitted_at',
      header: '提交时间',
      render: (report) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(report.submitted_at)}
        </span>
      ),
    },
    {
      key: 'manual_score',
      header: '批改得分',
      align: 'right',
      mono: true,
      render: (report) =>
        report.status === ExperimentReportStatus.GRADED ? formatScore(report.manual_score) : '—',
    },
    {
      key: 'comment',
      header: '批改意见',
      render: (report) =>
        report.comment ? (
          <span className="line-clamp-2 text-sm text-ink-sub">{report.comment}</span>
        ) : (
          <span className="text-ink-sub">尚未批改</span>
        ),
    },
    {
      key: 'status',
      header: '状态',
      render: (report) => (
        <StatusIndicator
          tone={experimentReportStatusTone(report.status)}
          label={experimentReportStatusLabel(report.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (report) => (
        <Button variant="ghost" size="sm" onClick={() => setGradeTarget(report)}>
          {report.status === ExperimentReportStatus.GRADED ? '修改评分' : '批改'}
        </Button>
      ),
    },
  ]

  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实践' },
              { label: '实验编排', href: '/teacher/experiments' },
              { label: experiment.name },
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

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="报告总数" value={totalCount ?? '—'} icon={FileText} />
          <Stat label="待批改" value={pendingCount ?? '—'} icon={ClipboardCheck} />
          <Stat label="已批改" value={gradedCount ?? '—'} icon={ClipboardCheck} />
          <Stat
            label="检查点合计"
            value={experiment.components.checkpoints.reduce((sum, item) => sum + item.score, 0)}
            icon={LayoutTemplate}
            hint="报告分与检查点分合计即实验得分"
          />
        </div>
      </PageSection>

      {experiment.require_report ? null : (
        <Callout tone="info">
          这个实验没有要求提交报告,学生的得分只来自检查点。要收报告请在编排向导第 1 步勾选。
        </Callout>
      )}

      <PageBody rail={isGroup ? <GroupsPanel experiment={experiment} /> : undefined}>
        <PageSection title="实验报告" description={`共 ${reports.total} 份报告`}>
          <ResourceState
            resource={reports}
            emptyIcon={FileText}
            emptyTitle="还没有报告"
            emptyDescription="学生在实验里提交报告后会出现在这里。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={reports.page}
                  pageSize={reports.pageSize}
                  total={reports.total}
                  onPageChange={reports.setPage}
                />
              </div>
            )}
          </ResourceState>
        </PageSection>
      </PageBody>

      {gradeTarget ? (
        <GradeReportModal
          report={gradeTarget}
          onClose={() => setGradeTarget(undefined)}
          onSaved={() => {
            setGradeTarget(undefined)
            reports.reload()
          }}
        />
      ) : null}
    </>
  )
}

interface GradeReportModalProps {
  report: ReportDTO
  onClose: () => void
  onSaved: () => void
}

/**
 * GradeReportModal 给单份报告打分写评语。
 * 报告正文是对象存储引用,取件走 storage 授权;这里只做评分,不在弹窗里内嵌阅读器。
 */
function GradeReportModal({ report, onClose, onSaved }: GradeReportModalProps) {
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
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>批改实验报告</ModalTitle>
          <ModalDescription>
            报告分会与检查点得分相加成为实验总分。批改意见会展示给学生。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '提交时间', description: formatDateTime(report.submitted_at), mono: true },
              {
                term: '当前状态',
                description: experimentReportStatusLabel(report.status),
              },
            ]}
          />

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
            helper="说清楚得分依据与改进方向,学生会在实验详情里看到"
          >
            <Textarea
              id="report-comment"
              value={comment}
              rows={5}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            提交批改
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * GroupsPanel 编排小组与成员角色。
 * 角色只能从实验定义的角色集合里选(后端 roleAllowed 同一口径);
 * 学生从账号档案里选,不让教师手填学生编号。
 */
function GroupsPanel({ experiment }: { experiment: Experiment }) {
  const [createOpen, setCreateOpen] = useState(false)
  const [memberTarget, setMemberTarget] = useState<ExperimentGroup>()

  const groups = useAsyncResource(
    () => api.experiment.listGroups(experiment.id),
    [experiment.id],
    (value) => (value ?? []).length === 0,
  )

  return (
    <Card>
      <CardHeader
        title="小组编排"
        description={`每组 ${experiment.group_config?.size ?? 0} 人。同组成员共享同一套实验环境。`}
        actions={
          <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            新建小组
          </Button>
        }
      />
      <CardBody className="flex flex-col gap-3">
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
            <div className="flex flex-col gap-3">
              {list.map((group) => (
                <div key={group.id} className="flex flex-col gap-2 well p-3">
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
                          <Badge tone="neutral">{member.role}</Badge>
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
      </CardBody>

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
    </Card>
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

  const students = useAsyncResource(
    () => (classId === '' ? Promise.resolve<ClassStudent[]>([]) : api.identity.listClassStudents(classId)),
    [classId],
    (value) => (value ?? []).length === 0,
  )

  const classOptions = useMemo(
    () => (classes.data ?? []).map((item: Class) => ({ value: item.id, label: item.name })),
    [classes.data],
  )

  const studentOptions = useMemo(
    () =>
      (students.data ?? []).map((student) => ({
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
