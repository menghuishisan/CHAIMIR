// gradeLevel.test.ts 验证「分数 → 等级与绩点」:与后端 ComputeGPA 同一口径(按阈值降序,取第一个分数不低于阈值的档)。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { levelForScore } from './gradeLevel.ts'

const mapping = [
  { min: 60, grade: 'C', gpa: 2 },
  { min: 90, grade: 'A', gpa: 4 },
  { min: 0, grade: 'F', gpa: 0 },
  { min: 75, grade: 'B', gpa: 3 },
]

test('按阈值降序取第一个命中的档,与传入顺序无关', () => {
  assert.deepEqual(levelForScore(95, mapping), { grade: 'A', gpa: 4 })
  assert.deepEqual(levelForScore(90, mapping), { grade: 'A', gpa: 4 })
  assert.deepEqual(levelForScore(89.5, mapping), { grade: 'B', gpa: 3 })
  assert.deepEqual(levelForScore(60, mapping), { grade: 'C', gpa: 2 })
  assert.deepEqual(levelForScore(12, mapping), { grade: 'F', gpa: 0 })
})

test('没有配置或没有命中的档时返回 undefined', () => {
  assert.equal(levelForScore(80, []), undefined)
  assert.equal(levelForScore(-1, [{ min: 0, grade: 'F', gpa: 0 }]), undefined)
})
