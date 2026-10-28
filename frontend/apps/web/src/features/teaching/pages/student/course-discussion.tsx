// 课程讨论与公告(课程详情页内区块)。
// 讨论帖与公告都是课程内页能力(对齐清单 §3.2「课程讨论」从课程进入,不进侧栏),
// 学生与教师共用讨论流支持发帖、回复和点赞;置顶与删除只在教师侧启用。

import { Megaphone, Pin } from 'lucide-react'
import type { TeachingAnnouncement } from '@chaimir/api-client'
import {
  Badge,
  Card,
  CardBody,
  CardHeader,
  Icon,
  PageSection,
  Pagination,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { CoursePosts } from '../../components/CoursePosts'

export interface CourseDiscussionProps {
  courseId: string
}

/**
 * CourseDiscussion 组合公告栏与讨论区。
 * 两块数据独立读取:公告失败不该让讨论区也不可见,各自给出自己的三态。
 */
export function CourseDiscussion({ courseId }: CourseDiscussionProps) {
  return (
    <>
      <CourseAnnouncements courseId={courseId} />
      <CoursePosts key={courseId} courseId={courseId} />
    </>
  )
}

/**
 * CourseAnnouncements 展示课程公告,置顶公告排在前面(后端已按置顶与时间排序)。
 */
function CourseAnnouncements({ courseId }: CourseDiscussionProps) {
  const announcements = usePagedResource<TeachingAnnouncement>(
    (params) => api.teaching.listAnnouncements(courseId, params),
    [courseId],
  )

  return (
    // 卡片列表型页内子视图(§6.5.5 B):公告主体是整段正文,压不进一行事件条,
    // 故按卡片列表排 —— 卡本身就是抬起片,不再套 DataPanel(那会成为片里套片)。
    <PageSection title="课程公告" description="老师发布的课程通知。">
      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="暂无公告"
        emptyDescription="老师发布课程公告后会显示在这里。"
        skeleton={<Skeleton variant="line" lines={3} />}
      >
        {(page) => (
          <div className="flex flex-col gap-3">
            {page.list.map((item) => (
              <AnnouncementItem key={item.id} announcement={item} />
            ))}
            <Pagination
              page={announcements.page}
              pageSize={announcements.pageSize}
              total={announcements.total}
              onPageChange={announcements.setPage}
            />
          </div>
        )}
      </ResourceState>
    </PageSection>
  )
}

/**
 * AnnouncementItem 渲染单条公告。置顶用图标与徽标双通道表达,不只靠位置。
 */
function AnnouncementItem({ announcement }: { announcement: TeachingAnnouncement }) {
  return (
    <Card>
      <CardHeader
        title={
          <span className="flex items-center gap-2">
            {announcement.is_pinned ? (
              <Icon icon={Pin} size="sm" className="shrink-0 text-primary" />
            ) : null}
            <span className="min-w-0 truncate">{announcement.title}</span>
            {announcement.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
          </span>
        }
        description={formatShortDateTime(announcement.created_at)}
      />
      <CardBody>
        <p className="whitespace-pre-wrap text-base text-ink">{announcement.content}</p>
      </CardBody>
    </Card>
  )
}
