// contestLifecycle.test.ts 赛事「下一步」判定(与后端 validateContestTransitionWindow 同一口径)。

import assert from 'node:assert/strict'
import { test } from 'node:test'
import { contestNextStep, type LifecycleContest } from './contestLifecycle.ts'

const base: LifecycleContest = {
  status: 1,
  signup_end: '2026-11-12T12:00:00+08:00',
  start_at: '2026-11-13T09:00:00+08:00',
  end_at: '2026-11-13T12:00:00+08:00',
  freeze_minutes: 30,
}

const at = (value: string) => new Date(value)

test('草稿在报名截止前可以发布,截止后不能发布并说明原因', () => {
  assert.deepEqual(contestNextStep(base, at('2026-11-10T10:00:00+08:00')), { action: 'publish', available: true })
  const late = contestNextStep(base, at('2026-11-12T12:00:01+08:00'))
  assert.equal(late?.action, 'publish')
  assert.equal(late?.available, false)
  assert.match(late?.reason ?? '', /报名截止时间已过/)
})

test('报名中:开赛时刻前不能手动开赛,开赛到结束之间可以', () => {
  const signup = { ...base, status: 2 }
  const early = contestNextStep(signup, at('2026-11-13T08:59:00+08:00'))
  assert.equal(early?.action, 'start')
  assert.equal(early?.available, false)
  assert.match(early?.reason ?? '', /自动开始/)
  assert.deepEqual(contestNextStep(signup, at('2026-11-13T09:00:00+08:00')), { action: 'start', available: true })
})

test('进行中且封榜:封榜窗口前不可用,窗口内可以封榜', () => {
  const running = { ...base, status: 3 }
  const early = contestNextStep(running, at('2026-11-13T11:29:00+08:00'))
  assert.equal(early?.action, 'freeze')
  assert.equal(early?.available, false)
  assert.match(early?.reason ?? '', /自动封榜/)
  assert.deepEqual(contestNextStep(running, at('2026-11-13T11:30:00+08:00')), { action: 'freeze', available: true })
})

test('进行中不封榜:下一步是结束,到结束时刻才可用;过了结束时刻即使封榜也是结束', () => {
  const running = { ...base, status: 3, freeze_minutes: 0 }
  const early = contestNextStep(running, at('2026-11-13T11:00:00+08:00'))
  assert.equal(early?.action, 'end')
  assert.equal(early?.available, false)
  assert.match(early?.reason ?? '', /自动结束/)
  assert.deepEqual(contestNextStep(running, at('2026-11-13T12:00:00+08:00')), { action: 'end', available: true })
  assert.deepEqual(contestNextStep({ ...base, status: 3 }, at('2026-11-13T12:05:00+08:00')), { action: 'end', available: true })
})

test('封榜中到结束时刻才能结束;已结束可以归档;已归档没有下一步', () => {
  const frozen = { ...base, status: 4 }
  assert.equal(contestNextStep(frozen, at('2026-11-13T11:45:00+08:00'))?.available, false)
  assert.deepEqual(contestNextStep(frozen, at('2026-11-13T12:00:00+08:00')), { action: 'end', available: true })
  assert.deepEqual(contestNextStep({ ...base, status: 5 }, at('2026-11-14T00:00:00+08:00')), { action: 'archive', available: true })
  assert.equal(contestNextStep({ ...base, status: 6 }, at('2026-11-14T00:00:00+08:00')), undefined)
})

test('时间读不出来时不提供可用的动作', () => {
  const broken = contestNextStep({ ...base, status: 2, start_at: 'not-a-time' }, at('2026-11-13T10:00:00+08:00'))
  assert.equal(broken?.action, 'start')
  assert.equal(broken?.available, false)
  assert.match(broken?.reason ?? '', /时间/)
})
