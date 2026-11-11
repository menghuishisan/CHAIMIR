// contestTimeline.test.ts 验证赛程条的阶段判定与时间提示(设计 §7.2 竞赛列表 / 竞赛详情):
// 节点按「报名开始 → 报名截止 → 开赛 → 封榜 → 结束」排列,不封榜时没有封榜节点;
// 当前阶段以赛事状态为准(状态由服务端或教师推进,不一定与时刻表对齐);
// 列表卡片的时间提示:报名中「报名还剩 2 天」,进行中「距结束 3 小时」,已结束显示日期。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { contestCountdown, contestTimeHint, contestTimeline, type TimelineContest } from './contestTimeline.ts'

const SIGNUP = 2
const RUNNING = 3
const FROZEN = 4
const ENDED = 5
const ARCHIVED = 6

const base: TimelineContest = {
  status: SIGNUP,
  signup_start: '2026-11-01T08:00:00+08:00',
  signup_end: '2026-11-12T20:00:00+08:00',
  start_at: '2026-11-15T09:00:00+08:00',
  end_at: '2026-11-15T17:00:00+08:00',
  freeze_minutes: 60,
}

test('节点顺序:报名开始、报名截止、开赛、封榜、结束;封榜时刻 = 结束前 freeze_minutes', () => {
  const nodes = contestTimeline(base)
  assert.deepEqual(nodes.map((node) => node.key), ['signup_start', 'signup_end', 'start', 'freeze', 'end'])
  assert.equal(new Date(nodes[3].at).toISOString(), new Date('2026-11-15T16:00:00+08:00').toISOString())
})

test('不封榜时没有封榜节点', () => {
  assert.deepEqual(
    contestTimeline({ ...base, freeze_minutes: 0 }).map((node) => node.key),
    ['signup_start', 'signup_end', 'start', 'end'],
  )
})

test('当前阶段以状态为准:报名中停在报名开始,进行中停在开赛,封榜停在封榜,结束与归档停在结束', () => {
  const current = (status: number, freeze = 60) =>
    contestTimeline({ ...base, status, freeze_minutes: freeze }).find((node) => node.state === 'current')?.key
  assert.equal(current(SIGNUP), 'signup_start')
  assert.equal(current(RUNNING), 'start')
  assert.equal(current(FROZEN), 'freeze')
  assert.equal(current(ENDED), 'end')
  assert.equal(current(ARCHIVED), 'end')
})

test('当前阶段之前的节点都算已过去,之后的都算未到', () => {
  const states = contestTimeline({ ...base, status: RUNNING }).map((node) => node.state)
  assert.deepEqual(states, ['done', 'done', 'current', 'upcoming', 'upcoming'])
})

test('报名中但已过报名截止时刻:当前阶段停在报名截止(等待开赛)', () => {
  const now = new Date('2026-11-13T10:00:00+08:00')
  const current = contestTimeline(base, now).find((node) => node.state === 'current')?.key
  assert.equal(current, 'signup_end')
})

test('卡片时间提示:报名还剩几天、距开赛、距结束几小时、已结束写日期', () => {
  assert.equal(contestTimeHint(base, new Date('2026-11-10T20:00:00+08:00')), '报名还剩 2 天')
  assert.equal(contestTimeHint(base, new Date('2026-11-12T15:30:00+08:00')), '报名还剩 4 小时')
  assert.equal(contestTimeHint(base, new Date('2026-11-13T10:00:00+08:00')), '报名已截止,2 天后开赛')
  assert.equal(contestTimeHint({ ...base, status: RUNNING }, new Date('2026-11-15T14:00:00+08:00')), '距结束 3 小时')
  assert.equal(contestTimeHint({ ...base, status: FROZEN }, new Date('2026-11-15T16:40:00+08:00')), '距结束 20 分钟')
  assert.equal(contestTimeHint({ ...base, status: ENDED }, new Date('2026-11-20T10:00:00+08:00')), '2026.11.15 结束')
})

test('草稿的时间提示是发布期限:报名截止前须发布,过了截止说明已过', () => {
  const draft = { ...base, status: 1 }
  assert.equal(contestTimeHint(draft, new Date('2026-11-10T20:00:00+08:00')), '2 天内须发布')
  assert.equal(contestTimeHint(draft, new Date('2026-11-12T15:30:00+08:00')), '4 小时内须发布')
  assert.equal(contestTimeHint(draft, new Date('2026-11-12T20:00:01+08:00')), '报名截止时间已过')
})

test('时间读不出来时不报错,提示为空', () => {
  assert.equal(contestTimeHint({ ...base, signup_end: 'bad' }, new Date('2026-11-10T20:00:00+08:00')), undefined)
})

test('答题倒计时:时:分:秒,最后 10 分钟标为紧迫,结束后为 0', () => {
  const endAt = '2026-11-15T17:00:00+08:00'
  assert.deepEqual(contestCountdown(endAt, new Date('2026-11-15T14:58:05+08:00')), { text: '2:01:55', urgent: false, over: false })
  assert.deepEqual(contestCountdown(endAt, new Date('2026-11-15T16:51:00+08:00')), { text: '0:09:00', urgent: true, over: false })
  assert.deepEqual(contestCountdown(endAt, new Date('2026-11-15T17:00:01+08:00')), { text: '0:00:00', urgent: true, over: true })
  assert.equal(contestCountdown('bad', new Date()), undefined)
})
