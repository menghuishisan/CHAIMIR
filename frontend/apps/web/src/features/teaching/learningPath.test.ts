// learningPath.test.ts 验证学习路径:课时按章、课时顺序排列;「下一个课时」先取第一个学习中、再取第一个未开始;
// 上一节 / 下一节由同一顺序计算。进度缺失的课时视为未开始。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { adjacentLessons, orderedLessons, pickNextLesson, type PathInput } from './learningPath.ts'

const NOT_STARTED = 1
const IN_PROGRESS = 2
const DONE = 3

const input: PathInput = {
  chapters: [
    { id: 'c2', sort: 2 },
    { id: 'c1', sort: 1 },
  ],
  lessons: [
    { id: 'l3', chapter_id: 'c2', sort: 1 },
    { id: 'l2', chapter_id: 'c1', sort: 2 },
    { id: 'l1', chapter_id: 'c1', sort: 1 },
  ],
  progress: [],
}

test('课时按章序、再按章内序排列', () => {
  assert.deepEqual(orderedLessons(input).map((row) => row.lessonId), ['l1', 'l2', 'l3'])
})

test('没有进度记录时下一个课时是第一节', () => {
  assert.equal(pickNextLesson(orderedLessons(input))?.lessonId, 'l1')
})

test('有学习中的课时时优先取它,哪怕前面还有未开始的', () => {
  const rows = orderedLessons({ ...input, progress: [{ lesson_id: 'l3', status: IN_PROGRESS, duration_sec: 60 }] })
  assert.equal(pickNextLesson(rows)?.lessonId, 'l3')
})

test('没有学习中时取第一个未开始', () => {
  const rows = orderedLessons({
    ...input,
    progress: [
      { lesson_id: 'l1', status: DONE, duration_sec: 600 },
      { lesson_id: 'l2', status: NOT_STARTED, duration_sec: 0 },
    ],
  })
  assert.equal(pickNextLesson(rows)?.lessonId, 'l2')
})

test('全部学完时没有下一个课时', () => {
  const rows = orderedLessons({
    ...input,
    progress: ['l1', 'l2', 'l3'].map((id) => ({ lesson_id: id, status: DONE, duration_sec: 1 })),
  })
  assert.equal(pickNextLesson(rows), null)
})

test('上一节与下一节', () => {
  const rows = orderedLessons(input)
  assert.deepEqual(adjacentLessons(rows, 'l2'), { previous: 'l1', next: 'l3' })
  assert.deepEqual(adjacentLessons(rows, 'l1'), { previous: null, next: 'l2' })
  assert.deepEqual(adjacentLessons(rows, 'l3'), { previous: 'l2', next: null })
  assert.deepEqual(adjacentLessons(rows, 'missing'), { previous: null, next: null })
})
