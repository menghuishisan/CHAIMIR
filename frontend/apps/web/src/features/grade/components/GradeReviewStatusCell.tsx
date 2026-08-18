// GradeReviewStatusCell 统一成绩报送在教师与校管视角下的状态展示。

import type { GradeReview } from '@chaimir/api-client'
import { Badge, StatusIndicator } from '@chaimir/ui'
import { gradeReviewStatusLabel } from '../../../utils/labels/grade'
import { gradeReviewStatusTone } from '../statusPresentation'

export interface GradeReviewStatusCellProps {
  review: GradeReview
}

/** GradeReviewStatusCell 展示审核状态,并显式标记已锁定的成绩。 */
export function GradeReviewStatusCell({ review }: GradeReviewStatusCellProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <StatusIndicator
        tone={gradeReviewStatusTone(review.status)}
        label={gradeReviewStatusLabel(review.status)}
      />
      {review.is_locked ? <Badge tone="neutral">已锁定</Badge> : null}
    </div>
  )
}
