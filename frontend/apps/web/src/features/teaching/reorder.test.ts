// reorder.test.ts 验证章节与课时的「上移 / 下移」(设计 §7.4 课程详情·章节课时):与相邻一项互换序号,
// 到头到尾不动;只返回需要写回服务端的两条。序号重复时(旧数据)先按顺序重新编号再互换。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { moveItem } from './reorder.ts'

const items = [
  { id: 'a', sort: 1 },
  { id: 'b', sort: 2 },
  { id: 'c', sort: 3 },
]

test('上移:与上一项互换序号', () => {
  // 返回顺序与传入顺序一致
  assert.deepEqual(moveItem(items, 'b', -1), [
    { id: 'a', sort: 2 },
    { id: 'b', sort: 1 },
  ])
})

test('下移:与下一项互换序号', () => {
  assert.deepEqual(moveItem(items, 'b', 1), [
    { id: 'b', sort: 3 },
    { id: 'c', sort: 2 },
  ])
})

test('到头到尾或找不到时不动', () => {
  assert.deepEqual(moveItem(items, 'a', -1), [])
  assert.deepEqual(moveItem(items, 'c', 1), [])
  assert.deepEqual(moveItem(items, 'x', 1), [])
})

test('序号重复时先按顺序重新编号,只返回序号真正变了的项', () => {
  const dup = [
    { id: 'a', sort: 1 },
    { id: 'b', sort: 1 },
    { id: 'c', sort: 2 },
  ]
  // 重新编号 a1 b2 c3,c 上移 → a1 c2 b3:a、c 与原值相同,只有 b 要写回
  assert.deepEqual(moveItem(dup, 'c', -1), [{ id: 'b', sort: 3 }])
})

test('重新编号会带动后面的项时,这些项也一并写回', () => {
  const gaps = [
    { id: 'a', sort: 1 },
    { id: 'b', sort: 1 },
    { id: 'c', sort: 2 },
    { id: 'd', sort: 5 },
  ]
  // 重新编号 a1 b2 c3 d4,a 下移 → b1 a2 c3 d4
  assert.deepEqual(moveItem(gaps, 'a', 1), [
    { id: 'a', sort: 2 },
    { id: 'c', sort: 3 },
    { id: 'd', sort: 4 },
  ])
})
