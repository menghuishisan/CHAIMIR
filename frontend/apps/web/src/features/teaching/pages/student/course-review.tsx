// 课程评价卡(学生课程详情右栏)。设计 §7.2:只在课程已结课(或已归档)时出现;
// 没评过显示「给这门课打分」的表单,评过之后显示自己的评分和建议,并给「修改」。
// 结课前不显示任何卡片(「不能评价」卡片已去掉)。保存后重读服务端记录作为权威。

import { useState } from 'react'
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
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 评分档位:后端 rating 是 1-5 的整数 */
const RATING_OPTIONS = [1, 2, 3, 4, 5].map((value) => ({
  value: String(value),
  label: `${value} 分`,
}))

export interface CourseReviewCardProps {
  courseId: string
  courseStatus: CourseStatus
}

/** CourseReviewCard 已结课时的评价卡;课程还在进行时不渲染 */
export function CourseReviewCard({ courseId, courseStatus }: CourseReviewCardProps) {
  if (courseStatus !== CourseStatus.ENDED && courseStatus !== CourseStatus.ARCHIVED) return null
  return <CourseReview key={courseId} courseId={courseId} />
}

/** CourseReview 读取本人评价:没评过直接给表单,评过先显示评分,点「修改」再编辑 */
function CourseReview({ courseId }: { courseId: string }) {
  const review = useAsyncResource(
    () => api.teaching.getMyCourseReview(courseId),
    [courseId],
    () => false
  )
  const [editing, setEditing] = useState(false)
  const saved = review.data?.review ?? null

  return (
    <Card>
      <CardHeader
        title={saved && !editing ? '我的课程评价' : '给这门课打分'}
        actions={
          saved && !editing ? (
            <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
              修改
            </Button>
          ) : undefined
        }
      />
      <CardBody>
        {review.status === 'loading' ? (
          <Skeleton variant="line" lines={3} />
        ) : review.status === 'error' ? (
          <ResourceError
            message={review.error?.message ?? '评价暂时读不到'}
            traceId={review.error?.traceId}
            onRetry={review.reload}
          />
        ) : saved && !editing ? (
          <SavedReview review={saved} />
        ) : (
          <CourseReviewForm
            key={`${courseId}:${review.refreshVersion}`}
            courseId={courseId}
            initialReview={saved}
            onCancel={saved ? () => setEditing(false) : undefined}
            onSaved={() => {
              setEditing(false)
              review.reload()
            }}
          />
        )}
      </CardBody>
    </Card>
  )
}

/** SavedReview 已保存的评分与建议 */
function SavedReview({ review }: { review: NonNullable<MyCourseReview['review']> }) {
  return (
    <div className="flex flex-col gap-2">
      <p className="flex items-center gap-1" aria-label={`评分 ${review.rating} 分`}>
        {[1, 2, 3, 4, 5].map((value) => (
          <Star
            key={value}
            aria-hidden="true"
            className={
              value <= review.rating ? 'size-4 fill-primary text-primary' : 'size-4 text-ink-faint'
            }
          />
        ))}
        <span className="ml-1 text-sm font-medium text-ink">{review.rating} 分</span>
      </p>
      {review.comment ? (
        <p className="whitespace-pre-wrap text-sm text-ink">{review.comment}</p>
      ) : (
        <p className="text-sm text-ink-sub">没有写建议。</p>
      )}
      <p className="text-xs text-ink-sub">老师只看到匿名汇总。</p>
    </div>
  )
}

interface CourseReviewFormProps {
  courseId: string
  initialReview: MyCourseReview['review']
  onCancel?: () => void
  onSaved: () => void
}

/** CourseReviewForm 从已读取的评价初始化,保存后由调用方重读 */
function CourseReviewForm({ courseId, initialReview, onCancel, onSaved }: CourseReviewFormProps) {
  const [rating, setRating] = useState(String(initialReview?.rating ?? 5))
  const [comment, setComment] = useState(initialReview?.comment ?? '')
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  /** submit 保存本人评价 */
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (submitting) return
    setFormError(undefined)
    setSubmitting(true)
    try {
      await api.teaching.reviewCourse(courseId, { rating: Number(rating), comment: comment.trim() })
      toast.success('评价已保存')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '评价没有保存成功,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <fieldset disabled={submitting} className="flex flex-col gap-4">
        <FormField label="评分" required>
          <SegmentedControl
            aria-label="课程评分"
            size="sm"
            options={RATING_OPTIONS}
            value={rating}
            onValueChange={setRating}
          />
        </FormField>
        <FormField label="建议" helper="选填,可以写收获和希望改进的地方;老师只看到匿名汇总">
          <Textarea value={comment} rows={3} onChange={(event) => setComment(event.target.value)} />
        </FormField>
        {formError ? <Callout tone="danger">{formError}</Callout> : null}
        <div className="flex flex-wrap items-center gap-2">
          <Button type="submit" variant="primary" leftIcon={Star} loading={submitting}>
            {initialReview ? '保存修改' : '提交评价'}
          </Button>
          {onCancel ? (
            <Button type="button" variant="ghost" onClick={onCancel}>
              取消
            </Button>
          ) : null}
        </div>
      </fieldset>
    </form>
  )
}
