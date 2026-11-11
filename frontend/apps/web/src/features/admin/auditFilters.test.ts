// auditFilters.test.ts 验证审计页地址里的起止日期换成查询时间:按本地时区取整天,颠倒时对调,格式不对的忽略。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { auditActionFilter, auditActorFilter, auditDayRange } from './auditFilters.ts'

const startOf = (year: number, month: number, day: number) => new Date(year, month - 1, day, 0, 0, 0, 0).toISOString()
const endOf = (year: number, month: number, day: number) => new Date(year, month - 1, day, 23, 59, 59, 999).toISOString()

test('起止日期取本地时区的整天', () => {
  assert.deepEqual(auditDayRange('2026-11-01', '2026-11-10'), {
    from: '2026-11-01',
    to: '2026-11-10',
    params: { from: startOf(2026, 11, 1), to: endOf(2026, 11, 10) },
  })
})

test('只给一端时只限那一端', () => {
  assert.deepEqual(auditDayRange('2026-11-01', ''), { from: '2026-11-01', to: '', params: { from: startOf(2026, 11, 1) } })
  assert.deepEqual(auditDayRange('', '2026-11-10'), { from: '', to: '2026-11-10', params: { to: endOf(2026, 11, 10) } })
})

test('开始晚于结束时对调', () => {
  assert.deepEqual(auditDayRange('2026-11-10', '2026-11-01'), {
    from: '2026-11-01',
    to: '2026-11-10',
    params: { from: startOf(2026, 11, 1), to: endOf(2026, 11, 10) },
  })
})

test('格式不对或不存在的日期忽略', () => {
  assert.deepEqual(auditDayRange('2026/11/01', 'abc'), { from: '', to: '', params: {} })
  assert.deepEqual(auditDayRange('2026-02-30', '2026-13-01'), { from: '', to: '', params: {} })
})

test('动作参数只接受点分的小写标识', () => {
  assert.equal(auditActionFilter('auth.login'), 'auth.login')
  assert.equal(auditActionFilter('contest.battle.entry.submit'), 'contest.battle.entry.submit')
  assert.equal(auditActionFilter('all'), undefined)
  assert.equal(auditActionFilter('auth'), undefined)
  assert.equal(auditActionFilter('DROP TABLE'), undefined)
  assert.equal(auditActionFilter(`${'a'.repeat(64)}.b`), undefined)
})

test('操作人参数只接受正的账号编号', () => {
  assert.equal(auditActorFilter('1234567890123'), '1234567890123')
  assert.equal(auditActorFilter('9223372036854775807'), '9223372036854775807')
  assert.equal(auditActorFilter('9223372036854775808'), undefined)
  for (const raw of ['', '0', '-1', 'abc', '12a', '0123']) assert.equal(auditActorFilter(raw), undefined)
})
