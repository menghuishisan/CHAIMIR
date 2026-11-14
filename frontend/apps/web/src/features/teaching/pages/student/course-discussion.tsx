// 课程「讨论与公告」标签(学生课程详情)。设计 §7.2:公告列表与讨论流(发帖、点赞)。
// 公告是列表行:标题、发布时间(相对时间)、置顶标记,点开就地展开全文;讨论流在下方。
// 两块数据独立读取:公告失败不该让讨论区也看不到。

import { useState } from 'react'
import { Megaphone, Pin } from 'lucide-react'
import type { TeachingAnnouncement } from '@chaimir/api-client'
import { Badge, ListRow, Pagination, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { CoursePosts } from '../../components/CoursePosts'

export interface CourseDiscussionProps {
  courseId: string
}

/** CourseDiscussion 公告在上、讨论在下 */
export function CourseDiscussion({ courseId }: CourseDiscussionProps) {
  return (
    <div className="flex flex-col gap-8">
      <CourseAnnouncements courseId={courseId} />
      <CoursePosts key={courseId} courseId={courseId} />
    </div>
  )
}

/** CourseAnnouncements 课程公告:置顶在前(后端按置顶与时间排序),点开就地展开全文 */
function CourseAnnouncements({ courseId }: CourseDiscussionProps) {
  const [expanded, setExpanded] = useState<string>()
  const announcements = usePagedResource<TeachingAnnouncement>(
    (params) => api.teaching.listAnnouncements(courseId, params),
    [courseId],
    { urlPageKey: 'notice_page' }
  )

  return (
    <section aria-labelledby="course-announcements-title">
      <h2 id="course-announcements-title" className="mb-3 text-lg font-semibold text-ink">
        课程公告
      </h2>
      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="还没有公告"
        emptyDescription="老师发布课程公告后会显示在这里。"
        skeleton={<Skeleton variant="line" lines={3} />}
      >
        {(page) => (
          <>
            <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
              {page.list.map((item) => {
                const open = expanded === item.id
                return (
                  <li key={item.id}>
                    <ListRow
                      title={item.title}
                      leading={
                        item.is_pinned ? (
                          <Pin aria-hidden="true" className="size-4 text-primary" />
                        ) : undefined
                      }
                      meta={
                        <span title={formatDateTime(item.created_at)}>
                          {formatRelativeOccurrence(item.created_at)}发布
                        </span>
                      }
                      trailing={item.is_pinned ? <Badge tone="jade">置顶</Badge> : undefined}
                      selected={open}
                      onSelect={() => setExpanded(open ? undefined : item.id)}
                    />
                    {open ? (
                      <p className="whitespace-pre-wrap px-3 pb-4 pt-1 text-base leading-relaxed text-ink">
                        {item.content}
                      </p>
                    ) : null}
                  </li>
                )
              })}
            </ul>
            <div className="mt-3">
              <Pagination
                page={announcements.page}
                pageSize={announcements.pageSize}
                total={announcements.total}
                onPageChange={announcements.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>
    </section>
  )
}
