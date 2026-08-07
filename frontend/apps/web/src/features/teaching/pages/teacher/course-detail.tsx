// 课程详情页(教师深页,/teacher/courses/:courseId)。
// 一页承载课程编辑入口、章节课时、选课成员、作业管理、讨论公告与课程成绩六件事 ——
// 它们都以同一门课程为上下文,拆成六个侧栏项会让教师在页面间来回跳。

import { useNavigate, useParams } from 'react-router'
import {
  Book,
  ClipboardList,
  Layers,
  MessageSquare,
  Pencil,
  Send,
  Users,
} from 'lucide-react'
import { useState } from 'react'
import type { CourseOutline } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  PageHeader,
  PageScaffold,
  PageSection,
  Stat,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDate } from '../../../../utils/formatters'
import {
  courseStatusLabel,
  courseStatusTone,
  courseTypeLabel,
  courseVisibilityLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { CourseFormModal } from './course-form'
import { CourseChapters } from './course-chapters'
import { CourseMembers } from './course-members'
import { CourseAssignments } from './course-assignments'
import { CourseBoard } from './course-board'
import { CourseGrades } from './course-grades'

/**
 * TeacherCourseDetailPage 读取课程大纲并按任务分区呈现管理能力。
 */
export default function TeacherCourseDetailPage() {
  const { courseId = '' } = useParams<{ courseId: string }>()

  const outline = useAsyncResource(
    () => api.teaching.getCourseOutline(courseId),
    [courseId],
    () => false,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={outline}
        emptyIcon={Book}
        emptyTitle="课程不可用"
        emptyDescription="这门课程可能已被删除,请回到课程管理查看。"
      >
        {(data) => <CourseDetailContent courseId={courseId} outline={data} onRefresh={outline.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface CourseDetailContentProps {
  courseId: string
  outline: CourseOutline
  onRefresh: () => void
}

/**
 * CourseDetailContent 渲染课程头部、指标带与六个管理分区。
 */
function CourseDetailContent({ courseId, outline, onRefresh }: CourseDetailContentProps) {
  const navigate = useNavigate()
  const [editOpen, setEditOpen] = useState(false)
  const course = outline.course

  // 进度统计走教师专用接口:大纲里的 progress 是本人视角,教师需要全班聚合
  const stats = useAsyncResource(
    () => api.teaching.getProgressStats(courseId),
    [courseId],
    () => false,
  )

  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '教学' },
              { label: '课程管理', href: '/teacher/courses' },
              { label: course.name },
            ]}
          />
        }
        title={course.name}
        description={course.description || '还没有填写课程简介。'}
        icon={Book}
        actions={
          <div className="flex items-center gap-2">
            <StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />
            <Button variant="outline" leftIcon={Pencil} onClick={() => setEditOpen(true)}>
              编辑课程
            </Button>
          </div>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="选课人数"
            value={stats.data ? stats.data.member_count : '—'}
            icon={Users}
          />
          <Stat
            label="课时数"
            value={stats.data ? stats.data.lesson_count : outline.lessons.length}
            icon={Layers}
          />
          <Stat
            label="累计完成课时"
            value={stats.data ? stats.data.completed_count : '—'}
            icon={ClipboardList}
            hint="全班累计的课时完成次数"
          />
          <Stat
            label="课程学分"
            value={course.credits}
            icon={Book}
            hint={`${course.semester} · ${courseTypeLabel(course.type)}`}
          />
        </div>
      </PageSection>

      <PageSection>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="neutral">{teachingDifficultyLabel(course.difficulty)}</Badge>
          <Badge tone="neutral">{courseVisibilityLabel(course.visibility)}</Badge>
          <Badge tone="neutral">
            {formatDate(course.start_at)} — {formatDate(course.end_at)}
          </Badge>
          {course.invite_code ? (
            <Badge tone="jade">邀请码 {course.invite_code}</Badge>
          ) : null}
        </div>
      </PageSection>

      <Tabs defaultValue="chapters">
        <TabsList>
          <TabsTrigger value="chapters" icon={Layers}>
            章节课时
          </TabsTrigger>
          <TabsTrigger value="assignments" icon={ClipboardList}>
            作业管理
          </TabsTrigger>
          <TabsTrigger value="members" icon={Users}>
            选课成员
          </TabsTrigger>
          <TabsTrigger value="board" icon={MessageSquare}>
            讨论与公告
          </TabsTrigger>
          <TabsTrigger value="grades" icon={Send}>
            课程成绩
          </TabsTrigger>
        </TabsList>

        <TabsContent value="chapters">
          <CourseChapters courseId={courseId} outline={outline} onChanged={onRefresh} />
        </TabsContent>
        <TabsContent value="assignments">
          <CourseAssignments
            courseId={courseId}
            outline={outline}
            onOpenGrading={(assignmentId: string) => navigate(`/teacher/grading?assignment=${assignmentId}`)}
          />
        </TabsContent>
        <TabsContent value="members">
          <CourseMembers courseId={courseId} />
        </TabsContent>
        <TabsContent value="board">
          <CourseBoard courseId={courseId} />
        </TabsContent>
        <TabsContent value="grades">
          <CourseGrades courseId={courseId} />
        </TabsContent>
      </Tabs>

      {editOpen ? (
        <CourseFormModal
          course={course}
          onClose={() => setEditOpen(false)}
          onSaved={() => {
            setEditOpen(false)
            onRefresh()
          }}
        />
      ) : null}
    </>
  )
}
