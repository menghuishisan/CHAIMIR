// 课程讨论与公告(课程详情页内区块)。
// 讨论帖与公告都是课程内页能力(对齐清单 §3.2「课程讨论」从课程进入,不进侧栏),
// 学生可发帖、可点赞;置顶与删除是教师能力,学生侧不出现这些动作。

import { useCallback, useState } from 'react'
import { MessageSquare, Megaphone, Pin, Send, ThumbsUp } from 'lucide-react'
import type { TeachingAnnouncement, TeachingPost } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  FormField,
  Icon,
  PageSection,
  Pagination,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

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
      <CoursePosts courseId={courseId} />
    </>
  )
}

/**
 * CourseAnnouncements 展示课程公告,置顶公告排在前面(后端已按置顶与时间排序)。
 */
function CourseAnnouncements({ courseId }: CourseDiscussionProps) {
  const announcements = useAsyncResource(
    () => api.teaching.listAnnouncements(courseId),
    [courseId],
  )

  return (
    <PageSection title="课程公告" description="老师发布的课程通知。">
      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="暂无公告"
        emptyDescription="老师发布课程公告后会显示在这里。"
        skeleton={<Skeleton variant="line" lines={3} />}
      >
        {(list) => (
          <div className="flex flex-col gap-3">
            {list.map((item) => (
              <AnnouncementItem key={item.id} announcement={item} />
            ))}
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

/**
 * CoursePosts 展示讨论帖分页列表,并提供发帖入口与点赞动作。
 */
function CoursePosts({ courseId }: CourseDiscussionProps) {
  const posts = usePagedResource<TeachingPost>(
    (params) => api.teaching.listPosts(courseId, params),
    [courseId],
  )

  return (
    <PageSection title="课程讨论" description={`共 ${posts.total} 条讨论`}>
      <div className="flex flex-col gap-4">
        <NewPostCard courseId={courseId} onPosted={posts.reload} />

        <ResourceState
          resource={posts}
          emptyIcon={MessageSquare}
          emptyTitle="还没有讨论"
          emptyDescription="有疑问可以在这里提出,老师和同学都能看到。"
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(page) => (
            <div className="flex flex-col gap-3">
              {page.list.map((post) => (
                <PostItem key={post.id} post={post} onChanged={posts.reload} />
              ))}
              <Pagination
                page={posts.page}
                pageSize={posts.pageSize}
                total={posts.total}
                onPageChange={posts.setPage}
              />
            </div>
          )}
        </ResourceState>
      </div>
    </PageSection>
  )
}

interface NewPostCardProps {
  courseId: string
  onPosted: () => void
}

/**
 * NewPostCard 是发帖动作卡:提交 loading、失败就近内联反馈。
 */
function NewPostCard({ courseId, onPosted }: NewPostCardProps) {
  const [content, setContent] = useState('')
  const [fieldError, setFieldError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const text = content.trim()
      if (!text) {
        setFieldError('请输入要发布的内容')
        return
      }
      setFieldError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.createPost(courseId, { content: text })
        setContent('')
        toast.success('已发布')
        onPosted()
      } catch (postError) {
        setFieldError(userFacingErrorMessage(postError, '发布失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [content, courseId, onPosted],
  )

  return (
    <Card>
      <CardBody>
        <form onSubmit={handleSubmit} noValidate>
          <FormField label="发表讨论" required error={fieldError}>
            <Textarea
              value={content}
              rows={3}
              placeholder="说说你的想法或提出问题"
              invalid={Boolean(fieldError)}
              onChange={(event) => setContent(event.target.value)}
            />
          </FormField>
          <div className="mt-3 flex justify-end">
            <Button type="submit" leftIcon={Send} loading={submitting}>
              发布
            </Button>
          </div>
        </form>
      </CardBody>
    </Card>
  )
}

interface PostItemProps {
  post: TeachingPost
  onChanged: () => void
}

/**
 * PostItem 渲染单条讨论并承载点赞。
 * 点赞失败给就近错误提示而不是静默:用户点了没反应会反复点。
 */
function PostItem({ post, onChanged }: PostItemProps) {
  const [liking, setLiking] = useState(false)
  const [likeError, setLikeError] = useState<string>()

  const handleLike = useCallback(async () => {
    setLiking(true)
    setLikeError(undefined)
    try {
      await api.teaching.likePost(post.id)
      onChanged()
    } catch (error) {
      setLikeError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setLiking(false)
    }
  }, [onChanged, post.id])

  return (
    <Card>
      <CardBody className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          {post.is_pinned ? <Icon icon={Pin} size="sm" className="shrink-0 text-primary" /> : null}
          {post.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
          <span className="font-mono text-xs tabular-nums text-ink-faint">
            {formatShortDateTime(post.created_at)}
          </span>
        </div>
        <p className="whitespace-pre-wrap text-base text-ink">{post.content}</p>
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" leftIcon={ThumbsUp} loading={liking} onClick={() => void handleLike()}>
            有帮助 {post.like_count}
          </Button>
          {likeError ? <span className="text-xs text-danger">{likeError}</span> : null}
        </div>
      </CardBody>
    </Card>
  )
}
