// 验证运营统计的自然日范围和图表量纲。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { statisticsDateRange, STATISTICS_METRIC_GROUPS } from './statisticsPresentation.ts'

test('近N天包含今天，跨月和跨年不多取一天', () => {
  const now = new Date(2026, 0, 3, 12)
  for (const [range, from] of [['7', '2025-12-28'], ['30', '2025-12-05'], ['90', '2025-10-06']]) {
    assert.deepEqual(statisticsDateRange({ range, from: '', to: '' }, now), {
      from, to: '2026-01-03',
    })
  }
  assert.equal(now.getDate(), 3)
})

test('预设按本地自然日计算，跨夏令时仍为连续日期', () => {
  assert.deepEqual(statisticsDateRange({ range: '7', from: '', to: '' }, new Date(2026, 2, 10, 12)), {
    from: '2026-03-04', to: '2026-03-10',
  })
})

test('自定义闭区间接受闰日与同一天，清除首尾空白', () => {
  for (const [from, to] of [['2024-02-29', '2024-03-01'], ['2026-11-24', '2026-11-24']]) {
    assert.deepEqual(statisticsDateRange({ range: 'custom', from: ` ${from} `, to }), { from, to })
  }
})

test('缺失、无效或颠倒的自定义日期回到近30天', () => {
  const now = new Date(2026, 10, 24, 12)
  for (const [from, to] of [
    ['', '2026-11-24'], ['2026-11-01', ''],
    ['2026-02-29', '2026-03-01'], ['2026-02-30', '2026-03-01'],
    ['2026-13-01', '2026-13-03'], ['2026-2-01', '2026-02-02'],
    ['2026-11-24', '2026-11-01'],
  ]) {
    assert.deepEqual(statisticsDateRange({ range: 'custom', from, to }, now), {
      from: '2026-10-26', to: '2026-11-24',
    })
  }
})

test('预设不使用残留自定义字段，未知预设使用近30天', () => {
  const now = new Date(2026, 10, 24, 12)
  assert.deepEqual(statisticsDateRange({ range: '7', from: '2020-01-01', to: '2020-01-01' }, now), {
    from: '2026-11-18', to: '2026-11-24',
  })
  assert.deepEqual(statisticsDateRange({ range: 'invalid', from: '', to: '' }, now), {
    from: '2026-10-26', to: '2026-11-24',
  })
})

test('环境数量和学习秒数不共用纵轴', () => {
  const resource = STATISTICS_METRIC_GROUPS.find((group) => group.keys.includes('active_sandbox_count'))
  const learning = STATISTICS_METRIC_GROUPS.find((group) => group.keys.includes('learning_duration_sec'))
  assert.ok(resource)
  assert.ok(learning)
  assert.notEqual(resource.id, learning.id)
})
