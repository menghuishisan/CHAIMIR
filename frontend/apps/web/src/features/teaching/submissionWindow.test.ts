// submissionWindow.test.ts 验证作业还能不能提交的判断(设计 §7.2 作业作答 / 作业结果):
// 次数用完、截止后不接收时不能再交并给出原因;截止后允许迟交时仍可提交但标为迟交。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { submissionWindow } from './submissionWindow.ts'

const REJECT = 1
const PENALIZE = 2
const NO_PENALTY = 3
const now = new Date('2026-11-10T10:00:00+08:00')
const future = '2026-11-12T23:59:00+08:00'
const past = '2026-11-09T23:59:00+08:00'

test('截止前且还有次数:可以提交,不算迟交,给出剩余次数', () => {
  assert.deepEqual(
    submissionWindow({ dueAt: future, latePolicy: REJECT, maxAttempts: 3, attemptsUsed: 1 }, now),
    {
      open: true,
      late: false,
      remaining: 2,
      reason: undefined,
    }
  )
})

test('次数用完:不能再交,原因写明共几次', () => {
  const result = submissionWindow(
    { dueAt: future, latePolicy: PENALIZE, maxAttempts: 2, attemptsUsed: 2 },
    now
  )
  assert.equal(result.open, false)
  assert.equal(result.remaining, 0)
  assert.equal(result.reason, '提交次数已用完(共 2 次)')
})

test('截止后不接收:不能再交', () => {
  const result = submissionWindow(
    { dueAt: past, latePolicy: REJECT, maxAttempts: 3, attemptsUsed: 0 },
    now
  )
  assert.equal(result.open, false)
  assert.equal(result.late, true)
  assert.equal(result.reason, '已过截止时间,这份作业不再接收提交')
})

test('截止后允许迟交:仍可提交,标为迟交', () => {
  for (const latePolicy of [PENALIZE, NO_PENALTY]) {
    const result = submissionWindow(
      { dueAt: past, latePolicy, maxAttempts: 3, attemptsUsed: 1 },
      now
    )
    assert.equal(result.open, true)
    assert.equal(result.late, true)
    assert.equal(result.reason, undefined)
  }
})

test('次数用完且已截止:先说次数用完', () => {
  const result = submissionWindow(
    { dueAt: past, latePolicy: REJECT, maxAttempts: 1, attemptsUsed: 1 },
    now
  )
  assert.equal(result.reason, '提交次数已用完(共 1 次)')
})

test('截止时间无法解析:不在前端判迟交,交给服务端判断', () => {
  const result = submissionWindow(
    { dueAt: 'not-a-date', latePolicy: REJECT, maxAttempts: 3, attemptsUsed: 0 },
    now
  )
  assert.equal(result.open, true)
  assert.equal(result.late, false)
})
