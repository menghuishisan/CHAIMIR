// 竞赛列表复用正式状态选项；排序仍优先学生已参赛赛事和教师待办。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  sortStudentContests,
  sortTeacherContests,
  type OrderContest,
  type TeacherOrderContest,
} from './contestOrder.ts'
import { ContestStatus } from '@chaimir/api-client'
import { CONTEST_STATUS_OPTIONS, contestStatusLabel } from '../../utils/labels/contest.ts'

const DRAFT = 1
const SIGNUP = 2
const RUNNING = 3
const FROZEN = 4
const ENDED = 5
const ARCHIVED = 6

/** contest 造一场只含排序字段的赛事 */
function contest(id: string, status: number, signupEnd: string, endAt: string): OrderContest {
  return { id, status, signup_end: signupEnd, end_at: endAt }
}

test('正式状态选项覆盖SDK枚举且不合并封榜和归档', () => {
  const statuses = Object.values(ContestStatus).filter((status): status is ContestStatus => typeof status === 'number')
  assert.deepEqual(CONTEST_STATUS_OPTIONS, statuses.map((status) => ({ value: String(status), label: contestStatusLabel(status) })))
  assert.equal(CONTEST_STATUS_OPTIONS.find((option) => option.value === String(ContestStatus.FROZEN))?.label, '封榜中')
  assert.equal(CONTEST_STATUS_OPTIONS.find((option) => option.value === String(ContestStatus.ARCHIVED))?.label, '已归档')
})

test('我已报名且进行中的排最前,然后报名中、其余进行中、已结束', () => {
  const list = [
    contest('ended-old', ENDED, '2026-01-01', '2026-02-01'),
    contest('running-other', RUNNING, '2026-10-01', '2026-11-20'),
    contest('signup-late', SIGNUP, '2026-11-30', '2026-12-10'),
    contest('frozen-mine', FROZEN, '2026-10-01', '2026-11-11'),
    contest('signup-soon', SIGNUP, '2026-11-12', '2026-11-20'),
    contest('signup-mine', SIGNUP, '2026-12-20', '2026-12-30'),
    contest('archived-recent', ARCHIVED, '2026-09-01', '2026-10-01'),
  ]
  const joined = new Set(['frozen-mine', 'signup-mine'])
  assert.deepEqual(
    sortStudentContests(list, joined).map((item) => item.id),
    ['frozen-mine', 'signup-mine', 'signup-soon', 'signup-late', 'running-other', 'archived-recent', 'ended-old'],
  )
})

test('不改变传入的数组', () => {
  const list = [contest('a', ENDED, '2026-01-01', '2026-01-02'), contest('b', SIGNUP, '2026-12-01', '2026-12-02')]
  sortStudentContests(list, new Set())
  assert.deepEqual(list.map((item) => item.id), ['a', 'b'])
})

/** teacherContest 造一场含教师排序字段的赛事 */
function teacherContest(id: string, status: number, signupEnd: string, startAt: string, endAt: string): TeacherOrderContest {
  return { id, status, signup_end: signupEnd, start_at: startAt, end_at: endAt }
}

test('教师排序:进行中(先结束的在前)→ 报名中(先开赛的在前)→ 草稿(发布期限近的在前)→ 已结束 → 已归档', () => {
  const list = [
    teacherContest('archived', ARCHIVED, '2026-09-01', '2026-09-10', '2026-09-11'),
    teacherContest('draft-late', DRAFT, '2026-12-20', '2026-12-25', '2026-12-26'),
    teacherContest('ended-old', ENDED, '2026-10-01', '2026-10-05', '2026-10-06'),
    teacherContest('signup-late', SIGNUP, '2026-11-20', '2026-11-28', '2026-11-29'),
    teacherContest('running-late', RUNNING, '2026-11-01', '2026-11-09', '2026-11-12'),
    teacherContest('ended-recent', ENDED, '2026-10-20', '2026-11-01', '2026-11-02'),
    teacherContest('draft-soon', DRAFT, '2026-11-15', '2026-11-18', '2026-11-19'),
    teacherContest('frozen-soon', FROZEN, '2026-11-01', '2026-11-09', '2026-11-10'),
    teacherContest('signup-soon', SIGNUP, '2026-11-12', '2026-11-14', '2026-11-15'),
  ]
  assert.deepEqual(
    sortTeacherContests(list).map((item) => item.id),
    ['frozen-soon', 'running-late', 'signup-soon', 'signup-late', 'draft-soon', 'draft-late', 'ended-recent', 'ended-old', 'archived'],
  )
})
