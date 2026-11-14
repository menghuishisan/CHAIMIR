// facets.test.ts 验证分段计数从列表接口的 facets 读取:各分段取对应取值,「全部」是该列之和,没返回分组时为未知。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { segmentCounts } from './facets.ts'

const SEGMENTS = [
  { value: 'pending', status: 1 },
  { value: 'approved', status: 2 },
  { value: 'rejected', status: 3 },
  { value: 'all' },
] as const

test('分段计数:各分段取对应取值的条数,没有记录的取值按 0', () => {
  const counts = segmentCounts({ status: { '1': 4, '2': 9 } }, 'status', SEGMENTS, (segment) => ('status' in segment ? segment.status : undefined))
  assert.deepEqual(counts, { pending: 4, approved: 9, rejected: 0, all: 13 })
})

test('分段计数:列表还没读到或接口没返回这一组时,数量未知', () => {
  assert.deepEqual(segmentCounts(undefined, 'status', SEGMENTS, (segment) => ('status' in segment ? segment.status : undefined)), {
    pending: undefined,
    approved: undefined,
    rejected: undefined,
    all: undefined,
  })
  assert.deepEqual(segmentCounts({ other: { '1': 2 } }, 'status', SEGMENTS, (segment) => ('status' in segment ? segment.status : undefined)).all, undefined)
})

test('分段计数:取值可以是字符串或布尔', () => {
  const channels = [{ value: 'import' }, { value: 'export' }, { value: 'all' }] as const
  const counts = segmentCounts({ channel: { import: 3, export: 1 } }, 'channel', channels, (segment) =>
    segment.value === 'all' ? undefined : segment.value,
  )
  assert.deepEqual(counts, { import: 3, export: 1, all: 4 })

  const unread = [{ value: 'unread' }, { value: 'all' }] as const
  const read = segmentCounts({ is_read: { false: 5, true: 20 } }, 'is_read', unread, (segment) =>
    segment.value === 'unread' ? false : undefined,
  )
  assert.deepEqual(read, { unread: 5, all: 25 })
})
