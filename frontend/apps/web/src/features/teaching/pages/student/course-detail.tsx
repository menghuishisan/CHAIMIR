// 学生课程详情(/student/courses/:courseId)。设计 §7.2:
// 学生进课是为了接着学、看有没有要交的作业。一门课 2–20 章、5–60 个课时、3–10 份作业。
// 页头 = 课程名 + 状态 + 一行属性;下方「下一步」卡(进度环 + 下一个课时 + 主按钮 + 累计学习时长);
// 再往下左主右次:左栏学习路径(按章分段,每节一行,整行链向课时),右栏近期截止、课程公告、讨论入口;
// 结课后右栏多一张课程评价。标签「学习 / 作业 / 讨论与公告」写进地址。
// 授课教师姓名随课程大纲返回;作业清单带本人提交情况,所以右栏是「待完成」:还没截止、本人还没交过的作业。

import { useMemo } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  BookOpen,
  ChevronRight,
  ClipboardList,
  Megaphone,
  MessageSquare,
  Timer,
} from 'lucide-react'
import {
  LessonContentType,
  ProgressStatus,
  type Assignment,
  type CourseOutline,
  type Lesson,
  type TeachingAnnouncement,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Card,
  CardHeader,
  Empty,
  Icon,
  ListRow,
  PageBody,
  PageHeader,
  PageScaffold,
  Pagination,
  ProgressRing,
  Skeleton,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  cn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import {
  locationLabel,
  useAsyncResource,
  usePageLocation,
  usePagedResource,
  useUrlState,
} from '../../../../hooks'
import {
  formatDateRange,
  formatDateTime,
  formatDuration,
  formatRelativeDeadline,
  formatRelativeOccurrence,
} from '../../../../utils/formatters'
import {
  courseStatusLabel,
  courseTypeLabel,
  latePolicyLabel,
  submissionStatusLabel,
  lessonContentTypeLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { orderedLessons, pickNextLesson } from '../../learningPath'
import { LESSON_TYPE_ICON, LessonStatusIcon } from '../../components/LessonIcons'
import { ASSIGNMENT_DUE_TONE, courseStatusTone } from '../../statusPresentation'
import { CourseDiscussion } from './course-discussion'
import { CourseReviewCard } from './course-review'

/** 标签页;当前标签写进地址 */
const COURSE_QUERY = {
  tab: { defaultValue: 'learn', allowed: ['learn', 'assignments', 'discussion'] },
} as const

/** 「下一步」主按钮按课时类型的说法(设计 §7.2) */
function primaryActionText(lesson: Lesson, status: ProgressStatus): string {
  if (lesson.content_type === LessonContentType.EXPERIMENT) return '去做实验'
  if (lesson.content_type === LessonContentType.SIMULATION) return '进入推演'
  return status === ProgressStatus.IN_PROGRESS ? '继续学习' : '开始学习'
}

/**
 * StudentCourseDetailPage 读取课程大纲(课程、章、课时、本人进度一次回齐)。
 */
export default function StudentCourseDetailPage() {
  const { courseId = '' } = useParams<{ courseId: string }>()
  const outline = useAsyncResource(() => api.teaching.getCourseOutline(courseId), [courseId], () => false)

  usePageLocation([{ label: locationLabel(outline, (data) => data.course.name, '课程详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={outline}
        emptyIcon={BookOpen}
        emptyTitle="课程内容暂未开放"
        emptyDescription="老师还没有发布章节课时,发布后会显示在这里。"
        skeleton={<CourseDetailSkeleton />}
      >
        {(data) => <CourseDetailContent courseId={courseId} outline={data} />}
      </ResourceState>
    </PageScaffold>
  )
}

/** CourseDetailSkeleton 与真实页面同形:页头、下一步卡、左右两栏 */
function CourseDetailSkeleton() {
  return (
    <div className="flex flex-col gap-6" aria-busy="true">
      <div className="flex flex-col gap-3">
        <Skeleton variant="title" width="half" />
        <Skeleton variant="line" width="third" />
      </div>
      <Skeleton variant="block" />
      <div className="flex flex-col gap-6 lg:flex-row">
        <div className="flex-1">
          <Skeleton variant="line" lines={6} />
        </div>
        <div className="lg:w-80">
          <Skeleton variant="block" />
        </div>
      </div>
    </div>
  )
}

interface CourseDetailContentProps {
  courseId: string
  outline: CourseOutline
}

/**
 * CourseDetailContent 渲染页头、下一步卡与三个标签页。
 */
function CourseDetailContent({ courseId, outline }: CourseDetailContentProps) {
  const course = outline.course
  const [query, setQuery] = useUrlState(COURSE_QUERY)
  const path = useMemo(() => orderedLessons(outline), [outline])
  const lessonById = useMemo(() => new Map(outline.lessons.map((lesson) => [lesson.id, lesson])), [outline.lessons])
  const doneCount = path.filter((row) => row.status === ProgressStatus.DONE).length
  const learnedSeconds = path.reduce((sum, row) => sum + row.durationSec, 0)

  return (
    <>
      <PageHeader
        title={course.name}
        status={<StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />}
        meta={[
          ...(course.teacher_name ? [{ value: `${course.teacher_name} 老师` }] : []),
          { value: course.semester },
          { value: `${course.credits} 学分` },
          { value: courseTypeLabel(course.type) },
          { value: teachingDifficultyLabel(course.difficulty) },
          { value: formatDateRange(course.start_at, course.end_at) },
        ]}
        description={course.description || undefined}
      />

      <NextStepCard
        courseId={courseId}
        outline={outline}
        done={doneCount}
        total={path.length}
        learnedSeconds={learnedSeconds}
        nextLesson={(() => {
          const next = pickNextLesson(path)
          return next ? { lesson: lessonById.get(next.lessonId), status: next.status as ProgressStatus } : null
        })()}
        firstLesson={path[0] ? lessonById.get(path[0].lessonId) : undefined}
      />

      <Tabs value={query.tab} onValueChange={(tab) => setQuery({ tab })} className="mt-8">
        <TabsList aria-label="课程内容">
          <TabsTrigger value="learn">学习</TabsTrigger>
          <TabsTrigger value="assignments">作业</TabsTrigger>
          <TabsTrigger value="discussion">讨论与公告</TabsTrigger>
        </TabsList>

        <TabsContent value="learn" className="mt-5">
          <PageBody
            rail={
              <div className="flex flex-col gap-4">
                <UpcomingAssignments courseId={courseId} onShowAll={() => setQuery({ tab: 'assignments' })} />
                <LatestAnnouncements courseId={courseId} onShowAll={() => setQuery({ tab: 'discussion' })} />
                <DiscussionEntry onOpen={() => setQuery({ tab: 'discussion' })} />
                <CourseReviewCard courseId={courseId} courseStatus={course.status} />
              </div>
            }
          >
            <LearningPath courseId={courseId} outline={outline} />
          </PageBody>
        </TabsContent>

        <TabsContent value="assignments" className="mt-5">
          <AllAssignments courseId={courseId} />
        </TabsContent>

        <TabsContent value="discussion" className="mt-5">
          <CourseDiscussion courseId={courseId} />
        </TabsContent>
      </Tabs>
    </>
  )
}

interface NextStepCardProps {
  courseId: string
  outline: CourseOutline
  done: number
  total: number
  learnedSeconds: number
  nextLesson: { lesson: Lesson | undefined; status: ProgressStatus } | null
  firstLesson: Lesson | undefined
}

/**
 * NextStepCard「下一步」:左侧进度环(已学完课时 / 全部课时),中间下一个课时的章节、类型、名称,
 * 右侧主按钮与累计学习时长。全部学完时显示「全部课时已学完」,主按钮变「复习」(进入第一课时)。
 */
function NextStepCard({ courseId, outline, done, total, learnedSeconds, nextLesson, firstLesson }: NextStepCardProps) {
  const navigate = useNavigate()
  const chapterById = new Map(outline.chapters.map((chapter) => [chapter.id, chapter]))
  const lesson = nextLesson?.lesson
  const allDone = total > 0 && nextLesson === null

  if (total === 0) {
    return (
      <Card className="p-6">
        <p className="text-base font-semibold text-ink">老师还没有发布课时</p>
        <p className="mt-1 text-sm text-ink-sub">发布后,这里会告诉你从哪一节开始。</p>
      </Card>
    )
  }

  const target = lesson ?? firstLesson
  const targetHref = target ? `/student/courses/${courseId}/lessons/${target.id}` : undefined

  return (
    <Card className="flex flex-col gap-5 p-5 sm:flex-row sm:items-center sm:p-6">
      <ProgressRing done={done} total={total} label="课时完成进度" />
      <div className="min-w-0 flex-1">
        {allDone ? (
          <>
            <p className="text-sm text-ink-sub">已学完 {done} / {total} 个课时</p>
            <p className="mt-1 text-lg font-bold text-ink">全部课时已学完</p>
          </>
        ) : lesson ? (
          <>
            <p className="text-sm text-ink-sub">
              {nextLesson?.status === ProgressStatus.IN_PROGRESS ? '上次学到' : '下一节'} ·{' '}
              {chapterById.get(lesson.chapter_id)?.title ?? '未分章'} · {lessonContentTypeLabel(lesson.content_type)}
            </p>
            <p className="mt-1 truncate text-lg font-bold text-ink" title={lesson.title}>
              {lesson.title}
            </p>
            <p className="mt-1 text-sm text-ink-sub">已学完 {done} / {total} 个课时</p>
          </>
        ) : null}
      </div>
      <div className="flex flex-col gap-2 sm:items-end">
        {targetHref ? (
          <Button
            variant="primary"
            size="lg"
            onClick={() => navigate(targetHref)}
            className="w-full sm:w-auto"
          >
            {allDone || !lesson ? '复习' : primaryActionText(lesson, nextLesson?.status ?? ProgressStatus.NOT_STARTED)}
          </Button>
        ) : null}
        <span className="flex items-center gap-1.5 text-sm text-ink-sub">
          <Icon icon={Timer} size="sm" />
          累计学习 {formatDuration(learnedSeconds)}
        </span>
      </div>
    </Card>
  )
}

/**
 * LearningPath 学习路径:按章分段,每个课时一行(状态图标、名称、类型、学习时长或「未开始」),整行链向课时。
 */
function LearningPath({ courseId, outline }: { courseId: string; outline: CourseOutline }) {
  const navigate = useNavigate()
  const progressByLesson = new Map(outline.progress.map((item) => [item.lesson_id, item]))
  const chapters = [...outline.chapters].sort((a, b) => a.sort - b.sort)
  const lessonsByChapter = new Map<string, Lesson[]>()
  for (const lesson of outline.lessons) {
    const list = lessonsByChapter.get(lesson.chapter_id) ?? []
    list.push(lesson)
    lessonsByChapter.set(lesson.chapter_id, list)
  }

  if (outline.lessons.length === 0) {
    return <Empty icon={BookOpen} title="还没有课时" description="老师发布章节课时后会显示在这里。" />
  }

  return (
    <section aria-label="学习路径" className="flex flex-col gap-5">
      {chapters.map((chapter, chapterIndex) => {
        const lessons = [...(lessonsByChapter.get(chapter.id) ?? [])].sort((a, b) => a.sort - b.sort)
        if (lessons.length === 0) return null
        const chapterDone = lessons.filter((lesson) => progressByLesson.get(lesson.id)?.status === ProgressStatus.DONE).length
        return (
          <div key={chapter.id} className="overflow-hidden rounded-lg bg-surface shadow-xs">
            <div className="flex items-baseline justify-between gap-3 px-5 pb-2 pt-4">
              <h2 className="min-w-0 truncate text-md font-bold text-ink">
                <span className="mr-2 text-ink-sub">第 {chapterIndex + 1} 章</span>
                {chapter.title}
              </h2>
              <span className="shrink-0 text-sm tabular-nums text-ink-sub">
                {chapterDone} / {lessons.length}
              </span>
            </div>
            <ul className="divide-y divide-line">
              {lessons.map((lesson) => {
                const progress = progressByLesson.get(lesson.id)
                const status = progress ? progress.status : ProgressStatus.NOT_STARTED
                return (
                  <li key={lesson.id}>
                    <ListRow
                      href={`/student/courses/${courseId}/lessons/${lesson.id}`}
                      onNavigate={navigate}
                      leading={<LessonStatusIcon status={status} />}
                      title={lesson.title}
                      meta={
                        <span className="inline-flex items-center gap-1.5">
                          <Icon icon={LESSON_TYPE_ICON[lesson.content_type]} size="xs" />
                          {lessonContentTypeLabel(lesson.content_type)}
                        </span>
                      }
                      trailing={
                        <span className="text-sm tabular-nums text-ink-sub">
                          {status === ProgressStatus.NOT_STARTED ? '未开始' : formatDuration(progress?.duration_sec ?? 0)}
                        </span>
                      }
                    />
                  </li>
                )
              })}
            </ul>
          </div>
        )
      })}
    </section>
  )
}

/** 右栏待完成最多显示的作业数 */
const UPCOMING_LIMIT = 3

/**
 * UpcomingAssignments 右栏「待完成」:还没截止、本人还没交过的作业,按截止时间取前 3 条,显示倒计时与「去作答」。
 * 交过的作业不再出现在这里(去「作业」标签看结果)。
 * 一门课 3–10 份作业,一页 100 条读得下,排序在前端做。
 */
function UpcomingAssignments({ courseId, onShowAll }: { courseId: string; onShowAll: () => void }) {
  const navigate = useNavigate()
  const assignments = useAsyncResource(
    () => api.teaching.listCourseAssignments(courseId, { page: 1, size: 100 }),
    [courseId],
    () => false,
  )
  const upcoming = useMemo(() => {
    const now = Date.now()
    return (assignments.data?.list ?? [])
      .filter((item) => new Date(item.due_at).getTime() > now && (item.my_submission?.attempts ?? 0) === 0)
      .sort((a, b) => new Date(a.due_at).getTime() - new Date(b.due_at).getTime())
      .slice(0, UPCOMING_LIMIT)
  }, [assignments.data])

  return (
    <Card>
      <CardHeader
        title="待完成"
        count={assignments.status === 'success' ? upcoming.length : undefined}
        actions={
          <Button variant="ghost" size="sm" onClick={onShowAll}>
            全部作业
          </Button>
        }
      />
      <div className="px-3 pb-3">
        {assignments.status === 'loading' ? (
          <div className="px-2 pb-2">
            <Skeleton variant="line" lines={2} />
          </div>
        ) : assignments.status === 'error' ? (
          <p className="px-2 pb-2 text-sm text-ink-sub">
            作业暂时读不到。
            <button type="button" onClick={assignments.reload} className="ml-1 text-primary hover:underline">
              重试
            </button>
          </p>
        ) : upcoming.length === 0 ? (
          <p className="px-2 pb-2 text-sm text-ink-sub">没有待完成的作业。</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {upcoming.map((assignment) => {
              const deadline = formatRelativeDeadline(assignment.due_at)
              const href = `/student/courses/${courseId}/assignments/${assignment.id}`
              return (
                <li key={assignment.id} className="rounded-md bg-surface-hover px-3 py-3">
                  <Link to={href} className="block truncate text-base font-semibold text-ink hover:text-primary">
                    {assignment.title}
                  </Link>
                  <div className="mt-1.5 flex flex-wrap items-center gap-2 text-sm text-ink-sub">
                    <Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]}>{deadline.text}</Badge>
                    <span>可交 {assignment.max_attempts} 次</span>
                  </div>
                  <Button variant="secondary" size="sm" className="mt-2.5" onClick={() => navigate(href)}>
                    去作答
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </Card>
  )
}

/** 公告标为「新」的时间窗口(毫秒) */
const NEW_ANNOUNCEMENT_MS = 3 * 86_400_000

/** LatestAnnouncements 右栏「课程公告」:最新 2 条,三天内的带「新」 */
function LatestAnnouncements({ courseId, onShowAll }: { courseId: string; onShowAll: () => void }) {
  const announcements = useAsyncResource(
    () => api.teaching.listAnnouncements(courseId, { page: 1, size: 2 }),
    [courseId],
    () => false,
  )
  const list: TeachingAnnouncement[] = announcements.data?.list ?? []

  return (
    <Card>
      <CardHeader
        title="课程公告"
        actions={
          <Button variant="ghost" size="sm" onClick={onShowAll}>
            全部
          </Button>
        }
      />
      <div className="px-5 pb-4">
        {announcements.status === 'loading' ? (
          <Skeleton variant="line" lines={2} />
        ) : announcements.status === 'error' ? (
          <p className="text-sm text-ink-sub">
            公告暂时读不到。
            <button type="button" onClick={announcements.reload} className="ml-1 text-primary hover:underline">
              重试
            </button>
          </p>
        ) : list.length === 0 ? (
          <p className="text-sm text-ink-sub">老师还没有发布公告。</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {list.map((item) => {
              const fresh = Date.now() - new Date(item.created_at).getTime() < NEW_ANNOUNCEMENT_MS
              return (
                <li key={item.id}>
                  <p className="flex items-center gap-2">
                    <Icon icon={Megaphone} size="xs" className="shrink-0 text-primary" />
                    <span className="min-w-0 truncate text-base font-medium text-ink">{item.title}</span>
                    {fresh ? <Badge tone="jade">新</Badge> : null}
                  </p>
                  <p className="mt-0.5 line-clamp-2 text-sm text-ink-sub">{item.content}</p>
                  <time dateTime={item.created_at} title={formatDateTime(item.created_at)} className="mt-0.5 block text-xs text-ink-sub">
                    {formatRelativeOccurrence(item.created_at)}
                  </time>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </Card>
  )
}

/** DiscussionEntry 右栏「讨论与答疑」入口 */
function DiscussionEntry({ onOpen }: { onOpen: () => void }) {
  return (
    <button
      type="button"
      onClick={onOpen}
      className="pressable flex items-center gap-3 rounded-lg bg-surface px-5 py-4 text-left shadow-xs transition-shadow hover:shadow-sm focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
    >
      <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary-soft text-primary">
        <Icon icon={MessageSquare} size="sm" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-base font-semibold text-ink">讨论与答疑</span>
        <span className="block text-sm text-ink-sub">向老师和同学提问</span>
      </span>
      <Icon icon={ChevronRight} size="sm" className="text-ink-faint" />
    </button>
  )
}

/**
 * AllAssignments「作业」标签:全部作业,未截止的在前、已截止的沉底;每行写明本人交了几次、最近一次的状态。
 * 还能交的链向作答,已截止或次数用完的链向结果。
 * 学生只会收到已发布作业(服务端按身份过滤)。
 */
function AllAssignments({ courseId }: { courseId: string }) {
  const navigate = useNavigate()
  const assignments = usePagedResource<Assignment>(
    (params) => api.teaching.listCourseAssignments(courseId, params),
    [courseId],
    { urlPageKey: 'page' },
  )

  return (
    <div className="overflow-hidden rounded-lg bg-surface shadow-xs">
      <ResourceState
        resource={assignments}
        emptyIcon={ClipboardList}
        emptyTitle="还没有作业"
        emptyDescription="老师发布作业后会显示在这里,截止时间也会一并给出。"
        skeleton={
          <div className="p-5">
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(page) => {
          const now = Date.now()
          const sorted = [...page.list].sort((a, b) => {
            const aOver = new Date(a.due_at).getTime() <= now
            const bOver = new Date(b.due_at).getTime() <= now
            if (aOver !== bOver) return aOver ? 1 : -1
            return new Date(a.due_at).getTime() - new Date(b.due_at).getTime()
          })
          return (
            <>
              <ul className="divide-y divide-line">
                {sorted.map((assignment) => {
                  const deadline = formatRelativeDeadline(assignment.due_at)
                  const overdue = deadline.urgency === 'overdue'
                  const attempts = assignment.my_submission?.attempts ?? 0
                  const usedUp = attempts >= assignment.max_attempts
                  const closed = overdue || usedUp
                  const base = `/student/courses/${courseId}/assignments/${assignment.id}`
                  return (
                    <li key={assignment.id}>
                      <ListRow
                        href={closed ? `${base}/submissions` : base}
                        onNavigate={navigate}
                        leading={
                          <Icon icon={ClipboardList} size="md" className={cn(closed ? 'text-ink-faint' : 'text-primary')} />
                        }
                        title={assignment.title}
                        meta={`${formatDateTime(assignment.due_at)} 截止 · 已交 ${attempts}/${assignment.max_attempts} 次 · ${latePolicyLabel(assignment.late_policy)}`}
                        trailing={
                          <>
                            {assignment.my_submission && assignment.my_submission.latest_status !== 0 ? (
                              <span className="whitespace-nowrap text-sm text-ink-sub">
                                {submissionStatusLabel(assignment.my_submission.latest_status)}
                              </span>
                            ) : null}
                            <Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]}>{deadline.text}</Badge>
                            {closed || attempts === 0 ? null : (
                              <Button variant="ghost" size="sm" onClick={() => navigate(`${base}/submissions`)}>
                                看结果
                              </Button>
                            )}
                          </>
                        }
                      />
                    </li>
                  )
                })}
              </ul>
              <div className="border-t border-line px-4 py-3">
                <Pagination
                  page={assignments.page}
                  pageSize={assignments.pageSize}
                  total={assignments.total}
                  onPageChange={assignments.setPage}
                />
              </div>
            </>
          )
        }}
      </ResourceState>
    </div>
  )
}
