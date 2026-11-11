// 我的课程(学生登录后的第一页,/student/courses)。设计 §7.2:
// 学生来这里是为了找到正在上的课进入学习,偶尔用邀请码加入新课;一学期 3–8 门。
// 所以主体是卡片网格(整卡链向课程详情),状态分段写进地址;网格末尾一张「加入新课程」小卡。
// 学习进度需要课时总数,只有单课程 outline 同时给出;逐门课请求会让一页发二十次请求,
// 因此进度只在课程详情显示(数据缺口见设计 §10,第 16 批补齐)。

import { useCallback, useEffect, useId, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { BookOpen, Plus, TicketCheck } from 'lucide-react'
import { CourseStatus, type Course } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Card,
  CoverImage,
  FormField,
  Icon,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
  cn,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal, useUrlState } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateRange } from '../../../../utils/formatters'
import { courseStatusLabel, courseTypeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { courseTypeCover } from '../../coursePresentation'
import { courseStatusTone } from '../../statusPresentation'

/** 状态分段;空串表示全部 */
const STATUS_SEGMENTS = [
  { value: String(CourseStatus.RUNNING), label: '进行中' },
  { value: String(CourseStatus.PUBLISHED), label: '待开课' },
  { value: String(CourseStatus.ENDED), label: '已结课' },
  { value: 'all', label: '全部' },
] as const

/** 地址参数:status 不传时由页面按「有进行中的课就看进行中,否则看全部」决定 */
const COURSES_QUERY = {
  status: { defaultValue: '', allowed: ['', ...STATUS_SEGMENTS.map((item) => item.value)] },
} as const

/** 每页课程数:一学期 3–8 门,一页放得下,超过一页才出分页 */
const PAGE_SIZE = 24

/**
 * StudentCoursesPage 呈现已加入课程,并提供邀请码加入入口。
 */
export default function StudentCoursesPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(COURSES_QUERY)
  const [joinOpen, setJoinOpen] = useState(false)
  const [highlightedIds, setHighlightedIds] = useState<ReadonlySet<string>>(new Set())

  // 默认分段:有进行中的课就看进行中,否则看全部(设计 §7.2);用服务端全量计数判断
  const runningCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'student', status: CourseStatus.RUNNING, ...params }),
    [],
    0,
  )
  const defaultStatus = runningCount === undefined ? undefined : runningCount > 0 ? String(CourseStatus.RUNNING) : 'all'
  const status = query.status || defaultStatus

  const courses = usePagedResource<Course>(
    (params) =>
      api.teaching.getCourses({
        role: 'student',
        status: status && status !== 'all' ? (Number(status) as CourseStatus) : undefined,
        ...params,
      }),
    [status ?? 'pending'],
    { pageSize: PAGE_SIZE, urlPageKey: 'page' },
  )

  /** 加入成功:加入接口不返回课程编号,无法直接跳转;刷新列表并短暂高亮新出现的课程 */
  const handleJoined = useCallback(() => {
    const before = new Set((courses.data?.list ?? []).map((course) => course.id))
    setJoinOpen(false)
    setQuery({ status: 'all' })
    courses.reload()
    setPendingHighlight(before)
  }, [courses, setQuery])
  const [pendingHighlight, setPendingHighlight] = useState<Set<string> | null>(null)
  useEffect(() => {
    if (!pendingHighlight || courses.status !== 'success' || !courses.data) return
    const fresh = courses.data.list.filter((course) => !pendingHighlight.has(course.id)).map((course) => course.id)
    setPendingHighlight(null)
    if (fresh.length === 0) return
    setHighlightedIds(new Set(fresh))
    const timer = window.setTimeout(() => setHighlightedIds(new Set()), 2400)
    return () => window.clearTimeout(timer)
  }, [courses.data, courses.status, pendingHighlight])

  const list = courses.data?.list ?? []
  const showGrid = courses.status === 'success' || (courses.status === 'loading' && list.length > 0)
  const filtered = status !== 'all'

  return (
    <PageScaffold>
      <PageHeader
        title="我的课程"
        actions={
          <Button variant="secondary" leftIcon={TicketCheck} onClick={() => setJoinOpen(true)}>
            加入课程
          </Button>
        }
      />

      <div className="mb-5">
        <SegmentedControl
          aria-label="按课程状态筛选"
          options={STATUS_SEGMENTS.map((item) => ({ value: item.value, label: item.label }))}
          value={status ?? ''}
          onValueChange={(value) => setQuery({ status: value })}
        />
      </div>

      {courses.status === 'error' ? (
        <ResourceError
          message={courses.error?.message ?? '课程暂时读取不到'}
          traceId={courses.error?.traceId}
          onRetry={courses.reload}
        />
      ) : courses.status === 'empty' ? (
        filtered && status !== undefined ? (
          <EmptyFiltered onShowAll={() => setQuery({ status: 'all' })} onJoin={() => setJoinOpen(true)} />
        ) : (
          <EmptyNoCourses onJoined={handleJoined} />
        )
      ) : showGrid ? (
        <>
          <ul className="card-grid gap-4" aria-busy={courses.status === 'loading' || undefined}>
            {list.map((course) => (
              <li key={course.id}>
                <CourseCard course={course} highlighted={highlightedIds.has(course.id)} onOpen={navigate} />
              </li>
            ))}
            <li>
              <button
                type="button"
                onClick={() => setJoinOpen(true)}
                className="pressable flex size-full min-h-40 flex-col items-center justify-center gap-2 rounded-lg bg-surface-hover text-ink-sub transition-colors duration-fast hover:bg-primary-soft hover:text-primary focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
              >
                <Icon icon={Plus} size="lg" />
                <span className="text-base font-medium">加入新课程</span>
                <span className="text-sm">用老师给的邀请码</span>
              </button>
            </li>
          </ul>
          <div className="mt-6">
            <Pagination page={courses.page} pageSize={courses.pageSize} total={courses.total} onPageChange={courses.setPage} />
          </div>
        </>
      ) : (
        <CourseGridSkeleton />
      )}

      <Modal open={joinOpen} onOpenChange={setJoinOpen}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>加入新课程</ModalTitle>
            <ModalDescription>老师会给出一串课程邀请码,填入后即可加入。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <JoinCourseForm onJoined={handleJoined} />
          </ModalBody>
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface CourseCardProps {
  course: Course
  highlighted: boolean
  onOpen: (path: string) => void
}

/**
 * CourseCard 一门课:纸材质封面条与课程类型题识(列表不逐行换取真实封面,规范 §1.3)、课程名、
 * 学期 · 学分 · 类型、状态、开课区间;整卡是一个链向课程详情的真实链接。
 */
function CourseCard({ course, highlighted, onOpen }: CourseCardProps) {
  const cover = courseTypeCover(course.type)
  const href = `/student/courses/${course.id}`
  return (
    <Card
      interactive
      className={cn('flex h-full flex-col overflow-hidden', highlighted && 'outline-2 outline-offset-2 outline-accent')}
    >
      <CoverImage
        id={course.id}
        name={course.name}
        glyph={cover.glyph}
        accent={cover.accent}
        className="aspect-auto h-24 rounded-none"
      />
      <div className="flex flex-1 flex-col gap-2 p-5">
        <div className="flex items-start justify-between gap-3">
          <Link
            to={href}
            onClick={(event) => {
              if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
              event.preventDefault()
              onOpen(href)
            }}
            className="card-link line-clamp-2 text-lg font-bold text-ink"
          >
            {course.name}
          </Link>
          <StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />
        </div>
        <p className="text-sm text-ink-sub">
          {course.semester} · {course.credits} 学分 · {courseTypeLabel(course.type)}
        </p>
        <p className="mt-auto pt-2 font-mono text-xs tabular-nums text-ink-sub">{formatDateRange(course.start_at, course.end_at)}</p>
      </div>
    </Card>
  )
}

/** CourseGridSkeleton 与真实卡片同形的骨架(规范 §6.5.3 三态) */
function CourseGridSkeleton() {
  return (
    <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取课程">
      {Array.from({ length: 3 }, (_, index) => (
        <li key={index} className="overflow-hidden rounded-lg bg-surface shadow-xs">
          <div className="h-24 skeleton-shimmer" />
          <div className="flex flex-col gap-3 p-5">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="line" lines={2} />
          </div>
        </li>
      ))}
    </ul>
  )
}

/** EmptyFiltered 筛选结果为空:这个状态下没有课,给「看全部课程」 */
function EmptyFiltered({ onShowAll, onJoin }: { onShowAll: () => void; onJoin: () => void }) {
  return (
    <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
      <p className="text-md font-semibold text-ink">这里暂时没有课程</p>
      <p className="mt-1.5 text-sm text-ink-sub">切换到「全部」看看已经加入的其他课程,或用邀请码加入新课。</p>
      <div className="mt-5 flex flex-wrap justify-center gap-3">
        <Button variant="outline" onClick={onShowAll}>
          看全部课程
        </Button>
        <Button variant="secondary" leftIcon={TicketCheck} onClick={onJoin}>
          加入课程
        </Button>
      </div>
    </div>
  )
}

/** EmptyNoCourses 还没有加入任何课程:整页空状态里直接放邀请码输入框(设计 §7.2) */
function EmptyNoCourses({ onJoined }: { onJoined: () => void }) {
  return (
    <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-16 text-center shadow-xs">
      <div className="mb-4 flex size-12 items-center justify-center rounded-full bg-primary-soft text-primary">
        <Icon icon={BookOpen} size="lg" />
      </div>
      <p className="text-lg font-bold text-ink">还没有加入课程</p>
      <p className="mt-1.5 max-w-md text-sm text-ink-sub">老师会给你一串邀请码,填入后即可加入课程开始学习。</p>
      <div className="mt-6 w-full max-w-sm text-left">
        <JoinCourseForm onJoined={onJoined} />
      </div>
    </div>
  )
}

interface JoinCourseFormProps {
  /** 加入成功后刷新课程列表 */
  onJoined: () => void
}

/**
 * JoinCourseForm 邀请码加入课程:只有邀请码一个字段,提交即完成,无跨步骤中间态,故不落草稿(FE-7)。
 */
function JoinCourseForm({ onJoined }: JoinCourseFormProps) {
  const fieldId = useId()
  const [inviteCode, setInviteCode] = useState('')
  const [submitError, setSubmitError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const { errors, validate, reset } = useFormValidation(() => ({
    inviteCode: !inviteCode.trim() ? '请输入课程邀请码' : null,
  }))

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const code = inviteCode.trim()
      if (!validate()) return
      setSubmitError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.joinCourse({ invite_code: code })
        setInviteCode('')
        reset()
        toast.success('已加入课程')
        onJoined()
      } catch (joinError) {
        setSubmitError(userFacingErrorMessage(joinError, '加入课程失败,请确认邀请码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [inviteCode, onJoined, reset, validate],
  )

  return (
    <form onSubmit={handleSubmit} noValidate>
      <FormField label="课程邀请码" htmlFor={fieldId} required error={errors.inviteCode}>
        <Input
          id={fieldId}
          value={inviteCode}
          autoComplete="off"
          spellCheck={false}
          placeholder="输入老师给的邀请码"
          invalid={Boolean(errors.inviteCode)}
          onChange={(event) => setInviteCode(event.target.value)}
          onBlur={() => validate(['inviteCode'])}
        />
      </FormField>
      {submitError ? <Callout tone="danger" className="mt-4">{submitError}</Callout> : null}
      <Button type="submit" variant="primary" leftIcon={TicketCheck} loading={submitting} className="mt-4 w-full">
        加入课程
      </Button>
    </form>
  )
}
