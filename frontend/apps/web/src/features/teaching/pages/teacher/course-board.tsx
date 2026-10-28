// 课程讨论与公告(课程详情页内区块,教师视角)。
// 教师比学生多三个动作:置顶帖子、删除违规帖、发布公告并置顶。
// 删除是不可逆的,故走确认弹层而不是 window.confirm(规范:危险操作需确认且视觉分离)。

import { useCallback, useState } from 'react'
import { Megaphone, Pin, Send } from 'lucide-react'
import type { TeachingAnnouncement } from '@chaimir/api-client'
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
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatShortDateTime } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CoursePosts } from '../../components/CoursePosts'

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
      <CoursePosts key={courseId} courseId={courseId} canModerate />
    </>
  )
}

/**
 * CourseAnnouncements 发布与置顶课程公告。
 */
function CourseAnnouncements({ courseId }: CourseBoardProps) {
  const [composeOpen, setComposeOpen] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const [pendingPin, setPendingPin] = useState<string>()

	const announcements = usePagedResource<TeachingAnnouncement>((params) => api.teaching.listAnnouncements(courseId, params), [courseId])

  const togglePin = useCallback(
    async (announcement: TeachingAnnouncement) => {
      if (pendingPin) return
      setPendingPin(announcement.id)
      setActionError(undefined)
      try {
        await api.teaching.pinAnnouncement(announcement.id, !announcement.is_pinned)
        announcements.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
      } finally {
        setPendingPin(undefined)
      }
    },
    [announcements, pendingPin],
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
                        disabled={Boolean(pendingPin)}
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
  const { errors, validate } = useFormValidation(() => ({
    title: title.trim() ? null : '请输入公告标题',
    content: content.trim() ? null : '请输入公告内容',
  }))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
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
    },
    [content, courseId, onPosted, pinned, title, validate, working],
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
            <FormField label="公告标题" htmlFor="announcement-title" required error={errors.title}>
              <Input
                id="announcement-title"
                value={title}
                invalid={Boolean(errors.title)}
                onBlur={() => validate(['title'])}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>
            <FormField label="公告内容" htmlFor="announcement-content" required error={errors.content}>
              <Textarea
                id="announcement-content"
                value={content}
                invalid={Boolean(errors.content)}
                onBlur={() => validate(['content'])}
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
