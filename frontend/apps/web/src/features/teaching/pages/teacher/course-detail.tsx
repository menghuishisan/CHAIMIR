// 课程详情页(教师深页,/teacher/courses/:courseId)。设计 §7.4:
// 页头为课程名 + 状态 + 一行属性(学期 · 学分 · 类型 · 选课人数 · 开课区间)+ 邀请码复制;右侧主按钮与更多菜单。
// 下方标签「章节课时 / 作业 / 成员 / 讨论与公告 / 成绩」,标签带计数,当前标签写进地址。
// 主操作按状态:草稿为「发布课程」(章节为空时提醒先加内容);已发布或进行中为「复制邀请码」;
// 已结课不显示主按钮。一页承载同一门课的全部维护工作,拆成多个侧栏项会让教师在页面间来回跳。

import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Book, ClipboardCopy, ClipboardList, Layers, MessageSquare, MoreHorizontal, Pencil, Send, Users } from 'lucide-react'
import { CourseStatus, TENANT_MODULE, type CourseOutline } from '@chaimir/api-client'
import {
  Button,
  Callout,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  Modal,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Skeleton,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard/session'
import { locationLabel, useAsyncResource, usePageLocation, useUrlState } from '../../../../hooks'
import { copyText } from '../../../../utils/clipboard'
import { formatDateRange } from '../../../../utils/formatters'
import { courseStatusLabel, courseTypeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CourseFormModal } from '../../components/CourseFormModal'
import { CourseGrades } from '../../components/CourseGrades'
import { courseStatusTone } from '../../statusPresentation'
import { CourseAssignments } from './course-assignments'
import { CourseBoard } from './course-board'
import { CourseChapters } from './course-chapters'
import { CourseMembers } from './course-members'

/** 地址参数:tab 是当前标签 */
const DETAIL_QUERY = {
  tab: { defaultValue: 'chapters', allowed: ['chapters', 'assignments', 'members', 'board', 'grades'] },
} as const

/**
 * TeacherCourseDetailPage 读取课程大纲并按任务分区呈现管理能力。
 */
export default function TeacherCourseDetailPage() {
  const { courseId = '' } = useParams<{ courseId: string }>()
  const outline = useAsyncResource(() => api.teaching.getCourseOutline(courseId), [courseId], () => false)
  usePageLocation([{ label: locationLabel(outline, (data) => data.course.name, '课程详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={outline}
        emptyIcon={Book}
        emptyTitle="课程不可用"
        emptyDescription="这门课程可能已被删除,请回到课程管理查看。"
        skeleton={
          <div aria-busy="true" className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="line" width="third" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) => <CourseDetailContent courseId={courseId} outline={data} onRefresh={outline.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * CourseDetailContent 渲染课程页头、主操作与五个管理标签。
 */
function CourseDetailContent({
  courseId,
  outline,
  onRefresh,
}: {
  courseId: string
  outline: CourseOutline
  onRefresh: () => void
}) {
  const navigate = useNavigate()
  const { me } = useSession()
  const gradesEnabled = me.enabled_modules.includes(TENANT_MODULE.GRADE)
  const [query, setQuery] = useUrlState(DETAIL_QUERY)
  const tab = !gradesEnabled && query.tab === 'grades' ? 'chapters' : query.tab
  const [editOpen, setEditOpen] = useState(false)
  const [publishOpen, setPublishOpen] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const course = outline.course

  // 选课人数取教师专用的全班统计;读不到时页头不显示这一项
  const stats = useAsyncResource(() => api.teaching.getProgressStats(courseId), [courseId], () => false)
  const memberCount = stats.data?.member_count
  const inviteCode = course.invite_code
  const canShareInvite = Boolean(inviteCode) && (course.status === CourseStatus.PUBLISHED || course.status === CourseStatus.RUNNING)

  const copyInvite = useCallback(() => {
    if (!inviteCode) return
    void copyText(inviteCode, { what: '邀请码', operation: 'teaching.course.copyInviteCode' }).then((ok) => {
      if (ok) toast.success('邀请码已复制,发给学生即可加入')
    })
  }, [inviteCode])

  const publish = useCallback(async () => {
    setPublishing(true)
    setActionError(undefined)
    try {
      await api.teaching.publishCourse(courseId)
      toast.success('课程已发布,学生可以用邀请码加入')
      setPublishOpen(false)
      onRefresh()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '课程没有发布成功,请稍后重试。'))
    } finally {
      setPublishing(false)
    }
  }, [courseId, onRefresh])

  return (
    <>
      <PageHeader
        title={course.name}
        status={<StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />}
        meta={[
          { value: course.semester },
          { value: `${course.credits} 学分` },
          { value: courseTypeLabel(course.type) },
          { label: '选课', value: memberCount !== undefined ? `${memberCount} 人` : undefined },
          { value: formatDateRange(course.start_at, course.end_at) },
          {
            label: '邀请码',
            value: canShareInvite && inviteCode ? (
              <span className="inline-flex items-center gap-1">
                <span className="font-mono">{inviteCode}</span>
                <IconButton variant="ghost" size="sm" icon={ClipboardCopy} aria-label="复制邀请码" onClick={copyInvite} />
              </span>
            ) : undefined,
          },
        ]}
        actions={
          <>
            {course.status === CourseStatus.DRAFT ? (
              <Button variant="seal" leftIcon={Send} onClick={() => setPublishOpen(true)}>
                发布课程
              </Button>
            ) : canShareInvite ? (
              <Button variant="primary" leftIcon={ClipboardCopy} onClick={copyInvite}>
                复制邀请码
              </Button>
            ) : null}
            <Menu>
              <MenuTrigger asChild>
                <IconButton variant="outline" icon={MoreHorizontal} aria-label="更多操作" />
              </MenuTrigger>
              <MenuContent align="end">
                <MenuItem icon={Pencil} onSelect={() => setEditOpen(true)}>
                  编辑课程
                </MenuItem>
              </MenuContent>
            </Menu>
          </>
        }
      />

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      <Tabs value={tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="课程管理">
          <TabsTrigger value="chapters" icon={Layers} count={outline.lessons.length}>
            章节课时
          </TabsTrigger>
          <TabsTrigger value="assignments" icon={ClipboardList}>
            作业
          </TabsTrigger>
          <TabsTrigger value="members" icon={Users} count={memberCount}>
            成员
          </TabsTrigger>
          <TabsTrigger value="board" icon={MessageSquare}>
            讨论与公告
          </TabsTrigger>
          {gradesEnabled ? (
            <TabsTrigger value="grades" icon={Send}>
              成绩
            </TabsTrigger>
          ) : null}
        </TabsList>
        <TabsContent value="chapters">
          <CourseChapters courseId={courseId} outline={outline} onChanged={onRefresh} />
        </TabsContent>
        <TabsContent value="assignments">
          <CourseAssignments
            courseId={courseId}
            outline={outline}
            onOpenGrading={(assignmentId: string) =>
              navigate(`/teacher/grading?course=${courseId}&assignment=${assignmentId}`)
            }
          />
        </TabsContent>
        <TabsContent value="members">
          <CourseMembers courseId={courseId} />
        </TabsContent>
        <TabsContent value="board">
          <CourseBoard courseId={courseId} />
        </TabsContent>
        {gradesEnabled ? (
          <TabsContent value="grades">
            <CourseGrades courseId={courseId} />
          </TabsContent>
        ) : null}
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

      <Modal open={publishOpen} onOpenChange={(open) => (publishing ? undefined : setPublishOpen(open))}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>发布「{course.name}」?</ModalTitle>
            <ModalDescription>发布后学生可以用邀请码加入,课程基础信息将不能再随意修改。</ModalDescription>
          </ModalHeader>
          {outline.lessons.length === 0 ? (
            <div className="px-6">
              <Callout tone="warning">这门课还没有课时,学生加入后会看到一门空课。建议先在「章节课时」里加上内容。</Callout>
            </div>
          ) : null}
          <ModalFooter>
            <Button variant="outline" disabled={publishing} onClick={() => setPublishOpen(false)}>
              取消
            </Button>
            <Button variant="seal" loading={publishing} onClick={() => void publish()}>
              发布课程
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </>
  )
}
