// 课程详情页(深页,/student/courses/:courseId)。
// 完成率在这里计算:outline 是唯一同时返回课时总数与本人进度的接口。
// 作业清单走 GET /teaching/courses/{id}/assignments —— 学生取得作业编号的唯一入口。

import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'
import {
  BookOpen,
  CalendarClock,
  ClipboardList,
  FileText,
  Layers,
  MessageSquare,
} from 'lucide-react'
import { ProgressStatus, type Assignment, type Chapter, type CourseOutline, type Lesson } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Card,
  CardBody,
  CardHeader,
  CoverImage,
  DescriptionList,
  Empty,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Stat,
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  formatDate,
  formatDateTime,
  formatDuration,
  formatPercent,
  formatRelativeDeadline,
} from '../../../../utils/formatters'
import {
  ASSIGNMENT_DUE_TONE,
  courseStatusLabel,
  courseStatusTone,
  courseTypeCover,
  courseTypeLabel,
  latePolicyLabel,
  lessonContentTypeLabel,
  progressStatusLabel,
  progressStatusTone,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { useCourseCoverSrc } from '../../useCourseCoverSrc'
import { CourseDiscussion } from './course-discussion'
import { CourseReviewCard } from './course-review'

/** OutlineRow 是章节课时表的一行:课时挂在其所属章节下,进度按课时对齐。 */
interface OutlineRow {
  lesson: Lesson
  chapter: Chapter | undefined
  status: ProgressStatus
  durationSec: number
}

/**
 * outlineRows 把大纲的三份数组按章节顺序摊平成表格行。
 * 后端 outline 分别给出章节、课时与进度,章节内课时按 sort 排列;
 * 进度可能缺失(未开始学的课时没有记录),缺失即未开始。
 */
function outlineRows(outline: CourseOutline): OutlineRow[] {
  const chapterById = new Map(outline.chapters.map((chapter) => [chapter.id, chapter]))
  const progressByLesson = new Map(outline.progress.map((item) => [item.lesson_id, item]))
  const chapterOrder = new Map(outline.chapters.map((chapter, index) => [chapter.id, index]))

  return [...outline.lessons]
    .sort((a, b) => {
      const chapterDiff = (chapterOrder.get(a.chapter_id) ?? 0) - (chapterOrder.get(b.chapter_id) ?? 0)
      return chapterDiff !== 0 ? chapterDiff : a.sort - b.sort
    })
    .map((lesson) => {
      const progress = progressByLesson.get(lesson.id)
      return {
        lesson,
        chapter: chapterById.get(lesson.chapter_id),
        status: progress ? progress.status : ProgressStatus.NOT_STARTED,
        durationSec: progress ? progress.duration_sec : 0,
      }
    })
}

/**
 * StudentCourseDetailPage 承载大纲、进度、作业、讨论与课程评价。
 */
export default function StudentCourseDetailPage() {
  const { courseId = '' } = useParams<{ courseId: string }>()
  const navigate = useNavigate()

  const outline = useAsyncResource(() => api.teaching.getCourseOutline(courseId), [courseId], () => false)

  return (
    <PageScaffold>
      <ResourceState
        resource={outline}
        emptyIcon={BookOpen}
        emptyTitle="课程内容暂未开放"
        emptyDescription="老师还没有发布章节课时,发布后会显示在这里。"
      >
        {(data) => <CourseDetailContent courseId={courseId} outline={data} onNavigate={navigate} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface CourseDetailContentProps {
  courseId: string
  outline: CourseOutline
  onNavigate: (path: string) => void
}

/**
 * CourseDetailContent 渲染课程头部、指标带与分区内容。
 */
function CourseDetailContent({ courseId, outline, onNavigate }: CourseDetailContentProps) {
  const course = outline.course
  const coverSrc = useCourseCoverSrc(course.id, course.cover_ref)
  const rows = useMemo(() => outlineRows(outline), [outline])
  const doneCount = rows.filter((row) => row.status === ProgressStatus.DONE).length
  const learnedSeconds = rows.reduce((sum, row) => sum + row.durationSec, 0)

  const lessonColumns: TableColumn<OutlineRow>[] = [
    {
      key: 'chapter',
      header: '章节',
      render: (row) => <span className="text-ink-sub">{row.chapter ? row.chapter.title : '未分章'}</span>,
    },
    {
      key: 'lesson',
      header: '课时',
      render: (row) => (
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium text-ink">{row.lesson.title}</span>
          <Badge tone="neutral">{lessonContentTypeLabel(row.lesson.content_type)}</Badge>
        </div>
      ),
    },
    {
      key: 'duration',
      header: '学习时长',
      align: 'right',
      render: (row) => <span className="text-ink-sub">{formatDuration(row.durationSec)}</span>,
    },
    {
      key: 'status',
      header: '学习状态',
      render: (row) => (
        <StatusIndicator tone={progressStatusTone(row.status)} label={progressStatusLabel(row.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (row) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onNavigate(`/student/courses/${courseId}/lessons/${row.lesson.id}`)}
        >
          {row.status === ProgressStatus.NOT_STARTED ? '开始学习' : '继续学习'}
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
              { label: '学习区' },
              { label: '我的课程', href: '/student/courses' },
              { label: course.name },
            ]}
          />
        }
        title={course.name}
        description={course.description}
        icon={BookOpen}
        actions={<StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />}
      />

      {/* 封面在详情页只占首屏一小条:课程目标与继续学习动作优先于装饰(规范 §1.3 资产不抢内容) */}
      <PageSection>
        <CoverImage
          id={course.id}
          coverSrc={coverSrc}
          name={course.name}
          glyph={courseTypeCover(course.type).glyph}
          accent={courseTypeCover(course.type).accent}
          ratio="3/2"
          className="max-h-52 w-full"
        />
      </PageSection>

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="课时完成"
            value={formatPercent(doneCount, rows.length)}
            icon={Layers}
            chain={{ done: doneCount, total: rows.length }}
            hint={`共 ${rows.length} 个课时`}
          />
          <Stat label="累计学习时长" value={formatDuration(learnedSeconds)} icon={CalendarClock} />
          <Stat label="课程学分" value={course.credits} icon={FileText} hint={course.semester} />
          <Stat
            label="课程形式"
            value={courseTypeLabel(course.type)}
            icon={BookOpen}
            hint={teachingDifficultyLabel(course.difficulty)}
          />
        </div>
      </PageSection>

      <PageBody rail={<CourseInfoRail outline={outline} courseId={courseId} />}>
        <Tabs defaultValue="lessons">
          <TabsList>
            <TabsTrigger value="lessons" icon={Layers}>
              章节课时
            </TabsTrigger>
            <TabsTrigger value="assignments" icon={ClipboardList}>
              课程作业
            </TabsTrigger>
            <TabsTrigger value="discussion" icon={MessageSquare}>
              讨论与公告
            </TabsTrigger>
          </TabsList>

          <TabsContent value="lessons">
            <PageSection title="章节课时" description={`已完成 ${doneCount} / ${rows.length} 个课时`}>
              <Table
                columns={lessonColumns}
                data={rows}
                rowKey={(row) => row.lesson.id}
                empty={
                  <Empty
                    icon={Layers}
                    title="暂无课时"
                    description="老师发布章节课时后会显示在这里。"
                  />
                }
              />
            </PageSection>
          </TabsContent>

          <TabsContent value="assignments">
            <CourseAssignments courseId={courseId} onNavigate={onNavigate} />
          </TabsContent>

          <TabsContent value="discussion">
            <CourseDiscussion courseId={courseId} />
          </TabsContent>
        </Tabs>
      </PageBody>
    </>
  )
}

interface CourseInfoRailProps {
  outline: CourseOutline
  courseId: string
}

/**
 * CourseInfoRail 是右侧信息与动作区:课程档案 + 课程评价入口。
 */
function CourseInfoRail({ outline, courseId }: CourseInfoRailProps) {
  const course = outline.course
  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader title="课程信息" />
        <CardBody>
          <DescriptionList
            dense
            items={[
              { term: '学期', description: course.semester },
              { term: '学分', description: course.credits, mono: true },
              { term: '课程形式', description: courseTypeLabel(course.type) },
              { term: '难度', description: teachingDifficultyLabel(course.difficulty) },
              { term: '开课时间', description: formatDate(course.start_at), mono: true },
              { term: '结课时间', description: formatDate(course.end_at), mono: true },
            ]}
          />
        </CardBody>
      </Card>
      <CourseReviewCard courseId={courseId} courseStatus={course.status} />
    </div>
  )
}

interface CourseAssignmentsProps {
  courseId: string
  onNavigate: (path: string) => void
}

/**
 * CourseAssignments 列出课程作业。学生只会收到已发布作业(服务端按身份过滤)。
 */
function CourseAssignments({ courseId, onNavigate }: CourseAssignmentsProps) {
  const assignments = useAsyncResource(
    () => api.teaching.listCourseAssignments(courseId),
    [courseId],
  )

  const columns: TableColumn<Assignment>[] = [
    {
      key: 'title',
      header: '作业',
      render: (assignment) => <span className="font-medium text-ink">{assignment.title}</span>,
    },
    {
      key: 'due_at',
      header: '截止时间',
      render: (assignment) => {
        const deadline = formatRelativeDeadline(assignment.due_at)
        return (
          <div className="flex flex-col gap-1">
            <span className="font-mono text-xs tabular-nums text-ink-sub">
              {formatDateTime(assignment.due_at)}
            </span>
            <Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]}>{deadline.text}</Badge>
          </div>
        )
      },
    },
    {
      key: 'max_attempts',
      header: '可提交次数',
      align: 'right',
      mono: true,
    },
    {
      key: 'late_policy',
      header: '迟交规则',
      render: (assignment) => <span className="text-ink-sub">{latePolicyLabel(assignment.late_policy)}</span>,
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (assignment) => (
        <div className="flex justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onNavigate(`/student/courses/${courseId}/assignments/${assignment.id}`)}
          >
            去作答
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onNavigate(`/student/courses/${courseId}/assignments/${assignment.id}/submissions`)
            }
          >
            看结果
          </Button>        </div>
      ),
    },
  ]

  return (
    <PageSection title="课程作业" description="作业按截止时间从近到远排列。">
      <ResourceState
        resource={assignments}
        emptyIcon={ClipboardList}
        emptyTitle="暂无作业"
        emptyDescription="老师发布作业后会显示在这里,截止时间也会一并给出。"
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(list) => <Table columns={columns} data={list} rowKey={(item) => item.id} />}
      </ResourceState>
    </PageSection>
  )
}
