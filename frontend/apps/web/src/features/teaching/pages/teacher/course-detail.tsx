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
  Breadcrumb,
  Button,
  MetricStrip,
  ObjectIdentity,
  PageHeader,
  PageScaffold,
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
  courseTypeLabel,
  courseVisibilityLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { courseStatusTone } from '../../statusPresentation'
import { CourseFormModal } from '../../components/CourseFormModal'
import { CourseChapters } from './course-chapters'
import { CourseMembers } from './course-members'
import { CourseAssignments } from './course-assignments'
import { CourseBoard } from './course-board'
import { CourseGrades } from '../../components/CourseGrades'

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
      {/*
        归族:详情族(§6.5.3 第 ④)。h1 由 ObjectIdentity 的课程名承担,
        故 PageHeader 只出面包屑 —— 一页只该有一个 h1(§6.5.0 通则 1)。
        面包屑末节到「课程管理」为止,不再重复课程名。
      */}
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '教学' },
              { label: '课程管理', href: '/teacher/courses' },
            ]}
          />
        }
      />

      {/*
        对象身份区:课程名 + 状态 + 关键属性横排 + 对象级动作。
        属性用横排而不是 Stat 大卡 —— 学分、学期、课程类型是**属性**不是指标,
        套上 Display 字号会用排数字的方式排正文(§2.3)。
        全班进度那三个数字是真指标,放在下方的分组里(它们要与课时列表对照着看)。
      */}
      <ObjectIdentity
        name={course.name}
        status={
          <StatusIndicator
            tone={courseStatusTone(course.status)}
            label={courseStatusLabel(course.status)}
          />
        }
        subtitle={course.description || '还没有填写课程简介。'}
        actions={
          <Button variant="outline" leftIcon={Pencil} onClick={() => setEditOpen(true)}>
            编辑课程
          </Button>
        }
        properties={[
          { label: '学分', value: course.credits },
          { label: '学期', value: course.semester },
          { label: '课程类型', value: courseTypeLabel(course.type) },
          { label: '难度', value: teachingDifficultyLabel(course.difficulty) },
          { label: '可见范围', value: courseVisibilityLabel(course.visibility) },
          {
            label: '开课区间',
            value: `${formatDate(course.start_at)} — ${formatDate(course.end_at)}`,
          },
          {
            label: '邀请码',
            value: course.invite_code ? (
              <span className="font-mono">{course.invite_code}</span>
            ) : (
              '未启用'
            ),
          },
        ]}
      />

      {/* 全班进度摘要:三项都取教师专用聚合接口,不是大纲里的本人视角(§6.5.4 全量口径) */}
      <MetricStrip
        label="全班进度摘要"
        className="mt-4 mb-5"
        items={[
          { label: '选课人数', value: stats.data ? stats.data.member_count : '—', hint: '在读成员' },
          {
            label: '课时数',
            value: stats.data ? stats.data.lesson_count : outline.lessons.length,
            hint: '已编排的课时',
          },
          {
            label: '累计完成课时',
            value: stats.data ? stats.data.completed_count : '—',
            hint: '全班累计的课时完成次数',
          },
        ]}
      />

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
