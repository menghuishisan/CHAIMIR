// 成绩审核页(校管侧栏,/school-admin/approvals)。
//
// 教师报送 → 学校审核 → 通过即锁定。锁定后教师不能改分,要改必须先解锁。
// 三个动作后果不同:通过会锁定并让成绩进入学生的成绩中心;驳回退回教师修改;
// 解锁把已通过的成绩重新打开(通常是申诉受理后的补救),故各自确认并说明。

import { useCallback, useMemo, useState } from 'react'
import { CircleCheck, CircleX, LockOpen, Send } from 'lucide-react'
import {
  GradeReviewStatus,
  type Course,
  type GradeReview,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DescriptionList,
  FormField,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Stat,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { gradeReviewStatusLabel, gradeReviewStatusTone } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { TranscriptBatchSection } from './transcript-batch'

/** 课程选择器一次取回的条数:后端分页上限 100。 */
const COURSE_PICKER_SIZE = 100

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(GradeReviewStatus.PENDING), label: '待审核' },
  { value: String(GradeReviewStatus.APPROVED), label: '已通过' },
  { value: String(GradeReviewStatus.REJECTED), label: '已驳回' },
] as const

/** 审核动作:三者后果不同,文案分别说明。 */
type ReviewAction = 'approve' | 'reject' | 'unlock'

const ACTION_COPY: Record<ReviewAction, { title: string; description: string; confirm: string; danger?: boolean }> = {
  approve: {
    title: '通过成绩审核',
    description:
      '通过后这门课的成绩会锁定并进入学生的成绩中心。锁定后教师不能再调分,需要改分要先解锁。',
    confirm: '确认通过',
  },
  reject: {
    title: '驳回成绩审核',
    description: '驳回后教师可以修改成绩并重新报送。请在意见里写清需要调整什么。',
    confirm: '确认驳回',
  },
  unlock: {
    title: '解锁成绩',
    description:
      '解锁把已通过的成绩重新打开,教师可以调分后重新报送。通常在受理学生申诉后使用。',
    confirm: '确认解锁',
    danger: true,
  },
}

/**
 * SchoolAdminApprovalsPage 审核教师报送的课程成绩。
 */
export default function SchoolAdminApprovalsPage() {
  const [statusFilter, setStatusFilter] = useState<string>(String(GradeReviewStatus.PENDING))
  const [target, setTarget] = useState<{ review: GradeReview; action: ReviewAction }>()

  const reviews = usePagedResource<GradeReview>(
    (params) =>
      api.grade.listReviews({
        status: statusFilter ? (Number(statusFilter) as GradeReviewStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  // 审核记录只回 course_id / semester_id,课程名与学期名在此解析,不把内部编号抛到界面上
  const courses = useAsyncResource(
    () => api.teaching.getCourses({ page: 1, size: COURSE_PICKER_SIZE }),
    [],
    () => false,
  )

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data],
  )

  const semesterNameById = useMemo(
    () => new Map((semesters.data ?? []).map((semester: Semester) => [semester.id, semester.name])),
    [semesters.data],
  )

  const stats = useMemo(() => {
    const list = reviews.data ? reviews.data.list : []
    return {
      pending: list.filter((item) => item.status === GradeReviewStatus.PENDING).length,
      approved: list.filter((item) => item.status === GradeReviewStatus.APPROVED).length,
      locked: list.filter((item) => item.is_locked).length,
    }
  }, [reviews.data])

  const columns: TableColumn<GradeReview>[] = [
    {
      key: 'course_id',
      header: '课程',
      render: (review) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">
            {courseNameById.get(review.course_id) ?? '已归档的课程'}
          </div>
          <div className="truncate text-xs text-ink-sub">
            {review.semester_id ? (semesterNameById.get(review.semester_id) ?? '未登记学期') : '按课程学期'}
          </div>
        </div>
      ),
    },
    {
      key: 'submitted_at',
      header: '报送时间',
      render: (review) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(review.submitted_at)}
        </span>
      ),
    },
    {
      key: 'comment',
      header: '说明',
      render: (review) =>
        review.comment ? (
          <span className="line-clamp-2 text-sm text-ink-sub">{review.comment}</span>
        ) : (
          <span className="text-ink-sub">未填写</span>
        ),
    },
    {
      key: 'status',
      header: '状态',
      render: (review) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={gradeReviewStatusTone(review.status)}
            label={gradeReviewStatusLabel(review.status)}
          />
          {review.is_locked ? <Badge tone="neutral">已锁定</Badge> : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (review) => (
        <div className="flex items-center justify-end gap-1">
          {review.status === GradeReviewStatus.PENDING ? (
            <>
              <Button
                variant="ghost"
                size="sm"
                leftIcon={CircleCheck}
                onClick={() => setTarget({ review, action: 'approve' })}
              >
                通过
              </Button>
              <Button
                variant="ghost"
                size="sm"
                leftIcon={CircleX}
                onClick={() => setTarget({ review, action: 'reject' })}
              >
                驳回
              </Button>
            </>
          ) : review.is_locked ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={LockOpen}
              onClick={() => setTarget({ review, action: 'unlock' })}
            >
              解锁
            </Button>
          ) : (
            <span className="text-sm text-ink-faint">已处理</span>
          )}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '教务与成绩' }, { label: '成绩审核' }]} />}
        title="成绩审核"
        description="教师报送的课程成绩在这里审核。通过后成绩锁定并进入学生的成绩中心。"
        icon={CircleCheck}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="报送记录" value={reviews.total} icon={Send} />
          <Stat label="待审核" value={stats.pending} icon={CircleCheck} hint="需要你处理" />
          <Stat label="已通过" value={stats.approved} icon={CircleCheck} />
          <Stat label="已锁定" value={stats.locked} icon={LockOpen} hint="教师不能再调分" />
        </div>
      </PageSection>

      <PageSection
        title="报送记录"
        description={`共 ${reviews.total} 条`}
        actions={
          <SegmentedControl
            aria-label="按审核状态筛选"
            size="sm"
            options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
            value={statusFilter}
            onValueChange={setStatusFilter}
          />
        }
      >
        <ResourceState
          resource={reviews}
          emptyIcon={Send}
          emptyTitle={statusFilter ? '这个状态下没有记录' : '还没有成绩报送'}
          emptyDescription={
            statusFilter ? '换个状态看看。' : '教师在教学端确认课程成绩后报送,记录会出现在这里。'
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              <Pagination
                page={reviews.page}
                pageSize={reviews.pageSize}
                total={reviews.total}
                onPageChange={reviews.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>

      {/* 批量成绩单:出成绩单的前提是成绩已锁定,故归到本页而不是单独放侧栏 */}
      <TranscriptBatchSection />

      {target ? (
        <ReviewDecisionModal
          review={target.review}
          action={target.action}
          courseName={courseNameById.get(target.review.course_id) ?? '该课程'}
          semesters={semesters.data ?? []}
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            reviews.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface ReviewDecisionModalProps {
  review: GradeReview
  action: ReviewAction
  courseName: string
  semesters: Semester[]
  onClose: () => void
  onSaved: () => void
}

/**
 * ReviewDecisionModal 提交通过、驳回或解锁决定。
 * 通过时可以指定归档学期(教师报送时可缺省),驳回与解锁只需意见。
 */
function ReviewDecisionModal({
  review,
  action,
  courseName,
  semesters,
  onClose,
  onSaved,
}: ReviewDecisionModalProps) {
  const [semesterId, setSemesterId] = useState(review.semester_id ?? '')
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const copy = ACTION_COPY[action]
  const needsComment = action !== 'approve'

  const submit = useCallback(async () => {
    if (needsComment && comment.trim() === '') {
      setFormError(action === 'reject' ? '请写下驳回理由,教师需要知道改什么' : '请写下解锁原因,记录会留档')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      const payload = {
        comment: comment.trim() || undefined,
        semester_id: action === 'approve' ? semesterId || undefined : undefined,
      }
      if (action === 'approve') await api.grade.approveReview(review.id, payload)
      if (action === 'reject') await api.grade.rejectReview(review.id, payload)
      if (action === 'unlock') await api.grade.unlockReview(review.id, payload)
      toast.success(
        action === 'approve' ? '已通过,成绩已锁定' : action === 'reject' ? '已驳回' : '已解锁',
      )
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [action, comment, needsComment, onSaved, review.id, semesterId])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{copy.title}</ModalTitle>
          <ModalDescription>{copy.description}</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '课程', description: courseName },
              { term: '报送时间', description: formatDateTime(review.submitted_at), mono: true },
              { term: '教师说明', description: review.comment || '未填写' },
            ]}
          />

          {action === 'approve' ? (
            <FormField
              label="归档学期"
              htmlFor="approve-semester"
              helper="不选则按教师报送时的学期归属"
            >
              <Select
                id="approve-semester"
                options={[
                  { value: '', label: '按报送时的学期' },
                  ...semesters.map((semester) => ({ value: semester.id, label: semester.name })),
                ]}
                value={semesterId}
                placeholder="按报送时的学期"
                onValueChange={setSemesterId}
              />
            </FormField>
          ) : null}

          <FormField
            label="审核意见"
            htmlFor="review-comment"
            required={needsComment}
            error={formError}
            helper={
              action === 'reject'
                ? '写清需要教师调整什么,教师会看到这段话'
                : action === 'unlock'
                  ? '写清解锁原因,记录会计入审计'
                  : '可以留空'
            }
          >
            <Textarea
              id="review-comment"
              value={comment}
              rows={4}
              invalid={Boolean(formError)}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {action === 'unlock' ? (
            <Callout tone="warning">
              解锁后学生的成绩中心会暂时看不到这门课的最终成绩,教师重新报送并通过后恢复。
            </Callout>
          ) : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant={copy.danger ? 'danger' : 'seal'} loading={working} onClick={() => void submit()}>
            {copy.confirm}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
