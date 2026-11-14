// reportStatus.test.ts 验证成绩报送页的课程报送状态(设计 §7.4 成绩报送):
// 每门课取最近一次报送的状态,没有报送过为「未报送」;默认选中第一门未报送(或被驳回待重报)的课;
// 待审核、已通过时不能再报送并给出原因。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { courseReportStates, defaultReportCourse, reportBlockedReason } from './reportStatus.ts'

const PENDING = 1
const APPROVED = 2
const REJECTED = 3

const reviews = [
  { course_id: 'c1', status: REJECTED, submitted_at: '2026-06-01T10:00:00Z', comment: '第三次作业成绩漏录' },
  { course_id: 'c1', status: PENDING, submitted_at: '2026-06-05T10:00:00Z' },
  { course_id: 'c2', status: APPROVED, submitted_at: '2026-06-02T10:00:00Z' },
  { course_id: 'c3', status: REJECTED, submitted_at: '2026-06-03T10:00:00Z', comment: '权重合计不是 100%' },
]

test('每门课取最近一次报送;没报送过的不在结果里', () => {
  const states = courseReportStates(reviews)
  assert.equal(states.get('c1')?.status, PENDING)
  assert.equal(states.get('c2')?.status, APPROVED)
  assert.equal(states.get('c3')?.comment, '权重合计不是 100%')
  assert.equal(states.has('c4'), false)
})

test('默认选中第一门未报送或被驳回的课;都报过了就选第一门', () => {
  const states = courseReportStates(reviews)
  assert.equal(defaultReportCourse(['c1', 'c2', 'c3', 'c4'], states), 'c3')
  assert.equal(defaultReportCourse(['c1', 'c4'], states), 'c4')
  assert.equal(defaultReportCourse(['c1', 'c2'], states), 'c1')
  assert.equal(defaultReportCourse([], states), undefined)
})

test('待审核、已通过时不能报送并写原因;未报送、已驳回可以', () => {
  const states = courseReportStates(reviews)
  assert.equal(reportBlockedReason(states.get('c1')), '已报送,正在等待学校审核')
  assert.equal(reportBlockedReason(states.get('c2')), '学校已审核通过,成绩已锁定;需要改分请先联系学校解锁')
  assert.equal(reportBlockedReason(states.get('c3')), undefined)
  assert.equal(reportBlockedReason(undefined), undefined)
})
