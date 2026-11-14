// tenantExpiry.test.ts 验证学校服务到期的说法:长期有效、已过期、30 天内到期(含今天)、更远;按本地日历天数算。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { tenantExpiry } from './tenantExpiry.ts'

const now = new Date(2026, 10, 11, 10, 0)
const at = (year: number, month: number, day: number, hour = 23, minute = 59, second = 59) =>
  new Date(year, month - 1, day, hour, minute, second).toISOString()

test('没有到期时间时长期有效,读不出的时间单独说明', () => {
  assert.deepEqual(tenantExpiry(undefined, now), { kind: 'none' })
  assert.deepEqual(tenantExpiry('', now), { kind: 'none' })
  assert.deepEqual(tenantExpiry('not a time', now), { kind: 'unknown' })
})

test('到期时刻已过就是已过期,哪怕是今天早些时候', () => {
  assert.deepEqual(tenantExpiry(at(2026, 11, 11, 9, 0, 0), now), { kind: 'expired' })
  assert.deepEqual(tenantExpiry(at(2026, 10, 1), now), { kind: 'expired' })
})

test('30 天内(含今天)到期给出剩余天数', () => {
  assert.deepEqual(tenantExpiry(at(2026, 11, 11), now), { kind: 'soon', days: 0 })
  assert.deepEqual(tenantExpiry(at(2026, 11, 12), now), { kind: 'soon', days: 1 })
  assert.deepEqual(tenantExpiry(at(2026, 12, 11), now), { kind: 'soon', days: 30 })
})

test('更远的到期日是正常状态,跨年按日历天数算', () => {
  assert.deepEqual(tenantExpiry(at(2026, 12, 12), now), { kind: 'normal', days: 31 })
  assert.deepEqual(tenantExpiry(at(2027, 1, 1), now), { kind: 'normal', days: 51 })
})
