// 课程评价卡(课程详情页右栏动作区)。
// 结课/归档后先读取本人已保存评价,读取成功才开放表单;更新后重新读取服务端权威状态。

import { useCallback, useState } from 'react'
import { Star } from 'lucide-react'
import { CourseStatus, type MyCourseReview } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  FormField,
  SegmentedControl,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 评分档位:后端 rating 是 1-5 整数。 */
const RATING_OPTIONS = [
  { value: '1', label: '1 分' },
  { value: '2', label: '2 分' },
  { value: '3', label: '3 分' },
  { value: '4', label: '4 分' },
  { value: '5', label: '5 分' },
]

/** 允许评价的课程状态:结课与归档后才开放(需求规格 F3)。 */
const REVIEWABLE_STATUSES: ReadonlySet<CourseStatus> = new Set([
  CourseStatus.ENDED,
  CourseStatus.ARCHIVED,
])

export interface CourseReviewCardProps {
  courseId: string
  courseStatus: CourseStatus
}

/**
 * CourseReviewCard 按课程阶段开放本人评价读取与编辑。
 */
export function CourseReviewCard({ courseId, courseStatus }: CourseReviewCardProps) {
  if (!REVIEWABLE_STATUSES.has(courseStatus)) {
    return (
      <Card>
        <CardHeader title="课程评价" description="课程结束后可以对这门课打分并留下建议。" />
        <CardBody>
          <Callout tone="info">这门课还在进行中,结课后可以提交评价。</Callout>
        </CardBody>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader title="课程评价" description="给这门课打个分,写下你的建议。" />
      <CardBody>
        <SavedCourseReview key={courseId} courseId={courseId} />
      </CardBody>
    </Card>
  )
}

/** SavedCourseReview 区分未评价和读取失败,课程切换及刷新均隔离旧响应。 */
function SavedCourseReview({ courseId }: { courseId: string }) {
  const review = useAsyncResource(
    () => api.teaching.getMyCourseReview(courseId),
    [courseId],
    () => false
  )

  return (
    <ResourceState
      resource={review}
      emptyIcon={Star}
      emptyTitle="尚未评价"
      emptyDescription="给这门课打个分,写下你的建议。"
    >
      {(data) => (
        <CourseReviewForm
          key={`${courseId}:${review.refreshVersion}`}
          courseId={courseId}
          initialReview={data.review}
          onSaved={review.reload}
        />
      )}
    </ResourceState>
  )
}

/** CourseReviewForm 从已读取评价初始化,不以本地空状态覆盖服务端记录。 */
function CourseReviewForm({
  courseId,
  initialReview,
  onSaved,
}: {
  courseId: string
  initialReview: MyCourseReview['review']
  onSaved: () => void
}) {
  const [rating, setRating] = useState(String(initialReview?.rating ?? 5))
  const [comment, setComment] = useState(initialReview?.comment ?? '')
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  /** handleSubmit 更新本人评价,成功后重读以确认保存结果。 */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (submitting) return
      setFormError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.reviewCourse(courseId, {
          rating: Number(rating),
          comment: comment.trim(),
        })
        toast.success('评价已保存')
        onSaved()
      } catch (reviewError) {
        setFormError(userFacingErrorMessage(reviewError, '评价保存失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [comment, courseId, onSaved, rating, submitting]
  )

  return (
    <form onSubmit={handleSubmit} noValidate>
      {initialReview ? (
        <Callout tone="success" className="mb-4" title="评价已保存">
          你的评分和建议已提交,老师会看到匿名汇总结果。你可以在下方更新评价。
        </Callout>
      ) : null}
      <fieldset disabled={submitting}>
        <FormField label="课程评分" required>
          <SegmentedControl
            aria-label="课程评分"
            size="sm"
            options={RATING_OPTIONS}
            value={rating}
            onValueChange={setRating}
          />
        </FormField>
        <FormField label="评价内容" className="mt-4" helper="可以写课程收获、希望改进的地方">
          <Textarea
            value={comment}
            rows={4}
            placeholder="选填"
            onChange={(event) => setComment(event.target.value)}
          />
        </FormField>
        {formError ? (
          <Callout tone="danger" className="mt-3">
            {formError}
          </Callout>
        ) : null}
        <Button
          type="submit"
          variant="primary"
          leftIcon={Star}
          loading={submitting}
          className="mt-4 w-full"
        >
          {initialReview ? '更新评价' : '提交评价'}
        </Button>
      </fieldset>
    </form>
  )
}
