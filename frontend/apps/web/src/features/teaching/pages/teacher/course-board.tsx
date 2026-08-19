// 课程讨论与公告(课程详情页内区块,教师视角)。
// 教师比学生多三个动作:置顶帖子、删除违规帖、发布公告并置顶。
// 删除是不可逆的,故走确认弹层而不是 window.confirm(规范:危险操作需确认且视觉分离)。

import { useCallback, useState } from 'react'
import { Megaphone, MessageSquare, Pin, Send, Trash2 } from 'lucide-react'
import type { TeachingAnnouncement, TeachingPost } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageSection,
  Pagination,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

export interface CourseBoardProps {
  courseId: string
}

/**
 * CourseBoard 组合公告发布与讨论管理。
 */
export function CourseBoard({ courseId }: CourseBoardProps) {
  return (
    <>
      <CourseAnnouncements courseId={courseId} />
      <CoursePosts courseId={courseId} />
    </>
  )
}

/**
 * CourseAnnouncements 发布与置顶课程公告。
 */
function CourseAnnouncements({ courseId }: CourseBoardProps) {
  const [composeOpen, setComposeOpen] = useState(false)
  const [actionError, setActionError] = useState<string>()

	const announcements = usePagedResource<TeachingAnnouncement>((params) => api.teaching.listAnnouncements(courseId, params), [courseId])

  const togglePin = useCallback(
    async (announcement: TeachingAnnouncement) => {
      setActionError(undefined)
      try {
        await api.teaching.pinAnnouncement(announcement.id)
        announcements.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
      }
    },
    [announcements],
  )

  return (
    <PageSection
      title="课程公告"
      description="公告会显示在学生的课程详情页顶部。"
      actions={
        <Button variant="primary" leftIcon={Megaphone} onClick={() => setComposeOpen(true)}>
          发布公告
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={announcements}
          emptyIcon={Megaphone}
          emptyTitle="还没有公告"
          emptyDescription="重要通知发成公告,学生进入课程就能看到。"
          emptyAction={
            <Button variant="primary" leftIcon={Megaphone} onClick={() => setComposeOpen(true)}>
              发布公告
            </Button>
          }
          skeleton={<Skeleton variant="line" lines={3} />}
        >
			{(page) => (
				<div className="flex flex-col gap-3">
              {page.list.map((announcement) => (
                <Card key={announcement.id}>
                  <CardHeader
                    title={
                      <span className="flex flex-wrap items-center gap-2">
                        <span className="min-w-0 truncate">{announcement.title}</span>
                        {announcement.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
                      </span>
                    }
                    description={formatShortDateTime(announcement.created_at)}
                    actions={
                      <IconButton
                        variant="ghost"
                        size="sm"
                        icon={Pin}
                        aria-label={announcement.is_pinned ? '取消置顶公告' : '置顶公告'}
                        onClick={() => void togglePin(announcement)}
                      />
                    }
                  />
                  <CardBody>
                    <p className="whitespace-pre-wrap text-base leading-relaxed text-ink">
                      {announcement.content}
                    </p>
                  </CardBody>
                </Card>
					))}
					<Pagination page={announcements.page} pageSize={announcements.pageSize} total={announcements.total} onPageChange={announcements.setPage} />
				</div>
			)}
        </ResourceState>
      </div>

      {composeOpen ? (
        <AnnouncementComposeModal
          courseId={courseId}
          onClose={() => setComposeOpen(false)}
          onPosted={() => {
            setComposeOpen(false)
            announcements.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface AnnouncementComposeModalProps {
  courseId: string
  onClose: () => void
  onPosted: () => void
}

/**
 * AnnouncementComposeModal 发布课程公告。
 */
function AnnouncementComposeModal({ courseId, onClose, onPosted }: AnnouncementComposeModalProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [pinned, setPinned] = useState(false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (title.trim() === '' || content.trim() === '') {
        setFormError('标题和内容都要填写')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.teaching.createAnnouncement(courseId, {
          title: title.trim(),
          content: content.trim(),
          is_pinned: pinned,
        })
        toast.success('公告已发布')
        onPosted()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [content, courseId, onPosted, pinned, title],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>发布课程公告</ModalTitle>
          <ModalDescription>公告发布后学生进入课程即可看到。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="公告标题" htmlFor="announcement-title" required>
              <Input
                id="announcement-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>
            <FormField label="公告内容" htmlFor="announcement-content" required>
              <Textarea
                id="announcement-content"
                value={content}
                rows={6}
                onChange={(event) => setContent(event.target.value)}
              />
            </FormField>
            <Checkbox
              checked={pinned}
              label="置顶这条公告"
              onCheckedChange={(checked) => setPinned(checked === true)}
            />
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" leftIcon={Send} loading={working}>
              发布公告
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/**
 * CoursePosts 管理讨论帖:置顶与删除违规内容。
 */
function CoursePosts({ courseId }: CourseBoardProps) {
  const [deleteTarget, setDeleteTarget] = useState<TeachingPost>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const posts = usePagedResource<TeachingPost>(
    (params) => api.teaching.listPosts(courseId, params),
    [courseId],
  )

  const togglePin = useCallback(
    async (post: TeachingPost) => {
      setActionError(undefined)
      try {
        await api.teaching.pinPost(post.id)
        posts.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
      }
    },
    [posts],
  )

  const deletePost = useCallback(async () => {
    if (!deleteTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.teaching.deletePost(deleteTarget.id)
      toast.success('已删除')
      setDeleteTarget(undefined)
      posts.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [deleteTarget, posts])

  return (
    <PageSection title="课程讨论" description={`共 ${posts.total} 条。可以置顶重要讨论、删除违规内容。`}>
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={posts}
          emptyIcon={MessageSquare}
          emptyTitle="还没有讨论"
          emptyDescription="学生在课程里提问后会显示在这里。"
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(page) => (
            <>
              <div className="flex flex-col gap-3">
                {page.list.map((post) => (
                  <Card key={post.id}>
                    <CardBody className="flex flex-col gap-2">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="flex items-center gap-2">
                          {post.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
                          <span className="font-mono text-xs tabular-nums text-ink-faint">
                            {formatShortDateTime(post.created_at)}
                          </span>
                          <span className="text-xs text-ink-sub">{post.like_count} 人觉得有帮助</span>
                        </div>
                        <div className="flex items-center gap-1">
                          <IconButton
                            variant="ghost"
                            size="sm"
                            icon={Pin}
                            aria-label={post.is_pinned ? '取消置顶讨论' : '置顶讨论'}
                            onClick={() => void togglePin(post)}
                          />
                          <IconButton
                            variant="ghost"
                            size="sm"
                            icon={Trash2}
                            aria-label="删除这条讨论"
                            onClick={() => setDeleteTarget(post)}
                          />
                        </div>
                      </div>
                      <p className="whitespace-pre-wrap text-base text-ink">{post.content}</p>
                    </CardBody>
                  </Card>
                ))}
              </div>
              <Pagination
                page={posts.page}
                pageSize={posts.pageSize}
                total={posts.total}
                onPageChange={posts.setPage}
              />
            </>
          )}
        </ResourceState>
      </div>

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认删除这条讨论</ModalTitle>
            <ModalDescription>删除后学生看不到这条内容,操作不可撤销。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="line-clamp-4 text-base text-ink">{deleteTarget?.content}</p>
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(undefined)}>
              取消
            </Button>
            <Button variant="danger" loading={working} onClick={() => void deletePost()}>
              确认删除
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}
