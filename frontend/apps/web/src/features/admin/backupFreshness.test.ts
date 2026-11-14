// backupFreshness.test.ts 验证「最近一次成功备份」的说法:多久以前、是否超过 48 小时需要警示。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { backupFreshness } from './backupFreshness.ts'

const now = new Date(2026, 10, 11, 12, 0)
const hoursAgo = (hours: number) => new Date(now.getTime() - hours * 3_600_000).toISOString()

test('一小时内写分钟,超过一小时写小时', () => {
  assert.deepEqual(backupFreshness(hoursAgo(0.25), now), { text: '15 分钟前', stale: false })
  assert.deepEqual(backupFreshness(hoursAgo(5), now), { text: '5 小时前', stale: false })
  assert.deepEqual(backupFreshness(hoursAgo(47.9), now), { text: '47 小时前', stale: false })
})

test('超过 48 小时用天数并警示', () => {
  assert.deepEqual(backupFreshness(hoursAgo(48), now), { text: '2 天前', stale: true })
  assert.deepEqual(backupFreshness(hoursAgo(100), now), { text: '4 天前', stale: true })
})

test('不到一分钟写刚刚;时间读不出时警示', () => {
  assert.deepEqual(backupFreshness(hoursAgo(0), now), { text: '刚刚', stale: false })
  assert.deepEqual(backupFreshness('not a time', now), { text: '时间未知', stale: true })
})
