// 成绩中心页(学生侧栏,/student/grades)。设计 §7.2:
// 页头右侧「生成并下载成绩单」(secondary,打开范围选择弹窗)。上方一行:累计绩点、已修学分两个数字 +
// 各学期绩点柱状图(至少两个学期时才画,可切换数据表);下方学期筛选 + 成绩表(课程、学分、总评、
// 等级、绩点),每行末尾「申诉」小按钮,打开右侧抽屉,课程已预先选好,只需填理由。学期筛选写进地址。
//
// 学生编号取自服务端会话(useSession 的 /me),不接受任何客户端传参。
// 课程名:成绩汇总只回 course_id,与学生自己的课程列表做一次映射(一次请求,不逐行取大纲)。
// 等级与绩点:成绩接口只回总评,按学校默认等级配置换算(与后端绩点计算同一口径,见 gradeLevel.ts)。
// 申诉:每门课显示本人最近一次申诉的处理状态(申诉列表按身份分视角,学生只读本人);
// 还在处理中的课不能再申诉(服务端同样只允许一门课一条进行中的申诉),按钮换成状态。
// 成绩单由后端同步生成,生成后换一次性授权经统一文件服务下载,用户点一次即可。

import { useCallback, useMemo, useState } from 'react'
import { Download, GraduationCap, MessageSquareWarning } from 'lucide-react'
import {
  PAGINATION_DEFAULT_SIZE,
  PAGINATION_MAX_SIZE,
  TranscriptScope,
  GradeAppealStatus,
  type CourseGrade,
  type GradeAppeal,
  type GradeSummary,
  type Semester,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  ChartContainer,
  CompareBarChart,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  FormField,
  Icon,
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
  Select,
  Skeleton,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { collectAllPages } from '../../../../utils/allPages'
import { ResourceError } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { formatGpa, formatScore } from '../../../../utils/formatters'
import { gradeAppealStatusLabel, transcriptScopeLabel } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { levelForScore } from '../../gradeLevel'

/** 地址参数:semester 是学期编号(空为全部学期),page 是成绩表页码 */
const GRADES_QUERY = {
  semester: { defaultValue: '' },
  page: { defaultValue: '1' },
} as const

/** 学期筛选里「全部学期」的取值(Select 不接受空串) */
const ALL_SEMESTERS = 'all'

/**
 * StudentGradesPage 呈现绩点、课程成绩明细,并提供申诉与成绩单入口。
 */
export default function StudentGradesPage() {
  const { me } = useSession()
  const studentId = me.account.id
  const [query, setQuery] = useUrlState(GRADES_QUERY)
  const page = /^[1-9]\d*$/.test(query.page) ? Number(query.page) : 1
  const [transcriptOpen, setTranscriptOpen] = useState(false)
  const [appealCourse, setAppealCourse] = useState<CourseGrade>()

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)
  const history = useAsyncResource(() => api.grade.studentGPA(studentId), [studentId], () => false)
  const grades = useAsyncResource(
    () =>
      api.grade.studentGrades(studentId, {
        semester: query.semester || undefined,
        page,
        size: PAGINATION_DEFAULT_SIZE,
      }),
    [page, query.semester, studentId],
    () => false,
  )
  // 课程名与等级映射:读不到时课程列显示通用名、等级与绩点列显示「—」,不影响分数
  const courses = useAsyncResource(
    () => collectAllPages((params) => api.teaching.getCourses({ role: 'student', ...params }), PAGINATION_MAX_SIZE),
    [],
    () => false,
  )
  const levels = useAsyncResource(() => api.grade.listLevelConfigs(), [], () => false)
  // 本人申诉:按课程取最近一条;读不到时成绩照常显示,只是不显示申诉状态
  const appeals = useAsyncResource(() => collectAllPages((params) => api.grade.listAppeals(params), PAGINATION_MAX_SIZE), [], () => false)
  const latestAppeal = useMemo(() => latestAppealByCourse(appeals.data ?? []), [appeals.data])
  const courseName = useMemo(
    () => new Map((courses.data ?? []).map((course) => [course.id, course.name])),
    [courses.data],
  )
  const mapping = useMemo(() => levels.data?.find((config) => config.is_default)?.mapping ?? [], [levels.data])

  const semesterOptions = useMemo(
    () => [
      { value: ALL_SEMESTERS, label: '全部学期' },
      ...(semesters.data ?? []).map((semester) => ({ value: semester.id, label: semester.name })),
    ],
    [semesters.data],
  )

  const columns: TableColumn<CourseGrade>[] = [
    {
      key: 'course_id',
      header: '课程',
      render: (grade) => <span className="font-medium text-ink">{courseName.get(grade.course_id) ?? '已结束的课程'}</span>,
    },
    { key: 'credits', header: '学分', align: 'right', mono: true },
    {
      key: 'final_total',
      header: '总评',
      align: 'right',
      render: (grade) => <span className="font-semibold tabular-nums text-ink">{formatScore(grade.final_total)}</span>,
    },
    {
      key: 'level',
      header: '等级',
      render: (grade) => levelForScore(grade.final_total, mapping)?.grade ?? '—',
    },
    {
      key: 'gpa',
      header: '绩点',
      align: 'right',
      mono: true,
      render: (grade) => {
        const level = levelForScore(grade.final_total, mapping)
        return level ? formatGpa(level.gpa) : '—'
      },
    },
    {
      key: 'appeal',
      header: <span className="sr-only">申诉</span>,
      align: 'right',
      render: (grade) => (
        <AppealCell appeal={latestAppeal.get(grade.course_id)} onAppeal={() => setAppealCourse(grade)} />
      ),
    },
  ]

  const summary = grades.data
  const filtered = query.semester !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="成绩中心"
        actions={
          <Button variant="secondary" leftIcon={Download} onClick={() => setTranscriptOpen(true)}>
            生成并下载成绩单
          </Button>
        }
      />

      <OverviewRow summary={summary?.cumulative_gpa} credits={history.data} semesters={semesters.data ?? []} loading={history.status === 'loading'} />

      <section aria-labelledby="course-grades-title" className="mt-8">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h2 id="course-grades-title" className="text-md font-semibold text-ink">
            课程成绩
            {summary ? (
              <span className="ml-2 text-sm font-normal text-ink-sub">
                {filtered ? '本学期' : '全部'}绩点 {formatGpa(summary.gpa)} · {summary.total_credits} 学分
              </span>
            ) : null}
          </h2>
          <Select
            aria-label="按学期筛选"
            options={semesterOptions}
            value={query.semester || ALL_SEMESTERS}
            onValueChange={(value) => setQuery({ semester: value === ALL_SEMESTERS ? '' : value, page: '1' })}
            className="w-full sm:w-56"
          />
        </div>

        {grades.status === 'error' ? (
          <ResourceError
            message={grades.error?.message ?? '成绩暂时读取不到'}
            traceId={grades.error?.traceId}
            onRetry={grades.reload}
          />
        ) : (summary?.list.length ?? 0) === 0 && grades.status !== 'loading' ? (
          <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
            <Icon icon={GraduationCap} size="lg" className="text-ink-faint" />
            <p className="mt-3 text-md font-semibold text-ink">{filtered ? '这个学期还没有成绩' : '还没有成绩'}</p>
            <p className="mt-1.5 text-sm text-ink-sub">课程结课并由学校审核通过后,成绩会显示在这里。</p>
          </div>
        ) : (
          <>
            <Table
              columns={columns}
              data={summary?.list ?? []}
              rowKey={(grade) => grade.course_id}
              loading={grades.status === 'loading' && !summary}
              mobileCard={(grade) => {
                const level = levelForScore(grade.final_total, mapping)
                return {
                  title: courseName.get(grade.course_id) ?? '已结束的课程',
                  meta: `${grade.credits} 学分 · 总评 ${formatScore(grade.final_total)}${level ? ` · ${level.grade} · 绩点 ${formatGpa(level.gpa)}` : ''}`,
                  action: (
                    <AppealCell appeal={latestAppeal.get(grade.course_id)} onAppeal={() => setAppealCourse(grade)} />
                  ),
                }
              }}
            />
            {summary ? (
              <div className="mt-4">
                <Pagination
                  page={page}
                  pageSize={summary.size}
                  total={summary.total}
                  onPageChange={(next) => setQuery({ page: String(next) })}
                />
              </div>
            ) : null}
          </>
        )}
      </section>

      <TranscriptModal open={transcriptOpen} onOpenChange={setTranscriptOpen} studentId={studentId} semesters={semesters.data ?? []} />
      <AppealDrawer
        grade={appealCourse}
        courseName={appealCourse ? courseName.get(appealCourse.course_id) : undefined}
        onClose={() => setAppealCourse(undefined)}
        onSubmitted={appeals.reload}
      />
    </PageScaffold>
  )
}

/**
 * OverviewRow 上方一行:累计绩点、已修学分 + 各学期绩点柱状图(至少两个学期才画)。
 * 已修学分取各学期结算学分之和;学期名从学期清单换取,不显示内部编号。
 */
function OverviewRow({
  summary,
  credits,
  semesters,
  loading,
}: {
  summary: number | undefined
  credits: GradeSummary[] | null
  semesters: Semester[]
  loading: boolean
}) {
  const semesterName = useMemo(() => new Map(semesters.map((semester) => [semester.id, semester.name])), [semesters])
  const perSemester = (credits ?? []).filter((item) => item.semester_id)
  const totalCredits = perSemester.reduce((sum, item) => sum + item.total_credits, 0)
  const rows = perSemester.map((item) => ({
    semester: semesterName.get(item.semester_id ?? '') ?? '未登记学期',
    gpa: Number(item.gpa.toFixed(2)),
  }))
  const best = rows.reduce<(typeof rows)[number] | undefined>((top, row) => (!top || row.gpa > top.gpa ? row : top), undefined)

  if (loading) {
    return (
      <div aria-busy="true" className="grid gap-4 lg:grid-cols-3">
        <Skeleton variant="stat" />
        <Skeleton variant="block" className="lg:col-span-2" />
      </div>
    )
  }

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <dl className="grid grid-cols-2 gap-4 rounded-lg bg-surface p-5 shadow-xs lg:grid-cols-1">
        <div>
          <dt className="text-sm text-ink-sub">累计平均学分绩点</dt>
          <dd className="mt-1 text-stat font-bold tabular-nums text-ink">{summary !== undefined ? formatGpa(summary) : '—'}</dd>
        </div>
        <div>
          <dt className="text-sm text-ink-sub">已修学分</dt>
          <dd className="mt-1 text-stat font-bold tabular-nums text-ink">{perSemester.length > 0 ? totalCredits : '—'}</dd>
        </div>
      </dl>
      {rows.length >= 2 ? (
        <div className="lg:col-span-2">
          <ChartContainer
            title="各学期绩点"
            height={200}
            ariaSummary={`共 ${rows.length} 个学期:${rows.map((row) => `${row.semester} 绩点 ${row.gpa}`).join(';')}。${best ? `绩点最高的是 ${best.semester}(${best.gpa})。` : ''}`}
            dataTable={{ columns: ['学期', '绩点'], rows: rows.map((row) => [row.semester, row.gpa]) }}
          >
            <CompareBarChart data={rows} xKey="semester" series={[{ key: 'gpa', name: '平均学分绩点' }]} />
          </ChartContainer>
        </div>
      ) : null}
    </div>
  )
}

/** TranscriptModal 选择成绩单范围(全部 / 某一学期),生成后立即下载 */
function TranscriptModal({
  open,
  onOpenChange,
  studentId,
  semesters,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  studentId: string
  semesters: Semester[]
}) {
  const [scope, setScope] = useState(String(TranscriptScope.FULL))
  const [semesterId, setSemesterId] = useState('')
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const needsSemester = Number(scope) === TranscriptScope.SEMESTER
  const { errors, validate } = useFormValidation(() => ({
    semesterId: needsSemester && !semesterId ? '请选择要生成的学期' : null,
  }))

  /** generateAndDownload 生成成绩单后立即取件(一次性授权,经统一文件服务) */
  const generateAndDownload = useCallback(async () => {
    if (!validate()) return
    setWorking(true)
    setActionError(undefined)
    try {
      const transcript = await api.grade.generateTranscript({
        student_id: studentId,
        scope: Number(scope) as TranscriptScope,
        semester_id: needsSemester ? semesterId : undefined,
      })
      const grant = await api.grade.downloadTranscript(transcript.id)
      downloadAttachment(await api.storage.consumeGrant(grant.token))
      toast.success('成绩单已生成,正在下载')
      onOpenChange(false)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '成绩单没有生成成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [needsSemester, onOpenChange, scope, semesterId, studentId, validate])

  return (
    <Modal open={open} onOpenChange={(next) => (working ? undefined : onOpenChange(next))}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>生成成绩单</ModalTitle>
          <ModalDescription>成绩单带学校电子签章,生成后会自动下载。</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="范围" required>
            <SegmentedControl
              aria-label="成绩单范围"
              size="sm"
              options={[
                { value: String(TranscriptScope.FULL), label: transcriptScopeLabel(TranscriptScope.FULL) },
                { value: String(TranscriptScope.SEMESTER), label: transcriptScopeLabel(TranscriptScope.SEMESTER) },
              ]}
              value={scope}
              onValueChange={setScope}
            />
          </FormField>
          {needsSemester ? (
            <FormField label="学期" htmlFor="student-transcript-semester" required error={errors.semesterId}>
              <Select
                id="student-transcript-semester"
                options={semesters.map((semester) => ({ value: semester.id, label: semester.name }))}
                value={semesterId}
                placeholder="选择学期"
                onValueChange={setSemesterId}
                invalid={Boolean(errors.semesterId)}
                onBlur={() => validate(['semesterId'])}
              />
            </FormField>
          ) : null}
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button variant="primary" leftIcon={Download} loading={working} onClick={() => void generateAndDownload()}>
            生成并下载
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * AppealDrawer 对一门课的成绩提出申诉:课程已预先选好,只需填理由。
 * 可申诉范围以本人成绩明细为准(后端同样校验成绩存在与申诉期限);提交后这门课的状态换成「待处理」。
 */
function AppealDrawer({
  grade,
  courseName,
  onClose,
  onSubmitted,
}: {
  grade: CourseGrade | undefined
  courseName: string | undefined
  onClose: () => void
  onSubmitted: () => void
}) {
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string>()
  const { errors, validate } = useFormValidation(() => ({ reason: !reason.trim() ? '请说明申诉理由' : null }))

  const close = () => {
    if (submitting) return
    setReason('')
    setSubmitError(undefined)
    onClose()
  }

  const submit = useCallback(async () => {
    if (!grade || !validate()) return
    setSubmitting(true)
    setSubmitError(undefined)
    try {
      await api.grade.submitAppeal({ course_id: grade.course_id, reason: reason.trim() })
      toast.success('申诉已提交,处理结果会显示在这门课的成绩旁')
      onSubmitted()
      setReason('')
      onClose()
    } catch (error) {
      setSubmitError(userFacingErrorMessage(error, '申诉没有提交成功,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [grade, onClose, onSubmitted, reason, validate])

  return (
    <Drawer open={grade !== undefined} onOpenChange={(open) => (open ? undefined : close())}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>申诉成绩</DrawerTitle>
          <DrawerDescription>任课老师会核查你的成绩,处理结果会显示在这门课的成绩旁,也会发站内通知。</DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-4">
          {grade ? (
            <div className="rounded-md bg-surface-hover p-4">
              <p className="text-base font-semibold text-ink">{courseName ?? '已结束的课程'}</p>
              <p className="mt-1 text-sm text-ink-sub">
                {grade.credits} 学分 · 总评 {formatScore(grade.final_total)}
              </p>
            </div>
          ) : null}
          <FormField
            label="申诉理由"
            htmlFor="appeal-reason"
            required
            error={errors.reason}
            helper="说清楚你认为哪部分成绩有问题、依据是什么"
          >
            <Textarea
              id="appeal-reason"
              value={reason}
              rows={6}
              placeholder="例如:第三次作业已按时提交,成绩里显示为缺交"
              invalid={Boolean(errors.reason)}
              onChange={(event) => setReason(event.target.value)}
              onBlur={() => validate(['reason'])}
            />
          </FormField>
          {submitError ? <Callout tone="danger">{submitError}</Callout> : null}
        </DrawerBody>
        <DrawerFooter>
          <Button variant="outline" disabled={submitting} onClick={close}>
            取消
          </Button>
          <Button variant="primary" leftIcon={MessageSquareWarning} loading={submitting} onClick={() => void submit()}>
            提交申诉
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}

/** latestAppealByCourse 每门课只留本人最近一次申诉(列表已按提交时间倒序) */
function latestAppealByCourse(appeals: readonly GradeAppeal[]): Map<string, GradeAppeal> {
  const latest = new Map<string, GradeAppeal>()
  for (const appeal of appeals) {
    if (!latest.has(appeal.course_id)) latest.set(appeal.course_id, appeal)
  }
  return latest
}

/** 申诉状态的提示色:处理中为警示,通过或已改分为成功,未通过为中性 */
const APPEAL_TONE: Record<GradeAppealStatus, 'warning' | 'success' | 'neutral'> = {
  [GradeAppealStatus.PENDING]: 'warning',
  [GradeAppealStatus.ACCEPTED]: 'warning',
  [GradeAppealStatus.COMPLETED]: 'success',
  [GradeAppealStatus.REJECTED]: 'neutral',
}

/**
 * AppealCell 成绩行末尾的申诉位:没申诉过或上一次已结束时给「申诉」按钮;
 * 还在处理中只显示状态(不能重复申诉);已结束的同时显示上一次的结果,处理意见悬停可见。
 */
function AppealCell({ appeal, onAppeal }: { appeal: GradeAppeal | undefined; onAppeal: () => void }) {
  const active = appeal?.status === GradeAppealStatus.PENDING || appeal?.status === GradeAppealStatus.ACCEPTED
  return (
    <span className="inline-flex items-center justify-end gap-2">
      {appeal ? (
        <span title={appeal.result_comment || undefined}>
          <StatusIndicator tone={APPEAL_TONE[appeal.status]} label={`申诉${gradeAppealStatusLabel(appeal.status)}`} />
        </span>
      ) : null}
      {active ? null : (
        <Button variant="ghost" size="sm" onClick={onAppeal}>
          {appeal ? '再次申诉' : '申诉'}
        </Button>
      )}
    </span>
  )
}
