import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isAutomaticTheoryQuestion, isTheoryAnswerEmpty } from './theoryAnswers.ts'

test('作答状态保留判断 false 和填空原位置', () => {
  assert.equal(isTheoryAnswerEmpty(false), false)
  assert.equal(isTheoryAnswerEmpty(true), false)
  const blanks = ['', '第二空']
  assert.equal(isTheoryAnswerEmpty(blanks), false)
  assert.deepEqual(blanks, ['', '第二空'])
  for (const value of [undefined, '', ' ', [], ['', ' ']]) {
    assert.equal(isTheoryAnswerEmpty(value), true)
  }
})

test('自动作答只接受四种客观理论题', () => {
  for (const q_type of ['single_choice', 'multiple_choice', 'true_false', 'fill_blank']) {
    assert.equal(isAutomaticTheoryQuestion({ q_type }), true)
  }
  for (const q_type of ['short_answer', 'unknown', undefined, null]) {
    assert.equal(isAutomaticTheoryQuestion({ q_type }), false)
  }
})
