// 课程评价卡(课程详情页右栏动作区)。
// 后端 POST /teaching/courses/{id}/review 是 upsert(同一学生同一课程覆盖上次评价),
// 且需求规格 F3 明确「课程结束后学生评分评论」,故仅在结课/归档状态开放评价入口。

import { useCallback, useState } from 'react'
import { Star } from 'lucide-react'
import { CourseStatus } from '@chaimir/api-client'
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
 * CourseReviewCard 提交课程评价。
 * 未结课时不给表单,而是说明何时可评 —— 给一个提交必然失败的表单是更差的体验。
 */
export function CourseReviewCard({ courseId, courseStatus }: CourseReviewCardProps) {
  const [rating, setRating] = useState('5')
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      setFormError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.reviewCourse(courseId, { rating: Number(rating), comment: comment.trim() })
        setSubmitted(true)
        toast.success('评价已提交')
      } catch (reviewError) {
        setFormError(userFacingErrorMessage(reviewError, '评价提交失败,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [comment, courseId, rating],
  )

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

  if (submitted) {
    return (
      <Card>
        <CardHeader title="课程评价" />
        <CardBody>
          <Callout tone="success" title="感谢你的评价">
            你的评分和建议已提交,老师会看到匿名汇总结果。
          </Callout>
        </CardBody>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader title="课程评价" description="给这门课打个分,写下你的建议。" />
      <CardBody>
        <form onSubmit={handleSubmit} noValidate>
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
          <Button type="submit" variant="seal" leftIcon={Star} loading={submitting} className="mt-4 w-full">
            提交评价
          </Button>
        </form>
      </CardBody>
    </Card>
  )
}
