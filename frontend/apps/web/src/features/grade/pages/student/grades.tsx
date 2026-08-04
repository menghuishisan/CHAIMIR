// 成绩中心页(学生侧栏,/student/grades)。
// 学生编号取自服务端会话(useSession 的 /me),不接受任何客户端传参 ——
// 后端 normalizeReadableStudent 会二次校验,前端也不把编号放进可编辑位置。
//
// 课程名:M11 成绩汇总只回 course_id(它只聚合分数,课程档案归 M6),
// 故与 M6 学生课程列表在页面层做一次映射 —— 内部编号不进界面。
//
// 申诉只做提交:GET /grade-center/appeals 由 M11 定为教师/管理员能力,
// grade 模块也不发申诉状态通知,故学生侧不建申诉列表(对齐清单 §6.6)。

import { useCallback, useMemo, useState } from 'react'
import { Download, FileText, GraduationCap, Layers, MessageSquareWarning } from 'lucide-react'
import {
  TranscriptScope,
  type Course,
  type CourseGrade,
  type GradeSummary,
  type Semester,
} from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  Empty,
  FormField,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Select,
  Stat,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime, formatGpa, formatScore } from '../../../../utils/formatters'
import { transcriptScopeLabel } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 课程名映射一次取回的课程条数上限:后端分页上限 100,足以覆盖一名学生的在读与历史课程。 */
const COURSE_NAME_LOOKUP_SIZE = 100

/** GradeView 是成绩中心一次读齐的数据。 */
interface GradeView {
  semesters: Semester[]
  summary: GradeSummary
  history: GradeSummary[]
  /** 课程编号 → 课程档案,用于把成绩行显示成课程名而不是内部编号 */
  courses: Map<string, Course>
}

/**
 * StudentGradesPage 呈现 GPA、课程成绩明细,并提供申诉与成绩单入口。
 */
export default function StudentGradesPage() {
  const { me } = useSession()
  const studentId = me.account.id
  const [semesterId, setSemesterId] = useState<string>('')

  const view = useAsyncResource<GradeView>(
    () =>
      Promise.all([
        api.grade.listSemesters(),
        api.grade.studentGrades(studentId, semesterId || undefined),
        api.grade.studentGPA(studentId),
        api.teaching.getCourses({ role: 'student', page: 1, size: COURSE_NAME_LOOKUP_SIZE }),
      ]).then(([semesters, summary, history, courses]) => ({
        semesters,
        summary,
        history,
        courses: new Map(courses.list.map((course) => [course.id, course])),
      })),
    [semesterId, studentId],
    () => false,
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学业区' }, { label: '成绩中心' }]} />}
        title="成绩中心"
        description="这里是你的课程成绩、平均学分绩点与成绩单。对成绩有疑问可以在这里提出申诉。"
        icon={GraduationCap}
      />

      <ResourceState
        resource={view}
        emptyIcon={GraduationCap}
        emptyTitle="暂无成绩记录"
        emptyDescription="课程结课并由学校审核通过后,成绩会显示在这里。"
      >
        {(data) => (
          <GradeContent
            studentId={studentId}
            view={data}
            semesterId={semesterId}
            onSemesterChange={setSemesterId}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

interface GradeContentProps {
  studentId: string
  view: GradeView
  semesterId: string
  onSemesterChange: (semesterId: string) => void
}

/**
 * GradeContent 渲染指标带、成绩明细与右侧动作区。
 */
function GradeContent({ studentId, view, semesterId, onSemesterChange }: GradeContentProps) {
  const { semesters, summary, history, courses } = view

  // 学期筛选:空串表示全部学期(后端 semester 参数缺省即不按学期过滤)
  const semesterOptions = useMemo(
    () => [
      { value: '', label: '全部学期' },
      ...semesters.map((semester) => ({ value: semester.id, label: semester.name })),
    ],
    [semesters],
  )

  const columns: TableColumn<CourseGrade>[] = [
    {
      key: 'course_id',
      header: '课程',
      render: (grade) => {
        const course = courses.get(grade.course_id)
        return (
          <div className="min-w-0">
            <div className="truncate font-medium text-ink">{course ? course.name : '已结束的课程'}</div>
            {course ? <div className="truncate text-xs text-ink-sub">{course.semester}</div> : null}
          </div>
        )
      },
    },
    { key: 'credits', header: '学分', align: 'right', mono: true },
    {
      key: 'final_total',
      header: '总评成绩',
      align: 'right',
      mono: true,
      render: (grade) => (
        <span className="font-medium text-ink">{formatScore(grade.final_total)}</span>
      ),
    },
  ]

  return (
    <>
      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="本范围绩点"
            value={formatGpa(summary.gpa)}
            icon={GraduationCap}
            hint={semesterId ? '所选学期' : '全部学期'}
          />
          <Stat label="累计绩点" value={formatGpa(summary.cumulative_gpa)} icon={GraduationCap} />
          <Stat label="已修学分" value={summary.total_credits} icon={Layers} />
          <Stat label="课程数" value={summary.course_grades.length} icon={FileText} />
        </div>
      </PageSection>

      <PageBody
        rail={
          <div className="flex flex-col gap-4">
            <TranscriptCard studentId={studentId} semesters={semesters} />
            <AppealCard grades={summary.course_grades} courses={courses} />
          </div>
        }
      >
        <PageSection
          title="课程成绩"
          description={`共 ${summary.course_grades.length} 门课程 · 统计时间 ${formatDateTime(summary.computed_at)}`}
          actions={
            <SegmentedControl
              aria-label="按学期筛选成绩"
              size="sm"
              options={semesterOptions.slice(0, 4)}
              value={semesterId}
              onValueChange={onSemesterChange}
            />
          }
        >
          <Table
            columns={columns}
            data={summary.course_grades}
            rowKey={(grade) => grade.course_id}
            empty={
              <Empty
                icon={GraduationCap}
                title="这个范围内还没有成绩"
                description="换个学期看看,或等课程成绩审核通过后再来。"
              />
            }
          />
        </PageSection>

        {history.length > 0 ? (
          <PageSection title="各学期绩点" description="按学期结算的平均学分绩点。">
            <SemesterGpaList history={history} semesters={semesters} />
          </PageSection>
        ) : null}
      </PageBody>
    </>
  )
}

interface SemesterGpaListProps {
  history: GradeSummary[]
  semesters: Semester[]
}

/**
 * SemesterGpaList 列出各学期绩点。
 * 学期名从学期清单换取:汇总记录只带 semester_id,不把内部编号显示给用户。
 */
function SemesterGpaList({ history, semesters }: SemesterGpaListProps) {
  const semesterName = useMemo(
    () => new Map(semesters.map((semester) => [semester.id, semester.name])),
    [semesters],
  )

  return (
    <Card>
      <CardBody>
        <DescriptionList
          columns={2}
          items={history.map((item) => ({
            term: item.semester_id ? (semesterName.get(item.semester_id) ?? '学期成绩') : '全部学期',
            description: `绩点 ${formatGpa(item.gpa)} · ${item.total_credits} 学分`,
            mono: true,
          }))}
        />
      </CardBody>
    </Card>
  )
}

interface TranscriptCardProps {
  studentId: string
  semesters: Semester[]
}

/**
 * TranscriptCard 生成成绩单并取件。
 * 生成与下载是两步:生成落库产出记录,下载再换一次性授权(统一文件服务)。
 */
function TranscriptCard({ studentId, semesters }: TranscriptCardProps) {
  const [scope, setScope] = useState(String(TranscriptScope.FULL))
  const [semesterId, setSemesterId] = useState<string>('')
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const needsSemester = Number(scope) === TranscriptScope.SEMESTER

  /** generateAndDownload 生成成绩单后立即取件,用户只需点一次。 */
  const generateAndDownload = useCallback(async () => {
    if (needsSemester && !semesterId) {
      setActionError('请选择要生成的学期')
      return
    }
    setWorking(true)
    setActionError(undefined)
    try {
      const transcript = await api.grade.generateTranscript({
        student_id: studentId,
        scope: Number(scope) as TranscriptScope,
        semester_id: needsSemester ? semesterId : undefined,
      })
      const grant = await api.grade.downloadTranscript(transcript.id)
      const file = await api.storage.consumeGrant(grant.token)
      const url = URL.createObjectURL(file.blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = file.fileName
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success('成绩单已生成')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '成绩单生成失败,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [needsSemester, scope, semesterId, studentId])

  return (
    <Card>
      <CardHeader title="成绩单" description="生成后立即下载,文件带学校电子签章。" />
      <CardBody className="flex flex-col gap-4">
        <FormField label="成绩单范围" required>
          <SegmentedControl
            aria-label="成绩单范围"
            size="sm"
            options={[
              { value: String(TranscriptScope.FULL), label: transcriptScopeLabel(TranscriptScope.FULL) },
              { value: String(TranscriptScope.SEMESTER), label: transcriptScopeLabel(TranscriptScope.SEMESTER) },
            ]}
            value={scope}
            onValueChange={(value) => {
              setScope(value)
              setActionError(undefined)
            }}
          />
        </FormField>

        {needsSemester ? (
          <FormField label="学期" required>
            <Select
              options={semesters.map((semester) => ({ value: semester.id, label: semester.name }))}
              value={semesterId}
              placeholder="选择学期"
              onValueChange={setSemesterId}
            />
          </FormField>
        ) : null}

        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <Button variant="primary" leftIcon={Download} loading={working} onClick={() => void generateAndDownload()}>
          生成并下载
        </Button>
      </CardBody>
    </Card>
  )
}

interface AppealCardProps {
  grades: CourseGrade[]
  courses: Map<string, Course>
}

/**
 * AppealCard 提交成绩申诉。
 * 可申诉课程取自本人成绩明细(后端 validateAppealCourse 也要求成绩存在),
 * 选项显示课程名、值用课程编号 —— 用户不需要知道也不需要填内部编号。
 * 提交后只给就近反馈:申诉状态列表属教师/管理员能力,学生侧不显示进度(§6.6)。
 */
function AppealCard({ grades, courses }: AppealCardProps) {
  const [courseId, setCourseId] = useState<string>('')
  const [reason, setReason] = useState('')
  const [fieldError, setFieldError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)

  const courseOptions = useMemo(
    () =>
      grades.map((grade) => {
        const course = courses.get(grade.course_id)
        return {
          value: grade.course_id,
          label: course
            ? `${course.name} · 总评 ${formatScore(grade.final_total)}`
            : `已结束的课程 · 总评 ${formatScore(grade.final_total)}`,
        }
      }),
    [courses, grades],
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!courseId) {
        setFieldError('请选择要申诉的课程')
        return
      }
      if (reason.trim() === '') {
        setFieldError('请说明申诉理由')
        return
      }
      setFieldError(undefined)
      setSubmitting(true)
      try {
        await api.grade.submitAppeal({ course_id: courseId, reason: reason.trim() })
        setSubmitted(true)
        toast.success('申诉已提交')
      } catch (error) {
        setFieldError(userFacingErrorMessage(error, '申诉提交失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [courseId, reason],
  )

  if (submitted) {
    return (
      <Card>
        <CardHeader title="成绩申诉" />
        <CardBody>
          <Callout tone="success" title="申诉已提交">
            任课老师会核查你的成绩。处理结果以最终成绩为准,如需了解进度请直接联系老师。
          </Callout>
        </CardBody>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader title="成绩申诉" description="对成绩有疑问可以提出申诉,由任课老师核查。" />
      <CardBody>
        <form onSubmit={submit} noValidate>
          <FormField label="申诉课程" required>
            <Select
              options={courseOptions}
              value={courseId}
              placeholder={courseOptions.length > 0 ? '选择课程' : '暂无可申诉的课程'}
              disabled={courseOptions.length === 0}
              onValueChange={setCourseId}
            />
          </FormField>
          <FormField
            label="申诉理由"
            required
            error={fieldError}
            helper="说清楚你认为哪部分成绩有问题、依据是什么"
            className="mt-4"
          >
            <Textarea
              value={reason}
              rows={4}
              placeholder="请具体说明"
              invalid={Boolean(fieldError)}
              onChange={(event) => setReason(event.target.value)}
            />
          </FormField>
          <Button
            type="submit"
            variant="seal"
            leftIcon={MessageSquareWarning}
            loading={submitting}
            disabled={courseOptions.length === 0}
            className="mt-4 w-full"
          >
            提交申诉
          </Button>
        </form>
      </CardBody>
    </Card>
  )
}
