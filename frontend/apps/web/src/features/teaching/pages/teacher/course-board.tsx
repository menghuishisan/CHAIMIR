// 课程「讨论与公告」标签(教师课程详情)。设计 §7.4:与学生侧同一形态,教师多发布公告、置顶公告与帖子、删除违规帖。
// 公告是列表行(标题、发布时间、置顶标记与置顶开关),点开就地展开全文;下方是讨论流。

import { useState } from 'react'
import { Megaphone, Pin, PinOff, Send } from 'lucide-react'
import type { TeachingAnnouncement } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Checkbox,
  FormField,
  IconButton,
  Input,
  ListRow,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Pagination,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CoursePosts } from '../../components/CoursePosts'

export interface CourseBoardProps {
  courseId: string
}

/** CourseBoard 公告在上、讨论在下(教师可置顶与删除帖子) */
export function CourseBoard({ courseId }: CourseBoardProps) {
  return (
    <div className="flex flex-col gap-8">
      <CourseAnnouncements courseId={courseId} />
      <CoursePosts key={courseId} courseId={courseId} canModerate />
    </div>
  )
}

/** CourseAnnouncements 课程公告:发布、置顶切换,点开就地展开全文 */
function CourseAnnouncements({ courseId }: CourseBoardProps) {
  const [composeOpen, setComposeOpen] = useState(false)
  const [expanded, setExpanded] = useState<string>()
  const [pendingPin, setPendingPin] = useState<string>()
  const [actionError, setActionError] = useState<string>()
  const announcements = usePagedResource<TeachingAnnouncement>(
    (params) => api.teaching.listAnnouncements(courseId, params),
    [courseId],
    { urlPageKey: 'notice_page' }
  )

  /** togglePin 置顶或取消置顶一条公告 */
  const togglePin = async (announcement: TeachingAnnouncement) => {
    if (pendingPin) return
    setPendingPin(announcement.id)
    setActionError(undefined)
    try {
      await api.teaching.pinAnnouncement(announcement.id, !announcement.is_pinned)
      announcements.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '置顶没有改成功,请稍后重试。'))
    } finally {
      setPendingPin(undefined)
    }
  }

  return (
    <section aria-labelledby="teacher-announcements-title">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 id="teacher-announcements-title" className="text-lg font-semibold text-ink">
          课程公告
        </h2>
        <Button
          variant="primary"
          size="sm"
          leftIcon={Megaphone}
          onClick={() => setComposeOpen(true)}
        >
          发布公告
        </Button>
      </div>
      {actionError ? (
        <Callout tone="danger" className="mb-3">
          {actionError}
        </Callout>
      ) : null}
      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="还没有公告"
        emptyDescription="重要通知发成公告,学生进入课程就能看到。"
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
                      trailing={
                        <span className="flex items-center gap-1">
                          {item.is_pinned ? <Badge tone="jade">置顶</Badge> : null}
                          <IconButton
                            variant="ghost"
                            size="sm"
                            icon={item.is_pinned ? PinOff : Pin}
                            aria-label={
                              item.is_pinned
                                ? `取消置顶公告 ${item.title}`
                                : `置顶公告 ${item.title}`
                            }
                            disabled={Boolean(pendingPin)}
                            onClick={() => void togglePin(item)}
                          />
                        </span>
                      }
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
    </section>
  )
}

interface AnnouncementComposeModalProps {
  courseId: string
  onClose: () => void
  onPosted: () => void
}

/** AnnouncementComposeModal 发布课程公告 */
function AnnouncementComposeModal({ courseId, onClose, onPosted }: AnnouncementComposeModalProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [pinned, setPinned] = useState(false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(() => ({
    title: title.trim() ? null : '请输入公告标题',
    content: content.trim() ? null : '请输入公告内容',
  }))

  /** submit 发布公告 */
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (working || !validate()) return
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
  }

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>发布课程公告</ModalTitle>
          <ModalDescription>
            发布后学生进入课程就能看到,最新的两条会显示在课程首页右栏。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="公告标题" htmlFor="announcement-title" required error={errors.title}>
              <Input
                id="announcement-title"
                value={title}
                invalid={Boolean(errors.title)}
                onBlur={() => validate(['title'])}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>
            <FormField
              label="公告内容"
              htmlFor="announcement-content"
              required
              helper="换行会原样保留"
              error={errors.content}
            >
              <Textarea
                id="announcement-content"
                value={content}
                rows={6}
                invalid={Boolean(errors.content)}
                onBlur={() => validate(['content'])}
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
            <Button type="button" variant="outline" disabled={working} onClick={onClose}>
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
