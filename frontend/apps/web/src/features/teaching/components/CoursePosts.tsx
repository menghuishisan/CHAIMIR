// 教师和学生共用的课程讨论流;发帖、回复、点赞和分页只有这一套实现。
// 教师管理入口由页面显式启用,真实权限继续由服务端课程归属和角色守卫校验。

import { useState } from 'react'
import type { TeachingPost } from '@chaimir/api-client'
import { MessageSquare, Pin, Reply, ThumbsUp, Trash2 } from 'lucide-react'
import {
  Badge, Button, Callout, Card, CardBody, IconButton,
  Modal, ModalBody, ModalContent, ModalDescription, ModalFooter, ModalHeader, ModalTitle,
  PageSection, Pagination, Skeleton, toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { formatShortDateTime } from '../../../utils/formatters'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { CoursePostComposer } from './CoursePostComposer'

interface CoursePostsProps {
  courseId: string
  canModerate?: boolean
}

/** CoursePosts 在同一分页流显示主帖、回复及受控引用,操作后重读服务端记录。 */
export function CoursePosts({ courseId, canModerate = false }: CoursePostsProps) {
  const posts = usePagedResource<TeachingPost>(
    (params) => api.teaching.listPosts(courseId, params),
    [courseId],
  )
  const [replyTarget, setReplyTarget] = useState<TeachingPost>()
  const [replySubmitting, setReplySubmitting] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<TeachingPost>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  /** publishComplete 回到第一页并刷新,不以本地拼接记录替代服务端结果。 */
  function publishComplete() {
    setReplyTarget(undefined)
    posts.setPage(1)
    posts.reload()
  }

  /** deletePost 确认删除当前内容,失败留在确认框内显示原因。 */
  async function deletePost() {
    if (!canModerate || !deleteTarget || working) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.teaching.deletePost(deleteTarget.id)
      toast.success('已删除')
      setDeleteTarget(undefined)
      posts.reload()
    } catch (cause) {
      setActionError(userFacingErrorMessage(cause, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  return (
    <PageSection title="课程讨论" description={`共 ${posts.total ?? '—'} 条。可以提出问题,回复老师和同学。`}>
      <div className="flex flex-col gap-4">
        <Card>
          <CardBody>
            <CoursePostComposer courseId={courseId} onPosted={publishComplete} />
          </CardBody>
        </Card>
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
                <CoursePostCard
                  key={post.id}
                  post={post}
                  canModerate={canModerate}
                  onChanged={posts.reload}
                  onReply={() => setReplyTarget(post)}
                  onDelete={() => {
                    setActionError(undefined)
                    setDeleteTarget(post)
                  }}
                />
              ))}
              <Pagination page={posts.page} pageSize={posts.pageSize} total={posts.total} onPageChange={posts.setPage} />
            </div>
          )}
        </ResourceState>
      </div>

      <Modal open={replyTarget !== undefined} onOpenChange={(open) => !replySubmitting && !open && setReplyTarget(undefined)}>
        <ModalContent size="lg">
          <ModalHeader>
            <ModalTitle>回复讨论</ModalTitle>
            <ModalDescription>回复发布后,课程里的老师和同学都能看到。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            {replyTarget ? (
              <CoursePostComposer key={replyTarget.id} courseId={courseId} parent={replyTarget} onPosted={publishComplete} onSubmittingChange={setReplySubmitting} />
            ) : null}
          </ModalBody>
        </ModalContent>
      </Modal>

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !working && !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认删除这条讨论</ModalTitle>
            <ModalDescription>删除后无法恢复。已有回复会保留,但不再显示这条原文。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="line-clamp-4 whitespace-pre-wrap break-words text-base text-ink">{deleteTarget?.content}</p>
            {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" disabled={working} onClick={() => setDeleteTarget(undefined)}>取消</Button>
            <Button variant="danger" loading={working} onClick={() => void deletePost()}>确认删除</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}

interface CoursePostCardProps {
  post: TeachingPost
  canModerate: boolean
  onChanged: () => void
  onReply: () => void
  onDelete: () => void
}

/** CoursePostCard 用文本节点显示正文和引用,点赞/置顶失败在本卡就近提示。 */
function CoursePostCard({ post, canModerate, onChanged, onReply, onDelete }: CoursePostCardProps) {
  const [pendingAction, setPendingAction] = useState<'like' | 'pin'>()
  const [error, setError] = useState<string>()

  /** changePost 互斥执行本卡动作,只在服务端完成后刷新记录。 */
  async function changePost(action: 'like' | 'pin') {
    if (pendingAction || (action === 'pin' && !canModerate)) return
    setPendingAction(action)
    setError(undefined)
    try {
      if (action === 'like') await api.teaching.likePost(post.id)
      else await api.teaching.pinPost(post.id, !post.is_pinned)
      onChanged()
    } catch (cause) {
      setError(userFacingErrorMessage(cause, '操作没有完成,请稍后重试。'))
    } finally {
      setPendingAction(undefined)
    }
  }

  return (
    <Card>
      <CardBody className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            {post.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
            {post.parent_id ? <Badge tone="neutral">回复</Badge> : null}
            <span className="font-mono text-xs tabular-nums text-ink-faint">{formatShortDateTime(post.created_at)}</span>
          </div>
          {canModerate ? (
            <div className="flex items-center gap-1">
              <IconButton variant="ghost" size="sm" icon={Pin} aria-label={post.is_pinned ? '取消置顶讨论' : '置顶讨论'} disabled={Boolean(pendingAction)} onClick={() => void changePost('pin')} />
              <IconButton variant="ghost" size="sm" icon={Trash2} aria-label="删除这条讨论" onClick={onDelete} />
            </div>
          ) : null}
        </div>
        {post.parent_id ? (
          <blockquote className="whitespace-pre-wrap break-words border-l-2 border-primary pl-3 text-sm text-ink-sub">
            <span className="mb-1 block text-xs text-ink-faint">回复的内容</span>
            {post.parent_content ?? '原讨论已移除'}
          </blockquote>
        ) : null}
        <p className="whitespace-pre-wrap break-words text-base text-ink">{post.content}</p>
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="ghost" size="sm" leftIcon={Reply} onClick={onReply}>回复</Button>
          <Button variant="ghost" size="sm" leftIcon={ThumbsUp} loading={pendingAction === 'like'} disabled={pendingAction === 'pin'} onClick={() => void changePost('like')}>
            有帮助 {post.like_count}
          </Button>
        </div>
        {error ? <Callout tone="danger">{error}</Callout> : null}
      </CardBody>
    </Card>
  )
}
