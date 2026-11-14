// passwordRules.test.ts 验证密码规则的逐条判断:与后端 ValidatePassword 同一口径(至少 8 位、含字母、含数字)。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { passwordRuleStates } from './passwordRules.ts'

test('空密码三条都未满足', () => {
  assert.deepEqual(passwordRuleStates(''), { minLength: false, letter: false, digit: false })
})

test('只有字母或只有数字时对应条目满足', () => {
  assert.deepEqual(passwordRuleStates('abcdefgh'), { minLength: true, letter: true, digit: false })
  assert.deepEqual(passwordRuleStates('1234'), { minLength: false, letter: false, digit: true })
})

test('三条都满足', () => {
  assert.deepEqual(passwordRuleStates('chain2026'), { minLength: true, letter: true, digit: true })
})

test('全角或中文字符不算字母,与后端一致', () => {
  assert.deepEqual(passwordRuleStates('区块链区块链12'), { minLength: true, letter: false, digit: true })
})
