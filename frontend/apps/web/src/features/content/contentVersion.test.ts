// contentVersion.test.ts 已发布题目「建新版本并编辑」时自动取的下一个版本号:在修订位上加一,跳过已存在的版本。

import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextFreeVersion } from './contentVersion.ts'

test('修订位加一', () => {
  assert.equal(nextFreeVersion('1.0.0', ['1.0.0']), '1.0.1')
  assert.equal(nextFreeVersion('2.3.9', ['2.3.9']), '2.3.10')
})

test('跳过已经存在的版本号', () => {
  assert.equal(nextFreeVersion('1.0.0', ['1.0.0', '1.0.1', '1.0.2']), '1.0.3')
  assert.equal(nextFreeVersion('1.0.1', ['1.0.0', '1.0.1', '1.0.2', '1.0.4']), '1.0.3')
})

test('版本号不是三段数字时从 1.0.1 起找空位', () => {
  assert.equal(nextFreeVersion('beta', ['beta']), '1.0.1')
  assert.equal(nextFreeVersion('1.0', ['1.0', '1.0.1']), '1.0.2')
})
