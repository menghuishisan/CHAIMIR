// formatters.test.ts 验证截止类与发生类时间的用户向说法;「现在」固定,输入用本地时间构造,与运行机器的时区无关。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  fileKindLabel,
  formatDateRange,
  formatDateTimeSeconds,
  formatRelativeDeadline,
  formatRelativeOccurrence,
} from './formatters.ts'

const now = new Date(2026, 10, 10, 15, 0)

/** at 用本地时间构造接口返回的那种 ISO 时间串。 */
function at(year: number, month: number, day: number, hour: number, minute: number, second = 0): string {
  return new Date(year, month, day, hour, minute, second).toISOString()
}

test('截止时间:今天、明天、几天后、已截止', () => {
  assert.deepEqual(formatRelativeDeadline(at(2026, 10, 10, 23, 59), now), { text: '今天 23:59 截止', urgency: 'urgent' })
  assert.deepEqual(formatRelativeDeadline(at(2026, 10, 11, 9, 0), now), { text: '明天 09:00 截止', urgency: 'urgent' })
  assert.deepEqual(formatRelativeDeadline(at(2026, 10, 13, 10, 0), now), { text: '还剩 3 天', urgency: 'urgent' })
  assert.deepEqual(formatRelativeDeadline(at(2026, 10, 14, 10, 0), now), { text: '还剩 4 天', urgency: 'normal' })
  assert.deepEqual(formatRelativeDeadline(at(2026, 10, 10, 14, 59), now), { text: '已截止', urgency: 'overdue' })
})

test('截止时间:跨年按日历天数算', () => {
  const lateDecember = new Date(2026, 11, 31, 20, 0)
  assert.deepEqual(formatRelativeDeadline(at(2027, 0, 1, 8, 0), lateDecember), { text: '明天 08:00 截止', urgency: 'urgent' })
})

test('截止时间:读不出的时间不报错', () => {
  assert.deepEqual(formatRelativeDeadline('not-a-date', now), { text: '时间未知', urgency: 'normal' })
})

test('发生时间:刚刚、几分钟前、今天、昨天、今年、往年', () => {
  assert.equal(formatRelativeOccurrence(at(2026, 10, 10, 14, 59, 40), now), '刚刚')
  assert.equal(formatRelativeOccurrence(at(2026, 10, 10, 14, 57), now), '3 分钟前')
  assert.equal(formatRelativeOccurrence(at(2026, 10, 10, 9, 5), now), '今天 09:05')
  assert.equal(formatRelativeOccurrence(at(2026, 10, 9, 14, 20), now), '昨天 14:20')
  assert.equal(formatRelativeOccurrence(at(2026, 10, 1, 8, 0), now), '11月1日')
  assert.equal(formatRelativeOccurrence(at(2025, 10, 1, 8, 0), now), '2025.11.01')
})

test('发生时间:服务器时钟略快于本机时显示刚刚,读不出的时间不报错', () => {
  assert.equal(formatRelativeOccurrence(at(2026, 10, 10, 15, 1), now), '刚刚')
  assert.equal(formatRelativeOccurrence('not-a-date', now), '时间未知')
})

test('日期区间写成 2026.09.01 – 2027.01.16', () => {
  assert.equal(formatDateRange(at(2026, 8, 1, 0, 0), at(2027, 0, 16, 23, 59)), '2026.09.01 – 2027.01.16')
  assert.equal(formatDateRange('not-a-date', at(2027, 0, 16, 0, 0)), '时间未知 – 2027.01.16')
})

test('文件种类按扩展名写成用户看得懂的说法,不认识的写扩展名大写', () => {
  assert.equal(fileKindLabel('拜占庭将军问题课件.pdf'), 'PDF 文档')
  assert.equal(fileKindLabel('讲义.PPTX'), '演示文稿')
  assert.equal(fileKindLabel('实验数据.xlsx'), '表格')
  assert.equal(fileKindLabel('代码.zip'), '压缩包')
  assert.equal(fileKindLabel('合约.sol'), 'SOL 文件')
  assert.equal(fileKindLabel('README'), undefined)
  assert.equal(fileKindLabel('.env'), undefined)
})

test('精确到秒的完整时间(审计、日志类页面用)', () => {
  assert.equal(formatDateTimeSeconds(at(2026, 10, 10, 9, 5, 7)), '2026/11/10 09:05:07')
  assert.equal(formatDateTimeSeconds(at(2026, 10, 10, 0, 0, 3)), '2026/11/10 00:00:03')
  assert.equal(formatDateTimeSeconds('not a time'), '时间未知')
})
