// reviewQueue.test.ts 验证审阅队列的移动规则(设计 §6 QueueDetailLayout:上下方向键切换、提交后自动进下一条):
// 方向键在队列内移动且不越界;「下一条待处理」先往后找,后面没有再从头找,都没有就是全部处理完。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nextPendingId, stepQueue } from './reviewQueue.ts'

const items = [
  { id: 'a', done: true },
  { id: 'b', done: false },
  { id: 'c', done: true },
  { id: 'd', done: false },
]
const isPending = (item: { done: boolean }) => !item.done

test('方向键:下移与上移,到头到尾停住', () => {
  assert.equal(stepQueue(items, 'a', 1), 'b')
  assert.equal(stepQueue(items, 'b', -1), 'a')
  assert.equal(stepQueue(items, 'd', 1), 'd')
  assert.equal(stepQueue(items, 'a', -1), 'a')
})

test('方向键:当前没有选中时下移到第一条,空队列没有结果', () => {
  assert.equal(stepQueue(items, undefined, 1), 'a')
  assert.equal(stepQueue([], undefined, 1), undefined)
})

test('下一条待处理:先往后找,后面没有再从头找,不包含当前条', () => {
  assert.equal(nextPendingId(items, 'b', isPending), 'd')
  assert.equal(nextPendingId(items, 'd', isPending), 'b')
  assert.equal(nextPendingId([{ id: 'x', done: false }], 'x', isPending), undefined)
})

test('下一条待处理:全部处理完时为空', () => {
  assert.equal(
    nextPendingId(
      items.map((item) => ({ ...item, done: true })),
      'a',
      isPending,
    ),
    undefined,
  )
})
